package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dlvhdr/gh-enhance/internal/api"
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

func parseJobLogs(jobLogs string) []api.StepLogsWithTime {
	lines := strings.Lines(jobLogs)
	stepsLogs := make([]api.StepLogsWithTime, 0)

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

		expand := ExpandSymbol + " "
		log := strings.Join(dateAndLog[1:], "")
		if strings.Contains(line, stepStartMarker) {
			depth++
			log = strings.Replace(log, groupStartMarker, expand, 1)
			log = stepStartMarkerStyle.Render(log)
		} else if strings.Contains(line, groupStartMarker) {
			depth++
			log = strings.Replace(log, groupStartMarker, expand, 1)
			log = groupStartMarkerStyle.Render(log)
		} else if strings.Contains(log, groupEndMarker) {
			depth = max(0, depth-1)
			log = "\n"
		} else if strings.Contains(log, postJobCleanupMarker) {
			log = stepStartMarkerStyle.Render(log)
		} else if strings.Contains(log, commandMarker) {
			log = strings.Replace(log, commandMarker, "", 1)
			log = commandStyle.Render(log)
		} else {
			sep := ""
			if depth > 0 {
				sep = separatorStyle.Render(strings.Repeat(fmt.Sprintf("%s  ", Separator), depth))
			}
			log = sep + log
		}

		log = strings.TrimRight(log, "\n")
		stepsLogs = append(stepsLogs, api.StepLogsWithTime{Time: lineDate, Log: log})
	}

	return stepsLogs
}

func parseRunOutputMarkdown(output string, width int) (string, error) {
	renderer := markdown.GetMarkdownRenderer(width)
	return renderer.Render(output)
}
