package tui

import (
	"charm.land/bubbles/v2/key"
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
			zoomPaneKey,
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
			modeKey,
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
