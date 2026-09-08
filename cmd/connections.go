package cmd

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/config"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
)

type connectionRequest struct {
	result      tui.ConnectResult
	profileName string
	save        bool
}

func openManagedConnection() (*sql.DB, tui.ConnectResult, error) {
	request, err := chooseConnection()
	if err != nil || request.result.Quitting {
		return nil, request.result, err
	}
	if err := validateConnectionResult(request.result); err != nil {
		return nil, request.result, err
	}

	connection, err := db.ConnectPostgres(db.ConnectionOptions{
		User: request.result.User, Password: request.result.Password, Host: request.result.Host,
		Port: request.result.Port, Database: request.result.DBName, SSLMode: request.result.SSLMode,
	})
	if err != nil {
		return nil, request.result, err
	}
	if request.save {
		if err := saveConnectionProfileAs(request.profileName, request.result); err != nil {
			connection.Close()
			return nil, request.result, fmt.Errorf("save connection: %w", err)
		}
	}
	return connection, request.result, nil
}

func chooseConnection() (connectionRequest, error) {
	for {
		names, err := config.ListDatabaseConnections()
		if err != nil {
			return connectionRequest{}, fmt.Errorf("load saved connections: %w", err)
		}
		if len(names) == 0 {
			result, err := tui.RunConnectForm()
			return connectionRequest{result: result, save: !result.Quitting}, err
		}

		selection, err := tui.RunConnectionManager(names)
		if err != nil {
			return connectionRequest{}, err
		}
		switch selection.Action {
		case tui.ConnectionActionQuit:
			return connectionRequest{result: tui.ConnectResult{Quitting: true}}, nil
		case tui.ConnectionActionNew:
			result, err := tui.RunConnectForm()
			if err != nil {
				return connectionRequest{}, err
			}
			if result.Quitting {
				continue
			}
			return connectionRequest{result: result, save: true}, nil
		case tui.ConnectionActionConnect:
			request, err := requestFromSavedConnection(selection.Name, false)
			if err != nil {
				return connectionRequest{}, err
			}
			if request.result.Quitting {
				continue
			}
			return request, nil
		case tui.ConnectionActionEdit:
			request, err := requestFromSavedConnection(selection.Name, true)
			if err != nil {
				return connectionRequest{}, err
			}
			if request.result.Quitting {
				continue
			}
			return request, nil
		case tui.ConnectionActionRename:
			newName, confirmed, err := tui.RunNameForm("Rename connection", selection.Name)
			if err != nil {
				return connectionRequest{}, err
			}
			if confirmed {
				if err := config.RenameDatabaseConnection(selection.Name, newName); err != nil {
					return connectionRequest{}, fmt.Errorf("rename connection: %w", err)
				}
			}
		case tui.ConnectionActionDelete:
			confirmed, err := tui.RunConfirmation(fmt.Sprintf("Delete saved connection %q?", selection.Name))
			if err != nil {
				return connectionRequest{}, err
			}
			if confirmed {
				if err := config.DeleteDatabaseConnection(selection.Name); err != nil {
					return connectionRequest{}, fmt.Errorf("delete connection: %w", err)
				}
			}
		}
	}
}

func requestFromSavedConnection(name string, edit bool) (connectionRequest, error) {
	details, err := config.LoadDatabaseConnection(name)
	if err != nil {
		return connectionRequest{}, fmt.Errorf("load connection %q: %w", name, err)
	}
	defaults := tui.ConnectResult{
		DBType: "psql", Host: details.Host, Port: details.Port,
		User: details.User, DBName: details.DBName, SSLMode: details.SSLMode,
	}
	if edit {
		result, err := tui.RunConnectFormWithDefaults(defaults)
		if err != nil {
			return connectionRequest{}, err
		}
		return connectionRequest{result: result, profileName: name, save: !result.Quitting}, nil
	}

	password, err := tui.RunPasswordForm()
	if err != nil {
		return connectionRequest{}, err
	}
	if password.Quitting {
		return connectionRequest{result: tui.ConnectResult{Quitting: true}}, nil
	}
	defaults.Password = password.Password
	return connectionRequest{result: defaults}, nil
}

func validateConnectionResult(result tui.ConnectResult) error {
	fields := []struct {
		name  string
		value string
	}{
		{"host", result.Host},
		{"port", result.Port},
		{"username", result.User},
		{"database", result.DBName},
		{"SSL mode", result.SSLMode},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	return nil
}

func saveConnectionProfileAs(name string, result tui.ConnectResult) error {
	details := config.ConnectionDetails{
		Host: result.Host, Port: result.Port, User: result.User,
		DBName: result.DBName, SSLMode: result.SSLMode,
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("%s@%s:%s/%s", result.User, result.Host, result.Port, result.DBName)
	}
	return config.SaveDatabaseConnection(name, details, result.Password)
}
