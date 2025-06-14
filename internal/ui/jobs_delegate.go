package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
)

type jobItem struct {
	title       string
	description string
	workflow    string
	id          string
	logs        string
	loading     bool
	state       string
}

func (i jobItem) Title() string { return fmt.Sprintf("%s %s", i.viewStatus(), i.title) }

func (i jobItem) viewStatus() string {
	if i.state == "SUCCESS" {
		return successGlyph.Render()
	}

	if i.state == "PENDING" {
		return waitingGlyph.Render()
	}

	return failureGlyph.Render()
}

func (i jobItem) Description() string { return i.description }

func (i jobItem) FilterValue() string { return i.title }

func newCheckItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(jobItem); ok {
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
