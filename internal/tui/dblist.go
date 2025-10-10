package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type dbListModel struct {
	dbNames []string
}

func (m dbListModel) Init() tea.Cmd {
	return nil
}

func (m dbListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m dbListModel) View() string {
	var b strings.Builder
	b.WriteString("Databases on Server:\n\n")
	for _, name := range m.dbNames {
		b.WriteString(fmt.Sprintf("- %s\n", name))
	}
	b.WriteString("\n(press 'q' to quit)")
	return b.String()
}

func RunDBList(dbNames []string) error {
	model := dbListModel{dbNames: dbNames}
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}
