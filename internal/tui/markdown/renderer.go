package markdown

import (
	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
)

var markdownStyle *ansi.StyleConfig

func GetMarkdownRenderer(width int) glamour.TermRenderer {
	markdownRenderer, _ := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
		glamour.WithPreservedNewLines(),
	)

	return *markdownRenderer
}
