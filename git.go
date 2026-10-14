package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type repository struct{ dir, home string }
type fileEntry struct{ path, status string }
type snapshot struct {
	files          []fileEntry
	branch, remote string
	needsCheckout  bool
}

func runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_LITERAL_PATHSPECS=1")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("Git excedeu o tempo limite")
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

func (r repository) git(args ...string) (string, error) {
	return runGit(r.home, append([]string{"--git-dir=" + r.dir, "--work-tree=" + r.home}, args...)...)
}

func (r repository) read() (snapshot, error) {
	s := snapshot{}
	bare, err := r.git("config", "--get", "core.bare")
	if err != nil {
		return s, fmt.Errorf("não consegui abrir as configurações. Use F4 ou mydotty --init URL para conectar: %w", err)
	}
	if strings.TrimSpace(bare) != "true" {
		return s, fmt.Errorf("%s precisa ser um repositório Git bare", r.dir)
	}
	tracked, err := r.git("ls-files", "-z")
	if err != nil {
		return s, err
	}
	status, err := r.git("status", "--porcelain=v1", "-z", "--untracked-files=no", "--no-renames")
	if err != nil {
		return s, err
	}
	files := map[string]string{}
	for _, path := range strings.Split(tracked, "\x00") {
		if path != "" {
			files[path] = "  "
		}
	}
	for _, record := range strings.Split(status, "\x00") {
		if len(record) >= 4 {
			files[record[3:]] = record[:2]
		}
	}
	for path, status := range files {
		s.files = append(s.files, fileEntry{path, status})
	}
	sort.Slice(s.files, func(i, j int) bool { return s.files[i].path < s.files[j].path })
	s.branch, err = r.git("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return s, fmt.Errorf("selecione uma branch no Git antes de usar o MyDotty: %w", err)
	}
	s.branch = strings.TrimSpace(s.branch)
	s.remote, _ = r.git("config", "--get", "remote.origin.url")
	s.remote = strings.TrimSpace(s.remote)
	if _, err := os.Lstat(filepath.Join(r.dir, "index")); os.IsNotExist(err) {
		_, err := r.git("rev-parse", "--verify", "HEAD")
		s.needsCheckout = err == nil
	}
	return s, nil
}

func (r repository) unstage(path string) (string, error) {
	if _, err := r.git("rev-parse", "--verify", "HEAD"); err != nil {
		return r.git("rm", "--cached", "--", path)
	}
	return r.git("restore", "--staged", "--", path)
}

// Explicit files and folders are accepted, but never the entire home or Git metadata.
func (r repository) relativePath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(r.home, path[2:])
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.home, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(r.home, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("escolha um arquivo ou pasta dentro de %s", r.home)
	}
	metadata, _ := filepath.Rel(r.dir, path)
	if metadata == "." || (metadata != ".." && !strings.HasPrefix(metadata, ".."+string(filepath.Separator))) || rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
		return "", fmt.Errorf("os dados internos do Git não podem ser selecionados")
	}
	parent, _ := filepath.Rel(path, r.dir)
	if parent != ".." && !strings.HasPrefix(parent, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("essa pasta contém os dados internos do MyDotty; escolha uma subpasta")
	}
	return rel, nil
}

func (r repository) add(path string) (string, error) {
	rel, err := r.relativePath(path)
	if err != nil {
		return "", err
	}
	_, err = os.Lstat(filepath.Join(r.home, rel))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := r.ensure(); err != nil {
		return "", err
	}
	if _, err := r.git("add", "--", rel); err != nil {
		return "", err
	}
	return "Adicionado. Use Sincronizar (F3 ou --att) para salvar e enviar.", nil
}

func (r repository) diff(path string) (string, error) {
	path, err := r.relativePath(path)
	if err != nil {
		return "", err
	}
	staged, err := r.git("diff", "--cached", "--no-ext-diff", "--no-textconv", "--color=never", "--", path)
	if err != nil {
		return "", err
	}
	working, err := r.git("diff", "--no-ext-diff", "--no-textconv", "--color=never", "--", path)
	if err != nil {
		return "", err
	}
	return "ALTERAÇÕES SELECIONADAS\n" + staged + "\nEDIÇÕES NO COMPUTADOR\n" + working, nil
}

func (r repository) clone(url string) (string, error) {
	if _, err := os.Lstat(r.dir); !os.IsNotExist(err) {
		return "", fmt.Errorf("o destino já existe: %s", r.dir)
	}
	if _, err := runGit(r.home, "clone", "--bare", "--", url, r.dir); err != nil {
		return "", err
	}
	if err := r.configure(); err != nil {
		return "", err
	}
	if _, err := r.git("rev-parse", "--verify", "HEAD"); err != nil {
		return "Conectado! Adicione seus arquivos ou pastas e depois sincronize.", nil
	}
	if _, err := r.apply(); err != nil {
		return "", err
	}
	return "Conectado! Suas configurações foram instaladas nesta máquina.", nil
}

// This is only for the first checkout. Existing files are never overwritten.
func (r repository) apply() (string, error) {
	if _, err := os.Lstat(filepath.Join(r.dir, "index")); !os.IsNotExist(err) {
		return "", fmt.Errorf("o repositório já tem arquivos preparados; aplicar é apenas para a primeira instalação")
	}
	paths, err := r.git("ls-tree", "-rz", "--name-only", "HEAD")
	if err != nil {
		return "", err
	}
	for _, path := range strings.Split(paths, "\x00") {
		if path == "" {
			continue
		}
		// Check parent components too: a symlink must not redirect the checkout.
		for part := filepath.Join(r.home, path); part != r.home; part = filepath.Dir(part) {
			info, err := os.Lstat(part)
			if err == nil && (part == filepath.Join(r.home, path) || !info.IsDir()) {
				return "", fmt.Errorf("já existe: %s. Faça um backup e mova esse caminho antes de aplicar (tecla i)", part)
			}
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	return r.git("checkout")
}

// Configuration stays local to this repository; no global Git settings are changed.
func (r repository) configure() error {
	settings := map[string]string{
		"status.showUntrackedFiles": "no",
	}
	if url, _ := r.git("config", "--get", "remote.origin.url"); strings.TrimSpace(url) != "" {
		settings["remote.origin.fetch"] = "+refs/heads/*:refs/remotes/origin/*"
	}
	for key, fallback := range map[string]string{"user.name": "MyDotty", "user.email": "mydotty@localhost"} {
		value, _ := r.git("config", "--get", key)
		if strings.TrimSpace(value) == "" {
			settings[key] = fallback
		}
	}
	for key, value := range settings {
		if _, err := r.git("config", key, value); err != nil {
			return err
		}
	}
	return nil
}

func (r repository) ensure() error {
	if _, err := os.Lstat(r.dir); os.IsNotExist(err) {
		if _, err := runGit(r.home, "init", "--bare", "-b", "main", r.dir); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	state, err := r.read()
	if err != nil {
		return err
	}
	if state.needsCheckout {
		return fmt.Errorf("faltam instalar os arquivos: use F4 ou mydotty --apply")
	}
	return r.configure()
}

func (r repository) setup(url string) (string, error) {
	if _, err := os.Lstat(r.dir); os.IsNotExist(err) {
		return r.clone(url)
	}
	if err := r.ensure(); err != nil {
		return "", err
	}
	remote, _ := r.git("config", "--get", "remote.origin.url")
	if strings.TrimSpace(remote) != "" && strings.TrimSpace(remote) != url {
		return "", fmt.Errorf("já conectado a %s; use a URL desse repositório", strings.TrimSpace(remote))
	}
	if strings.TrimSpace(remote) == "" {
		if _, err := r.git("remote", "add", "origin", url); err != nil {
			return "", err
		}
	}
	refs, err := r.git("ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", fmt.Errorf("não foi possível acessar o repositório. Verifique a URL e seu acesso: %w", err)
	}
	if _, err := r.git("rev-parse", "--verify", "HEAD"); err != nil {
		for _, line := range strings.Split(refs, "\n") {
			if !strings.HasPrefix(line, "ref: refs/heads/") {
				continue
			}
			ref := strings.Fields(line)[1]
			if _, err := r.git("symbolic-ref", "HEAD", ref); err != nil {
				return "", err
			}
			tracked, err := r.git("ls-files", "-z")
			if err != nil {
				return "", err
			}
			if tracked != "" {
				break
			}
			if _, err := r.git("fetch", "origin"); err != nil {
				return "", err
			}
			remoteHead, err := r.git("rev-parse", "--verify", "refs/remotes/origin/"+strings.TrimPrefix(ref, "refs/heads/"))
			if err != nil {
				return "", err
			}
			if err := os.Remove(filepath.Join(r.dir, "index")); err != nil && !os.IsNotExist(err) {
				return "", err
			}
			if _, err := r.git("update-ref", "HEAD", strings.TrimSpace(remoteHead)); err != nil {
				return "", err
			}
			if _, err := r.apply(); err != nil {
				return "", err
			}
			break
		}
	}
	return "Conectado! Use Sincronizar (F3 ou --att) para salvar, baixar e enviar.", nil
}

func (r repository) remove(path string) (string, error) {
	path, err := r.relativePath(path)
	if err != nil {
		return "", err
	}
	if err := r.ensure(); err != nil {
		return "", err
	}
	if _, err := r.git("rm", "-r", "--cached", "--", path); err != nil {
		return "", err
	}
	return "Deixei de acompanhar esse caminho. Os arquivos continuam nesta máquina. Sincronize para atualizar o repositório.", nil
}

func (r repository) sync() (string, error) {
	if err := r.ensure(); err != nil {
		return "", err
	}
	state, err := r.read()
	if err != nil {
		return "", err
	}
	if state.remote == "" {
		return "", fmt.Errorf("primeiro conecte seu repositório: F4 ou mydotty --init URL")
	}
	unmerged, err := r.git("ls-files", "--unmerged")
	if err != nil {
		return "", err
	}
	if unmerged != "" {
		return "", fmt.Errorf("há um conflito pendente no repositório; resolva-o antes de sincronizar")
	}
	if _, err := r.git("add", "--update"); err != nil {
		return "", err
	}
	changed, err := r.git("diff", "--cached", "--name-only", "-z")
	if err != nil {
		return "", err
	}
	if changed != "" {
		if _, err := r.git("commit", "-m", "Atualiza dotfiles · "+time.Now().Format("2006-01-02 15:04:05")); err != nil {
			return "", err
		}
	}
	if _, err := r.git("fetch", "origin"); err != nil {
		return "", fmt.Errorf("não consegui baixar as novidades. Suas alterações estão salvas nesta máquina; tente sincronizar novamente.\n%w", err)
	}
	remoteRef := "refs/remotes/origin/" + state.branch
	remoteHead, remoteErr := r.git("rev-parse", "--verify", remoteRef)
	_, headErr := r.git("rev-parse", "--verify", "HEAD")
	if remoteErr == nil {
		if headErr != nil {
			if _, err := r.git("update-ref", "HEAD", strings.TrimSpace(remoteHead)); err != nil {
				return "", err
			}
			if _, err := r.apply(); err != nil {
				return "", err
			}
		} else if _, err := r.git("rebase", remoteRef); err != nil {
			conflicts, _ := r.git("diff", "--name-only", "--diff-filter=U")
			for _, directory := range []string{"rebase-merge", "rebase-apply"} {
				if _, statErr := os.Stat(filepath.Join(r.dir, directory)); statErr == nil {
					if _, abortErr := r.git("rebase", "--abort"); abortErr != nil {
						return "", fmt.Errorf("sincronização interrompida; não consegui desfazer a integração: %w", abortErr)
					}
					break
				}
			}
			return "", fmt.Errorf("não consegui combinar as versões. Suas alterações locais foram preservadas.\nArquivos: %s\n%w", strings.TrimSpace(conflicts), err)
		}
	} else if headErr != nil {
		return "Nenhum arquivo para sincronizar. Use Adicionar (F2 ou --add) primeiro.", nil
	}
	if _, err := r.git("push", "-u", "origin", "HEAD"); err != nil {
		return "", fmt.Errorf("suas alterações foram salvas nesta máquina, mas não consegui enviá-las. Tente sincronizar novamente.\n%w", err)
	}
	return "Tudo sincronizado! Suas configurações estão salvas e atualizadas.", nil
}
