package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type mainMenuModel struct {
	cursor   int
	choices  []string
	done     bool
	quitting bool
}

func initialMainMenuModel() mainMenuModel {
	return mainMenuModel{
		choices: []string{"Connect to a DB", "create a new DB", "List all DBs"},
	}
}

func (m mainMenuModel) Init() tea.Cmd {
	return nil
}

func (m mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m mainMenuModel) View() string {
	if m.done || m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString("What would you like to do?\n\n")

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
	}

	b.WriteString("\n(press q to quit)")
	return b.String()
}

func RunMainMenu() (int, error) {
	p := tea.NewProgram(initialMainMenuModel())
	m, err := p.Run()
	if err != nil {
		return -1, err
	}

	model := m.(mainMenuModel)
	if model.quitting {
		return -1, nil
	}

	return model.cursor, nil
}
