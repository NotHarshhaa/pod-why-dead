package analyzer

import (
	"fmt"
	"strings"
	"time"

	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

// Report is the structured death report.
type Report struct {
	PodName           string            `json:"pod_name"`
	Namespace         string            `json:"namespace"`
	NodeName          string            `json:"node_name"`
	Cause             string            `json:"cause"`
	CauseDetail       string            `json:"cause_detail"`
	Containers        []ContainerReport `json:"containers"`
	Timeline          []TimelineEntry   `json:"timeline"`
	NodePressure      *NodePressure     `json:"node_pressure,omitempty"`
	LogLines          string            `json:"log_lines,omitempty"`
	LogLineCount      int               `json:"log_line_count"`
	Recommendations   []string          `json:"recommendations,omitempty"`
	RestartCount      int32             `json:"restart_count"`
	NoRecommendations bool              `json:"-"`
}

// ContainerReport holds per-container death details.
type ContainerReport struct {
	Name          string `json:"name"`
	Image         string `json:"image"`
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
	Time    string `json:"time"`
	Event   string `json:"event"`
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

// Analyze builds a full death report from gathered Kubernetes data.
func Analyze(podInfo *k8s.PodInfo, events []k8s.EventInfo, logs string, nodeConditions *k8s.NodeConditions, logLineCount int) *Report {
	report := &Report{
		PodName:      podInfo.Name,
		Namespace:    podInfo.Namespace,
		NodeName:     podInfo.NodeName,
		LogLines:     logs,
		LogLineCount: logLineCount,
		RestartCount: podInfo.RestartCount,
	}

	// Build container reports
	for _, c := range podInfo.Containers {
		cr := ContainerReport{
			Name:          c.Name,
			Image:         c.Image,
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

	// Determine cause
	report.Cause, report.CauseDetail = determineCause(podInfo, events, report.Containers)

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

	// Generate recommendations
	report.Recommendations = generateRecommendations(report)

	return report
}

func determineCause(podInfo *k8s.PodInfo, events []k8s.EventInfo, containers []ContainerReport) (string, string) {
	// Check pod-level eviction
	if podInfo.Reason == "Evicted" {
		return "Evicted", podInfo.Message
	}

	// Check for scheduling failures (Pending pods)
	if podInfo.Phase == "Pending" {
		for _, e := range events {
			if e.Reason == "FailedScheduling" {
				return "Pending — never started", e.Message
			}
		}
		return "Pending — never started", "Pod has not been scheduled"
	}

	// Check container-level causes
	for _, c := range containers {
		switch {
		case c.Reason == "OOMKilled":
			detail := fmt.Sprintf("Container %s (exit code %d)", c.Name, c.ExitCode)
			if c.MemoryLimit != "" {
				detail += fmt.Sprintf(", memory limit: %s", c.MemoryLimit)
			}
			return "OOMKilled", detail

		case c.Reason == "CrashLoopBackOff":
			detail := fmt.Sprintf("Container %s, restart count: %d", c.Name, c.RestartCount)
			return "CrashLoopBackOff", detail

		case c.Reason == "ImagePullBackOff" || c.Reason == "ErrImagePull":
			detail := fmt.Sprintf("Image: %s", c.Image)
			if c.Message != "" {
				detail += fmt.Sprintf(" — %s", c.Message)
			}
			return "ImagePullBackOff", detail

		case c.Reason == "Error" || (c.ExitCode != 0 && c.State == "terminated"):
			reason := c.Reason
			if reason == "" {
				reason = fmt.Sprintf("Error (exit code %d)", c.ExitCode)
			}
			detail := fmt.Sprintf("Container %s, exit code: %d", c.Name, c.ExitCode)
			if c.Command != "" {
				detail += fmt.Sprintf(", command: %s", c.Command)
			}
			return reason, detail
		}
	}

	// Check for liveness probe failures in events
	for _, e := range events {
		if strings.Contains(e.Message, "Liveness probe failed") {
			return "Liveness probe failed", e.Message
		}
	}

	// Fallback
	if podInfo.Reason != "" {
		return podInfo.Reason, podInfo.Message
	}

	return "Unknown", "Could not determine cause of death"
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
