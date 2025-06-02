package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbletea-app-template/pkg/model"
)

func main() {
	p := tea.NewProgram(model.New())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
