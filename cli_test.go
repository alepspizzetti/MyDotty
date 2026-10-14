package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIWorkflowWithoutTerminal(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, ".dotfiles")
	remote := emptyRemote(t)
	invoke := func(args ...string) string {
		t.Helper()
		var out, stderr bytes.Buffer
		args = append([]string{"--repo", repo, "--home", home}, args...)
		if code := run(args, &out, &stderr); code != 0 {
			t.Fatalf("%v exited %d: %s", args, code, stderr.String())
		}
		return out.String()
	}
	invoke("--init", remote)
	if _, err := runGit(home, "--git-dir="+repo, "config", "commit.gpgsign", "false"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, home, ".config/app/config com espaço", "initial\n")
	invoke("--add", ".config/app")
	if text := invoke("--status"); !strings.Contains(text, "novo") {
		t.Fatalf("status: %s", text)
	}
	invoke("--att")
	writeFile(t, home, ".config/app/config com espaço", "modified\n")
	if text := invoke("--diff", ".config/app/config com espaço"); !strings.Contains(text, "+modified") {
		t.Fatalf("diff: %s", text)
	}
	invoke("--sync")
	invoke("--remove", ".config/app/config com espaço")
	invoke("--att")
	if _, err := os.Stat(filepath.Join(home, ".config/app/config com espaço")); err != nil {
		t.Fatal("removed local contents:", err)
	}
}

func TestCLIRejectsAmbiguousActionsAndDocumentsCommands(t *testing.T) {
	for _, args := range [][]string{{"--add", "x", "--att"}, {"unexpected"}, {"--add", ""}} {
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"--help"}, &out, &stderr); code != 0 {
		t.Fatal(code)
	}
	for _, action := range []string{"-init", "-add", "-att", "-sync", "-status", "-diff", "-remove", "-apply"} {
		if !strings.Contains(stderr.String(), action) {
			t.Fatalf("help omits %s", action)
		}
	}
}
