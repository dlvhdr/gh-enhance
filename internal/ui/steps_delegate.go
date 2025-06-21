package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
)

type stepItem struct {
	title       string
	description string
	state       string
	conclusion  string
	startedAt   time.Time
	completedAt time.Time
}

func (i stepItem) Title() string { return fmt.Sprintf("%s %s", i.viewConclusion(), i.title) }

func (i stepItem) viewConclusion() string {
	if i.conclusion == "success" {
		return successGlyph.Render()
	}

	if i.conclusion == "failure" {
		return failureGlyph.Render()
	}

	return waitingGlyph.Render()
}

func (i stepItem) Description() string { return i.description }

func (i stepItem) FilterValue() string { return i.title }

func newStepItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(stepItem); ok {
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
