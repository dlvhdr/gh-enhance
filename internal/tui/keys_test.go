package tui

import (
	"reflect"
	"testing"

	"github.com/dlvhdr/gh-enhance/internal/config"
)

func TestApplyKeybindingsRemapsBuiltinKey(t *testing.T) {
	resetKeybindings()
	t.Cleanup(resetKeybindings)

	err := ApplyKeybindings(config.Keybindings{
		Universal: []config.Keybinding{
			{
				Builtin: "openUrl",
				Key:     "enter",
				Name:    "open selected item",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertKeys(t, openUrlKey.Keys(), "enter")
	if got := openUrlKey.Help().Desc; got != "open selected item" {
		t.Fatalf("expected help description to be updated, got %q", got)
	}
}

func TestApplyKeybindingsResetsDefaults(t *testing.T) {
	resetKeybindings()
	t.Cleanup(resetKeybindings)

	err := ApplyKeybindings(config.Keybindings{
		Universal: []config.Keybinding{
			{Builtin: "openUrl", Key: "enter"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ApplyKeybindings(config.Keybindings{})
	if err != nil {
		t.Fatal(err)
	}

	assertKeys(t, openUrlKey.Keys(), "o")
}

func TestApplyKeybindingsRejectsUnknownBuiltin(t *testing.T) {
	resetKeybindings()
	t.Cleanup(resetKeybindings)

	err := ApplyKeybindings(config.Keybindings{
		Universal: []config.Keybinding{
			{Builtin: "notARealAction", Key: "x"},
		},
	})
	if err == nil {
		t.Fatal("expected unknown builtin error")
	}
}

func TestRemappedNavigationKeysUpdateComponents(t *testing.T) {
	resetKeybindings()
	t.Cleanup(resetKeybindings)

	err := ApplyKeybindings(config.Keybindings{
		Universal: []config.Keybinding{
			{Builtin: "nextRow", Key: "s"},
			{Builtin: "prevRow", Key: "w"},
			{Builtin: "gotoTop", Key: "t"},
			{Builtin: "gotoBottom", Key: "b"},
			{Builtin: "right", Key: "d"},
			{Builtin: "left", Key: "a"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	m := NewModel("dlvhdr/gh-enhance", "1", ModelOpts{})

	assertKeys(t, m.runsList.KeyMap.CursorDown.Keys(), "s")
	assertKeys(t, m.runsList.KeyMap.CursorUp.Keys(), "w")
	assertKeys(t, m.runsList.KeyMap.GoToStart.Keys(), "t")
	assertKeys(t, m.runsList.KeyMap.GoToEnd.Keys(), "b")
	assertKeys(t, m.logsViewport.KeyMap.Down.Keys(), "s")
	assertKeys(t, m.logsViewport.KeyMap.Up.Keys(), "w")
	assertKeys(t, m.logsViewport.KeyMap.Right.Keys(), "d")
	assertKeys(t, m.logsViewport.KeyMap.Left.Keys(), "a")
}

func assertKeys(t *testing.T, got []string, want ...string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected keys %v, got %v", want, got)
	}
}
