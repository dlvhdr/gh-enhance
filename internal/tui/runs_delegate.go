package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-enhance/internal/data"
)

type runItem struct {
	meta      itemMeta
	run       *data.WorkflowRun
	jobsItems []*jobItem
	loading   bool
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *runItem) Title() string {
	status := i.viewStatus()
	s := i.meta.TitleStyle()
	w := i.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Render(status), s.Render(" "),
		s.Width(w).Render(ansi.Truncate(s.Render(i.run.Name), w, Ellipsis)))
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *runItem) Description() string {
	if i.run.Event == "" {
		return i.run.Workflow
	}

	return fmt.Sprintf("on: %s", i.run.Event)
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *runItem) FilterValue() string { return i.run.Name }

func (i *runItem) viewStatus() string {
	s := i.meta.TitleStyle()
	switch i.run.Bucket {
	case data.CheckBucketPass:
		return i.meta.styles.successGlyph.Inherit(s).Render()
	case data.CheckBucketFail:
		return i.meta.styles.failureGlyph.Inherit(s).Render()
	case data.CheckBucketSkipping:
		return i.meta.styles.skippedGlyph.Inherit(s).Render()
	case data.CheckBucketCancel:
		return i.meta.styles.canceledGlyph.Inherit(s).Render()
	default:
		return i.meta.styles.pendingGlyph.Inherit(s).Render()
	}
}

// runsDelegate implements list.ItemDelegate
type runsDelegate struct {
	commonDelegate
}

func (d *runsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(*runItem)
	if !ok {
		return
	}

	d.commonDelegate.Render(w, m, index, ri, &ri.meta)
}

// Height implements github.com/charmbracelet/bubbles.list.ItemDelegate.Height
func (d *runsDelegate) Height() int {
	return 2
}

// Spacing implements github.com/charmbracelet/bubbles.list.ItemDelegate.Spacing
func (d *runsDelegate) Spacing() int {
	return 1
}

// Update implements github.com/charmbracelet/bubbles.list.ItemDelegate.Update
func (d *runsDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	selected, ok := m.SelectedItem().(*runItem)

	if !ok {
		return nil
	}

	selectedID := selected.run.Id
	for _, it := range m.Items() {
		ri := it.(*runItem)
		ri.meta.focused = selectedID == ri.run.Id
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Debug("key pressed on run", "key", msg.Text)
		switch msg.Text {
		case "o":
			return makeOpenUrlCmd(selected.run.Link)
		}
	}

	return nil
}

func newRunItemDelegate(styles styles) list.ItemDelegate {
	d := runsDelegate{commonDelegate{styles: styles, focused: true}}
	return &d
}

func NewRunItem(run data.WorkflowRun, styles styles) runItem {
	jobs := make([]*jobItem, 0)
	for _, job := range run.Jobs {
		ji := NewJobItem(job, styles)
		jobs = append(jobs, &ji)
	}

	return runItem{
		meta:      itemMeta{styles: styles},
		run:       &run,
		jobsItems: jobs,
		loading:   true,
	}
}
