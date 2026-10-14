package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func press(m model, key tea.KeyType) model {
	next, _ := m.Update(tea.KeyMsg{Type: key})
	return next.(model)
}

func TestPathCompletionAndCycling(t *testing.T) {
	home := t.TempDir()
	r := repository{filepath.Join(home, ".dotfiles"), home}
	writeFile(t, home, ".config/kitty/kitty.conf", "")
	writeFile(t, home, ".config/kitty/keymap.conf", "")
	writeFile(t, home, ".config/kitty/tema com acentuação.conf", "")
	if err := os.Mkdir(r.dir, 0700); err != nil {
		t.Fatal(err)
	}
	m := model{repo: r, prompt: "add", input: ".con"}
	m = press(m, tea.KeyTab)
	if m.input != ".config/" {
		t.Fatalf("complete directory: %q", m.input)
	}
	m = press(m, tea.KeyTab)
	if m.input != ".config/kitty/" {
		t.Fatalf("descend directory: %q", m.input)
	}
	m = press(m, tea.KeyTab)
	if len(m.completions) != 3 || !strings.HasSuffix(m.input, "keymap.conf") {
		t.Fatalf("file candidates: %q %#v", m.input, m.completions)
	}
	m = press(m, tea.KeyTab)
	if !strings.HasSuffix(m.input, "kitty.conf") {
		t.Fatalf("cycle: %q", m.input)
	}
	m = press(m, tea.KeyShiftTab)
	if !strings.HasSuffix(m.input, "keymap.conf") {
		t.Fatalf("reverse cycle: %q", m.input)
	}
	m.input = ".config/kitty/tema"
	m = press(m, tea.KeyTab)
	if m.input != ".config/kitty/tema com acentuação.conf" {
		t.Fatalf("spaces/unicode: %q", m.input)
	}
	for _, input := range []string{"~/", home + string(filepath.Separator)} {
		matches, err := pathCompletions(r, input)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 || !strings.HasSuffix(matches[0], ".config/") {
			t.Fatalf("home/absolute matches: %#v", matches)
		}
	}
	matches, err := pathCompletions(r, "nothing-matches")
	if err != nil || len(matches) != 0 {
		t.Fatalf("no matches: %#v %v", matches, err)
	}
	if _, err := pathCompletions(r, "../"); err == nil {
		t.Fatal("completed outside home")
	}
}

func TestTUIActionsUseVisibleShortcuts(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyF2, tea.KeyF3, tea.KeyF4, tea.KeyF5} {
		m := model{width: 80, height: 24}
		next, cmd := m.Update(tea.KeyMsg{Type: key})
		m = next.(model)
		switch key {
		case tea.KeyF2:
			if m.prompt != "add" {
				t.Fatal("F2 did not open Add")
			}
		case tea.KeyF4:
			if m.prompt != "clone" {
				t.Fatal("F4 did not open Connect")
			}
		default:
			if cmd == nil || !m.busy {
				t.Fatalf("%v did not start action", key)
			}
		}
	}
	m := model{width: 80, height: 24, missing: true}
	m = press(m, tea.KeyF1)
	if !strings.Contains(m.detail, "mydotty --att") {
		t.Fatal("CLI commands missing from help")
	}
	m = press(m, tea.KeyEsc)
	if m.detail != "" {
		t.Fatal("Esc did not close help")
	}
	m = press(m, tea.KeyF4)
	if m.prompt != "clone" {
		t.Fatal("F4 unavailable on first run")
	}
}
