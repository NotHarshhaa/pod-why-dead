package analyzer

import (
	"fmt"
	"strings"
	"time"

	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

// Report is the structured death report.
type Report struct {
	PodName             string               `json:"pod_name"`
	Namespace           string               `json:"namespace"`
	NodeName            string               `json:"node_name"`
	Cause               string               `json:"cause"`
	CauseDetail         string               `json:"cause_detail"`
	ExitCodeExplanation string               `json:"exit_code_explanation,omitempty"`
	Containers          []ContainerReport    `json:"containers"`
	Timeline            []TimelineEntry      `json:"timeline"`
	NodePressure        *NodePressure        `json:"node_pressure,omitempty"`
	NodeInfo            *NodeInfo            `json:"node_info,omitempty"`
	PVCs                []PVCReport          `json:"pvcs,omitempty"`
	ResourceQuota       *QuotaReport         `json:"resource_quota,omitempty"`
	Scheduling          *SchedulingInfo      `json:"scheduling,omitempty"`
	LogLines            string               `json:"log_lines,omitempty"`
	LogLineCount        int                  `json:"log_line_count"`
	Recommendations     []string             `json:"recommendations,omitempty"`
	KubectlCommands     []string             `json:"kubectl_commands,omitempty"`
	RestartCount        int32                `json:"restart_count"`
	NoRecommendations   bool                 `json:"-"`
	ReferencedResources []ReferencedResource `json:"referenced_resources,omitempty"`
	NetworkPolicies     []NetworkPolicyInfo  `json:"network_policies,omitempty"`
	NamespaceStats      map[string]int32     `json:"namespace_stats,omitempty"`
}

// ReferencedResource holds information about a referenced ConfigMap or Secret.
type ReferencedResource struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Exists    bool   `json:"exists"`
}

// NetworkPolicyInfo holds network policy information.
type NetworkPolicyInfo struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	PodSelector []string `json:"pod_selector"`
	Ingress     bool     `json:"ingress"`
	Egress      bool     `json:"egress"`
}

// ContainerReport holds per-container death details.
type ContainerReport struct {
	Name          string `json:"name"`
	Image         string `json:"image"`
	ImageDigest   string `json:"image_digest,omitempty"`
	ExitCode      int32  `json:"exit_code"`
	Signal        int32  `json:"signal,omitempty"`
	Reason        string `json:"reason"`
	Message       string `json:"message,omitempty"`
	State         string `json:"state"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	CPURequest    string `json:"cpu_request,omitempty"`
	RestartCount  int32  `json:"restart_count"`
	KilledAt      string `json:"killed_at,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
	ProbeFailure  string `json:"probe_failure,omitempty"`
	Command       string `json:"command,omitempty"`
}

// TimelineEntry is a single event in the pod's timeline.
type TimelineEntry struct {
	Time    string    `json:"time"`
	Event   string    `json:"event"`
	RawTime time.Time `json:"-"`
}

// NodePressure holds node conditions at time of death.
type NodePressure struct {
	MemoryPressure    bool   `json:"memory_pressure"`
	DiskPressure      bool   `json:"disk_pressure"`
	PIDPressure       bool   `json:"pid_pressure"`
	NodeReady         bool   `json:"node_ready"`
	MemoryCapacity    string `json:"memory_capacity,omitempty"`
	MemoryAllocatable string `json:"memory_allocatable,omitempty"`
	CPUCapacity       string `json:"cpu_capacity,omitempty"`
	CPUAllocatable    string `json:"cpu_allocatable,omitempty"`
}

// NodeInfo holds detailed node information.
type NodeInfo struct {
	Name             string            `json:"name"`
	KernelVersion    string            `json:"kernel_version"`
	OSImage          string            `json:"os_image"`
	ContainerRuntime string            `json:"container_runtime"`
	KubeletVersion   string            `json:"kubelet_version"`
	Taints           []string          `json:"taints,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

// PVCReport holds PVC status information.
type PVCReport struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Bound        bool   `json:"bound"`
	Capacity     string `json:"capacity,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
	AccessModes  string `json:"access_modes,omitempty"`
}

// QuotaReport holds resource quota information.
type QuotaReport struct {
	Name       string `json:"name"`
	HardCPU    string `json:"hard_cpu"`
	HardMemory string `json:"hard_memory"`
	HardPods   string `json:"hard_pods"`
	UsedCPU    string `json:"used_cpu"`
	UsedMemory string `json:"used_memory"`
	UsedPods   string `json:"used_pods"`
}

// SchedulingInfo holds scheduling-related information.
type SchedulingInfo struct {
	NodeSelector  map[string]string `json:"node_selector,omitempty"`
	Tolerations   []string          `json:"tolerations,omitempty"`
	AffinityRules []string          `json:"affinity_rules,omitempty"`
}

// Analyze builds a full death report from gathered Kubernetes data.
func Analyze(podInfo *k8s.PodInfo, events []k8s.EventInfo, logs string, nodeConditions *k8s.NodeConditions, logLineCount int,
	nodeInfo *k8s.NodeInfo, pvcs []k8s.PVCInfo, quota *k8s.QuotaInfo, referencedResources []k8s.ReferencedResource,
	networkPolicies []k8s.NetworkPolicyInfo, namespaceStats map[string]int32) *Report {
	report := &Report{
		PodName:      podInfo.Name,
		Namespace:    podInfo.Namespace,
		NodeName:     podInfo.NodeName,
		LogLines:     logs,
		LogLineCount: logLineCount,
		RestartCount: podInfo.RestartCount,
	}

	// Add scheduling info
	if len(podInfo.NodeSelector) > 0 || len(podInfo.Tolerations) > 0 || len(podInfo.AffinityRules) > 0 {
		report.Scheduling = &SchedulingInfo{
			NodeSelector:  podInfo.NodeSelector,
			Tolerations:   podInfo.Tolerations,
			AffinityRules: podInfo.AffinityRules,
		}
	}

	// Add node info
	if nodeInfo != nil {
		taints := make([]string, len(nodeInfo.Taints))
		for i, t := range nodeInfo.Taints {
			taints[i] = fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
		}
		report.NodeInfo = &NodeInfo{
			Name:             nodeInfo.Name,
			KernelVersion:    nodeInfo.KernelVersion,
			OSImage:          nodeInfo.OSImage,
			ContainerRuntime: nodeInfo.ContainerRuntime,
			KubeletVersion:   nodeInfo.KubeletVersion,
			Taints:           taints,
			Labels:           nodeInfo.Labels,
		}
	}

	// Add PVCs
	if len(pvcs) > 0 {
		for _, pvc := range pvcs {
			report.PVCs = append(report.PVCs, PVCReport{
				Name:         pvc.Name,
				Status:       pvc.Status,
				Bound:        pvc.Bound,
				Capacity:     pvc.Capacity,
				StorageClass: pvc.StorageClass,
				AccessModes:  pvc.AccessModes,
			})
		}
	}

	// Add quota info
	if quota != nil {
		report.ResourceQuota = &QuotaReport{
			Name:       quota.Name,
			HardCPU:    quota.HardCPU,
			HardMemory: quota.HardMemory,
			HardPods:   quota.HardPods,
			UsedCPU:    quota.UsedCPU,
			UsedMemory: quota.UsedMemory,
			UsedPods:   quota.UsedPods,
		}
	}

	// Build container reports
	for _, c := range podInfo.Containers {
		cr := ContainerReport{
			Name:          c.Name,
			Image:         c.Image,
			ImageDigest:   podInfo.ImageDigest,
			ExitCode:      c.ExitCode,
			Signal:        c.Signal,
			Reason:        c.Reason,
			Message:       c.Message,
			State:         c.State,
			MemoryLimit:   c.MemoryLimit,
			MemoryRequest: c.MemoryRequest,
			CPULimit:      c.CPULimit,
			CPURequest:    c.CPURequest,
			RestartCount:  c.RestartCount,
		}
		if c.FinishedAt != nil {
			cr.KilledAt = c.FinishedAt.UTC().Format("2006-01-02 15:04:05 UTC")
		}
		if c.StartedAt != nil {
			cr.StartedAt = c.StartedAt.UTC().Format("2006-01-02 15:04:05 UTC")
		}
		if len(c.Command) > 0 {
			cr.Command = strings.Join(c.Command, " ")
		}

		// Check for probe failures in events
		if c.LivenessProbe != nil {
			cr.ProbeFailure = findProbeFailure(events, "Liveness", c.LivenessProbe)
		}
		if cr.ProbeFailure == "" && c.ReadinessProbe != nil {
			cr.ProbeFailure = findProbeFailure(events, "Readiness", c.ReadinessProbe)
		}

		report.Containers = append(report.Containers, cr)
	}

	// Add referenced resources
	if len(referencedResources) > 0 {
		for _, r := range referencedResources {
			report.ReferencedResources = append(report.ReferencedResources, ReferencedResource{
				Name:      r.Name,
				Kind:      r.Kind,
				Namespace: r.Namespace,
				Exists:    r.Exists,
			})
		}
	}

	// Add network policies
	if len(networkPolicies) > 0 {
		for _, np := range networkPolicies {
			report.NetworkPolicies = append(report.NetworkPolicies, NetworkPolicyInfo{
				Name:        np.Name,
				Namespace:   np.Namespace,
				PodSelector: np.PodSelector,
				Ingress:     np.Ingress,
				Egress:      np.Egress,
			})
		}
	}

	// Add namespace stats
	if namespaceStats != nil {
		report.NamespaceStats = namespaceStats
	}

	// Determine cause
	report.Cause, report.CauseDetail, report.ExitCodeExplanation = determineCause(podInfo, events, report.Containers)

	// Build timeline
	report.Timeline = buildTimeline(podInfo, events)

	// Node pressure
	if nodeConditions != nil {
		report.NodePressure = &NodePressure{
			MemoryPressure:    nodeConditions.MemoryPressure,
			DiskPressure:      nodeConditions.DiskPressure,
			PIDPressure:       nodeConditions.PIDPressure,
			NodeReady:         nodeConditions.Ready,
			MemoryCapacity:    nodeConditions.MemoryCapacity,
			MemoryAllocatable: nodeConditions.MemoryAllocatable,
			CPUCapacity:       nodeConditions.CPUCapacity,
			CPUAllocatable:    nodeConditions.CPUAllocatable,
		}
	}

	// Generate recommendations and kubectl commands
	report.Recommendations, report.KubectlCommands = generateRecommendations(report)

	return report
}

func determineCause(podInfo *k8s.PodInfo, events []k8s.EventInfo, containers []ContainerReport) (string, string, string) {
	// Check pod-level eviction
	if podInfo.Reason == "Evicted" {
		return "Evicted", podInfo.Message, ""
	}

	// Check for scheduling failures (Pending pods)
	if podInfo.Phase == "Pending" {
		for _, e := range events {
			if e.Reason == "FailedScheduling" {
				return "Pending — never started", e.Message, ""
			}
		}
		return "Pending — never started", "Pod has not been scheduled", ""
	}

	// Check container-level causes
	for _, c := range containers {
		switch {
		case c.Reason == "OOMKilled":
			detail := fmt.Sprintf("Container %s (exit code %d)", c.Name, c.ExitCode)
			if c.MemoryLimit != "" {
				detail += fmt.Sprintf(", memory limit: %s", c.MemoryLimit)
			}
			return "OOMKilled", detail, explainExitCode(c.ExitCode)

		case c.Reason == "CrashLoopBackOff":
			detail := fmt.Sprintf("Container %s, restart count: %d", c.Name, c.RestartCount)
			return "CrashLoopBackOff", detail, explainExitCode(c.ExitCode)

		case c.Reason == "ImagePullBackOff" || c.Reason == "ErrImagePull":
			detail := fmt.Sprintf("Image: %s", c.Image)
			if c.Message != "" {
				detail += fmt.Sprintf(" — %s", c.Message)
			}
			return "ImagePullBackOff", detail, ""

		case c.Reason == "Error" || (c.ExitCode != 0 && c.State == "terminated"):
			reason := c.Reason
			if reason == "" {
				reason = fmt.Sprintf("Error (exit code %d)", c.ExitCode)
			}
			detail := fmt.Sprintf("Container %s, exit code: %d", c.Name, c.ExitCode)
			if c.Command != "" {
				detail += fmt.Sprintf(", command: %s", c.Command)
			}
			return reason, detail, explainExitCode(c.ExitCode)
		}
	}

	// Check for liveness probe failures in events
	for _, e := range events {
		if strings.Contains(e.Message, "Liveness probe failed") {
			return "Liveness probe failed", e.Message, ""
		}
	}

	// Fallback
	if podInfo.Reason != "" {
		return podInfo.Reason, podInfo.Message, ""
	}

	return "Unknown", "Could not determine cause of death", ""
}

func findProbeFailure(events []k8s.EventInfo, probeType string, probe *k8s.ProbeInfo) string {
	for _, e := range events {
		if strings.Contains(e.Message, probeType+" probe failed") {
			return fmt.Sprintf("%s probe (%s %s:%s) failed — %s",
				probeType, probe.Type, probe.Path, probe.Port, e.Message)
		}
	}
	return ""
}

func buildTimeline(podInfo *k8s.PodInfo, events []k8s.EventInfo) []TimelineEntry {
	var timeline []TimelineEntry

	for _, e := range events {
		if e.Time.IsZero() {
			continue
		}
		entry := TimelineEntry{
			Time:    e.Time.UTC().Format("15:04:05"),
			Event:   fmt.Sprintf("[%s] %s", e.Reason, e.Message),
			RawTime: e.Time,
		}
		timeline = append(timeline, entry)
	}

	// Add container start/finish events from pod info
	for _, c := range podInfo.Containers {
		if c.StartedAt != nil {
			timeline = append(timeline, TimelineEntry{
				Time:    c.StartedAt.UTC().Format("15:04:05"),
				Event:   fmt.Sprintf("Container %s started", c.Name),
				RawTime: *c.StartedAt,
			})
		}
		if c.FinishedAt != nil {
			reason := c.Reason
			if reason == "" {
				reason = fmt.Sprintf("exit code %d", c.ExitCode)
			}
			timeline = append(timeline, TimelineEntry{
				Time:    c.FinishedAt.UTC().Format("15:04:05"),
				Event:   fmt.Sprintf("Container %s terminated: %s", c.Name, reason),
				RawTime: *c.FinishedAt,
			})
		}
	}

	// Sort by time
	for i := 0; i < len(timeline); i++ {
		for j := i + 1; j < len(timeline); j++ {
			if timeline[j].RawTime.Before(timeline[i].RawTime) {
				timeline[i], timeline[j] = timeline[j], timeline[i]
			}
		}
	}

	// Deduplicate adjacent entries with same event text
	if len(timeline) > 1 {
		deduped := []TimelineEntry{timeline[0]}
		for i := 1; i < len(timeline); i++ {
			if timeline[i].Event != timeline[i-1].Event {
				deduped = append(deduped, timeline[i])
			}
		}
		timeline = deduped
	}

	return timeline
}

func explainExitCode(code int32) string {
	if code == 0 {
		return "Exit code 0 means success (container exited normally)"
	}

	switch code {
	case 1:
		return "Exit code 1: General application error (check logs for details)"
	case 2:
		return "Exit code 2: Shell misuse or incorrect command usage"
	case 126:
		return "Exit code 126: Command found but not executable (check file permissions)"
	case 127:
		return "Exit code 127: Command not found (check if binary exists in image)"
	case 128:
		return "Exit code 128: Invalid exit argument passed to exit command"
	case 130:
		return "Exit code 130: Container terminated by SIGINT (Ctrl+C)"
	case 137:
		return "Exit code 137: Container killed by SIGKILL (typically OOMKilled)"
	case 139:
		return "Exit code 139: Container terminated by SIGSEGV (segmentation fault)"
	case 143:
		return "Exit code 143: Container terminated by SIGTERM (graceful shutdown request)"
	default:
		if code > 128 {
			signal := code - 128
			return fmt.Sprintf("Exit code %d: Process killed by signal %d", code, signal)
		}
		return fmt.Sprintf("Exit code %d: Application-specific error (check logs)", code)
	}
}
