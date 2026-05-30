package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"

	"github.com/dlvhdr/gh-enhance/internal/config"
)

func newOpenUrlKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	)
}

func newOpenPRKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "open PR"),
	)
}

func newQuitKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)
}

func newNextRowKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "next row"),
	)
}

func newPrevRowKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "previous row"),
	)
}

func newZoomPaneKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("z"),
		key.WithHelp("z", "zoom pane"),
	)
}

func newNextPaneKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "next pane"),
	)
}

func newPrevPaneKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "previous pane"),
	)
}

func newGotoTopKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "go to top"),
	)
}

func newGotoBottomKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("shift+g", "G"),
		key.WithHelp("G", "go to bottom"),
	)
}

func newRightKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "move right"),
	)
}

func newLeftKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "move left"),
	)
}

func newSearchKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search in pane"),
	)
}

func newModeKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "switch display mode"),
	)
}

func newCancelSearchKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel search"),
	)
}

func newApplySearchKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply search"),
	)
}

func newNextSearchMatchKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("n", "ctrl+n"),
		key.WithHelp("ctrl+n", "next match"),
	)
}

func newPrevSearchMatchKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("N", "ctrl+p"),
		key.WithHelp("ctrl+p", "prev match"),
	)
}

func newRefreshAllKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh all"),
	)
}

func newRerunKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "rerun"),
	)
}

func newHelpKey() key.Binding {
	return key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	)
}

var (
	openUrlKey         = newOpenUrlKey()
	openPRKey          = newOpenPRKey()
	quitKey            = newQuitKey()
	nextRowKey         = newNextRowKey()
	prevRowKey         = newPrevRowKey()
	zoomPaneKey        = newZoomPaneKey()
	nextPaneKey        = newNextPaneKey()
	prevPaneKey        = newPrevPaneKey()
	gotoTopKey         = newGotoTopKey()
	gotoBottomKey      = newGotoBottomKey()
	rightKey           = newRightKey()
	leftKey            = newLeftKey()
	searchKey          = newSearchKey()
	modeKey            = newModeKey()
	cancelSearchKey    = newCancelSearchKey()
	applySearchKey     = newApplySearchKey()
	nextSearchMatchKey = newNextSearchMatchKey()
	prevSearchMatchKey = newPrevSearchMatchKey()
	refreshAllKey      = newRefreshAllKey()
	rerunKey           = newRerunKey()
	helpKey            = newHelpKey()
)

func resetKeybindings() {
	openUrlKey = newOpenUrlKey()
	openPRKey = newOpenPRKey()
	quitKey = newQuitKey()
	nextRowKey = newNextRowKey()
	prevRowKey = newPrevRowKey()
	zoomPaneKey = newZoomPaneKey()
	nextPaneKey = newNextPaneKey()
	prevPaneKey = newPrevPaneKey()
	gotoTopKey = newGotoTopKey()
	gotoBottomKey = newGotoBottomKey()
	rightKey = newRightKey()
	leftKey = newLeftKey()
	searchKey = newSearchKey()
	modeKey = newModeKey()
	cancelSearchKey = newCancelSearchKey()
	applySearchKey = newApplySearchKey()
	nextSearchMatchKey = newNextSearchMatchKey()
	prevSearchMatchKey = newPrevSearchMatchKey()
	refreshAllKey = newRefreshAllKey()
	rerunKey = newRerunKey()
	helpKey = newHelpKey()
}

func ApplyKeybindings(keybindings config.Keybindings) error {
	resetKeybindings()

	for _, keybinding := range keybindings.Universal {
		if err := applyKeybinding(keybinding); err != nil {
			return err
		}
	}

	return nil
}

func applyKeybinding(keybinding config.Keybinding) error {
	if keybinding.Builtin == "" {
		return fmt.Errorf("keybinding builtin is required")
	}
	if keybinding.Key == "" {
		return fmt.Errorf("keybinding key is required for builtin %q", keybinding.Builtin)
	}

	binding, ok := builtinKeybinding(keybinding.Builtin)
	if !ok {
		return fmt.Errorf("unknown builtin keybinding: %s", keybinding.Builtin)
	}

	desc := binding.Help().Desc
	if keybinding.Name != "" {
		desc = keybinding.Name
	}

	binding.SetKeys(keybinding.Key)
	binding.SetHelp(keybinding.Key, desc)
	return nil
}

func builtinKeybinding(builtin string) (*key.Binding, bool) {
	switch builtin {
	case "openUrl", "openURL":
		return &openUrlKey, true
	case "openPR":
		return &openPRKey, true
	case "quit":
		return &quitKey, true
	case "nextRow":
		return &nextRowKey, true
	case "prevRow":
		return &prevRowKey, true
	case "zoomPane":
		return &zoomPaneKey, true
	case "nextPane":
		return &nextPaneKey, true
	case "prevPane":
		return &prevPaneKey, true
	case "gotoTop":
		return &gotoTopKey, true
	case "gotoBottom":
		return &gotoBottomKey, true
	case "right":
		return &rightKey, true
	case "left":
		return &leftKey, true
	case "search":
		return &searchKey, true
	case "mode", "switchMode":
		return &modeKey, true
	case "cancelSearch":
		return &cancelSearchKey, true
	case "applySearch":
		return &applySearchKey, true
	case "nextSearchMatch":
		return &nextSearchMatchKey, true
	case "prevSearchMatch":
		return &prevSearchMatchKey, true
	case "refreshAll":
		return &refreshAllKey, true
	case "rerun":
		return &rerunKey, true
	case "help":
		return &helpKey, true
	default:
		return nil, false
	}
}
