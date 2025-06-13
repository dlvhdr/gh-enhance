package ui

import (
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
}

func (i checkItem) Title() string { return i.title }

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
