package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sqlEditorModel struct {
	textarea textarea.Model
	viewport viewport.Model
	ready    bool
	db       *sql.DB
	dbName   string
	results  string
	error    string
	quitting bool
}

func initialSQLEditorModel(db *sql.DB, dbName string) sqlEditorModel {
	ta := textarea.New()
	ta.Placeholder = "Enter your SQL query here...\n\nExample:\nSELECT * FROM users;\nINSERT INTO users (name) VALUES ('John');\nUPDATE users SET name = 'Jane' WHERE id = 1;"
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(50)
	ta.SetHeight(15) // Start with a smaller height, will be adjusted by window size
	ta.ShowLineNumbers = true
	ta.Prompt = ""

	vp := viewport.New(50, 15) // Start with a smaller height, will be adjusted by window size
	vp.SetContent("Welcome to the SQL Editor!\n\n" +
		"Instructions:\n" +
		"• Type your SQL queries in the left panel\n" +
		"• Press Ctrl+E to execute the query\n" +
		"• Results will appear in this panel\n" +
		"• Press Ctrl+R to clear results\n" +
		"• Press Esc to quit\n\n" +
		"Example queries:\n" +
		"SELECT * FROM users;\n" +
		"INSERT INTO users (name) VALUES ('John');\n" +
		"UPDATE users SET name = 'Jane' WHERE id = 1;")

	return sqlEditorModel{
		textarea: ta,
		viewport: vp,
		db:       db,
		dbName:   dbName,
	}
}

func (m sqlEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m sqlEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Reserve space for header, footer, and some padding
		// Use a more conservative approach to ensure content is visible
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		// Add extra padding for borders and spacing
		verticalMarginHeight := headerHeight + footerHeight + 6

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we need to wait for
			// them.
			m.textarea.SetWidth(msg.Width / 2)
			m.textarea.SetHeight(msg.Height - verticalMarginHeight)
			m.viewport.Width = msg.Width / 2
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.ready = true
		} else {
			m.textarea.SetWidth(msg.Width / 2)
			m.textarea.SetHeight(msg.Height - verticalMarginHeight)
			m.viewport.Width = msg.Width / 2
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlE:
			// Execute SQL query
			query := strings.TrimSpace(m.textarea.Value())
			if query != "" {
				m.executeQuery(query)
			}
			return m, nil
		case tea.KeyCtrlR:
			// Clear results
			m.results = ""
			m.error = ""
			m.viewport.SetContent("Results cleared.\n\n" +
				"Ready for a new query. Type your SQL in the left panel and press Ctrl+E to execute.")
			return m, nil
		}
	}

	// Update textarea and viewport (let them handle their own input)
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *sqlEditorModel) executeQuery(query string) {
	// Clear previous results
	m.results = ""
	m.error = ""

	// Execute the query
	rows, err := m.db.Query(query)
	if err != nil {
		m.error = fmt.Sprintf("Error executing query:\n%s", err.Error())
		m.viewport.SetContent(m.error)
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		m.error = fmt.Sprintf("Error getting columns:\n%s", err.Error())
		m.viewport.SetContent(m.error)
		return
	}

	// Build results table
	var result strings.Builder
	result.WriteString("Query executed successfully!\n\n")

	// Create header
	header := "│"
	for _, col := range columns {
		header += fmt.Sprintf(" %-15s │", col)
	}
	result.WriteString(header + "\n")

	// Create separator
	separator := "├"
	for i := 0; i < len(columns); i++ {
		separator += strings.Repeat("─", 17)
		if i < len(columns)-1 {
			separator += "┼"
		}
	}
	separator += "┤"
	result.WriteString(separator + "\n")

	// Process rows
	rowCount := 0
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Scan the row
		if err := rows.Scan(valuePtrs...); err != nil {
			m.error = fmt.Sprintf("Error scanning row:\n%s", err.Error())
			m.viewport.SetContent(m.error)
			return
		}

		// Build row string
		rowStr := "│"
		for _, val := range values {
			cellValue := "NULL"
			if val != nil {
				cellValue = fmt.Sprintf("%v", val)
				// Truncate long values
				if len(cellValue) > 15 {
					cellValue = cellValue[:12] + "..."
				}
			}
			rowStr += fmt.Sprintf(" %-15s │", cellValue)
		}
		result.WriteString(rowStr + "\n")
		rowCount++

		// Limit results to prevent overwhelming output
		if rowCount >= 100 {
			result.WriteString("\n... (showing first 100 rows only)\n")
			break
		}
	}

	if err := rows.Err(); err != nil {
		m.error = fmt.Sprintf("Error iterating rows:\n%s", err.Error())
		m.viewport.SetContent(m.error)
		return
	}

	result.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))
	m.results = result.String()
	m.viewport.SetContent(m.results)
}

func (m sqlEditorModel) headerView() string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true).
		Render(fmt.Sprintf("SQL Editor - Database: %s", m.dbName))

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true).
		Render("Ctrl+E: Execute | Ctrl+R: Clear | Esc: Quit")

	return lipgloss.JoinHorizontal(lipgloss.Top, title, instructions)
}

func (m sqlEditorModel) footerView() string {
	info := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true).
		Render("Left: SQL Query | Right: Results")

	return info
}

func (m sqlEditorModel) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return "\n  Initializing SQL Editor..."
	}

	// Use the existing header and footer methods for consistency
	headerContent := m.headerView()
	footerContent := m.footerView()

	// Create left panel (SQL Query) with minimal styling to avoid clipping
	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(m.textarea.Width() + 2).
		Render(m.textarea.View())

	// Create right panel (Results) with minimal styling to avoid clipping
	rightPanel := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(m.viewport.Width + 2).
		Render(m.viewport.View())

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Combine all parts with minimal spacing
	content := lipgloss.JoinVertical(lipgloss.Left,
		headerContent,
		"",
		panels,
		"",
		footerContent,
	)

	return content
}

func RunSQLEditor(db *sql.DB, dbName string) error {
	p := tea.NewProgram(initialSQLEditorModel(db, dbName))
	_, err := p.Run()
	return err
}
