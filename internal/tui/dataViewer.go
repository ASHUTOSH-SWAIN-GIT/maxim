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
		case "ctrl+c", "q", "esc", "enter":
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

	// Simple title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(titleStyle.Render(fmt.Sprintf(" Table: %s (%d rows)", m.tableName, len(m.rows))))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(noDataStyle.Render("📭 No data found in this table"))
		b.WriteString("\n\n")
	} else {
		// Calculate column widths
		columnWidths := make([]int, len(m.columns))
		for i, col := range m.columns {
			width := len(col.Title)
			if width < 12 {
				width = 12
			}
			columnWidths[i] = width
		}

		// Simple table header
		headerRow := ""
		for i, col := range m.columns {
			headerStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true)
			headerText := headerStyle.Render(fmt.Sprintf(" %-*s ", columnWidths[i]-2, col.Title))
			headerRow += headerText + "│"
		}
		b.WriteString("│" + headerRow)
		b.WriteString("\n")

		// Simple separator
		separator := "├"
		for i, width := range columnWidths {
			separator += strings.Repeat("─", width)
			if i < len(columnWidths)-1 {
				separator += "┼"
			}
		}
		separator += "┤"
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(separator))
		b.WriteString("\n")

		// Data rows - show all data
		for _, row := range m.rows {
			// Build row content
			rowContent := ""
			for j, cell := range row {
				// Convert cell to string properly
				cellValue := fmt.Sprintf("%v", cell)

				// Truncate long values
				if len(cellValue) > columnWidths[j]-2 {
					cellValue = cellValue[:columnWidths[j]-5] + "..."
				}

				// Pad the cell content
				paddedValue := fmt.Sprintf(" %-*s ", columnWidths[j]-2, cellValue)
				rowContent += paddedValue + "│"
			}

			// Style the row
			normalStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("7"))
			b.WriteString("│" + normalStyle.Render(rowContent))
			b.WriteString("\n")
		}

		// Bottom border
		bottomBorder := "└"
		for i, width := range columnWidths {
			bottomBorder += strings.Repeat("─", width)
			if i < len(columnWidths)-1 {
				bottomBorder += "┴"
			}
		}
		bottomBorder += "┘"
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(bottomBorder))
		b.WriteString("\n")

	}

	// Simple footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	b.WriteString(footerStyle.Render("Press any key to close"))
	return b.String()
}

func RunDataViewer(tableName string, columns []table.Column, rows []table.Row) error {
	p := tea.NewProgram(initialDataViewerModel(tableName, columns, rows))
	_, err := p.Run()
	return err
}
