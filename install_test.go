package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerDelegatesAutoSyncToInstalledCommand(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("shell installer is tested on Linux")
	}
	root := t.TempDir()
	commands, source, prefix := filepath.Join(root, "commands"), filepath.Join(root, "source"), filepath.Join(root, "prefix with spaces")
	for _, dir := range []string{commands, source} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"dirname", "mktemp", "rm", "mkdir", "install", "git"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s unavailable", name)
		}
		if err := os.Symlink(path, filepath.Join(commands, name)); err != nil {
			t.Fatal(err)
		}
	}
	installer, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, source, "install.sh", string(installer))
	writeFile(t, source, "mydotty", "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TEST_ROOT/invocations\"\n")
	if err := os.Chmod(filepath.Join(source, "mydotty"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, option := range []string{"--auto-sync", "--no-auto-sync"} {
		cmd := exec.Command("/bin/sh", filepath.Join(source, "install.sh"), option)
		cmd.Dir = "/"
		cmd.Env = []string{"PATH=" + commands, "PREFIX=" + prefix, "TEST_ROOT=" + root}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", option, err, out)
		}
	}
	calls, err := os.ReadFile(filepath.Join(root, "invocations"))
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "--help\n--auto-sync\n--no-auto-sync\n" {
		t.Fatalf("unexpected invocations: %q", calls)
	}
	cmd := exec.Command("/bin/sh", filepath.Join(source, "install.sh"), "--unknown")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "Opção desconhecida") {
		t.Fatal("unknown option was not rejected")
	}
}
