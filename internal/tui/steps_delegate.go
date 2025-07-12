package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type stepItem struct {
	meta   itemMeta
	step   *api.Step
	jobUrl string
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *stepItem) Title() string {
	status := i.viewConclusion()
	s := i.meta.TitleStyle()
	w := i.meta.width - lipgloss.Width(status) - 2
	return lipgloss.JoinHorizontal(lipgloss.Top, s.Render(status), s.Render(" "),
		s.Width(w).Render(ansi.Truncate(s.Render(i.step.Name), w, Ellipsis)))
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *stepItem) Description() string {
	if i.step.CompletedAt.IsZero() || i.step.StartedAt.IsZero() {
		if i.step.Status == api.StatusInProgress {
			return "Running..."
		}
		return strings.ToTitle(string(i.step.Status))
	}
	return i.step.CompletedAt.Sub(i.step.StartedAt).String()
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *stepItem) FilterValue() string { return i.step.Name }

func (i *stepItem) viewConclusion() string {
	if i.step.Conclusion == api.ConclusionSuccess {
		return i.meta.styles.successGlyph.Render()
	}

	if api.IsFailureConclusion(i.step.Conclusion) {
		return i.meta.styles.failureGlyph.Render()
	}

	if i.step.Status == api.StatusInProgress {
		return i.meta.styles.waitingGlyph.Render()
	}

	if i.step.Status == api.StatusPending {
		return i.meta.styles.pendingGlyph.Render()
	}

	if i.step.Status == api.StatusCompleted {
		return i.meta.styles.successGlyph.Render()
	}

	return string(i.step.Status)
}

// stepsDelegate implements list.ItemDelegate
type stepsDelegate struct {
	commonDelegate
}

func newStepItemDelegate(styles styles) list.ItemDelegate {
	d := stepsDelegate{commonDelegate{styles: styles, focused: true}}
	return &d
}

// Update implements github.com/charmbracelet/bubbles.list.ItemDelegate.Update
func (d *stepsDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	step, ok := m.SelectedItem().(*stepItem)
	if !ok {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Debug("key pressed on step", "key", msg.Text)
		switch msg.Text {
		case "o":
			return makeOpenUrlCmd(step.Link())
		}
	}

	return nil
}

func (d *stepsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(*stepItem)
	if !ok {
		return
	}

	d.commonDelegate.Render(w, m, index, si, &si.meta)
}

func (si *stepItem) Link() string {
	return fmt.Sprintf("%s#step:%d:1", si.jobUrl, si.step.Number)
}

func NewStepItem(step api.Step, url string, styles styles) stepItem {
	return stepItem{
		meta:   itemMeta{styles: styles},
		jobUrl: url,
		step:   &step,
	}
}
