package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tableListModel struct {
	tables []string
	cursor int
	done   bool
}

func initialTableListModel(tables []string) tableListModel {
	return tableListModel{
		tables: tables,
	}
}

func (m tableListModel) Init() tea.Cmd {
	return nil
}

func (m tableListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.tables)-1 {
				m.cursor++
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m tableListModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("📋 Tables in Database"))
	b.WriteString("\n\n")

	if len(m.tables) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(noDataStyle.Render("No tables found in this database."))
		b.WriteString("\n\n")
	} else {
		for i, table := range m.tables {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			tableStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("6"))
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, tableStyle.Render(table)))
		}
	}

	b.WriteString("\n(press Enter to select, q to quit)")
	return b.String()
}

func RunTableList(tables []string) (string, error) {
	p := tea.NewProgram(initialTableListModel(tables))
	m, err := p.Run()
	if err != nil {
		return "", err
	}

	model := m.(tableListModel)
	if model.done && model.cursor < len(model.tables) {
		return model.tables[model.cursor], nil
	}
	return "", fmt.Errorf("no table selected")
}
