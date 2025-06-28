package ui

import (
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/lipgloss/v2"
)

var (
	focusedColor   = lipgloss.Color("4")
	unfocusedColor = lipgloss.Color("7")
	faintColor     = lipgloss.Color("8")
	fainterColor   = lipgloss.Color("236")

	defaultListStyles          = list.DefaultStyles(true)
	focusedPaneTitleStyle      = defaultListStyles.Title.UnsetBackground().Bold(true).PaddingLeft(0).PaddingRight(0).Margin(0)
	unfocusedPaneTitleStyle    = defaultListStyles.Title.UnsetBackground().Faint(true).PaddingLeft(0).PaddingRight(0).Margin(0)
	focusedPaneTitleBarStyle   = defaultListStyles.Title.UnsetBackground().Bold(true).PaddingLeft(1).PaddingRight(0).MarginBottom(1)
	unfocusedPaneTitleBarStyle = defaultListStyles.Title.UnsetBackground().Faint(true).PaddingLeft(1).PaddingRight(0).MarginBottom(1)

	defaultItemStyles         = list.NewDefaultItemStyles(true)
	normalItemDescStyle       = defaultItemStyles.DimmedDesc.PaddingLeft(4)
	focusedPaneItemTitleStyle = defaultItemStyles.SelectedTitle.Bold(true).Foreground(focusedColor).BorderForeground(
		focusedColor).BorderStyle(lipgloss.InnerHalfBlockBorder())
	unfocusedPaneItemTitleStyle = defaultItemStyles.SelectedTitle.Bold(true).Foreground(focusedColor).BorderForeground(unfocusedColor)
	focusedPaneItemDescStyle    = defaultItemStyles.SelectedDesc.BorderForeground(focusedColor).Foreground(
		defaultItemStyles.NormalDesc.GetForeground()).BorderStyle(lipgloss.InnerHalfBlockBorder()).PaddingLeft(3)
	unfocusedPaneItemDescStyle = defaultItemStyles.SelectedDesc.BorderForeground(unfocusedColor).Foreground(
		defaultItemStyles.NormalDesc.GetForeground()).PaddingLeft(3)
	paneStyle = lipgloss.NewStyle().BorderRight(true).BorderStyle(
		lipgloss.NormalBorder()).BorderForeground(faintColor)

	lineNumbersStyle = lipgloss.NewStyle().Foreground(fainterColor).Align(lipgloss.Right)

	canceledGlyph = lipgloss.NewStyle().
			Foreground(faintColor).
			SetString(CanceledIcon)
	skippedGlyph = lipgloss.NewStyle().
			Foreground(faintColor).
			SetString(SkippedIcon)
	waitingGlyph = lipgloss.NewStyle().
			Foreground(lipgloss.Yellow).
			SetString(WaitingIcon)
	pendingGlyph = lipgloss.NewStyle().
			Foreground(lipgloss.Yellow).
			SetString(PendingIcon)
	failureGlyph = lipgloss.NewStyle().
			Foreground(lipgloss.Red).
			SetString(FailureIcon)
	successGlyph = lipgloss.NewStyle().
			Foreground(lipgloss.Green).
			SetString(SuccessIcon)

	noLogsStyle                = lipgloss.NewStyle().Foreground(faintColor).Bold(true)
	watermarkIllustrationStyle = lipgloss.NewStyle().Foreground(lipgloss.White)

	debugStyle = lipgloss.NewStyle().Background(lipgloss.Color("1"))

	errorBgStyle          = lipgloss.NewStyle().Background(lipgloss.Color("#1C0D0F"))
	errorStyle            = errorBgStyle.Foreground(lipgloss.Red).Bold(false)
	errorTitleStyle       = errorBgStyle.Foreground(lipgloss.Red).Bold(true)
	separatorStyle        = lipgloss.NewStyle().Foreground(fainterColor)
	commandStyle          = lipgloss.NewStyle().Foreground(lipgloss.Blue).Inline(true)
	stepStartMarkerStyle  = lipgloss.NewStyle().Bold(true).Inline(true)
	groupStartMarkerStyle = lipgloss.NewStyle().Inline(true)

	scrollbarStyle      = lipgloss.NewStyle()
	scrollbarThumbStyle = lipgloss.NewStyle().Foreground(faintColor)
	scrollbarTrackStyle = lipgloss.NewStyle().Foreground(fainterColor)
)
