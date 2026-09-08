//go:build integration

package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildMaximForTest(t *testing.T) string {
	t.Helper()
	binaryName := "maxim"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(t.TempDir(), binaryName)
	command := exec.Command("go", "build", "-trimpath", "-o", binaryPath, ".")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build Maxim: %v\n%s", err, output)
	}
	return binaryPath
}

func TestIntegrationCLIVersionAndHelp(t *testing.T) {
	binaryPath := buildMaximForTest(t)

	version, err := exec.Command(binaryPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run --version: %v\n%s", err, version)
	}
	if !strings.Contains(string(version), "maxim version") {
		t.Fatalf("unexpected version output: %s", version)
	}

	help, err := exec.Command(binaryPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("run --help: %v\n%s", err, help)
	}
	for _, expected := range []string{"Available Commands", "connect", "create", "delete", "list", "start"} {
		if !strings.Contains(string(help), expected) {
			t.Errorf("help output missing %q:\n%s", expected, help)
		}
	}
}
