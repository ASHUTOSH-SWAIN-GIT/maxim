package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type createTypeMenuModel struct {
	cursor   int
	choices  []string
	done     bool
	quitting bool
}

func initialCreateTypeMenuModel() createTypeMenuModel {
	return createTypeMenuModel{
		choices: []string{"Local", "Docker"},
	}
}

func (m createTypeMenuModel) Init() tea.Cmd {
	return nil
}

func (m createTypeMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
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

func (m createTypeMenuModel) View() string {
	if m.done || m.quitting {
		return ""
	}
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("Create Database"))
	b.WriteString("\n\n")
	b.WriteString("Where would you like to create the database?\n\n")

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		choiceText := choice
		if choice == "Docker" {
			choiceText += " (in Docker container)"
		}

		b.WriteString(fmt.Sprintf("%s %s\n", cursor, choiceText))
	}

	b.WriteString("\n(press Enter to submit, Esc to quit)")
	return b.String()
}

func RunCreateTypeMenu() (int, error) {
	p := tea.NewProgram(initialCreateTypeMenuModel())
	m, err := p.Run()
	if err != nil {
		return -1, err
	}

	model := m.(createTypeMenuModel)
	if model.quitting {
		return -1, nil
	}

	return model.cursor, nil
}

