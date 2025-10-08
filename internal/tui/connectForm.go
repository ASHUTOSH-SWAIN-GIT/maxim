package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConnectFormModel struct {
	focusIndex int
	Inputs     []textinput.Model
	Quitting   bool
}

func RunConnectForm() (ConnectFormModel, error) {
	m, err := tea.NewProgram(initialModel()).Run()
	if err != nil {
		return ConnectFormModel{}, err
	}
	return m.(ConnectFormModel), nil
}

func initialModel() ConnectFormModel {
	m := ConnectFormModel{
		Inputs: make([]textinput.Model, 4),
	}

	var t textinput.Model
	for i := range m.Inputs {
		t = textinput.New()
		t.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		t.CharLimit = 32
		t.Prompt = ""

		switch i {
		case 0:
			t.Placeholder = "postgres"
			t.Focus()
		case 1:
			t.Placeholder = "password"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		case 2:
			t.Placeholder = "5432"
		case 3:
			t.Placeholder = "postgres"
		}

		m.Inputs[i] = t
	}

	return m
}

func (m ConnectFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ConnectFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.Quitting = true
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.focusIndex == len(m.Inputs)-1 {
				return m, tea.Quit
			}
			m.nextInput()
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyShiftTab, tea.KeyCtrlP:
			m.prevInput()
		case tea.KeyTab, tea.KeyCtrlN:
			m.nextInput()
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m ConnectFormModel) View() string {
	if m.Quitting {
		return ""
	}
	var b strings.Builder

	b.WriteString("Enter Database Credentials (host is localhost)\n\n")

	labels := []string{"Username:      ", "Password:      ", "Port:          ", "Database Name: "}
	for i := range m.Inputs {
		b.WriteString(labels[i])
		b.WriteString(m.Inputs[i].View())
		b.WriteRune('\n')
	}

	b.WriteString("\n(press Enter to submit, Esc to quit)")
	return b.String()
}

func (m *ConnectFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Inputs))
	for i := range m.Inputs {
		m.Inputs[i], cmds[i] = m.Inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *ConnectFormModel) nextInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.Inputs)
	m.Inputs[m.focusIndex].Focus()
}

func (m *ConnectFormModel) prevInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.Inputs) - 1
	}
	m.Inputs[m.focusIndex].Focus()
}
