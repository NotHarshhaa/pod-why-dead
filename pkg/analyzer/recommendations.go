package analyzer

import (
	"fmt"
	"strings"
)

// generateRecommendations produces actionable suggestions based on the death cause.
func generateRecommendations(report *Report) []string {
	var recs []string

	switch {
	case strings.Contains(report.Cause, "OOMKilled"):
		recs = append(recs, oomKilledRecommendations(report)...)
	case strings.Contains(report.Cause, "CrashLoopBackOff"):
		recs = append(recs, crashLoopRecommendations(report)...)
	case strings.Contains(report.Cause, "ImagePullBackOff") || strings.Contains(report.Cause, "ErrImagePull"):
		recs = append(recs, imagePullRecommendations(report)...)
	case strings.Contains(report.Cause, "Evicted"):
		recs = append(recs, evictedRecommendations(report)...)
	case strings.Contains(report.Cause, "Liveness probe failed"):
		recs = append(recs, livenessProbeRecommendations(report)...)
	case strings.Contains(report.Cause, "Pending"):
		recs = append(recs, pendingRecommendations(report)...)
	case strings.Contains(report.Cause, "Error"):
		recs = append(recs, errorRecommendations(report)...)
	default:
		recs = append(recs, "Review pod logs and events for more details on the failure cause.")
	}

	// Add node-level recommendations
	if report.NodePressure != nil {
		recs = append(recs, nodePressureRecommendations(report.NodePressure)...)
	}

	// Add restart-based recommendations
	if report.RestartCount > 5 {
		recs = append(recs, fmt.Sprintf(
			"Pod has restarted %d times. Consider investigating the root cause rather than relying on restart recovery.",
			report.RestartCount))
	}

	return recs
}

func oomKilledRecommendations(report *Report) []string {
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
			break
		}
	}
	return recs
}

func crashLoopRecommendations(report *Report) []string {
	recs := []string{
		"Check the previous container logs for the root cause of repeated crashes.",
		"Verify the container's entrypoint/command is correct and the application starts successfully.",
		"Ensure all required environment variables, ConfigMaps, and Secrets are present and valid.",
	}
	if report.RestartCount > 10 {
		recs = append(recs, "The high restart count suggests a persistent issue — consider rolling back to a known-good version.")
	}
	return recs
}

func imagePullRecommendations(report *Report) []string {
	recs := []string{
		"Verify the image name and tag are correct.",
		"Check that the image exists in the registry.",
		"If using a private registry, ensure imagePullSecrets are configured on the pod or service account.",
		"Check network connectivity from the node to the container registry.",
	}
	return recs
}

func evictedRecommendations(report *Report) []string {
	recs := []string{
		"The pod was evicted due to node resource pressure.",
		"Review resource requests and limits to ensure they match actual usage.",
		"Consider using PodDisruptionBudgets to control eviction behavior.",
		"Consider using pod priority classes to protect critical workloads from eviction.",
	}
	if report.CauseDetail != "" {
		recs = append(recs, fmt.Sprintf("Eviction detail: %s", report.CauseDetail))
	}
	return recs
}

func livenessProbeRecommendations(report *Report) []string {
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
	return recs
}

func pendingRecommendations(report *Report) []string {
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
	return recs
}

func errorRecommendations(report *Report) []string {
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
	return recs
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
