package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sqlEditorModel struct {
	textarea        textarea.Model
	viewport        viewport.Model
	ready           bool
	db              *sql.DB
	dbName          string
	results         string
	error           string
	quitting        bool
	queryCache      *QueryCache
	suggestions     []string
	selectedIndex   int
	showSuggestions bool
	justSelected    bool
}

func initialSQLEditorModel(conn *sql.DB, dbName string) sqlEditorModel {
	ta := textarea.New()
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(50)
	ta.SetHeight(15) // Start with a smaller height, will be adjusted by window size
	ta.ShowLineNumbers = true
	ta.Prompt = ""

	vp := viewport.New(50, 15) // Start with a smaller height, will be adjusted by window size
	vp.SetContent("SQL Editor - Database: " + dbName + "\n\n" +
		"Instructions:\n" +
		"• Type your SQL queries in the left panel\n" +
		"• Press Ctrl+E to execute the query\n" +
		"• Autocomplete suggestions appear in this panel\n" +
		"• Press Tab to cycle through suggestions\n" +
		"• Press Enter to select highlighted suggestion\n" +
		"• Press Ctrl+R to clear results\n" +
		"• Press Esc to quit\n\n" +
		"Example queries:\n" +
		"SELECT * FROM users;\n" +
		"INSERT INTO users (name) VALUES ('John');\n" +
		"UPDATE users SET name = 'Jane' WHERE id = 1;")

	// Create query cache and cache columns
	queryCache := NewQueryCache()

	// Cache all columns and tables from the database for autocomplete
	columns, err := db.GetAllColumns(conn)
	if err == nil && len(columns) > 0 {
		queryCache.CacheColumns(columns)
	}

	tables, err := db.GetAllTables(conn)
	if err == nil && len(tables) > 0 {
		queryCache.CacheTables(tables)
	}

	return sqlEditorModel{
		textarea:        ta,
		viewport:        vp,
		db:              conn,
		dbName:          dbName,
		queryCache:      queryCache,
		suggestions:     []string{},
		selectedIndex:   0,
		showSuggestions: false,
		justSelected:    false,
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
		// Use the full terminal size for the editor
		// Reserve minimal space for borders and padding
		verticalMarginHeight := 4 // Minimal padding for borders

		if !m.ready {
			// Use full terminal width and height
			m.textarea.SetWidth(msg.Width / 2)
			m.textarea.SetHeight(msg.Height - verticalMarginHeight)
			m.viewport.Width = msg.Width / 2
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.ready = true
		} else {
			// Responsive resizing - use full terminal size
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
		case tea.KeyTab:
			// Cycle through suggestions
			if m.showSuggestions && len(m.suggestions) > 0 {
				m.selectedIndex = (m.selectedIndex + 1) % len(m.suggestions)
				return m, nil
			}
			// If no suggestions, let textarea handle it normally
		case tea.KeyShiftTab:
			// Cycle through suggestions backwards
			if m.showSuggestions && len(m.suggestions) > 0 {
				m.selectedIndex = (m.selectedIndex - 1 + len(m.suggestions)) % len(m.suggestions)
				return m, nil
			}
			// If no suggestions, let textarea handle it normally
		case tea.KeyEnter:
			// Select the highlighted suggestion
			if m.showSuggestions && len(m.suggestions) > 0 {
				// Insert the selected suggestion
				selectedSuggestion := m.suggestions[m.selectedIndex]
				currentValue := m.textarea.Value()
				lines := strings.Split(currentValue, "\n")
				if len(lines) > 0 {
					// Get the current line
					currentLine := lines[len(lines)-1]
					// Find the last word to replace
					words := strings.Fields(currentLine)
					if len(words) > 0 {
						// Replace the last word with the suggestion
						words[len(words)-1] = selectedSuggestion
						lines[len(lines)-1] = strings.Join(words, " ") + " "
						m.textarea.SetValue(strings.Join(lines, "\n"))
					} else {
						// No words, just add the suggestion
						lines[len(lines)-1] = selectedSuggestion + " "
						m.textarea.SetValue(strings.Join(lines, "\n"))
					}
				}
				// Clear suggestions after selection but keep right panel restored
				m.showSuggestions = false
				m.suggestions = []string{}
				m.selectedIndex = 0
				m.justSelected = true
				// Restore the right panel to show previous results or welcome message
				if m.results != "" {
					m.viewport.SetContent(m.results)
				} else if m.error != "" {
					m.viewport.SetContent(m.error)
				} else {
					m.viewport.SetContent("SQL Editor - Database: " + m.dbName + "\n\n" +
						"Instructions:\n" +
						"• Type your SQL queries in the left panel\n" +
						"• Press Ctrl+E to execute the query\n" +
						"• Autocomplete suggestions appear in this panel\n" +
						"• Press Tab to cycle through suggestions\n" +
						"• Press Enter to select highlighted suggestion\n" +
						"• Press Ctrl+R to clear results\n" +
						"• Press Esc to quit\n\n" +
						"Example queries:\n" +
						"SELECT * FROM users;\n" +
						"INSERT INTO users (name) VALUES ('John');\n" +
						"UPDATE users SET name = 'Jane' WHERE id = 1;")
				}
				return m, nil
			}
			// If no suggestions, let textarea handle Enter normally (new line)
		}
	}

	// Update textarea and viewport (let them handle their own input)
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	// Update suggestions based on current input
	m.updateSuggestions()

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *sqlEditorModel) updateSuggestions() {
	// Don't update suggestions if we just selected one
	if m.justSelected {
		m.justSelected = false
		return
	}

	currentValue := m.textarea.Value()
	lines := strings.Split(currentValue, "\n")
	if len(lines) > 0 {
		currentLine := lines[len(lines)-1]
		words := strings.Fields(currentLine)
		if len(words) > 0 {
			lastWord := words[len(words)-1]
			if len(lastWord) > 0 {
				suggestions := m.queryCache.GetSuggestions(lastWord)
				if len(suggestions) > 0 {
					// Only reset suggestions if they're different
					if !m.suggestionsEqual(m.suggestions, suggestions) {
						m.suggestions = suggestions
						m.selectedIndex = 0
					}
					m.showSuggestions = true
				} else {
					m.showSuggestions = false
					m.suggestions = []string{}
					m.selectedIndex = 0
				}
			} else {
				m.showSuggestions = false
				m.suggestions = []string{}
				m.selectedIndex = 0
			}
		} else {
			m.showSuggestions = false
			m.suggestions = []string{}
			m.selectedIndex = 0
		}
	} else {
		m.showSuggestions = false
		m.suggestions = []string{}
		m.selectedIndex = 0
	}
}

// Helper function to compare two string slices
func (m *sqlEditorModel) suggestionsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (m *sqlEditorModel) executeQuery(query string) {
	// Clear previous results
	m.results = ""
	m.error = ""

	// Add the query to cache for future suggestions
	m.queryCache.AddCommand(query)

	// Execute the query using the separated database logic
	result := db.ExecuteQuery(m.db, query)

	if result.Success {
		m.results = result.Data
		m.viewport.SetContent(m.results)
		// Clear the textarea after successful execution
		m.textarea.SetValue("")
	} else {
		m.error = result.Error
		// Format error with better styling and add clear instruction
		formattedError := fmt.Sprintf("%s\n\nPress Ctrl+R to clear this error", result.Error)
		m.viewport.SetContent(formattedError)
	}
}

func (m sqlEditorModel) headerView() string {
	// Return empty string to remove external header
	return ""
}

func (m sqlEditorModel) footerView() string {
	// Return empty string to remove external footer
	return ""
}

func (m sqlEditorModel) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return "\n  Initializing SQL Editor..."
	}

	// Create left panel content (SQL Query only)
	leftContent := m.textarea.View()

	// Calculate consistent panel dimensions (use the smaller of the two to ensure symmetry)
	panelWidth := m.textarea.Width()
	if m.viewport.Width < panelWidth {
		panelWidth = m.viewport.Width
	}
	panelWidth += 2 // Add border width

	// Calculate consistent panel height
	panelHeight := m.textarea.Height()
	if m.viewport.Height > panelHeight {
		panelHeight = m.viewport.Height
	}
	panelHeight += 2 // Add border height

	// Create left panel (SQL Query) with consistent dimensions
	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(panelWidth).
		Height(panelHeight).
		Render(leftContent)

	// Create right panel content (Results or Suggestions)
	var rightContent string

	// Show suggestions if available, otherwise show results
	if m.showSuggestions && len(m.suggestions) > 0 {
		rightContent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true).
			Render("Suggestions (Tab: cycle, Enter: select):")

		for i, suggestion := range m.suggestions {
			if i >= 8 { // Limit to 8 visible suggestions in right panel
				break
			}
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
			if i == m.selectedIndex {
				style = style.Foreground(lipgloss.Color("3")).Bold(true)
			}
			rightContent += "\n  " + style.Render("• "+suggestion)
		}

		// Pad the suggestions content to match viewport height
		lines := strings.Split(rightContent, "\n")
		viewportHeight := m.viewport.Height
		if len(lines) < viewportHeight {
			// Add empty lines to match viewport height
			for i := len(lines); i < viewportHeight; i++ {
				rightContent += "\n"
			}
		}
	} else {
		rightContent = m.viewport.View()
	}

	// Create right panel (Results or Suggestions) with consistent dimensions
	rightPanel := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(panelWidth).
		Height(panelHeight).
		Render(rightContent)

	// Join panels horizontally to create single container
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Return just the panels - single container taking up whole terminal
	return panels
}

func RunSQLEditor(db *sql.DB, dbName string) error {
	p := tea.NewProgram(initialSQLEditorModel(db, dbName))
	_, err := p.Run()
	return err
}
