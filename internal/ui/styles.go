package ui

import (
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/lipgloss/v2"
)

var (
	defaultListStyles       = list.DefaultStyles(true)
	focusedPaneTitleStyle   = defaultListStyles.Title.Bold(true).PaddingLeft(1).PaddingRight(0)
	unfocusedPaneTitleStyle = defaultListStyles.Title.Background(lipgloss.Color("8")).PaddingLeft(1).PaddingRight(0)

	defaultItemStyles           = list.NewDefaultItemStyles(true)
	normalItemDescStyle         = defaultItemStyles.DimmedDesc
	focusedPaneItemTitleStyle   = defaultItemStyles.SelectedTitle.Bold(true).Foreground(lipgloss.Color("4")).BorderForeground(lipgloss.Color("4"))
	unfocusedPaneItemTitleStyle = defaultItemStyles.SelectedTitle.Bold(true).Foreground(lipgloss.Color("7")).BorderForeground(lipgloss.Color("7"))
	focusedPaneItemDescStyle    = defaultItemStyles.SelectedDesc.BorderForeground(lipgloss.Color("4")).Foreground(defaultItemStyles.NormalDesc.GetForeground())
	unfocusedPaneItemDescStyle  = defaultItemStyles.SelectedDesc.BorderForeground(lipgloss.Color("7")).Foreground(defaultItemStyles.NormalDesc.GetForeground())
	paneStyle                   = lipgloss.NewStyle().BorderRight(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8"))
)
