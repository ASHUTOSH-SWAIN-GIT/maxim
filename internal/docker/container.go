package docker

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// IsDockerAvailable checks if Docker is installed and running
func IsDockerAvailable() error {
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available. Please install Docker and ensure it's running: %w", err)
	}
	return nil
}

// ContainerInfo holds information about a container
type ContainerInfo struct {
	ContainerName string
	DatabaseName  string
	Port          string
	Password      string
    Username      string
}

// StartPostgreSQLContainer starts a PostgreSQL container with the given configuration
func StartPostgreSQLContainer(info ContainerInfo) error {
	// Build docker run command
	args := []string{
		"run",
		"-d",
		"--name", info.ContainerName,
		"-p", fmt.Sprintf("%s:5432", info.Port),
		"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", info.Password),
		"-e", fmt.Sprintf("POSTGRES_DB=%s", info.DatabaseName),
        "-e", fmt.Sprintf("POSTGRES_USER=%s", info.Username),
		"postgres:latest",
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if container name already exists
		if strings.Contains(string(output), "already in use") {
			return fmt.Errorf("container name '%s' is already in use", info.ContainerName)
		}
		// Check if port is already in use
		if strings.Contains(string(output), "bind") || strings.Contains(string(output), "port") {
			return fmt.Errorf("port %s is already in use", info.Port)
		}
		return fmt.Errorf("failed to start container: %s", string(output))
	}

	return nil
}

// WaitForContainerReady waits for the PostgreSQL container to be ready
func WaitForContainerReady(containerName string, maxWaitTime time.Duration) error {
	timeout := time.After(maxWaitTime)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("container did not become ready within %v", maxWaitTime)
		case <-ticker.C:
			// Check if container is running
			cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Status}}")
			output, err := cmd.CombinedOutput()
			if err != nil {
				continue
			}

			status := strings.TrimSpace(string(output))
			if status == "" {
				// Container might not be running, check if it exists
				cmd = exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Status}}")
				output, err = cmd.CombinedOutput()
				if err != nil {
					continue
				}
				status = strings.TrimSpace(string(output))
				if strings.Contains(status, "Exited") {
					return fmt.Errorf("container exited. Check logs with: docker logs %s", containerName)
				}
				continue
			}

			// Try to connect using pg_isready to check if it's ready
			cmd = exec.Command("docker", "exec", containerName, "pg_isready", "-U", "postgres")
			if err := cmd.Run(); err == nil {
				// PostgreSQL is ready, give it a moment to fully initialize
				time.Sleep(2 * time.Second)
				return nil
			}
		}
	}
}

// IsContainerRunning checks if a container is currently running
func IsContainerRunning(containerName string) (bool, error) {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(output)) == containerName, nil
}

// StopContainer stops a running container
func StopContainer(containerName string) error {
	cmd := exec.Command("docker", "stop", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop container: %s", string(output))
	}
	return nil
}

// RemoveContainer removes a container
func RemoveContainer(containerName string) error {
	cmd := exec.Command("docker", "rm", "-f", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove container: %s", string(output))
	}
	return nil
}

// GetContainerLogs returns the logs of a container
func GetContainerLogs(containerName string) (string, error) {
	cmd := exec.Command("docker", "logs", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	return string(output), nil
}

// ContainerDBInfo holds information about a database in a container
type ContainerDBInfo struct {
	ContainerName string
	DatabaseName  string
	Port          string
}

// FindContainerByDatabaseName finds a PostgreSQL container that has the specified database
func FindContainerByDatabaseName(dbName string) (*ContainerDBInfo, error) {
	// List all running containers with postgres image
	cmd := exec.Command("docker", "ps", "--filter", "ancestor=postgres:latest", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Docker might not be available, return nil (not an error)
		return nil, nil
	}

	containerNames := strings.TrimSpace(string(output))
	if containerNames == "" {
		return nil, nil
	}

	// Check each container for the database name
	for _, containerName := range strings.Split(containerNames, "\n") {
		containerName = strings.TrimSpace(containerName)
		if containerName == "" {
			continue
		}

		// Get the POSTGRES_DB environment variable
		cmd = exec.Command("docker", "inspect", "--format", "{{index .Config.Env}}", containerName)
		envOutput, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		// Parse environment variables to find POSTGRES_DB
		envStr := string(envOutput)
		if strings.Contains(envStr, fmt.Sprintf("POSTGRES_DB=%s", dbName)) {
			// Get the port mapping
			portCmd := exec.Command("docker", "port", containerName)
			portOutput, err := portCmd.CombinedOutput()
			port := ""
			if err == nil {
				portLines := strings.Split(strings.TrimSpace(string(portOutput)), "\n")
				if len(portLines) > 0 {
					// Format is "0.0.0.0:5432->5432/tcp" or "0.0.0.0:5432"
					portPart := strings.Split(portLines[0], ":")
					if len(portPart) > 1 {
						port = strings.Split(portPart[1], "->")[0]
					}
				}
			}

			return &ContainerDBInfo{
				ContainerName: containerName,
				DatabaseName:  dbName,
				Port:          port,
			}, nil
		}
	}

	return nil, nil
}

// GetAllContainerDatabases returns all databases running in Docker containers
func GetAllContainerDatabases() ([]ContainerDBInfo, error) {
	var containers []ContainerDBInfo

	// List all running containers with postgres image
	cmd := exec.Command("docker", "ps", "--filter", "ancestor=postgres:latest", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Docker might not be available, return empty list
		return containers, nil
	}

	containerNames := strings.TrimSpace(string(output))
	if containerNames == "" {
		return containers, nil
	}

	// Get database info for each container
	for _, containerName := range strings.Split(containerNames, "\n") {
		containerName = strings.TrimSpace(containerName)
		if containerName == "" {
			continue
		}

		// Get the POSTGRES_DB environment variable
		cmd = exec.Command("docker", "inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", containerName)
		envOutput, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		// Parse environment variables to find POSTGRES_DB
		envLines := strings.Split(string(envOutput), "\n")
		dbName := ""
		for _, line := range envLines {
			if strings.HasPrefix(line, "POSTGRES_DB=") {
				dbName = strings.TrimPrefix(line, "POSTGRES_DB=")
				break
			}
		}

		if dbName != "" {
			// Get the port mapping
			portCmd := exec.Command("docker", "port", containerName)
			portOutput, err := portCmd.CombinedOutput()
			port := ""
			if err == nil {
				portLines := strings.Split(strings.TrimSpace(string(portOutput)), "\n")
				if len(portLines) > 0 {
					// Format is "0.0.0.0:5432->5432/tcp" or "0.0.0.0:5432"
					portPart := strings.Split(portLines[0], ":")
					if len(portPart) > 1 {
						port = strings.Split(portPart[1], "->")[0]
					}
				}
			}

			containers = append(containers, ContainerDBInfo{
				ContainerName: containerName,
				DatabaseName:  dbName,
				Port:          port,
			})
		}
	}

	return containers, nil
}
