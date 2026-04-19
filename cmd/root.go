package cmd

import (
	"fmt"
	"os"
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

  # Output as JSON
  pod-why-dead -n production my-pod --output json

  # Output as Markdown for incident reports
  pod-why-dead -n production my-pod --output markdown > incident.md

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
	deadPods, err := client.ListDeadPods(namespace, sinceDuration)
	if err != nil {
		return fmt.Errorf("failed to list dead pods: %w", err)
	}

	f := formatter.New(outputFormat)
	return f.FormatDeadPodList(os.Stdout, deadPods, namespace, since)
}

func runInspectMode(client *k8s.Client, podName string, sinceDuration time.Duration) error {
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
	if podInfo.NodeName != "" {
		nodeConditions, err = client.GetNodeConditions(podInfo.NodeName)
		if err != nil {
			nodeConditions = nil
		}
	}

	report := analyzer.Analyze(podInfo, events, logs, nodeConditions, logLines)
	report.NoRecommendations = noRecommendations

	f := formatter.New(outputFormat)
	return f.FormatReport(os.Stdout, report)
}
