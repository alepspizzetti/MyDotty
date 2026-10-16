package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const updateRepository = "https://github.com/alepspizzetti/MyDotty.git"

// Filled by the installer and by the updater for subsequent version checks.
var buildRevision string

func checkUpdateRequirements(binary string) error {
	for _, name := range []string{"git", "go"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("a atualização automática requer %s instalado", name)
		}
	}
	probe, err := os.CreateTemp(filepath.Dir(binary), ".mydotty-write-*")
	if err != nil {
		return fmt.Errorf("a atualização automática precisa de permissão na pasta do executável; reinstale para seu usuário com PREFIX=\"$HOME/.local\" ./install.sh: %w", err)
	}
	probe.Close()
	return os.Remove(probe.Name())
}

func updateBinary(binary, sourceURL string, out io.Writer) error {
	if err := checkUpdateRequirements(binary); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(filepath.Dir(binary), ".mydotty-update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("outra atualização do MyDotty está em andamento: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	dir, err := os.MkdirTemp(filepath.Dir(binary), ".mydotty-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "source")
	if _, err := runGit(dir, "clone", "--depth", "1", "--", sourceURL, source); err != nil {
		return fmt.Errorf("não consegui baixar a atualização; o executável atual foi mantido: %w", err)
	}
	revision, err := runGit(source, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	revision = strings.TrimSpace(revision)
	if buildRevision == revision {
		fmt.Fprintln(out, "O MyDotty já está atualizado.")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	staged := filepath.Join(dir, "mydotty")
	cmd := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-trimpath", "-ldflags=-s -w -X main.buildRevision="+revision, "-o", staged, ".")
	cmd.Dir = source
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("não consegui compilar a atualização; o executável atual foi mantido: %w\n%s", err, output)
	}
	if output, err := exec.CommandContext(ctx, staged, "--help").CombinedOutput(); err != nil {
		return fmt.Errorf("a atualização não passou na verificação; o executável atual foi mantido: %w\n%s", err, output)
	}
	if err := os.Chmod(staged, 0755); err != nil {
		return err
	}
	if err := os.Rename(staged, binary); err != nil {
		return err
	}
	fmt.Fprintln(out, "MyDotty atualizado! A nova versão será usada na próxima execução.")
	return nil
}
