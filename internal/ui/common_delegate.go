package ui

import "github.com/charmbracelet/bubbles/v2/key"

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
