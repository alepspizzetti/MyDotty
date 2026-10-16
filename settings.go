package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

type settings struct {
	PackageSync bool `json:"package_sync"`
	LoginSync   bool `json:"login_sync"`
	LogoutSync  bool `json:"logout_sync"`
	AutoUpdate  bool `json:"auto_update"`
}

var settingOptions = []struct{ key, label, description string }{
	{"packages", "Atualização de pacotes", "Após pacman / Cachy-Update (Arch/CachyOS)."},
	{"login", "Início de sessão", "Ao iniciar a sessão gráfica ou o systemd do usuário."},
	{"logout", "Encerramento de sessão", "Ao sair da sessão gráfica. Requer rede e acesso ao Git."},
	{"update", "Ativo", "Diário. Requer Go e instalação em ~/.local/bin."},
}

type settingsResult struct {
	value settings
	err   error
}

func (m *model) openSettings() {
	m.settingsOpen = true
	s, err := loadSettings()
	m.settings = s
	if err != nil {
		m.message = err.Error()
	} else {
		m.message = "Selecione uma opção. As três formas de auto-sync podem ficar ativas juntas."
	}
}

func (m model) settingsRows(width, rows int) []string {
	lines := []string{accent.Render("AUTO-SYNC")}
	for i, option := range settingOptions {
		if i == 3 {
			lines = append(lines, accent.Render("AUTO UPDATE · MyDotty"))
		}
		check, mark := " ", " "
		if m.settings.enabled(option.key) {
			check = "x"
		}
		if i == m.settingsCursor {
			mark = "▸"
		}
		line := fit(fmt.Sprintf("%s [%s] %s", mark, check, option.label), width)
		if i == m.settingsCursor {
			line = selection.Inherit(ink).Bold(true).Render(line)
		}
		lines = append(lines, line)
	}
	selected := m.settingsCursor + 1
	if m.settingsCursor == 3 {
		selected++
	}
	start := max(0, min(selected-rows+1, len(lines)-rows))
	return lines[start:min(len(lines), start+rows)]
}

func setupInstallation(out, stderr io.Writer) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	if err := configureInstallPath(binary); err != nil {
		return err
	}
	fmt.Fprintln(out, "PATH configurado. Abra um novo terminal para usar a instalação atual.")
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		s, err := loadSettings()
		if err != nil {
			return err
		}
		if !s.AutoUpdate {
			fmt.Fprintln(out, "Auto update permanece desativado conforme suas preferências.")
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return changeSetting("update=true", out, stderr)
}

func configureInstallPath(binary string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	binDir := quote(filepath.Dir(binary))
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "fish" {
		path := filepath.Join(configDir, "fish", "conf.d", "mydotty.fish")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("fish_add_path --move --prepend --path "+binDir+"\n"), 0644)
	}
	path := filepath.Join(configDir, "mydotty", "path.sh")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := "case \"$PATH\" in\n    " + binDir + "|" + binDir + ":*) ;;\n    *) export PATH=" + binDir + ":\"$PATH\" ;;\nesac\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	profiles := []string{filepath.Join(home, ".profile")}
	switch shell {
	case "bash":
		for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
			if _, err := os.Stat(filepath.Join(home, name)); err == nil {
				profiles[0] = filepath.Join(home, name)
				break
			}
		}
		profiles = append(profiles, filepath.Join(home, ".bashrc"))
	case "zsh":
		dir := os.Getenv("ZDOTDIR")
		if dir == "" {
			dir = home
		}
		profiles = []string{filepath.Join(dir, ".zshrc")}
	}
	line := ". " + quote(path)
	for _, profile := range profiles {
		data, err := os.ReadFile(profile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if slices.Contains(strings.Split(string(data), "\n"), line) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(profile), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(profile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString("\n" + line + "\n")
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "mydotty", "config.json"), err
}

func loadSettings() (settings, error) {
	s := settings{PackageSync: autoSyncEnabled()}
	path, err := settingsPath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("não consegui ler %s: %w", path, err)
	}
	return s, nil
}

func (s settings) enabled(key string) bool {
	switch key {
	case "packages":
		return s.PackageSync
	case "login":
		return s.LoginSync
	case "logout":
		return s.LogoutSync
	case "update":
		return s.AutoUpdate
	}
	return false
}

func (s *settings) set(key string, enabled bool) error {
	switch key {
	case "packages":
		s.PackageSync = enabled
	case "login":
		s.LoginSync = enabled
	case "logout":
		s.LogoutSync = enabled
	case "update":
		s.AutoUpdate = enabled
	default:
		return fmt.Errorf("opção desconhecida: %s (use packages, login, logout ou update)", key)
	}
	return nil
}

func (s settings) save() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func changeSetting(value string, out, stderr io.Writer) error {
	key, raw, ok := strings.Cut(value, "=")
	enabled, err := strconv.ParseBool(raw)
	if !ok || err != nil {
		return fmt.Errorf("use --set OPÇÃO=true ou OPÇÃO=false (packages, login, logout, update)")
	}
	s, err := loadSettings()
	if err != nil {
		return err
	}
	previous := s
	path, err := settingsPath()
	if err != nil {
		return err
	}
	_, statErr := os.Stat(path)
	newFile := os.IsNotExist(statErr)
	if err := s.set(key, enabled); err != nil {
		return err
	}
	// Persist first so disabling an armed logout service cannot trigger a sync.
	if err := s.save(); err != nil {
		return err
	}
	if key == "packages" {
		err = configureAutoSync(enabled, out, stderr)
	} else {
		var c automationConfig
		c, err = currentAutomationConfig()
		if err == nil {
			err = c.configure(key, enabled, out, stderr)
		}
	}
	if err != nil {
		var restoreErr error
		if newFile {
			restoreErr = os.Remove(path)
		} else {
			restoreErr = previous.save()
		}
		if restoreErr != nil {
			return fmt.Errorf("%w; não consegui restaurar as opções: %v", err, restoreErr)
		}
		return err
	}
	fmt.Fprintln(out, "Configuração salva.")
	return nil
}

func systemdQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`, `$`, `$$`).Replace(value) + `"`
}

type automationConfig struct{ binary, unitDir string }

func currentAutomationConfig() (automationConfig, error) {
	if runtime.GOOS != "linux" {
		return automationConfig{}, fmt.Errorf("as automações de sessão e atualização requerem Linux com systemd")
	}
	if err := exec.Command("systemctl", "--user", "show-environment").Run(); err != nil {
		return automationConfig{}, fmt.Errorf("abra uma sessão do seu usuário com systemd para configurar as automações: %w", err)
	}
	binary, err := os.Executable()
	if err != nil {
		return automationConfig{}, err
	}
	dir, err := os.UserConfigDir()
	return automationConfig{binary, filepath.Join(dir, "systemd", "user")}, err
}

func (c automationConfig) configure(key string, enabled bool, out, stderr io.Writer) error {
	run := func(args ...string) error {
		cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
		cmd.Stdout, cmd.Stderr = out, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("não consegui configurar %s: %w", key, err)
		}
		return nil
	}
	name := "mydotty-" + key + ".service"
	files := map[string]string{}
	command := "/usr/bin/env " + systemdQuote(c.binary)
	unit := "[Unit]\nDescription=MyDotty " + key + "\n"
	if key == "login" || key == "logout" {
		unit += "PartOf=graphical-session.target\n"
	}
	service := unit + "\n[Service]\nType=oneshot\nWorkingDirectory=%h\nEnvironment=SSH_ASKPASS_REQUIRE=never\n"
	switch key {
	case "login":
		files[name] = service + "TimeoutStartSec=180\nExecStart=" + command + " --auto-run login\n\n[Install]\nWantedBy=default.target graphical-session.target\n"
	case "logout":
		files[name] = service + "RemainAfterExit=yes\nExecStart=/usr/bin/true\nExecStop=" + command + " --auto-run logout\nTimeoutStopSec=180\n\n[Install]\nWantedBy=default.target graphical-session.target\n"
	case "update":
		if enabled {
			if err := checkUpdateRequirements(c.binary); err != nil {
				return err
			}
		}
		files[name] = service + "TimeoutStartSec=600\nExecStart=" + command + " --auto-run update\n"
		name = "mydotty-update.timer"
		files[name] = "[Unit]\nDescription=Atualizar o MyDotty diariamente\n\n[Timer]\nOnCalendar=daily\nPersistent=true\nRandomizedDelaySec=5m\n\n[Install]\nWantedBy=timers.target\n"
	default:
		return fmt.Errorf("automação desconhecida: %s", key)
	}
	if !enabled {
		if _, err := os.Stat(filepath.Join(c.unitDir, name)); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
		if err := run("disable", "--now", name); err != nil {
			return err
		}
		for path := range files {
			if err := os.Remove(filepath.Join(c.unitDir, path)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return run("daemon-reload")
	}
	if err := os.MkdirAll(c.unitDir, 0755); err != nil {
		return err
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(c.unitDir, path), []byte(content), 0644); err != nil {
			return err
		}
	}
	if err := run("daemon-reload"); err != nil {
		return err
	}
	if err := run("import-environment", "PATH"); err != nil {
		return err
	}
	if os.Getenv("SSH_AUTH_SOCK") != "" && key != "update" {
		if err := run("import-environment", "SSH_AUTH_SOCK"); err != nil {
			return err
		}
	}
	args := []string{"enable"}
	if key != "login" {
		args = append(args, "--now")
	}
	return run(append(args, name)...)
}
