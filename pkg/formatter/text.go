package formatter

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"

	"github.com/NotHarshhaa/pod-why-dead/pkg/analyzer"
	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

var (
	headerStyle = color.New(color.FgCyan, color.Bold)
	causeStyle  = color.New(color.FgRed, color.Bold)
	labelStyle  = color.New(color.FgWhite, color.Bold)
	valueStyle  = color.New(color.FgWhite)
	timeStyle   = color.New(color.FgYellow)
	warnStyle   = color.New(color.FgYellow, color.Bold)
	greenStyle  = color.New(color.FgGreen)
	dimStyle    = color.New(color.Faint)
	separator   = strings.Repeat("─", 58)
)

func formatText(w io.Writer, report *analyzer.Report) error {
	fmt.Fprintln(w)
	headerStyle.Fprintf(w, " Pod Death Report %s\n", separator)

	// Basic info
	printField(w, "  Pod       ", report.PodName)
	printField(w, "  Namespace ", report.Namespace)
	if report.NodeName != "" {
		printField(w, "  Node      ", report.NodeName)
	}
	fmt.Fprintln(w)

	// Cause
	causeStyle.Fprintf(w, " Cause: %s\n", report.Cause)
	if report.CauseDetail != "" {
		dimStyle.Fprintf(w, "  %s\n", report.CauseDetail)
	}
	if report.ExitCodeExplanation != "" {
		dimStyle.Fprintf(w, "  %s\n", report.ExitCodeExplanation)
	}
	fmt.Fprintln(w)

	// Container details
	for _, c := range report.Containers {
		if c.Reason == "" && c.ExitCode == 0 && c.State == "running" {
			continue
		}
		labelStyle.Fprintf(w, "  Container  %s", c.Name)
		if c.ExitCode != 0 {
			fmt.Fprintf(w, " (exit code %d)", c.ExitCode)
		}
		fmt.Fprintln(w)

		printField(w, "  Image          ", c.Image)
		if c.ImageDigest != "" {
			printField(w, "  Image digest   ", c.ImageDigest)
		}
		if c.MemoryLimit != "" {
			printField(w, "  Memory limit   ", c.MemoryLimit)
		}
		if c.MemoryRequest != "" {
			printField(w, "  Memory request ", c.MemoryRequest)
		}
		if c.CPULimit != "" {
			printField(w, "  CPU limit      ", c.CPULimit)
		}
		if c.CPURequest != "" {
			printField(w, "  CPU request    ", c.CPURequest)
		}
		if c.KilledAt != "" {
			printField(w, "  Killed at      ", c.KilledAt)
		}
		if c.RestartCount > 0 {
			printField(w, "  Restart count  ", strconv.Itoa(int(c.RestartCount)))
		}
		if c.ProbeFailure != "" {
			warnStyle.Fprintf(w, "  Probe failure  %s\n", c.ProbeFailure)
		}
		if c.Command != "" {
			printField(w, "  Command        ", c.Command)
		}
		fmt.Fprintln(w)
	}

	// Timeline
	if len(report.Timeline) > 0 {
		headerStyle.Fprintln(w, " Timeline")
		for _, t := range report.Timeline {
			timeStyle.Fprintf(w, "  %s  ", t.Time)
			valueStyle.Fprintln(w, t.Event)
		}
		fmt.Fprintln(w)
	}

	// Node pressure
	if report.NodePressure != nil {
		headerStyle.Fprintln(w, " Node Pressure at time of death")
		printCondition(w, "  Memory pressure ", report.NodePressure.MemoryPressure)
		printCondition(w, "  Disk pressure   ", report.NodePressure.DiskPressure)
		printCondition(w, "  PID pressure    ", report.NodePressure.PIDPressure)
		printNodeReady(w, "  Node ready      ", report.NodePressure.NodeReady)

		if report.NodePressure.MemoryAllocatable != "" {
			printField(w, "  Memory allocatable ", report.NodePressure.MemoryAllocatable)
		}
		if report.NodePressure.MemoryCapacity != "" {
			printField(w, "  Memory capacity    ", report.NodePressure.MemoryCapacity)
		}
		fmt.Fprintln(w)
	}

	// Node info
	if report.NodeInfo != nil {
		headerStyle.Fprintln(w, " Node Information")
		if report.NodeInfo.KernelVersion != "" {
			printField(w, "  Kernel version   ", report.NodeInfo.KernelVersion)
		}
		if report.NodeInfo.OSImage != "" {
			printField(w, "  OS image         ", report.NodeInfo.OSImage)
		}
		if report.NodeInfo.ContainerRuntime != "" {
			printField(w, "  Container runtime", report.NodeInfo.ContainerRuntime)
		}
		if report.NodeInfo.KubeletVersion != "" {
			printField(w, "  Kubelet version  ", report.NodeInfo.KubeletVersion)
		}
		if len(report.NodeInfo.Taints) > 0 {
			labelStyle.Fprintln(w, "  Taints           ")
			for _, taint := range report.NodeInfo.Taints {
				warnStyle.Fprintf(w, "    • %s\n", taint)
			}
		}
		fmt.Fprintln(w)
	}

	// Scheduling info
	if report.Scheduling != nil {
		if len(report.Scheduling.NodeSelector) > 0 || len(report.Scheduling.Tolerations) > 0 || len(report.Scheduling.AffinityRules) > 0 {
			headerStyle.Fprintln(w, " Scheduling Constraints")
			if len(report.Scheduling.NodeSelector) > 0 {
				labelStyle.Fprintln(w, "  Node selector    ")
				for k, v := range report.Scheduling.NodeSelector {
					valueStyle.Fprintf(w, "    • %s = %s\n", k, v)
				}
			}
			if len(report.Scheduling.Tolerations) > 0 {
				labelStyle.Fprintln(w, "  Tolerations       ")
				for _, tol := range report.Scheduling.Tolerations {
					valueStyle.Fprintf(w, "    • %s\n", tol)
				}
			}
			if len(report.Scheduling.AffinityRules) > 0 {
				labelStyle.Fprintln(w, "  Affinity rules    ")
				for _, rule := range report.Scheduling.AffinityRules {
					valueStyle.Fprintf(w, "    • %s\n", rule)
				}
			}
			fmt.Fprintln(w)
		}
	}

	// PVCs
	if len(report.PVCs) > 0 {
		headerStyle.Fprintln(w, " Persistent Volume Claims")
		for _, pvc := range report.PVCs {
			labelStyle.Fprintf(w, "  • %s ", pvc.Name)
			if !pvc.Bound {
				causeStyle.Fprintf(w, "(not bound) ")
			} else {
				greenStyle.Fprintf(w, "(bound) ")
			}
			if pvc.Capacity != "" {
				valueStyle.Fprintf(w, " %s", pvc.Capacity)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	// Resource quota
	if report.ResourceQuota != nil {
		headerStyle.Fprintln(w, " Resource Quota")
		printField(w, "  Name             ", report.ResourceQuota.Name)
		printField(w, "  CPU (hard/used)  ", fmt.Sprintf("%s / %s", report.ResourceQuota.HardCPU, report.ResourceQuota.UsedCPU))
		printField(w, "  Memory (hard/used)", fmt.Sprintf("%s / %s", report.ResourceQuota.HardMemory, report.ResourceQuota.UsedMemory))
		printField(w, "  Pods (hard/used)  ", fmt.Sprintf("%s / %s", report.ResourceQuota.HardPods, report.ResourceQuota.UsedPods))
		fmt.Fprintln(w)
	}

	// Referenced resources (ConfigMaps/Secrets)
	if len(report.ReferencedResources) > 0 {
		headerStyle.Fprintln(w, " Referenced Resources")
		for _, res := range report.ReferencedResources {
			labelStyle.Fprintf(w, "  • %s/%s ", res.Kind, res.Name)
			if res.Exists {
				greenStyle.Fprintf(w, "(exists)")
			} else {
				causeStyle.Fprintf(w, "(not found)")
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	// Network policies
	if len(report.NetworkPolicies) > 0 {
		headerStyle.Fprintln(w, " Network Policies")
		for _, np := range report.NetworkPolicies {
			labelStyle.Fprintf(w, "  • %s\n", np.Name)
			if len(np.PodSelector) > 0 {
				printField(w, "    Pod selector  ", strings.Join(np.PodSelector, ", "))
			}
			policyType := ""
			if np.Ingress {
				policyType += "ingress "
			}
			if np.Egress {
				policyType += "egress"
			}
			if policyType != "" {
				printField(w, "    Type          ", strings.TrimSpace(policyType))
			}
		}
		fmt.Fprintln(w)
	}

	// Namespace stats
	if len(report.NamespaceStats) > 0 {
		headerStyle.Fprintln(w, " Namespace Pod Statistics")
		printField(w, "  Total pods   ", strconv.Itoa(int(report.NamespaceStats["total"])))
		printField(w, "  Running      ", strconv.Itoa(int(report.NamespaceStats["running"])))
		printField(w, "  Pending      ", strconv.Itoa(int(report.NamespaceStats["pending"])))
		if report.NamespaceStats["failed"] > 0 {
			causeStyle.Fprintf(w, "  Failed       %d\n", report.NamespaceStats["failed"])
		} else {
			printField(w, "  Failed       ", "0")
		}
		printField(w, "  Succeeded    ", strconv.Itoa(int(report.NamespaceStats["succeeded"])))
		fmt.Fprintln(w)
	}

	// Logs
	if report.LogLines != "" {
		headerStyle.Fprintf(w, " Last %d log lines (before death)\n", report.LogLineCount)
		lines := strings.Split(strings.TrimRight(report.LogLines, "\n"), "\n")
		for _, line := range lines {
			dimStyle.Fprintf(w, "  %s\n", line)
		}
		fmt.Fprintln(w)
	}

	// Recommendations
	if !report.NoRecommendations && len(report.Recommendations) > 0 {
		headerStyle.Fprintln(w, " Recommendation")
		for _, rec := range report.Recommendations {
			greenStyle.Fprintf(w, "  • %s\n", rec)
		}
		fmt.Fprintln(w)
	}

	// Kubectl commands
	if len(report.KubectlCommands) > 0 {
		headerStyle.Fprintln(w, " Suggested kubectl commands")
		for _, cmd := range report.KubectlCommands {
			valueStyle.Fprintf(w, "  $ %s\n", cmd)
		}
		fmt.Fprintln(w)
	}

	dimStyle.Fprintf(w, "%s\n", strings.Repeat("─", 58))
	return nil
}

func formatDeadPodListText(w io.Writer, pods []k8s.DeadPodSummary, namespace, since string) error {
	if len(pods) == 0 {
		fmt.Fprintf(w, "\n No dead pods found in namespace %q (last %s)\n\n", namespace, since)
		return nil
	}

	fmt.Fprintln(w)
	headerStyle.Fprintf(w, " Recently Dead Pods (last %s) — namespace: %s\n", since, namespace)

	// Find max name length for alignment
	maxName := 0
	for _, p := range pods {
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
	}
	if maxName > 45 {
		maxName = 45
	}

	for _, p := range pods {
		name := p.Name
		if len(name) > 45 {
			name = name[:42] + "..."
		}
		cause := p.Cause
		if cause == "" {
			cause = fmt.Sprintf("Error (%d)", p.ExitCode)
		}

		padding := strings.Repeat(" ", maxName-len(name)+2)
		timeStr := p.DeathTime.UTC().Format("15:04:05")

		causeColor := causeStyle
		if strings.Contains(cause, "OOMKilled") {
			causeColor = color.New(color.FgRed, color.Bold)
		} else if strings.Contains(cause, "CrashLoop") {
			causeColor = color.New(color.FgYellow, color.Bold)
		}

		fmt.Fprintf(w, "  %s%s", name, padding)
		causeColor.Fprintf(w, "%-20s", cause)
		timeStyle.Fprintf(w, " %s\n", timeStr)
	}
	fmt.Fprintln(w)
	return nil
}

func printField(w io.Writer, label, value string) {
	labelStyle.Fprintf(w, "%s ", label)
	valueStyle.Fprintln(w, value)
}

func printCondition(w io.Writer, label string, active bool) {
	labelStyle.Fprintf(w, "%s ", label)
	if active {
		causeStyle.Fprintln(w, "true  ⚠")
	} else {
		greenStyle.Fprintln(w, "false")
	}
}

func printNodeReady(w io.Writer, label string, ready bool) {
	labelStyle.Fprintf(w, "%s ", label)
	if ready {
		greenStyle.Fprintln(w, "true")
	} else {
		causeStyle.Fprintln(w, "false  ⚠")
	}
}
