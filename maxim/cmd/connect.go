package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/config"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a database and save credentials",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
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
			fmt.Printf("\n Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		detailsToSave := config.ConnectionDetails{
			Host:   "localhost",
			Port:   result.Port,
			User:   result.User,
			DBName: result.DBName,
		}

		// Create a connection name based on the database name
		connectionName := fmt.Sprintf("%s@%s:%s", result.User, "localhost", result.Port)

		if err := config.SaveDatabaseConnection(connectionName, detailsToSave, result.Password); err != nil {
			fmt.Printf("\n Failed to save credentials: %v\n", err)
			os.Exit(1)
		}

		// Show database operations menu (same as TUI version)
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
	},
}
