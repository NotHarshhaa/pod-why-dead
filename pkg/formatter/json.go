package formatter

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/NotHarshhaa/pod-why-dead/pkg/analyzer"
	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

func formatJSON(w io.Writer, report *analyzer.Report) error {
	output := report
	if report.NoRecommendations {
		output.Recommendations = nil
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

type deadPodListJSON struct {
	Namespace string               `json:"namespace"`
	Since     string               `json:"since"`
	Pods      []deadPodSummaryJSON `json:"pods"`
}

type deadPodSummaryJSON struct {
	Name      string `json:"name"`
	Cause     string `json:"cause"`
	DeathTime string `json:"death_time"`
	ExitCode  int32  `json:"exit_code,omitempty"`
}

func formatDeadPodListJSON(w io.Writer, pods []k8s.DeadPodSummary, namespace, since string) error {
	output := deadPodListJSON{
		Namespace: namespace,
		Since:     since,
		Pods:      make([]deadPodSummaryJSON, 0, len(pods)),
	}
	for _, p := range pods {
		output.Pods = append(output.Pods, deadPodSummaryJSON{
			Name:      p.Name,
			Cause:     p.Cause,
			DeathTime: p.DeathTime.UTC().Format("2006-01-02T15:04:05Z"),
			ExitCode:  p.ExitCode,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
