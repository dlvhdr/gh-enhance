package tui

import (
	"io"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-enhance/internal/data"
)

type jobItem struct {
	meta               itemMeta
	job                *data.WorkflowJob
	logs               []data.LogsWithTime
	renderedLogs       string
	renderedText       string
	title              string
	initiatedLogsFetch bool
	loadingLogs        bool
	loadingSteps       bool
	steps              []*stepItem
	styles             styles
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *jobItem) Title() string {
	status := i.viewStatus()
	s := i.meta.TitleStyle()
	w := i.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Render(status), s.Render(" "),
		s.Width(w).Render(ansi.Truncate(s.Render(i.job.Name), w, Ellipsis)))
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *jobItem) Description() string {
	if i.job.Bucket == data.CheckBucketSkipping {
		return "Skipped"
	}
	if i.job.Bucket == data.CheckBucketPending {
		return "Pending"
	}

	if i.job.CompletedAt.IsZero() && !i.job.StartedAt.IsZero() {
		return "Running..."
	}

	// return i.job.CompletedAt.Sub(i.job.StartedAt).String()
	return string(i.job.Conclusion)
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *jobItem) FilterValue() string { return i.job.Name }

func (i *jobItem) viewStatus() string {
	s := i.meta.TitleStyle()
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

// jobsDelegate implements list.ItemDelegate
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
		log.Debug("key pressed on job", "key", msg.Text)
		switch msg.Text {
		case "o":
			return makeOpenUrlCmd(job.job.Link)
		}
	}

	return nil
}

func NewJobItem(job data.WorkflowJob, styles styles) jobItem {
	loadingSteps := true
	if job.Kind != data.JobKindGithubActions {
		loadingSteps = false
	}
	return jobItem{
		meta:         itemMeta{styles: styles},
		job:          &job,
		logs:         make([]data.LogsWithTime, 0),
		loadingLogs:  true,
		loadingSteps: loadingSteps,
		steps:        make([]*stepItem, 0),
	}
}
