package tui

import (
	"fmt"
	"io"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-enhance/internal/data"
)

type runItem struct {
	meta           itemMeta
	run            *data.WorkflowRun
	jobsItems      []*jobItem
	loadingJobs    bool
	lastFetchJobs  time.Time
	loadingSteps   bool
	lastFetchSteps time.Time
}

// Title implements /charm.land/bubbles.list.DefaultItem.Title
func (ri *runItem) Title() string {
	status := ri.viewStatus()
	s := ri.meta.TitleStyle()
	w := ri.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Render(status), s.Render(" "),
		s.Width(w).Render(ansi.Truncate(s.Render(ri.run.Name), w, Ellipsis)))
}

// Description implements /charm.land/bubbles.list.DefaultItem.Description
func (ri *runItem) Description() string {
	if ri.run.Event == "" {
		if ri.run.Workflow == "" {
			return "status check"
		}
		return ri.run.Workflow
	}

	startedAt := ""
	if !ri.run.StartedAt.IsZero() {
		if time.Since(ri.run.StartedAt) >= time.Hour*24 {
			startedAt = fmt.Sprintf(
				" at %s",
				ri.run.StartedAt.Local().Local().Format("Jan 02, 15:04 MST-07"),
			)
		} else {
			startedAt = fmt.Sprintf(
				" · %s ago",
				TimeElapsed(ri.run.StartedAt),
			)
		}
	}

	return fmt.Sprintf("on %s%s", ri.run.Event, startedAt)
}

// FilterValue implements /charm.land/bubbles.list.Item.FilterValue
func (ri *runItem) FilterValue() string { return ri.run.Name }

func (ri *runItem) IsInProgress() bool {
	return ri.run.Status == "in_progress"
}

func (ri *runItem) HasNotConcluded() bool {
	numPending := 0
	for _, ji := range ri.jobsItems {
		if ji.isStatusInProgress() {
			numPending++
		}
	}
	if numPending > 0 {
		return true
	}

	return ri.run.Conclusion == "action_required" ||
		ri.run.Status == "in_progress" ||
		ri.run.Status == "queued" ||
		ri.run.Status == "requested" ||
		ri.run.Status == "waiting" ||
		ri.run.Status == "pending"
}

func (ri *runItem) ShouldFetchJobs() bool {
	return !ri.loadingJobs &&
		(ri.lastFetchJobs.IsZero() || (time.Since(ri.lastFetchJobs) > refreshInterval && ri.HasNotConcluded()))
}

func (ri *runItem) viewStatus() string {
	s := ri.meta.TitleStyle()

	if ri.run.Status == "in_progress" {
		return cachedSpinner.View()
	}

	return bucketToIcon(ri.run.Bucket, ri.run.Status, s, ri.meta.styles)
}

func (ri *runItem) Tick() tea.Cmd {
	if ri.IsInProgress() {
		return cachedSpinner.Tick
	}

	return nil
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

// Height implements charm.land/bubbles.list.ItemDelegate.Height
func (d *runsDelegate) Height() int {
	return 2
}

// Spacing implements charm.land/bubbles.list.ItemDelegate.Spacing
func (d *runsDelegate) Spacing() int {
	return 1
}

// Update implements charm.land/bubbles.list.ItemDelegate.Update
func (d *runsDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	selected, ok := m.SelectedItem().(*runItem)

	if !ok {
		return nil
	}

	selectedID := selected.run.Id
	for _, it := range m.VisibleItems() {
		ri := it.(*runItem)
		ri.meta.focused = selectedID == ri.run.Id
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Info("key pressed on run", "key", msg.Text)
		switch {
		case key.Matches(msg, openUrlKey):
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
		meta:          itemMeta{styles: styles},
		run:           &run,
		jobsItems:     jobs,
		loadingSteps:  false,
		loadingJobs:   false,
		lastFetchJobs: time.Now(),
	}
}
