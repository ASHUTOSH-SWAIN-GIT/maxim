package tui

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/lib/pq"
)

var workspaceConnectionSequence atomic.Uint64

type workspaceMode int

const (
	workspaceModeBrowse workspaceMode = iota
	workspaceModeEditor
)

const (
	workspaceMinimumWidth  = 60
	workspaceMinimumHeight = 18
	workspaceContentChrome = 10
)

type workspaceTab int

const (
	workspaceTabStructure workspaceTab = iota
	workspaceTabData
)

type workspaceTablesLoadedMsg struct {
	requestID uint64
	tables    []db.Relation
	err       error
}

type workspaceTableLoadedMsg struct {
	requestID        uint64
	relation         db.Relation
	structure        []db.TableColumnInfo
	columns          []table.Column
	rows             []db.DataRow
	offset           int
	hasNext          bool
	nextCursor       *db.TableCursor
	keysetEnabled    bool
	paginationReason string
	sortColumn       string
	sortDescending   bool
	filterColumn     string
	filterValue      string
	err              error
	rowKeys          []db.RowKey
}

type workspaceFullRowLoadedMsg struct {
	requestID uint64
	relation  db.Relation
	rowIndex  int
	row       db.DataRow
	err       error
}

type workspaceBrowseIntent struct {
	relation       db.Relation
	request        db.TableBrowseRequest
	cursorHistory  []*db.TableCursor
	closeNavigator bool
}

type workspaceModel struct {
	db                *sql.DB
	connectionID      uint64
	dbName            string
	connectionLabel   string
	changeConnection  bool
	mode              workspaceMode
	navigatorOpen     bool
	tab               workspaceTab
	tables            []db.Relation
	cursor            int
	selectedRelation  db.Relation
	structure         []db.TableColumnInfo
	columns           []table.Column
	rows              []db.DataRow
	rowKeys           []db.RowKey
	rowCursor         int
	rowPeek           bool
	rowPeekGridOffset int
	pageSize          int
	offset            int
	hasNext           bool
	nextCursor        *db.TableCursor
	keysetEnabled     bool
	paginationReason  string
	cursorHistory     []*db.TableCursor
	sortColumn        string
	sortDescending    bool
	filterColumn      string
	filterValue       string
	filterEditing     bool
	filterInput       textinput.Model
	loading           bool
	err               string
	width             int
	height            int
	content           viewport.Model
	editor            *sqlEditorModel
	nextRequestID     uint64
	activeRequestID   uint64
	requestContext    context.Context
	requestCancel     context.CancelFunc
	pendingBrowse     *workspaceBrowseIntent
	failedBrowse      *workspaceBrowseIntent
	notice            string
	tablesLoadFailed  bool
	helpOpen          bool
	disconnected      bool
	fullRowLoading    bool
	fullRowRequestID  uint64
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
		connectionID:  workspaceConnectionSequence.Add(1),
		navigatorOpen: true, tab: workspaceTabData,
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
		tables, err := db.GetRelationsContext(ctx, database)
		return workspaceTablesLoadedMsg{requestID: requestID, tables: tables, err: err}
	}
}

func loadWorkspaceBrowse(ctx context.Context, database *sql.DB, relation db.Relation, request db.TableBrowseRequest, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		page, err := db.BrowseRelationContext(ctx, database, relation, request)
		return workspaceTableLoadedMsg{
			requestID: requestID, relation: relation, structure: page.Structure, columns: page.Columns,
			rows: page.Rows, offset: request.Offset, hasNext: page.HasNext,
			nextCursor: page.NextCursor, keysetEnabled: page.KeysetEnabled,
			paginationReason: page.PaginationReason,
			sortColumn:       page.SortColumn, sortDescending: request.Descending,
			filterColumn: request.FilterColumn, filterValue: request.FilterValue, err: err,
			rowKeys: page.RowKeys,
		}
	}
}

func loadWorkspaceFullRow(database *sql.DB, relation db.Relation, key db.RowKey, rowIndex int, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		row, err := db.FetchRelationRowContext(context.Background(), database, relation, key, db.BrowseTimeout)
		return workspaceFullRowLoadedMsg{requestID: requestID, relation: relation, rowIndex: rowIndex, row: row, err: err}
	}
}

func (m *workspaceModel) startBrowse(intent workspaceBrowseIntent) tea.Cmd {
	if m.requestCancel != nil {
		m.requestCancel()
	}
	m.nextRequestID++
	m.fullRowRequestID++
	m.fullRowLoading = false
	m.activeRequestID = m.nextRequestID
	ctx, cancel := context.WithCancel(context.Background())
	m.requestContext = ctx
	m.requestCancel = cancel
	intent.cursorHistory = cloneCursorHistory(intent.cursorHistory)
	m.pendingBrowse = &intent
	m.failedBrowse = nil
	m.notice = ""
	m.loading = true
	return loadWorkspaceBrowse(ctx, m.db, intent.relation, intent.request, m.activeRequestID)
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
	m.err = ""
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

func (m workspaceModel) browseRequest(cursor *db.TableCursor, offset int) db.TableBrowseRequest {
	return db.TableBrowseRequest{
		Limit: m.pageSize, Offset: offset, SortColumn: m.sortColumn,
		Descending: m.sortDescending, FilterColumn: m.filterColumn,
		FilterValue: m.filterValue, ConnectionID: m.connectionID, Cursor: cursor.Clone(),
	}
}

func (m workspaceModel) browseIntent(cursor *db.TableCursor, offset int) workspaceBrowseIntent {
	return workspaceBrowseIntent{
		relation:      m.selectedRelation,
		request:       m.browseRequest(cursor, offset),
		cursorHistory: cloneCursorHistory(m.cursorHistory),
	}
}

func cloneCursorHistory(cursors []*db.TableCursor) []*db.TableCursor {
	cloned := make([]*db.TableCursor, len(cursors))
	for index, cursor := range cursors {
		cloned[index] = cursor.Clone()
	}
	return cloned
}

func formatWorkspaceFailure(err error) (message string, disconnected bool) {
	if errors.Is(err, context.Canceled) {
		return "Cancelled · database operation stopped.", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Timed out · the database did not respond before the deadline. Press r to retry.", false
	}
	var pqError *pq.Error
	if errors.As(err, &pqError) {
		if pqError.Code == "42501" {
			return "Permission denied · " + pqError.Message, false
		}
		if strings.HasPrefix(string(pqError.Code), "08") {
			return "Disconnected · " + pqError.Message + ".", true
		}
	}
	lowerMessage := strings.ToLower(err.Error())
	if errors.Is(err, driver.ErrBadConn) || strings.Contains(lowerMessage, "connection refused") ||
		strings.Contains(lowerMessage, "connection reset") || strings.Contains(lowerMessage, "server closed the connection") ||
		strings.Contains(lowerMessage, "broken pipe") {
		return "Disconnected · the database connection was lost. Press r to retry.", true
	}
	if strings.Contains(lowerMessage, "permission denied") {
		return "Permission denied · " + err.Error(), false
	}
	return "Load failed · " + err.Error(), false
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
			m.err, m.disconnected = formatWorkspaceFailure(message.err)
			m.tablesLoadFailed = true
			m.refreshContent()
			return m, nil
		}
		m.err = ""
		m.disconnected = false
		m.tablesLoadFailed = false
		m.tables = message.tables
		if len(m.tables) == 0 {
			m.content.SetContent("No user tables found.")
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
			m.notice, m.disconnected = formatWorkspaceFailure(message.err)
			m.failedBrowse = intent
		} else {
			m.err = ""
			m.notice = ""
			m.failedBrowse = nil
			m.disconnected = false
			m.selectedRelation = message.relation
			m.structure = message.structure
			m.columns = message.columns
			m.rows = message.rows
			m.rowKeys = message.rowKeys
			m.offset = message.offset
			m.hasNext = message.hasNext
			m.nextCursor = message.nextCursor
			m.keysetEnabled = message.keysetEnabled
			m.paginationReason = message.paginationReason
			m.sortColumn = message.sortColumn
			m.sortDescending = message.sortDescending
			m.filterColumn = message.filterColumn
			m.filterValue = message.filterValue
			if intent != nil {
				m.cursorHistory = cloneCursorHistory(intent.cursorHistory)
				if intent.closeNavigator {
					m.navigatorOpen = false
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
	case workspaceFullRowLoadedMsg:
		if message.requestID != m.fullRowRequestID {
			return m, nil
		}
		m.fullRowLoading = false
		if message.relation != m.selectedRelation || message.rowIndex != m.rowCursor || !m.rowPeek {
			return m, nil
		}
		if message.err != nil {
			m.notice, _ = formatWorkspaceFailure(message.err)
		} else if message.rowIndex >= 0 && message.rowIndex < len(m.rows) {
			m.rows[message.rowIndex] = message.row
			m.notice = ""
		}
		m.refreshContent()
		return m, nil
	}
	if key, ok := message.(tea.KeyMsg); ok {
		if m.helpOpen {
			if key.Type == tea.KeyEsc || key.Type == tea.KeyF1 || key.String() == "?" {
				m.helpOpen = false
			}
			return m, nil
		}
		if key.Type == tea.KeyF1 {
			m.helpOpen = true
			return m, nil
		}
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
				intent := m.browseIntent(nil, 0)
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
				m.content.SetYOffset(m.rowPeekGridOffset)
				return m, nil
			}
		case "?":
			m.helpOpen = true
			return m, nil
		case "ctrl+x":
			if m.loading {
				m.cancelRequest()
				m.notice = "Cancelled · database load stopped."
				m.refreshContent()
			}
			return m, nil
		case "c":
			m.cancelRequest()
			m.changeConnection = true
			return m, tea.Quit
		case "tab":
			if !m.navigatorOpen && m.selectedRelation.Name != "" {
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
			if m.rowPeek {
				m.rowPeek = false
				m.refreshContent()
				m.content.SetYOffset(m.rowPeekGridOffset)
			}
			m.navigatorOpen = !m.navigatorOpen
			return m, nil
		case "/":
			if !m.navigatorOpen && m.selectedRelation.Name != "" {
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
				intent := m.browseIntent(nil, 0)
				intent.request.SortColumn = m.structure[next].Name
				intent.cursorHistory = nil
				return m, m.startBrowse(intent)
			}
		case "S":
			if !m.navigatorOpen && m.selectedRelation.Name != "" {
				intent := m.browseIntent(nil, 0)
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
					relation:       m.tables[m.cursor],
					request:        db.TableBrowseRequest{Limit: m.pageSize},
					closeNavigator: true,
				}
				return m, m.startBrowse(intent)
			}
			if m.tab == workspaceTabData && len(m.rows) > 0 {
				m.rowPeekGridOffset = m.content.YOffset
				m.rowPeek = true
				m.refreshContent()
				for _, cell := range m.rows[m.rowCursor] {
					if cell.Truncated && m.rowCursor < len(m.rowKeys) && len(m.rowKeys[m.rowCursor]) > 0 {
						m.fullRowLoading = true
						m.fullRowRequestID++
						return m, loadWorkspaceFullRow(m.db, m.selectedRelation, m.rowKeys[m.rowCursor], m.rowCursor, m.fullRowRequestID)
					}
				}
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
			if !m.navigatorOpen && m.selectedRelation.Name != "" && m.hasNext {
				intent := m.browseIntent(nil, m.offset+m.pageSize)
				if m.keysetEnabled {
					intent.request.Cursor = m.nextCursor.Clone()
					intent.cursorHistory = append(intent.cursorHistory, m.nextCursor.Clone())
				}
				return m, m.startBrowse(intent)
			}
		case "p":
			if !m.navigatorOpen && m.selectedRelation.Name != "" && m.offset > 0 {
				nextOffset := m.offset - m.pageSize
				if nextOffset < 0 {
					nextOffset = 0
				}
				var cursor *db.TableCursor
				history := cloneCursorHistory(m.cursorHistory)
				if m.keysetEnabled && len(history) > 0 {
					history = history[:len(history)-1]
					if len(history) > 0 {
						cursor = history[len(history)-1].Clone()
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
			if m.rowPeek {
				m.content.LineUp(1)
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
			if m.rowPeek {
				m.content.LineDown(1)
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
	peekOffset := m.content.YOffset
	height := m.height - workspaceContentChrome
	if height < 4 {
		height = 4
	}
	width := m.width - 4
	if width < 20 {
		width = 20
	}
	m.content.Width = width
	m.content.Height = height
	m.filterInput.Width = max(min(m.width-4, 80), 20)
	if m.editor != nil {
		updated, _ := m.editor.Update(tea.WindowSizeMsg{Width: m.width, Height: max(m.height-4, 8)})
		editor := updated.(sqlEditorModel)
		m.editor = &editor
	}
	m.refreshContent()
	if m.rowPeek {
		m.content.SetYOffset(peekOffset)
	}
}

func (m *workspaceModel) refreshContent() {
	if m.err != "" {
		m.content.SetContent("Error\n\n" + m.err)
		return
	}
	if m.selectedRelation.Name == "" {
		m.content.SetContent("Choose a table to inspect its structure and data.")
		return
	}
	if m.tab == workspaceTabStructure {
		m.content.SetContent(renderWorkspaceStructure(m.selectedRelation.DisplayName(), m.structure))
	} else if m.rowPeek && len(m.rows) > 0 {
		m.content.SetContent(renderWorkspaceRowPeek(m.selectedRelation.DisplayName(), m.columns, m.rows[m.rowCursor], m.offset+m.rowCursor+1, m.content.Width))
	} else {
		m.content.SetContent(renderWorkspaceRows(m.selectedRelation.DisplayName(), m.columns, m.rows, m.offset, m.content.Width, m.rowCursor))
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

func renderWorkspaceRows(tableName string, columns []table.Column, rows []db.DataRow, offset, maxWidth, selectedRow int) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Data · %s · rows %d–%d\n\n", tableName, offset+1, offset+len(rows)))
	if len(rows) == 0 {
		output.WriteString("Empty result\n\nNo rows matched this table page and filter.")
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
			for columnIndex, cell := range row {
				if columnIndex >= len(columns) {
					break
				}
				value := cell.DisplayText(false)
				if cell.IsNull {
					value = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(value)
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
		for index, cell := range row {
			widths[index] = min(max(widths[index], ansi.StringWidth(cell.DisplayText(false))), columnLimit)
		}
	}
	writeRow := func(values []string, nulls []bool, prefix string) {
		output.WriteString(prefix)
		for index, value := range values {
			if ansi.StringWidth(value) > widths[index] {
				value = ansi.Truncate(value, widths[index], "...")
			}
			padding := strings.Repeat(" ", max(widths[index]-ansi.StringWidth(value), 0))
			if index < len(nulls) && nulls[index] {
				value = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(value)
			}
			output.WriteString(value + padding)
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
	writeRow(headings, nil, "  ")
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
		values := make([]string, len(row))
		nulls := make([]bool, len(row))
		for index, cell := range row {
			values[index] = cell.DisplayText(false)
			nulls[index] = cell.IsNull
		}
		writeRow(values, nulls, prefix)
		if rowIndex < len(rows)-1 {
			output.WriteString(divider)
		}
	}
	return output.String()
}

func renderWorkspaceRowPeek(tableName string, columns []table.Column, row db.DataRow, rowNumber, width int) string {
	var output strings.Builder
	hasTruncated := false
	output.WriteString(fmt.Sprintf("Row %d · %s\n\n", rowNumber, tableName))
	labelWidth := 0
	for _, column := range columns {
		labelWidth = max(labelWidth, ansi.StringWidth(column.Title))
	}
	labelWidth = min(labelWidth, min(24, max(width/3, 12)))
	valueWidth := max(width-labelWidth-2, 8)
	for index, cell := range row {
		if index >= len(columns) {
			break
		}
		hasTruncated = hasTruncated || cell.Truncated
		label := ansi.Truncate(columns[index].Title, labelWidth, "…")
		labelPadding := strings.Repeat(" ", max(labelWidth-ansi.StringWidth(label), 0))
		value := cell.DisplayText(true)
		if cell.IsNull {
			value = "SQL NULL"
		}
		wrappedLines := strings.Split(ansi.Hardwrap(value, valueWidth, true), "\n")
		if len(wrappedLines) == 0 {
			wrappedLines = []string{""}
		}
		output.WriteString(label + labelPadding + "  " + wrappedLines[0] + "\n")
		indent := strings.Repeat(" ", labelWidth+2)
		for _, line := range wrappedLines[1:] {
			output.WriteString(indent + line + "\n")
		}
	}
	if hasTruncated {
		output.WriteString("\nLoading the complete row from its primary key…\n")
	}
	output.WriteString("\nEsc returns to the data grid.")
	return output.String()
}

func renderWorkspaceHelp(m workspaceModel) string {
	var title string
	var commands []string
	switch {
	case m.mode == workspaceModeEditor:
		title = "Query help"
		commands = []string{
			"Ctrl+A  Run all SQL in the editor",
			"Ctrl+X  Cancel the running query",
			"Ctrl+R  Clear the result panel",
			"Tab     Cycle autocomplete suggestions",
			"Enter   Accept a suggestion",
			"Esc     Return to the database workspace",
			"F1      Open or close this help while editing",
		}
	case m.filterEditing:
		title = "Filter help"
		commands = []string{
			"Type a filter as column=value",
			"Enter   Apply the filter",
			"Esc     Cancel without changing the current filter",
			"F1      Open or close this help while typing",
		}
	case m.navigatorOpen:
		title = "Table navigator help"
		commands = []string{
			"↑/↓ or j/k  Select a table",
			"Enter       Open the selected table",
			"b           Close the navigator",
			"r           Retry a failed load",
			"Ctrl+X      Cancel an active load",
			"c           Choose another connection",
			"q           Quit Maxim",
			"?           Open or close this help",
		}
	case m.rowPeek:
		title = "Row peek help"
		commands = []string{
			"↑/↓ or j/k  Scroll one line",
			"PgUp/PgDn   Scroll one page",
			"Esc         Return to the same grid row",
			"b           Open the table navigator",
			"?           Open or close this help",
		}
	case m.tab == workspaceTabStructure:
		title = "Structure help"
		commands = []string{
			"Tab or →  Show table data",
			"↑/↓        Scroll column details",
			"b          Open the table navigator",
			"e          Open Query",
			"?          Open or close this help",
		}
	default:
		title = "Data help"
		commands = []string{
			"↑/↓ or j/k  Select a row",
			"Enter       Peek at the selected row",
			"/           Filter with column=value",
			"s / S       Change sort column / direction",
			"n / p       Next / previous page",
			"Tab or ←    Show table structure",
			"b           Open the table navigator",
			"e           Open Query",
			"Ctrl+X      Cancel an active load",
			"?           Open or close this help",
		}
	}
	return title + "\n\n" + strings.Join(commands, "\n")
}

func (m workspaceModel) View() string {
	accent := lipgloss.Color("6")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	if m.width > 0 && (m.width < workspaceMinimumWidth || m.height < workspaceMinimumHeight) {
		message := fmt.Sprintf(
			"Maxim needs a little more room\n\nCurrent: %d×%d\nMinimum: %d×%d\n\nResize the terminal to continue.\nCtrl+C quits.",
			m.width, m.height, workspaceMinimumWidth, workspaceMinimumHeight,
		)
		return clipWorkspaceView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, message), m.width, m.height)
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("● connected")
	if m.disconnected {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("● disconnected")
	}
	top := header.Render("MAXIM") + "  " + m.dbName
	if m.width >= 100 {
		top += "  " + muted.Render(m.connectionLabel)
	}
	top += "  " + status
	top = ansi.Truncate(top, max(m.width, 1), "…")
	separatorWidth := max(m.width, 30)
	separator := muted.Render(strings.Repeat("─", separatorWidth))
	if m.helpOpen {
		help := renderWorkspaceHelp(m)
		footer := muted.Render("Esc close help • F1 close help")
		return clipWorkspaceView(top+"\n"+separator+"\n"+help+"\n"+footer, m.width, m.height)
	}

	if m.mode == workspaceModeEditor && m.editor != nil {
		footer := muted.Render("Esc workspace • Ctrl+A run • Ctrl+X cancel • Ctrl+R clear • F1 help")
		return clipWorkspaceView(top+"\n"+separator+"\n"+header.Render("Query")+"\n"+m.editor.View()+"\n"+footer, m.width, m.height)
	}

	if m.navigatorOpen {
		explorerContent := header.Render("Choose a table") + "  " + muted.Render("all schemas") + "\n\n"
		if m.loading && len(m.tables) == 0 {
			explorerContent += muted.Render("Loading · discovering tables…")
		} else if len(m.tables) == 0 {
			explorerContent += muted.Render("Empty database · no user tables found")
		} else {
			availableRows := m.height - 10
			if m.pendingBrowse != nil {
				availableRows -= 2
			}
			if m.notice != "" {
				availableRows -= 2
			}
			availableRows = max(availableRows, 1)
			start := max(m.cursor-availableRows/2, 0)
			if start+availableRows > len(m.tables) {
				start = max(len(m.tables)-availableRows, 0)
			}
			end := min(start+availableRows, len(m.tables))
			if start > 0 {
				explorerContent += muted.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n"
			}
			for index := start; index < end; index++ {
				tableName := m.tables[index].DisplayName()
				prefix := "  "
				style := muted
				if index == m.cursor {
					prefix = "> "
					style = lipgloss.NewStyle().Foreground(accent).Bold(true)
				}
				tableName = ansi.Truncate(tableName, max(m.width-4, 1), "…")
				explorerContent += prefix + style.Render(tableName) + "\n"
			}
			if end < len(m.tables) {
				explorerContent += muted.Render(fmt.Sprintf("  ↓ %d more", len(m.tables)-end)) + "\n"
			}
		}
		if m.pendingBrowse != nil {
			explorerContent += "\n" + muted.Render("Opening "+m.pendingBrowse.relation.DisplayName()+"…") + "\n"
		}
		if m.notice != "" {
			explorerContent += "\n" + renderWorkspaceNotice(m.notice) + "\n"
		} else if m.err != "" {
			explorerContent += "\n" + renderWorkspaceNotice(m.err) + "\n"
		}
		footer := muted.Render("↑/↓ select • Enter open • b close • c connections • ? help • q quit")
		if m.failedBrowse != nil || m.tablesLoadFailed {
			footer = muted.Render("r retry • ↑/↓ select • Enter open • b close • c connections • ? help • q quit")
		}
		return clipWorkspaceView(top+"\n"+separator+"\n\n"+explorerContent+"\n"+footer, m.width, m.height)
	}

	context := muted.Render(m.selectedRelation.Schema+" / ") + header.Render(m.selectedRelation.Name)
	tabs := "Data  Structure  Query"
	if m.tab == workspaceTabStructure {
		tabs = muted.Render("Data") + "  " + header.Render("Structure") + "  " + muted.Render("Query")
	} else {
		tabs = header.Render("Data") + "  " + muted.Render("Structure") + "  " + muted.Render("Query")
	}
	contentBody := ansi.Truncate(context+"    "+tabs, max(m.width, 1), "…") + "\n" + separator + "\n"
	if m.filterEditing {
		contentBody += m.filterInput.View() + "\n" + separator + "\n"
	} else if m.selectedRelation.Name != "" {
		direction := "ASC"
		if m.sortDescending {
			direction = "DESC"
		}
		toolbar := "Sort: " + m.sortColumn + " " + direction
		if m.filterColumn != "" {
			toolbar += "  •  Filter: " + m.filterColumn + " = " + m.filterValue
		}
		if m.paginationReason != "" {
			toolbar += "  •  " + m.paginationReason
		}
		contentBody += muted.Render(ansi.Truncate(toolbar, max(m.width, 1), "…")) + "\n" + separator + "\n"
	}
	contentBody += "\n"
	if m.loading && m.pendingBrowse != nil {
		contentBody += muted.Render("Loading · "+m.pendingBrowse.relation.DisplayName()+"…  Ctrl+X cancel") + "\n"
	}
	if m.notice != "" {
		contentBody += renderWorkspaceNotice(m.notice) + "\n"
	}
	contentBody += clipWorkspaceView(m.content.View(), m.width, m.content.Height)

	footerText := "↑/↓ row • Enter peek • / filter • s column • S direction • n/p page • b tables • e query • q quit"
	if m.filterEditing {
		footerText = "Enter apply filter • Esc cancel • F1 help"
	} else if m.rowPeek {
		footerText = "↑/↓ scroll • PgUp/PgDn page • Esc data grid • ? help • q quit"
	} else if m.width > 0 && m.width < 90 {
		footerText = "↑/↓ row • Enter peek • / filter • s sort • n/p page • b tables • ? help • q quit"
	} else {
		footerText += " • ? help"
	}
	if m.failedBrowse != nil {
		footerText = "r retry • " + footerText
	}
	footer := muted.Render(footerText)
	return clipWorkspaceView(top+"\n"+separator+"\n"+contentBody+"\n"+footer, m.width, m.height)
}

func clipWorkspaceView(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	if width > 0 {
		for index := range lines {
			lines[index] = ansi.Truncate(lines[index], width, "")
		}
	}
	return strings.Join(lines, "\n")
}

func renderWorkspaceNotice(message string) string {
	color := lipgloss.Color("1")
	if strings.HasPrefix(message, "Cancelled") || strings.HasPrefix(message, "Timed out") {
		color = lipgloss.Color("3")
	}
	return lipgloss.NewStyle().Foreground(color).Render(message)
}

func RunWorkspace(database *sql.DB, dbName, connectionLabel string) (bool, error) {
	program := tea.NewProgram(initialWorkspaceModel(database, dbName, connectionLabel), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}
	return finalModel.(workspaceModel).changeConnection, nil
}
