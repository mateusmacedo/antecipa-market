---
id: SPEC-317RXZ73
slug: migrar-mantenedor-github-linear
title: Migrar a manutenção do template para mateusmacedo — GitHub, GitHub Packages e Linear
stage: done
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-72/spec-317rxz73-migrar-a-manutencao-do-template-para-mateusmacedo-github
subtask_urls: []
created: 2026-10-09
---

# SPEC-317RXZ73: Migrar a manutenção do template para mateusmacedo — GitHub, GitHub Packages e Linear

## Resumo

O template deixa de ser mantido pela organização contratante e passa a ser
mantido por `mateusmacedo`, em `github.com/mateusmacedo/dmpf-nx-template`.
Esta spec troca a plataforma (Gitea → GitHub), o registry (Verdaccio → GitHub
Packages, escopo `@mateusmacedo`) e o tracker (Jira → Linear, projeto
`nx-base-template` no time DevTeam), internaliza os workflows reutilizáveis
usados do `actions-templates` e retira do repositório a identificação da
contratante, seguindo o precedente da ADR-043 do `dmpf`.

## Contexto

- **Problema**:
  - Os workflows rodam em `gitea-runner` (`ci.yml:24`, `create-release.yml:27`,
    `nx-release.yml:38`), o `create-release.yml` abre PR pela API do Gitea
    (`/api/v1`) e o bot de release usa um e-mail do domínio da contratante
    (`nx-release.yml:65-66`).
  - `nx-publish-libs.yml:37` e `cd-dev-hmg.yml:43` chamam reutilizáveis de um
    repositório da contratante no Gitea, inacessível a partir do GitHub; o
    `publish-libs.yaml` de lá publica num Verdaccio privado com credencial via
    OIDC/SSM da AWS da contratante.
  - No GitHub, tag empurrada com `GITHUB_TOKEN` não dispara outro workflow: o
    gatilho `push: tags` do `nx-publish-libs.yml` nunca rodaria depois do
    `nx-release.yml`.
  - 13 documentos vivos descrevem Gitea, Verdaccio e o escopo antigo; specs e
    ADRs citam a contratante pelo nome, URLs do Jira dela e caminhos de máquina.
  - As issues das specs do template já existem no Linear (DEVS-9, DEVS-10,
    DEVS-11, DEVS-20, DEVS-71), soltas no projeto DMPF; as specs ainda apontam
    para o Jira.
  - O repositório no GitHub está vazio (sem branches), não é template e não
    tem regras de proteção.
- **Referência**: o `dmpf` (`github.com/mateusmacedo/dmpf`) já fez a mesma
  migração — ADR-043, workflows `create-release.yml`, `nx-release.yml`,
  `publish-libs.yaml`, `detect-apps.yaml`, `nx-publish-libs.yml` e os
  rulesets "CI obrigatório na develop" e "Proteção da master".
- **Links relevantes**:
  - SPEC-TXJ6A4XX (DEVS-71) — perde os itens de plataforma para esta spec e
    passa a depender dela.
  - ADR-004, ADR-005, ADR-008 — decisões da era Gitea que esta spec supersede.

<constraints>
- [P0] Nenhuma credencial em arquivo versionado ou log; publicação autentica
  só com `secrets.GITHUB_TOKEN`.
- [P0] Registros históricos (ADRs, specs concluídas) mantêm as decisões e os
  IDs `ARQ-<n>`; mudam apenas o nome da contratante, URLs do tracker dela e
  caminhos de máquina. ADRs substituídas mudam o status, não a decisão.
- [P0] Workflows internalizados não carregam nada da contratante (URLs, conta
  AWS, nomes de repositório, runner, e-mail).
- [P0] Não redeclarar targets nem mudar o versionamento do Nx Release; muda
  só a identidade do bot, o push e o encadeamento da publicação.
- [P1] Stacks, tooling (Biome, Jest, Lefthook), `nx.json` e `.claude/`
  permanecem como estão.
</constraints>

## Requisitos

### Funcionais

**A. Linear**

- [x] **[P0] A1 — Projeto**: criar o projeto `nx-base-template` no time
  DevTeam, lead `mateusmacedo`, com resumo, descrição no formato do projeto
  DMPF (uma spec = uma issue, stage ↔ status, prioridade, fluxo git) e links
  para o repositório, `AGENTS.md`, catálogo de specs e índice de ADRs.
- [x] **[P0] A2 — Issues no projeto**: mover DEVS-9, DEVS-10, DEVS-11,
  DEVS-20 e DEVS-71 do projeto DMPF para `nx-base-template`; criar a issue
  desta spec e a issue (Done) da SPEC-4NR9KKS8, que ainda não tem.
- [x] **[P0] A3 — `ticket_url` no Linear**: toda spec do catálogo aponta para
  a issue do Linear.
- [x] **[P1] A4 — Dependência**: DEVS-71 bloqueada pela issue desta spec.

**B. Workflows no GitHub**

- [x] **[P0] B1 — Runner hospedado**: todo `runs-on` e todo default de input
  `runner` usam `ubuntu-26.04`, o mesmo do `dmpf`.
- [x] **[P0] B2 — Branch de release**: `create-release.yml` cria e empurra
  `release/X.Y.Z` e emite um aviso para abrir o PR com a conta do mantenedor
  (PR aberto pelo `GITHUB_TOKEN` não dispara o CI); sem `curl` em `/api/v1`
  e sem `pull-requests: write`. A versão do ciclo anterior vem do assunto dos
  merges em `master`, porque o GitHub apaga a branch mergeada.
- [x] **[P0] B3 — Identidade do bot**: `nx-release.yml` comita como
  `github-actions[bot]` /
  `41898282+github-actions[bot]@users.noreply.github.com`.
- [x] **[P0] B4 — Reutilizáveis locais**: `detect-apps.yaml` e
  `publish-libs.yaml` passam a viver em `.github/workflows/` (portados do
  `dmpf`) e são chamados por `uses: ./.github/workflows/...`. O
  `deploy-eks.yaml` não é internalizado: o exemplo comentado do
  `cd-dev-hmg.yml` aponta para o caminho local a criar quando o CD for ligado.
- [x] **[P0] B5 — Publicação encadeada**: o `nx-release.yml` empurra o commit
  e só as tags criadas no run (`git push --atomic`) e chama
  `publish-libs.yaml` num job `publish` com `needs`; o `nx-publish-libs.yml`
  vira republicação manual (`workflow_dispatch` com `tag` e `ref`). A
  publicação usa `https://npm.pkg.github.com`, escopo `@mateusmacedo`,
  `permissions: packages: write` e `secrets.GITHUB_TOKEN`; sem AWS, OIDC ou
  Verdaccio.
- [x] **[P0] B6 — Escopo npm**: exemplos, regras e a skill `nx-commit` usam
  `@mateusmacedo/<lib>`.

**C. Documentação e governança**

- [x] **[P0] C1 — ADR-010**: "Migrar a manutenção para o GitHub e o Linear".
  Supersede a ADR-005 por inteiro e, parcialmente, a ADR-004 (alvo de
  publicação e runner) e a ADR-008 (identidade do bot e push); essas recebem
  o status atualizado e o índice `docs/adr/README.md` também.
- [x] **[P0] C2 — Docs vivas**: `AGENTS.md`, `CONTRIBUTING.md`, `README.md`,
  `docs/onboarding.md`, `docs/guides/development-workflow.md`,
  `docs/nx-reference/tasks.md`, `docs/ci-cd/*`, `docs/specs/README.md`,
  `.agents/skills/nx-commit/SKILL.md` e `.github/PULL_REQUEST_TEMPLATE.md`
  descrevem GitHub, `gh`, GitHub Packages, os nomes reais dos workflows e o
  tracker Linear: projeto, `DEVS-<n>` na branch e no título do PR, e a
  sincronização spec ↔ issue (título, status, prioridade, `depends_on`).
- [x] **[P0] C3 — Sem identificação da contratante**: nenhum arquivo
  versionado cita o nome da contratante ou do produto dela, URLs do Jira
  dela ou caminhos de máquina; os registros históricos usam termos neutros
  ("contratante", pseudônimos estáveis). As specs SPEC-W4RWD02M e
  SPEC-4NR9KKS8 trocam o slug e o nome do arquivo.
- [x] **[P1] C4 — Licença**: `LICENSE` com copyright de Mateus Macedo Dos
  Anjos, como no `dmpf`.
- [x] **[P1] C5 — SPEC-TXJ6A4XX**: os itens B3, B4, C3 e C4 dela passam para
  esta spec; ela ganha `depends_on: [SPEC-317RXZ73]` e a ADR do DMPF passa a
  ser a ADR-011.

**D. Repositório no GitHub**

- [x] **[P0] D1 — Branches publicadas**: `master` e `develop` em
  `github.com/mateusmacedo/dmpf-nx-template`, com `master` como branch
  padrão. As tags antigas não são publicadas.
- [x] **[P1] D2 — Template e rulesets**: repositório marcado como template;
  ruleset da `develop` (sem deleção, sem force push, PR obrigatório sem
  aprovação mínima, check do CI obrigatório, bypass de admin só por PR) e da
  `master` (sem deleção, sem force push), como no `dmpf`.

### Não-funcionais

- [x] Segurança: `permissions` mínimas por workflow e por job; nenhum token
  impresso.
- [x] Compatibilidade: Node 24, pnpm 11.14.0, Nx 23.1.0; versões de actions
  iguais às já usadas no template (`@v4`, `@v5` do `nx-set-shas`).
- [x] Comentários nos workflows seguem a regra do `CLAUDE.md` do projeto
  (só o porquê, no máximo 3 linhas).

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| CI/CD (`.github/workflows/`) | [x] | Runner, release, reutilizáveis locais, GitHub Packages |
| Ativos de IA (`.agents/skills/nx-commit`) | [x] | Escopo npm do exemplo |
| Documentação (`README.md`, `AGENTS.md`, `docs/`, `LICENSE`) | [x] | Plataforma, registry, tracker, ADR-010, anonimização |
| Specs (`docs/specs/`) | [x] | `ticket_url`, renomeação, SPEC-TXJ6A4XX dependente |
| Linear e GitHub (fora do repo) | [x] | Projeto, issues, branches, template, rulesets |
| Apps, libs, tooling, `nx.json` | [ ] | Sem mudança |

## Localização de código

```text
.github/workflows/ci.yml                 — B1
.github/workflows/create-release.yml     — B1, B2
.github/workflows/nx-release.yml         — B1, B3, B5
.github/workflows/nx-publish-libs.yml    — B5 (republicação manual)
.github/workflows/cd-dev-hmg.yml         — B4 (detect local, deploy comentado)
.github/workflows/publish-libs.yaml      — novo (B4, B5)
.github/workflows/detect-apps.yaml       — novo (B4)
.github/PULL_REQUEST_TEMPLATE.md         — C2
.agents/skills/nx-commit/SKILL.md        — B6
LICENSE                                  — C4
docs/adr/010-migracao-github-linear.md   — novo (C1)
docs/adr/*.md, docs/adr/README.md        — status (C1), anonimização (C3)
AGENTS.md, CONTRIBUTING.md, README.md, docs/onboarding.md,
docs/guides/development-workflow.md, docs/nx-reference/tasks.md,
docs/ci-cd/*.md, docs/specs/README.md    — C2, C3
docs/specs/SPEC-*.md                     — A3, C3, C5
```

## Design

### Arquitetura

```text
antes                                         depois
Gitea da contratante                          github.com/mateusmacedo/dmpf-nx-template
  runs-on: gitea-runner                         runs-on: ubuntu-26.04
  create-release: curl /api/v1/.../pulls        create-release: push da branch + aviso
  uses: <org>/actions-templates/...@main        uses: ./.github/workflows/*.yaml
  push de tag → nx-publish-libs (Verdaccio)     nx-release → job publish (needs) → GitHub Packages
  Jira ARQ-<n>                                  Linear DEVS-<n>, projeto nx-base-template
```

### Fluxo de release

1. `create-release.yml` (dispatch) cria `release/X.Y.Z` a partir de `master`;
   o mantenedor abre o PR para `master`.
2. Merge em `master` dispara `nx-release.yml`: `nx release --skip-publish`,
   push atômico de `master` e das tags novas.
3. O job `publish` do mesmo run chama `publish-libs.yaml` no commit de versão.
4. Se a publicação falhar, `nx-publish-libs.yml` (dispatch) republica.

## Decisões técnicas

- **Portar do `dmpf` em vez de reescrever**: os reutilizáveis e o fluxo de
  release de lá já rodam no GitHub com o mesmo Nx e o mesmo pnpm; a
  divergência fica restrita ao que é do DMPF (grupos Go, plugin, consumidor).
- **Publicação encadeada por `needs`**: o GitHub não dispara workflow a partir
  de evento gerado pelo `GITHUB_TOKEN`; sem o encadeamento, nenhuma lib seria
  publicada.
- **Sem PR automático de release**: o mesmo motivo — um PR do bot não roda o
  CI; o aviso no run orienta abrir o PR com a conta do mantenedor.
- **`deploy-eks.yaml` fora**: o original depende de roles AWS, repositório
  GitOps e bot da contratante; generalizá-lo sem um alvo real seria código
  não testado. Mesmo corte da ADR-043 do `dmpf`.
- **Anonimizar em vez de apagar o histórico**: as decisões continuam
  legíveis; só sai o que identifica a contratante. Os IDs `ARQ-<n>` ficam,
  as URLs do tenant dela saem.
- **ADR-010 para a plataforma, ADR-011 para o DMPF**: a migração chega antes
  do bootstrap DMPF e ocupa o próximo número livre.

## Regras relacionadas

- `AGENTS.md` e `CONTRIBUTING.md` — git-flow, Conventional Commits e escopo.
- ADR-002 — não redeclarar targets.
- `.claude/rules/git-safety.md` — branches protegidas e revisão antes de
  commit.

## Verificação e testes

### Critérios de aceite

- [x] `git grep -ilE 'liderca[p]|tele ?se[n]a|atlassia[n]'` retorna 0 arquivos.
- [x] Docs vivas não descrevem Gitea, Verdaccio ou Jira como vigentes: `rg -n -i 'gitea|verdaccio|jira' AGENTS.md CONTRIBUTING.md README.md docs/onboarding.md docs/guides docs/nx-reference docs/ci-cd docs/specs/README.md .agents .github | rg -v '00[45]-(workflows-verdaccio|plataforma-gitea)'` retorna 0 linhas (sobram só links para os arquivos das ADRs históricas).
- [x] `rg -n 'gitea-runner|/api/v1|actions-templates' .github` retorna 0 linhas.
- [x] `actionlint` com shellcheck sem achados nos workflows criados ou alterados por esta spec (os achados `info` em linhas antigas do `ci.yml` e do `create-release.yml` ficam fora).
- [x] Projeto Linear `nx-base-template` com as 7 issues das specs do template, e cada `ticket_url` resolvendo para a issue de mesmo `SPEC-<id>`.
- [x] `gh repo view mateusmacedo/dmpf-nx-template --json isTemplate,defaultBranchRef` retorna `true` e `master`.
- [x] Rulesets ativos em `develop` e `master` (`gh api repos/mateusmacedo/dmpf-nx-template/rulesets`).
- [x] CI do PR desta spec verde em `ubuntu-26.04`.
- [x] `pnpm biome ci .` e `pnpm nx run-many -t lint,typecheck,test,build --exclude=@nx-base-template/source` verdes.

### Cenários de teste

```text
DADO um PR para develop no GitHub
QUANDO o ci.yml roda
ENTÃO o job sobe em ubuntu-26.04 e executa biome ci e nx affected sem erro

DADO o create-release.yml disparado com bump-type=patch
QUANDO o job termina
ENTÃO existe a branch release/X.Y.Z no remoto e o run emite o aviso para
      abrir o PR, sem chamar API de PR

DADO um workspace sem nenhuma lib type:lib (estado atual)
QUANDO o nx-release.yml roda após o merge de release/X.Y.Z em master
ENTÃO versionamento, push e publish são pulados, e o run termina verde

DADO uma lib @mateusmacedo/<lib> com tag type:lib e commits novos
QUANDO o nx-release.yml roda após o merge da release
ENTÃO o job publish publica a versão nova no GitHub Packages usando só o
      GITHUB_TOKEN

DADO o cd-dev-hmg.yml disparado manualmente sem app
QUANDO o job detect roda
ENTÃO usa o detect-apps.yaml local e não referencia repositório externo
```

<critical_constraints>
- [P0] Nenhuma credencial em arquivo versionado ou log; publicação só com
  `secrets.GITHUB_TOKEN`.
- [P0] Registros históricos mantêm decisões e IDs `ARQ-<n>`; mudam só nome da
  contratante, URLs do tracker dela e caminhos de máquina.
- [P0] Workflows internalizados sem nada da contratante.
- [P0] Sem redeclarar targets nem mudar o versionamento do Nx Release.
- [P1] Stacks, tooling, `nx.json` e `.claude/` permanecem como estão.
</critical_constraints>

## Escopo fora

- **Bootstrap DMPF** (plugin, `.npmrc` de consumo, smoke, ADR-011): continua
  na SPEC-TXJ6A4XX.
- **`deploy-eks.yaml` e demais reutilizáveis do `actions-templates`**: só
  entram quando o CD for ligado, com alvo real.
- **GitHub Releases pelo Nx (`createRelease`)**: mudaria o fluxo da ADR-008;
  vira spec própria se for desejado.
- **Reescrever a autoria dos commits**: o histórico carrega o e-mail
  corporativo antigo; corrigir muda todos os SHAs. Mesma decisão do `dmpf`.
- **Tags antigas das libs removidas** (`*/shared-*@0.0.x`): não são publicadas.
- **Repositórios da contratante**: não são alterados.
- **Placeholders do `CODEOWNERS` e campos `author`/`repository` do
  `package.json` raiz**: pertencem a quem adota o template.
- **Caminho local do clone**: fora do repositório.
