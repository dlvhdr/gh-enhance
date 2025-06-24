package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type stepItem struct {
	title       string
	description string
	state       string
	conclusion  string
	startedAt   time.Time
	completedAt time.Time
}

// Title implements /github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (i *stepItem) Title() string { return fmt.Sprintf("%s %s", i.viewConclusion(), i.title) }

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (i *stepItem) Description() string { return i.description }

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (i *stepItem) FilterValue() string { return i.title }

func (i *stepItem) viewConclusion() string {
	if i.conclusion == "success" {
		return successGlyph.Render()
	}

	if i.conclusion == "failure" {
		return failureGlyph.Render()
	}

	if i.state == "in_progress" {
		return waitingGlyph.Render()
	}

	if i.state == "pending" {
		return pendingGlyph.Render()
	}

	return i.state
}

func newStepItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(*stepItem); ok {
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

func NewStepItem(step api.Step) stepItem {
	return stepItem{
		title:       step.Name,
		description: step.StartedAt.String(),
		state:       step.Status,
		conclusion:  step.Conclusion,
		startedAt:   step.StartedAt,
		completedAt: step.CompletedAt,
	}
}
