package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) repository {
	t.Helper()
	home := t.TempDir()
	r := repository{filepath.Join(home, ".dotfiles"), home}
	if _, err := runGit(home, "init", "--bare", "-b", "main", r.dir); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"user.name": "MyDotty Test", "user.email": "test@example.invalid", "commit.gpgsign": "false"} {
		if _, err := r.git("config", key, value); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func writeFile(t *testing.T, home, path, content string) {
	t.Helper()
	full := filepath.Join(home, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func mustGit(t *testing.T, r repository, args ...string) string {
	t.Helper()
	out, err := r.git(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExplicitStagingAndStatus(t *testing.T) {
	r := fixture(t)
	path := ".config/kitty/file with space\nand newline.conf"
	writeFile(t, r.home, path, "original\n")
	writeFile(t, r.home, "private.txt", "private\n")
	if _, err := r.add(path); err != nil {
		t.Fatal(err)
	}
	s, err := r.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.files) != 1 || s.files[0] != (fileEntry{path, "A "}) {
		t.Fatalf("unexpected files: %#v", s.files)
	}
	mustGit(t, r, "commit", "-m", "initial")
	writeFile(t, r.home, path, "modified\n")
	s, err = r.read()
	if err != nil || s.files[0].status != " M" {
		t.Fatalf("status: %#v, %v", s, err)
	}
	diff, err := r.diff(path)
	if err != nil || !strings.Contains(diff, "+modified") {
		t.Fatalf("diff: %s, %v", diff, err)
	}
	if _, err := r.add(filepath.Join(r.home, path)); err != nil {
		t.Fatal(err)
	}
	s, err = r.read()
	if err != nil || s.files[0].status != "M " {
		t.Fatalf("staged: %#v, %v", s, err)
	}
	if err := os.Remove(filepath.Join(r.home, path)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.add(path); err != nil {
		t.Fatal(err)
	}
	s, err = r.read()
	if err != nil || len(s.files) != 1 || s.files[0].status != "D " {
		t.Fatalf("deleted: %#v, %v", s, err)
	}
}

func TestAddRejectsBroadAndExternalPaths(t *testing.T) {
	r := fixture(t)
	for _, path := range []string{".", r.home, "..", r.dir, filepath.Join(r.dir, "HEAD")} {
		if _, err := r.add(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	writeFile(t, r.home, ".config/app/config", "content")
	if _, err := r.add(".config"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, r.home, "literal*file", "content")
	writeFile(t, r.home, "literal-other-file", "private")
	if _, err := r.add("literal*file"); err != nil {
		t.Fatal(err)
	}
	if got := mustGit(t, r, "ls-files", "-z"); got != ".config/app/config\x00literal*file\x00" {
		t.Fatalf("pathspec expanded: %q", got)
	}
}

func TestCloneConflictsAndRetry(t *testing.T) {
	source := fixture(t)
	path := ".config/app/config"
	writeFile(t, source.home, path, "remote\n")
	if _, err := source.add(path); err != nil {
		t.Fatal(err)
	}
	mustGit(t, source, "commit", "-m", "initial")
	home := t.TempDir()
	target := repository{filepath.Join(home, ".dotfiles"), home}
	writeFile(t, home, path, "local\n")
	if _, err := target.clone(source.dir); err == nil {
		t.Fatal("clone overwrote existing configuration")
	}
	got, err := os.ReadFile(filepath.Join(home, path))
	if err != nil || string(got) != "local\n" {
		t.Fatalf("existing content: %q, %v", got, err)
	}
	backup := filepath.Join(home, "backup")
	if err := os.Rename(filepath.Join(home, path), backup); err != nil {
		t.Fatal(err)
	}
	if _, err := target.apply(); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(filepath.Join(home, path))
	if err != nil || string(got) != "remote\n" {
		t.Fatalf("checkout content: %q, %v", got, err)
	}
	if _, err := target.apply(); err == nil {
		t.Error("accepted a second initial checkout")
	}
	writeFile(t, source.home, path, "updated\n")
	if _, err := source.add(path); err != nil {
		t.Fatal(err)
	}
	mustGit(t, source, "commit", "-m", "update")
	mustGit(t, target, "pull", "--ff-only", "origin", "main")
	got, err = os.ReadFile(filepath.Join(home, path))
	if err != nil || string(got) != "updated\n" {
		t.Fatalf("pull content: %q, %v", got, err)
	}
	for key, value := range map[string]string{"user.name": "Test", "user.email": "test@example.invalid", "commit.gpgsign": "false"} {
		mustGit(t, target, "config", key, value)
	}
	writeFile(t, home, path, "from second pc\n")
	if _, err := target.add(path); err != nil {
		t.Fatal(err)
	}
	mustGit(t, target, "commit", "-m", "second pc")
	mustGit(t, target, "push", "-u", "origin", "HEAD")
	if got := mustGit(t, source, "show", "HEAD:"+path); got != "from second pc\n" {
		t.Fatalf("push content: %q", got)
	}
}

func TestCheckoutRejectsSymlinkParent(t *testing.T) {
	source := fixture(t)
	writeFile(t, source.home, ".config/app/config", "remote")
	if _, err := source.add(".config/app/config"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, source, "commit", "-m", "initial")
	home, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, ".config")); err != nil {
		t.Skip(err)
	}
	target := repository{filepath.Join(home, ".dotfiles"), home}
	if _, err := target.clone(source.dir); err == nil {
		t.Fatal("accepted symlink parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "app/config")); !os.IsNotExist(err) {
		t.Fatalf("wrote outside home: %v", err)
	}
}

func TestUnstageKeepsFileContents(t *testing.T) {
	r := fixture(t)
	writeFile(t, r.home, "config", "original\n")
	if _, err := r.add("config"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.unstage("config"); err != nil {
		t.Fatal(err)
	}
	if got := mustGit(t, r, "ls-files"); got != "" {
		t.Fatalf("still staged: %q", got)
	}
	if _, err := r.add("config"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "commit", "-m", "initial")
	writeFile(t, r.home, "config", "changed\n")
	if _, err := r.add("config"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.unstage("config"); err != nil {
		t.Fatal(err)
	}
	s, err := r.read()
	if err != nil || s.files[0].status != " M" {
		t.Fatalf("status: %#v, %v", s, err)
	}
	content, err := os.ReadFile(filepath.Join(r.home, "config"))
	if err != nil || string(content) != "changed\n" {
		t.Fatalf("file contents: %q, %v", content, err)
	}
}

func emptyRemote(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	remote := filepath.Join(home, "remote.git")
	if _, err := runGit(home, "init", "--bare", "-b", "main", remote); err != nil {
		t.Fatal(err)
	}
	return remote
}

func newClient(t *testing.T, remote string) repository {
	t.Helper()
	home := t.TempDir()
	r := repository{filepath.Join(home, ".dotfiles"), home}
	if _, err := r.setup(remote); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "config", "commit.gpgsign", "false")
	return r
}

func TestBeginnerSetupAndSyncAcrossComputers(t *testing.T) {
	remote := emptyRemote(t)
	a := newClient(t, remote)
	writeFile(t, a.home, ".config/app/first", "first\n")
	writeFile(t, a.home, ".config/app/second", "second\n")
	writeFile(t, a.home, "private", "not selected")
	if _, err := a.add(".config/app"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.sync(); err != nil {
		t.Fatal(err)
	}
	if got := mustGit(t, a, "ls-tree", "-r", "--name-only", "HEAD"); strings.Contains(got, "private") {
		t.Fatal("saved an unselected file")
	}
	b := newClient(t, remote)
	if got, err := os.ReadFile(filepath.Join(b.home, ".config/app/first")); err != nil || string(got) != "first\n" {
		t.Fatalf("installation: %q, %v", got, err)
	}
	writeFile(t, b.home, ".config/app/second", "from b\n")
	if _, err := b.sync(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, a.home, ".config/app/first", "from a\n")
	if _, err := a.sync(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(a.home, ".config/app/second")); string(got) != "from b\n" {
		t.Fatalf("did not download b changes: %q", got)
	}
	if _, err := b.sync(); err != nil {
		t.Fatal(err)
	}
	if got := mustGit(t, a, "rev-parse", "HEAD"); got != mustGit(t, b, "rev-parse", "HEAD") {
		t.Fatal("clients did not converge")
	}
	if _, err := a.sync(); err != nil {
		t.Fatal("sync without changes:", err)
	}
	if _, err := a.remove(".config/app/first"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.home, ".config/app/first")); err != nil {
		t.Fatal("remove deleted local file:", err)
	}
	if got := mustGit(t, a, "ls-files"); strings.Contains(got, "first") {
		t.Fatal("removed file is still tracked")
	}
}

func TestSyncConflictPreservesLocalAndRemote(t *testing.T) {
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
	writeFile(t, a.home, "config", "version a\n")
	if _, err := a.sync(); err != nil {
		t.Fatal(err)
	}
	remoteHead := mustGit(t, a, "rev-parse", "HEAD")
	writeFile(t, b.home, "config", "version b\n")
	if _, err := b.sync(); err == nil || !strings.Contains(err.Error(), "preservadas") {
		t.Fatalf("expected conflict: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.home, "config")); string(got) != "version b\n" {
		t.Fatalf("local changes lost: %q", got)
	}
	if got := mustGit(t, b, "show", "HEAD:config"); got != "version b\n" {
		t.Fatalf("local changes not saved: %q", got)
	}
	if got := mustGit(t, b, "rev-parse", "refs/remotes/origin/main"); got != remoteHead {
		t.Fatal("remote history changed")
	}
	if _, err := b.read(); err != nil {
		t.Fatal("repository left in interrupted rebase:", err)
	}
}

func TestAddCreatesLocalRepositoryAndSyncKeepsOfflineChanges(t *testing.T) {
	home := t.TempDir()
	r := repository{filepath.Join(home, ".dotfiles"), home}
	writeFile(t, home, "config", "local\n")
	if _, err := r.add("config"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.sync(); err == nil || !strings.Contains(err.Error(), "conecte") {
		t.Fatalf("missing remote: %v", err)
	}
	if _, err := r.setup(emptyRemote(t)); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "config", "commit.gpgsign", "false")
	if _, err := r.sync(); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	writeFile(t, home, "config", "offline edit\n")
	if _, err := r.sync(); err == nil || !strings.Contains(err.Error(), "salvas") {
		t.Fatalf("expected offline error: %v", err)
	}
	if got := mustGit(t, r, "show", "HEAD:config"); got != "offline edit\n" {
		t.Fatalf("offline edit not saved: %q", got)
	}
}

func TestSetupExistingEmptyLocalUsesRemoteDefaultBranch(t *testing.T) {
	source := fixture(t)
	mustGit(t, source, "symbolic-ref", "HEAD", "refs/heads/configs")
	writeFile(t, source.home, "remote-config", "from remote\n")
	if _, err := source.add("remote-config"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, source, "commit", "-m", "initial")
	home := t.TempDir()
	target := repository{filepath.Join(home, ".dotfiles"), home}
	if err := target.ensure(); err != nil {
		t.Fatal(err)
	}
	if _, err := target.setup(source.dir); err != nil {
		t.Fatal(err)
	}
	state, err := target.read()
	if err != nil || state.branch != "configs" || state.needsCheckout {
		t.Fatalf("state: %#v %v", state, err)
	}
	if got, _ := os.ReadFile(filepath.Join(home, "remote-config")); string(got) != "from remote\n" {
		t.Fatalf("remote config not installed: %q", got)
	}
}

func TestSetupWithoutGlobalGitIdentity(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	r := newClient(t, emptyRemote(t))
	if got := strings.TrimSpace(mustGit(t, r, "config", "--local", "user.name")); got != "MyDotty" {
		t.Fatalf("name: %q", got)
	}
	if got := strings.TrimSpace(mustGit(t, r, "config", "--local", "user.email")); got != "mydotty@localhost" {
		t.Fatalf("email: %q", got)
	}
	writeFile(t, r.home, "config", "new\n")
	if _, err := r.add("~/config"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.diff("~/config"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.remove("~/config"); err != nil {
		t.Fatal(err)
	}
}
