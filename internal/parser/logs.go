package parser

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"

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

func GroupStepsLogsByMarkers(jobId string, steps []api.Step, jobLogs string) []api.StepLogs {
	lines := strings.Lines(jobLogs)
	stepsLogs := make([][]string, 0)

	// append "Set up job" step
	stepsLogs = append(stepsLogs, make([]string, 0))

	var name, step string
	count, group, depth := 0, 0, 0
	lastTimestamp := ""
	for line := range lines {
		fields := strings.SplitN(line, string('\t'), 3)
		if len(fields) < 3 {
			stepsLogs[group] = append(stepsLogs[group], line)
			continue
		}

		if count == 0 {
			name = fields[0]
			step = fields[1]
		}

		if name != "" && step != "" {
			line = strings.Replace(line, name+string('\t'), "", 1)
			line = strings.Replace(line, step+string('\t'), "", 1)
		}

		hasCompletedAt := true
		var completedAt time.Time
		var err error
		if len(steps) <= group {
			hasCompletedAt = false
		} else {
			// completedAt, err = time.Parse(time.RFC3339, steps[group].CompletedAt)
			completedAt = steps[group].CompletedAt
			if err != nil {
				hasCompletedAt = false
			}
		}

		hasStartedAt := true
		var startedAt time.Time
		if len(steps) <= group {
			hasStartedAt = false
		} else {
			// startedAt, err = time.Parse(time.RFC3339, steps[group].StartedAt)
			startedAt = steps[group].StartedAt
			if err != nil {
				hasStartedAt = false
			}
		}

		var rounded time.Time
		dateAndLog := strings.SplitN(fields[2], " ", 2)
		if len(dateAndLog) == 2 {
			date, err := time.Parse(time.RFC3339, dateAndLog[0])
			if err == nil {
				rounded = date.Truncate(time.Second)
			}
		}

		if strings.Contains(line, stepStartMarker) && hasStartedAt && startedAt.After(rounded) {
			depth = 1
			stepsLogs = append(stepsLogs, make([]string, 0))
			group++
		} else if strings.Contains(line, postJobCleanupMarker) || strings.Contains(line, completeJobMarker) {
			stepsLogs = append(stepsLogs, make([]string, 0))
			group++
			depth = 1
		} else if strings.Contains(line, groupStartMarker) {
			depth++
		}

		if len(dateAndLog) == 2 {
			date, err := time.Parse(time.RFC3339, dateAndLog[0])
			rounded := date.Round(time.Second)

			if hasCompletedAt && rounded.After(completedAt) {
				log.Debug("date is after", "date", date, "completedAt", completedAt)
				group++
				depth = 1
				stepsLogs = append(stepsLogs, make([]string, 0))
			}

			formattedDate := strings.Repeat(" ", 8)
			log := dateAndLog[1]
			if err == nil {
				formattedDate = date.Format(time.RFC3339Nano)
				lastTimestamp = formattedDate
			} else {
				formattedDate = lastTimestamp
				log = strings.Join(dateAndLog, " ")
			}

			stepsLogs[group] = append(stepsLogs[group], strings.Join([]string{
				formattedDate,
				sep,
				log,
			}, " "))
		} else {
			stepsLogs[group] = append(stepsLogs[group], strings.Join([]string{lastTimestamp, sep, dateAndLog[0]}, " "))
		}
		log.Debug("collected line", "jobId", jobId, "group", group, "len(stepLogs)", len(stepsLogs[group]))

		if strings.Contains(line, groupEndMarker) {
			depth--
		}
	}

	res := make([]api.StepLogs, 0)
	for i, logs := range stepsLogs {
		log.Debug("success parsing step logs", "jobId", jobId, "step", i, "len(stepLogs)", len(logs))
		res = append(res, api.StepLogs(strings.Join(logs, "")))
	}

	log.Debug("success parsing job logs", "jobId", jobId, "len(stepsLogs)", len(res))
	return res
}

func GroupStepsLogsByTime(jobId string, steps []api.Step, jobLogs string) []api.StepLogs {
	lines := strings.Lines(jobLogs)
	stepsLogs := make([][]string, len(steps))
	for i := range steps {
		stepsLogs[i] = make([]string, 0)
	}

	var lastTime time.Time
	for line := range lines {

		fields := strings.SplitN(line, string('\t'), 3)

		dateAndLog := strings.SplitN(fields[2], " ", 2)
		var err error
		var lineDate time.Time
		if len(dateAndLog) == 2 {
			lineDate, err = time.Parse(time.RFC3339, dateAndLog[0])
			if err == nil {
				lineDate = lineDate.Truncate(time.Second)
				lastTime = lineDate
			}
		} else {
			lineDate = lastTime
		}

		for i, step := range steps {
			// log.Debug("line", "lineDate", lineDate, "step.startedAt", step.StartedAt, "step.completedAt", step.CompletedAt, "lineDate", lineDate, "line", line)
			if (lineDate.Equal(step.StartedAt) || lineDate.After(step.StartedAt)) && (lineDate.Equal(step.CompletedAt) || lineDate.Before(step.CompletedAt)) {
				stepsLogs[i] = append(stepsLogs[i], line)
			} else if strings.Contains(step.Name, "Complete job") {
				log.Debug("bad line :(", "line", line)
				log.Debug("bad line :(", "step.startedAt", step.StartedAt, "lineDate", lineDate, "startedAt equal?", lineDate.Equal(step.StartedAt), "startedAt after?", lineDate.After(step.StartedAt))
			}
		}
	}

	res := make([]api.StepLogs, 0)
	for i, logs := range stepsLogs {
		log.Debug("success parsing step logs", "jobId", jobId, "step", i, "len(stepLogs)", len(logs))
		res = append(res, api.StepLogs(strings.Join(logs, "")))
	}

	log.Debug("success parsing job logs", "jobId", jobId, "len(stepsLogs)", len(res))
	return res
}

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
