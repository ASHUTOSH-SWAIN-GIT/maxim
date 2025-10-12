package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dbOperationsModel struct {
	cursor   int
	choices  []string
	done     bool
	quitting bool
	dbName   string
}

func initialDBOperationsModel(dbName string) dbOperationsModel {
	return dbOperationsModel{
		dbName: dbName,
		choices: []string{
			"List all tables",
			"Show table data",
			"Editor",
		},
	}
}

func (m dbOperationsModel) Init() tea.Cmd {
	return nil
}

func (m dbOperationsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m dbOperationsModel) View() string {
	if m.done || m.quitting {
		return ""
	}

	var b strings.Builder

	// Header with database name
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render(fmt.Sprintf("Database: %s", m.dbName)))
	b.WriteString("\n\n")

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

func RunDBOperationsMenu(dbName string) (int, error) {
	p := tea.NewProgram(initialDBOperationsModel(dbName))
	m, err := p.Run()
	if err != nil {
		return 0, err
	}

	model := m.(dbOperationsModel)
	if model.quitting {
		return -1, nil // Special value to indicate quit
	}

	return model.cursor, nil
}
