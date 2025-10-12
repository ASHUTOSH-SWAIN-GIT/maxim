package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dataViewerModel struct {
	tableName string
	columns   []table.Column
	rows      []table.Row
	cursor    int
	done      bool
}

func initialDataViewerModel(tableName string, columns []table.Column, rows []table.Row) dataViewerModel {
	return dataViewerModel{
		tableName: tableName,
		columns:   columns,
		rows:      rows,
	}
}

func (m dataViewerModel) Init() tea.Cmd {
	return nil
}

func (m dataViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dataViewerModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render(fmt.Sprintf("📊 Data from table: %s", m.tableName)))
	b.WriteString("\n")

	// Row count
	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)
	b.WriteString(countStyle.Render(fmt.Sprintf("Showing %d rows", len(m.rows))))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(noDataStyle.Render("No data found in this table."))
		b.WriteString("\n\n")
	} else {
		// Column headers
		headerRow := make([]string, len(m.columns))
		for i, col := range m.columns {
			headerStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true).
				Width(15)
			headerRow[i] = headerStyle.Render(col.Title)
		}
		b.WriteString(strings.Join(headerRow, " | "))
		b.WriteString("\n")

		// Separator line
		separator := make([]string, len(m.columns))
		for i := range separator {
			separator[i] = strings.Repeat("-", 15)
		}
		b.WriteString(strings.Join(separator, "-+-"))
		b.WriteString("\n")

		// Data rows
		start := m.cursor
		end := start + 10 // Show 10 rows at a time
		if end > len(m.rows) {
			end = len(m.rows)
		}

		for i := start; i < end; i++ {
			row := m.rows[i]
			rowStrings := make([]string, len(row))
			for j, cell := range row {
				cellStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("7")).
					Width(15)
				rowStrings[j] = cellStyle.Render(fmt.Sprintf("%s", cell))
			}

			// Highlight current row
			if i == m.cursor {
				cursorStyle := lipgloss.NewStyle().
					Background(lipgloss.Color("4")).
					Foreground(lipgloss.Color("15"))
				b.WriteString(cursorStyle.Render("> " + strings.Join(rowStrings, " | ")))
			} else {
				b.WriteString("  " + strings.Join(rowStrings, " | "))
			}
			b.WriteString("\n")
		}

		// Navigation info
		if len(m.rows) > 10 {
			b.WriteString("\n")
			navStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Italic(true)
			b.WriteString(navStyle.Render(fmt.Sprintf("Showing rows %d-%d of %d (↑↓ to navigate)", start+1, end, len(m.rows))))
		}
	}

	b.WriteString("\n\n(press q to quit)")
	return b.String()
}

func RunDataViewer(tableName string, columns []table.Column, rows []table.Row) error {
	p := tea.NewProgram(initialDataViewerModel(tableName, columns, rows))
	_, err := p.Run()
	return err
}
