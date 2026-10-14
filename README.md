# MyDotty

Suas configurações salvas e sincronizadas entre computadores, pelo terminal. Você cria um repositório no GitHub, GitLab ou outro serviço; o MyDotty prepara o restante. Não é necessário conhecer comandos Git.

Os arquivos continuam nas pastas usadas pelos aplicativos. O MyDotty guarda seu histórico em `~/.dotfiles`, sem copiar suas configurações para outra pasta.

## Instalação

É necessário ter o Git instalado. Com Go 1.24 ou superior, o instalador compila para sua máquina e instala o comando globalmente:

```sh
git clone https://github.com/alepspizzetti/MyDotty.git
cd MyDotty
./install.sh
mydotty
```

O instalador usa `/usr/local/bin` e solicita `sudo` somente se precisar de permissão para instalar. Depois, `mydotty`, `mydotty --add`, `mydotty --att` e os demais comandos ficam disponíveis de qualquer pasta, em qualquer shell. Para atualizar, execute `./install.sh` novamente.

Sem Go, o instalador usa um executável `mydotty` compatível que já esteja ao lado do script. O executável instalado precisa do Git, mas não do Go.

Para instalar somente para seu usuário, sem permissão de administrador:

```sh
PREFIX="$HOME/.local" ./install.sh
```

Nesse caso, `~/.local/bin` precisa estar no seu `PATH`. O instalador avisa quando a pasta de destino não está no PATH.

## Primeiro uso

1. Crie um repositório para suas configurações no serviço de sua preferência, de preferência privado. Ele pode estar vazio.
2. Execute `mydotty`, pressione **F4** e cole a URL desse repositório.
3. Pressione **F2** para adicionar um arquivo ou pasta, como `.zshrc` ou `.config/kitty`. Use **Tab** para completar o caminho.
4. Pressione **F3** para salvar e sincronizar.

O MyDotty cria e configura o repositório local `.dotfiles`, conecta o destino e usa a identidade Git já configurada. Se não existir uma identidade, usa `MyDotty <mydotty@localhost>` somente nesse repositório; nenhuma configuração global é alterada.

É necessário que o Git tenha acesso ao destino. Para repositórios privados, configure o acesso SSH ou o gerenciador de credenciais do seu serviço. O MyDotty usa esse acesso e não armazena senhas nem pede senhas dentro da interface.

## Atalhos no terminal e na interface

Sem argumentos, `mydotty` abre a interface. Os comandos abaixo funcionam sem abrir a TUI:

| Terminal | Interface | Ação |
| --- | --- | --- |
| `mydotty --init URL` | **F4** | Conectar o repositório e preparar o `.dotfiles` |
| `mydotty --add CAMINHO` | **F2** | Adicionar um arquivo ou pasta |
| `mydotty --att` ou `mydotty --sync` | **F3** | Salvar alterações, baixar novidades e enviar ao destino |
| `mydotty --status` | **F5** | Listar os arquivos e seu estado; na TUI, atualizar a lista |
| `mydotty --diff CAMINHO` | **Enter** | Ver o que mudou no arquivo selecionado |
| `mydotty --remove CAMINHO` | **Del** | Parar de acompanhar, mantendo os arquivos locais |
| `mydotty --apply` | **F4**, durante instalação pendente | Tentar instalar novamente após resolver caminhos existentes |
| `mydotty --help` | **F1** | Consultar comandos e instruções |

Use **↑/↓** para selecionar arquivos, **Esc** para voltar ou cancelar uma entrada e **Esc** na tela principal para sair. **Ctrl+C** encerra o programa. A retirada de um arquivo na TUI pede confirmação com Enter.

Exemplo completo:

```sh
mydotty --init git@github.com:voce/dotfiles.git
mydotty --add .config/kitty
mydotty --add .zshrc
mydotty --att
```

Os caminhos relativos partem da sua home, independentemente da pasta em que você abriu o programa. Caminhos absolutos e `~/` também funcionam. No shell, use aspas para caminhos com espaços:

```sh
mydotty --add "$HOME/.config/meu app"
```

Adicionar uma pasta inclui seus arquivos atuais. Para acompanhar novos arquivos criados nessa pasta, adicione-a novamente. Sincronizar atualiza somente os arquivos já adicionados, incluindo suas exclusões; outros arquivos da home ficam de fora. Adicionar a home inteira e os dados internos do Git é bloqueado.

## Autocomplete de caminhos

No campo **Adicionar** da TUI:

- **Tab** completa a parte comum do caminho; se existir uma única opção, preenche o nome.
- **Tab** novamente percorre as opções; **Shift+Tab** percorre no sentido contrário.
- Pastas terminam com `/`. Continue digitando ou completando para entrar nelas.
- Arquivos ocultos, espaços e acentos são aceitos. Dentro da TUI, não coloque aspas ao redor do caminho.

O autocomplete da TUI consulta os arquivos locais e funciona em Bash, Zsh, Fish ou outros shells. Ele é implementado pelo MyDotty, pois as funções de autocomplete do shell não recebem as teclas enquanto a interface está aberta. Plugins e sugestões personalizados do shell não são importados para a TUI. No comando `mydotty --add`, a expansão de `~`, variáveis e o autocomplete continuam sendo os do seu shell.

## Outro computador

Instale o MyDotty e o Git, configure o acesso ao serviço e conecte a mesma URL com **F4** ou `--init`. O MyDotty baixa e instala as configurações nos caminhos originais. A instalação dos aplicativos é feita separadamente.

Se um caminho já existir, o MyDotty não o sobrescreve: mova esse caminho para um backup e pressione **F4**, ou execute `mydotty --apply`, para tentar novamente. Pastas simbólicas que redirecionariam a instalação também são rejeitadas.

No uso diário, edite suas configurações normalmente e use **F3** ou `--att`. O MyDotty salva suas alterações antes de baixar novidades e combina alterações de computadores diferentes. Se houver conflito nas mesmas partes do arquivo, desfaz a tentativa de integração e preserva a versão local já salva; a sincronização fica pendente até as versões serem conciliadas. Não há envio forçado.

Sem conexão, as alterações ficam salvas localmente. Tente sincronizar novamente quando a conexão voltar. A interface diferencia arquivos **novos**, **alterados**, **removidos** e **salvos**; “salvo” descreve o histórico local, e a mensagem da sincronização informa se o envio terminou.

Para usar pastas personalizadas:

```sh
mydotty --repo /caminho/repositorio --home /caminho/configuracoes
```

## Desenvolvimento

```sh
go test -race ./...
go vet ./...
```

Os testes usam repositórios temporários, sem acessar GitHub nem modificar seus dotfiles. Cobrem conexão a repositório vazio, instalação, sincronização entre computadores, conflitos, alterações offline, comandos CLI, autocomplete e limites de tamanho da interface.

A interface usa Bubble Tea e Lip Gloss. Operações Git são executadas fora do processamento de eventos da TUI, com limite de 90 segundos por comando.

## Licença

[MIT](LICENSE).
