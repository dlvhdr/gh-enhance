package tui

import (
	"encoding/json"
	"fmt"
	"os"

	"charm.land/log/v2"

	tint "github.com/lrstanley/bubbletint/v2"
)

// loadTintFile reads a bubbletint theme from a JSON file, in the format bubbletint's
// tints marshal to, for themes that are not in its registry.
func loadTintFile(path string) (*tint.Tint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading theme file: %w", err)
	}

	t := &tint.Tint{}
	if err := json.Unmarshal(data, t); err != nil {
		return nil, fmt.Errorf("parsing theme file %s: %w", path, err)
	}
	if t.Fg == nil || t.Bg == nil {
		return nil, fmt.Errorf("theme file %s needs at least fg and bg", path)
	}
	if t.ID == "" {
		t.ID = path
	}

	return t, nil
}

// setTheme selects ENHANCE_THEME's value as a built-in theme ID, or failing that as the
// path to a theme file.
func setTheme(theme string) {
	if theme == "" || tint.SetTintID(theme) {
		return
	}

	t, err := loadTintFile(theme)
	if err != nil {
		log.Error("ENHANCE_THEME is neither a known theme nor a readable theme file", "err", err)
		return
	}
	tint.SetTint(t)
}
