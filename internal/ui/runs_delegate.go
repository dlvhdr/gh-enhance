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
	run      *api.CheckRun
	id       string
	title    string
	workflow string
	event    string
	link     string
	bucket   string
	jobs     []*jobItem
	loading  bool
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *runItem) Title() string {
	status := i.viewWarnings()

	name := i.workflow
	if name == "" && len(i.jobs) > 0 {
		name = i.jobs[0].title
	}

	return fmt.Sprintf("%s %s", status, name)
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *runItem) Description() string {
	if i.event == "" {
		return i.link
	}

	return fmt.Sprintf("on: %s", i.event)
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *runItem) FilterValue() string { return i.title }

func (i *runItem) viewWarnings() string {
	switch i.bucket {
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
				return makeOpenUrlCmd(run.link)
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
		id:       run.Id,
		workflow: run.Workflow,
		jobs:     jobs,
		event:    run.Event,
		link:     run.Link,
		bucket:   run.Bucket,
		loading:  true,
	}
}
