# Maxim

**Maxim** is a fast, modern terminal user interface (TUI) for working with PostgreSQL databases. Browse tables, view data, and run SQL queries with intelligent autocomplete—all without leaving your terminal.

![Maxim](https://img.shields.io/badge/version-1.0.0-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Go](https://img.shields.io/badge/go-1.21+-00ADD8)

## Features

- 🚀 **Fast & Modern TUI** - Beautiful, keyboard-driven interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
-  **Database Management** - Create, list, and delete PostgreSQL databases
-  **Table Browser** - Browse tables and view data with pagination (100 rows per page)
-  **SQL Editor** - Write and execute SQL queries with syntax highlighting
-  **Intelligent Autocomplete** - Smart suggestions for SQL keywords, table names, and column names
-  **Keyboard-Driven** - Full keyboard navigation—no mouse required
-  **Connection Management** - Save and reuse database connections securely

## Installation

### Quick Install (Recommended)

The easiest way to install Maxim is using Go's `go install` command:

```bash
go install github.com/ASHUTOSH-SWAIN-GIT/maxim@latest
```

**Prerequisites:**
- Go 1.21 or later installed ([download Go](https://golang.org/dl/))
- `$GOPATH/bin` or `$GOBIN` in your `PATH` (usually already configured)

The binary will be installed to your `$GOPATH/bin` or `$GOBIN` directory (typically `~/go/bin`).

**Verify installation:**
```bash
maxim --version
```

**Adding to PATH (if command not found):**

If the `maxim` command is not found after installation, you need to add Go's bin directory to your PATH:

**Linux/macOS:**

For **Bash** (add to `~/.bashrc`):
```bash
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
```

For **Zsh** (add to `~/.zshrc`):
```bash
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
source ~/.zshrc
```

For **Fish** (add to `~/.config/fish/config.fish`):
```bash
echo 'set -gx PATH $PATH (go env GOPATH)/bin' >> ~/.config/fish/config.fish
source ~/.config/fish/config.fish
```

**Windows (PowerShell):**

```powershell
# Add to PowerShell profile (run once)
[System.Environment]::SetEnvironmentVariable('Path', $env:Path + ";$env:USERPROFILE\go\bin", 'User')

# Or manually add C:\Users\<YourUsername>\go\bin to your PATH environment variable
```

**Windows (Command Prompt):**

```cmd
# Add to PATH permanently (replace <username> with your username)
setx PATH "%PATH%;C:\Users\<username>\go\bin"
```

After adding to PATH, restart your terminal or run `source ~/.bashrc` (Linux/macOS) / restart PowerShell (Windows).

### Alternative: Download Pre-built Binaries

If you prefer pre-built binaries, download from the [Releases](https://github.com/ASHUTOSH-SWAIN-GIT/maxim/releases) page:

1. Download the appropriate archive for your platform
2. Extract and move to your PATH
3. Verify: `maxim --version`

See the [Releases](https://github.com/ASHUTOSH-SWAIN-GIT/maxim/releases) page for all available binaries.

## Requirements

- PostgreSQL server running and accessible
- Superuser credentials for database management operations (create/delete databases)

### Creating a PostgreSQL Superuser

If you don't already have a PostgreSQL superuser to use with Maxim, create one using one of the options below.

Option A: Using psql

```bash
# 1) Switch to the postgres system user (Linux)
sudo -u postgres psql

# 2) Inside psql, create a login role with superuser privileges
CREATE ROLE maxim_admin WITH LOGIN SUPERUSER PASSWORD 'your-strong-password';

# 3) Verify
\du

# 4) Exit psql
\q
```

Option B: Using createuser

```bash
# Linux/macOS
sudo -u postgres createuser --superuser maxim_admin
sudo -u postgres psql -c "ALTER USER maxim_admin WITH PASSWORD 'your-strong-password';"
```

Notes

- Use a strong password and store it securely.
- If connecting remotely, ensure `postgresql.conf` and `pg_hba.conf` allow your host/IP.
- On managed services (RDS, Cloud SQL, Azure), use the platform-provided admin user instead of creating your own superuser.

## Quick Start

### 1. Launch Maxim

```bash
maxim start
```

This will open the main menu with the following options:
- Connect to a DB
- Create a new DB
- List all DBs
- Delete a DB

### 2. Connect to a Database

Choose **"Connect to a DB"** from the main menu, or use:

```bash
maxim connect
```

Enter your database credentials:
- Port (default: 5432)
- Username
- Password
- Database Name

Your connection details (except password) will be saved for future use.

### 3. Explore Your Database

After connecting, you'll see the database operations menu:

- **Show table data** - Browse tables and view data with pagination
- **Editor** - Open the SQL editor

## Usage Guide

### Main Commands

```bash
maxim start      # Launch interactive TUI interface
maxim connect    # Connect to a database
maxim create     # Create a new database and user
maxim list       # List all databases on the server
maxim delete     # Delete a database
maxim --version  # Show version information
maxim --help     # Show help
```

### SQL Editor

The SQL Editor provides a split-panel interface:

- **Left Panel**: Type your SQL queries
- **Right Panel**: View results or autocomplete suggestions

**Keyboard Shortcuts:**
- `Ctrl+E` - Execute the current SQL query
- `Ctrl+R` - Clear results
- `Tab` - Cycle through autocomplete suggestions
- `Enter` - Select highlighted suggestion
- `Esc` - Return to database operations menu

### Viewing Table Data

When viewing table data:
- **Enter** - Load next page (100 rows)
- **Arrow keys** - Scroll through data
- **PageUp/PageDown** - Navigate pages
- **Esc** - Return to menu

## Configuration

Connection details are stored securely in your system's configuration directory:

- **Linux/macOS**: `~/.config/maxim/config.json`
- **Windows**: `%APPDATA%\maxim\config.json`

Only connection details (host, port, username, database name) are saved—passwords are never stored and must be entered each time.

## Keyboard Navigation

Maxim is fully keyboard-driven:

- **Arrow Keys** / **j/k** - Navigate menus and lists
- **Enter** - Select/Submit
- **Esc** / **q** - Quit/Go back
- **Tab** - Move to next field (in forms)
- **Shift+Tab** - Move to previous field

## Examples

### Create a New Database

```bash
maxim create
# Follow the prompts to enter:
# - Database name
# - Username
# - Password
```

### List All Databases

```bash
maxim list
# Shows all databases on the server
# Press Esc to return to menu
```

### Connect and Query

```bash
maxim connect
# After connecting:
# 1. Select "Editor"
# 2. Type: SELECT * FROM users;
# 3. Press Ctrl+E to execute
```

## Troubleshooting

### Connection Failed

- Verify PostgreSQL is running: `sudo systemctl status postgresql` (Linux)
- Check if the port is correct (default: 5432)
- Ensure your user has the necessary permissions
- Verify network connectivity to the database server

### Permission Denied

- Ensure you're using superuser credentials for database creation/deletion
- Check PostgreSQL user permissions
- Verify database ownership

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) - A powerful TUI framework
- Uses [Cobra](https://github.com/spf13/cobra) for CLI commands

## Support

For issues, questions, or feature requests, please open an issue on [GitHub](https://github.com/ASHUTOSH-SWAIN-GIT/maxim/issues).

---

**Enjoy working with your databases! 🚀**

