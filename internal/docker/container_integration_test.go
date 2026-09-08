//go:build integration

package docker

import (
	"fmt"
	"net"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func availablePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release local port: %v", err)
	}
	return fmt.Sprintf("%d", port)
}

func TestIntegrationPostgreSQLContainerLifecycle(t *testing.T) {
	if err := IsDockerAvailable(); err != nil {
		t.Fatalf("Docker must be available for integration tests: %v", err)
	}

	suffix := time.Now().UnixNano()
	info := ContainerInfo{
		ContainerName: fmt.Sprintf("maxim-it-%d", suffix),
		DatabaseName:  "maxim_integration",
		Username:      "maxim_user",
		Password:      "MaximDocker_42",
		Port:          availablePort(t),
	}
	started := false
	t.Cleanup(func() {
		if started {
			_ = RemoveContainer(info.ContainerName)
		}
	})

	if err := StartPostgreSQLContainer(info); err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	started = true
	if err := StartPostgreSQLContainer(info); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("duplicate container name did not return the expected error: %v", err)
	}
	if err := WaitForContainerReady(info.ContainerName, 90*time.Second); err != nil {
		logs, _ := GetContainerLogs(info.ContainerName)
		t.Fatalf("wait for PostgreSQL container: %v\n%s", err, logs)
	}

	running, err := IsContainerRunning(info.ContainerName)
	if err != nil || !running {
		t.Fatalf("container is not running: running=%t err=%v", running, err)
	}

	found, err := FindContainerByDatabaseName(info.DatabaseName)
	if err != nil {
		t.Fatalf("find container database: %v", err)
	}
	if found == nil || found.ContainerName != info.ContainerName || found.Port != info.Port {
		t.Fatalf("unexpected discovered container: %#v", found)
	}

	containers, err := GetAllContainerDatabases()
	if err != nil {
		t.Fatalf("list container databases: %v", err)
	}
	if !slices.ContainsFunc(containers, func(container ContainerDBInfo) bool {
		return container.ContainerName == info.ContainerName && container.DatabaseName == info.DatabaseName
	}) {
		t.Fatalf("created container missing from discovery result: %#v", containers)
	}

	query := exec.Command(
		"docker", "exec", info.ContainerName, "psql",
		"-U", info.Username, "-d", info.DatabaseName, "-Atc", "SELECT current_database(), current_user",
	)
	output, err := query.CombinedOutput()
	if err != nil {
		t.Fatalf("query container database: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != info.DatabaseName+"|"+info.Username {
		t.Fatalf("unexpected container query result: %s", output)
	}

	if err := StopContainer(info.ContainerName); err != nil {
		t.Fatalf("stop container: %v", err)
	}
	running, err = IsContainerRunning(info.ContainerName)
	if err != nil || running {
		t.Fatalf("container still running after stop: running=%t err=%v", running, err)
	}
	if err := RemoveContainer(info.ContainerName); err != nil {
		t.Fatalf("remove container: %v", err)
	}
	started = false
}
