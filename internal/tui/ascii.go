package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// showASCIIArt displays a beautiful ASCII art for "MAXIM"
func showASCIIArt() {
	// Clear screen
	fmt.Print("\033[2J\033[H")

	// ASCII Art for "MAXIM" - using a bold, modern style
	asciiArt := `
███╗   ███╗ █████╗ ██╗  ██╗██╗███╗   ███╗
████╗ ████║██╔══██╗╚██╗██╔╝██║████╗ ████║
██╔████╔██║███████║ ╚███╔╝ ██║██╔████╔██║
██║╚██╔╝██║██╔══██║ ██╔██╗ ██║██║╚██╔╝██║
██║ ╚═╝ ██║██║  ██║██╔╝ ██╗██║██║ ╚═╝ ██║
╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚═╝     ╚═╝
`

	// Style for ASCII art
	artStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Align(lipgloss.Center)

	// Welcome message
	welcomeMsg := "Welcome to Maxim - Your Terminal Database Companion"
	welcomeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Bold(true).
		Align(lipgloss.Center).
		MarginTop(1).
		MarginBottom(2)

	// Description
	description := `A fast, modern terminal user interface (TUI) for working with PostgreSQL.
Browse tables, view data, run SQL queries, and perform common operations
without leaving your terminal.`

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Align(lipgloss.Center).
		MarginBottom(3)

	// Features list
	features := ` Features:
• Connect to PostgreSQL databases
• Create and manage databases
• Browse tables and view data
• Run SQL queries with syntax highlighting
• Intuitive keyboard-driven interface`

	featuresStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Align(lipgloss.Center).
		MarginBottom(2)

	// Usage instructions
	usage := `Usage:
  maxim start    - Launch the interactive TUI interface
  maxim connect  - Connect to a database
  maxim create   - Create a new database
  maxim list     - List all databases
  maxim delete   - Delete a database
  maxim --help   - Show help information`

	usageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")).
		Align(lipgloss.Center).
		MarginBottom(2)

	// Display everything
	fmt.Println(artStyle.Render(asciiArt))
	fmt.Println(welcomeStyle.Render(welcomeMsg))
	fmt.Println(descStyle.Render(description))
	fmt.Println(featuresStyle.Render(features))
	fmt.Println(usageStyle.Render(usage))

	// Add a small delay for dramatic effect
	time.Sleep(1500 * time.Millisecond)
}

// ShowASCIIArt is the public function to display the ASCII art
func ShowASCIIArt() {
	showASCIIArt()
}
