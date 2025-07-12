package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type delegateKeyMap struct {
	openInBrowser key.Binding
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		openInBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
	}
}

type itemMeta struct {
	focused  bool
	selected bool
	styles   styles
	width    int
}

func (i itemMeta) TitleStyle() lipgloss.Style {
	if i.selected && i.focused {
		return i.styles.paneItem.focusedSelectedTitleStyle
	} else if i.selected {
		return i.styles.paneItem.selectedTitleStyle
	}

	return i.styles.paneItem.unfocusedTitleStyle
}

func (i itemMeta) DescStyle() lipgloss.Style {
	if i.selected {
		w := i.width - i.styles.paneItem.selectedDescStyle.GetPaddingLeft() + 1
		return i.styles.paneItem.selectedDescStyle.Width(w).MaxHeight(1)
	}

	return i.styles.paneItem.descStyle
}

// commonDelegate partially implements list.ItemDelegate
type commonDelegate struct {
	focused bool
	styles  styles
}

func (d *commonDelegate) Render(w io.Writer, m list.Model, index int, item list.DefaultItem, meta *itemMeta) {
	isSelected := index == m.Index()
	meta.focused = d.focused
	meta.selected = isSelected
	meta.width = m.Width()

	var title, desc string

	title = item.Title()
	desc = item.Description()

	if m.Width() <= 0 {
		// short-circuit
		return
	}

	itemStyle := lipgloss.NewStyle().PaddingLeft(1)
	if d.focused && isSelected {
		itemStyle = meta.styles.paneItem.focusedSelectedStyle
	} else if isSelected {
		itemStyle = meta.styles.paneItem.selectedStyle
	}

	textwidth := m.Width() - itemStyle.GetBorderLeftSize() - itemStyle.GetPaddingLeft()
	ts := meta.TitleStyle()
	title = ts.Render(title)
	ds := meta.DescStyle()
	desc = ds.Render(ansi.Truncate(desc, textwidth-ds.GetPaddingLeft(), Ellipsis))

	// TODO: implement filtering styles

	fmt.Fprintf(w, "%s", itemStyle.Width(m.Width()).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, desc)))
}

// Height implements github.com/charmbracelet/bubbles.list.ItemDelegate.Height
func (d *commonDelegate) Height() int {
	return 2
}

// Spacing implements github.com/charmbracelet/bubbles.list.ItemDelegate.Spacing
func (d *commonDelegate) Spacing() int {
	return 1
}
