package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

// keyMap implements help.KeyMap
type keyMap struct{}

func (km keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			nextRowKey,
			prevRowKey,
			nextPaneKey,
			prevPaneKey,
			gotoTopKey,
			gotoBottomKey,
		},
		{
			searchKey,
			cancelSearchKey,
			applySearchKey,
			nextSearchMatchKey,
			prevSearchMatchKey,
		},
		{
			rerunKey,
			openUrlKey,
			openPRKey,
			refreshAllKey,
		},
		{
			quitKey,
			helpKey,
		},
	}
}

func (km keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		helpKey,
	}
}

var keys = keyMap{}
