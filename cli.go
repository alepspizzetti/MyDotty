package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func run(args []string, out, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	flags := flag.NewFlagSet("mydotty", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("repo", filepath.Join(home, ".dotfiles"), "pasta interna do MyDotty")
	root := flags.String("home", home, "pasta que contém suas configurações")
	url := flags.String("init", "", "conectar ao repositório: --init URL")
	add := flags.String("add", "", "acompanhar um arquivo ou pasta: --add CAMINHO")
	remove := flags.String("remove", "", "deixar de acompanhar, mantendo os arquivos: --remove CAMINHO")
	diff := flags.String("diff", "", "ver alterações de um arquivo: --diff CAMINHO")
	att := flags.Bool("att", false, "salvar, baixar e enviar todas as alterações acompanhadas")
	flags.BoolVar(att, "sync", false, "sinônimo de --att")
	status := flags.Bool("status", false, "listar arquivos acompanhados e suas alterações")
	apply := flags.Bool("apply", false, "tentar instalar novamente após mover arquivos em conflito")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "MyDotty · suas configurações salvas e sincronizadas\n\nUse mydotty para abrir a interface, ou escolha uma ação:")
		flags.PrintDefaults()
		fmt.Fprintln(stderr, "\nPrimeiro uso: mydotty --init URL_DO_SEU_REPOSITORIO\nDepois: mydotty --add .config/kitty\nSincronizar: mydotty --att\nCaminhos relativos partem da sua home. Use ~/ ou caminhos absolutos também.")
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	actions := 0
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "init", "add", "remove", "diff", "att", "sync", "status", "apply":
			actions++
		}
	})
	if flags.NArg() != 0 || actions > 1 {
		fmt.Fprintln(stderr, "Escolha uma ação por vez. Use mydotty --help para ver exemplos.")
		return 2
	}
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(stderr, "Instale o Git para usar o MyDotty.")
		return 1
	}
	repoDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	workTree, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r := repository{repoDir, workTree}
	if actions == 0 {
		_, err = os.Lstat(repoDir)
		missing := os.IsNotExist(err)
		m := model{repo: r, width: 80, height: 24, missing: missing, busy: !missing}
		_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	} else {
		var text string
		switch {
		case strings.TrimSpace(*url) != "":
			text, err = r.setup(strings.TrimSpace(*url))
		case *add != "":
			text, err = r.add(*add)
		case *remove != "":
			text, err = r.remove(*remove)
		case *diff != "":
			if _, err = r.read(); err == nil {
				text, err = r.diff(*diff)
			}
		case *att:
			text, err = r.sync()
		case *apply:
			text, err = r.apply()
		case *status:
			var state snapshot
			state, err = r.read()
			if err == nil {
				if state.needsCheckout {
					text = "Instalação pendente. Mova os caminhos em conflito e use mydotty --apply.\n"
				}
				for _, file := range state.files {
					label, _ := fileStatus(file.status)
					text += fmt.Sprintf("%-12s %s\n", label, displayPath(file.path))
				}
				if len(state.files) == 0 && !state.needsCheckout {
					text = "Nenhum arquivo acompanhado. Use mydotty --add CAMINHO."
				}
			}
		default:
			err = fmt.Errorf("informe o valor da ação. Veja mydotty --help")
		}
		if err == nil {
			fmt.Fprintln(out, strings.TrimSpace(safe(text)))
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, safe(err.Error()))
		return 1
	}
	return 0
}
