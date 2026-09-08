package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type jobItem struct {
	meta               itemMeta
	job                *data.WorkflowJob
	logs               []data.LogsWithTime
	logsErr            error
	logsStderr         string
	renderedLogs       []string
	unstyledLogs       []string
	errorLine          int
	renderedText       string
	title              string
	initiatedLogsFetch bool
	loadingLogs        bool
	loadingSteps       bool
	stepsItems         []*stepItem
	styles             styles
}

// Title implements charm.land/bubbles.list.DefaultItem.Title
func (ji *jobItem) Title() string {
	status := ji.viewStatus()
	s := ji.meta.TitleStyle()
	w := ji.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		s.Render(status),
		s.Render(" "),
		s.Width(w).
			Render(ansi.Truncate(s.Render(ji.job.Name+fmt.Sprintf(" jid=%s wfrid=%s", ji.job.Id, ji.job.WorkflowRunId)), w, Ellipsis)),
	)
}

// Description implements charm.land/bubbles.list.DefaultItem.Description
func (ji *jobItem) Description() string {
	if ji.job.Bucket == data.CheckBucketActionRequired {
		return "Action required"
	}
	if ji.job.Bucket == data.CheckBucketSkipping {
		return "Skipped"
	}
	if ji.job.Bucket == data.CheckBucketCancel {
		return "Cancelled"
	}
	if ji.job.CompletedAt.IsZero() && !ji.job.StartedAt.IsZero() {
		return fmt.Sprintf("Running for %v%s", utils.FormatTimeSince(ji.job.StartedAt), Ellipsis)
	}
	if ji.job.Bucket == data.CheckBucketPending {
		if ji.job.State == api.StatusWaiting {
			return "Waiting"
		}

		return "Pending"
	}

	return ji.job.CompletedAt.Sub(ji.job.StartedAt).String()
}

// FilterValue implements charm.land/bubbles.list.Item.FilterValue
func (ji *jobItem) FilterValue() string {
	return ji.job.Name
}

func (ji *jobItem) viewStatus() string {
	s := ji.meta.TitleStyle()
	if ji.job.CompletedAt.IsZero() && !ji.job.StartedAt.IsZero() {
		return cachedSpinner.View()
	}
	return bucketToIcon(ji.job.Bucket, strings.ToLower(string(ji.job.State)), s, ji.meta.styles)
}

// jobsDelegate implements charm.land/bubbles.list.ItemDelegate
type jobsDelegate struct {
	commonDelegate
}

func newJobItemDelegate(styles styles) list.ItemDelegate {
	d := jobsDelegate{commonDelegate{styles: styles, focused: true}}
	return &d
}

func (d *jobsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ji, ok := item.(*jobItem)
	if !ok {
		return
	}

	d.commonDelegate.Render(w, m, index, ji, &ji.meta)
}

// Update implements charm.land/bubbles.list.ItemDelegate.Update
func (d *jobsDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	job, ok := m.SelectedItem().(*jobItem)
	if !ok {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Info("key pressed on job", "key", msg.Text)
		switch {
		case key.Matches(msg, openUrlKey):
			return makeOpenUrlCmd(job.job.Link)
		}
	}

	return nil
}

func (ji *jobItem) isStatusInProgress() bool {
	return ji.job.State == api.StatusInProgress || ji.job.State == api.StatusPending ||
		ji.job.State == api.StatusQueued || (ji.logsErr != nil &&
		strings.Contains(ji.logsStderr, "is still in progress;"))
}

func (ji *jobItem) hasInProgressSteps() bool {
	// if the job is in progress we must have in progress steps
	if ji.isStatusInProgress() {
		return true
	}

	// if the job isn't in progress but we have stale steps that are still showing
	// as in progress
	for _, si := range ji.stepsItems {
		if si.step.CompletedAt.IsZero() {
			return true
		}
	}
	return false
}

func (ji *jobItem) Tick() tea.Cmd {
	if ji.isStatusInProgress() {
		return cachedSpinner.Tick
	}

	return nil
}

func NewJobItem(job data.WorkflowJob, styles styles) jobItem {
	loadingSteps := job.Kind == data.JobKindGithubActions
	return jobItem{
		meta:         itemMeta{styles: styles},
		job:          &job,
		logs:         make([]data.LogsWithTime, 0),
		loadingLogs:  false,
		loadingSteps: loadingSteps,
		stepsItems:   make([]*stepItem, 0),
	}
}
