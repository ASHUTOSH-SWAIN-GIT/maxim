package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PasswordFormModel struct {
	Input   textinput.Model
	Quitting bool
	done     bool
}

type PasswordFormResult struct {
	Password string
	Quitting bool
}

func RunPasswordForm() (PasswordFormResult, error) {
	m, err := tea.NewProgram(initialPasswordFormModel()).Run()
	if err != nil {
		return PasswordFormResult{}, err
	}

	model := m.(PasswordFormModel)
	if model.Quitting || !model.done {
		return PasswordFormResult{Quitting: true}, nil
	}

	return PasswordFormResult{
		Password: model.Input.Value(),
		Quitting: false,
	}, nil
}

func initialPasswordFormModel() PasswordFormModel {
	m := PasswordFormModel{
		Input: textinput.New(),
	}

	m.Input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.Input.Prompt = "Password: "
	m.Input.EchoMode = textinput.EchoPassword
	m.Input.EchoCharacter = '•'
	m.Input.Focus()

	return m
}

func (m PasswordFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PasswordFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.Quitting = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}

func (m PasswordFormModel) View() string {
	if m.Quitting || m.done {
		return ""
	}
	return m.Input.View() + "\n\n(press Enter to submit, Esc to cancel)"
}

