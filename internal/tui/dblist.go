package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dbListModel struct {
	databases []string
	cursor    int
	done      bool
}

func initialDBListModel(databases []string) dbListModel {
	return dbListModel{
		databases: databases,
	}
}

func (m dbListModel) Init() tea.Cmd {
	return nil
}

func (m dbListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.databases)-1 {
				m.cursor++
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dbListModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("Which database do you want to delete?"))
	b.WriteString("\n\n")

	if len(m.databases) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(noDataStyle.Render("No databases found on this server."))
		b.WriteString("\n\n")
	} else {
		for i, db := range m.databases {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			dbStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("6"))
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, dbStyle.Render(db)))
		}
	}

	return b.String()
}

// RunDBList is used for selection (e.g., when deleting a database)
func RunDBList(databases []string) (string, error) {
	p := tea.NewProgram(initialDBListModel(databases))
	m, err := p.Run()
	if err != nil {
		return "", err
	}

	model := m.(dbListModel)
	if model.done && model.cursor < len(model.databases) {
		return model.databases[model.cursor], nil
	}
	return "", fmt.Errorf("no database selected")
}

// Simple display-only model for listing databases
type dbListDisplayModel struct {
	databases []string
	done      bool
}

func initialDBListDisplayModel(databases []string) dbListDisplayModel {
	return dbListDisplayModel{
		databases: databases,
	}
}

func (m dbListDisplayModel) Init() tea.Cmd {
	return nil
}

func (m dbListDisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dbListDisplayModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("Databases on Server"))
	b.WriteString("\n\n")

	if len(m.databases) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(noDataStyle.Render("No databases found on this server."))
		b.WriteString("\n")
	} else {
		for _, db := range m.databases {
			dbStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("6"))
			b.WriteString(fmt.Sprintf("  %s\n", dbStyle.Render(db)))
		}
	}

	return b.String()
}

// RunDBListDisplay simply displays all databases without selection
func RunDBListDisplay(databases []string) error {
	p := tea.NewProgram(initialDBListDisplayModel(databases))
	_, err := p.Run()
	return err
}
