package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type runItem struct {
	run       *api.CheckRun
	jobsItems []*jobItem
	loading   bool
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *runItem) Title() string {
	status := i.viewWarnings()

	name := i.run.Workflow
	if name == "" && len(i.jobsItems) > 0 {
		name = i.jobsItems[0].job.Name
	}

	return fmt.Sprintf("%s %s", status, name)
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *runItem) Description() string {
	if i.run.Event == "" {
		return i.run.Link
	}

	return fmt.Sprintf("on: %s", i.run.Event)
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *runItem) FilterValue() string { return i.run.Name }

func (i *runItem) viewWarnings() string {
	switch i.run.Bucket {
	case "pass":
		return successGlyph.Render()
	case "fail":
		return failureGlyph.Render()
	case "skipping":
		return skippedGlyph.Render()
	case "cancel":
		return canceledGlyph.Render()
	default:
		return pendingGlyph.Render()
	}
}

func newRunItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		run, ok := m.SelectedItem().(*runItem)
		if !ok {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			log.Debug("key pressed on run", "key", msg.Text)
			switch msg.Text {
			case "o":
				return makeOpenUrlCmd(run.run.Link)
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

func NewRunItem(run api.CheckRun) runItem {
	jobs := make([]*jobItem, 0)
	for _, job := range run.Jobs {
		ji := NewJobItem(job)
		jobs = append(jobs, &ji)
	}

	return runItem{
		run:       &run,
		jobsItems: jobs,
		loading:   true,
	}
}
