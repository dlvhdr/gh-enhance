package ui

import (
	"strings"
	"time"

	"github.com/dlvhdr/gh-enhance/internal/ui/markdown"
)

const (
	stepStartMarker      = "##[group]Run "
	groupStartMarker     = "##[group]"
	groupEndMarker       = "##[endgroup]"
	commandMarker        = "[command]"
	errorMarker          = "##[error]"
	postJobCleanupMarker = "Post job cleanup."
	completeJobMarker    = "Cleaning up orphan processes"
)

func parseJobLogs(jobLogs string) []LogsWithTime {
	lines := strings.Lines(jobLogs)
	stepsLogs := make([]LogsWithTime, 0)

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
		log := LogsWithTime{Time: lineDate}
		if strings.Contains(line, stepStartMarker) {
			depth++
			log.Kind = LogKindStepStart
		} else if strings.Contains(line, groupStartMarker) {
			depth++
			log.Kind = LogKindGroupStart
		} else if strings.Contains(text, groupEndMarker) {
			depth = max(0, depth-1)
			text = "\n"
			log.Kind = LogKindGroupEnd
		} else if strings.Contains(text, postJobCleanupMarker) {
			log.Kind = LogKindJobCleanup
		} else if strings.Contains(text, commandMarker) {
			log.Kind = LogKindCommand
		} else if strings.Contains(text, errorMarker) {
			log.Kind = LogKindError
		}

		log.Depth = depth
		log.Log = strings.TrimRight(text, "\n")
		stepsLogs = append(stepsLogs, log)
	}

	return stepsLogs
}

func parseRunOutputMarkdown(output string, width int) (string, error) {
	renderer := markdown.GetMarkdownRenderer(width)
	return renderer.Render(output)
}
