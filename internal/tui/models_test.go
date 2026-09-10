package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func key(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}

func TestMainMenuNavigationAndSelection(t *testing.T) {
	model := initialMainMenuModel()
	updated, _ := model.Update(key(tea.KeyDown))
	model = updated.(mainMenuModel)
	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", model.cursor)
	}
	updated, command := model.Update(key(tea.KeyEnter))
	model = updated.(mainMenuModel)
	if !model.done || command == nil {
		t.Fatalf("enter did not finish menu: %#v", model)
	}
	if model.View() != "" {
		t.Fatal("completed menu should render nothing")
	}
}

func TestMenusRespectBoundsAndQuit(t *testing.T) {
	tests := []struct {
		name   string
		update func(tea.Msg) (tea.Model, tea.Cmd)
		check  func(tea.Model) (cursor int, quitting bool)
	}{
		{
			name: "database operations",
			update: func(message tea.Msg) (tea.Model, tea.Cmd) {
				return initialDBOperationsModel("maxim").Update(message)
			},
			check: func(model tea.Model) (int, bool) {
				menu := model.(dbOperationsModel)
				return menu.cursor, menu.quitting
			},
		},
		{
			name: "connection type",
			update: func(message tea.Msg) (tea.Model, tea.Cmd) {
				return initialConnectTypeMenuModel().Update(message)
			},
			check: func(model tea.Model) (int, bool) {
				menu := model.(connectTypeMenuModel)
				return menu.cursor, menu.quitting
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, _ := test.update(key(tea.KeyUp))
			cursor, _ := test.check(model)
			if cursor != 0 {
				t.Fatalf("cursor moved above first item: %d", cursor)
			}
			model, command := test.update(key(tea.KeyEsc))
			_, quitting := test.check(model)
			if !quitting || command == nil {
				t.Fatal("escape did not quit menu")
			}
		})
	}
}

func TestTableListSelectionAndEmptyView(t *testing.T) {
	model := initialTableListModel([]string{"customers", "orders"})
	updated, _ := model.Update(key(tea.KeyDown))
	model = updated.(tableListModel)
	updated, _ = model.Update(key(tea.KeyEnter))
	model = updated.(tableListModel)
	if !model.done || model.tables[model.cursor] != "orders" {
		t.Fatalf("unexpected selection: %#v", model)
	}
	if view := initialTableListModel(nil).View(); !strings.Contains(view, "No tables found") {
		t.Fatalf("empty-state message missing: %q", view)
	}
}

func TestConnectionFormDefaultsAndNavigation(t *testing.T) {
	model := initialConnectFormModel()
	if model.Inputs[0].Value() != "localhost" || model.Inputs[1].Value() != "5432" || model.Inputs[5].Value() != "prefer" {
		t.Fatalf("unexpected connection defaults: host=%q port=%q ssl=%q",
			model.Inputs[0].Value(), model.Inputs[1].Value(), model.Inputs[5].Value())
	}
	if model.Inputs[3].EchoMode != textinput.EchoPassword {
		t.Fatal("password input is not masked")
	}

	updated, _ := model.Update(key(tea.KeyTab))
	model = updated.(ConnectFormModel)
	if model.focusIndex != 1 {
		t.Fatalf("tab focus = %d, want 1", model.focusIndex)
	}
	updated, _ = model.Update(key(tea.KeyShiftTab))
	model = updated.(ConnectFormModel)
	if model.focusIndex != 0 {
		t.Fatalf("shift-tab focus = %d, want 0", model.focusIndex)
	}
	updated, command := model.Update(key(tea.KeyEsc))
	model = updated.(ConnectFormModel)
	if !model.Quitting || command == nil {
		t.Fatal("escape did not cancel connection form")
	}
}

func TestFormFocusWraps(t *testing.T) {
	create := initialCreateFormModel()
	create.prevInput()
	if create.focusIndex != len(create.Inputs)-1 {
		t.Fatalf("create form focus did not wrap: %d", create.focusIndex)
	}

	admin := initialAdminFormModel()
	admin.prevInput()
	if admin.focusIndex != len(admin.Inputs)-1 {
		t.Fatalf("admin form focus did not wrap: %d", admin.focusIndex)
	}

	container := initialContainerFormModel()
	container.prevInput()
	if container.focusIndex != len(container.Inputs)-1 {
		t.Fatalf("container form focus did not wrap: %d", container.focusIndex)
	}
	if container.Inputs[4].EchoMode != textinput.EchoPassword {
		t.Fatal("container password input is not masked")
	}
}

func TestPasswordFormSubmitAndCancel(t *testing.T) {
	model := initialPasswordFormModel()
	model.Input.SetValue("secret")
	updated, command := model.Update(key(tea.KeyEnter))
	model = updated.(PasswordFormModel)
	if !model.done || command == nil || model.View() != "" {
		t.Fatal("enter did not submit password form")
	}

	cancelled := initialPasswordFormModel()
	updated, command = cancelled.Update(key(tea.KeyEsc))
	cancelled = updated.(PasswordFormModel)
	if !cancelled.Quitting || command == nil {
		t.Fatal("escape did not cancel password form")
	}
}

func TestQueryCacheSuggestionsAndFrequency(t *testing.T) {
	cache := NewQueryCache()
	cache.CacheTables([]string{"orders", "customers"})
	cache.CacheColumns([]string{"order_id", "customer_id"})
	cache.AddCommand("SELECT id FROM orders")
	cache.AddCommand("SELECT id FROM customers")
	cache.AddCommand("SELECT")

	suggestions := cache.GetSuggestions("ord")
	if !slices.Contains(suggestions, "orders") || !slices.Contains(suggestions, "ORDER") || !slices.Contains(suggestions, "order_id") {
		t.Fatalf("missing autocomplete suggestions: %v", suggestions)
	}
	mostUsed := cache.GetMostUsedCommands(1)
	if !slices.Equal(mostUsed, []string{"SELECT"}) {
		t.Fatalf("unexpected frequency result: %v", mostUsed)
	}
	if suggestions := cache.GetSuggestions(""); len(suggestions) != 0 {
		t.Fatalf("empty input returned suggestions: %v", suggestions)
	}
}

func TestSplitSQLStatements(t *testing.T) {
	input := "SELECT 'a;b'; INSERT INTO notes VALUES ('hello'); SELECT 3"
	statements := splitSQLStatements(input)
	if !slices.Equal(statements, []string{"SELECT 'a;b'", " INSERT INTO notes VALUES ('hello')", "SELECT 3"}) {
		t.Fatalf("unexpected statements: %#v", statements)
	}
}

func TestSQLEditorRunsQueriesAsynchronouslyAndIgnoresStaleResults(t *testing.T) {
	model := initialSQLEditorModel(nil, "app")
	defer model.cancelOperations()
	model.textarea.SetValue("SELECT 1")

	updated, command := model.Update(key(tea.KeyCtrlA))
	model = updated.(sqlEditorModel)
	if command == nil || !model.queryRunning || model.activeQueryID == 0 {
		t.Fatalf("query did not start asynchronously: %#v", model)
	}
	if model.textarea.Value() != "SELECT 1" {
		t.Fatal("query draft was cleared before execution completed")
	}

	activeID := model.activeQueryID
	updated, _ = model.Update(sqlEditorQueryFinishedMsg{requestID: activeID + 1, results: "stale"})
	model = updated.(sqlEditorModel)
	if !model.queryRunning || model.results == "stale" {
		t.Fatal("stale query result changed editor state")
	}

	updated, _ = model.Update(sqlEditorQueryFinishedMsg{requestID: activeID, results: "done"})
	model = updated.(sqlEditorModel)
	if model.queryRunning || model.results != "done" || model.textarea.Value() != "" {
		t.Fatalf("current query result was not committed: %#v", model)
	}
}

func TestSQLEditorCancelsRunningQueryAndLoadsAutocomplete(t *testing.T) {
	model := initialSQLEditorModel(nil, "app")
	defer model.cancelOperations()
	updated, _ := model.Update(sqlEditorSchemaLoadedMsg{
		columns: []string{"customer_id"},
		tables:  []string{"customers"},
	})
	model = updated.(sqlEditorModel)
	if suggestions := model.queryCache.GetSuggestions("cust"); !slices.Contains(suggestions, "customer_id") || !slices.Contains(suggestions, "customers") {
		t.Fatalf("schema was not added to autocomplete: %v", suggestions)
	}

	model.textarea.SetValue("SELECT pg_sleep(10)")
	updated, _ = model.Update(key(tea.KeyCtrlA))
	model = updated.(sqlEditorModel)
	updated, _ = model.Update(key(tea.KeyCtrlX))
	model = updated.(sqlEditorModel)
	if model.queryRunning || model.activeQueryID != 0 || model.results != "Query cancelled." {
		t.Fatalf("query cancellation was not reflected in the editor: %#v", model)
	}
}

func TestConnectionManagerActions(t *testing.T) {
	tests := []struct {
		name       string
		key        tea.KeyMsg
		cursor     int
		wantAction ConnectionAction
		wantName   string
	}{
		{"connect", key(tea.KeyEnter), 0, ConnectionActionConnect, "local"},
		{"new shortcut", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, 0, ConnectionActionNew, ""},
		{"edit", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, 0, ConnectionActionEdit, "local"},
		{"rename", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, 0, ConnectionActionRename, "local"},
		{"delete", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, 0, ConnectionActionDelete, "local"},
		{"new item", key(tea.KeyEnter), 2, ConnectionActionNew, ""},
		{"quit", key(tea.KeyEsc), 0, ConnectionActionQuit, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := initialConnectionManagerModel([]string{"local", "production"})
			model.cursor = test.cursor
			updated, command := model.Update(test.key)
			model = updated.(connectionManagerModel)
			if command == nil || !model.done {
				t.Fatal("action did not finish the connection manager")
			}
			if model.result.Action != test.wantAction || model.result.Name != test.wantName {
				t.Fatalf("result = %#v, want action=%q name=%q", model.result, test.wantAction, test.wantName)
			}
		})
	}
}

func TestConnectionManagerNavigationAndView(t *testing.T) {
	model := initialConnectionManagerModel([]string{"local"})
	if view := model.View(); !strings.Contains(view, "local") || !strings.Contains(view, "+ Add connection") {
		t.Fatalf("connection manager view is incomplete: %q", view)
	}
	updated, _ := model.Update(key(tea.KeyDown))
	model = updated.(connectionManagerModel)
	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want add-connection item", model.cursor)
	}
	updated, _ = model.Update(key(tea.KeyDown))
	if updated.(connectionManagerModel).cursor != 1 {
		t.Fatal("cursor moved past add-connection item")
	}
}

func TestNameAndConfirmationModels(t *testing.T) {
	name := initialNameFormModel("Rename", "old")
	name.input.SetValue("   ")
	updated, command := name.Update(key(tea.KeyEnter))
	name = updated.(nameFormModel)
	if command != nil || name.done || name.error == "" {
		t.Fatal("empty name should remain open with an error")
	}
	name.input.SetValue("new")
	updated, command = name.Update(key(tea.KeyEnter))
	name = updated.(nameFormModel)
	if command == nil || !name.done {
		t.Fatal("valid name did not submit")
	}

	confirmation := confirmationModel{prompt: "Delete?"}
	updated, command = confirmation.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	confirmation = updated.(confirmationModel)
	if command == nil || !confirmation.done || !confirmation.confirmed {
		t.Fatal("yes did not confirm operation")
	}
}

func TestConnectionFormUsesSavedDefaults(t *testing.T) {
	model := initialConnectFormModelWithDefaults(ConnectResult{
		Host: "db.example.com", Port: "6432", User: "maxim",
		DBName: "app", SSLMode: "verify-full",
	})
	values := []string{
		model.Inputs[0].Value(), model.Inputs[1].Value(), model.Inputs[2].Value(),
		model.Inputs[4].Value(), model.Inputs[5].Value(),
	}
	if !slices.Equal(values, []string{"db.example.com", "6432", "maxim", "app", "verify-full"}) {
		t.Fatalf("saved defaults not applied: %v", values)
	}
}

func TestWorkspaceNavigationAndTableContent(t *testing.T) {
	model := initialWorkspaceModel(nil, "maxim_demo", "maxim@localhost:5432")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model = updated.(workspaceModel)
	updated, _ = model.Update(workspaceTablesLoadedMsg{requestID: 1, tables: []string{"customers", "orders"}})
	model = updated.(workspaceModel)
	if model.loading || len(model.tables) != 2 {
		t.Fatalf("tables did not load: %#v", model)
	}

	updated, _ = model.Update(key(tea.KeyDown))
	model = updated.(workspaceModel)
	if model.cursor != 1 {
		t.Fatalf("table cursor = %d, want 1", model.cursor)
	}
	updated, command := model.Update(key(tea.KeyEnter))
	model = updated.(workspaceModel)
	if !model.loading || command == nil || !model.navigatorOpen || model.selectedTable != "" {
		t.Fatal("opening a table changed navigation state before loading succeeded")
	}

	loaded := workspaceTableLoadedMsg{
		requestID: 2,
		tableName: "orders",
		structure: []db.TableColumnInfo{{Name: "id", DataType: "bigint", PrimaryKey: true}},
		columns:   []table.Column{{Title: "id"}, {Title: "status"}},
		rows:      []table.Row{{"1", "paid"}},
	}
	updated, _ = model.Update(loaded)
	model = updated.(workspaceModel)
	if model.selectedTable != "orders" || model.navigatorOpen || !strings.Contains(model.content.View(), "Data · orders") {
		t.Fatalf("table data did not open: %q", model.content.View())
	}
	updated, _ = model.Update(key(tea.KeyEnter))
	model = updated.(workspaceModel)
	if !model.rowPeek || !strings.Contains(model.content.View(), "Row 1 · orders") || !strings.Contains(model.content.View(), "status  paid") {
		t.Fatalf("row peek did not open: %q", model.content.View())
	}
	updated, _ = model.Update(key(tea.KeyEsc))
	model = updated.(workspaceModel)
	if model.rowPeek {
		t.Fatal("row peek did not close")
	}

	updated, _ = model.Update(key(tea.KeyTab))
	model = updated.(workspaceModel)
	if model.focus != workspaceFocusContent || model.tab != workspaceTabStructure || !strings.Contains(model.content.View(), "Structure · orders") {
		t.Fatalf("structure tab did not activate: focus=%d tab=%d content=%q", model.focus, model.tab, model.content.View())
	}
	if view := model.View(); !strings.Contains(view, "MAXIM") || !strings.Contains(view, "connected") {
		t.Fatalf("workspace chrome missing: %q", view)
	}
}

func TestWorkspaceErrorsAndResponsiveLayout(t *testing.T) {
	model := initialWorkspaceModel(nil, "app", "user@host:5432")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 70, Height: 24})
	model = updated.(workspaceModel)
	updated, _ = model.Update(workspaceTablesLoadedMsg{requestID: 1, err: errors.New("database unavailable")})
	model = updated.(workspaceModel)
	if !strings.Contains(model.content.View(), "database unavailable") {
		t.Fatalf("workspace error missing: %q", model.content.View())
	}
	if view := model.View(); !strings.Contains(view, "app") || !strings.Contains(view, "connected") {
		t.Fatalf("compact layout lost database context: %q", view)
	}
}

func TestWorkspaceSupportsEightyByTwentyFourAndSmallTerminalFallback(t *testing.T) {
	model := initialWorkspaceModel(nil, "app", "user@host:5432")
	model.tables = make([]string, 40)
	for index := range model.tables {
		model.tables[index] = fmt.Sprintf("table_%02d", index)
	}
	model.cursor = 20
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(workspaceModel)
	assertViewFits(t, model.View(), 80, 24)
	if !strings.Contains(model.View(), "table_20") {
		t.Fatal("selected table was clipped from the navigator")
	}

	model.navigatorOpen = false
	model.selectedTable = "orders"
	model.structure = []db.TableColumnInfo{{Name: "id", PrimaryKey: true}, {Name: "status"}}
	model.columns = []table.Column{{Title: "id"}, {Title: "status"}}
	model.rows = []table.Row{{"1", "paid"}, {"2", "shipped"}}
	model.refreshContent()
	assertViewFits(t, model.View(), 80, 24)
	model.tab = workspaceTabStructure
	model.refreshContent()
	assertViewFits(t, model.View(), 80, 24)
	model.tab = workspaceTabData
	model.rowPeek = true
	model.refreshContent()
	assertViewFits(t, model.View(), 80, 24)
	model.rowPeek = false
	model.filterEditing = true
	assertViewFits(t, model.View(), 80, 24)
	model.filterEditing = false
	model.loading = true
	model.pendingBrowse = &workspaceBrowseIntent{tableName: "orders"}
	model.notice = strings.Repeat("database error ", 20)
	assertViewFits(t, model.View(), 80, 24)
	model.loading = false
	model.pendingBrowse = nil
	model.notice = ""

	editor := initialSQLEditorModel(nil, "app")
	defer editor.cancelOperations()
	model.editor = &editor
	model.mode = workspaceModeEditor
	model.resizeContent()
	assertViewFits(t, model.View(), 80, 24)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: workspaceMinimumWidth, Height: workspaceMinimumHeight})
	model = updated.(workspaceModel)
	assertViewFits(t, model.View(), workspaceMinimumWidth, workspaceMinimumHeight)
	if strings.Contains(model.View(), "needs a little more room") {
		t.Fatal("minimum supported terminal incorrectly showed the fallback")
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 59, Height: 17})
	model = updated.(workspaceModel)
	view := model.View()
	assertViewFits(t, view, 59, 17)
	if !strings.Contains(view, "Minimum: 60×18") {
		t.Fatalf("minimum-size guidance missing: %q", view)
	}
}

func TestWorkspaceRowPeekWrapsScrollsAndRestoresGridPosition(t *testing.T) {
	model := initialWorkspaceModel(nil, "app", "user@host:5432")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(workspaceModel)
	model.navigatorOpen = false
	model.selectedTable = "events"
	for index := 0; index < 12; index++ {
		model.columns = append(model.columns, table.Column{Title: fmt.Sprintf("long_column_%02d", index)})
	}
	longValue := strings.Repeat("a wide unicode value 世界 ", 6) + "\nsecond database line"
	row := make(table.Row, len(model.columns))
	for index := range row {
		row[index] = longValue
	}
	for index := 0; index < 30; index++ {
		model.rows = append(model.rows, append(table.Row(nil), row...))
	}
	model.rowCursor = 10
	model.refreshContent()
	gridOffset := model.content.YOffset
	if gridOffset == 0 {
		t.Fatal("fixture did not produce a scrolled data grid")
	}

	updated, _ = model.Update(key(tea.KeyEnter))
	model = updated.(workspaceModel)
	if !model.rowPeek || model.rowCursor != 10 || model.content.YOffset != 0 {
		t.Fatalf("row peek did not open at its own scroll origin: %#v", model)
	}
	peek := renderWorkspaceRowPeek("events", model.columns, row, 11, model.content.Width)
	if !strings.Contains(peek, "second database line") {
		t.Fatal("multiline value was not retained")
	}
	for _, line := range strings.Split(peek, "\n") {
		if width := ansi.StringWidth(line); width > model.content.Width {
			t.Fatalf("row-peek line width = %d, viewport width = %d: %q", width, model.content.Width, line)
		}
	}

	updated, _ = model.Update(key(tea.KeyDown))
	model = updated.(workspaceModel)
	if model.content.YOffset == 0 || model.rowCursor != 10 {
		t.Fatal("row-peek scrolling moved the selected database row")
	}
	peekOffset := model.content.YOffset
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	model = updated.(workspaceModel)
	if model.content.YOffset != peekOffset {
		t.Fatalf("resize reset row-peek scroll: got %d, want %d", model.content.YOffset, peekOffset)
	}
	updated, _ = model.Update(key(tea.KeyEsc))
	model = updated.(workspaceModel)
	if model.rowPeek || model.rowCursor != 10 || model.content.YOffset != gridOffset {
		t.Fatalf("closing row peek did not restore the grid position: row=%d offset=%d, want row=10 offset=%d", model.rowCursor, model.content.YOffset, gridOffset)
	}
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()
	if got := lipgloss.Width(view); got > width {
		t.Fatalf("view width = %d, terminal width = %d\n%s", got, width, view)
	}
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("view height = %d, terminal height = %d\n%s", got, height, view)
	}
}

func TestWorkspaceFilterAndSortControls(t *testing.T) {
	model := initialWorkspaceModel(nil, "app", "user@host:5432")
	model.navigatorOpen = false
	model.selectedTable = "orders"
	model.structure = []db.TableColumnInfo{{Name: "id", PrimaryKey: true}, {Name: "status"}}
	model.sortColumn = "id"

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(workspaceModel)
	if !model.filterEditing {
		t.Fatal("filter editor did not open")
	}
	model.filterInput.SetValue("status=paid")
	updated, command := model.Update(key(tea.KeyEnter))
	model = updated.(workspaceModel)
	if model.filterEditing || model.filterColumn != "" || model.pendingBrowse == nil || model.pendingBrowse.request.FilterColumn != "status" || command == nil {
		t.Fatalf("filter was not staged: %#v", model)
	}
	updated, _ = model.Update(workspaceTableLoadedMsg{
		requestID: 2, tableName: "orders", structure: model.structure,
		columns: []table.Column{{Title: "id"}, {Title: "status"}}, rows: []table.Row{{"1", "paid"}},
		sortColumn: "id", filterColumn: "status", filterValue: "paid",
	})
	model = updated.(workspaceModel)
	if model.filterColumn != "status" || model.filterValue != "paid" {
		t.Fatalf("successful filter was not committed: %#v", model)
	}

	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(workspaceModel)
	if model.sortColumn != "id" || model.pendingBrowse == nil || model.pendingBrowse.request.SortColumn != "status" || command == nil {
		t.Fatalf("sort column was not staged: %#v", model)
	}
	updated, _ = model.Update(workspaceTableLoadedMsg{
		requestID: 3, tableName: "orders", structure: model.structure,
		columns: model.columns, rows: model.rows, sortColumn: "status",
		filterColumn: "status", filterValue: "paid",
	})
	model = updated.(workspaceModel)
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	model = updated.(workspaceModel)
	if model.sortDescending || model.pendingBrowse == nil || !model.pendingBrowse.request.Descending || command == nil {
		t.Fatal("sort direction was not staged")
	}
}

func TestWorkspaceRequestLifecycle(t *testing.T) {
	model := initialWorkspaceModel(nil, "app", "user@host:5432")
	model.activeRequestID = 3
	model.nextRequestID = 3
	model.selectedTable = "current"
	model.loading = true

	updated, _ := model.Update(workspaceTableLoadedMsg{
		requestID: 2, tableName: "stale", rows: []table.Row{{"old"}},
	})
	model = updated.(workspaceModel)
	if model.selectedTable != "current" || !model.loading {
		t.Fatalf("stale response changed workspace state: %#v", model)
	}

	model.filterEditing = true
	updated, _ = model.Update(workspaceTableLoadedMsg{
		requestID: 3, tableName: "latest", columns: []table.Column{{Title: "id"}}, rows: []table.Row{{"1"}},
	})
	model = updated.(workspaceModel)
	if model.selectedTable != "latest" || model.loading || !model.filterEditing || model.activeRequestID != 0 {
		t.Fatalf("current response was not handled while filter was open: %#v", model)
	}

	cancelled := false
	model.requestCancel = func() { cancelled = true }
	command := model.startBrowse(workspaceBrowseIntent{tableName: "latest", request: db.TableBrowseRequest{Limit: 100}})
	if !cancelled || command == nil || model.activeRequestID != 4 || !model.loading {
		t.Fatalf("replacement request did not cancel and advance: cancelled=%t id=%d loading=%t", cancelled, model.activeRequestID, model.loading)
	}
	model.filterEditing = false
	model.mode = workspaceModeEditor
	updated, _ = model.Update(workspaceTableLoadedMsg{
		requestID: 4, tableName: "editor-result", columns: []table.Column{{Title: "id"}}, rows: []table.Row{{"2"}},
	})
	model = updated.(workspaceModel)
	if model.selectedTable != "editor-result" || model.loading {
		t.Fatalf("current response was not handled while editor was open: %#v", model)
	}

	command = model.startBrowse(workspaceBrowseIntent{tableName: "editor-result", request: db.TableBrowseRequest{Limit: 100}})
	if command == nil || model.activeRequestID != 5 {
		t.Fatalf("next request did not advance after completion: id=%d", model.activeRequestID)
	}
	model.cancelRequest()
	if model.activeRequestID != 0 || model.loading || model.requestCancel != nil {
		t.Fatalf("request cancellation did not clear lifecycle state: %#v", model)
	}
}

func TestWorkspaceFailedPagePreservesCommittedStateAndRetries(t *testing.T) {
	model := initialWorkspaceModel(nil, "app", "user@host:5432")
	model.navigatorOpen = false
	model.selectedTable = "orders"
	model.columns = []table.Column{{Title: "id"}}
	model.rows = []table.Row{{"100"}}
	model.offset = 0
	model.hasNext = true
	model.nextCursor = "100"
	model.keysetEnabled = true
	model.sortColumn = "id"
	model.cursorHistory = []string{}
	model.refreshContent()

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updated.(workspaceModel)
	if command == nil || model.offset != 0 || len(model.cursorHistory) != 0 || model.pendingBrowse == nil {
		t.Fatalf("next page changed committed state before success: %#v", model)
	}

	updated, _ = model.Update(workspaceTableLoadedMsg{requestID: 2, tableName: "orders", offset: 100, err: errors.New("timeout")})
	model = updated.(workspaceModel)
	if model.offset != 0 || len(model.cursorHistory) != 0 || model.rows[0][0] != "100" || model.failedBrowse == nil {
		t.Fatalf("failed page replaced committed state: %#v", model)
	}
	if !strings.Contains(model.content.View(), "100") || !strings.Contains(model.View(), "Load failed") {
		t.Fatalf("failure did not preserve data and show notice: %q", model.View())
	}

	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = updated.(workspaceModel)
	if command == nil || model.activeRequestID != 3 || model.pendingBrowse == nil || model.pendingBrowse.request.Cursor != "100" {
		t.Fatalf("retry did not restore failed request: %#v", model)
	}
}
