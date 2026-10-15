package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	accent = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#087F8C", Dark: "#79E2CB"}).Bold(true)
	muted  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#637080", Dark: "#8592A3"})
	ink    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#243247", Dark: "#E3EAF3"})
	green  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#28734A", Dark: "#A6DA95"})
	amber  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#946200", Dark: "#EBCB8B"})
	red    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B5364B", Dark: "#ED8796"})
	panel  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#C3CED9", Dark: "#364355"}).Padding(0, 1)
	selection = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#E1F1EF", Dark: "#203D43"})
)

type result struct {
	state  snapshot
	text   string
	err    error
	detail bool
}
type model struct {
	repo                           repository
	state                          snapshot
	cursor, width, height, offset  int
	busy, missing, autoSync        bool
	prompt, input, message, detail string
	completions                    []string
	lastCompletion, completionNote string
	completionIndex                int
}

func (m model) perform(action func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		text, err := action()
		state, readErr := m.repo.read()
		if err == nil {
			err = readErr
		}
		return result{state: state, text: text, err: err}
	}
}

func (m model) Init() tea.Cmd {
	if m.missing {
		return nil
	}
	return m.perform(func() (string, error) { return "F2 adiciona arquivos ou pastas. F3 salva e sincroniza tudo.", nil })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.detail != "" {
			m.offset = min(m.offset, max(0, len(strings.Split(m.detail, "\n"))-m.diffPageSize()))
		}
	case result:
		m.busy = false
		if msg.detail {
			if msg.err == nil {
				m.detail, m.offset = msg.text, 0
			} else {
				m.message = msg.err.Error()
			}
			return m, nil
		}
		m.state = msg.state
		m.cursor = min(m.cursor, max(0, len(m.state.files)-1))
		_, err := os.Lstat(m.repo.dir)
		m.missing = os.IsNotExist(err)
		if msg.err != nil {
			m.message = msg.err.Error()
			var conflict *syncConflict
			if errors.As(msg.err, &conflict) {
				m.prompt = "conflict"
				m.detail, m.offset = "Escolha a versão para TODOS os arquivos em conflito:\n"+strings.Join(conflict.paths, "\n"), 0
			}
		} else {
			m.message = strings.TrimSpace(msg.text)
			if m.message == "" {
				m.message = "Pronto. Lista atualizada."
			}
		}
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.prompt == "conflict" {
			switch key {
			case "up", "down", "pgup", "pgdown":
				limit := max(0, len(strings.Split(m.detail, "\n"))-m.diffPageSize())
				switch key {
				case "up":
					m.offset = max(0, m.offset-1)
				case "down":
					m.offset = min(limit, m.offset+1)
				case "pgup":
					m.offset = max(0, m.offset-m.diffPageSize())
				case "pgdown":
					m.offset = min(limit, m.offset+m.diffPageSize())
				}
			case "esc":
				m.prompt, m.detail = "", ""
			case "l", "r":
				keep := "local"
				if key == "r" {
					keep = "remote"
				}
				m.prompt, m.detail, m.busy = "", "", true
				return m, m.perform(func() (string, error) { return m.repo.syncKeeping(keep) })
			}
			return m, nil
		}
		if m.prompt == "auto-sync" {
			switch key {
			case "esc":
				m.prompt = ""
			case "enter":
				binary, err := os.Executable()
				if err != nil {
					m.message = err.Error()
					m.prompt = ""
					return m, nil
				}
				flag, message := "--auto-sync", "Auto-sync ativado para as próximas atualizações do sistema."
				if m.autoSync {
					flag, message = "--no-auto-sync", "Auto-sync desativado."
				}
				m.prompt, m.busy = "", true
				var details bytes.Buffer
				command := exec.Command(binary, flag)
				command.Stderr = io.MultiWriter(os.Stderr, &details)
				return m, tea.ExecProcess(command, func(err error) tea.Msg {
					if err != nil {
						err = fmt.Errorf("a configuração do auto-sync não foi concluída: %v\n%s", err, strings.TrimSpace(details.String()))
					}
					return result{state: m.state, text: message, err: err}
				})
			}
			return m, nil
		}
		if m.prompt != "" {
			switch key {
			case "esc":
				m.prompt, m.input, m.completionNote = "", "", ""
				m.completions = nil
			case "tab", "shift+tab":
				if m.prompt == "add" {
					m.completePath(key == "shift+tab")
				}
			case "backspace":
				m.completions, m.completionNote = nil, ""
				runes := []rune(m.input)
				if len(runes) > 0 {
					m.input = string(runes[:len(runes)-1])
				}
			case "enter":
				value := m.input
				if m.prompt == "commit" || m.prompt == "clone" {
					value = strings.TrimSpace(value)
				}
				if value == "" {
					return m, nil
				}
				prompt := m.prompt
				m.prompt, m.input, m.completionNote = "", "", ""
				m.completions = nil
				m.busy = true
				switch prompt {
				case "add":
					return m, m.perform(func() (string, error) { return m.repo.add(value) })
				case "remove":
					return m, m.perform(func() (string, error) { return m.repo.remove(value) })
				case "commit":
					return m, m.perform(func() (string, error) { return m.repo.git("commit", "-m", value) })
				case "clone":
					return m, m.perform(func() (string, error) { return m.repo.setup(value) })
				}
			default:
				m.completions, m.completionNote = nil, ""
				if msg.Type == tea.KeyRunes {
					m.input += string(msg.Runes)
				} else if msg.Type == tea.KeySpace {
					m.input += " "
				}
			}
			return m, nil
		}
		if key == "q" || key == "esc" && m.detail == "" {
			return m, tea.Quit
		}
		if m.detail != "" {
			limit := max(0, len(strings.Split(m.detail, "\n"))-m.diffPageSize())
			switch key {
			case "esc", "enter":
				m.detail = ""
			case "down", "j":
				m.offset = min(limit, m.offset+1)
			case "up", "k":
				m.offset = max(0, m.offset-1)
			case "pgdown":
				m.offset = min(limit, m.offset+m.diffPageSize())
			case "pgup":
				m.offset = max(0, m.offset-m.diffPageSize())
			}
			return m, nil
		}
		if key == "f1" && m.detail == "" {
			m.detail, m.offset = commandHelp, 0
			return m, nil
		}
		if key == "f6" {
			m.autoSync, m.prompt = autoSyncEnabled(), "auto-sync"
			return m, nil
		}
		if m.missing {
			if key == "n" || key == "f4" || key == "enter" {
				m.prompt = "clone"
			}
			return m, nil
		}
		if m.state.needsCheckout && key != "i" && key != "f4" && key != "r" && key != "f5" {
			return m, nil
		}
		switch key {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(max(0, len(m.state.files)-1), m.cursor+1)
		case "a", "f2":
			m.prompt = "add"
		case "f3", "s":
			m.busy = true
			return m, m.perform(m.repo.sync)
		case "f4":
			if m.state.needsCheckout {
				m.busy = true
				return m, m.perform(m.repo.apply)
			}
			m.prompt = "clone"
		case "delete":
			if len(m.state.files) > 0 {
				m.prompt = "remove"
				m.input = m.state.files[m.cursor].path
			}
		case "c":
			m.prompt = "commit"
		case "enter", "d":
			if len(m.state.files) > 0 {
				path := m.state.files[m.cursor].path
				m.busy = true
				return m, func() tea.Msg { text, err := m.repo.diff(path); return result{text: text, err: err, detail: true} }
			}
		case "u":
			if len(m.state.files) > 0 {
				path := m.state.files[m.cursor].path
				m.busy = true
				return m, m.perform(func() (string, error) { return m.repo.unstage(path) })
			}
		case "p":
			m.busy = true
			return m, m.perform(func() (string, error) { return m.repo.git("push", "-u", "origin", "HEAD") })
		case "l":
			if m.state.branch == "" {
				m.message = "Atualize a lista antes de baixar."
				return m, nil
			}
			m.busy = true
			return m, m.perform(m.repo.sync)
		case "i":
			m.busy = true
			return m, m.perform(m.repo.apply)
		case "r", "f5":
			m.busy = true
			return m, m.perform(func() (string, error) { return "Lista atualizada.", nil })
		}
	}
	return m, nil
}

func safe(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, ansi.Strip(text))
}

// One terminal row per path, even when the filename contains control characters.
func displayPath(path string) string {
	return strings.ReplaceAll(safe(path), "\n", "\\n")
}

func fit(text string, width int) string {
	text = ansi.Truncate(text, max(0, width), "…")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}

func spread(left, right string, width int) string {
	right = ansi.Truncate(right, max(0, width/2), "…")
	return fit(left, max(0, width-ansi.StringWidth(right)-1)) + " " + right
}

func shortcuts(items []string, width int) string {
	var lines []string
	line := ""
	for _, item := range items {
		key, label, _ := strings.Cut(item, " ")
		token := selection.Inherit(accent).Bold(true).Render(" "+key+" ") + ink.Render(" "+label)
		if line != "" && ansi.StringWidth(line)+2+ansi.StringWidth(token) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += "  "
		}
		line += token
	}
	return strings.Join(append(lines, line), "\n")
}

const commandHelp = `MYDOTTY · GUIA RÁPIDO

1. Crie um repositório no GitHub, GitLab ou outro serviço.
2. Conecte a URL com F4. O MyDotty prepara o .dotfiles.
3. Adicione arquivos ou pastas com F2 (Tab completa).
4. Use F3 para salvar, baixar e enviar suas alterações.

NO TERMINAL                    NA INTERFACE
mydotty --init URL              F4  Conectar
mydotty --add CAMINHO           F2  Adicionar
mydotty --att / --sync          F3  Sincronizar
mydotty --status                F5  Atualizar lista
mydotty --diff CAMINHO          Enter  Ver alterações
mydotty --remove CAMINHO        Del  Não acompanhar
mydotty --apply                 F4  Repetir instalação
mydotty --help                  F1  Comandos
mydotty --auto-sync             F6  Ativar auto-sync
mydotty --no-auto-sync          F6  Desativar auto-sync

CONFLITOS AO SINCRONIZAR
L mantém arquivos inteiros desta máquina; R, do remoto.
Esc cancela. As duas versões continuam no histórico.
No terminal, --att pergunta quando houver conflito.
Sem interação: --att --keep local / --att --keep remote.

CAMINHOS
Relativos à sua home, ou use ~/ e caminhos absolutos.
Exemplo: mydotty --add .config/kitty
Pastas incluem seus arquivos atuais. Adicione novamente
para incluir novos arquivos criados dentro delas.

AUTOCOMPLETE NA INTERFACE
Tab completa o caminho; Tab novamente percorre opções.
Shift+Tab percorre no sentido contrário.
Espaços e acentos são aceitos sem aspas.

Sincronizar salva somente arquivos já adicionados.
Não acompanhar mantém os arquivos no computador.
Para sair, use Esc na tela principal ou Ctrl+C.`

func fileStatus(status string) (string, lipgloss.Style) {
	switch {
	case strings.Contains(status, "U"), status == "AA", status == "DD":
		return "conflito", red
	case strings.Contains(status, "D"):
		return "removido", red
	case strings.Contains(status, "A"):
		return "novo", green
	case status != "  ":
		return "alterado", amber
	default:
		return "salvo", muted
	}
}

func (m model) help() string {
	keys := []string{"F2 Adicionar", "F3 Sincronizar", "F4 Conectar", "F6 Auto-sync", "Enter Ver alterações", "Del Não acompanhar", "F1 Comandos", "F5 Atualizar lista", "Esc Sair"}
	if m.detail != "" {
		keys = []string{"↑↓ Rolar", "PgUp/PgDn Página", "Esc Voltar", "q Sair"}
	} else if m.missing {
		keys = []string{"F4 Conectar repositório", "F6 Auto-sync", "F1 Comandos", "Esc Sair"}
	} else if m.state.needsCheckout {
		keys = []string{"F4 Instalar arquivos", "F6 Auto-sync", "F1 Comandos", "Esc Sair"}
	}
	if m.prompt != "" {
		keys = []string{"Enter Confirmar", "Esc Cancelar"}
		if m.prompt == "conflict" {
			keys = []string{"L Manter local", "R Manter remoto", "↑↓ Rolar lista", "Esc Cancelar"}
		}
		if m.prompt == "add" {
			keys = append([]string{"Tab Completar caminho"}, keys...)
		}
	}
	return shortcuts(keys, m.width-4)
}

func (m model) diffPageSize() int {
	padding := 0
	if m.height >= 20 {
		padding = 2
	}
	return max(1, m.height-padding-9-lipgloss.Height(m.help()))
}

func (m model) View() string {
	if m.width < 60 || m.height < 16 {
		lines := []string{"MyDotty", "Amplie o terminal para 60×16.", "q sair · Ctrl+C sair"}
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], max(0, m.width), "…")
		}
		return strings.Join(lines[:min(len(lines), max(0, m.height))], "\n")
	}
	width := m.width - 4
	inner := width - 4
	title := accent.Render("◈ MyDotty") + muted.Render("  /  sua casa, versionada")
	badge := muted.Render("SUAS CONFIGURAÇÕES")
	if m.busy {
		badge = amber.Render("● EXECUTANDO")
	}
	meta := muted.Render("Destino: ") + accent.Render("ainda não conectado")
	if m.state.remote != "" {
		meta = muted.Render("Destino: ") + accent.Render(displayPath(m.state.remote))
	}
	header := spread(title, badge, width) + "\n" + muted.Render(fit(displayPath(m.repo.dir), width)) + "\n" + fit(meta, width)

	notice := "\n" + muted.Render(fit("Pronto para cuidar dos seus dotfiles.", width)) + "\n"
	if m.message != "" {
		lines := strings.Split(ansi.Wrap(safe(m.message), width, ""), "\n")
		for len(lines) < 3 {
			lines = append(lines, "")
		}
		notice = ink.Render(fit(lines[0], width)) + "\n" + muted.Render(fit(lines[1], width)) + "\n" + muted.Render(fit(lines[2], width))
	}
	if m.busy {
		notice = "\n" + amber.Render("● Cuidando das suas configurações…") + "\n" + muted.Render("A lista será atualizada ao concluir.")
	}
	if m.prompt != "" {
		labels := map[string]string{"add": "ADICIONAR · arquivo ou pasta da sua home", "commit": "SALVAR · descrição das alterações", "clone": "CONECTAR · cole a URL do repositório que você criou", "remove": "PARAR DE ACOMPANHAR · mantém o arquivo nesta máquina"}
		input := displayPath(m.input)
		input = ansi.TruncateLeft(input, max(0, ansi.StringWidth(input)-width+6), "…")
		notice = accent.Render(fit(labels[m.prompt], width)) + "\n" + selection.Render(fit(" › "+input+"▏", width)) + "\n" + muted.Render(fit(m.completionNote, width))
	}
	if m.prompt == "auto-sync" {
		status, action := "DESATIVADO", "ativar"
		if m.autoSync {
			status, action = "ATIVADO", "desativar"
		}
		notice = accent.Render(fit("AUTO-SYNC · "+status, width)) + "\n" + ink.Render(fit("Sincroniza após atualizações do pacman / Cachy-Update.", width)) + "\n" + muted.Render(fit("Enter para "+action+" · pode solicitar sua senha do sudo", width))
	}
	if m.prompt == "conflict" {
		notice = accent.Render(fit("CONFLITO · qual versão deve ficar ativa?", width)) + "\n" + ink.Render(fit("L: desta máquina · R: remoto (arquivos inteiros)", width)) + "\n" + muted.Render(fit("Esc cancela e preserva as duas versões no histórico.", width))
	}
	help := m.help()
	padding := 0
	if m.height >= 20 {
		padding = 1
	}
	// Header, notification and shortcuts have measured heights; the panel uses the rest.
	bodyHeight := m.height - 2*padding - lipgloss.Height(header) - lipgloss.Height(notice) - lipgloss.Height(help)
	rows := bodyHeight - 2
	var body []string
	if m.detail != "" {
		lines := strings.Split(safe(m.detail), "\n")
		page := m.diffPageSize()
		offset := min(m.offset, max(0, len(lines)-page))
		body = append(body, spread(accent.Render("ALTERAÇÕES / AJUDA"), muted.Render(fmt.Sprintf("%d / %d", offset+1, len(lines))), inner))
		for _, line := range lines[offset:min(len(lines), offset+min(page, rows-1))] {
			style := ink
			switch {
			case strings.HasPrefix(line, "+"):
				style = green
			case strings.HasPrefix(line, "-"):
				style = red
			case strings.HasPrefix(line, "@@"):
				style = accent
			case strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
				style = muted
			case line == "ALTERAÇÕES SELECIONADAS", line == "EDIÇÕES NO COMPUTADOR":
				style = amber.Bold(true)
			}
			body = append(body, style.Render(fit(line, inner)))
		}
	} else if m.missing || m.state.needsCheckout || len(m.state.files) == 0 {
		heading, description, hint := "Sua casa merece um histórico.", "Conecte seu repositório para trazer seus dotfiles.", "Pressione F4 e cole a URL do seu repositório."
		if m.state.needsCheckout {
			heading, description, hint = "Tudo pronto para instalar.", "Mova os caminhos existentes para um backup.", "Pressione F4 para instalar as configurações."
		} else if !m.missing {
			heading, description, hint = "Um bom começo, uma casa organizada.", "Nenhum arquivo está sendo acompanhado ainda.", "Pressione F2 para adicionar um arquivo ou pasta."
		}
		for len(body) < max(0, (rows-5)/2) {
			body = append(body, "")
		}
		for _, line := range []string{accent.Render("◈"), "", ink.Bold(true).Render(heading), muted.Render(description), accent.Render(hint)} {
			body = append(body, lipgloss.PlaceHorizontal(inner, lipgloss.Center, ansi.Truncate(line, inner, "…")))
		}
	} else {
		changed := 0
		for _, f := range m.state.files {
			if f.status != "  " {
				changed++
			}
		}
		counts := amber.Render(fmt.Sprintf("%d para salvar", changed))
		body = append(body, spread(accent.Render("MINHAS CONFIGURAÇÕES"), counts, inner))
		body = append(body, muted.Render(fit("   SITUAÇÃO        ARQUIVO / PASTA", inner)))
		count := max(1, rows-3)
		start := max(0, min(m.cursor-count/2, len(m.state.files)-count))
		for i := start; i < min(len(m.state.files), start+count); i++ {
			f := m.state.files[i]
			mark := " "
			if i == m.cursor {
				mark = "▸"
			}
			label, style := fileStatus(f.status)
			path := displayPath(f.path)
			dir, name := filepath.Split(path)
			line := accent.Render(mark) + " " + style.Render(fmt.Sprintf("%-14s", label)) + " " + muted.Render(dir) + ink.Render(name)
			line = fit(line, inner)
			if i == m.cursor {
				line = selection.Inherit(ink).Bold(true).Render(ansi.Strip(line))
			}
			body = append(body, line)
		}
		for len(body) < rows-1 {
			body = append(body, "")
		}
		body = append(body, spread(muted.Render("F3 salva, baixa e envia tudo"), muted.Render(fmt.Sprintf("%d–%d / %d", start+1, min(len(m.state.files), start+count), len(m.state.files))), inner))
	}
	for len(body) < rows {
		body = append(body, "")
	}
	for i := range body {
		body[i] = fit(body[i], inner)
	}
	box := panel.Render(strings.Join(body[:rows], "\n"))
	return lipgloss.NewStyle().Padding(padding, 2).Render(strings.Join([]string{header, box, notice, help}, "\n"))
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
