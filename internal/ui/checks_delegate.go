package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
)

type checkItem struct {
	title       string
	description string
	workflow    string
	id          string
	logs        string
	loading     bool
	state       string
}

func (i checkItem) Title() string { return fmt.Sprintf("%s %s", i.viewStatus(), i.title) }

func (i checkItem) viewStatus() string {
	if i.state == "SUCCESS" {
		return successGlyph.Render()
	}

	if i.state == "PENDING" {
		return waitingGlyph.Render()
	}

	return failureGlyph.Render()
}

func (i checkItem) Description() string { return i.description }

func (i checkItem) FilterValue() string { return i.title }

func newCheckItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(checkItem); ok {
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
