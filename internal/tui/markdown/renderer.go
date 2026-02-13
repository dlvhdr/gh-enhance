package markdown

import (
	"charm.land/glamour/v2"
)

func GetMarkdownRenderer(width int) glamour.TermRenderer {
	markdownRenderer, _ := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
		glamour.WithPreservedNewLines(),
	)

	return *markdownRenderer
}
