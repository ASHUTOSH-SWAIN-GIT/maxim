package tui

import (
	"fmt"
	"strings"

	"database/sql"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dataViewerModel struct {
	db        *sql.DB
	tableName string
	columns   []table.Column
	rows      []table.Row
	done      bool
	viewport  viewport.Model
	ready     bool
	limit     int
	offset    int
}

func initialDataViewerModel(conn *sql.DB, tableName string, limit int) dataViewerModel {
	return dataViewerModel{
		db:        conn,
		tableName: tableName,
		limit:     limit,
		offset:    0,
	}
}

func (m dataViewerModel) Init() tea.Cmd {
	return nil
}

func (m dataViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Initialize viewport to fit terminal
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-2)
			m.viewport.YPosition = 0
			m.viewport.HighPerformanceRendering = false
			m.loadPage()
			m.viewport.SetContent(m.renderTable())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
			if len(m.rows) == 0 {
				m.loadPage()
			}
			m.viewport.SetContent(m.renderTable())
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Load next page
			m.offset += m.limit
			m.loadPage()
			m.viewport.SetContent(m.renderTable())
			return m, nil
		case "ctrl+c", "q", "esc":
			m.done = true
			return m, tea.Quit
		}
		// Let viewport handle scrolling keys
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m dataViewerModel) View() string {
	if m.done {
		return ""
	}

	if !m.ready {
		return "\n  Loading table..."
	}

	return m.viewport.View()
}

// renderTable builds the full table content without truncation or hard width caps.
func (m dataViewerModel) renderTable() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Table: %s (%d rows)", m.tableName, len(m.rows))))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		b.WriteString(noDataStyle.Render("No data found in this table"))
		b.WriteString("\n\n")
		return b.String()
	}

	// Calculate dynamic column widths from the data (no truncation)
	columnWidths := make([]int, len(m.columns))
	for i, col := range m.columns {
		columnWidths[i] = len(col.Title)
	}
	for _, row := range m.rows {
		for j, cell := range row {
			cellValue := fmt.Sprintf("%v", cell)
			if len(cellValue) > columnWidths[j] {
				columnWidths[j] = len(cellValue)
			}
		}
	}
	for i := range columnWidths {
		if columnWidths[i] < 1 {
			columnWidths[i] = 1
		}
	}

	// Helper to render a row given a slice of strings
	renderRow := func(values []string) string {
		var parts []string
		for i, v := range values {
			parts = append(parts, fmt.Sprintf("%-*s", columnWidths[i], v))
		}
		return " " + strings.Join(parts, " | ") + " \n"
	}

	// Header
	headers := make([]string, len(m.columns))
	for i, col := range m.columns {
		headers[i] = col.Title
	}
	b.WriteString(renderRow(headers))

	// Separator
	sepParts := make([]string, len(columnWidths))
	totalLen := 1 // leading space
	for i, w := range columnWidths {
		dashes := strings.Repeat("-", w)
		sepParts[i] = dashes
		totalLen += w
		if i < len(columnWidths)-1 {
			totalLen += 3 // " | "
		}
	}
	b.WriteString(" " + strings.Join(sepParts, "-+-") + " \n")

	// Rows
	for _, row := range m.rows {
		cells := make([]string, len(row))
		for j, cell := range row {
			cells[j] = fmt.Sprintf("%v", cell)
		}
		b.WriteString(renderRow(cells))
	}

	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)
	b.WriteString(footerStyle.Render("Use arrow keys/PageUp/PageDown to scroll, q to close"))

	return b.String()
}

// RunPagedDataViewer opens a viewer that loads 100-row pages on demand (Enter to load more)
func RunPagedDataViewer(conn *sql.DB, tableName string) error {
	p := tea.NewProgram(initialDataViewerModel(conn, tableName, 100))
	_, err := p.Run()
	return err
}

func (m *dataViewerModel) loadPage() {
	if m.db == nil {
		return
	}
	cols, rows, err := db.GetTableDataPage(m.db, m.tableName, m.limit, m.offset)
	if err != nil {
		// Render error message
		m.columns = []table.Column{{Title: "error", Width: 20}}
		m.rows = []table.Row{{fmt.Sprintf("%v", err)}}
		return
	}
	if len(m.columns) == 0 {
		m.columns = cols
	}
	m.rows = rows
}
