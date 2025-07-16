package tui

import (
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

type paneItemStyles struct {
	focusedTitleStyle   lipgloss.Style
	unfocusedTitleStyle lipgloss.Style

	selectedDescStyle lipgloss.Style
	descStyle         lipgloss.Style

	selectedStyle             lipgloss.Style
	selectedTitleStyle        lipgloss.Style
	focusedSelectedTitleStyle lipgloss.Style

	focusedSelectedStyle lipgloss.Style
}

type styles struct {
	defaultListStyles          lipgloss.Style
	focusedPaneTitleStyle      lipgloss.Style
	unfocusedPaneTitleStyle    lipgloss.Style
	focusedPaneTitleBarStyle   lipgloss.Style
	unfocusedPaneTitleBarStyle lipgloss.Style
	normalItemDescStyle        lipgloss.Style

	paneItem paneItemStyles

	paneStyle                  lipgloss.Style
	lineNumbersStyle           lipgloss.Style
	canceledGlyph              lipgloss.Style
	skippedGlyph               lipgloss.Style
	waitingGlyph               lipgloss.Style
	pendingGlyph               lipgloss.Style
	failureGlyph               lipgloss.Style
	successGlyph               lipgloss.Style
	noLogsStyle                lipgloss.Style
	watermarkIllustrationStyle lipgloss.Style
	debugStyle                 lipgloss.Style
	errorBgStyle               lipgloss.Style
	errorStyle                 lipgloss.Style
	errorTitleStyle            lipgloss.Style
	separatorStyle             lipgloss.Style
	commandStyle               lipgloss.Style
	stepStartMarkerStyle       lipgloss.Style
	groupStartMarkerStyle      lipgloss.Style
	scrollbarStyle             lipgloss.Style
	scrollbarThumbStyle        lipgloss.Style
	scrollbarTrackStyle        lipgloss.Style
}

func makeStyles() styles {
	t := tint.Current()

	defaultItemStyles := list.NewDefaultItemStyles(true)
	focusedColor := t.Blue
	unfocusedColor := tint.Darken(t.BrightBlue, 70)
	faintColor := tint.Darken(focusedColor, 50)
	fainterColor := tint.Darken(focusedColor, 80)

	errorBgStyle := lipgloss.NewStyle().Background(lipgloss.Color("#1C0D0F"))
	bg := tint.Lighten(t.Bg, 10)

	baseTitleStyle := lipgloss.NewStyle().Bold(true).PaddingLeft(1).PaddingRight(1).Margin(0)

	return styles{
		focusedPaneTitleStyle:      baseTitleStyle.Foreground(t.Black).Background(focusedColor),
		unfocusedPaneTitleStyle:    baseTitleStyle.Background(unfocusedColor).Foreground(t.Fg),
		focusedPaneTitleBarStyle:   lipgloss.NewStyle().Bold(true).PaddingRight(0).MarginBottom(1),
		unfocusedPaneTitleBarStyle: lipgloss.NewStyle().Bold(true).Faint(true).PaddingRight(0).MarginBottom(1),

		normalItemDescStyle: defaultItemStyles.DimmedDesc.PaddingLeft(4),

		paneItem: paneItemStyles{
			selectedStyle: lipgloss.NewStyle().
				Background(bg).
				BorderBackground(bg).
				Border(lipgloss.OuterHalfBlockBorder(), false, false, false, true).
				BorderForeground(t.BrightWhite),

			focusedSelectedStyle: lipgloss.NewStyle().
				Background(bg).
				BorderForeground(focusedColor).
				BorderBackground(bg).
				Border(lipgloss.OuterHalfBlockBorder(), false, false, false, true),

			selectedTitleStyle: lipgloss.NewStyle().
				Bold(true).
				Foreground(t.BrightWhite).
				Background(bg),

			focusedTitleStyle:         lipgloss.NewStyle().Bold(true).Foreground(focusedColor),
			focusedSelectedTitleStyle: lipgloss.NewStyle().Bold(true).Foreground(focusedColor).Background(bg),

			unfocusedTitleStyle: lipgloss.NewStyle().Bold(true),

			selectedDescStyle: lipgloss.NewStyle().Foreground(t.White).PaddingLeft(2).Background(bg),
			descStyle:         lipgloss.NewStyle().Foreground(faintColor).PaddingLeft(2),
		},

		paneStyle: lipgloss.NewStyle().BorderRight(true).BorderStyle(
			lipgloss.NormalBorder()).BorderForeground(fainterColor),
		lineNumbersStyle:           lipgloss.NewStyle().Foreground(fainterColor).Align(lipgloss.Right),
		canceledGlyph:              lipgloss.NewStyle().Foreground(faintColor).SetString(CanceledIcon),
		skippedGlyph:               lipgloss.NewStyle().Foreground(faintColor).SetString(SkippedIcon),
		waitingGlyph:               lipgloss.NewStyle().Foreground(lipgloss.Yellow).SetString(WaitingIcon),
		pendingGlyph:               lipgloss.NewStyle().Foreground(lipgloss.Yellow).SetString(PendingIcon),
		failureGlyph:               lipgloss.NewStyle().Foreground(lipgloss.Red).SetString(FailureIcon),
		successGlyph:               lipgloss.NewStyle().Foreground(lipgloss.Green).SetString(SuccessIcon),
		noLogsStyle:                lipgloss.NewStyle().Foreground(faintColor).Bold(true),
		watermarkIllustrationStyle: lipgloss.NewStyle().Foreground(lipgloss.White),
		debugStyle:                 lipgloss.NewStyle().Background(lipgloss.Color("1")),
		errorBgStyle:               errorBgStyle,
		errorStyle:                 errorBgStyle.Foreground(lipgloss.Red).Bold(false),
		errorTitleStyle:            errorBgStyle.Foreground(lipgloss.Red).Bold(true),
		separatorStyle:             lipgloss.NewStyle().Foreground(fainterColor),
		commandStyle:               lipgloss.NewStyle().Foreground(lipgloss.Blue).Inline(true),
		stepStartMarkerStyle:       lipgloss.NewStyle().Bold(true).Inline(true),
		groupStartMarkerStyle:      lipgloss.NewStyle().Inline(true),
		scrollbarStyle:             lipgloss.NewStyle(),
		scrollbarThumbStyle:        lipgloss.NewStyle().Foreground(faintColor),
		scrollbarTrackStyle:        lipgloss.NewStyle().Foreground(fainterColor),
	}
}
