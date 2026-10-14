package main

import (
	"fmt"
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
		"auto-sync": {prompt: "auto-sync"},
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
					if m.prompt != "" && m.prompt != "auto-sync" && !strings.Contains(output, "▏") {
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
