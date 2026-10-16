package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestInputPreservesSingleSpacesAndUnicode(t *testing.T) {
	m := model{prompt: "commit"}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("Atualiza")},
		{Type: tea.KeySpace, Runes: []rune{' '}},
		{Type: tea.KeyRunes, Runes: []rune("configuração")},
	} {
		next, _ := m.Update(key)
		m = next.(model)
	}
	if m.input != "Atualiza configuração" {
		t.Fatalf("input: %q", m.input)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if next.(model).input != "Atualiza configuraçã" {
		t.Fatalf("backspace: %q", next.(model).input)
	}
}

func TestTUIAddUploadsImmediately(t *testing.T) {
	remote := emptyRemote(t)
	r := newClient(t, remote)
	writeFile(t, r.home, "config", "added from F2\n")
	m := model{repo: r, prompt: "add", input: "config", width: 80, height: 24}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !next.(model).busy {
		t.Fatal("add did not start")
	}
	next, _ = next.(model).Update(cmd())
	if !strings.Contains(next.(model).message, "Tudo sincronizado") {
		t.Fatal(next.(model).message)
	}
	if got, err := runGit(r.home, "--git-dir="+remote, "show", "HEAD:config"); err != nil || got != "added from F2\n" {
		t.Fatalf("F2 did not upload: %q, %v", got, err)
	}
}

func TestTUISyncConflictChoice(t *testing.T) {
	for _, choice := range []string{"l", "r", "esc"} {
		t.Run(choice, func(t *testing.T) {
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
			m := model{repo: b, width: 80, height: 24}
			next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF3})
			next, _ = next.(model).Update(cmd())
			m = next.(model)
			if m.prompt != "conflict" {
				t.Fatalf("no conflict choice: %q", m.message)
			}
			key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(choice)}
			if choice == "esc" {
				key = tea.KeyMsg{Type: tea.KeyEsc}
			}
			next, cmd = m.Update(key)
			if choice == "esc" {
				if cmd != nil || next.(model).prompt != "" {
					t.Fatal("cancel did not close prompt")
				}
				if got := mustGit(t, b, "show", "HEAD:config"); got != "local\n" {
					t.Fatal(got)
				}
				return
			}
			if cmd == nil {
				t.Fatal("choice did not start synchronization")
			}
			res := cmd().(result)
			if res.err != nil {
				t.Fatal(res.err)
			}
			want := "local\n"
			if choice == "r" {
				want = "remote\n"
			}
			if got := mustGit(t, b, "show", "HEAD:config"); got != want {
				t.Fatalf("active config: %q", got)
			}
			if got, err := os.ReadFile(filepath.Join(b.home, "config")); err != nil || string(got) != want {
				t.Fatalf("working config: %q, %v", got, err)
			}
			if _, err := a.sync(); err != nil {
				t.Fatal(err)
			}
			if got := mustGit(t, a, "show", "HEAD:config"); got != want {
				t.Fatalf("other computer: %q", got)
			}
			if got := mustGit(t, b, "status", "--porcelain", "--untracked-files=no"); got != "" {
				t.Fatalf("dirty after merge: %q", got)
			}
		})
	}
}

func TestTerminalOutputRemovesControlSequences(t *testing.T) {
	text := safe("\x1b[31mfilename\x1b[0m\x1b]52;c;Y2xpcGJvYXJk\a\nline\x00")
	if strings.ContainsAny(text, "\x1b\x00\a") || !strings.Contains(text, "filename\nline") {
		t.Fatalf("unsafe output: %q", text)
	}
}

func TestIncompleteCheckoutBlocksCommit(t *testing.T) {
	m := model{state: snapshot{needsCheckout: true}}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil || next.(model).prompt != "" {
		t.Fatal("commit allowed before initial checkout")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd == nil {
		t.Fatal("initial checkout was blocked")
	}
}

func TestLayoutFitsTerminal(t *testing.T) {
	files := []fileEntry{{".config/kitty/kitty.conf", " M"}, {".zshrc", "M "}, {".config/arquivo\ncom quebra.conf", "A "}}
	for i := 0; i < 40; i++ {
		files = append(files, fileEntry{fmt.Sprintf(".config/app/config-%02d", i), "  "})
	}
	states := map[string]model{
		"files":     {state: snapshot{files: files, branch: "main", remote: strings.Repeat("remote", 30)}, cursor: 30},
		"empty":     {},
		"missing":   {missing: true},
		"checkout":  {state: snapshot{needsCheckout: true}},
		"busy":      {busy: true},
		"message":   {message: strings.Repeat("erro longo", 40) + "\nsegunda linha\nterceira linha"},
		"add":       {prompt: "add", input: strings.Repeat("configuração/", 30)},
		"commit":    {prompt: "commit", input: "Atualiza configuração"},
		"clone":     {missing: true, prompt: "clone"},
		"settings":  {settingsOpen: true},
		"settings2": {settingsOpen: true, settingsCursor: 3, message: strings.Repeat("erro longo", 40) + "\nsegunda linha"},
		"conflict":  {prompt: "conflict", detail: strings.Repeat("config\n", 60)},
		"diff":      {detail: strings.Repeat("+alteração\n", 60), offset: 500},
	}
	for _, size := range [][2]int{{1, 1}, {35, 10}, {59, 15}, {60, 16}, {60, 19}, {60, 20}, {80, 24}, {120, 36}, {160, 48}} {
		for name, m := range states {
			t.Run(fmt.Sprintf("%s/%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m.width, m.height = size[0], size[1]
				output := m.View()
				lines := strings.Split(output, "\n")
				if len(lines) > m.height {
					t.Fatalf("height %d exceeds %d", len(lines), m.height)
				}
				for i, line := range lines {
					if width := ansi.StringWidth(line); width > m.width {
						t.Fatalf("line %d width %d exceeds %d: %q", i, width, m.width, line)
					}
				}
				if m.width >= 60 && m.height >= 16 {
					if !strings.Contains(strings.ToLower(ansi.Strip(output)), "sair") && m.prompt == "" {
						t.Fatal("quit shortcut is missing")
					}
					if m.prompt != "" && m.prompt != "conflict" && !strings.Contains(output, "▏") {
						t.Fatal("input cursor is missing")
					}
					if name == "files" && !strings.Contains(output, files[m.cursor].path) {
						t.Fatal("selected file is not visible")
					}
				}
			})
		}
	}
}

func TestDiffPaginationAndResize(t *testing.T) {
	var lines []string
	for i := 0; i < 80; i++ {
		lines = append(lines, fmt.Sprintf("+linha %03d", i))
	}
	for _, size := range [][2]int{{60, 16}, {60, 24}, {80, 24}, {120, 36}} {
		m := model{width: size[0], height: size[1], detail: strings.Join(lines, "\n")}
		page := m.diffPageSize()
		if !strings.Contains(m.View(), lines[page-1]) {
			t.Fatalf("last line of first page missing at %v", size)
		}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = next.(model)
		if m.offset != page || !strings.Contains(m.View(), lines[page]) {
			t.Fatalf("pagination skipped lines at %v", size)
		}
		for i := 0; i < 80; i++ {
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = next.(model)
		}
		if !strings.Contains(m.View(), lines[len(lines)-1]) {
			t.Fatal("last diff line is inaccessible")
		}
		next, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
		m = next.(model)
		if m.offset > len(lines)-m.diffPageSize() {
			t.Fatal("resize left scroll offset outside viewport")
		}
	}
}
