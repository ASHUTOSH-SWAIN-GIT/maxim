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
	done       bool
}

type ConnectResult struct {
	DBType   string
	Port     string
	User     string
	Password string
	DBName   string
	Quitting bool
}

func RunConnectForm() (ConnectResult, error) {
	m, err := tea.NewProgram(initialConnectFormModel()).Run()
	if err != nil {
		return ConnectResult{}, err
	}

	model := m.(ConnectFormModel)
	result := ConnectResult{
		DBType:   "psql",
		Port:     model.Inputs[0].Value(),
		User:     model.Inputs[1].Value(),
		Password: model.Inputs[2].Value(),
		DBName:   model.Inputs[3].Value(),
		Quitting: model.Quitting,
	}

	return result, nil
}

func initialConnectFormModel() ConnectFormModel {
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
			t.Placeholder = "5432"
			t.Focus()
		case 1:
			t.Placeholder = "postgres"
		case 2:
			t.Placeholder = "password"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
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
				m.done = true
				return m, tea.Quit
			}
			m.nextInput()
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
	if m.Quitting || m.done {
		return ""
	}

	var b strings.Builder
	b.WriteString("Enter Database Credentials\n\n")
	labels := []string{"Port:     ", "Username: ", "Password: ", "DB Name:  "}
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
