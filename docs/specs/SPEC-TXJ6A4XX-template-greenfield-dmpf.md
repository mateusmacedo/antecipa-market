---
id: SPEC-TXJ6A4XX
slug: template-greenfield-dmpf
title: Template greenfield DMPF — workspace pronto para consumir o DMPF por versão
stage: done
priority: P1
depends_on: [SPEC-317RXZ73]
ticket_url: https://linear.app/mmda/issue/DEVS-71/spec-txj6a4xx-template-greenfield-dmpf-workspace-pronto-para-consumir
subtask_urls: []
created: 2026-10-06
---

# SPEC-TXJ6A4XX: Template greenfield DMPF — workspace pronto para consumir o DMPF por versão

## Resumo

Transformar o `nx-base-template` na base de projetos greenfield que usam o
DMPF: kernel Go, `dmpf-conformance`, generator `bounded-context`, gates,
infra, CI e ativos de IA, todos recebidos **por versão** do plugin
`@mateusmacedo/dmpf-plugin`. Como mantenedor, quero criar um repositório a
partir do template, configurar o module path num comando e gerar o primeiro
bounded context, deixando-o verde nos gates com os passos documentados, sem
copiar nada do platform.

**Dependência externa resolvida**: a SPEC-P846PH1P do repositório `dmpf`
(`github.com/mateusmacedo/dmpf`, DEVS-70 concluída) publicou o plugin no
GitHub Packages. Ainda não há versão estável: o template fixa a `1.0.0-rc.3`,
publicada em 2026-10-09 com o dist-tag `next` (o `latest` aponta para a
`1.0.0-rc.2`). O campo `depends_on` registra só a dependência local
(SPEC-317RXZ73), porque o catálogo só resolve IDs locais.

## Contexto

- **Problema**:
  - O template não tem nada de DMPF: nenhuma app ou lib, `go.work` com
    `go 1.25` (`go.work:1`) contra a diretiva `1.26.9` do `versions.json` da
    rc.3, sem namedInput `go` no `nx.json`, sem `.golangci.yml`, sem gates,
    infra ou ativos de IA de bounded context.
  - A migração da plataforma (GitHub, GitHub Packages, Linear, repositório
    template) foi feita pela SPEC-317RXZ73, pré-requisito local desta spec.
- **Impacto**: um projeto novo nasce de `gh repo create --template` e chega
  ao primeiro contexto verde em poucos comandos, sempre com as versões que o
  plugin declara compatíveis.
- **Inspiração**: repositórios template do GitHub; workspaces Nx que se
  atualizam com `nx migrate`; o `tools/external-consumer.sh` do platform, que
  prova o consumo externo do plugin.
- **Revalidação contra a `1.0.0-rc.3`** (2026-10-09, análise estática do
  plugin na tag `@mateusmacedo/dmpf-plugin@1.0.0-rc.3`, commit `af98888`). Os
  achados que mudaram esta spec:
  - O `.env.example` da raiz é arquivo gerenciado pelo `init`, e o
    `infra/local/docker-compose.yml` é semente; os dois já existem no
    template com outro conteúdo (A1).
  - O pnpm 11.5.3 ou superior ignora `${...}` em credencial do `.npmrc` do
    projeto (A2).
  - O `bounded-context` grava `release.groups` no `nx.json`, que o Nx 23.1.0
    recusa junto de `release.projects`, e o contrato Go gerado tem a tag
    `type:lib` (A5).
  - O workflow reutilizável roda o gate de conformance sem condição, e o
    `conformance` reprova workspace sem módulo Go (B1).
  - O contexto gerado não nasce verde: precisa de `tidy`, de baseline e de um
    `.proto` (B2, C1).
- **Links relevantes**:
  - SPEC-P846PH1P (repo `dmpf`) — torna o DMPF consumível por versão.
  - Guia de consumo do platform: `docs/guides/dmpf-consumo-externo.md` no
    commit `5def4d2d749bf8d5b2ef683f3cdc97eadf354c26`, o `workflowRef` da rc.3.
  - SPEC-317RXZ73 — migração para GitHub, GitHub Packages e Linear (ADR-010).
  - SPEC-7VQFX9RP — remoção das libs de exemplo (ADR-009).
  - ADR-002 (tasks e cache), ADR-007 (template autônomo), ADR-009 (sem libs
    de exemplo), ADR-010 (GitHub e Linear).

<constraints>
- [P0] Todo arquivo DMPF do template é saída do plugin (`init`) na versão
  fixada no `package.json`; nenhum arquivo DMPF é escrito ou editado à mão.
- [P0] Nenhuma app ou lib de exemplo no template (ADR-009); a prova de que o
  template gera projeto funcional é um job de CI que descarta o que gera.
- [P0] Token do GitHub Packages só por variável de ambiente ou secret; o
  `.npmrc` versionado não contém credencial.
- [P0] Kernel e plugin só se atualizam juntos, por `nx migrate`; nenhuma
  automação sobe um sem o outro.
- [P1] As stacks JS/TS atuais (Express, Nest, Next, Angular) e os agentes e
  rules genéricos de `.claude/` permanecem como estão.
</constraints>

## Requisitos

### Funcionais

**A. Bootstrap DMPF**

- [x] **[P0] A1 — Workspace inicializado pelo plugin**: o template contém a
  saída de
  `pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=example.com/change-me --npmScope=@mateusmacedo --composeProject=nx-base-template`,
  com o plugin na versão `1.0.0-rc.3`: `dmpf.json`, `dmpf.rendered.json`,
  `go.work`, configuração Go no `nx.json`, `.golangci.yml`, `infra/`,
  `.env.example`, `tools/dmpf-baseline/units-baseline.json`,
  `.github/workflows/dmpf-ci.yml` e os ativos de IA
  (`.claude/{agents,commands,rules}/dmpf-*` e
  `.agents/skills/dmpf-{bounded-context,testkit}/`).
  - `git diff` depois de rodar o mesmo `init`, com as mesmas flags, sobre o
    template é vazio.
  - `--npmScope` e `--composeProject` são explícitos porque os defaults
    derivam do `name` da raiz (`@nx-base-template/source`) e dariam
    `@nx-base-template` e `source`.
  - Instalação: o plugin entra pelo `catalog:` do `pnpm-workspace.yaml`, como
    as demais dependências, e o `init` roda em seguida. O `nx add` não serve
    ao template: ele roda o `init` sem opções, com os defaults acima, e o
    `init` seguinte não reescreve a semente do compose.
  - Colisão: o `.env.example` da raiz e o `infra/local/docker-compose.yml`
    atuais vão para a lixeira antes do primeiro `init`. O `.env.example`
    gerenciado preserva o cabeçalho atual, e a semente do compose inclui
    `compose/postgres.yml` e `compose/redis.yml`, os serviços que o compose
    atual oferece.
  - Pré-requisitos do `init`: Go 1.26.9 e acesso ao proxy de módulos (o
    callback roda `infrasync --write` via `go run`), além de `bash`, `jq` e
    `git`.
  - O `biome.json` exclui `dmpf.json`, `dmpf.rendered.json`, `infra` e
    `tools/dmpf-baseline`: o pre-commit roda `biome check --write` e, sem a
    exclusão, poderia reescrever arquivo gerado, quebrando o registro de
    hashes do `init` ou o `infrasync --check`.
- [x] **[P0] A2 — Registry**: `.npmrc` versionado só com
  `@mateusmacedo:registry=https://npm.pkg.github.com`. A credencial
  (`//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}`) vai no `~/.npmrc`
  do usuário: desde a 11.5.3, o pnpm ignora `${...}` em credencial do
  `.npmrc` do projeto (GHSA-3qhv-2rgh-x77r).
  - No CI, a composite `setup-node-pnpm` e o `publish-libs.yaml` gravam um
    npmrc de usuário com o placeholder em `$RUNNER_TEMP` e exportam
    `NPM_CONFIG_USERCONFIG` antes do `pnpm install`; os workflows definem
    `NODE_AUTH_TOKEN: ${{ github.token }}` e têm `packages: read` (ou
    `write`).
  - O `infra/docker/Dockerfile.node.example` instala as dependências com dois
    secrets do BuildKit: o npmrc de usuário (`id=npmrc`) e o
    `NODE_AUTH_TOKEN` (`env=`), sem gravar credencial em camada da imagem.
  - Edge: `pnpm install` sem a credencial falha com erro de autenticação no
    `npm.pkg.github.com`, mesmo com o pacote público; o README explica como
    criar o token (`read:packages`) e onde gravá-lo, logo na seção de
    pré-requisitos.
- [x] **[P0] A3 — Configuração do projeto novo**: o primeiro passo depois de
  criar o repositório é
  `pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=<module path do projeto>`,
  que reescreve sem `--force` os arquivos gerenciados não editados. Enquanto
  o prefixo for `example.com/change-me`, o `bounded-context` recusa rodar
  antes de escrever qualquer arquivo, com a mensagem
  `dmpf.json still has the reserved modulePrefix example.com/change-me; set the real one first: … --modulePrefix=<path>`
  (comportamento do plugin).
  - Edge: o `init` não reescreve a semente `infra/local/docker-compose.yml`;
    o nome do projeto Compose (`nx-base-template-local`) é ajustado à mão no
    projeto derivado, e o README diz isso.
- [x] **[P1] A4 — Renovate em lockstep**: o `renovate.json` ganha uma
  `packageRules` com `enabled: false` para `@mateusmacedo/dmpf-plugin`,
  `@nx-go/nx-go`, os módulos `github.com/mateusmacedo/dmpf/**` e o `uses:`
  de `mateusmacedo/dmpf` nos workflows; a atualização desses itens acontece
  só por `nx migrate`. `ignoreDeps` não serve porque só casa nome exato, e o
  `helpers:pinGitHubActionDigests` atual mexeria no SHA do workflow
  reutilizável.
- [x] **[P0] A5 — Release sem grupos no template**: o `nx.json` perde
  `release.projects` e `release.docker`; o topo do `release` fica só com as
  opções globais (versionamento independente, tag `{projectName}@{version}`,
  conventional commits, changelog, git), como no `nx.json` do platform. Cada
  grupo nasce com o primeiro membro:
  - `go-contract-<nome>`: gravado pelo `bounded-context`;
  - `libs` (`["tag:type:lib", "!tag:stack:go"]`) e `apps` (com o bloco
    `docker`): passos documentados em `docs/nx-reference/tasks.md`, na
    criação da primeira lib ou app.
  - Motivo: o `bounded-context` grava `release.groups`, e o Nx 23.1.0 recusa
    `projects` e `groups` juntos (`PROJECTS_AND_GROUPS_DEFINED`); grupo sem
    projeto é erro, inclusive em `--dry-run` (`RELEASE_GROUP_MATCHES_NO_PROJECTS`);
    qualquer grupo explícito elimina o grupo implícito; e o `docker` do topo
    é herdado por todos os grupos, inclusive o contrato Go.
  - Os filtros de publish npm (`nx-release.yml`, `publish-libs.yaml`,
    `nx-publish-libs.yml`) e o release Docker do `nx-release.yml` excluem
    `tag:stack:go`; o contrato Go gerado tem as tags `type:lib` e `stack:go`,
    e as apps Go, `type:app`.
  - A chave `release` é do template; o `init` não a altera, e o critério de
    `git diff` vazio do A1 continua valendo. O trecho de release do
    `AGENTS.md` acompanha a mudança.

**B. CI/CD no GitHub**

- [x] **[P0] B1 — CI Go**: o `.github/workflows/dmpf-ci.yml`, gerado pelo
  `init`, chama
  `mateusmacedo/dmpf/.github/workflows/dmpf-go-ci.yml@5def4d2d749bf8d5b2ef683f3cdc97eadf354c26`
  (o `workflowRef` do `versions.json` da rc.3), com `permissions` de
  `actions`, `contents` e `packages` em `read` e o input `plugin-version`
  igual à versão instalada. O `ci.yml` existente mantém o job de qualidade
  JS (`biome ci`, `nx affected -t lint,typecheck,test,build`) em runner
  hospedado pelo GitHub e ganha `packages: read` e a credencial de usuário do
  A2. O `nx affected` desse job exclui `tag:stack:go`: os projetos Go ficam a
  cargo do `dmpf-ci.yml`, e o job JS não instala Go.
  - Enquanto o template não tem módulo Go, o passo "DMPF conformance gate"
    do reutilizável falha: `conformance --root .` devolve erro quando não
    encontra módulo (`cmd/conformance/main.go:149-151` na tag). O
    `dmpf-ci.yml` fica vermelho nos PRs do template e **não** entra nos
    checks obrigatórios dos rulesets de `develop` e `master`. No projeto
    derivado, ele passa a valer a partir do primeiro contexto.
- [x] **[P0] B2 — Smoke do template**: o job `template-smoke` roda em todo PR
  o script `tools/template-smoke.sh`, que copia o working tree (arquivos
  versionados e não ignorados) para um repositório novo em `mktemp`, instala
  as dependências e repete ali a sequência do `tools/external-consumer.sh` do
  platform na versão fixada. O script roda igual localmente e no CI e não
  altera o working tree de origem:
  0. `bounded-context` com o prefixo reservado, que precisa falhar sem
     escrever arquivos (A3);
  1. `init --modulePrefix=<prefixo de teste>`;
  2. `bounded-context <nome> --boundedContext=<nome>`;
  3. `pnpm nx run-many -t tidy`;
  4. `conformance --root . --write-baseline`;
  5. um `.proto` mínimo, porque contrato vazio reprova no buf;
  6. `go build`, `go vet` e `go test` dos pacotes gerados;
  7. `conformance --root .`, `modsync --root . --check` e
     `infrasync --root . --check`, via
     `go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/<ferramenta>@v1.0.0-rc.3`;
  8. context-check com
     `node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh --phase structural`
     (com `DMPF_APPS_DIR` e `DMPF_KERNEL_DDL`);
  9. `pnpm nx run-many -t buf-lint`;
  10. `pnpm nx release --dry-run`, que prova o A5 com o grupo Go gerado.
  - O script roda com `CI` desligado (`unset CI`), porque com `CI=true` os
    testes de infra reprovam quando a infra falta. Como o repositório de
    trabalho é novo e completo, o `modsync` (que recusa clone raso) não
    depende do `fetch-depth` do checkout.
- [x] **[P0] B3 — Sem Gitea e Verdaccio** (movido para a SPEC-317RXZ73): `nx-release.yml`,
  `nx-publish-libs.yml`, `create-release.yml` e `cd-dev-hmg.yml` rodam em
  runner hospedado pelo GitHub, usam `gh` e publicam libs TS no GitHub
  Packages; `rg -il 'gitea|verdaccio'` fora de `docs/adr/` e `docs/specs/`
  retorna 0 arquivos.
- [x] **[P1] B4 — Repositório template** (movido para a SPEC-317RXZ73): `master` e `develop` publicadas em
  `github.com/mateusmacedo/dmpf-nx-template`, repositório marcado como
  template (`isTemplate: true`) e com proteção de `master` e `develop`
  conforme o fluxo git-flow do `CONTRIBUTING.md`.

**C. Documentação**

- [x] **[P0] C1 — README**: quickstart greenfield com:
  - criação via template e token no `~/.npmrc`;
  - `pnpm install` e `init --modulePrefix` (com `--composeProject`);
  - primeiro `bounded-context`, `tidy`, `conformance --write-baseline` e o
    primeiro `.proto`;
  - gates e `nx migrate`.

  A "Stack suportada" inclui Go/DMPF, e a "Convenção de tags" inclui
  `stack:go` e `layer:*`.
- [x] **[P0] C2 — ADR-011 "O template consome o DMPF por versão"**: plugin como
  âncora única, GitHub Packages, ativos de IA pelo plugin, smoke em vez de app
  de exemplo, release sem grupos no template e `dmpf-ci.yml` fora dos checks
  obrigatórios do template.
- [x] **[P0] C3 — ADR de plataforma GitHub** (movido para a SPEC-317RXZ73, que a
  publica como ADR-010): supersede a
  ADR-005 e as partes de Gitea e Verdaccio das ADRs 004 e 008; ADRs antigas
  recebem o status de superseded, sem reescrever o histórico.
- [x] **[P1] C4 — Docs de processo sem Gitea** (movido para a SPEC-317RXZ73): `AGENTS.md`, `CONTRIBUTING.md`,
  `docs/onboarding.md`, `docs/ci-cd/*` e `docs/guides/development-workflow.md`
  descrevem GitHub e GitHub Packages.
- [x] **[P1] C5 — Guia do primeiro contexto**:
  `docs/guides/primeiro-bounded-context.md`, curto. Ele remete ao guia de
  consumo do platform (`docs/guides/dmpf-consumo-externo.md` no commit
  `5def4d2d749bf8d5b2ef683f3cdc97eadf354c26`) e cobre só o que é próprio do
  template.

### Não-funcionais

- [x] Compatibilidade: Nx 23.x (peer `@nx/devkit ^23.0.0` do plugin), Node 24,
  pnpm 11.14.0 e a diretiva Go do `versions.json` do plugin (1.26.9 na rc.3).
- [ ] Desempenho: `template-smoke` termina em até 25 min no runner hospedado
  pelo GitHub, com cache de Go e pnpm (o mesmo teto do job
  `external-consumer` do platform).
- [x] Segurança: workflows com `permissions` mínimas (`contents: read`,
  `packages: read` e, no `dmpf-ci.yml`, `actions: read`); nenhum token em
  log ou arquivo versionado.

### Pontos a verificar na implementação

- O `pnpm install` depois do `nx migrate` em CI falha por `frozen-lockfile`,
  pendência registrada pelo platform para a rc.3. O README documenta o
  `pnpm install` sem a flag no fluxo local.
- O Renovate pode precisar de `hostRules` para `npm.pkg.github.com` a fim de
  manter o lockfile, porque o registry exige autenticação mesmo para pacote
  público; observar o primeiro run depois do merge.

Resolvidos no planejamento: o `nx migrate` atualiza versões declaradas pelo
`catalog:` no `pnpm-workspace.yaml` (Nx 23.1.0); os alvos Go saem do
`nx affected` do `ci.yml` (B1); as apps Go saem do release Docker do
`nx-release.yml` (A5); o `versions.json` expõe a versão das ferramentas na
chave `conformance`.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Configuração do workspace (`nx.json`, `go.work`, `package.json`, `pnpm-workspace.yaml`, `.npmrc`, `renovate.json`, `biome.json`) | [x] | Saída do `init`, catálogo, registry, lockstep, release sem grupos e exclusões do Biome |
| Ferramentas (`.golangci.yml`, `tools/dmpf-baseline/`, `tools/template-smoke.sh`) | [x] | Saída do `init` e smoke |
| Infra (`infra/`, `.env.example`, `infra/docker/Dockerfile.node.example`) | [x] | Saída do `init`, substituindo o compose e o `.env.example` atuais; secrets do registry no Dockerfile de exemplo |
| CI/CD (`.github/workflows/`, `.github/actions/setup-node-pnpm/`) | [x] | Chamador DMPF, credencial do registry, smoke e filtros de release |
| Ativos de IA (`.claude/`, `.agents/`) | [x] | Ativos DMPF entram pelo `init`; o resto fica intacto |
| Documentação (`README.md`, `AGENTS.md`, `docs/`) | [x] | Quickstart, ADR-011, guia e criação dos grupos de release |
| Apps e libs | [ ] | Continuam vazias (ADR-009) |

## Localização de código

```text
nx-base-template/
  .npmrc                     — novo, só o registry do escopo (A2)
  dmpf.json, dmpf.rendered.json — novos, saída do init (A1)
  go.work, nx.json, .golangci.yml, .env.example — saída do init (A1)
  nx.json                    — sem release.projects e release.docker (A5)
  biome.json                 — exclusões dos arquivos do DMPF (A1)
  infra/                     — saída do init; o compose atual vai para a lixeira (A1)
  infra/docker/Dockerfile.node.example — secrets do registry (A2)
  tools/dmpf-baseline/       — novo, saída do init (A1)
  tools/template-smoke.sh    — novo, smoke do template (B2)
  .claude/, .agents/skills/  — ativos DMPF da saída do init (A1)
  renovate.json              — regras de lockstep (A4)
  .github/actions/setup-node-pnpm/ — credencial do registry (A2)
  .github/workflows/         — dmpf-ci.yml (B1), ci.yml (A2, B1, B2), nx-release.yml,
                               publish-libs.yaml, nx-publish-libs.yml (A2, A5)
  docs/adr/011-template-consome-dmpf-por-versao.md — novo (C2)
  docs/guides/primeiro-bounded-context.md          — novo (C5)
  docs/nx-reference/tasks.md — grupos de release e tags (A5, C1)
  README.md, AGENTS.md, docs/onboarding.md — (A5, C1)
```

**Arquivos a modificar**:

- `pnpm-workspace.yaml` e `package.json` — `@mateusmacedo/dmpf-plugin`
  `1.0.0-rc.3` no `catalog:` e na devDependency.
- `.github/workflows/ci.yml` — job `template-smoke` (B2), `packages: read`,
  `NODE_AUTH_TOKEN` (A2) e exclusão de `tag:stack:go` (B1).
- `.github/workflows/nx-release.yml`, `publish-libs.yaml`,
  `nx-publish-libs.yml` — credencial (A2) e filtros `!tag:stack:go` (A5).
- `docs/adr/README.md` — linha da ADR-011.

## Design

### Arquitetura

```text
github.com/mateusmacedo/dmpf (platform)          dmpf-nx-template (este repo)
  @mateusmacedo/dmpf-plugin@X ──npm.pkg.github──▶ package.json + saída do init
  dmpf-go-ci.yml@<sha> ◀──────── uses ─────────── .github/workflows/dmpf-ci.yml
  kernel vK ─────── proxy.golang.org ───────────▶ (só no projeto derivado, ao gerar contexto)
                                                          │ gh repo create --template
                                                          ▼
                                                  projeto greenfield
```

### Fluxo principal

1. `gh repo create <org>/<repo> --template mateusmacedo/dmpf-nx-template --clone`.
2. Token com `read:packages` no `~/.npmrc` (via `NODE_AUTH_TOKEN`) e
   `pnpm install`.
3. `pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=github.com/<org>/<repo> --composeProject=<repo>`
   e ajuste do nome do projeto Compose na semente.
4. `pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <nome> --boundedContext=<nome>`,
   seguido de `pnpm nx run-many -t tidy`, `conformance --write-baseline` e o
   primeiro `.proto`.
5. PR: `ci.yml` e `dmpf-ci.yml` verdes.
6. Atualização: `pnpm nx migrate @mateusmacedo/dmpf-plugin@<nova>`,
   `pnpm install` e `pnpm nx migrate --run-migrations`.

## Decisões técnicas

- **Saída do `init` commitada no template**: o projeto derivado já nasce com
  infra, gates e ativos de IA, e o mesmo `init` reconfigura o module path.
  Alternativa descartada: template vazio que exige `init` completo no primeiro
  uso, que deixaria o template sem prova de que a versão fixada funciona.
- **Prefixo reservado `example.com/change-me`**: força a configuração antes do
  primeiro contexto. Alternativa descartada: prefixo do próprio template, que
  vazaria para módulos de projetos derivados sem erro.
- **Fixar a `1.0.0-rc.3`**: não há versão estável, e a rc.3 é a única com o
  `workflowRef` e a diretiva Go atuais. A troca para a 1.0.0 é um
  `nx migrate`. Alternativa descartada: esperar a 1.0.0, sem data prevista.
- **`dmpf-ci.yml` vermelho e fora dos checks obrigatórios no template**: o
  reutilizável roda o gate de conformance sem condição, e editar o chamador
  violaria a P0 de arquivos DMPF. A prova de funcionamento no template é o
  `template-smoke`. Alternativa descartada: esperar uma versão do platform
  em que o `conformance` aceite workspace sem módulo Go.
- **Release sem grupos no template**: o topo do `release` fica só com opções
  globais, e cada grupo nasce com o primeiro membro. Alternativas
  descartadas: grupo `libs` explícito no template, que fica vazio num
  derivado só com Go e quebra o `nx release`; manter `release.projects`, que
  deixa todo derivado com o `nx.json` inválido no primeiro contexto.
- **Credencial do registry no CI pelo padrão do platform**: npmrc de usuário
  com placeholder e `NPM_CONFIG_USERCONFIG`, montado na composite antes do
  install. Alternativa descartada: token literal num npmrc, que grava o
  segredo em disco.
- **Biome fora dos arquivos do DMPF**: o pre-commit nunca reescreve arquivo
  gerado. Alternativa descartada: confiar que o JSON gerado já está no
  formato do Biome.
- **Go fora do pipeline JS**: o `ci.yml` exclui `tag:stack:go`, e o
  `dmpf-ci.yml` cobre os projetos Go. Alternativa descartada: instalar Go no
  job JS e repetir o que o `dmpf-ci.yml` já faz.
- **Smoke por cópia do working tree**: o mesmo script roda no CI e
  localmente, sem sujar o checkout. Alternativa descartada: rodar no próprio
  checkout do CI, que não é reproduzível localmente.
- **Smoke no CI em vez de app de exemplo**: coerente com a ADR-009; prova o
  template a cada PR sem código a manter. Alternativa descartada: app de
  exemplo versionada.
- **Renovate fora do DMPF**: kernel e plugin andam em lockstep; um bump avulso
  quebraria a compatibilidade declarada em `versions.json`.
- **ADR própria para o consumo do DMPF (ADR-011)**: a decisão de plataforma já
  está na ADR-010; as duas têm ciclos de revisão distintos.

## Regras relacionadas

- `AGENTS.md` e `CONTRIBUTING.md` — fluxo git-flow e convenções do template.
- ADR-007 — template autônomo; o plugin DMPF é dependência versionada, não
  acoplamento ao repositório do plugin de processo.
- SPEC-P846PH1P (repo `dmpf`) — contrato do plugin consumido aqui.

## Verificação e testes

### Critérios de aceite

- [x] `pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=example.com/change-me --npmScope=@mateusmacedo --composeProject=nx-base-template` sobre o template produz `git diff` vazio.
- [x] `go.work` com a diretiva Go de `versions.json` (1.26.9 na rc.3).
- [x] `nx.json` sem `release.projects` e sem `release.docker`; `pnpm nx release --dry-run` passa no smoke, com o grupo Go gerado. (No template sem projetos o comando continua falhando por grupo vazio, como antes desta spec.)
- [x] Nenhuma credencial em arquivo versionado; o `pnpm install` dos workflows e do `Dockerfile.node.example` autentica pelo npmrc de usuário.
- [ ] `template-smoke` verde em PR, em até 25 min, sem arquivos novos versionados ao final.
- [x] `dmpf-ci.yml` fora dos checks obrigatórios dos rulesets de `develop` e `master`.
- [x] ADR-011 publicada.
- [ ] README com quickstart executado do zero em repositório criado pelo template, até o primeiro contexto verde.
- [x] Validação do projeto passando: `pnpm biome ci .` e `pnpm nx run-many -t lint,typecheck,test,build`.

**Pendências da entrega (2026-10-10):** o `template-smoke` passou localmente duas vezes
(80 s e 53 s, com caches quentes); o tempo e o verde no runner do GitHub dependem do primeiro CI da PR. O
quickstart do zero depende do merge na `master`, porque o `gh repo create --template` usa
a branch padrão do template. O repositório não tem rulesets hoje, então o `dmpf-ci.yml`
não é obrigatório em nenhuma branch.

### Cenários de teste

```text
DADO um repositório criado a partir do template com a credencial no ~/.npmrc
QUANDO roda pnpm install, init --modulePrefix=github.com/org/repo, bounded-context,
       tidy, conformance --write-baseline e cria o primeiro .proto
ENTÃO o contexto compila, conformance sai com exit 0 e os go.mod requerem o
      kernel por tag, sem replace

DADO o template recém-clonado, ainda com o prefixo example.com/change-me
QUANDO roda bounded-context
ENTÃO o generator falha sem escrever arquivos e imprime o comando init

DADO um ambiente sem credencial do npm.pkg.github.com
QUANDO roda pnpm install
ENTÃO a instalação falha com erro de autenticação no npm.pkg.github.com, e o
      README indica como criar o token e onde gravá-lo

DADO um PR qualquer no template
QUANDO o CI roda
ENTÃO template-smoke gera e verifica um contexto descartável e termina sem
      deixar arquivos versionados novos, e o dmpf-ci.yml vermelho não bloqueia
      o merge
```

<critical_constraints>
- [P0] Todo arquivo DMPF do template é saída do plugin (`init`) na versão
  fixada; nenhum arquivo DMPF é escrito ou editado à mão.
- [P0] Nenhuma app ou lib de exemplo (ADR-009); o smoke de CI descarta o que gera.
- [P0] Token do GitHub Packages só por variável de ambiente ou secret.
- [P0] Kernel e plugin só se atualizam juntos, por `nx migrate`.
- [P1] Stacks JS/TS atuais e agentes e rules genéricos de `.claude/`
  permanecem como estão.
</critical_constraints>

## Escopo fora

- **Mudanças no platform** (plugin, conformance, workflow reutilizável):
  pertencem ao repositório `dmpf`. Inclui o gate de conformance em workspace
  sem módulo Go e o `frozen-lockfile` depois do `nx migrate`.
- **App de exemplo**: ADR-009; a referência viva é o `bookings` do platform.
- **BFF e borda REST pública**: o primeiro contexto é gRPC; borda HTTP é spec
  própria.
- **Suíte de carga k6 e dashboards de carga**: ficam no platform.
- **BOM e certificação de produto**: ADR-059 do platform.
- **Alterar as stacks JS/TS atuais**: permanecem como estão.
- **Migrar projetos existentes**: a entrega vale só para projetos novos.
- **Rulesets e proteção de branches no GitHub**: o repositório não tem
  rulesets hoje; ajustá-los é tarefa separada.
- **Pipeline de release próprio para contratos e imagens Go**: o template só
  impede que eles entrem no publish npm e no release Docker das stacks JS.
