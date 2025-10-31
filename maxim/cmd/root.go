package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

// showLoadingMessage prints a loading message and returns a function to stop it
func showLoadingMessage(msg string) func() {
	done := make(chan bool)
	go func() {
		spinner := []string{".", "..", "...", "   "}
		i := 0
		for {
			select {
			case <-done:
				fmt.Print("\r" + strings.Repeat(" ", len(msg)+5) + "\r")
				return
			default:
				fmt.Printf("\r%s%s", msg, spinner[i%len(spinner)])
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()
	return func() {
		done <- true
		time.Sleep(250 * time.Millisecond)
	}
}

func startPythonAPI() (*exec.Cmd, error) {
	root, err := locateRepoRootForPy()
	if err != nil {
		return nil, fmt.Errorf("failed to locate repo root: %w", err)
	}

	// Try to find Python with uvicorn - check venv first, then system
	pythonCmd := "python"
	venvPath := filepath.Join(root, "maxim-nl-api", "myenv", "bin", "python")
	if _, err := os.Stat(venvPath); err == nil {
		pythonCmd = venvPath
	}

	cmd := exec.Command(pythonCmd, "-m", "uvicorn", "maxim-nl-api.main:app", "--host", "127.0.0.1", "--port", "5000")
	cmd.Env = os.Environ()
	cmd.Dir = root

	// Capture stderr to see startup errors
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Python process: %w", err)
	}

	// Give it a moment to start
	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:5000/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return cmd, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Timeout - capture error output
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	stderrOutput := stderr.String()
	if stderrOutput != "" {
		hint := ""
		if strings.Contains(stderrOutput, "No module named uvicorn") {
			hint = "\n\nHint: Make sure uvicorn is installed. If using venv, ensure maxim-nl-api/myenv/bin/python exists and has uvicorn installed."
		}
		return nil, fmt.Errorf("NL2SQL server failed to start (timeout after 60s). Error output:\n%s%s", stderrOutput, hint)
	}
	return nil, fmt.Errorf("NL2SQL server did not start in time (timeout after 60s). Check if port 5000 is available")
}

func stopPythonAPI(proc *exec.Cmd) {
	if proc != nil && proc.Process != nil {
		_ = proc.Process.Kill()
		_ = proc.Wait()
	}
}

func locateRepoRootForPy() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 4; i++ {
		candidate := wd
		for j := 0; j < i; j++ {
			candidate = filepath.Dir(candidate)
		}
		if _, err := os.Stat(filepath.Join(candidate, "maxim-nl-api", "main.py")); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not locate maxim-nl-api/main.py from %s", wd)
}

var rootCmd = &cobra.Command{
	Use:   "maxim",
	Short: "Maxim is a terminal-based client for PostgreSQL and MySQL.",
	Long: `A fast and modern TUI for interacting with your databases
	directly from the terminal.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show beautiful ASCII art and description
		tui.ShowASCIIArt()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %s\n", err)
		os.Exit(1)
	}
}

// startCmd represents the start command for TUI interface
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the interactive TUI interface",
	Long: `Start the interactive terminal user interface for database operations.
This will launch the main menu where you can connect to databases,
create new databases, and perform various database operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		choice, err := tui.RunMainMenu()
		if err != nil {
			fmt.Printf("Error running main menu: %v\n", err)
			os.Exit(1)
		}

		// Handle quitting
		if choice == -1 {
			fmt.Println("Goodbye!")
			return
		}

		switch choice {
		case 0:
			// Connect flow - connect to a specific database
			result, err := tui.RunConnectForm()
			if err != nil {
				fmt.Printf("Error running form: %v\n", err)
				os.Exit(1)
			}
			if result.Quitting {
				fmt.Println("Connection cancelled.")
				os.Exit(0)
			}

			conn, err := db.ConnectAndVerify("psql", result.User, result.Password, "localhost", result.Port, result.DBName)
			if err != nil {
				fmt.Printf(" Connection failed: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			// Show database operations menu
			for {
				choice, err := tui.RunDBOperationsMenu(result.DBName)
				if err != nil {
					fmt.Printf("Error running operations menu: %v\n", err)
					break
				}

				// Check if user pressed 'q' to quit
				if choice == -1 {
					break
				}

				switch choice {
				case 0: // Show table data
					tables, err := db.GetTables(conn)
					if err != nil {
						fmt.Printf("Error fetching tables: %v\n", err)
						continue
					}
					selectedTable, err := tui.RunTableList(tables)
					if err != nil {
						continue
					}

					if err := tui.RunPagedDataViewer(conn, selectedTable); err != nil {
						fmt.Printf("Error displaying data: %v\n", err)
					}

				case 1: // Editor
					if err := tui.RunSQLEditor(conn, result.DBName); err != nil {
						fmt.Printf("Error running SQL editor: %v\n", err)
					}
				case 2: // NL2SQL
					form, err := tui.RunNL2SQLForm()
					if err != nil || form.Quitting || form.NL == "" {
						continue
					}

					stopLoading := showLoadingMessage("Starting API server")
					apiProc, err := startPythonAPI()
					stopLoading()
					if err != nil {
						fmt.Printf("Failed to start NL2SQL server: %v\n", err)
						continue
					}
					defer stopPythonAPI(apiProc)

					dbURI := fmt.Sprintf("postgresql://%s:%s@localhost:%s/%s", result.User, result.Password, result.Port, result.DBName)
					payload := map[string]string{
						"nl_query": form.NL,
						"db_uri":   dbURI,
					}
					body, _ := json.Marshal(payload)

					stopLoading = showLoadingMessage("Generating SQL query")
					resp, err := http.Post("http://127.0.0.1:5000/generate-sql", "application/json", bytes.NewReader(body))
					stopLoading()
					if err != nil {
						fmt.Printf("Error calling NL2SQL API: %v\n", err)
						stopPythonAPI(apiProc)
						continue
					}
					respBody, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					stopPythonAPI(apiProc)
					if resp.StatusCode != 200 {
						fmt.Printf("NL2SQL API error: %s\n", string(respBody))
						continue
					}
					sql := strings.TrimSpace(string(respBody))

					stopLoading = showLoadingMessage("Executing query")
					res := db.ExecuteQuery(conn, sql)
					stopLoading()
					if res.Success {
						out := res.Data
						out = strings.Replace(out, "Query executed successfully!\n\n", "", 1)
						if err := tui.WaitForEsc(out); err != nil {
							fmt.Printf("Error displaying results: %v\n", err)
						}
					} else {
						if err := tui.WaitForEsc(res.Error); err != nil {
							fmt.Printf("Error displaying error: %v\n", err)
						}
					}
				default:
					return
				}
			}
		case 1:
			// Create flow
			adminInfo, err := getAdminConnectionInfo()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminInfo.DB.Close()

			formData, err := tui.RunCreateForm()
			if err != nil {
				fmt.Printf("Error: could not open create form: %v\n", err)
				os.Exit(1)
			}
			if formData.Quitting {
				fmt.Println("Cancelled: database creation aborted by user.")
				return
			}
			dbName := formData.Inputs[0].Value()
			newUser := formData.Inputs[1].Value()
			newPassword := formData.Inputs[2].Value()
			if err := db.CreateDBAndUser(adminInfo.DB, "psql", dbName, newUser, newPassword, adminInfo.User, adminInfo.Password, adminInfo.Host, adminInfo.Port); err != nil {
				fmt.Printf("Error: failed to create database/user: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Success: created database '%s' and user '%s'.\n", dbName, newUser)
		case 2:
			// List databases flow
			adminInfo, err := getAdminConnectionInfo()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminInfo.DB.Close()

			dbNames, err := db.ListDatabases(adminInfo.DB)
			if err != nil {
				fmt.Printf("Could not fetch database list: %v\n", err)
				os.Exit(1)
			}

			selectedDB, err := tui.RunDBList(dbNames)
			if err != nil {
				fmt.Printf("Error selecting database: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Selected database: %s\n", selectedDB)
		case 3:
			// Delete database flow
			adminInfo, err := getAdminConnectionInfo()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminInfo.DB.Close()

			dbNames, err := db.ListDatabases(adminInfo.DB)
			if err != nil {
				fmt.Printf("Could not fetch database list: %v\n", err)
				os.Exit(1)
			}

			selectedDB, err := tui.RunDBList(dbNames)
			if err != nil {
				fmt.Printf("Error selecting database: %v\n", err)
				os.Exit(1)
			}

			// Confirm deletion
			fmt.Printf("Are you sure you want to delete database '%s'? This action cannot be undone! (y/N): ", selectedDB)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Database deletion cancelled.")
				return
			}

			if err := db.DeleteDatabase(adminInfo.DB, "psql", selectedDB); err != nil {
				fmt.Printf("Error deleting database: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Success: database '%s' has been deleted.\n", selectedDB)
		}
	},
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a database",
	Long: `Delete a PostgreSQL database.
This will permanently remove the selected database and all its data.`,
	Run: func(cmd *cobra.Command, args []string) {
		adminInfo, err := getAdminConnectionInfo()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		defer adminInfo.DB.Close()

		dbNames, err := db.ListDatabases(adminInfo.DB)
		if err != nil {
			fmt.Printf("Could not fetch database list: %v\n", err)
			os.Exit(1)
		}

		selectedDB, err := tui.RunDBList(dbNames)
		if err != nil {
			fmt.Printf("Error selecting database: %v\n", err)
			os.Exit(1)
		}

		// Confirm deletion
		fmt.Printf("Are you sure you want to delete database '%s'? This action cannot be undone! (y/N): ", selectedDB)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Database deletion cancelled.")
			return
		}

		if err := db.DeleteDatabase(adminInfo.DB, "psql", selectedDB); err != nil {
			fmt.Printf("Error deleting database: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Success: database '%s' has been deleted.\n", selectedDB)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	// rootCmd.AddCommand(dbCmd)
}
