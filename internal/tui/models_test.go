package tui

import (
	"slices"
	"strings"
	"testing"

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
