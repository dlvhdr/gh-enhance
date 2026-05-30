package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPathUsesXDGConfigHome(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(configDir, "gh-enhance", "config.yml")
	if got != want {
		t.Fatalf("expected config path %q, got %q", want, got)
	}
}

func TestLoadFileReturnsEmptyConfigWhenMissing(t *testing.T) {
	got, err := LoadFile(filepath.Join(t.TempDir(), "missing.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("expected empty config, got %+v", got)
	}
}

func TestLoadFileParsesConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(configPath, []byte(`
theme: dracula
flat: false
keybindings:
  universal:
    - builtin: openUrl
      key: enter
      name: open selected item
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	got, err := LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if got.Theme != "dracula" {
		t.Fatalf("expected theme dracula, got %q", got.Theme)
	}
	if got.Flat == nil || *got.Flat {
		t.Fatalf("expected flat to be explicitly false, got %+v", got.Flat)
	}

	wantKeybindings := []Keybinding{
		{
			Builtin: "openUrl",
			Key:     "enter",
			Name:    "open selected item",
		},
	}
	if !reflect.DeepEqual(got.Keybindings.Universal, wantKeybindings) {
		t.Fatalf("expected keybindings %+v, got %+v", wantKeybindings, got.Keybindings.Universal)
	}
}

func TestLoadFileReturnsParseError(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(configPath, []byte("theme: ["), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadFile(configPath)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
