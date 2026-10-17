package tui

import (
	"os"
	"path/filepath"
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"
)

func writeTheme(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing theme file: %v", err)
	}
	return path
}

func TestSetThemeKeepsBuiltInIDs(t *testing.T) {
	tint.NewDefaultRegistry()
	setTheme(tint.TintDracula.ID)
	if got := tint.Current().ID; got != tint.TintDracula.ID {
		t.Errorf("current theme = %q, want %q", got, tint.TintDracula.ID)
	}
}

func TestSetThemeLoadsAFile(t *testing.T) {
	tint.NewDefaultRegistry()
	path := writeTheme(
		t,
		`{"id": "mine", "fg": "#e0d3df", "bg": "#1c1220", "bright_blue": "#f08fb8"}`,
	)
	setTheme(path)
	current := tint.Current()
	if current.ID != "mine" {
		t.Fatalf("current theme = %q, want mine", current.ID)
	}
	if got := current.BrightBlue.Hex(); got != "#f08fb8" {
		t.Errorf("bright blue = %s, want #f08fb8", got)
	}
}

func TestLoadTintFileNamesAThemeByItsPath(t *testing.T) {
	path := writeTheme(t, `{"fg": "#e0d3df", "bg": "#1c1220"}`)
	theme, err := loadTintFile(path)
	if err != nil {
		t.Fatalf("loading theme: %v", err)
	}
	if theme.ID != path {
		t.Errorf("ID = %q, want the path %q", theme.ID, path)
	}
}

func TestLoadTintFileRejectsBadFiles(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      "fg: #e0d3df",
		"no background": `{"fg": "#e0d3df"}`,
	} {
		if _, err := loadTintFile(writeTheme(t, body)); err == nil {
			t.Errorf("%s: loaded without an error", name)
		}
	}
	if _, err := loadTintFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("a missing file loaded without an error")
	}
}

func TestSetThemeKeepsTheDefaultOnABadValue(t *testing.T) {
	tint.NewDefaultRegistry()
	tint.SetTintID(tint.TintTokyoNightStorm.ID)
	setTheme("no-such-theme")
	if got := tint.Current().ID; got != tint.TintTokyoNightStorm.ID {
		t.Errorf("current theme = %q, want the default %q", got, tint.TintTokyoNightStorm.ID)
	}
}
