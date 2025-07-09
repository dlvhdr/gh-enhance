package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"
)

type jobItem struct {
	job                *WorkflowJob
	logs               []LogsWithTime
	renderedLogs       string
	renderedText       string
	title              string
	initiatedLogsFetch bool
	loadingLogs        bool
	loadingSteps       bool
	steps              []*stepItem
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *jobItem) Title() string { return fmt.Sprintf("%s %s", i.viewStatus(), i.job.Name) }

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *jobItem) Description() string {
	if i.job.Bucket == CheckBucketSkipping {
		return "Skipped"
	}

	if i.job.CompletedAt.IsZero() || i.job.StartedAt.IsZero() {
		return "Running..."
	}

	return i.job.CompletedAt.Sub(i.job.StartedAt).String()
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *jobItem) FilterValue() string { return i.job.Name }

func (i *jobItem) viewStatus() string {
	if i.job.Bucket == CheckBucketPass {
		return successGlyph.Render()
	}

	if i.job.Bucket == CheckBucketSkipping {
		return skippedGlyph.Render()
	}

	if i.job.Bucket == CheckBucketCancel {
		return canceledGlyph.Render()
	}

	if i.job.Bucket == CheckBucketFail {
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
				return makeOpenUrlCmd(job.job.Link)
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

func NewJobItem(job WorkflowJob) jobItem {
	loadingSteps := true
	if job.Kind != JobKindGithubActions {
		loadingSteps = false
	}
	return jobItem{
		job:          &job,
		logs:         make([]LogsWithTime, 0),
		loadingLogs:  true,
		loadingSteps: loadingSteps,
		steps:        make([]*stepItem, 0),
	}
}
