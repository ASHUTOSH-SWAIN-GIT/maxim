package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConnectionAction string

const (
	ConnectionActionConnect ConnectionAction = "connect"
	ConnectionActionNew     ConnectionAction = "new"
	ConnectionActionEdit    ConnectionAction = "edit"
	ConnectionActionRename  ConnectionAction = "rename"
	ConnectionActionDelete  ConnectionAction = "delete"
	ConnectionActionQuit    ConnectionAction = "quit"
)

type ConnectionManagerResult struct {
	Action ConnectionAction
	Name   string
}

type connectionManagerModel struct {
	connections []string
	cursor      int
	result      ConnectionManagerResult
	done        bool
}

func initialConnectionManagerModel(connections []string) connectionManagerModel {
	return connectionManagerModel{connections: connections}
}

func (m connectionManagerModel) Init() tea.Cmd { return nil }

func (m connectionManagerModel) selectedName() string {
	if m.cursor >= 0 && m.cursor < len(m.connections) {
		return m.connections[m.cursor]
	}
	return ""
}

func (m connectionManagerModel) finish(action ConnectionAction, name string) (tea.Model, tea.Cmd) {
	m.result = ConnectionManagerResult{Action: action, Name: name}
	m.done = true
	return m, tea.Quit
}

func (m connectionManagerModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "ctrl+c", "q", "esc":
		return m.finish(ConnectionActionQuit, "")
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.connections) {
			m.cursor++
		}
	case "enter":
		if name := m.selectedName(); name != "" {
			return m.finish(ConnectionActionConnect, name)
		}
		return m.finish(ConnectionActionNew, "")
	case "n":
		return m.finish(ConnectionActionNew, "")
	case "e":
		if name := m.selectedName(); name != "" {
			return m.finish(ConnectionActionEdit, name)
		}
	case "r":
		if name := m.selectedName(); name != "" {
			return m.finish(ConnectionActionRename, name)
		}
	case "d":
		if name := m.selectedName(); name != "" {
			return m.finish(ConnectionActionDelete, name)
		}
	}
	return m, nil
}

func (m connectionManagerModel) View() string {
	if m.done {
		return ""
	}

	var view strings.Builder
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	view.WriteString(header.Render("Saved connections"))
	view.WriteString("\n\n")
	for index, name := range m.connections {
		prefix := "  "
		style := muted
		if index == m.cursor {
			prefix = "> "
			style = selected
		}
		view.WriteString(prefix + style.Render(name) + "\n")
	}
	newPrefix := "  "
	newStyle := muted
	if m.cursor == len(m.connections) {
		newPrefix = "> "
		newStyle = selected
	}
	view.WriteString(newPrefix + newStyle.Render("+ Add connection") + "\n")
	view.WriteString("\n")
	view.WriteString(muted.Render("Enter connect • n new • e edit • r rename • d delete • Esc quit"))
	return view.String()
}

func RunConnectionManager(connections []string) (ConnectionManagerResult, error) {
	program := tea.NewProgram(initialConnectionManagerModel(connections))
	finalModel, err := program.Run()
	if err != nil {
		return ConnectionManagerResult{}, err
	}
	return finalModel.(connectionManagerModel).result, nil
}

type nameFormModel struct {
	input    textinput.Model
	title    string
	done     bool
	quitting bool
	error    string
}

func initialNameFormModel(title, currentName string) nameFormModel {
	input := textinput.New()
	input.SetValue(currentName)
	input.CharLimit = 255
	input.Focus()
	return nameFormModel{input: input, title: title}
}

func (m nameFormModel) Init() tea.Cmd { return textinput.Blink }

func (m nameFormModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.error = "Name cannot be empty"
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}
	var command tea.Cmd
	m.input, command = m.input.Update(message)
	return m, command
}

func (m nameFormModel) View() string {
	if m.done || m.quitting {
		return ""
	}
	view := fmt.Sprintf("%s\n\n%s", m.title, m.input.View())
	if m.error != "" {
		view += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.error)
	}
	return view + "\n\n(Enter save • Esc cancel)"
}

func RunNameForm(title, currentName string) (string, bool, error) {
	program := tea.NewProgram(initialNameFormModel(title, currentName))
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}
	model := finalModel.(nameFormModel)
	return strings.TrimSpace(model.input.Value()), model.done && !model.quitting, nil
}

type confirmationModel struct {
	prompt    string
	done      bool
	confirmed bool
}

func (m confirmationModel) Init() tea.Cmd { return nil }

func (m confirmationModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "Y":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "n", "N", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmationModel) View() string {
	if m.done {
		return ""
	}
	return m.prompt + "\n\n(y/N)"
}

func RunConfirmation(prompt string) (bool, error) {
	program := tea.NewProgram(confirmationModel{prompt: prompt})
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}
	return finalModel.(confirmationModel).confirmed, nil
}
