package tui

import (
	"image/color"
	"strings"

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

type colors struct {
	darkColor      color.Color
	lightColor     color.Color
	errorColor     color.Color
	warnColor      color.Color
	successColor   color.Color
	focusedColor   color.Color
	unfocusedColor color.Color
	faintColor     color.Color
	fainterColor   color.Color
}

type styles struct {
	colors colors

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
	faintFgStyle               lipgloss.Style

	headerStyle lipgloss.Style
	logoStyle   lipgloss.Style
	footerStyle lipgloss.Style
}

func makeStyles() styles {
	t := tint.Current()

	defaultItemStyles := list.NewDefaultItemStyles(true)
	focusedColor := t.Blue
	colors := colors{
		focusedColor:   focusedColor,
		unfocusedColor: tint.Darken(t.BrightBlue, 70),
		darkColor:      tint.Darken(focusedColor, 20),
		lightColor:     tint.Lighten(focusedColor, 20),
		errorColor:     t.BrightRed,
		warnColor:      t.Yellow,
		successColor:   t.Green,
		faintColor:     tint.Darken(focusedColor, 50),
		fainterColor:   tint.Darken(focusedColor, 80),
	}

	errorBgStyle := lipgloss.NewStyle().Background(tint.Darken(t.Red, 80))
	bg := tint.Darken(t.Bg, 10)
	unfocusedBg := tint.Darken(focusedColor, 50)
	unfocusedFg := tint.Darken(focusedColor, 10)
	headerBg := colors.fainterColor

	baseTitleStyle := lipgloss.NewStyle().Bold(true).Margin(0)

	return styles{
		colors: colors,

		faintFgStyle: lipgloss.NewStyle().Foreground(colors.faintColor),

		headerStyle: lipgloss.NewStyle().Foreground(focusedColor).PaddingLeft(1).PaddingTop(1).PaddingRight(1).Border(
			lipgloss.InnerHalfBlockBorder(), false, false, true,
			false).BorderForeground(headerBg).Background(headerBg),
		logoStyle:   lipgloss.NewStyle().Foreground(t.Blue).Background(headerBg),
		footerStyle: lipgloss.NewStyle().Background(colors.fainterColor).Foreground(focusedColor).PaddingLeft(1).PaddingRight(1),

		focusedPaneTitleStyle:      baseTitleStyle.Foreground(t.Black),
		unfocusedPaneTitleStyle:    baseTitleStyle.Foreground(t.Fg),
		focusedPaneTitleBarStyle:   lipgloss.NewStyle().Bold(true).PaddingRight(0).MarginBottom(1),
		unfocusedPaneTitleBarStyle: lipgloss.NewStyle().Bold(true).PaddingRight(0).MarginBottom(1),

		normalItemDescStyle: defaultItemStyles.DimmedDesc.PaddingLeft(4),

		paneItem: paneItemStyles{
			selectedStyle: lipgloss.NewStyle().
				Background(bg).
				BorderBackground(bg).
				Border(lipgloss.OuterHalfBlockBorder(), false, false, false, true).
				BorderForeground(unfocusedBg),

			focusedSelectedStyle: lipgloss.NewStyle().
				Background(bg).
				BorderForeground(focusedColor).
				BorderBackground(bg).
				Border(lipgloss.OuterHalfBlockBorder(), false, false, false, true),

			selectedTitleStyle: lipgloss.NewStyle().
				Bold(true).
				Foreground(unfocusedFg).
				Background(bg),

			focusedTitleStyle:         lipgloss.NewStyle().Bold(true).Foreground(focusedColor),
			focusedSelectedTitleStyle: lipgloss.NewStyle().Bold(true).Foreground(focusedColor).Background(bg),

			unfocusedTitleStyle: lipgloss.NewStyle().Bold(true),

			selectedDescStyle: lipgloss.NewStyle().Foreground(t.White).PaddingLeft(2).Background(bg),
			descStyle:         lipgloss.NewStyle().Foreground(colors.faintColor).PaddingLeft(2),
		},

		paneStyle: lipgloss.NewStyle().BorderRight(true).BorderStyle(
			lipgloss.NormalBorder()).BorderForeground(colors.faintColor),
		lineNumbersStyle:           lipgloss.NewStyle().Foreground(colors.faintColor).Align(lipgloss.Right),
		canceledGlyph:              lipgloss.NewStyle().Foreground(colors.faintColor).SetString(CanceledIcon),
		skippedGlyph:               lipgloss.NewStyle().Foreground(colors.faintColor).SetString(SkippedIcon),
		waitingGlyph:               lipgloss.NewStyle().Foreground(t.Yellow).SetString(WaitingIcon),
		pendingGlyph:               lipgloss.NewStyle().Foreground(t.Yellow).SetString(PendingIcon),
		failureGlyph:               lipgloss.NewStyle().Foreground(t.Red).SetString(FailureIcon),
		successGlyph:               lipgloss.NewStyle().Foreground(colors.successColor).SetString(SuccessIcon),
		noLogsStyle:                lipgloss.NewStyle().Foreground(colors.faintColor).Bold(true),
		watermarkIllustrationStyle: lipgloss.NewStyle().Foreground(t.White),
		debugStyle:                 lipgloss.NewStyle().Background(lipgloss.Color("1")),
		errorBgStyle:               errorBgStyle,
		errorStyle:                 errorBgStyle.Foreground(colors.errorColor).Bold(false),
		errorTitleStyle:            errorBgStyle.Foreground(colors.errorColor).Bold(true),
		separatorStyle:             lipgloss.NewStyle().Foreground(colors.fainterColor),
		commandStyle:               lipgloss.NewStyle().Foreground(t.Blue).Inline(true),
		stepStartMarkerStyle:       lipgloss.NewStyle().Bold(true).Inline(true),
		groupStartMarkerStyle:      lipgloss.NewStyle().Inline(true),
		scrollbarStyle:             lipgloss.NewStyle(),
		scrollbarThumbStyle:        lipgloss.NewStyle().Foreground(colors.faintColor),
		scrollbarTrackStyle:        lipgloss.NewStyle().Foreground(colors.fainterColor),
	}
}

func makePill(text string, textStyle lipgloss.Style, bg color.Color) string {
	sBg := lipgloss.NewStyle().Foreground(bg)
	sFg := lipgloss.NewStyle().Inherit(textStyle).Background(bg)
	return lipgloss.JoinHorizontal(lipgloss.Top, sBg.Render(""), sFg.Render(text), sBg.Render(""))
}

func makePointingBorder(old string) string {
	return strings.Replace(old, lipgloss.NormalBorder().Right, lipgloss.RoundedBorder().TopLeft, 1)
}
