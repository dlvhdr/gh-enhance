package ui

import "github.com/charmbracelet/bubbles/v2/key"

var quitKeys = key.NewBinding(
	key.WithKeys("ctrl+c"),
	key.WithHelp("ctrl+c", "press q to quit"),
)

var nextPaneKey = key.NewBinding(
	key.WithKeys("l"),
	key.WithHelp("l", "next pane"),
)

var prevPaneKey = key.NewBinding(
	key.WithKeys("h"),
	key.WithHelp("h", "previous pane"),
)
