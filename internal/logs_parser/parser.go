package logs_parser

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

const (
	stepStartMarker      = "##[group]Run "
	groupStartMarker     = "##[group]"
	groupEndMarker       = "##[endgroup]"
	commandMarker        = "[command]"
	postJobCleanupMarker = "Post job cleanup."
	completeJobMarker    = "Cleaning up orphan processes"
)

var (
	sep = lipgloss.NewStyle().Foreground(lipgloss.Color("234")).Render("")
)

type LogsGroup struct {
	Title string
	Logs  []string
}

func MarkStepsLogsByTime(jobId string, steps []api.Step, jobLogs string) []api.StepLogsWithTime {
	lines := strings.Lines(jobLogs)
	stepsLogs := make([]api.StepLogsWithTime, 0)

	var lastTime time.Time
	var err error
	count := 0
	var name, step string
	// groups := make([]LogsGroup, 0)

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

		expandSymbol := "▸ "
		log := strings.Join(dateAndLog[1:], "")
		if strings.Contains(line, stepStartMarker) {
			log = strings.Replace(log, groupStartMarker, expandSymbol, 1)
			log = lipgloss.NewStyle().Background(lipgloss.Color("8")).Inline(true).Underline(true).Render(log) + "\n"
		} else if strings.Contains(line, groupStartMarker) {
			log = strings.Replace(log, groupStartMarker, expandSymbol, 1)
			log = lipgloss.NewStyle().Inline(true).Underline(true).Render(log) + "\n"
		} else if strings.Contains(log, groupEndMarker) {
			log = lipgloss.NewStyle().Render("----------------------------") + "\n"
		} else if strings.Contains(log, postJobCleanupMarker) {
			log = lipgloss.NewStyle().Background(lipgloss.Color("8")).Inline(true).Underline(true).Render(log) + "\n"
		} else if strings.Contains(log, commandMarker) {
			log = strings.Replace(log, commandMarker, "", 1)
			log = lipgloss.NewStyle().Foreground(lipgloss.Green).Inline(true).Render(log) + "\n"
		}

		stepsLogs = append(stepsLogs, api.StepLogsWithTime{Time: lineDate, Log: log})
	}

	return stepsLogs
}
