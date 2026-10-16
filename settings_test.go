package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mockSystemctl(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "systemctl", `#!/bin/sh
printf '%s\n' "$*" >> "$TEST_SYSTEMCTL_LOG"
if [ "${TEST_SYSTEMCTL_FAIL:-}" = yes ]; then exit 1; fi
`)
	if err := os.Chmod(filepath.Join(root, "systemctl"), 0755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(root, "calls")
	t.Setenv("TEST_SYSTEMCTL_LOG", log)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func TestSettingsCLIIndependentOptionsAndPersistence(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	log := mockSystemctl(t)
	// Existing pacman integration must survive changes to the other options.
	if err := (settings{PackageSync: true}).save(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"login=true", "logout=true", "update=true"} {
		var out, stderr bytes.Buffer
		if code := run([]string{"--set", value}, &out, &stderr); code != 0 {
			t.Fatalf("%s: %s", value, stderr.String())
		}
	}
	s, err := loadSettings()
	if err != nil || s != (settings{true, true, true, true}) {
		t.Fatalf("options were not retained together: %+v, %v", s, err)
	}
	unitDir := filepath.Join(configDir, "systemd", "user")
	for name, expected := range map[string]string{
		"mydotty-login.service":  "--auto-run login",
		"mydotty-logout.service": "--auto-run logout",
		"mydotty-update.service": "--auto-run update",
		"mydotty-update.timer":   "OnCalendar=daily",
	} {
		data, err := os.ReadFile(filepath.Join(unitDir, name))
		if err != nil || !strings.Contains(string(data), expected) {
			t.Fatalf("%s: %s, %v", name, data, err)
		}
		if name == "mydotty-login.service" || name == "mydotty-logout.service" {
			if !strings.Contains(string(data), "PartOf=graphical-session.target") || !strings.Contains(string(data), "WantedBy=default.target graphical-session.target") {
				t.Fatalf("graphical session lifecycle missing: %s", data)
			}
		}
	}
	if verifier, err := exec.LookPath("systemd-analyze"); err == nil {
		args := []string{"--user", "verify"}
		for _, name := range []string{"mydotty-login.service", "mydotty-logout.service", "mydotty-update.service", "mydotty-update.timer"} {
			args = append(args, filepath.Join(unitDir, name))
		}
		if output, err := exec.Command(verifier, args...).CombinedOutput(); err != nil {
			t.Fatalf("invalid automation units: %v\n%s", err, output)
		}
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"--set", "login=false"}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	s, err = loadSettings()
	if err != nil || s != (settings{true, false, true, true}) {
		t.Fatalf("disabling login changed other options: %+v, %v", s, err)
	}
	if _, err := os.Stat(filepath.Join(unitDir, "mydotty-login.service")); !os.IsNotExist(err) {
		t.Fatal("login unit was not removed")
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{"enable mydotty-login.service", "enable --now mydotty-logout.service", "enable --now mydotty-update.timer", "disable --now mydotty-login.service"} {
		if !strings.Contains(string(data), call) {
			t.Fatalf("missing lifecycle operation %s: %s", call, data)
		}
	}
	if strings.Contains(string(data), "enable --now mydotty-login") {
		t.Fatal("enabling login should not start a sync immediately")
	}
	for _, value := range []string{"logout=false", "update=false", "logout=false", "update=false"} {
		if code := run([]string{"--set", value}, &out, &stderr); code != 0 {
			t.Fatalf("%s: %s", value, stderr.String())
		}
	}
	s, err = loadSettings()
	if err != nil || s != (settings{PackageSync: true}) {
		t.Fatalf("disabling automations affected pacman: %+v, %v", s, err)
	}
	for _, name := range []string{"mydotty-logout.service", "mydotty-update.service", "mydotty-update.timer"} {
		if _, err := os.Stat(filepath.Join(unitDir, name)); !os.IsNotExist(err) {
			t.Fatalf("disabled unit remains: %s", name)
		}
	}
}

func TestSettingsRejectInvalidAndFailedChanges(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mockSystemctl(t)
	initial := settings{PackageSync: true, LogoutSync: true}
	if err := initial.save(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"unknown=true", "login", "login=maybe", ""} {
		var out, stderr bytes.Buffer
		if code := run([]string{"--set", value}, &out, &stderr); code == 0 {
			t.Fatalf("accepted %q", value)
		}
	}
	t.Setenv("TEST_SYSTEMCTL_FAIL", "yes")
	if err := changeSetting("login=true", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted failed systemd configuration")
	}
	s, err := loadSettings()
	if err != nil || s != initial {
		t.Fatalf("failed changes were persisted: %+v, %v", s, err)
	}
}

func TestSessionSyncChecksPersistedOptions(t *testing.T) {
	for _, trigger := range []string{"login", "logout"} {
		t.Run(trigger, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			r := newClient(t, emptyRemote(t))
			writeFile(t, r.home, "config", "initial\n")
			if _, err := r.addAndSync("config", ""); err != nil {
				t.Fatal(err)
			}
			writeFile(t, r.home, "config", "edited before session event\n")
			invoke := func() {
				t.Helper()
				var out, stderr bytes.Buffer
				if code := run([]string{"--repo", r.dir, "--home", r.home, "--auto-run", trigger}, &out, &stderr); code != 0 {
					t.Fatal(stderr.String())
				}
			}
			s := settings{}
			if err := s.save(); err != nil {
				t.Fatal(err)
			}
			invoke()
			if got := mustGit(t, r, "show", "HEAD:config"); got != "initial\n" {
				t.Fatal("disabled session sync saved changes")
			}
			if err := s.set(trigger, true); err != nil {
				t.Fatal(err)
			}
			if err := s.save(); err != nil {
				t.Fatal(err)
			}
			invoke()
			remote := strings.TrimSpace(mustGit(t, r, "config", "remote.origin.url"))
			if got, err := runGit(r.home, "--git-dir="+remote, "show", "HEAD:config"); err != nil || got != "edited before session event\n" {
				t.Fatalf("session event did not upload: %q, %v", got, err)
			}
		})
	}
}
