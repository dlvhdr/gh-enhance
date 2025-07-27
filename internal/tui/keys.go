package tui

import "github.com/charmbracelet/bubbles/v2/key"

var (
	openUrlKey = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	)

	openPRKey = key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "open PR"),
	)

	quitKey = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
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

	searchKey = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search in pane"),
	)

	cancelSearchKey = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel search"),
	)

	applySearchKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply search"),
	)

	nextSearchMatchKey = key.NewBinding(
		key.WithKeys("n", "ctrl+n"),
		key.WithHelp("ctrl+n", "next match"),
	)

	prevSearchMatchKey = key.NewBinding(
		key.WithKeys("N", "ctrl+p"),
		key.WithHelp("ctrl+p", "prev match"),
	)

	helpKey = key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	)
)
