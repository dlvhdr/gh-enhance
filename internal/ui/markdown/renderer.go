package markdown

import (
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

var markdownStyle *ansi.StyleConfig

func GetMarkdownRenderer(width int) glamour.TermRenderer {
	markdownRenderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(styles.DarkStyleConfig),
		glamour.WithWordWrap(width),
	)

	return *markdownRenderer
}
