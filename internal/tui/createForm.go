package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CreateFormModel struct {
	focusIndex int
	Inputs     []textinput.Model
	Quitting   bool
}

func RunCreateForm() (CreateFormModel, error) {
	m, err := tea.NewProgram(initialCreateFormModel()).Run()
	if err != nil {
		return CreateFormModel{}, err
	}
	return m.(CreateFormModel), nil
}

func initialCreateFormModel() CreateFormModel {
	m := CreateFormModel{
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
			t.Placeholder = "new_db_name"
			t.Focus()
		case 1:
			t.Placeholder = "new_user"
		case 2:
			t.Placeholder = "password"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}
		m.Inputs[i] = t
	}
	return m
}

func (m CreateFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m CreateFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case tea.KeyTab, tea.KeyCtrlN:
			m.nextInput()
		case tea.KeyShiftTab, tea.KeyCtrlP:
			m.prevInput()
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m CreateFormModel) View() string {
	if m.Quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString("Enter Details for New Database and User\n\n")

	labels := []string{"Database Name: ", "New Username:  ", "New Password:  "}
	for i := range m.Inputs {
		b.WriteString(labels[i])
		b.WriteString(m.Inputs[i].View())
		b.WriteRune('\n')
	}

	b.WriteString("\n(press Enter to submit, Esc to quit)")
	return b.String()
}

func (m *CreateFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Inputs))
	for i := range m.Inputs {
		m.Inputs[i], cmds[i] = m.Inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *CreateFormModel) nextInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.Inputs)
	m.Inputs[m.focusIndex].Focus()
}

func (m *CreateFormModel) prevInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.Inputs) - 1
	}
	m.Inputs[m.focusIndex].Focus()
}
