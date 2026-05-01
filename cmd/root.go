package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NotHarshhaa/pod-why-dead/pkg/analyzer"
	"github.com/NotHarshhaa/pod-why-dead/pkg/formatter"
	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

var (
	namespace         string
	kubeContext       string
	logLines          int
	outputFormat      string
	noRecommendations bool
	since             string
	listMode          bool
	namespaceAnalysis bool
	allNamespaces     bool
	filterCause       string
	exportFile        string
	verbose           bool
)

var rootCmd = &cobra.Command{
	Use:   "pod-why-dead [pod-name]",
	Short: "One command. Full death story of any Kubernetes pod.",
	Long: `pod-why-dead reconstructs the complete postmortem of a dead Kubernetes pod.
It gathers exit codes, previous logs, events, node conditions, and resource usage
to give you a structured death report in seconds.`,
	Example: `  # Inspect a specific dead pod
  pod-why-dead -n production my-api-7f9d4b-xkzp2

  # List all recently dead pods in a namespace
  pod-why-dead -n production --list --since 1h

  # List dead pods across all namespaces
  pod-why-dead --all-namespaces --list --since 1h

  # Filter by specific death cause
  pod-why-dead -n production --list --filter OOMKilled

  # Output as JSON
  pod-why-dead -n production my-pod --output json

  # Output as Markdown for incident reports
  pod-why-dead -n production my-pod --output markdown > incident.md

  # Export report to file
  pod-why-dead -n production my-pod --export report.json

  # Verbose mode for debugging
  pod-why-dead -n production my-pod --verbose

  # Use as kubectl plugin
  kubectl why-dead -n production my-pod-name`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          run,
}

func init() {
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	rootCmd.Flags().StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	rootCmd.Flags().IntVar(&logLines, "log-lines", 20, "Number of previous log lines to show")
	rootCmd.Flags().StringVar(&outputFormat, "output", "text", "Output format: text, json, markdown")
	rootCmd.Flags().BoolVar(&noRecommendations, "no-recommendations", false, "Skip the recommendations section")
	rootCmd.Flags().StringVar(&since, "since", "24h", "Look at pods that died within duration (e.g. 2h, 30m)")
	rootCmd.Flags().BoolVar(&listMode, "list", false, "List all recently dead pods in the namespace")
	rootCmd.Flags().BoolVar(&namespaceAnalysis, "namespace-analysis", false, "Include namespace-wide analysis in the report")
	rootCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List dead pods across all namespaces")
	rootCmd.Flags().StringVar(&filterCause, "filter", "", "Filter dead pods by cause (e.g., OOMKilled, CrashLoopBackOff)")
	rootCmd.Flags().StringVar(&exportFile, "export", "", "Export report to specified file")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output for debugging")
}

func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	sinceDuration, err := time.ParseDuration(since)
	if err != nil {
		return fmt.Errorf("invalid --since duration %q: %w", since, err)
	}

	client, err := k8s.NewClient(kubeContext)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Verbose mode enabled\n")
		fmt.Fprintf(os.Stderr, "Context: %s\n", kubeContext)
		fmt.Fprintf(os.Stderr, "Namespace: %s\n", namespace)
		fmt.Fprintf(os.Stderr, "All namespaces: %v\n", allNamespaces)
		fmt.Fprintf(os.Stderr, "Filter: %s\n", filterCause)
	}

	if listMode {
		return runListMode(client, sinceDuration)
	}

	if len(args) == 0 {
		return fmt.Errorf("pod name is required (or use --list to list dead pods)")
	}

	podName := args[0]
	return runInspectMode(client, podName, sinceDuration)
}

func runListMode(client *k8s.Client, sinceDuration time.Duration) error {
	var deadPods []k8s.DeadPodSummary
	var err error

	if allNamespaces {
		deadPods, err = client.ListDeadPodsAllNamespaces(sinceDuration)
		if err != nil {
			return fmt.Errorf("failed to list dead pods across all namespaces: %w", err)
		}
	} else {
		deadPods, err = client.ListDeadPods(namespace, sinceDuration)
		if err != nil {
			return fmt.Errorf("failed to list dead pods: %w", err)
		}
	}

	// Filter by cause if specified
	if filterCause != "" {
		var filtered []k8s.DeadPodSummary
		for _, pod := range deadPods {
			if strings.Contains(strings.ToLower(pod.Cause), strings.ToLower(filterCause)) {
				filtered = append(filtered, pod)
			}
		}
		deadPods = filtered
		if verbose {
			fmt.Fprintf(os.Stderr, "Filtered %d pods by cause '%s'\n", len(filtered), filterCause)
		}
	}

	var outputWriter io.Writer = os.Stdout
	if exportFile != "" {
		file, err := os.Create(exportFile)
		if err != nil {
			return fmt.Errorf("failed to create export file: %w", err)
		}
		defer file.Close()
		outputWriter = file
		if verbose {
			fmt.Fprintf(os.Stderr, "Exporting report to %s\n", exportFile)
		}
	}

	f := formatter.New(outputFormat)
	return f.FormatDeadPodList(outputWriter, deadPods, namespace, since)
}

func runInspectMode(client *k8s.Client, podName string, sinceDuration time.Duration) error {
	pod, err := client.GetPod(namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	podInfo, err := client.GetPodInfo(namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to get pod info: %w", err)
	}

	events, err := client.GetPodEvents(namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to get pod events: %w", err)
	}

	var logs string
	if podInfo.HasPreviousLogs {
		logs, err = client.GetPreviousLogs(namespace, podName, podInfo.DeadContainer, int64(logLines))
		if err != nil {
			logs = fmt.Sprintf("(failed to retrieve logs: %v)", err)
		}
	}

	var nodeConditions *k8s.NodeConditions
	var nodeInfo *k8s.NodeInfo
	if podInfo.NodeName != "" {
		nodeConditions, err = client.GetNodeConditions(podInfo.NodeName)
		if err != nil {
			nodeConditions = nil
		}
		nodeInfo, err = client.GetNodeInfo(podInfo.NodeName)
		if err != nil {
			nodeInfo = nil
		}
	}

	var pvcs []k8s.PVCInfo
	if len(podInfo.PVCNames) > 0 {
		pvcs, err = client.GetPVCInfo(namespace, podInfo.PVCNames)
		if err != nil {
			pvcs = nil
		}
	}

	quota, err := client.GetResourceQuota(namespace)
	if err != nil {
		quota = nil
	}

	// New features
	var referencedResources []k8s.ReferencedResource
	referencedResources, err = client.ValidateReferencedResources(namespace, pod)
	if err != nil {
		referencedResources = nil
	}

	var networkPolicies []k8s.NetworkPolicyInfo
	networkPolicies, err = client.CheckNetworkPolicies(namespace, pod.Labels)
	if err != nil {
		networkPolicies = nil
	}

	var namespaceStats map[string]int32
	if namespaceAnalysis {
		namespaceStats, err = client.GetNamespacePodStats(namespace)
		if err != nil {
			namespaceStats = nil
		}
	}

	report := analyzer.Analyze(podInfo, events, logs, nodeConditions, logLines, nodeInfo, pvcs, quota, referencedResources, networkPolicies, namespaceStats)
	report.NoRecommendations = noRecommendations

	var outputWriter io.Writer = os.Stdout
	if exportFile != "" {
		file, err := os.Create(exportFile)
		if err != nil {
			return fmt.Errorf("failed to create export file: %w", err)
		}
		defer file.Close()
		outputWriter = file
		if verbose {
			fmt.Fprintf(os.Stderr, "Exporting report to %s\n", exportFile)
		}
	}

	f := formatter.New(outputFormat)
	return f.FormatReport(outputWriter, report)
}
