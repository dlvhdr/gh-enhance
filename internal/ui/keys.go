package ui

import "github.com/charmbracelet/bubbles/v2/key"

var quitKeys = key.NewBinding(
	key.WithKeys("ctrl+c"),
	key.WithHelp("ctrl+c", "press q to quit"),
)

var nextRowKey = key.NewBinding(
	key.WithKeys("j", "down"),
	key.WithHelp("j/↓", "next row"),
)

var prevRowKey = key.NewBinding(
	key.WithKeys("k", "up"),
	key.WithHelp("k/↑", "previous row"),
)

var nextPaneKey = key.NewBinding(
	key.WithKeys("l"),
	key.WithHelp("l", "next pane"),
)

var prevPaneKey = key.NewBinding(
	key.WithKeys("h"),
	key.WithHelp("h", "previous pane"),
)
