# MyDotty

**Suas configurações, salvas e sincronizadas entre computadores.**

O MyDotty cuida dos seus *dotfiles*: arquivos de configuração como `.zshrc`, `.gitconfig` e os que ficam em `.config/`. Você escolhe o que acompanhar, e o programa salva as alterações e sincroniza com um repositório seu. Não é necessário aprender comandos Git para o uso diário.

Os arquivos continuam onde seus aplicativos esperam encontrá-los. O histórico fica em `~/.dotfiles`.

![Tela principal do MyDotty com arquivos novos, alterados e salvos, além dos atalhos destacados no rodapé](docs/images/visao-geral.png)

*Capturas geradas a partir da interface do MyDotty, com caminhos e repositório de exemplo.*

[Instalar](#instalação) · [Primeiro uso com imagens](#primeiro-uso-passo-a-passo) · [Comandos](#comandos-e-atalhos) · [Outro computador](#usar-em-outro-computador) · [Atualizar](#atualizar-o-mydotty) · [Configurações](#configurações) · [Ajuda](#quando-algo-não-funcionar)

## Instalação

O instalador foi testado em **Linux**. Para instalar a partir deste repositório, tenha **Git** e **Go 1.24 ou superior** instalados.

```sh
git clone https://github.com/alepspizzetti/MyDotty.git
cd MyDotty
./install.sh
```

O script compila e instala `mydotty` em `~/.local/bin`, configura o PATH de Bash, Zsh ou Fish e tenta ativar a atualização automática diária. Na primeira instalação, basta executar `./install.sh` em cada PC e abrir um novo terminal para chamar o comando de qualquer pasta:

```sh
mydotty
```

O Go é necessário para compilar e para usar a atualização automática; para sincronizar dotfiles, o programa instalado precisa apenas do Git. Sem Go, o instalador só funciona se já houver um executável `mydotty` compatível ao lado do script. O clone deste repositório não inclui esse executável.

<details>
<summary>Escolher outro diretório de instalação</summary>

A instalação por usuário é o padrão. Para instalar globalmente, por exemplo:

```sh
PREFIX=/usr/local ./install.sh
```

O instalador pode pedir sudo para copiar o executável nesse destino. Se a pasta não for gravável pelo usuário, a atualização automática ficará pendente. O instalador informa a causa sem desfazer a instalação.

</details>

## Primeiro uso: passo a passo

Você precisa de **dois repositórios diferentes**: o do MyDotty, que contém o programa, e um repositório seu, que receberá suas configurações. Nos exemplos abaixo, o seu se chama `dotfiles`.

### 1. Conecte seu repositório — F4

Crie um repositório no GitHub, GitLab ou outro serviço e copie a URL de acesso. Ele pode começar vazio. Para configurações pessoais, prefira um repositório privado.

Abra `mydotty`, pressione **F4**, cole a URL do **seu repositório de configurações** e confirme com **Enter**.

![Campo Conectar aberto com F4 e preenchido com a URL de exemplo git@github.com:voce/dotfiles.git](docs/images/conectar.png)

O MyDotty cria e configura o `~/.dotfiles` local e conecta o destino. Se o remoto já contiver configurações, tenta instalá-las nos caminhos originais.

O Git precisa ter acesso ao destino: use sua chave SSH ou o gerenciador de credenciais do serviço. O MyDotty aproveita esse acesso; ele não solicita senhas dentro da interface.

### 2. Escolha o que acompanhar — F2

Pressione **F2** e informe um arquivo ou uma pasta. Exemplos:

```text
.zshrc
.gitconfig
.config/kitty
.config/nvim
```

Use **Tab** para completar o caminho. Na captura abaixo, digitar `.config/ki` e pressionar Tab completou `.config/kitty/`. Confirme com **Enter** para adicionar, salvar e enviar a pasta ao seu repositório. Não é necessário usar F3 em seguida.

![Campo Adicionar com o caminho .config/kitty/ preenchido por autocomplete e as teclas Tab, Enter e Esc visíveis](docs/images/adicionar.png)

Os caminhos relativos começam na **sua home**, mesmo que você abra o MyDotty em outra pasta. Caminhos absolutos e `~/` também funcionam.

| No campo Adicionar | O que acontece |
| --- | --- |
| **Tab** | Completa o trecho comum ou a única opção encontrada |
| **Tab** novamente | Percorre as opções disponíveis |
| **Shift+Tab** | Percorre as opções anteriores |
| **Enter** | Adiciona, salva e sincroniza o caminho preenchido |
| **Esc** | Cancela a entrada |

Espaços, acentos e arquivos ocultos são aceitos. **Dentro da interface, digite caminhos com espaços sem aspas.** O autocomplete é do MyDotty e funciona independentemente do shell; ele não carrega plugins ou sugestões personalizados do Bash, Zsh ou Fish.

Se o envio falhar por falta de conexão, a adição fica salva em um commit local; use F3 ou `--att` quando a conexão voltar. Sem um remoto configurado, o MyDotty salva localmente e orienta conectar o repositório com F4 ou `--init`.

Adicionar uma pasta inclui seus arquivos atuais. Para incluir arquivos novos criados nela depois, adicione a pasta novamente. O MyDotty não acompanha automaticamente todos os arquivos da sua home.

### 3. Confira o que mudou — Enter

Depois de editar uma configuração, use **↑/↓** na lista para selecionar um arquivo e **Enter** para abrir suas diferenças. Linhas vermelhas mostram o que foi removido; linhas verdes mostram o que foi adicionado. Pressione **Esc** para voltar.

![Tela de diferenças mostrando a troca de font_size 12 por font_size 14, com remoção em vermelho e adição em verde](docs/images/diferencas.png)

A lista principal usa nomes simples para mostrar o estado dos arquivos:

| Situação | Significado |
| --- | --- |
| **novo** | Foi adicionado ao acompanhamento e aguarda o próximo salvamento |
| **alterado** | Tem alterações para salvar |
| **removido** | Sua exclusão aguarda salvamento |
| **salvo** | A versão local corresponde ao histórico salvo |

“Salvo” descreve o histórico local. A mensagem após sincronizar informa se o envio ao destino foi concluído.

### 4. Salve e sincronize — F3

Pressione **F3**. Em uma ação, o MyDotty:

1. Salva as alterações dos arquivos que você já adicionou, com uma descrição automática.
2. Baixa as novidades do repositório.
3. Combina as alterações e envia a versão atualizada.

Ao terminar, confira a mensagem **“Tudo sincronizado!”**.

![Lista de configurações com todos os arquivos salvos e a mensagem Tudo sincronizado após usar F3](docs/images/sincronizar.png)

No uso diário, basta editar suas configurações normalmente e pressionar **F3** quando quiser salvar e sincronizar. Use **F2** quando quiser acompanhar outros arquivos.

## Comandos e atalhos

Prefere ficar no shell? As mesmas ações estão disponíveis sem abrir a interface:

| Terminal | Na interface | Ação |
| --- | --- | --- |
| `mydotty` | — | Abrir a interface |
| `mydotty --init URL` | **F4** | Conectar seu repositório e preparar o `.dotfiles` |
| `mydotty --add CAMINHO` | **F2** | Adicionar, salvar e sincronizar um arquivo ou pasta |
| `mydotty --att` ou `mydotty --sync` | **F3** | Salvar, baixar e enviar alterações |
| `mydotty --status` | **F5** | Consultar os arquivos; na interface, atualizar a lista |
| `mydotty --diff CAMINHO` | **Enter** | Ver alterações do arquivo selecionado |
| `mydotty --remove CAMINHO` | **Del** | Parar de acompanhar, mantendo os arquivos locais |
| `mydotty --apply` | **F4**, se a instalação estiver pendente | Repetir a instalação após mover caminhos existentes |
| `mydotty --help` | **F1** | Consultar comandos e instruções |
| `mydotty --settings` | **F6** ou **Tab** | Abrir a aba Configurações |
| `mydotty --set login=true` | **↑/↓** e **Espaço/Enter** na aba | Ativar uma opção; use `false` para desativar |
| `mydotty --auto-sync` | Opção Atualização de pacotes | Ativar sincronização após atualizações do pacman |
| `mydotty --no-auto-sync` | Opção Atualização de pacotes | Desativar somente a sincronização após pacman |
| `mydotty --update` | — | Atualizar o programa manualmente |

**Esc** cancela entradas e fecha diferenças; na tela principal, sai do programa. **Ctrl+C** também encerra. Na interface, **Del** abre uma confirmação antes de parar de acompanhar o arquivo. Depois, sincronize para registrar essa retirada no repositório.

Exemplo de primeiro uso pelo terminal:

```sh
# Substitua pela URL do SEU repositório de configurações.
mydotty --init git@github.com:voce/dotfiles.git
mydotty --add .config/kitty
mydotty --add .zshrc
```

No shell, use aspas para caminhos com espaços:

```sh
mydotty --add "$HOME/.config/meu app"
```

A expansão de `~`, variáveis e o autocomplete nessa linha de comando continuam sendo os do seu shell. Os caminhos relativos informados ao MyDotty continuam partindo da home.

## Usar em outro computador

1. Instale o MyDotty e prepare o acesso Git ao seu serviço.
2. Abra `mydotty` e pressione **F4**, ou execute `mydotty --init URL`.
3. Informe a **mesma URL do seu repositório de configurações**.
4. O MyDotty baixa e instala os arquivos nos caminhos originais.
5. Use **F3** para manter os computadores sincronizados.

Se um caminho já existir, o MyDotty interrompe a instalação sem sobrescrevê-lo. Mova esse caminho para um backup e pressione **F4**, ou execute `mydotty --apply`, para tentar novamente. Pais simbólicos que redirecionariam a instalação também são rejeitados.

O MyDotty sincroniza os arquivos de configuração. A instalação dos aplicativos é feita separadamente.

## Atualizar o MyDotty

Na pasta onde você clonou **o projeto MyDotty**, execute:

```sh
git pull
./install.sh
```

Isso atualiza o código e substitui o executável em `~/.local/bin`. Se você escolheu outro `PREFIX`, repita esse destino ao reinstalar, por exemplo `PREFIX=/usr/local ./install.sh`.

Na primeira instalação, o instalador tenta ativar **Auto update** automaticamente. A reinstalação preserva a preferência de quem desativou essa opção. Você também pode alterá-la na aba Configurações. A atualização do programa preserva seu `.dotfiles`. **`mydotty --att` sincroniza suas configurações; não atualiza o programa.**

## Configurações

Abra **F6**, **Tab** na tela principal, ou `mydotty --settings`. Use **↑/↓** para selecionar e **Espaço** ou **Enter** para ativar/desativar. Cada alteração é aplicada e salva imediatamente. **Tab** ou **Esc** volta à lista de arquivos.

```text
AUTO-SYNC
[x] Atualização de pacotes
[x] Início de sessão
[x] Encerramento de sessão

AUTO UPDATE · MyDotty
[x] Ativo
```

As três opções de auto-sync são independentes: você pode deixar todas ativas. Auto update é uma quarta opção, separada da sincronização dos dotfiles. Cada linha mostra uma explicação ao ser selecionada. As opções ficam em `~/.config/mydotty/config.json` (ou no diretório definido por `XDG_CONFIG_HOME`) e são específicas de cada máquina.

Pelo terminal:

```sh
mydotty --set packages=true
mydotty --set login=true
mydotty --set logout=true
mydotty --set update=true
# Para desativar uma opção, use false no lugar de true.
```

### Sincronizar ao atualizar o sistema

Em **Arch Linux e CachyOS**, a opção **Atualização de pacotes** instala um hook do pacman. Após uma atualização concluída por `pacman -Syu` ou Cachy-Update, o hook agenda o `--att` para dali a **15 segundos**. Outra transação nesse intervalo reinicia a contagem. A atualização dos pacotes não espera a sincronização terminar.

A ativação dessa opção pode solicitar `sudo` para instalar o hook. Execute o MyDotty com seu usuário normal. Os comandos `mydotty --auto-sync` e `mydotty --no-auto-sync` continuam disponíveis e alteram somente essa opção. `./install.sh --auto-sync` instala e ativa essa integração.

### Sincronizar no início e no encerramento da sessão

Essas opções usam serviços do **systemd do usuário**, sem depender do pacman:

- **Início de sessão:** executa `--att` quando a sessão gráfica ou o gerenciador do usuário inicia. A ativação vale para a próxima sessão e não dispara uma sincronização imediatamente.
- **Encerramento de sessão:** prepara um serviço que executa a sincronização quando a sessão gráfica termina ou antes de o gerenciador do usuário encerrar, normalmente após sair da última sessão. Desativar a opção não dispara a sincronização.

A integração com a sessão gráfica usa `graphical-session.target`, gerenciado por desktops como GNOME e KDE. Com **linger** habilitado, o gerenciador do usuário inicia no boot e permanece ativo após o logout: o ciclo da sessão gráfica continua disparando as ações, mas logins somente em TTY/SSH acompanham o início e a parada desse gerenciador. Abrir ou fechar um terminal dentro do desktop não dispara uma sincronização de sessão. No primeiro login, o início do gerenciador e da sessão gráfica podem gerar duas sincronizações; elas são executadas em sequência. Consulte a [documentação de loginctl](https://www.freedesktop.org/software/systemd/man/latest/loginctl.html) para o ciclo de vida do gerenciador.

As automações usam o repositório padrão `~/.dotfiles`. Precisam de rede e de acesso ao Git sem pedir senha. Ao ativar, o MyDotty importa o agente SSH disponível para o gerenciador do usuário. No logout, o envio depende de a rede e o agente ainda estarem disponíveis; se falhar, as alterações ficam salvas localmente para a próxima sincronização. Os disparos simultâneos são executados em sequência para não disputar o mesmo repositório. Arquivos novos continuam exigindo uma adição explícita.

### Atualizar o MyDotty automaticamente

**Auto update → Ativo** cria um timer diário, com até cinco minutos de variação. Se o horário for perdido com a máquina desligada, o systemd agenda a verificação quando o timer voltar a ficar ativo. A atualização baixa o código do repositório oficial, compila e verifica o novo executável antes de substituí-lo. Uma falha de download, compilação ou verificação mantém a versão instalada. A nova versão é usada na próxima execução do comando.

É necessário ter **Go e Git instalados** e permissão de escrita na pasta do executável. Para uma instalação por usuário:

```sh
./install.sh
```

O instalador configura o PATH para priorizar essa instalação sobre uma versão global; abra um novo terminal após instalar. Se Go ou o systemd do usuário estiverem indisponíveis, a instalação termina e informa que a configuração ficou pendente. Execute o instalador novamente quando os requisitos estiverem disponíveis. Para uma instalação global sem permissão de escrita, continue usando `git pull` e `PREFIX=/usr/local ./install.sh` para atualizar manualmente. A atualização automática não pede senha em segundo plano.

### Consultar os resultados

```sh
journalctl --user -u mydotty-sync.service -n 30 --no-pager
journalctl --user -u mydotty-login.service -u mydotty-logout.service -n 30 --no-pager
journalctl --user -u mydotty-update.service -n 30 --no-pager
systemctl --user list-timers 'mydotty-*'
```

As opções criam seus próprios arquivos em `~/.config/systemd/user` (ou `XDG_CONFIG_HOME/systemd/user`):

| Opção | Arquivos |
| --- | --- |
| Pacotes | `mydotty-sync.service`, `mydotty-sync.timer` e `/etc/pacman.d/hooks/zz-mydotty-UID.hook` |
| Início de sessão | `mydotty-login.service` |
| Encerramento de sessão | `mydotty-logout.service` |
| Auto update | `mydotty-update.service`, `mydotty-update.timer` |

Desativar uma opção remove apenas a integração correspondente. Uma sincronização ou atualização já em andamento pode terminar normalmente. No outro computador, configure as opções desejadas novamente; não é necessário copiar hooks ou serviços manualmente.

Referências técnicas: [hooks do pacman](https://man.archlinux.org/man/alpm-hooks.5), [serviços do systemd](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html) e [timers](https://www.freedesktop.org/software/systemd/man/latest/systemd.timer.html).

## Quando algo não funcionar

| O que aconteceu | O que fazer |
| --- | --- |
| O instalador pede Go | Instale Go 1.24 ou superior para compilar a partir deste repositório |
| `mydotty` não foi encontrado | Abra um novo terminal para carregar o PATH configurado; o padrão é `~/.local/bin` |
| Não foi possível acessar o repositório | Confira a URL e o acesso SSH ou as credenciais do Git |
| A instalação encontrou um arquivo existente | Mova o caminho indicado para um backup e tente novamente com **F4** ou `--apply` |
| Estou sem conexão | Suas alterações ficam salvas localmente; tente **F3** novamente quando a conexão voltar |
| Dois computadores alteraram as mesmas partes de um arquivo | O MyDotty desfaz a tentativa de integração e preserva a versão local salva; as versões precisam ser conciliadas antes de sincronizar novamente |
| O terminal está pequeno | Amplie para pelo menos **60 colunas × 16 linhas** |
| F2 ou F3 controla brilho ou volume no notebook | Experimente **Fn+F2** ou **Fn+F3** |

A sincronização combina alterações compatíveis por merge. Em caso de conflito, a interface lista os caminhos e pede qual versão dos arquivos inteiros manter ativa: **L** para esta máquina, **R** para o remoto, ou **Esc** para cancelar. A escolha vale para todos os arquivos em conflito; as duas versões continuam no histórico. No terminal, `mydotty --add CAMINHO` e `mydotty --att` sincronizam direto e, somente se houver conflito, mostram os caminhos e perguntam qual versão manter (**L**, **R** ou **C** para cancelar, seguido de Enter). Para escolher sem interação, use `mydotty --att --keep local` ou `mydotty --att --keep remote`. O auto-sync preserva as versões e informa o conflito para você decidir depois. Outros erros são exibidos sem pedir uma escolha de versão. O MyDotty não faz envio forçado.

Quando o remoto adiciona um arquivo que já existe localmente, mas ainda não é acompanhado nesta máquina, o `--att` compara a data de modificação local com a data do último commit remoto que alterou aquele arquivo. Se o local for mais antigo, aceita automaticamente a versão remota. Datas iguais ou locais mais recentes mantêm a proteção do Git contra sobrescrita. Se o merge falhar, os arquivos locais substituídos nessa tentativa são restaurados. O Git não armazena a data de modificação original dos arquivos; a comparação usa a data do commit.

## Detalhes e desenvolvimento

O MyDotty usa um repositório Git *bare* em `~/.dotfiles` e a home como árvore de trabalho. Também funciona com repositórios existentes nesse formato. Adicionar a home inteira ou os dados internos do Git é bloqueado.

A identidade Git existente é reutilizada. Se ela não estiver configurada, o programa usa `MyDotty <mydotty@localhost>` somente nesse repositório, sem alterar configurações globais.

Para usar caminhos personalizados:

```sh
mydotty --repo /caminho/repositorio --home /caminho/configuracoes
```

Para verificar o código:

```sh
go test -race ./...
go vet ./...
```

Os testes usam repositórios temporários, sem acessar o GitHub nem modificar seus dotfiles. Cobrem instalação, repositório vazio, sincronização entre computadores, conflitos, alterações offline, CLI, autocomplete e limites de tamanho da interface.

A interface usa Bubble Tea e Lip Gloss. Operações Git executam fora do processamento de eventos da interface, com limite de 90 segundos por comando.

## Licença

[MIT](LICENSE).
