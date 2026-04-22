package formatter

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/NotHarshhaa/pod-why-dead/pkg/analyzer"
	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

var (
	// Lipgloss styles
	headerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).MarginTop(1).MarginBottom(1)
	causeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).MarginBottom(1)
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true)
	valueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	timeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	greenStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	redStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle      = lipgloss.NewStyle().Faint(true)
	sectionHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).MarginTop(1).MarginBottom(1)
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("59")).Padding(0, 1)
	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("61")).Padding(1)
	successIcon   = "✓"
	errorIcon     = "✗"
	warningIcon   = "⚠"
)

func renderStatus(active bool) string {
	if active {
		return warnStyle.Render("true ⚠")
	}
	return greenStyle.Render("false")
}

func renderReadyStatus(ready bool) string {
	if ready {
		return greenStyle.Render("true ✓")
	}
	return redStyle.Render("false ✗")
}

func formatText(w io.Writer, report *analyzer.Report) error {
	// Header
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		Background(lipgloss.Color("236")).
		Padding(0, 2).
		Width(60).
		Align(lipgloss.Center).
		Render("  ☠️  POD DEATH REPORT  ☠️  ")
	fmt.Fprintln(w, header)
	fmt.Fprintln(w)

	// Basic info box
	infoBox := fmt.Sprintf(
		"%s %s\n%s %s\n%s %s",
		labelStyle.Render("📍 Pod:"), valueStyle.Render(report.PodName),
		labelStyle.Render("🌐 Namespace:"), valueStyle.Render(report.Namespace),
		labelStyle.Render("🖥️  Node:"), valueStyle.Render(report.NodeName),
	)
	fmt.Fprintln(w, boxStyle.Render(infoBox))
	fmt.Fprintln(w)

	// Cause
	causeBox := fmt.Sprintf(
		"%s %s",
		labelStyle.Render("💀 Cause:"),
		causeStyle.Render(report.Cause),
	)
	fmt.Fprintln(w, borderStyle.Render(causeBox))
	if report.CauseDetail != "" {
		fmt.Fprintln(w, dimStyle.Render("  → "+report.CauseDetail))
	}
	if report.ExitCodeExplanation != "" {
		fmt.Fprintln(w, dimStyle.Render("  → "+report.ExitCodeExplanation))
	}
	fmt.Fprintln(w)

	// Container details
	if len(report.Containers) > 0 {
		fmt.Fprintln(w, sectionHeader.Render("  📦 CONTAINERS"))
		for _, c := range report.Containers {
			if c.Reason == "" && c.ExitCode == 0 && c.State == "running" {
				continue
			}

			containerTitle := fmt.Sprintf("  📦 %s", c.Name)
			if c.ExitCode != 0 {
				containerTitle += fmt.Sprintf(" (exit: %d)", c.ExitCode)
			}
			fmt.Fprintln(w, labelStyle.Render(containerTitle))

			containerInfo := fmt.Sprintf(
				"%s %s\n",
				labelStyle.Render("  Image:"), valueStyle.Render(c.Image),
			)
			if c.ImageDigest != "" {
				containerInfo += fmt.Sprintf("%s %s\n", labelStyle.Render("  Digest:"), dimStyle.Render(c.ImageDigest[:12]+"..."))
			}
			if c.MemoryLimit != "" {
				containerInfo += fmt.Sprintf("%s %s\n", labelStyle.Render("  Memory:"), valueStyle.Render(c.MemoryLimit))
			}
			if c.CPULimit != "" {
				containerInfo += fmt.Sprintf("%s %s\n", labelStyle.Render("  CPU:"), valueStyle.Render(c.CPULimit))
			}
			if c.KilledAt != "" {
				containerInfo += fmt.Sprintf("%s %s\n", labelStyle.Render("  Killed:"), timeStyle.Render(c.KilledAt))
			}
			if c.RestartCount > 0 {
				containerInfo += fmt.Sprintf("%s %s\n", labelStyle.Render("  Restarts:"), warnStyle.Render(strconv.Itoa(int(c.RestartCount))))
			}
			if c.ProbeFailure != "" {
				containerInfo += fmt.Sprintf("%s %s\n", labelStyle.Render("  Probe:"), warnStyle.Render(c.ProbeFailure))
			}
			fmt.Fprintln(w, borderStyle.Render(containerInfo))
			fmt.Fprintln(w)
		}
	}

	// Timeline
	if len(report.Timeline) > 0 {
		fmt.Fprintln(w, sectionHeader.Render("  ⏰ TIMELINE"))
		for _, t := range report.Timeline {
			icon := "•"
			if strings.Contains(t.Event, "Error") || strings.Contains(t.Event, "Failed") || strings.Contains(t.Event, "Killed") {
				icon = errorIcon
			} else if strings.Contains(t.Event, "Started") || strings.Contains(t.Event, "Scheduled") {
				icon = successIcon
			}
			fmt.Fprintf(w, "  %s %s %s\n", timeStyle.Render(t.Time), icon, valueStyle.Render(t.Event))
		}
		fmt.Fprintln(w)
	}

	// Node pressure
	if report.NodePressure != nil {
		fmt.Fprintln(w, sectionHeader.Render("  🖥️  NODE PRESSURE"))
		pressureInfo := fmt.Sprintf(
			"%s %s\n%s %s\n%s %s\n%s %s",
			labelStyle.Render("  Memory:"), renderStatus(report.NodePressure.MemoryPressure),
			labelStyle.Render("  Disk:"), renderStatus(report.NodePressure.DiskPressure),
			labelStyle.Render("  PID:"), renderStatus(report.NodePressure.PIDPressure),
			labelStyle.Render("  Ready:"), renderReadyStatus(report.NodePressure.NodeReady),
		)
		if report.NodePressure.MemoryAllocatable != "" {
			pressureInfo += fmt.Sprintf("\n%s %s", labelStyle.Render("  Allocatable:"), valueStyle.Render(report.NodePressure.MemoryAllocatable))
		}
		fmt.Fprintln(w, borderStyle.Render(pressureInfo))
		fmt.Fprintln(w)
	}

	// Node info
	if report.NodeInfo != nil {
		fmt.Fprintln(w, sectionHeader.Render("  🔧 NODE INFO"))
		nodeInfo := fmt.Sprintf(
			"%s %s\n%s %s\n%s %s\n%s %s",
			labelStyle.Render("  Kernel:"), valueStyle.Render(report.NodeInfo.KernelVersion),
			labelStyle.Render("  OS:"), valueStyle.Render(report.NodeInfo.OSImage),
			labelStyle.Render("  Runtime:"), valueStyle.Render(report.NodeInfo.ContainerRuntime),
			labelStyle.Render("  Kubelet:"), valueStyle.Render(report.NodeInfo.KubeletVersion),
		)
		if len(report.NodeInfo.Taints) > 0 {
			nodeInfo += fmt.Sprintf("\n%s\n", labelStyle.Render("  Taints:"))
			for _, taint := range report.NodeInfo.Taints {
				nodeInfo += fmt.Sprintf("    %s %s\n", warningIcon, dimStyle.Render(taint))
			}
		}
		fmt.Fprintln(w, borderStyle.Render(nodeInfo))
		fmt.Fprintln(w)
	}

	// Scheduling info
	if report.Scheduling != nil {
		if len(report.Scheduling.NodeSelector) > 0 || len(report.Scheduling.Tolerations) > 0 || len(report.Scheduling.AffinityRules) > 0 {
			fmt.Fprintln(w, sectionHeader.Render("  🎯 SCHEDULING"))
			if len(report.Scheduling.NodeSelector) > 0 {
				schedulingInfo := fmt.Sprintf("%s\n", labelStyle.Render("  Node Selector:"))
				for k, v := range report.Scheduling.NodeSelector {
					schedulingInfo += fmt.Sprintf("    • %s = %s\n", k, v)
				}
				fmt.Fprintln(w, borderStyle.Render(schedulingInfo))
			}
			if len(report.Scheduling.Tolerations) > 0 {
				schedulingInfo := fmt.Sprintf("%s\n", labelStyle.Render("  Tolerations:"))
				for _, tol := range report.Scheduling.Tolerations {
					schedulingInfo += fmt.Sprintf("    • %s\n", tol)
				}
				fmt.Fprintln(w, borderStyle.Render(schedulingInfo))
			}
			fmt.Fprintln(w)
		}
	}

	// PVCs
	if len(report.PVCs) > 0 {
		fmt.Fprintln(w, sectionHeader.Render("  💾 PERSISTENT VOLUMES"))
		for _, pvc := range report.PVCs {
			status := successIcon + " bound"
			if !pvc.Bound {
				status = errorIcon + " not bound"
			}
			pvcInfo := fmt.Sprintf("%s %s %s", labelStyle.Render("  • "+pvc.Name), status, valueStyle.Render(pvc.Capacity))
			fmt.Fprintln(w, pvcInfo)
		}
		fmt.Fprintln(w)
	}

	// Resource quota
	if report.ResourceQuota != nil {
		fmt.Fprintln(w, sectionHeader.Render("  📊 RESOURCE QUOTA"))
		quotaInfo := fmt.Sprintf(
			"%s %s\n%s %s\n%s %s",
			labelStyle.Render("  CPU:"), valueStyle.Render(fmt.Sprintf("%s / %s", report.ResourceQuota.HardCPU, report.ResourceQuota.UsedCPU)),
			labelStyle.Render("  Memory:"), valueStyle.Render(fmt.Sprintf("%s / %s", report.ResourceQuota.HardMemory, report.ResourceQuota.UsedMemory)),
			labelStyle.Render("  Pods:"), valueStyle.Render(fmt.Sprintf("%s / %s", report.ResourceQuota.HardPods, report.ResourceQuota.UsedPods)),
		)
		fmt.Fprintln(w, borderStyle.Render(quotaInfo))
		fmt.Fprintln(w)
	}

	// Referenced resources
	if len(report.ReferencedResources) > 0 {
		fmt.Fprintln(w, sectionHeader.Render("  🔑 REFERENCED RESOURCES"))
		for _, res := range report.ReferencedResources {
			status := successIcon + " exists"
			if !res.Exists {
				status = errorIcon + " not found"
			}
			resInfo := fmt.Sprintf("%s %s", labelStyle.Render("  • "+res.Kind+"/"+res.Name), status)
			fmt.Fprintln(w, resInfo)
		}
		fmt.Fprintln(w)
	}

	// Network policies
	if len(report.NetworkPolicies) > 0 {
		fmt.Fprintln(w, sectionHeader.Render("  🌐 NETWORK POLICIES"))
		for _, np := range report.NetworkPolicies {
			policyType := ""
			if np.Ingress {
				policyType += "ingress "
			}
			if np.Egress {
				policyType += "egress"
			}
			npInfo := fmt.Sprintf("%s %s", labelStyle.Render("  • "+np.Name), valueStyle.Render(strings.TrimSpace(policyType)))
			if len(np.PodSelector) > 0 {
				npInfo += fmt.Sprintf("\n    Selector: %s", strings.Join(np.PodSelector, ", "))
			}
			fmt.Fprintln(w, npInfo)
		}
		fmt.Fprintln(w)
	}

	// Namespace stats
	if len(report.NamespaceStats) > 0 {
		fmt.Fprintln(w, sectionHeader.Render("  📈 NAMESPACE STATS"))
		statsInfo := fmt.Sprintf(
			"%s %s\n%s %s\n%s %s\n%s %s\n%s %s",
			labelStyle.Render("  Total:"), valueStyle.Render(strconv.Itoa(int(report.NamespaceStats["total"]))),
			labelStyle.Render("  Running:"), greenStyle.Render(strconv.Itoa(int(report.NamespaceStats["running"]))),
			labelStyle.Render("  Pending:"), warnStyle.Render(strconv.Itoa(int(report.NamespaceStats["pending"]))),
			labelStyle.Render("  Failed:"), redStyle.Render(strconv.Itoa(int(report.NamespaceStats["failed"]))),
			labelStyle.Render("  Succeeded:"), valueStyle.Render(strconv.Itoa(int(report.NamespaceStats["succeeded"]))),
		)
		fmt.Fprintln(w, borderStyle.Render(statsInfo))
		fmt.Fprintln(w)
	}

	// Logs
	if report.LogLines != "" {
		fmt.Fprint(w, headerStyle.Render(fmt.Sprintf(" Last %d log lines (before death)\n", report.LogLineCount)))
		lines := strings.Split(strings.TrimRight(report.LogLines, "\n"), "\n")
		for _, line := range lines {
			fmt.Fprint(w, dimStyle.Render(fmt.Sprintf("  %s\n", line)))
		}
		fmt.Fprintln(w)
	}

	// Recommendations
	if !report.NoRecommendations && len(report.Recommendations) > 0 {
		fmt.Fprintln(w, headerStyle.Render(" Recommendation"))
		for _, rec := range report.Recommendations {
			fmt.Fprint(w, greenStyle.Render(fmt.Sprintf("  • %s\n", rec)))
		}
		fmt.Fprintln(w)
	}

	// Kubectl commands
	if len(report.KubectlCommands) > 0 {
		fmt.Fprintln(w, headerStyle.Render(" Suggested kubectl commands"))
		for _, cmd := range report.KubectlCommands {
			fmt.Fprint(w, valueStyle.Render(fmt.Sprintf("  $ %s\n", cmd)))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprint(w, dimStyle.Render(fmt.Sprintf("%s\n", strings.Repeat("─", 58))))
	return nil
}

func formatDeadPodListText(w io.Writer, pods []k8s.DeadPodSummary, namespace, since string) error {
	if len(pods) == 0 {
		fmt.Fprintf(w, "\n No dead pods found in namespace %q (last %s)\n\n", namespace, since)
		return nil
	}

	fmt.Fprintln(w)
	fmt.Fprint(w, headerStyle.Render(fmt.Sprintf(" Recently Dead Pods (last %s) — namespace: %s\n", since, namespace)))

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
			causeColor = redStyle
		} else if strings.Contains(cause, "CrashLoop") {
			causeColor = warnStyle
		}

		fmt.Fprintf(w, "  %s%s", name, padding)
		fmt.Fprint(w, causeColor.Render(fmt.Sprintf("%-20s", cause)))
		fmt.Fprint(w, timeStyle.Render(fmt.Sprintf(" %s\n", timeStr)))
	}
	fmt.Fprintln(w)
	return nil
}

