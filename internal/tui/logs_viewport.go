package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/robinovitch61/viewport/filterableviewport"
	"github.com/robinovitch61/viewport/viewport"
	"github.com/robinovitch61/viewport/viewport/item"
)

type logLine struct {
	item       item.Item
	line       int
	totalLines int
}

func newLogLine(log string, line int, totalLines int, s styles) logLine {
	return logLine{
		item: item.NewConcatWithPinned(
			1,
			item.NewItem(lipgloss.NewStyle().Foreground(s.colors.faintColor).Render(
				fmt.Sprintf(" %*d %s ", 5, line+1,
					lipgloss.NewStyle().Foreground(s.colors.fainterColor).Render("│")))),
			item.NewItem(log),
		),
	}
}

func (o logLine) GetItem() item.Item {
	return o.item
}

func newLogsViewport(s styles) *filterableviewport.Model[logLine] {
	vp := viewport.New(
		0,
		0,
		viewport.WithKeyMap[logLine](viewportKeyMap),
		viewport.WithSelectionEnabled[logLine](true),
		viewport.WithWrapText[logLine](true),
		viewport.WithStyles[logLine](
			viewport.Styles{
				SelectionPrefix: lipgloss.NewStyle().Foreground(lipgloss.Blue).Render("▐"),
				SelectedItemStyle: lipgloss.NewStyle().
					Background(s.colors.fainterColor).
					Foreground(lipgloss.BrightWhite),
				FooterStyle: lipgloss.NewStyle().
					BorderForeground(lipgloss.BrightBlack).
					Background(lipgloss.BrightBlack).
					Border(lipgloss.Border{Left: "", Right: ""}, false, true, false, true).
					Foreground(lipgloss.White).
					Italic(true),
			},
		),
	)

	filterableViewportKeyMap := filterableviewport.DefaultKeyMap()
	filterableViewportKeyMap.CancelFilterKey = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc/ctrl+c", "cancel filter"),
	)

	filterableViewportStyles := filterableviewport.DefaultStyles()
	filterableViewportStyles.Filter.Focused.TextInput.Text = lipgloss.NewStyle().
		Foreground(lipgloss.BrightWhite)
	filterableViewportStyles.Filter.Unfocused.TextInput.Text = lipgloss.NewStyle().
		Foreground(lipgloss.White)
	filterableViewportStyles.Filter.Empty = lipgloss.NewStyle().Foreground(lipgloss.White)
	filterableViewportStyles.MatchesCount.Matches = lipgloss.NewStyle().
		Foreground(lipgloss.BrightYellow)
	filterableViewportStyles.Match.Focused = lipgloss.NewStyle().
		Background(lipgloss.Darken(lipgloss.Yellow, 0.2)).
		Foreground(lipgloss.Lighten(lipgloss.Yellow, 0.5))
	filterableViewportStyles.Match.Unfocused = lipgloss.NewStyle().
		Background(lipgloss.Lighten(s.colors.fainterColor, 0.1)).
		Foreground(lipgloss.Lighten(lipgloss.BrightWhite, 0.5))

	return filterableviewport.New(
		vp,
		filterableviewport.WithKeyMap[logLine](filterableViewportKeyMap),
		filterableviewport.WithStyles[logLine](filterableViewportStyles),
		filterableviewport.WithPrefixText[logLine](
			lipgloss.NewStyle().Bold(true).Render("Filter:"),
		),
		filterableviewport.WithFilterModes[logLine](
			[]filterableviewport.FilterMode{
				filterableviewport.CaseInsensitiveFilterMode(
					key.NewBinding(
						key.WithKeys("/"),
						key.WithHelp("/", "insensitive fitler mode"),
					),
				), filterableviewport.RegexFilterMode(
					key.NewBinding(
						key.WithKeys("ctrl+r"),
						key.WithHelp("ctrl+r", "regex fitler mode"),
					),
				),
				filterableviewport.FuzzyFilterMode(
					key.NewBinding(
						key.WithKeys("ctrl+f"),
						key.WithHelp("ctrl+f", "fuzzy fitler mode"),
					),
				),
				filterableviewport.ExactFilterMode(key.NewBinding(
					key.WithKeys("ctrl+s"),
					key.WithHelp("ctrl+s", "case sensitive filter"),
				)),
			},
		),
		filterableviewport.WithPlaceholderText[logLine]("type to search…"),
		filterableviewport.WithItemDescriptor[logLine]("lines"),
		filterableviewport.WithEmptyText[logLine](
			" Search… "+lipgloss.NewStyle().
				Faint(true).
				Render("(/ insensitive ⋅ ⌃+s exact ⋅ ⌃+r regex ⋅ ⌃+f fuzzy)"),
		),
		filterableviewport.WithFilterLinePosition[logLine](filterableviewport.FilterLineTop),
		filterableviewport.WithMatchingItemsOnly[logLine](false),
		filterableviewport.WithCanToggleMatchingItemsOnly[logLine](true),
		filterableviewport.WithVerticalPad[logLine](8),
		filterableviewport.WithHorizontalPad[logLine](8),
	)
}

var viewportKeyMap = viewport.KeyMap{
	HalfPageDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "scroll half page down"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "scroll half page up"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k", "ctrl+y"),
		key.WithHelp("↑/k", "scroll up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j", "ctrl+e"),
		key.WithHelp("↓/j", "scroll down"),
	),
	Bottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "go to bottom"),
	),
	Top: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "go to top"),
	),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "scroll left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "scroll right"),
	),
}
