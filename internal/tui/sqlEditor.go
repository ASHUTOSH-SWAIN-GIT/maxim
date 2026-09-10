package tui

import (
	"context"
	"database/sql"
	"strings"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type sqlEditorSchemaLoadedMsg struct {
	columns []string
	tables  []string
}

type sqlEditorQueryFinishedMsg struct {
	requestID uint64
	results   string
	hadError  bool
}

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
	queryRunning    bool
	nextQueryID     uint64
	activeQueryID   uint64
	queryCancel     context.CancelFunc
	schemaContext   context.Context
	schemaCancel    context.CancelFunc
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
		"• Autocomplete suggestions appear in this panel\n" +
		"• Press Tab to cycle through suggestions\n" +
		"• Press Enter to select highlighted suggestion\n" +
		"• Press Ctrl+A to run all queries\n" +
		"• Press Ctrl+X to cancel a running query\n" +
		"• Press Ctrl+R to clear results\n" +
		"• Press Esc to return to the workspace\n\n" +
		"Example queries:\n" +
		"SELECT * FROM users;\n" +
		"INSERT INTO users (name) VALUES ('John');\n" +
		"UPDATE users SET name = 'Jane' WHERE id = 1;")

	queryCache := NewQueryCache()
	schemaContext, schemaCancel := context.WithTimeout(context.Background(), db.MetadataTimeout)

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
		schemaContext:   schemaContext,
		schemaCancel:    schemaCancel,
	}
}

func (m sqlEditorModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, loadSQLEditorSchema(m.schemaContext, m.db))
}

func loadSQLEditorSchema(ctx context.Context, conn *sql.DB) tea.Cmd {
	return func() tea.Msg {
		columns, _ := db.GetAllColumnsContext(ctx, conn)
		tables, _ := db.GetAllTablesContext(ctx, conn)
		return sqlEditorSchemaLoadedMsg{columns: columns, tables: tables}
	}
}

func executeSQLBatch(ctx context.Context, conn *sql.DB, content string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		var combined strings.Builder
		hadError := false
		for _, statement := range splitSQLStatements(content) {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			result := db.ExecuteQueryContext(ctx, conn, statement)
			if combined.Len() > 0 {
				combined.WriteString("\n\n")
			}
			if result.Success {
				combined.WriteString(result.Data)
			} else {
				hadError = true
				combined.WriteString(result.Error)
				break
			}
		}
		if combined.Len() == 0 {
			combined.WriteString("No statements to execute.")
		}
		return sqlEditorQueryFinishedMsg{requestID: requestID, results: combined.String(), hadError: hadError}
	}
}

func (m *sqlEditorModel) cancelOperations() {
	if m.queryCancel != nil {
		m.queryCancel()
		m.queryCancel = nil
	}
	if m.schemaCancel != nil {
		m.schemaCancel()
		m.schemaCancel = nil
	}
	m.queryRunning = false
	m.activeQueryID = 0
}

func (m sqlEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case sqlEditorSchemaLoadedMsg:
		m.queryCache.CacheColumns(msg.columns)
		m.queryCache.CacheTables(msg.tables)
		return m, nil

	case sqlEditorQueryFinishedMsg:
		if msg.requestID != m.activeQueryID {
			return m, nil
		}
		m.queryRunning = false
		m.activeQueryID = 0
		m.queryCancel = nil
		m.results = msg.results
		if msg.hadError {
			m.error = msg.results
		} else {
			m.error = ""
		}
		m.viewport.SetContent(msg.results)
		m.textarea.SetValue("")
		return m, nil

	case tea.WindowSizeMsg:
		// Account for both panels' borders and horizontal padding so the editor
		// never wraps beyond the terminal edge.
		panelWidth := max((msg.Width-8)/2, 16)
		panelHeight := max(msg.Height-4, 4)
		m.textarea.SetWidth(panelWidth)
		m.textarea.SetHeight(panelHeight)
		m.viewport.Width = panelWidth
		m.viewport.Height = panelHeight
		m.ready = true

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.cancelOperations()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			m.cancelOperations()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlX:
			if m.queryRunning {
				if m.queryCancel != nil {
					m.queryCancel()
				}
				m.queryRunning = false
				m.activeQueryID = 0
				m.queryCancel = nil
				m.results = "Query cancelled."
				m.error = m.results
				m.viewport.SetContent(m.results)
			}
			return m, nil
		case tea.KeyCtrlR:
			if m.queryCancel != nil {
				m.queryCancel()
			}
			m.queryRunning = false
			m.activeQueryID = 0
			m.queryCancel = nil
			m.results = ""
			m.error = ""
			m.viewport.SetContent("Results cleared.\n\n" +
				"Ready for a new query. Type your SQL in the left panel and press Ctrl+A to run all.")
			return m, nil
		case tea.KeyCtrlA:
			content := m.textarea.Value()
			if strings.TrimSpace(content) == "" {
				m.results = "No statements to execute."
				m.error = ""
				m.viewport.SetContent(m.results)
				return m, nil
			}
			if m.queryCancel != nil {
				m.queryCancel()
			}
			m.nextQueryID++
			m.activeQueryID = m.nextQueryID
			ctx, cancel := context.WithCancel(context.Background())
			m.queryCancel = cancel
			m.queryRunning = true
			m.error = ""
			m.viewport.SetContent("Running query…\n\nPress Ctrl+X to cancel.")
			m.queryCache.AddCommand(content)
			return m, executeSQLBatch(ctx, m.db, content, m.activeQueryID)
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
						"• Autocomplete suggestions appear in this panel\n" +
						"• Press Tab to cycle through suggestions\n" +
						"• Press Enter to select highlighted suggestion\n" +
						"• Press Ctrl+A to run all queries\n" +
						"• Press Ctrl+X to cancel a running query\n" +
						"• Press Ctrl+R to clear results\n" +
						"• Press Esc to return to the workspace\n\n" +
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

// splitSQLStatements splits SQL text into statements by semicolons while
// respecting simple single-quoted strings (no escape handling for quotes inside).
func splitSQLStatements(input string) []string {
	var stmts []string
	var current strings.Builder
	inSingleQuote := false

	for _, r := range input {
		switch r {
		case '\'':
			inSingleQuote = !inSingleQuote
			current.WriteRune(r)
		case ';':
			if inSingleQuote {
				current.WriteRune(r)
			} else {
				stmts = append(stmts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
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
	// Calculate consistent panel height
	panelHeight := m.textarea.Height()
	if m.viewport.Height > panelHeight {
		panelHeight = m.viewport.Height
	}
	leftContent = clipEditorContent(leftContent, max(panelWidth-4, 1), panelHeight)
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
	rightContent = clipEditorContent(rightContent, max(panelWidth-4, 1), panelHeight)

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

func clipEditorContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for index := range lines {
		lines[index] = ansi.Truncate(lines[index], width, "")
	}
	return strings.Join(lines, "\n")
}

func RunSQLEditor(db *sql.DB, dbName string) error {
	p := tea.NewProgram(initialSQLEditorModel(db, dbName))
	_, err := p.Run()
	return err
}
