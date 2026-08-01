package tui

import (
	"fmt"
	"io"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
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
	spinner        spinner.Model
}

// Title implements /charm.land/bubbles.list.DefaultItem.Title
func (i *runItem) Title() string {
	status := i.viewStatus()
	s := i.meta.TitleStyle()
	w := i.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Render(status), s.Render(" "),
		s.Width(w).Render(ansi.Truncate(s.Render(i.run.Name), w, Ellipsis)))
}

// Description implements /charm.land/bubbles.list.DefaultItem.Description
func (i *runItem) Description() string {
	if i.run.Event == "" {
		if i.run.Workflow == "" {
			return "status check"
		}
		return i.run.Workflow
	}

	startedAt := ""
	if !i.run.StartedAt.IsZero() {
		if time.Since(i.run.StartedAt) >= time.Hour*24 {
			startedAt = fmt.Sprintf(
				" at %s",
				i.run.StartedAt.Local().Local().Format("Jan 02, 15:04 MST-07"),
			)
		} else {
			startedAt = fmt.Sprintf(
				" · %s ago",
				TimeElapsed(i.run.StartedAt),
			)
		}
	}

	return fmt.Sprintf("on %s%s", i.run.Event, startedAt)
}

// FilterValue implements /charm.land/bubbles.list.Item.FilterValue
func (i *runItem) FilterValue() string { return i.run.Name }

func (i *runItem) IsInProgress() bool {
	return i.run.Status == "in_progress"
}

func (i *runItem) HasNotConcluded() bool {
	numPending := 0
	for _, ji := range i.jobsItems {
		if ji.isStatusInProgress() {
			numPending++
		}
	}
	if numPending > 0 {
		return true
	}

	return i.run.Conclusion == "action_required" ||
		i.run.Status == "in_progress" ||
		i.run.Status == "queued" ||
		i.run.Status == "requested" ||
		i.run.Status == "waiting" ||
		i.run.Status == "pending"
}

func (i *runItem) ShouldFetchJobs() bool {
	return !i.loadingJobs &&
		(i.lastFetchJobs.IsZero() || (time.Since(i.lastFetchJobs) > refreshInterval && i.HasNotConcluded()))
}

func (i *runItem) viewStatus() string {
	s := i.meta.TitleStyle()

	if i.run.Status == "in_progress" {
		return i.spinner.View()
	}

	return bucketToIcon(i.run.Bucket, s, i.meta.styles)
}

func (ri *runItem) Tick() tea.Cmd {
	if ri.IsInProgress() {
		return ri.spinner.Tick
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
		meta:         itemMeta{styles: styles},
		run:          &run,
		jobsItems:    jobs,
		loadingSteps: false,
		loadingJobs:  false,
		spinner:      NewClockSpinner(styles),
	}
}
