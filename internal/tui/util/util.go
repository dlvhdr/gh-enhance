package util

import tea "charm.land/bubbletea/v2"

type Model interface {
	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}
