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
	if text := invoke("--status"); !strings.Contains(text, "salvo") {
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
	for _, args := range [][]string{{"--add", "x", "--att"}, {"--auto-sync", "--att"}, {"unexpected"}, {"--add", ""}, {"--keep", "local"}, {"--att", "--keep", "invalid"}} {
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"--help"}, &out, &stderr); code != 0 {
		t.Fatal(code)
	}
	for _, action := range []string{"-init", "-add", "-att", "-sync", "-status", "-diff", "-remove", "-apply", "-auto-sync", "-no-auto-sync", "-settings", "-set", "-update"} {
		if !strings.Contains(stderr.String(), action) {
			t.Fatalf("help omits %s", action)
		}
	}
}

func TestAddUploadsImmediatelyAndKeepsOfflineCommit(t *testing.T) {
	remote := emptyRemote(t)
	r := newClient(t, remote)
	var out, stderr bytes.Buffer
	invoke := func(path string) int {
		t.Helper()
		return run([]string{"--repo", r.dir, "--home", r.home, "--add", path}, &out, &stderr)
	}
	writeFile(t, r.home, ".config/app/config", "uploaded by add\n")
	if code := invoke(".config/app"); code != 0 {
		t.Fatal(stderr.String())
	}
	if got, err := runGit(r.home, "--git-dir="+remote, "show", "HEAD:.config/app/config"); err != nil || got != "uploaded by add\n" {
		t.Fatalf("--add did not upload: %q, %v", got, err)
	}
	mustGit(t, r, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	writeFile(t, r.home, "offline", "saved offline\n")
	if code := invoke("offline"); code == 0 || !strings.Contains(stderr.String(), "salvas nesta máquina") {
		t.Fatalf("missing offline notification: %s", stderr.String())
	}
	if got := mustGit(t, r, "show", "HEAD:offline"); got != "saved offline\n" {
		t.Fatalf("offline addition not committed: %q", got)
	}
	mustGit(t, r, "remote", "set-url", "origin", remote)
	if _, err := r.sync(); err != nil {
		t.Fatal(err)
	}
}

func TestCLIAsksAfterSyncConflict(t *testing.T) {
	for _, answer := range []string{"l\n", "r\n", "x\nr\n", "c\n", "", "background"} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			remote := emptyRemote(t)
			a := newClient(t, remote)
			writeFile(t, a.home, "config", "original\n")
			if _, err := a.add("config"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.sync(); err != nil {
				t.Fatal(err)
			}
			b := newClient(t, remote)
			writeFile(t, a.home, "config", "remote\n")
			if _, err := a.sync(); err != nil {
				t.Fatal(err)
			}
			writeFile(t, b.home, "config", "local\n")
			var out, stderr bytes.Buffer
			args := []string{"--repo", b.dir, "--home", b.home, "--att"}
			if answer == "background" {
				if code := runWithInput(args, nil, &out, &stderr); code != 1 || strings.Contains(stderr.String(), "Qual versão") {
					t.Fatalf("noninteractive conflict: %d, %s", code, stderr.String())
				}
				if got := mustGit(t, b, "show", "HEAD:config"); got != "local\n" {
					t.Fatalf("local version lost: %q", got)
				}
				return
			}
			code := runWithInput(args, strings.NewReader(answer), &out, &stderr)
			if !strings.Contains(stderr.String(), "Qual versão") || !strings.Contains(stderr.String(), "config") {
				t.Fatalf("no conflict prompt: %s", stderr.String())
			}
			want := "local\n"
			if strings.Contains(answer, "r") {
				want = "remote\n"
			}
			cancel := answer == "c\n" || answer == ""
			if (code == 0) == cancel {
				t.Fatalf("exit code: %d", code)
			}
			got, err := os.ReadFile(filepath.Join(b.home, "config"))
			if err != nil || string(got) != want {
				t.Fatalf("active config: %q, %v", got, err)
			}
		})
	}
}

func TestCLIOnlyAsksForConflicts(t *testing.T) {
	r := newClient(t, emptyRemote(t))
	writeFile(t, r.home, "config", "local\n")
	if _, err := r.add("config"); err != nil {
		t.Fatal(err)
	}
	for _, offline := range []bool{false, true} {
		if offline {
			mustGit(t, r, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
		}
		input := strings.NewReader("r\n")
		var out, stderr bytes.Buffer
		code := runWithInput([]string{"--repo", r.dir, "--home", r.home, "--att"}, input, &out, &stderr)
		if (code == 0) == offline {
			t.Fatalf("offline=%v, code=%d: %s", offline, code, stderr.String())
		}
		if input.Len() != 2 || strings.Contains(stderr.String(), "Qual versão") {
			t.Fatal("asked without conflict")
		}
		if offline && !strings.Contains(stderr.String(), "baixar") {
			t.Fatalf("missing connection error: %s", stderr.String())
		}
	}
}
