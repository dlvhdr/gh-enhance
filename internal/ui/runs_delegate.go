package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"

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
		return i.bucket
		// return pendingGlyph.Render()
	}
}

func newRunItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(*runItem); ok {
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

func NewRunItem(run api.CheckRun) runItem {
	parts := strings.Split(run.Link, "/")

	jobs := make([]*jobItem, 0)
	for _, job := range run.Jobs {
		ji := NewJobItem(job)
		jobs = append(jobs, &ji)
	}

	return runItem{
		id:       parts[len(parts)-3],
		workflow: run.Workflow,
		jobs:     jobs,
		event:    run.Event,
		link:     run.Link,
		bucket:   run.Bucket,
	}
}
