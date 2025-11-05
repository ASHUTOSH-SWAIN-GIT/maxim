package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type connectTypeMenuModel struct {
	cursor   int
	choices  []string
	done     bool
	quitting bool
}

func initialConnectTypeMenuModel() connectTypeMenuModel {
	return connectTypeMenuModel{
		choices: []string{"Local", "Docker"},
	}
}

func (m connectTypeMenuModel) Init() tea.Cmd {
	return nil
}

func (m connectTypeMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m connectTypeMenuModel) View() string {
	if m.done || m.quitting {
		return ""
	}
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("Connect to Database"))
	b.WriteString("\n\n")
	b.WriteString("Where is the database located?\n\n")

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

func RunConnectTypeMenu() (int, error) {
	p := tea.NewProgram(initialConnectTypeMenuModel())
	m, err := p.Run()
	if err != nil {
		return -1, err
	}

	model := m.(connectTypeMenuModel)
	if model.quitting {
		return -1, nil
	}

	return model.cursor, nil
}

