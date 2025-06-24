package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type jobItem struct {
	id          string
	title       string
	description string
	workflow    string
	logs        []api.StepLogsWithTime
	loading     bool
	state       api.StatusCheckConclusion
	steps       []*stepItem
	startedAt   time.Time
	completedAt time.Time
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *jobItem) Title() string { return fmt.Sprintf("%s %s", i.viewStatus(), i.title) }

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *jobItem) Description() string {
	if i.state == api.StatusCheckConclusionSkipped {
		return "Skipped"
	}

	if i.completedAt.IsZero() || i.startedAt.IsZero() {
		return "Running..."
	}

	return i.completedAt.Sub(i.startedAt).String()
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *jobItem) FilterValue() string { return i.title }

func (i *jobItem) viewStatus() string {
	if i.state == api.StatusCheckConclusionSuccess {
		return successGlyph.Render()
	}

	if i.state == api.StatusCheckConclusionSkipped {
		return skippedGlyph.Render()
	}

	if i.state == api.StatusCheckConclusionCancelled {
		return canceledGlyph.Render()
	}

	if api.IsFailureStatusCheckState(i.state) {
		return failureGlyph.Render()
	}

	return waitingGlyph.Render()
}

func newCheckItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(*jobItem); ok {
		} else {
			return nil
		}

		return nil
	}

	help := []key.Binding{}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

func NewJobItem(job api.StatusCheck) jobItem {
	parts := strings.Split(job.Link, "/")
	id := parts[len(parts)-1]
	return jobItem{
		id:          id,
		title:       job.Name,
		description: id,
		workflow:    job.Workflow,
		logs:        make([]api.StepLogsWithTime, 0),
		state:       job.State,
		loading:     true,
		steps:       make([]*stepItem, 0),
		startedAt:   job.StartedAt,
		completedAt: job.CompletedAt,
	}
}
