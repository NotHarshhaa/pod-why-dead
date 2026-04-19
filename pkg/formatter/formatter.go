package formatter

import (
	"io"

	"github.com/NotHarshhaa/pod-why-dead/pkg/analyzer"
	"github.com/NotHarshhaa/pod-why-dead/pkg/k8s"
)

// Formatter handles output formatting.
type Formatter struct {
	format string
}

// New creates a Formatter for the given format type.
func New(format string) *Formatter {
	return &Formatter{format: format}
}

// FormatReport writes the death report in the configured format.
func (f *Formatter) FormatReport(w io.Writer, report *analyzer.Report) error {
	switch f.format {
	case "json":
		return formatJSON(w, report)
	case "markdown":
		return formatMarkdown(w, report)
	default:
		return formatText(w, report)
	}
}

// FormatDeadPodList writes the list of dead pods in the configured format.
func (f *Formatter) FormatDeadPodList(w io.Writer, pods []k8s.DeadPodSummary, namespace, since string) error {
	switch f.format {
	case "json":
		return formatDeadPodListJSON(w, pods, namespace, since)
	case "markdown":
		return formatDeadPodListMarkdown(w, pods, namespace, since)
	default:
		return formatDeadPodListText(w, pods, namespace, since)
	}
}
