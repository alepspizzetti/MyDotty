package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAutoSyncConfigurationLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pacman integration is for Linux")
	}
	root := t.TempDir()
	commands := filepath.Join(root, "commands")
	if err := os.Mkdir(commands, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"sudo": `printf '%s\n' "$*" >> "$TEST_ROOT/admin.log"
exec "$@"`,
		"systemctl": `printf '%s\n' "$*" >> "$TEST_ROOT/systemctl.log"`,
	} {
		writeFile(t, commands, name, "#!/bin/sh\nset -eu\n"+body+"\n")
		if err := os.Chmod(filepath.Join(commands, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", commands+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ROOT", root)
	t.Setenv("SSH_AUTH_SOCK", "/tmp/example-agent.sock")
	config := autoSyncConfig{binary: filepath.Join(root, `instalação %test $example "quoted"/mydotty`), username: "example", unitDir: filepath.Join(root, "units"), hookPath: filepath.Join(root, "hooks/zz-mydotty-1000.hook")}
	var out, stderr bytes.Buffer
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if err := config.configure(true, &out, &stderr); err != nil {
		t.Fatalf("enable: %v\n%s", err, stderr.String())
	}
	service := read(filepath.Join(config.unitDir, "mydotty-sync.service"))
	if !strings.Contains(service, "Type=oneshot") || !strings.Contains(service, "--att") || strings.Contains(service, "User=root") {
		t.Fatal("sync is not a user oneshot")
	}
	if !strings.Contains(service, `%%test $$example \"quoted\"`) {
		t.Fatalf("systemd path escaping: %s", service)
	}
	timer := read(filepath.Join(config.unitDir, "mydotty-sync.timer"))
	if !strings.Contains(timer, "OnActiveSec=15s") || !strings.Contains(timer, "Unit=mydotty-sync.service") {
		t.Fatal("timer does not schedule sync")
	}
	hook := read(config.hookPath)
	for _, expected := range []string{"Operation = Upgrade", "When = PostTransaction", "--user --machine=example@.host --no-block restart mydotty-sync.timer"} {
		if !strings.Contains(hook, expected) {
			t.Fatalf("hook missing %q", expected)
		}
	}
	if verifier, err := exec.LookPath("systemd-analyze"); err == nil {
		cmd := exec.Command(verifier, "--user", "verify", filepath.Join(config.unitDir, "mydotty-sync.service"), filepath.Join(config.unitDir, "mydotty-sync.timer"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("invalid systemd units: %v\n%s", err, out)
		}
	}
	if err := config.configure(true, &out, &stderr); err != nil {
		t.Fatal("repeat activation:", err)
	}
	log := read(filepath.Join(root, "systemctl.log"))
	if !strings.Contains(log, "import-environment SSH_AUTH_SOCK") {
		t.Fatal("SSH agent was not imported")
	}
	if strings.Contains(log, "start mydotty-sync") || strings.Contains(log, "restart mydotty-sync") {
		t.Fatal("activation unexpectedly started synchronization")
	}
	if err := config.configure(false, &out, &stderr); err != nil {
		t.Fatal("disable:", err)
	}
	for _, path := range []string{config.hookPath, filepath.Join(config.unitDir, "mydotty-sync.service"), filepath.Join(config.unitDir, "mydotty-sync.timer")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("integration file was not removed: %s", path)
		}
	}
	if err := config.configure(false, &out, &stderr); err != nil {
		t.Fatal("repeat deactivation:", err)
	}
}

func TestTUISettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{width: 60, height: 16}
	m = press(m, tea.KeyF6)
	if !m.settingsOpen || !strings.Contains(m.View(), "AUTO-SYNC") || !strings.Contains(m.View(), "AUTO UPDATE") {
		t.Fatal("F6 did not open all automation settings")
	}
	m = press(m, tea.KeyEsc)
	if m.settingsOpen {
		t.Fatal("Esc did not close settings")
	}
	m = press(m, tea.KeyTab)
	if !m.settingsOpen {
		t.Fatal("Tab did not switch to settings")
	}
	for i := range settingOptions {
		m.settingsCursor = i
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		if cmd == nil || !next.(model).busy || !next.(model).settingsOpen {
			t.Fatal("toggle did not hand configuration to the terminal")
		}
	}
	m = press(m, tea.KeyDown)
	if m.settingsCursor != 3 {
		t.Fatal("cursor moved past final option")
	}
	m = press(m, tea.KeyUp)
	if m.settingsCursor != 2 {
		t.Fatal("cursor did not move up")
	}
	m.settings = settings{true, true, true, true}
	if strings.Count(m.View(), "[x]") != 4 {
		t.Fatal("independent selections are not visible")
	}
	m.busy = true
	next, _ := m.Update(settingsResult{value: settings{LoginSync: true}})
	m = next.(model)
	if m.busy || !m.settings.LoginSync || m.settings.PackageSync {
		t.Fatal("settings were not refreshed after applying a change")
	}
}
