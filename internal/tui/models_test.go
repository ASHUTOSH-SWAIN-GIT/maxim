package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	updated, _ = model.Update(workspaceTablesLoadedMsg{tables: []string{"customers", "orders"}})
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
	if !model.loading || command == nil {
		t.Fatal("opening a table did not start loading")
	}

	loaded := workspaceTableLoadedMsg{
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
	updated, _ = model.Update(workspaceTablesLoadedMsg{err: errors.New("database unavailable")})
	model = updated.(workspaceModel)
	if !strings.Contains(model.content.View(), "database unavailable") {
		t.Fatalf("workspace error missing: %q", model.content.View())
	}
	if view := model.View(); !strings.Contains(view, "app") || !strings.Contains(view, "connected") {
		t.Fatalf("compact layout lost database context: %q", view)
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
	if model.filterEditing || model.filterColumn != "status" || model.filterValue != "paid" || command == nil {
		t.Fatalf("filter was not applied: %#v", model)
	}

	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(workspaceModel)
	if model.sortColumn != "status" || command == nil {
		t.Fatalf("sort column did not advance: %q", model.sortColumn)
	}
	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	model = updated.(workspaceModel)
	if !model.sortDescending || command == nil {
		t.Fatal("sort direction did not reverse")
	}
}
