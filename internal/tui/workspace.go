package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type workspaceMode int

const (
	workspaceModeBrowse workspaceMode = iota
	workspaceModeEditor
)

type workspaceFocus int

const (
	workspaceFocusExplorer workspaceFocus = iota
	workspaceFocusContent
)

type workspaceTab int

const (
	workspaceTabStructure workspaceTab = iota
	workspaceTabData
)

type workspaceTablesLoadedMsg struct {
	tables []string
	err    error
}

type workspaceTableLoadedMsg struct {
	tableName string
	structure []db.TableColumnInfo
	columns   []table.Column
	rows      []table.Row
	offset    int
	err       error
}

type workspaceModel struct {
	db               *sql.DB
	dbName           string
	connectionLabel  string
	changeConnection bool
	mode             workspaceMode
	focus            workspaceFocus
	navigatorOpen    bool
	tab              workspaceTab
	tables           []string
	cursor           int
	selectedTable    string
	structure        []db.TableColumnInfo
	columns          []table.Column
	rows             []table.Row
	rowCursor        int
	rowPeek          bool
	pageSize         int
	offset           int
	loading          bool
	err              string
	width            int
	height           int
	content          viewport.Model
	editor           *sqlEditorModel
}

func initialWorkspaceModel(database *sql.DB, dbName, connectionLabel string) workspaceModel {
	content := viewport.New(80, 20)
	content.SetContent("Choose a table to inspect its structure and data.")
	return workspaceModel{
		db: database, dbName: dbName, connectionLabel: connectionLabel,
		focus: workspaceFocusExplorer, navigatorOpen: true, tab: workspaceTabData,
		pageSize: 100, loading: true, content: content,
	}
}

func (m workspaceModel) Init() tea.Cmd {
	return loadWorkspaceTables(m.db)
}

func loadWorkspaceTables(database *sql.DB) tea.Cmd {
	return func() tea.Msg {
		tables, err := db.GetTables(database)
		return workspaceTablesLoadedMsg{tables: tables, err: err}
	}
}

func loadWorkspaceTable(database *sql.DB, tableName string, pageSize, offset int) tea.Cmd {
	return func() tea.Msg {
		structure, err := db.GetTableStructure(database, tableName)
		if err != nil {
			return workspaceTableLoadedMsg{tableName: tableName, offset: offset, err: err}
		}
		columns, rows, err := db.GetTableDataPage(database, tableName, pageSize, offset)
		return workspaceTableLoadedMsg{
			tableName: tableName, structure: structure, columns: columns,
			rows: rows, offset: offset, err: err,
		}
	}
}

func (m workspaceModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == workspaceModeEditor {
		return m.updateEditor(message)
	}

	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.resizeContent()
	case workspaceTablesLoadedMsg:
		m.loading = false
		if message.err != nil {
			m.err = message.err.Error()
			m.refreshContent()
			return m, nil
		}
		m.tables = message.tables
		if len(m.tables) == 0 {
			m.content.SetContent("No tables found in the public schema.")
		}
	case workspaceTableLoadedMsg:
		m.loading = false
		if message.err != nil {
			m.err = message.err.Error()
		} else {
			m.err = ""
			m.selectedTable = message.tableName
			m.structure = message.structure
			m.columns = message.columns
			m.rows = message.rows
			m.offset = message.offset
			m.rowCursor = 0
			m.rowPeek = false
		}
		m.refreshContent()
	case tea.KeyMsg:
		switch message.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.rowPeek {
				m.rowPeek = false
				m.refreshContent()
				return m, nil
			}
		case "c":
			m.changeConnection = true
			return m, tea.Quit
		case "tab":
			if !m.navigatorOpen && m.selectedTable != "" {
				m.rowPeek = false
				if m.tab == workspaceTabStructure {
					m.tab = workspaceTabData
				} else {
					m.tab = workspaceTabStructure
				}
				m.refreshContent()
			}
			return m, nil
		case "b":
			m.rowPeek = false
			m.navigatorOpen = !m.navigatorOpen
			if m.navigatorOpen {
				m.focus = workspaceFocusExplorer
			} else {
				m.focus = workspaceFocusContent
			}
			return m, nil
		case "e":
			editor := initialSQLEditorModel(m.db, m.dbName)
			m.editor = &editor
			m.mode = workspaceModeEditor
			if m.width > 0 && m.height > 0 {
				updated, command := editor.Update(tea.WindowSizeMsg{Width: m.width, Height: max(m.height-4, 8)})
				editor = updated.(sqlEditorModel)
				m.editor = &editor
				return m, command
			}
			return m, editor.Init()
		case "enter":
			if m.navigatorOpen && len(m.tables) > 0 {
				m.loading = true
				m.err = ""
				m.navigatorOpen = false
				m.focus = workspaceFocusContent
				return m, loadWorkspaceTable(m.db, m.tables[m.cursor], m.pageSize, 0)
			}
			if m.tab == workspaceTabData && len(m.rows) > 0 {
				m.rowPeek = true
				m.refreshContent()
				return m, nil
			}
		case "left", "h":
			if !m.navigatorOpen {
				m.rowPeek = false
				m.tab = workspaceTabStructure
				m.refreshContent()
				return m, nil
			}
		case "right", "l":
			if !m.navigatorOpen {
				m.rowPeek = false
				m.tab = workspaceTabData
				m.refreshContent()
				return m, nil
			}
		case "n":
			if !m.navigatorOpen && m.selectedTable != "" && len(m.rows) == m.pageSize {
				m.loading = true
				return m, loadWorkspaceTable(m.db, m.selectedTable, m.pageSize, m.offset+m.pageSize)
			}
		case "p":
			if !m.navigatorOpen && m.selectedTable != "" && m.offset > 0 {
				m.loading = true
				nextOffset := m.offset - m.pageSize
				if nextOffset < 0 {
					nextOffset = 0
				}
				return m, loadWorkspaceTable(m.db, m.selectedTable, m.pageSize, nextOffset)
			}
		case "up", "k":
			if m.navigatorOpen {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
			if m.tab == workspaceTabData && m.rowCursor > 0 {
				m.rowCursor--
				m.refreshContent()
				return m, nil
			}
		case "down", "j":
			if m.navigatorOpen {
				if m.cursor < len(m.tables)-1 {
					m.cursor++
				}
				return m, nil
			}
			if m.tab == workspaceTabData && m.rowCursor < len(m.rows)-1 {
				m.rowCursor++
				m.refreshContent()
				return m, nil
			}
		}
		if !m.navigatorOpen {
			var command tea.Cmd
			m.content, command = m.content.Update(message)
			return m, command
		}
	}
	return m, nil
}

func (m workspaceModel) updateEditor(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		if key.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if key.Type == tea.KeyEsc {
			m.mode = workspaceModeBrowse
			m.editor.quitting = false
			return m, nil
		}
	}
	updated, command := m.editor.Update(message)
	editor := updated.(sqlEditorModel)
	m.editor = &editor
	return m, command
}

func (m *workspaceModel) resizeContent() {
	height := m.height - 7
	if height < 4 {
		height = 4
	}
	width := m.width - 4
	if width < 20 {
		width = 20
	}
	m.content.Width = width
	m.content.Height = height
	if m.editor != nil {
		updated, _ := m.editor.Update(tea.WindowSizeMsg{Width: m.width, Height: max(m.height-4, 8)})
		editor := updated.(sqlEditorModel)
		m.editor = &editor
	}
}

func (m *workspaceModel) refreshContent() {
	if m.err != "" {
		m.content.SetContent("Error\n\n" + m.err)
		return
	}
	if m.selectedTable == "" {
		m.content.SetContent("Choose a table to inspect its structure and data.")
		return
	}
	if m.tab == workspaceTabStructure {
		m.content.SetContent(renderWorkspaceStructure(m.selectedTable, m.structure))
	} else if m.rowPeek && len(m.rows) > 0 {
		m.content.SetContent(renderWorkspaceRowPeek(m.selectedTable, m.columns, m.rows[m.rowCursor], m.offset+m.rowCursor+1))
	} else {
		m.content.SetContent(renderWorkspaceRows(m.selectedTable, m.columns, m.rows, m.offset, m.content.Width, m.rowCursor))
		selectedLine := 4 + m.rowCursor*2
		if m.content.Width < 80 || len(m.columns) > 6 {
			selectedLine = 2 + m.rowCursor*(len(m.columns)+2)
		}
		m.content.SetYOffset(max(selectedLine-m.content.Height/2, 0))
		return
	}
	m.content.GotoTop()
}

func renderWorkspaceStructure(tableName string, columns []db.TableColumnInfo) string {
	var output strings.Builder
	output.WriteString("Structure · " + tableName + "\n\n")
	for _, column := range columns {
		name := column.Name
		if column.PrimaryKey {
			name += " [primary key]"
		}
		nullable := "required"
		if column.Nullable {
			nullable = "nullable"
		}
		details := column.DataType + " · " + nullable
		if column.Default != "" {
			details += " · default: " + column.Default
		}
		output.WriteString(name + "\n  " + details + "\n\n")
	}
	return output.String()
}

func renderWorkspaceRows(tableName string, columns []table.Column, rows []table.Row, offset, maxWidth, selectedRow int) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Data · %s · rows %d–%d\n\n", tableName, offset+1, offset+len(rows)))
	if len(rows) == 0 {
		output.WriteString("No rows on this page.")
		return output.String()
	}
	if len(columns) == 0 {
		output.WriteString("The database returned rows without column metadata.")
		return output.String()
	}
	if maxWidth < 80 || len(columns) > 6 {
		for rowIndex, row := range rows {
			marker := "  "
			if rowIndex == selectedRow {
				marker = "> "
			}
			output.WriteString(fmt.Sprintf("%sRow %d\n", marker, offset+rowIndex+1))
			for columnIndex, value := range row {
				if columnIndex >= len(columns) {
					break
				}
				output.WriteString(fmt.Sprintf("  %s: %s\n", columns[columnIndex].Title, value))
			}
			output.WriteString("\n")
		}
		return output.String()
	}
	widths := make([]int, len(columns))
	separatorWidth := (len(columns)-1)*3 + 2
	columnLimit := (maxWidth - separatorWidth) / len(columns)
	columnLimit = min(max(columnLimit, 8), 28)
	for index, column := range columns {
		widths[index] = min(max(len(column.Title), 8), columnLimit)
	}
	for _, row := range rows {
		for index, value := range row {
			widths[index] = min(max(widths[index], len(value)), columnLimit)
		}
	}
	writeRow := func(values []string, prefix string) {
		output.WriteString(prefix)
		for index, value := range values {
			if len(value) > widths[index] {
				value = value[:widths[index]-3] + "..."
			}
			output.WriteString(fmt.Sprintf("%-*s", widths[index], value))
			if index < len(values)-1 {
				output.WriteString(" │ ")
			}
		}
		output.WriteString("\n")
	}
	headings := make([]string, len(columns))
	for index, column := range columns {
		headings[index] = column.Title
	}
	writeRow(headings, "  ")
	dividerParts := make([]string, len(widths))
	for index, width := range widths {
		dividerParts[index] = strings.Repeat("─", width)
	}
	divider := "──" + strings.Join(dividerParts, "─┼─") + "\n"
	output.WriteString(divider)
	for rowIndex, row := range rows {
		prefix := "  "
		if rowIndex == selectedRow {
			prefix = "> "
		}
		writeRow([]string(row), prefix)
		if rowIndex < len(rows)-1 {
			output.WriteString(divider)
		}
	}
	return output.String()
}

func renderWorkspaceRowPeek(tableName string, columns []table.Column, row table.Row, rowNumber int) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Row %d · %s\n\n", rowNumber, tableName))
	labelWidth := 0
	for _, column := range columns {
		labelWidth = max(labelWidth, len(column.Title))
	}
	for index, value := range row {
		if index >= len(columns) {
			break
		}
		output.WriteString(fmt.Sprintf("%-*s  %s\n", labelWidth, columns[index].Title, value))
	}
	output.WriteString("\nEsc returns to the data grid.")
	return output.String()
}

func (m workspaceModel) View() string {
	accent := lipgloss.Color("6")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("● connected")
	top := header.Render("MAXIM") + "  " + m.dbName
	if m.width >= 100 {
		top += "  " + muted.Render(m.connectionLabel)
	}
	top += "  " + status
	separatorWidth := max(m.width, 30)
	separator := muted.Render(strings.Repeat("─", separatorWidth))

	if m.mode == workspaceModeEditor && m.editor != nil {
		footer := muted.Render("Esc workspace • Ctrl+A run • Ctrl+R clear")
		return top + "\n" + separator + "\n" + header.Render("Query") + "\n" + m.editor.View() + "\n" + footer
	}

	if m.navigatorOpen {
		explorerContent := header.Render("Choose a table") + "  " + muted.Render("public") + "\n\n"
		if m.loading && len(m.tables) == 0 {
			explorerContent += muted.Render("Loading tables…")
		} else if len(m.tables) == 0 {
			explorerContent += muted.Render("No tables")
		} else {
			for index, tableName := range m.tables {
				prefix := "  "
				style := muted
				if index == m.cursor {
					prefix = "> "
					style = lipgloss.NewStyle().Foreground(accent).Bold(m.focus == workspaceFocusExplorer)
				}
				explorerContent += prefix + style.Render(tableName) + "\n"
			}
		}
		footer := muted.Render("↑/↓ select • Enter open • b close • c connections • q quit")
		return top + "\n" + separator + "\n\n" + explorerContent + "\n" + footer
	}

	context := muted.Render("public / ") + header.Render(m.selectedTable)
	tabs := "Data  Structure  Query"
	if m.tab == workspaceTabStructure {
		tabs = muted.Render("Data") + "  " + header.Render("Structure") + "  " + muted.Render("Query")
	} else {
		tabs = header.Render("Data") + "  " + muted.Render("Structure") + "  " + muted.Render("Query")
	}
	contentBody := context + "    " + tabs + "\n" + separator + "\n\n"
	if m.loading && m.selectedTable != "" {
		contentBody += muted.Render("Loading " + m.selectedTable + "…")
	} else {
		contentBody += m.content.View()
	}

	footerText := "↑/↓ row • Enter peek • b tables • Tab/←/→ view • n/p page • e query • c connections • q quit"
	if m.width > 0 && m.width < 90 {
		footerText = "↑/↓ row • Enter peek • b tables • Tab view • e query • q quit"
	}
	footer := muted.Render(footerText)
	return top + "\n" + separator + "\n" + contentBody + "\n" + footer
}

func RunWorkspace(database *sql.DB, dbName, connectionLabel string) (bool, error) {
	program := tea.NewProgram(initialWorkspaceModel(database, dbName, connectionLabel), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}
	return finalModel.(workspaceModel).changeConnection, nil
}
