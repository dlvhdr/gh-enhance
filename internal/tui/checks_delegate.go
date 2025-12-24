package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log/v2"

	"github.com/dlvhdr/gh-enhance/internal/data"
)

type checkItem struct {
	jobItem
}

// Title implements github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (ci *checkItem) Title() string {
	return ci.jobItem.Title()
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (ci *checkItem) Description() string {
	if ci.job.Event == "" {
		if ci.job.Workflow == "" {
			return "status check"
		}
		return ci.job.Workflow
	}

	return fmt.Sprintf("on: %s", ci.job.Event)
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (ci *checkItem) FilterValue() string { return ci.job.Name }

// checksDelegate implements list.ItemDelegate
type checksDelegate struct {
	commonDelegate
}

func (d *checksDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(*checkItem)
	if !ok {
		return
	}

	d.commonDelegate.Render(w, m, index, ri, &ri.meta)
}

// Height implements github.com/charmbracelet/bubbles.list.ItemDelegate.Height
func (d *checksDelegate) Height() int {
	return 2
}

// Spacing implements github.com/charmbracelet/bubbles.list.ItemDelegate.Spacing
func (d *checksDelegate) Spacing() int {
	return 1
}

// Update implements github.com/charmbracelet/bubbles.list.ItemDelegate.Update
func (d *checksDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	selected, ok := m.SelectedItem().(*checkItem)

	if !ok {
		return nil
	}

	selectedID := selected.job.Id
	for _, it := range m.VisibleItems() {
		ri := it.(*checkItem)
		ri.meta.focused = selectedID == ri.job.Id
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Info("key pressed on check", "key", msg.Text)
		switch {
		case key.Matches(msg, openUrlKey):
			return makeOpenUrlCmd(selected.job.Link)
		}
	}

	return nil
}

func newCheckItemDelegate(styles styles) list.ItemDelegate {
	d := checksDelegate{commonDelegate{styles: styles, focused: true}}
	return &d
}

func NewCheckItem(job data.WorkflowJob, styles styles) checkItem {
	return checkItem{
		jobItem: NewJobItem(job, styles),
	}
}
