Maxim
=====

A fast, modern terminal user interface (TUI) for working with PostgreSQL. Browse tables, view data, run SQL queries, and perform common operations without leaving your terminal.

Features
--------
- Connect to a PostgreSQL database using a simple, keyboard-driven flow
- Main menu with options to:
  - Connect to a database
  - Create a database and user (with superuser credentials)
  - List databases
- Table browser to list tables and view data
- SQL editor to run ad‑hoc queries against the connected database
- Clear, human‑readable results with column headers and row limits
- Thoughtful error messages for common connection issues
- Config is stored per-user under `~/.config/maxim/config.json`

Installation
------------

Option A: Go install (recommended)
- Prerequisites: Go 1.20+ installed and configured
- Run:
```
go install github.com/ASHUTOSH-SWAIN-GIT/maxim@latest
```
- Ensure your Go binaries directory is on PATH:
  - Linux/macOS (bash/zsh):
    - `export PATH="$PATH:$(go env GOPATH)/bin"`
    - Add the line above to your shell profile (e.g., `~/.bashrc`, `~/.zshrc`)
  - Windows (PowerShell):
    - `$env:PATH += ";$env:GOPATH\bin"`

Option B: Download release
- Download the archive for your OS/arch from the GitHub Releases page
- Extract the `maxim` (or `maxim.exe`) binary
- Place it on your PATH (e.g., `/usr/local/bin` on Linux/macOS)
- Make it executable if needed (`chmod +x maxim`)

Create a PostgreSQL superuser (if you don't have one)
----------------------------------------------------
Most PostgreSQL installs include a `postgres` superuser. If you do not have a superuser yet:

1. Connect to PostgreSQL:
```
psql -U postgres
```

2. Create a new superuser:
```sql
CREATE USER new_admin_name WITH SUPERUSER PASSWORD 'a_strong_password';
```

3. Exit psql:
```sql
\q
```

Notes:
- Replace `new_admin_name` and `a_strong_password` with your desired credentials.
- You can now use these credentials in Maxim for admin operations.

Quick Start
-----------
Run the CLI:
```
maxim
```

You will see the main TUI menu. Use the arrow keys or shortcuts to navigate.

Workflows
---------

Connect to a database
- Choose “Connect to a database”
- Enter: port, user, password, database name
- After a successful connection, you can:
  - List tables
  - View data from a table
  - Open the SQL editor

Create database and user
- Choose “Create database and user”
- You will be prompted for superuser credentials (password is hidden)
- Provide the new database name, username, and password
- On success, both the database and user will be created

List databases
- Choose “List databases”
- Requires superuser credentials
- Displays databases from your server

SQL Editor
----------
The editor runs queries against the database you connected to in the “Connect to a database” flow.

Keybindings:
- Ctrl+E: Execute the SQL in the left panel
- Ctrl+R: Clear results in the right panel
- Esc: Exit the editor

Notes:
- Results show column headers and up to 100 rows by default
- Long cell values are truncated for readability
- After a successful execution, the left panel (query input) is cleared to speed up iterative querying

Configuration
-------------
- Config file path: `~/.config/maxim/config.json`
  - Stores admin connection metadata and saved database connection entries (without passwords)
- You can delete this file to reset saved metadata:
  - `rm -f ~/.config/maxim/config.json`

Troubleshooting
---------------
- `maxim: command not found`
  - Ensure `$(go env GOPATH)/bin` is on PATH
  - Re-open your terminal session after updating PATH

- Connection errors
  - Invalid password: verify user credentials
  - Database does not exist: confirm DB name
  - Connection refused/failure: ensure PostgreSQL is running and the port/host are correct

Upgrading
---------
- If you installed with `go install`:
```
go install github.com/ASHUTOSH-SWAIN-GIT/maxim@latest
```

Development
-----------
Prerequisites:
- Go 1.20+

Run locally:
```
go build -o maxim .
./maxim
```

Project Layout (high level)
---------------------------
- `cmd/` – Cobra commands and CLI entrypoint
- `internal/tui/` – TUI screens and flows (main menu, editor, viewers)
- `internal/db/` – Database utilities (connect, list, table data, query executor)
- `internal/config/` – Config read/write utilities

License
-------
MIT License. See `LICENSE` if provided, or add one to your repository.


