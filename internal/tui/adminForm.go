package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AdminFormModel struct {
	focusIndex int
	Inputs     []textinput.Model
	Quitting   bool
}

type AdminResult struct {
	User     string
	Password string
	Port     string
	Quitting bool
}

func RunAdminForm() (AdminResult, error) {
	m, err := tea.NewProgram(initialAdminFormModel()).Run()
	if err != nil {
		return AdminResult{}, err
	}

	model := m.(AdminFormModel)
	result := AdminResult{
		User:     model.Inputs[0].Value(),
		Password: model.Inputs[1].Value(),
		Port:     model.Inputs[2].Value(),
		Quitting: model.Quitting,
	}

	return result, nil
}

func initialAdminFormModel() AdminFormModel {
	m := AdminFormModel{
		Inputs: make([]textinput.Model, 3),
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
		}
		m.Inputs[i] = t
	}
	return m
}

func (m AdminFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AdminFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case tea.KeyShiftTab, tea.KeyCtrlP:
			m.prevInput()
		case tea.KeyTab, tea.KeyCtrlN:
			m.nextInput()
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m AdminFormModel) View() string {
	if m.Quitting {
		return ""
	}
	var b strings.Builder

	b.WriteString("Enter Postgres Superuser Credentials\n\n")

	labels := []string{"Username: ", "Password: ", "Port:     "}
	for i := range m.Inputs {
		b.WriteString(labels[i])
		b.WriteString(m.Inputs[i].View())
		b.WriteRune('\n')
	}

	b.WriteString("\n(press Enter to submit, Esc to quit)")
	return b.String()
}

func (m *AdminFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Inputs))
	for i := range m.Inputs {
		m.Inputs[i], cmds[i] = m.Inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *AdminFormModel) nextInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.Inputs)
	m.Inputs[m.focusIndex].Focus()
}

func (m *AdminFormModel) prevInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.Inputs) - 1
	}
	m.Inputs[m.focusIndex].Focus()
}
