package model

type item struct {
	title       string
	description string
}

func (i item) Title() string { return i.title }

func (i item) Description() string { return i.description }

func (i item) FilterValue() string { return i.title }
