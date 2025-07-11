package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type stepItem struct {
	step   *api.Step
	jobUrl string
	styles styles
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *stepItem) Title() string { return fmt.Sprintf("%s %s", i.viewConclusion(), i.step.Name) }

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
		return i.styles.successGlyph.Render()
	}

	if api.IsFailureConclusion(i.step.Conclusion) {
		return i.styles.failureGlyph.Render()
	}

	if i.step.Status == api.StatusInProgress {
		return i.styles.waitingGlyph.Render()
	}

	if i.step.Status == api.StatusPending {
		return i.styles.pendingGlyph.Render()
	}

	if i.step.Status == api.StatusCompleted {
		return i.styles.successGlyph.Render()
	}

	return string(i.step.Status)
}

func newStepItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		step, ok := m.SelectedItem().(*stepItem)
		if !ok {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			log.Debug("key pressed on run", "key", msg.Text)
			switch msg.Text {
			case "o":
				return makeOpenUrlCmd(step.Link())
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

func (si *stepItem) Link() string {
	return fmt.Sprintf("%s#step:%d:1", si.jobUrl, si.step.Number)
}

func NewStepItem(step api.Step, url string, styles styles) stepItem {
	return stepItem{
		jobUrl: url,
		step:   &step,
		styles: styles,
	}
}
