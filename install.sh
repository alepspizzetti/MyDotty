#!/bin/sh
set -eu

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
prefix=${PREFIX:-"$HOME/.local"}
case "$prefix" in
    /*) ;;
    *) printf '%s\n' 'Erro: PREFIX precisa ser um caminho absoluto.' >&2; exit 1 ;;
esac
bin_dir=${prefix%/}/bin

auto_sync=false
if [ "$#" -gt 1 ]; then
    printf '%s\n' 'Uso: ./install.sh [--auto-sync | --no-auto-sync]' >&2
    exit 1
fi
case "${1:-}" in
    --auto-sync) auto_sync=true ;;
    --no-auto-sync) exec "$bin_dir/mydotty" --no-auto-sync ;;
    --help|-h)
        printf '%s\n' 'Uso: ./install.sh [--auto-sync | --no-auto-sync]' \
            '  --auto-sync     Instalar e ativar sincronização após atualizar pacotes.' \
            '  --no-auto-sync  Desativar a integração, mantendo o programa instalado.'
        exit 0 ;;
    '') ;;
    *) printf '%s\n' 'Opção desconhecida. Use ./install.sh --help.' >&2; exit 1 ;;
esac

if ! command -v git >/dev/null 2>&1; then
    printf '%s\n' 'Instale o Git antes de instalar o MyDotty.' >&2
    exit 1
fi

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' 0
trap 'exit 1' 1 2 15

printf '%s\n' 'MyDotty · instalação'
if command -v go >/dev/null 2>&1; then
    printf '%s\n' 'Compilando para esta máquina…'
    revision=$(git -C "$source_dir" rev-parse --verify HEAD 2>/dev/null || true)
    (cd "$source_dir" && go build -trimpath -ldflags="-s -w -X main.buildRevision=$revision" -o "$build_dir/mydotty" .)
    binary=$build_dir/mydotty
elif [ -x "$source_dir/mydotty" ]; then
    printf '%s\n' 'Usando o executável disponível nesta pasta.'
    binary=$source_dir/mydotty
else
    printf '%s\n' 'Instale Go 1.24 ou superior para compilar, ou coloque um executável mydotty compatível ao lado deste script.' >&2
    exit 1
fi

if ! "$binary" --help >/dev/null 2>&1; then
    printf '%s\n' 'O executável não funciona nesta máquina. Compile com o Go instalado neste sistema.' >&2
    exit 1
fi

if mkdir -p "$bin_dir" 2>/dev/null && [ -w "$bin_dir" ]; then
    install -m 755 "$binary" "$bin_dir/mydotty"
else
    if ! command -v sudo >/dev/null 2>&1; then
        printf '%s\n' 'Sem permissão para instalar. Execute como administrador ou use PREFIX="$HOME/.local" ./install.sh.' >&2
        exit 1
    fi
    printf 'Permissão de administrador necessária para instalar em %s.\n' "$bin_dir"
    sudo install -d "$bin_dir"
    sudo install -m 755 "$binary" "$bin_dir/mydotty"
fi

if ! "$bin_dir/mydotty" --install-setup; then
    printf '%s\n' 'O programa foi instalado, mas a configuração automática ficou pendente. Consulte a mensagem acima.' >&2
fi

if [ "$auto_sync" = true ]; then
    "$bin_dir/mydotty" --auto-sync
fi

printf '\nInstalado em %s/mydotty\n' "$bin_dir"
case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) printf 'Abra um novo terminal para carregar o PATH configurado, ou execute %s/mydotty neste terminal.\n' "$bin_dir" ;;
esac
printf '\n%s\n' 'Pronto! Comandos disponíveis:' '  mydotty' '  mydotty --init URL_DO_SEU_REPOSITORIO' '  mydotty --add .config/kitty' '  mydotty --att' '  mydotty --help'
