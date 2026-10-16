package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerConfiguresShellPathWithoutDuplicates(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "sh"} {
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				t.Skip(err)
			}
			home := filepath.Join(t.TempDir(), "home with 'quotes' and $dollars")
			configDir := filepath.Join(home, ".config")
			binDir := filepath.Join(home, ".local", "bin")
			binary := filepath.Join(binDir, "mydotty")
			writeFile(t, binDir, "mydotty", "#!/bin/sh\nexit 0\n")
			if err := os.Chmod(binary, 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", configDir)
			t.Setenv("SHELL", executable)
			t.Setenv("ZDOTDIR", filepath.Join(home, "zsh config"))
			profile := filepath.Join(home, ".profile")
			switch shell {
			case "bash":
				profile = filepath.Join(home, ".bashrc")
			case "zsh":
				profile = filepath.Join(os.Getenv("ZDOTDIR"), ".zshrc")
			case "fish":
				profile = filepath.Join(configDir, "fish", "conf.d", "mydotty.fish")
			}
			if shell != "fish" {
				writeFile(t, filepath.Dir(profile), filepath.Base(profile), "# existing settings\n")
			}
			if err := configureInstallPath(binary); err != nil {
				t.Fatal(err)
			}
			first, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			if err := configureInstallPath(binary); err != nil {
				t.Fatal(err)
			}
			second, err := os.ReadFile(profile)
			if err != nil || string(first) != string(second) {
				t.Fatal("repeat installation duplicated shell configuration")
			}
			if shell != "fish" && !strings.HasPrefix(string(second), "# existing settings\n") {
				t.Fatal("existing shell configuration changed")
			}
			// Read the written file with the actual shell; the new binary must win over PATH.
			cmd := exec.Command(executable, "-c", `. "$1"; . "$1"; command -v mydotty`, "--", profile)
			if shell == "fish" {
				cmd = exec.Command(executable, "-c", `source $argv[1]; source $argv[1]; command -s mydotty`, profile)
			}
			output, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != binary {
				t.Fatalf("shell did not resolve installed binary: %s, %v", output, err)
			}
		})
	}
}

func TestInstallerEnablesUpdatesAndPreservesDisabledPreference(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	t.Setenv("SHELL", "/bin/bash")
	log := mockSystemctl(t)
	var out, stderr bytes.Buffer
	if code := run([]string{"--install-setup"}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	s, err := loadSettings()
	if err != nil || !s.AutoUpdate {
		t.Fatalf("installer did not enable updates: %+v, %v", s, err)
	}
	s.AutoUpdate = false
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if err := setupInstallation(&out, &stderr); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(log)
	if err != nil || string(before) != string(after) {
		t.Fatal("installer reconfigured disabled updates")
	}
	s, err = loadSettings()
	if err != nil || s.AutoUpdate {
		t.Fatal("installer overwrote disabled preference")
	}
}

func TestInstallerCanRetryUnavailableSystemd(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	t.Setenv("SHELL", "/bin/bash")
	mockSystemctl(t)
	t.Setenv("TEST_SYSTEMCTL_FAIL", "yes")
	if err := setupInstallation(&bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected unavailable systemd error")
	}
	path, err := settingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed first setup stored a disabled preference")
	}
	t.Setenv("TEST_SYSTEMCTL_FAIL", "no")
	if err := setupInstallation(&bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal("setup could not retry:", err)
	}
}
