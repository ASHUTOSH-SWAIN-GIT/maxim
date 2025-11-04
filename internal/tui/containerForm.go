package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ContainerFormModel struct {
	focusIndex int
	Inputs     []textinput.Model
	Quitting   bool
}

type ContainerFormResult struct {
	ContainerName string
	DatabaseName  string
	Port          string
	Password      string
	Quitting      bool
}

func RunContainerForm() (ContainerFormResult, error) {
	m, err := tea.NewProgram(initialContainerFormModel()).Run()
	if err != nil {
		return ContainerFormResult{}, err
	}
	model := m.(ContainerFormModel)
	if model.Quitting {
		return ContainerFormResult{Quitting: true}, nil
	}

	return ContainerFormResult{
		ContainerName: model.Inputs[0].Value(),
		DatabaseName:  model.Inputs[1].Value(),
		Port:          model.Inputs[2].Value(),
		Password:      model.Inputs[3].Value(),
		Quitting:      false,
	}, nil
}

func initialContainerFormModel() ContainerFormModel {
	m := ContainerFormModel{
		Inputs: make([]textinput.Model, 4),
	}

	var t textinput.Model
	for i := range m.Inputs {
		t = textinput.New()
		t.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		t.Prompt = ""

		switch i {
		case 0:
			t.Focus()
			t.Placeholder = "my-postgres-container"
		case 1:
			t.Placeholder = "mydb"
		case 2:
			t.Placeholder = "5432"
			t.CharLimit = 5
		case 3:
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
			t.Placeholder = "password"
		}
		m.Inputs[i] = t
	}
	return m
}

func (m ContainerFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ContainerFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ContainerFormModel) View() string {
	if m.Quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString("Spin up PostgreSQL in Docker Container\n\n")

	labels := []string{
		"Container Name: ",
		"Database Name:  ",
		"Port:           ",
		"Password:       ",
	}

	for i := range m.Inputs {
		b.WriteString(labels[i])
		b.WriteString(m.Inputs[i].View())
		b.WriteRune('\n')
	}

	b.WriteString("\n(press Enter to submit, Esc to cancel)")
	return b.String()
}

func (m *ContainerFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Inputs))
	for i := range m.Inputs {
		m.Inputs[i], cmds[i] = m.Inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *ContainerFormModel) nextInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.Inputs)
	m.Inputs[m.focusIndex].Focus()
}

func (m *ContainerFormModel) prevInput() {
	m.Inputs[m.focusIndex].Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.Inputs) - 1
	}
	m.Inputs[m.focusIndex].Focus()
}

