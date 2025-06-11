package ui

import (
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/lipgloss/v2"
)

var (
	defaultListStyles          = list.DefaultStyles(true)
	focusedPaneTitleStyle      = defaultListStyles.Title.UnsetBackground().Bold(true).PaddingLeft(0).PaddingRight(0).Margin(0)
	unfocusedPaneTitleStyle    = defaultListStyles.Title.UnsetBackground().Faint(true).PaddingLeft(0).PaddingRight(0).Margin(0)
	focusedPaneTitleBarStyle   = defaultListStyles.Title.UnsetBackground().Bold(true).PaddingLeft(1).PaddingRight(0).MarginBottom(1)
	unfocusedPaneTitleBarStyle = defaultListStyles.Title.UnsetBackground().Faint(true).PaddingLeft(1).PaddingRight(0).MarginBottom(1)
	debugStyle                 = lipgloss.NewStyle().Background(lipgloss.Color("1"))

	defaultItemStyles           = list.NewDefaultItemStyles(true)
	normalItemDescStyle         = defaultItemStyles.DimmedDesc
	focusedPaneItemTitleStyle   = defaultItemStyles.SelectedTitle.Bold(true).Foreground(lipgloss.Color("4")).BorderForeground(lipgloss.Color("4")).BorderStyle(lipgloss.InnerHalfBlockBorder())
	unfocusedPaneItemTitleStyle = defaultItemStyles.SelectedTitle.Bold(true).Foreground(lipgloss.Color("4")).BorderForeground(lipgloss.Color("7"))
	focusedPaneItemDescStyle    = defaultItemStyles.SelectedDesc.BorderForeground(lipgloss.Color("4")).Foreground(defaultItemStyles.NormalDesc.GetForeground()).BorderStyle(lipgloss.InnerHalfBlockBorder())
	unfocusedPaneItemDescStyle  = defaultItemStyles.SelectedDesc.BorderForeground(lipgloss.Color("7")).Foreground(defaultItemStyles.NormalDesc.GetForeground())
	paneStyle                   = lipgloss.NewStyle().BorderRight(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
)
