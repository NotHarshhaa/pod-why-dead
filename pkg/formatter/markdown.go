package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/NotHarshhaa/pod-why-dead/pkg/analyzer"
	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

func formatMarkdown(w io.Writer, report *analyzer.Report) error {
	fmt.Fprintf(w, "# Pod Death Report\n\n")

	fmt.Fprintf(w, "| Field | Value |\n")
	fmt.Fprintf(w, "|---|---|\n")
	fmt.Fprintf(w, "| **Pod** | `%s` |\n", report.PodName)
	fmt.Fprintf(w, "| **Namespace** | `%s` |\n", report.Namespace)
	if report.NodeName != "" {
		fmt.Fprintf(w, "| **Node** | `%s` |\n", report.NodeName)
	}
	fmt.Fprintf(w, "| **Cause** | **%s** |\n", report.Cause)
	if report.RestartCount > 0 {
		fmt.Fprintf(w, "| **Restart Count** | %d |\n", report.RestartCount)
	}
	fmt.Fprintln(w)

	if report.CauseDetail != "" {
		fmt.Fprintf(w, "> %s\n\n", report.CauseDetail)
	}

	// Containers
	fmt.Fprintf(w, "## Containers\n\n")
	for _, c := range report.Containers {
		if c.Reason == "" && c.ExitCode == 0 && c.State == "running" {
			continue
		}
		fmt.Fprintf(w, "### %s\n\n", c.Name)
		fmt.Fprintf(w, "| Field | Value |\n")
		fmt.Fprintf(w, "|---|---|\n")
		fmt.Fprintf(w, "| State | %s |\n", c.State)
		if c.Reason != "" {
			fmt.Fprintf(w, "| Reason | %s |\n", c.Reason)
		}
		fmt.Fprintf(w, "| Exit Code | %d |\n", c.ExitCode)
		if c.MemoryLimit != "" {
			fmt.Fprintf(w, "| Memory Limit | %s |\n", c.MemoryLimit)
		}
		if c.MemoryRequest != "" {
			fmt.Fprintf(w, "| Memory Request | %s |\n", c.MemoryRequest)
		}
		if c.CPULimit != "" {
			fmt.Fprintf(w, "| CPU Limit | %s |\n", c.CPULimit)
		}
		if c.CPURequest != "" {
			fmt.Fprintf(w, "| CPU Request | %s |\n", c.CPURequest)
		}
		if c.KilledAt != "" {
			fmt.Fprintf(w, "| Killed At | %s |\n", c.KilledAt)
		}
		if c.RestartCount > 0 {
			fmt.Fprintf(w, "| Restart Count | %d |\n", c.RestartCount)
		}
		if c.Command != "" {
			fmt.Fprintf(w, "| Command | `%s` |\n", c.Command)
		}
		if c.ProbeFailure != "" {
			fmt.Fprintf(w, "| Probe Failure | %s |\n", c.ProbeFailure)
		}
		fmt.Fprintln(w)
	}

	// Timeline
	if len(report.Timeline) > 0 {
		fmt.Fprintf(w, "## Timeline\n\n")
		fmt.Fprintf(w, "| Time | Event |\n")
		fmt.Fprintf(w, "|---|---|\n")
		for _, t := range report.Timeline {
			fmt.Fprintf(w, "| %s | %s |\n", t.Time, t.Event)
		}
		fmt.Fprintln(w)
	}

	// Node pressure
	if report.NodePressure != nil {
		fmt.Fprintf(w, "## Node Pressure\n\n")
		fmt.Fprintf(w, "| Condition | Status |\n")
		fmt.Fprintf(w, "|---|---|\n")
		fmt.Fprintf(w, "| Memory Pressure | %s |\n", boolToStatus(report.NodePressure.MemoryPressure))
		fmt.Fprintf(w, "| Disk Pressure | %s |\n", boolToStatus(report.NodePressure.DiskPressure))
		fmt.Fprintf(w, "| PID Pressure | %s |\n", boolToStatus(report.NodePressure.PIDPressure))
		fmt.Fprintf(w, "| Node Ready | %s |\n", boolToReadyStatus(report.NodePressure.NodeReady))
		if report.NodePressure.MemoryAllocatable != "" {
			fmt.Fprintf(w, "| Memory Allocatable | %s |\n", report.NodePressure.MemoryAllocatable)
		}
		if report.NodePressure.MemoryCapacity != "" {
			fmt.Fprintf(w, "| Memory Capacity | %s |\n", report.NodePressure.MemoryCapacity)
		}
		fmt.Fprintln(w)
	}

	// Logs
	if report.LogLines != "" {
		fmt.Fprintf(w, "## Last %d Log Lines\n\n", report.LogLineCount)
		fmt.Fprintf(w, "```\n%s\n```\n\n", strings.TrimRight(report.LogLines, "\n"))
	}

	// Recommendations
	if !report.NoRecommendations && len(report.Recommendations) > 0 {
		fmt.Fprintf(w, "## Recommendations\n\n")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(w, "- %s\n", rec)
		}
		fmt.Fprintln(w)
	}

	return nil
}

func formatDeadPodListMarkdown(w io.Writer, pods []k8s.DeadPodSummary, namespace, since string) error {
	fmt.Fprintf(w, "# Recently Dead Pods\n\n")
	fmt.Fprintf(w, "**Namespace:** `%s` | **Since:** %s\n\n", namespace, since)

	if len(pods) == 0 {
		fmt.Fprintln(w, "No dead pods found.")
		return nil
	}

	fmt.Fprintf(w, "| Pod | Cause | Death Time |\n")
	fmt.Fprintf(w, "|---|---|---|\n")
	for _, p := range pods {
		cause := p.Cause
		if cause == "" {
			cause = fmt.Sprintf("Error (%d)", p.ExitCode)
		}
		fmt.Fprintf(w, "| `%s` | %s | %s |\n", p.Name, cause, p.DeathTime.UTC().Format("15:04:05"))
	}
	fmt.Fprintln(w)
	return nil
}

func boolToStatus(active bool) string {
	if active {
		return "**true** ⚠️"
	}
	return "false"
}

func boolToReadyStatus(ready bool) string {
	if ready {
		return "true ✅"
	}
	return "**false** ⚠️"
}
