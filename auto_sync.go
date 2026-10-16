package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
)

type autoSyncConfig struct {
	binary, username, unitDir, hookPath string
}

func autoSyncEnabled() bool {
	_, err := os.Stat(filepath.Join("/etc/pacman.d/hooks", "zz-mydotty-"+strconv.Itoa(os.Getuid())+".hook"))
	return err == nil
}

func configureAutoSync(enable bool, out, stderr io.Writer) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("auto-sync após atualizações está disponível em Arch Linux e CachyOS")
	}
	if os.Getuid() == 0 {
		return fmt.Errorf("execute sem sudo: a integração será configurada para seu usuário e pedirá permissão quando necessário")
	}
	for _, name := range []string{"pacman", "systemctl", "sudo"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("a integração requer pacman, systemd e sudo (falta %s)", name)
		}
	}
	probe := exec.Command("systemctl", "--user", "show-environment")
	if err := probe.Run(); err != nil {
		return fmt.Errorf("abra uma sessão do seu usuário para configurar o auto-sync")
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	config := autoSyncConfig{
		binary: binary, username: current.Username,
		unitDir:  filepath.Join(configDir, "systemd/user"),
		hookPath: filepath.Join("/etc/pacman.d/hooks", "zz-mydotty-"+current.Uid+".hook"),
	}
	return config.configure(enable, out, stderr)
}

func (c autoSyncConfig) configure(enable bool, out, stderr io.Writer) error {
	run := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("não consegui configurar o auto-sync (%s): %w", name, err)
		}
		return nil
	}
	servicePath := filepath.Join(c.unitDir, "mydotty-sync.service")
	timerPath := filepath.Join(c.unitDir, "mydotty-sync.timer")
	if !enable {
		if err := run("sudo", "rm", "-f", c.hookPath); err != nil {
			return err
		}
		// The timer may not exist if automatic synchronization was never enabled.
		_ = exec.Command("systemctl", "--user", "stop", "mydotty-sync.timer").Run()
		for _, path := range []string{servicePath, timerPath} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if err := run("systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
		fmt.Fprintln(out, "Auto-sync desativado. O MyDotty continua instalado.")
		return nil
	}
	// Quote an argument for systemd's ExecStart syntax, including its expansions.
	quotedBinary := systemdQuote(c.binary)
	service := fmt.Sprintf(`[Unit]
Description=Sincronizar dotfiles após atualizar pacotes
ConditionPathExists=%%h/.dotfiles/HEAD

[Service]
Type=oneshot
WorkingDirectory=%%h
Environment=SSH_ASKPASS_REQUIRE=never
ExecStart=/usr/bin/env %s --att
`, quotedBinary)
	timer := `[Unit]
Description=Aguardar o fim das atualizações antes de sincronizar dotfiles

[Timer]
OnActiveSec=15s
AccuracySec=1s
Unit=mydotty-sync.service
`
	if err := os.MkdirAll(c.unitDir, 0755); err != nil {
		return err
	}
	for path, content := range map[string]string{servicePath: service, timerPath: timer} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}
	if err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		if err := run("systemctl", "--user", "import-environment", "SSH_AUTH_SOCK"); err != nil {
			return err
		}
	}
	hook := fmt.Sprintf(`[Trigger]
Operation = Upgrade
Type = Package
Target = *

[Action]
Description = Agendando sincronização dos dotfiles com MyDotty...
When = PostTransaction
Depends = systemd
Exec = /usr/bin/systemctl --user --machine=%s@.host --no-block restart mydotty-sync.timer
`, c.username)
	file, err := os.CreateTemp("", "mydotty-*.hook")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.WriteString(hook)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := run("sudo", "install", "-D", "-m", "644", file.Name(), c.hookPath); err != nil {
		return err
	}
	fmt.Fprintln(out, "Auto-sync ativado! Após atualizações do pacman, suas configurações serão sincronizadas em segundo plano, com sua sessão aberta.")
	fmt.Fprintln(out, "Consultar resultado: journalctl --user -u mydotty-sync.service -n 30 --no-pager")
	return nil
}
