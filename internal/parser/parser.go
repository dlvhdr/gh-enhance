package parser

import (
	"strings"
	"time"

	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/ui/markdown"
)

const (
	StepStartMarker      = "##[group]Run "
	GroupStartMarker     = "##[group]"
	GroupEndMarker       = "##[endgroup]"
	CommandMarker        = "[command]"
	ErrorMarker          = "##[error]"
	PostJobCleanupMarker = "Post job cleanup."
	CompleteJobMarker    = "Cleaning up orphan processes"
)

func ParseJobLogs(jobLogs string) []data.LogsWithTime {
	lines := strings.Lines(jobLogs)
	stepsLogs := make([]data.LogsWithTime, 0)

	var lastTime time.Time
	var err error
	var name, step string
	count, depth := 0, 0

	for line := range lines {
		fields := strings.SplitN(line, string('\t'), 3)

		if count == 0 {
			name = fields[0]
			step = fields[1]
		}

		if name != "" && step != "" {
			line = strings.Replace(line, name+string('\t'), "", 1)
			line = strings.Replace(line, step+string('\t'), "", 1)
		}

		dateAndLog := strings.SplitN(fields[2], " ", 2)
		var lineDate time.Time
		if len(dateAndLog) == 2 {
			lineDate, err = time.Parse(time.RFC3339, dateAndLog[0])
			if err == nil {
				lastTime = lineDate
			} else {
				lineDate = lastTime
			}
		} else {
			lineDate = lastTime
		}

		text := strings.Join(dateAndLog[1:], "")
		log := data.LogsWithTime{Time: lineDate}
		if strings.Contains(line, StepStartMarker) {
			depth++
			log.Kind = data.LogKindStepStart
		} else if strings.Contains(line, GroupStartMarker) {
			depth++
			log.Kind = data.LogKindGroupStart
		} else if strings.Contains(text, GroupEndMarker) {
			depth = max(0, depth-1)
			text = "\n"
			log.Kind = data.LogKindGroupEnd
		} else if strings.Contains(text, PostJobCleanupMarker) {
			log.Kind = data.LogKindJobCleanup
		} else if strings.Contains(text, CommandMarker) {
			log.Kind = data.LogKindCommand
		} else if strings.Contains(text, ErrorMarker) {
			log.Kind = data.LogKindError
		}

		log.Depth = depth
		log.Log = strings.TrimRight(text, "\n")
		stepsLogs = append(stepsLogs, log)
	}

	return stepsLogs
}

func ParseRunOutputMarkdown(output string, width int) (string, error) {
	renderer := markdown.GetMarkdownRenderer(width)
	return renderer.Render(output)
}
