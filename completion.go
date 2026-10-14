package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func pathCompletions(r repository, input string) ([]string, error) {
	if input == "~" {
		return []string{"~/"}, nil
	}
	dir, prefix := filepath.Split(input)
	search := dir
	if strings.HasPrefix(search, "~/") {
		search = filepath.Join(r.home, search[2:])
	}
	if !filepath.IsAbs(search) {
		search = filepath.Join(r.home, search)
	}
	rel, err := filepath.Rel(r.home, search)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("escolha um caminho dentro da sua home")
	}
	entries, err := os.ReadDir(search)
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) || entry.Name() == ".git" || filepath.Join(search, entry.Name()) == r.dir {
			continue
		}
		name := dir + entry.Name()
		if entry.IsDir() {
			name += string(filepath.Separator)
		}
		matches = append(matches, name)
	}
	return matches, nil
}

func (m *model) completePath(reverse bool) {
	if len(m.completions) == 0 || m.input != m.lastCompletion || len(m.completions) == 1 && strings.HasSuffix(m.input, string(filepath.Separator)) {
		matches, err := pathCompletions(m.repo, m.input)
		m.completions, m.completionIndex = matches, -1
		if err != nil {
			m.completionNote = "Não consegui listar esse caminho: " + err.Error()
			return
		}
		if len(matches) == 0 {
			m.completionNote = "Nenhum caminho encontrado. Continue digitando ou confira a pasta."
			return
		}
		common := matches[0]
		for _, match := range matches[1:] {
			for !strings.HasPrefix(match, common) {
				common = common[:len(common)-1]
			}
		}
		for !utf8.ValidString(common) {
			common = common[:len(common)-1]
		}
		if !reverse && common != m.input {
			m.input, m.lastCompletion = common, common
			m.completionNote = fmt.Sprintf("%d opções · Tab próximo · Shift+Tab anterior", len(matches))
			return
		}
	}
	if reverse {
		if m.completionIndex <= 0 {
			m.completionIndex = len(m.completions)
		}
		m.completionIndex--
	} else {
		m.completionIndex = (m.completionIndex + 1) % len(m.completions)
	}
	m.input = m.completions[m.completionIndex]
	m.lastCompletion = m.input
	m.completionNote = fmt.Sprintf("%d/%d · %s · Tab próximo · Shift+Tab anterior", m.completionIndex+1, len(m.completions), displayPath(m.input))
}
