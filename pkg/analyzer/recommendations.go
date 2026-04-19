package analyzer

import (
	"fmt"
	"strings"
)

// generateRecommendations produces actionable suggestions based on the death cause.
func generateRecommendations(report *Report) ([]string, []string) {
	var recs []string
	var commands []string

	switch {
	case strings.Contains(report.Cause, "OOMKilled"):
		recs, commands = oomKilledRecommendations(report, commands)
	case strings.Contains(report.Cause, "CrashLoopBackOff"):
		recs, commands = crashLoopRecommendations(report, commands)
	case strings.Contains(report.Cause, "ImagePullBackOff") || strings.Contains(report.Cause, "ErrImagePull"):
		recs, commands = imagePullRecommendations(report, commands)
	case strings.Contains(report.Cause, "Evicted"):
		recs, commands = evictedRecommendations(report, commands)
	case strings.Contains(report.Cause, "Liveness probe failed"):
		recs, commands = livenessProbeRecommendations(report, commands)
	case strings.Contains(report.Cause, "Pending"):
		recs, commands = pendingRecommendations(report, commands)
	case strings.Contains(report.Cause, "Error"):
		recs, commands = errorRecommendations(report, commands)
	default:
		recs = append(recs, "Review pod logs and events for more details on the failure cause.")
		commands = append(commands, fmt.Sprintf("kubectl logs %s/%s --previous", report.Namespace, report.PodName))
		commands = append(commands, fmt.Sprintf("kubectl describe pod %s/%s", report.Namespace, report.PodName))
	}

	// Add node-level recommendations
	if report.NodePressure != nil {
		recs = append(recs, nodePressureRecommendations(report.NodePressure)...)
		if report.NodeName != "" {
			commands = append(commands, fmt.Sprintf("kubectl describe node %s", report.NodeName))
		}
	}

	// Add PVC recommendations
	if len(report.PVCs) > 0 {
		for _, pvc := range report.PVCs {
			if !pvc.Bound {
				recs = append(recs, fmt.Sprintf("PVC %s is not bound — check PV availability and storage class.", pvc.Name))
				commands = append(commands, fmt.Sprintf("kubectl describe pvc %s -n %s", pvc.Name, report.Namespace))
			}
		}
	}

	// Add quota recommendations
	if report.ResourceQuota != nil {
		recs = append(recs, "Check namespace resource quotas — you may be hitting limits.")
		commands = append(commands, fmt.Sprintf("kubectl get resourcequota -n %s", report.Namespace))
	}

	// Add scheduling recommendations
	if report.Scheduling != nil {
		if len(report.Scheduling.Tolerations) > 0 || len(report.Scheduling.AffinityRules) > 0 {
			recs = append(recs, "Pod has scheduling constraints (tolerations/affinity) that may affect placement.")
		}
	}

	// Add restart-based recommendations
	if report.RestartCount > 5 {
		recs = append(recs, fmt.Sprintf(
			"Pod has restarted %d times. Consider investigating the root cause rather than relying on restart recovery.",
			report.RestartCount))
	}

	// Add common commands if not already present
	if len(commands) == 0 {
		commands = append(commands, fmt.Sprintf("kubectl logs %s/%s --previous", report.Namespace, report.PodName))
		commands = append(commands, fmt.Sprintf("kubectl describe pod %s/%s", report.Namespace, report.PodName))
	}

	return recs, commands
}

func oomKilledRecommendations(report *Report, commands []string) ([]string, []string) {
	var recs []string
	for _, c := range report.Containers {
		if c.Reason == "OOMKilled" {
			if c.MemoryLimit != "" {
				recs = append(recs, fmt.Sprintf(
					"Increase memory limit above %s for container %s, or investigate what causes high memory consumption.",
					c.MemoryLimit, c.Name))
			} else {
				recs = append(recs, fmt.Sprintf(
					"Set a memory limit for container %s and profile memory usage to determine appropriate limits.",
					c.Name))
			}
			recs = append(recs,
				"Check for memory leaks — look for unbounded caches, growing buffers, or large batch processing.",
				"Consider using a VPA (Vertical Pod Autoscaler) to right-size memory limits automatically.")
			commands = append(commands, fmt.Sprintf("kubectl top pod %s -n %s", report.PodName, report.Namespace))
			break
		}
	}
	return recs, commands
}

func crashLoopRecommendations(report *Report, commands []string) ([]string, []string) {
	recs := []string{
		"Check the previous container logs for the root cause of repeated crashes.",
		"Verify the container's entrypoint/command is correct and the application starts successfully.",
		"Ensure all required environment variables, ConfigMaps, and Secrets are present and valid.",
	}
	if report.RestartCount > 10 {
		recs = append(recs, "The high restart count suggests a persistent issue — consider rolling back to a known-good version.")
	}
	commands = append(commands, fmt.Sprintf("kubectl logs %s/%s --previous", report.Namespace, report.PodName))
	return recs, commands
}

func imagePullRecommendations(report *Report, commands []string) ([]string, []string) {
	recs := []string{
		"Verify the image name and tag are correct.",
		"Check that the image exists in the registry.",
		"If using a private registry, ensure imagePullSecrets are configured on the pod or service account.",
		"Check network connectivity from the node to the container registry.",
	}
	for _, c := range report.Containers {
		commands = append(commands, fmt.Sprintf("docker pull %s", c.Image))
	}
	return recs, commands
}

func evictedRecommendations(report *Report, commands []string) ([]string, []string) {
	recs := []string{
		"The pod was evicted due to node resource pressure.",
		"Review resource requests and limits to ensure they match actual usage.",
		"Consider using PodDisruptionBudgets to control eviction behavior.",
		"Consider using pod priority classes to protect critical workloads from eviction.",
	}
	if report.CauseDetail != "" {
		recs = append(recs, fmt.Sprintf("Eviction detail: %s", report.CauseDetail))
	}
	if report.NodeName != "" {
		commands = append(commands, fmt.Sprintf("kubectl describe node %s", report.NodeName))
	}
	return recs, commands
}

func livenessProbeRecommendations(report *Report, commands []string) ([]string, []string) {
	recs := []string{
		"The liveness probe killed the container because it was unresponsive.",
		"Increase initialDelaySeconds if the application needs more time to start.",
		"Increase failureThreshold or periodSeconds to be more tolerant of transient slowness.",
		"Verify the probe endpoint or command actually works when the application is healthy.",
	}
	for _, c := range report.Containers {
		if c.ProbeFailure != "" {
			recs = append(recs, fmt.Sprintf("Probe details: %s", c.ProbeFailure))
		}
	}
	commands = append(commands, fmt.Sprintf("kubectl get pod %s/%s -o yaml", report.Namespace, report.PodName))
	return recs, commands
}

func pendingRecommendations(report *Report, commands []string) ([]string, []string) {
	recs := []string{
		"The pod could not be scheduled onto any node.",
	}
	if strings.Contains(report.CauseDetail, "Insufficient cpu") {
		recs = append(recs, "Reduce CPU requests or add more nodes with available CPU.")
	}
	if strings.Contains(report.CauseDetail, "Insufficient memory") {
		recs = append(recs, "Reduce memory requests or add more nodes with available memory.")
	}
	if strings.Contains(report.CauseDetail, "taints") || strings.Contains(report.CauseDetail, "toleration") {
		recs = append(recs, "Check node taints and pod tolerations — the pod may need a toleration to schedule on available nodes.")
	}
	if strings.Contains(report.CauseDetail, "affinity") {
		recs = append(recs, "Review node affinity rules — they may be too restrictive for the available nodes.")
	}
	recs = append(recs, "Run 'kubectl describe nodes' to see available resources across the cluster.")
	commands = append(commands, fmt.Sprintf("kubectl describe nodes"))
	if report.Scheduling != nil && len(report.Scheduling.NodeSelector) > 0 {
		commands = append(commands, fmt.Sprintf("kubectl get nodes -l %s", formatNodeSelector(report.Scheduling.NodeSelector)))
	}
	return recs, commands
}

func errorRecommendations(report *Report, commands []string) ([]string, []string) {
	recs := []string{
		"The container exited with a non-zero exit code indicating an application error.",
		"Check the container logs for error messages and stack traces.",
		"Verify the container command, arguments, and environment variables are correct.",
	}
	for _, c := range report.Containers {
		if c.ExitCode == 1 {
			recs = append(recs, "Exit code 1 typically indicates a general application error.")
		} else if c.ExitCode == 2 {
			recs = append(recs, "Exit code 2 typically indicates a shell misuse or incorrect command usage.")
		} else if c.ExitCode == 126 {
			recs = append(recs, "Exit code 126 means the command was found but is not executable — check file permissions.")
		} else if c.ExitCode == 127 {
			recs = append(recs, "Exit code 127 means command not found — check the image contains the required binary.")
		} else if c.ExitCode == 128 {
			recs = append(recs, "Exit code 128 indicates invalid exit argument.")
		} else if c.ExitCode > 128 {
			recs = append(recs, fmt.Sprintf("Exit code %d means the process was killed by signal %d.", c.ExitCode, c.ExitCode-128))
		}
	}
	commands = append(commands, fmt.Sprintf("kubectl logs %s/%s --previous", report.Namespace, report.PodName))
	return recs, commands
}

func formatNodeSelector(selector map[string]string) string {
	var parts []string
	for k, v := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

func nodePressureRecommendations(np *NodePressure) []string {
	var recs []string
	if np.MemoryPressure {
		rec := "Node is under memory pressure."
		if np.MemoryAllocatable != "" {
			rec += fmt.Sprintf(" Allocatable memory: %s.", np.MemoryAllocatable)
		}
		rec += " Consider spreading workloads across more nodes or increasing node size."
		recs = append(recs, rec)
	}
	if np.DiskPressure {
		recs = append(recs, "Node is under disk pressure. Clean up unused images/volumes or increase disk size.")
	}
	if np.PIDPressure {
		recs = append(recs, "Node is under PID pressure. Check for runaway processes or increase the PID limit.")
	}
	if !np.NodeReady {
		recs = append(recs, "Node is not in Ready state. The node itself may be unhealthy — check node status and kubelet logs.")
	}
	return recs
}
