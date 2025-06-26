package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type jobItem struct {
	id           string
	title        string
	workflow     string
	logs         []api.StepLogsWithTime
	loadingLogs  bool
	loadingSteps bool
	state        api.StatusCheckConclusion
	steps        []*stepItem
	startedAt    time.Time
	completedAt  time.Time
	link         string
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
		job, ok := m.SelectedItem().(*jobItem)
		if !ok {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			log.Debug("key pressed on run", "key", msg.Text)
			switch msg.Text {
			case "o":
				return makeOpenUrlCmd(job.link)
			}
		}

		return nil
	}

	keys := newDelegateKeyMap()
	help := []key.Binding{keys.openInBrowser}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

func NewJobItem(job api.StatusCheck) jobItem {
	return jobItem{
		id:           job.Id,
		title:        job.Name,
		workflow:     job.Workflow,
		logs:         make([]api.StepLogsWithTime, 0),
		state:        job.State,
		loadingLogs:  true,
		loadingSteps: true,
		steps:        make([]*stepItem, 0),
		startedAt:    job.StartedAt,
		completedAt:  job.CompletedAt,
		link:         job.Link,
	}
}
