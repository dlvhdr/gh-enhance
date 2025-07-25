package tui

import "github.com/charmbracelet/bubbles/v2/key"

var (
	openPR = key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "open PR"),
	)

	quitKeys = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "press q to quit"),
	)

	nextRowKey = key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "next row"),
	)

	prevRowKey = key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "previous row"),
	)

	nextPaneKey = key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "next pane"),
	)

	prevPaneKey = key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "previous pane"),
	)

	gotoTopKey = key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "go to top"),
	)

	gotoBottomKey = key.NewBinding(
		key.WithKeys("shift+g", "G"),
		key.WithHelp("G", "go to bottom"),
	)

	rightKey = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "move right"),
	)

	leftKey = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "move left"),
	)

	searchLogs = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search logs"),
	)
)
