package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

func run(args []string, out, stderr io.Writer) int {
	var input io.Reader
	if term.IsTerminal(os.Stdin.Fd()) {
		input = os.Stdin
	}
	return runWithInput(args, input, out, stderr)
}

func runWithInput(args []string, input io.Reader, out, stderr io.Writer) int {
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
	add := flags.String("add", "", "adicionar, salvar e sincronizar um arquivo ou pasta: --add CAMINHO")
	remove := flags.String("remove", "", "deixar de acompanhar, mantendo os arquivos: --remove CAMINHO")
	diff := flags.String("diff", "", "ver alterações de um arquivo: --diff CAMINHO")
	att := flags.Bool("att", false, "salvar, baixar e enviar todas as alterações acompanhadas")
	flags.BoolVar(att, "sync", false, "sinônimo de --att")
	keep := flags.String("keep", "", "com --add ou --att, resolver conflitos mantendo arquivos inteiros: local ou remote")
	status := flags.Bool("status", false, "listar arquivos acompanhados e suas alterações")
	autoEnable := flags.Bool("auto-sync", false, "ativar sincronização após atualizações do pacman (Arch/CachyOS)")
	autoDisable := flags.Bool("no-auto-sync", false, "desativar sincronização após atualizações do pacman")
	settingsView := flags.Bool("settings", false, "abrir a aba de configurações")
	setOption := flags.String("set", "", "configurar automação: packages, login, logout ou update (ex.: --set login=true)")
	update := flags.Bool("update", false, "atualizar o programa MyDotty")
	installSetup := flags.Bool("install-setup", false, "configurar PATH e atualização automática após instalar (uso do instalador)")
	autoRun := flags.String("auto-run", "", "executar uma automação ativa: login, logout ou update (uso do systemd)")
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
		case "init", "add", "remove", "diff", "att", "sync", "status", "apply", "auto-sync", "no-auto-sync", "settings", "set", "update", "auto-run", "install-setup":
			actions++
		}
	})
	if flags.NArg() != 0 || actions > 1 {
		fmt.Fprintln(stderr, "Escolha uma ação por vez. Use mydotty --help para ver exemplos.")
		return 2
	}
	if *keep != "" && ((!*att && *add == "") || (*keep != "local" && *keep != "remote")) {
		fmt.Fprintln(stderr, "Use --keep local ou --keep remote junto com --add / --att / --sync.")
		return 2
	}
	if *autoEnable || *autoDisable || *setOption != "" || *update || *installSetup {
		var actionErr error
		switch {
		case *installSetup:
			actionErr = setupInstallation(out, stderr)
		case *update:
			binary, err := os.Executable()
			if err == nil {
				actionErr = updateBinary(binary, updateRepository, out)
			} else {
				actionErr = err
			}
		case *setOption != "":
			actionErr = changeSetting(*setOption, out, stderr)
		default:
			actionErr = changeSetting(fmt.Sprintf("packages=%t", *autoEnable), out, stderr)
		}
		if err := actionErr; err != nil {
			fmt.Fprintln(stderr, safe(err.Error()))
			return 1
		}
		return 0
	}
	if *autoRun != "" {
		if *autoRun != "login" && *autoRun != "logout" && *autoRun != "update" {
			fmt.Fprintln(stderr, "Use --auto-run login, logout ou update.")
			return 2
		}
		s, err := loadSettings()
		if err != nil {
			fmt.Fprintln(stderr, safe(err.Error()))
			return 1
		}
		if !s.enabled(*autoRun) {
			return 0
		}
		if *autoRun == "update" {
			binary, err := os.Executable()
			if err == nil {
				err = updateBinary(binary, updateRepository, out)
			}
			if err != nil {
				fmt.Fprintln(stderr, safe(err.Error()))
				return 1
			}
			return 0
		}
		*att = true
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
	if actions == 0 || *settingsView {
		_, err = os.Lstat(repoDir)
		missing := os.IsNotExist(err)
		m := model{repo: r, width: 80, height: 24, missing: missing, busy: !missing}
		if *settingsView {
			m.openSettings()
		}
		_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	} else {
		var text string
		switch {
		case strings.TrimSpace(*url) != "":
			text, err = r.setup(strings.TrimSpace(*url))
		case *add != "":
			text, err = r.addAndSync(*add, *keep)
		case *remove != "":
			text, err = r.remove(*remove)
		case *diff != "":
			if _, err = r.read(); err == nil {
				text, err = r.diff(*diff)
			}
		case *att:
			text, err = r.syncKeeping(*keep)
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
		var conflict *syncConflict
		if *keep == "" && input != nil && errors.As(err, &conflict) {
			fmt.Fprintln(stderr, safe(err.Error()))
			scanner := bufio.NewScanner(input)
			for {
				fmt.Fprint(stderr, "Qual versão manter em TODOS os arquivos em conflito? [L] Local / [R] Remoto / [C] Cancelar: ")
				if !scanner.Scan() {
					if scanErr := scanner.Err(); scanErr != nil {
						fmt.Fprintln(stderr, safe(scanErr.Error()))
					}
					return 1
				}
				choice := strings.ToLower(strings.TrimSpace(scanner.Text()))
				switch choice {
				case "l", "local":
					choice = "local"
				case "r", "remote", "remoto":
					choice = "remote"
				case "", "c", "cancelar":
					fmt.Fprintln(stderr, "Cancelado. As duas versões foram preservadas.")
					return 1
				default:
					fmt.Fprintln(stderr, "Digite L, R ou C e pressione Enter.")
					continue
				}
				text, err = r.syncKeeping(choice)
				break
			}
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
