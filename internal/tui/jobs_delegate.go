package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log/v2"
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
	steps              []*stepItem
	spinner            spinner.Model
	styles             styles
}

// Title implements github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *jobItem) Title() string {
	status := i.viewStatus()
	s := i.meta.TitleStyle()
	w := i.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Render(status), s.Render(" "),
		s.Width(w).Render(ansi.Truncate(s.Render(i.job.Name), w, Ellipsis)))
}

// Description implements github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *jobItem) Description() string {
	if i.job.Bucket == data.CheckBucketSkipping {
		return "Skipped"
	}
	if i.job.Bucket == data.CheckBucketCancel {
		return "Cancelled"
	}
	if i.job.CompletedAt.IsZero() && !i.job.StartedAt.IsZero() {
		return fmt.Sprintf("Running for %v%s", utils.FormatTimeSince(i.job.StartedAt), Ellipsis)
	}
	if i.job.Bucket == data.CheckBucketPending {
		if i.job.State == api.StatusWaiting {
			return "Waiting"
		}

		return "Pending"
	}

	return i.job.CompletedAt.Sub(i.job.StartedAt).String()
}

// FilterValue implements github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *jobItem) FilterValue() string {
	return i.job.Name
}

func (i *jobItem) viewStatus() string {
	s := i.meta.TitleStyle()
	if i.job.CompletedAt.IsZero() && !i.job.StartedAt.IsZero() {
		return i.spinner.View()
	}
	switch i.job.Bucket {
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

// jobsDelegate implements github.com/charmbracelet/bubbles.list.ItemDelegate
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

// Update implements github.com/charmbracelet/bubbles.list.ItemDelegate.Update
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

func (ji *jobItem) Tick() tea.Cmd {
	if ji.isStatusInProgress() {
		return ji.spinner.Tick
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
		steps:        make([]*stepItem, 0),
		spinner:      NewClockSpinner(styles),
	}
}
