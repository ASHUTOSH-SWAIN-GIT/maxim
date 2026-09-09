package tui

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
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
	requestID uint64
	tables    []string
	err       error
}

type workspaceTableLoadedMsg struct {
	requestID      uint64
	tableName      string
	structure      []db.TableColumnInfo
	columns        []table.Column
	rows           []table.Row
	offset         int
	hasNext        bool
	nextCursor     string
	keysetEnabled  bool
	sortColumn     string
	sortDescending bool
	filterColumn   string
	filterValue    string
	err            error
}

type workspaceBrowseIntent struct {
	tableName      string
	request        db.TableBrowseRequest
	cursorHistory  []string
	closeNavigator bool
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
	hasNext          bool
	nextCursor       string
	keysetEnabled    bool
	cursorHistory    []string
	sortColumn       string
	sortDescending   bool
	filterColumn     string
	filterValue      string
	filterEditing    bool
	filterInput      textinput.Model
	loading          bool
	err              string
	width            int
	height           int
	content          viewport.Model
	editor           *sqlEditorModel
	nextRequestID    uint64
	activeRequestID  uint64
	requestContext   context.Context
	requestCancel    context.CancelFunc
	pendingBrowse    *workspaceBrowseIntent
	failedBrowse     *workspaceBrowseIntent
	notice           string
	tablesLoadFailed bool
}

func initialWorkspaceModel(database *sql.DB, dbName, connectionLabel string) workspaceModel {
	content := viewport.New(80, 20)
	content.SetContent("Choose a table to inspect its structure and data.")
	filterInput := textinput.New()
	filterInput.Prompt = "Filter column=value: "
	filterInput.Placeholder = "status=paid"
	requestContext, requestCancel := context.WithCancel(context.Background())
	return workspaceModel{
		db: database, dbName: dbName, connectionLabel: connectionLabel,
		focus: workspaceFocusExplorer, navigatorOpen: true, tab: workspaceTabData,
		pageSize: 100, loading: true, content: content, filterInput: filterInput,
		nextRequestID: 1, activeRequestID: 1,
		requestContext: requestContext, requestCancel: requestCancel,
	}
}

func (m workspaceModel) Init() tea.Cmd {
	return loadWorkspaceTables(m.requestContext, m.db, m.activeRequestID)
}

func loadWorkspaceTables(ctx context.Context, database *sql.DB, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		tables, err := db.GetTablesContext(ctx, database)
		return workspaceTablesLoadedMsg{requestID: requestID, tables: tables, err: err}
	}
}

func loadWorkspaceBrowse(ctx context.Context, database *sql.DB, tableName string, request db.TableBrowseRequest, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		structure, err := db.GetTableStructureContext(ctx, database, tableName)
		if err != nil {
			return workspaceTableLoadedMsg{requestID: requestID, tableName: tableName, offset: request.Offset, err: err}
		}
		page, err := db.BrowseTableContext(ctx, database, tableName, request)
		return workspaceTableLoadedMsg{
			requestID: requestID, tableName: tableName, structure: structure, columns: page.Columns,
			rows: page.Rows, offset: request.Offset, hasNext: page.HasNext,
			nextCursor: page.NextCursor, keysetEnabled: page.KeysetEnabled,
			sortColumn: page.SortColumn, sortDescending: request.Descending,
			filterColumn: request.FilterColumn, filterValue: request.FilterValue, err: err,
		}
	}
}

func (m *workspaceModel) startBrowse(intent workspaceBrowseIntent) tea.Cmd {
	if m.requestCancel != nil {
		m.requestCancel()
	}
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	ctx, cancel := context.WithCancel(context.Background())
	m.requestContext = ctx
	m.requestCancel = cancel
	intent.cursorHistory = append([]string(nil), intent.cursorHistory...)
	m.pendingBrowse = &intent
	m.failedBrowse = nil
	m.notice = ""
	m.loading = true
	return loadWorkspaceBrowse(ctx, m.db, intent.tableName, intent.request, m.activeRequestID)
}

func (m *workspaceModel) startTableDiscovery() tea.Cmd {
	if m.requestCancel != nil {
		m.requestCancel()
	}
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	ctx, cancel := context.WithCancel(context.Background())
	m.requestContext = ctx
	m.requestCancel = cancel
	m.loading = true
	m.tablesLoadFailed = false
	m.notice = ""
	return loadWorkspaceTables(ctx, m.db, m.activeRequestID)
}

func (m *workspaceModel) finishRequest() {
	if m.requestCancel != nil {
		m.requestCancel()
	}
	m.requestCancel = nil
	m.requestContext = nil
	m.activeRequestID = 0
}

func (m *workspaceModel) cancelRequest() {
	if m.requestCancel != nil {
		m.requestCancel()
		m.requestCancel = nil
		m.requestContext = nil
	}
	m.activeRequestID = 0
	m.loading = false
	m.pendingBrowse = nil
}

func (m workspaceModel) browseRequest(cursor string, offset int) db.TableBrowseRequest {
	return db.TableBrowseRequest{
		Limit: m.pageSize, Offset: offset, SortColumn: m.sortColumn,
		Descending: m.sortDescending, FilterColumn: m.filterColumn,
		FilterValue: m.filterValue, Cursor: cursor,
	}
}

func (m workspaceModel) browseIntent(cursor string, offset int) workspaceBrowseIntent {
	return workspaceBrowseIntent{
		tableName:     m.selectedTable,
		request:       m.browseRequest(cursor, offset),
		cursorHistory: append([]string(nil), m.cursorHistory...),
	}
}

func countPrimaryKeys(columns []db.TableColumnInfo) int {
	count := 0
	for _, column := range columns {
		if column.PrimaryKey {
			count++
		}
	}
	return count
}

func (m workspaceModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.resizeContent()
		return m, nil
	case workspaceTablesLoadedMsg:
		if message.requestID != m.activeRequestID {
			return m, nil
		}
		m.finishRequest()
		m.loading = false
		if message.err != nil {
			m.err = message.err.Error()
			m.tablesLoadFailed = true
			m.refreshContent()
			return m, nil
		}
		m.err = ""
		m.tablesLoadFailed = false
		m.tables = message.tables
		if len(m.tables) == 0 {
			m.content.SetContent("No tables found in the public schema.")
		}
		return m, nil
	case workspaceTableLoadedMsg:
		if message.requestID != m.activeRequestID {
			return m, nil
		}
		intent := m.pendingBrowse
		m.finishRequest()
		m.loading = false
		if message.err != nil {
			m.notice = "Load failed: " + message.err.Error()
			m.failedBrowse = intent
		} else {
			m.err = ""
			m.notice = ""
			m.failedBrowse = nil
			m.selectedTable = message.tableName
			m.structure = message.structure
			m.columns = message.columns
			m.rows = message.rows
			m.offset = message.offset
			m.hasNext = message.hasNext
			m.nextCursor = message.nextCursor
			m.keysetEnabled = message.keysetEnabled
			m.sortColumn = message.sortColumn
			m.sortDescending = message.sortDescending
			m.filterColumn = message.filterColumn
			m.filterValue = message.filterValue
			if intent != nil {
				m.cursorHistory = append([]string(nil), intent.cursorHistory...)
				if intent.closeNavigator {
					m.navigatorOpen = false
					m.focus = workspaceFocusContent
				}
			}
			m.rowCursor = 0
			m.rowPeek = false
		}
		m.pendingBrowse = nil
		if message.err == nil {
			m.refreshContent()
		}
		return m, nil
	}

	if m.mode == workspaceModeEditor {
		return m.updateEditor(message)
	}
	if m.filterEditing {
		if key, ok := message.(tea.KeyMsg); ok {
			switch key.Type {
			case tea.KeyEsc:
				m.filterEditing = false
				m.filterInput.Blur()
				return m, nil
			case tea.KeyEnter:
				value := strings.TrimSpace(m.filterInput.Value())
				column, filter, found := strings.Cut(value, "=")
				if value != "" && (!found || strings.TrimSpace(column) == "" || strings.TrimSpace(filter) == "") {
					m.notice = "Filter must use column=value, for example status=paid."
					return m, nil
				}
				filterColumn, filterValue := strings.TrimSpace(column), strings.TrimSpace(filter)
				if value == "" {
					filterColumn, filterValue = "", ""
				}
				m.filterEditing = false
				m.filterInput.Blur()
				intent := m.browseIntent("", 0)
				intent.request.FilterColumn = filterColumn
				intent.request.FilterValue = filterValue
				intent.cursorHistory = nil
				return m, m.startBrowse(intent)
			}
		}
		var command tea.Cmd
		m.filterInput, command = m.filterInput.Update(message)
		return m, command
	}

	if message, ok := message.(tea.KeyMsg); ok {
		switch message.String() {
		case "ctrl+c", "q":
			m.cancelRequest()
			return m, tea.Quit
		case "esc":
			if m.rowPeek {
				m.rowPeek = false
				m.refreshContent()
				return m, nil
			}
		case "c":
			m.cancelRequest()
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
		case "/":
			if !m.navigatorOpen && m.selectedTable != "" {
				m.filterEditing = true
				m.filterInput.SetValue("")
				if m.filterColumn != "" {
					m.filterInput.SetValue(m.filterColumn + "=" + m.filterValue)
				}
				m.filterInput.Focus()
				return m, textinput.Blink
			}
		case "s":
			if !m.navigatorOpen && len(m.structure) > 0 {
				next := 0
				for index, column := range m.structure {
					if column.Name == m.sortColumn {
						next = (index + 1) % len(m.structure)
						break
					}
				}
				intent := m.browseIntent("", 0)
				intent.request.SortColumn = m.structure[next].Name
				intent.cursorHistory = nil
				return m, m.startBrowse(intent)
			}
		case "S":
			if !m.navigatorOpen && m.selectedTable != "" {
				intent := m.browseIntent("", 0)
				intent.request.Descending = !m.sortDescending
				intent.cursorHistory = nil
				return m, m.startBrowse(intent)
			}
		case "e":
			editor := initialSQLEditorModel(m.db, m.dbName)
			m.editor = &editor
			m.mode = workspaceModeEditor
			if m.width > 0 && m.height > 0 {
				updated, command := editor.Update(tea.WindowSizeMsg{Width: m.width, Height: max(m.height-4, 8)})
				editor = updated.(sqlEditorModel)
				m.editor = &editor
				return m, tea.Batch(command, editor.Init())
			}
			return m, editor.Init()
		case "enter":
			if m.navigatorOpen && len(m.tables) > 0 {
				intent := workspaceBrowseIntent{
					tableName:      m.tables[m.cursor],
					request:        db.TableBrowseRequest{Limit: m.pageSize},
					closeNavigator: true,
				}
				return m, m.startBrowse(intent)
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
			if !m.navigatorOpen && m.selectedTable != "" && m.hasNext {
				intent := m.browseIntent("", m.offset+m.pageSize)
				if m.keysetEnabled {
					intent.request.Cursor = m.nextCursor
					intent.cursorHistory = append(intent.cursorHistory, m.nextCursor)
				}
				return m, m.startBrowse(intent)
			}
		case "p":
			if !m.navigatorOpen && m.selectedTable != "" && m.offset > 0 {
				nextOffset := m.offset - m.pageSize
				if nextOffset < 0 {
					nextOffset = 0
				}
				cursor := ""
				history := append([]string(nil), m.cursorHistory...)
				if m.keysetEnabled && len(history) > 0 {
					history = history[:len(history)-1]
					if len(history) > 0 {
						cursor = history[len(history)-1]
					}
				}
				intent := m.browseIntent(cursor, nextOffset)
				intent.cursorHistory = history
				return m, m.startBrowse(intent)
			}
		case "r":
			if m.failedBrowse != nil {
				return m, m.startBrowse(*m.failedBrowse)
			}
			if m.tablesLoadFailed {
				return m, m.startTableDiscovery()
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
			m.cancelRequest()
			m.editor.cancelOperations()
			return m, tea.Quit
		}
		if key.Type == tea.KeyEsc {
			m.editor.cancelOperations()
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

func inspectorFieldIndices(columns []table.Column, query string) []int {
	query = strings.ToLower(strings.TrimSpace(query))
	indices := make([]int, 0, len(columns))
	for index, column := range columns {
		if query == "" || strings.Contains(strings.ToLower(column.Title), query) {
			indices = append(indices, index)
		}
	}
	return indices
}

func prettyInspectorValue(value, dataType string) string {
	if value == "NULL" {
		return "NULL"
	}
	if dataType == "json" || dataType == "jsonb" {
		var formatted bytes.Buffer
		if json.Indent(&formatted, []byte(value), "", "  ") == nil {
			return formatted.String()
		}
	}
	return value
}

func truncateInspectorValue(value string, width int) string {
	value = strings.ReplaceAll(value, "\n", " ↵ ")
	if width < 4 {
		return ""
	}
	if len(value) > width {
		return value[:width-3] + "..."
	}
	return value
}

func renderWorkspaceRowInspector(
	tableName string,
	columns []table.Column,
	structure []db.TableColumnInfo,
	row table.Row,
	rowNumber, selected int,
	query string,
	searching, expanded bool,
	copyStatus string,
	width, height int,
) string {
	indices := inspectorFieldIndices(columns, query)
	if selected >= len(indices) {
		selected = max(len(indices)-1, 0)
	}
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Row %d · %s · %d fields\n", rowNumber, tableName, len(columns)))
	if searching {
		output.WriteString("Find field: " + query + "\n")
	} else if query != "" {
		output.WriteString("Fields matching \"" + query + "\"\n")
	}
	if copyStatus != "" {
		output.WriteString(copyStatus + "\n")
	}
	output.WriteString("\n")
	if len(indices) == 0 {
		output.WriteString("No fields match this search.\n\nEsc returns to the data grid.")
		return output.String()
	}

	fieldIndex := indices[selected]
	dataType := "unknown"
	keyLabel := ""
	if fieldIndex < len(structure) {
		dataType = structure[fieldIndex].DataType
		if structure[fieldIndex].PrimaryKey {
			keyLabel = "  PK"
		}
	}
	value := ""
	if fieldIndex < len(row) {
		value = row[fieldIndex]
	}
	formattedValue := prettyInspectorValue(value, dataType)
	if expanded {
		output.WriteString(columns[fieldIndex].Title + "  ·  " + dataType + keyLabel + "\n")
		output.WriteString(strings.Repeat("─", max(min(width, 80), 20)) + "\n")
		output.WriteString(formattedValue + "\n\n")
		output.WriteString("Esc returns to the field list.")
		return output.String()
	}
	if width < 100 {
		visibleRows := max(height-6, 4)
		start := max(selected-visibleRows/2, 0)
		if start+visibleRows > len(indices) {
			start = max(len(indices)-visibleRows, 0)
		}
		for position := start; position < len(indices) && position < start+visibleRows; position++ {
			index := indices[position]
			marker := "  "
			if position == selected {
				marker = "> "
			}
			typeName := "unknown"
			if index < len(structure) {
				typeName = structure[index].DataType
			}
			valuePreview := ""
			if index < len(row) {
				valuePreview = truncateInspectorValue(row[index], max(width-len(columns[index].Title)-len(typeName)-7, 8))
			}
			output.WriteString(fmt.Sprintf("%s%-16s %-12s %s\n", marker, truncateInspectorValue(columns[index].Title, 16), truncateInspectorValue(typeName, 12), valuePreview))
		}
		output.WriteString("\nEnter opens the full selected value.")
		return output.String()
	}

	listWidth := min(max(width/3, 34), 52)
	previewWidth := max(width-listWidth-3, 20)
	visibleRows := max(height-6, 4)
	start := max(selected-visibleRows/2, 0)
	if start+visibleRows > len(indices) {
		start = max(len(indices)-visibleRows, 0)
	}
	listLines := make([]string, 0, visibleRows)
	for position := start; position < len(indices) && len(listLines) < visibleRows; position++ {
		index := indices[position]
		marker := "  "
		if position == selected {
			marker = "> "
		}
		typeName := "unknown"
		if index < len(structure) {
			typeName = structure[index].DataType
		}
		valuePreview := ""
		if index < len(row) {
			valuePreview = truncateInspectorValue(row[index], max(listWidth-len(columns[index].Title)-len(typeName)-7, 8))
		}
		listLines = append(listLines, fmt.Sprintf("%s%-16s %-12s %s", marker, truncateInspectorValue(columns[index].Title, 16), truncateInspectorValue(typeName, 12), valuePreview))
	}
	previewLines := strings.Split(columns[fieldIndex].Title+"  ·  "+dataType+keyLabel+"\n\n"+formattedValue, "\n")
	lineCount := max(len(listLines), len(previewLines))
	for index := 0; index < lineCount; index++ {
		left, right := "", ""
		if index < len(listLines) {
			left = truncateInspectorValue(listLines[index], listWidth)
		}
		if index < len(previewLines) {
			right = truncateInspectorValue(previewLines[index], previewWidth)
		}
		output.WriteString(fmt.Sprintf("%-*s │ %s\n", listWidth, left, right))
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
		footer := muted.Render("Esc workspace • Ctrl+A run • Ctrl+X cancel • Ctrl+R clear")
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
		if m.pendingBrowse != nil {
			explorerContent += "\n" + muted.Render("Opening "+m.pendingBrowse.tableName+"…") + "\n"
		}
		if m.notice != "" {
			explorerContent += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(m.notice) + "\n"
		}
		footer := muted.Render("↑/↓ select • Enter open • b close • c connections • q quit")
		if m.failedBrowse != nil || m.tablesLoadFailed {
			footer = muted.Render("r retry • ↑/↓ select • Enter open • b close • c connections • q quit")
		}
		return top + "\n" + separator + "\n\n" + explorerContent + "\n" + footer
	}

	context := muted.Render("public / ") + header.Render(m.selectedTable)
	tabs := "Data  Structure  Query"
	if m.tab == workspaceTabStructure {
		tabs = muted.Render("Data") + "  " + header.Render("Structure") + "  " + muted.Render("Query")
	} else {
		tabs = header.Render("Data") + "  " + muted.Render("Structure") + "  " + muted.Render("Query")
	}
	contentBody := context + "    " + tabs + "\n" + separator + "\n"
	if m.filterEditing {
		contentBody += m.filterInput.View() + "\n" + separator + "\n"
	} else if m.selectedTable != "" {
		direction := "ASC"
		if m.sortDescending {
			direction = "DESC"
		}
		toolbar := "Sort: " + m.sortColumn + " " + direction
		if m.filterColumn != "" {
			toolbar += "  •  Filter: " + m.filterColumn + " = " + m.filterValue
		}
		if m.keysetEnabled {
			toolbar += "  •  keyset pagination"
		}
		contentBody += muted.Render(toolbar) + "\n" + separator + "\n"
	}
	contentBody += "\n"
	if m.loading && m.pendingBrowse != nil {
		contentBody += muted.Render("Loading "+m.pendingBrowse.tableName+"…") + "\n"
	}
	if m.notice != "" {
		contentBody += lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(m.notice) + "\n"
	}
	contentBody += m.content.View()

	footerText := "↑/↓ row • Enter peek • / filter • s column • S direction • n/p page • b tables • e query • q quit"
	if m.width > 0 && m.width < 90 {
		footerText = "↑/↓ row • Enter peek • / filter • s sort • n/p page • b tables • q quit"
	}
	if m.failedBrowse != nil {
		footerText = "r retry • " + footerText
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
