# AGENTS.md

Guidance for agents working in this repository. `CLAUDE.md` na raiz aponta para este arquivo (`@AGENTS.md`).

As recomendações abaixo são defaults pensados para este projeto e podem ser ajustadas quando o contexto justificar. Regras marcadas como **duras** cobrem riscos que afetam segurança, integridade de dados ou trabalho alheio — essas devem ser respeitadas sempre.

## Índice

1. [Meta](#meta) — Propósito, idioma, resolução de conflitos
2. [Regras duras](#regras-duras) — Comportamentos não negociáveis
3. [Visão geral e estrutura](#visão-geral-e-estrutura) — Apps, libs, diretórios
4. [Comandos](#comandos) — Nx, Biome, scripts raiz
5. [Tooling](#tooling) — Stack efetiva do workspace
6. [Convenções obrigatórias](#convenções-obrigatórias) — Tags, targets, commits
7. [Git e release](#git-e-release)
8. [Padrões de código](#padrões-de-código) — Defaults recomendados
9. [Workflows](#workflows) — Processos de trabalho
10. [Referências](#referências) — Documentação complementar
11. [Glossário](#glossário)
12. [General Guidelines for working with Nx](#general-guidelines-for-working-with-nx) — bloco Nx (autoatualizado)

---

## Meta

### Propósito e escopo

- Este arquivo descreve convenções e o inventário real do repositório para agentes.
- Em caso de conflito entre instruções, considere este arquivo junto com os arquivos de configuração efetivos do projeto (por exemplo: `package.json` / `pnpm-workspace.yaml`, `lefthook.yml`, `biome.json`, `nx.json`, `tsconfig.base.json`).
- Este repositório é um **template**. Projetos derivados herdam estas convenções e devem atualizar o inventário desta seção conforme criam apps e libs reais.

### Autonomia do template

Este template é **autônomo**: o scaffold de engenharia fica versionado no próprio repositório e vale por si, sem plugin externo. Isso inclui o catálogo e o template de specs (`docs/specs/`), as regras de domínio (`docs/rules/**`, nos escopos `BIZ-`/`APP-`/`PRD-`), o guia detalhado de fluxo (`docs/guides/development-workflow.md`), o índice de ADRs (`docs/adr/README.md`), as rules documentais em `.claude/rules/` e as skills de domínio em `.claude/skills/`.

O fluxo git deste repositório é o git-flow do template (ver [Git e release](#git-e-release)). A decisão histórica de fronteira bake/consume (já supersedida) está em `docs/adr/007-fronteira-plugin-template.md`.

### Idioma

Por default, respostas são em português (PT-BR) e código permanece em inglês. O idioma da interação é parâmetro do projeto e pode ser ajustado.

### Resolução de conflitos

| Nível | Significado | Exemplo |
| --- | --- | --- |
| **Duro** | Não negociável; envolve segurança, integridade ou trabalho alheio | Autorização explícita para commit/push |
| **Recomendado** | Default esperado; pode ser ajustado com justificativa | Padrão arquitetural adotado |
| **Opcional** | Melhoria útil quando couber | Remoção proativa de código morto |

---

## Regras duras

Comportamentos abaixo são não negociáveis.

### Validação após cada tarefa

Ao finalizar código, execute a cadeia de validação do projeto (lint, typecheck e testes via `pnpm nx` / scripts raiz). Se um passo falhar, corrija e repita até todos passarem. Rode o formatter (Biome) antes do lint quando fizer sentido.

Para tarefas de interface, verifique visualmente o resultado quando houver ferramenta disponível; se não houver, pergunte ao usuário como proceder.

### Proteções não negociáveis

- Não modifique testes para passar — corrija o código sob teste.
- Não desabilite hooks/verificações (por exemplo, `--no-verify`) sem pedido explícito.
- Não execute force push ou operações Git destrutivas sem autorização explícita.
- Revise `git diff` antes de criar um commit.
- Peça confirmação explícita antes de commit/push.
- Ao documentar configurações, copie fielmente os valores reais em vez de generalizar.
- Nunca edite diretamente em `master` ou `develop`.

### Escopo de alterações

- Para solicitações explícitas, faça apenas o que foi pedido.
- Ao encontrar problemas adjacentes:
  - Mudanças pequenas e evidentes: corrija junto e mencione.
  - Mudanças médias: mencione e pergunte antes de aplicar.
  - Mudanças grandes: registre para tarefa separada.
- Refatorações fora do escopo solicitado dependem de autorização.

### Refatoração de símbolos

Ao refatorar qualquer símbolo (variável, função, tipo):

1. Busque todos os usos no projeto antes de alterar.
2. Não assuma que o caso mencionado é o único uso.

### Imports ao mover arquivos

Ao mover arquivos ou pastas, verifique todos os imports afetados — inclusive os que o compilador pode não acusar (por exemplo, caminhos absolutos em mocks).

---

## Visão geral e estrutura

`nx-base-template` (workspace npm `@nx-base-template/source`) é o baseline de monorepo **Nx + pnpm** multistack mantido por `mateusmacedo`. O workspace já vem preparado para Express, Fastify, NestJS, Next.js, Angular e Go (`go.work` / `@nx-go/nx-go`) e traz a saída do `init` do `@mateusmacedo/dmpf-plugin` para bounded contexts Go do DMPF (ADR-011), mas **não contém apps** — quem parte deste template cria as suas.

Fonte de verdade dos projetos: `pnpm nx show projects`.

### Diretórios principais

```text
nx-base-template/
├── apps/
│   ├── backend/                    # placeholder — sem projeto Nx
│   ├── frontend/                   # placeholder — sem projeto Nx
│   └── serverless/                 # placeholder — sem projeto Nx
├── libs/
│   ├── backend/                    # placeholder — sem projeto Nx
│   ├── frontend/                   # placeholder — sem projeto Nx
│   └── shared/                     # placeholder — sem projeto Nx
├── docs/
│   ├── adr/                        # ADR-0NN (baseline, tasks, hardening, release, plataforma, fechamento, fronteira, identidade de automação, libs, migração GitHub/Linear, consumo do DMPF)
│   ├── specs/                      # catálogo e template de specs (SPEC-XXXX)
│   ├── rules/                      # regras de domínio, com README de índice
│   │   ├── business/               # regras de negócio (BIZ-)
│   │   ├── application/            # regras de aplicação (APP-)
│   │   └── product/                # regras de produto (PRD-)
│   ├── guides/                     # development-workflow.md (guia detalhado do fluxo)
│   ├── nx-reference/               # guia prático de tasks Nx
│   ├── ci-cd/                      # adoção de CI/CD e deploy
│   └── onboarding.md               # setup local e primeiro PR
├── infra/
│   ├── docker/                     # Dockerfile de referência
│   └── local/, k8s/, observability/, test/  # saída do dmpf-plugin — não editar à mão
├── tools/                          # generators, executors e scripts do workspace
│   └── dmpf-baseline/              # baseline de conformidade (saída do dmpf-plugin)
├── .agents/skills/                 # skills de workspace (Nx) e dmpf-* (saída do dmpf-plugin)
├── .claude/                        # agents, skills e rules para assistentes
├── dmpf.json                       # configuração do DMPF (saída do init)
├── go.work                         # workspace Go (diretiva do dmpf-plugin)
├── lefthook.yml
├── nx.json
├── package.json
└── pnpm-workspace.yaml
```

Apps Nest criadas a partir daqui seguem tipicamente `src/app/<feature>/` (controllers, services, DTOs colocalizados). Libs exportam pela `src/index.ts` do pacote.

### Apps

Nenhuma. `apps/backend`, `apps/frontend` e `apps/serverless` são diretórios de destino, sem projeto Nx registrado. Para criar a primeira app, invoque a skill `nx-generate` antes de qualquer exploração.

### Libs

Nenhuma. `libs/backend`, `libs/frontend` e `libs/shared` são diretórios de destino, sem projeto Nx registrado. Para criar a primeira lib compartilhada, use o generator do Nx (`pnpm nx g @nx/js:lib libs/shared/<name>`), com as três tags 3D e `--linter=none`; o passo a passo com todas as flags está em `docs/nx-reference/tasks.md`. O `go.work` declara `go 1.26.9` (diretiva do `@mateusmacedo/dmpf-plugin`), mas não há módulos nem arquivos `.go` no workspace até o primeiro bounded context (ver `docs/guides/primeiro-bounded-context.md`).

Sem nenhum projeto com a tag `type:lib`, o `nx release` sai com erro (`Release group "__default__" matches no projects`) em vez de concluir vazio. Por isso o `nx-release.yml` tem o step `Detect lib release candidates`, que pula o versionamento e o push enquanto não houver lib — mesmo padrão do step de candidatos Docker. O pipeline fica armado e ocioso, não vermelho.

O prefixo dos pacotes é `@mateusmacedo/`, tudo em minúsculas. No GitHub Packages o escopo do pacote precisa ser o owner do repositório; projetos derivados trocam pelo owner deles. O casing precisa bater exatamente entre o `name` de cada `package.json`, os `paths` do `tsconfig.base.json` e o input `scope` da publicação (job `publish` do `nx-release.yml` e `nx-publish-libs.yml`): o Nx resolve o registry pelo escopo do pacote, e qualquer divergência faz o `pnpm publish` cair no registry público e falhar.

---

## Comandos

Sempre via `pnpm nx` — nunca o `nx` global. O root `@nx-base-template/source` tem targets `nx:noop`; exclua-o de operações em lote com `--exclude=@nx-base-template/source` (como no `lefthook` / CI).

```bash
# Em lote (todos os projetos com o target)
pnpm nx run-many -t build
pnpm nx run-many -t lint
pnpm nx run-many -t test
pnpm nx run-many -t typecheck

# Apenas projetos afetados (preferir no dia a dia e em CI)
pnpm nx affected -t lint
pnpm nx affected -t typecheck
pnpm nx affected -t test
pnpm nx affected -t build

# Um único projeto
pnpm nx test @mateusmacedo/minha-lib
pnpm nx build @mateusmacedo/minha-lib

# Um único arquivo de teste (passthrough p/ Jest)
pnpm nx test @mateusmacedo/minha-lib --testPathPatterns="string"

# Formatação (Biome — não Prettier)
pnpm biome format --write . # aplica
pnpm biome ci . # checa (CI)

# Grafo de dependências
pnpm nx graph
```

Scripts raiz (`pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm format`, `pnpm format:check`) também existem; para tasks Nx prefira `pnpm nx ...`.

---

## Tooling

- **Runtime:** Node.js `^24`; `pnpm@11.14.0` (campo `packageManager`). O CI não fixa a versão do pnpm: o `pnpm/action-setup` infere do `packageManager`, e passar ambos causa `ERR_PNPM_BAD_PM_VERSION`.
- **Package manager:** pnpm (obrigatório). Versões de dependências são centralizadas no `catalog:` do `pnpm-workspace.yaml` — cada `package.json` referencia `"catalog:"`. O mesmo arquivo tem `allowBuilds`, que é a allowlist de scripts de postinstall: pacotes marcados `false` estão bloqueados deliberadamente.
- **Nx:** `23.1.0`. NestJS `11.1.28` disponível via catalog.
- **DMPF:** `@mateusmacedo/dmpf-plugin` `1.0.0-rc.4` pelo catalog, vindo do GitHub Packages. O `.npmrc` versionado só aponta o escopo `@mateusmacedo` para o registry; a credencial fica no npmrc de usuário (`~/.npmrc` localmente, `$RUNNER_TEMP/npmrc` + `NPM_CONFIG_USERCONFIG` no CI), porque o pnpm 11.5.3+ ignora `${VAR}` em credencial do `.npmrc` do projeto. Arquivos DMPF são saída do `init` e só mudam por `nx migrate` (ADR-011); o Biome os exclui (`dmpf.json`, `dmpf.rendered.json`, `infra/`, `tools/dmpf-baseline/`). Versão recém-publicada do plugin entra no `minimumReleaseAgeExclude` do pnpm.
- **Lint + format:** **Biome 2.4.16** é a ferramenta principal de format/lint. Estilo: 2 espaços, `lineWidth` 100, aspas simples, trailing commas `all`, semicolons sempre, arrow parens sempre. Regras notáveis: `noUnusedVariables: error`, `noExplicitAny: warn` e `useImportType: error` — neste workspace `import type` é obrigatório, não preferência. O parser tem `unsafeParameterDecoratorsEnabled: true` para suportar decorators de parâmetro do NestJS.
- **Testes:** Jest 30 transpilado por **SWC** (`@swc/jest`), config em `.spec.swcrc` (decorators + `keepClassNames` para DI do NestJS). O preset em `jest.preset.js` usa `passWithNoTests: true`, então projeto sem teste não quebra o lote.
- **TypeScript:** `strict: true`, `module`/`moduleResolution: nodenext`, `target: es2022`, Project References (`composite: true`, `emitDeclarationOnly: true`). O `tsconfig.base.json` tem `paths` vazio e o `tsconfig.json` tem `references` vazio: cada lib nova acrescenta a própria entrada nos dois.
- **Git hooks:** Lefthook (não husky), instalado via `pnpm prepare`. O script é tolerante a falha (`lefthook install || true`), para não quebrar `pnpm install` em ambiente sem o binário. `pre-commit` roda `biome check --write` nos arquivos staged; `pre-push` roda `nx affected` de lint/typecheck/test/build com `--parallel=3`, excluindo `@nx-base-template/source`.

---

## Convenções obrigatórias

- **Tags 3D em todo app/lib de produção:** uma de cada dimensão, conforme a taxonomia canônica abaixo. Ex.: `["type:lib", "scope:shared", "stack:node"]`. O publish npm do Nx Release cobre só projetos `type:lib` fora de Go (`tag:type:lib,!tag:stack:go`), organizados em grupos de release que nascem com o primeiro membro (ver "Release por grupos" em `docs/nx-reference/tasks.md`). Exceção conhecida: `@nx-base-template/source` (metadado raiz).

  | Dimensão | Valores válidos |
  | --- | --- |
  | `type:` | `lib`, `app`, `e2e` |
  | `scope:` | `shared`, `backend`, `frontend` |
  | `stack:` | `node`, `express`, `fastify`, `nest`, `next`, `react`, `angular`, `go`, `universal` |

  Esta tabela é a **fonte canônica** da taxonomia; `docs/nx-reference/tasks.md` a repete e não deve divergir. `stack:universal` é para libs sem dependência de runtime (tipos puros, utilitários); `stack:node` cobre libs que usam APIs de Node ou frameworks de servidor. Os projetos Go gerados pelo `bounded-context` do `@mateusmacedo/dmpf-plugin` levam, além das três, uma tag `layer:` (por exemplo, `layer:contract`, `layer:apps`), definida pelo plugin.
- **Não redeclarar targets** que um plugin ou `targetDefaults` (em `nx.json`) já fornece. Redeclarar quebra o cache silenciosamente (ver `docs/adr/002-nx-task-configuration.md`).
- **Commits (Conventional Commits, em PT-BR):** formato `<tipo>(<scope>): <descrição imperativa>`, máx. 72 chars no assunto. `scope` = nome do projeto Nx **sem** o prefixo do escopo npm (`minha-lib`, não `@mateusmacedo/minha-lib`). Projetos distintos vão em commits separados (nunca misture libs). Tipos: `feat`, `fix`, `refactor`, `chore`, `docs`, `ci`. Body só quando o motivo não é óbvio. Detalhes na skill `.agents/skills/nx-commit/`.

---

## Git e release

- **Plataforma:** **GitHub** (`github.com/mateusmacedo/dmpf-nx-template`, repositório marcado como template). O `gh` CLI opera normalmente contra ele. Decisão e contexto da migração em `docs/adr/010-migracao-github-linear.md` (ADR-010, que supersede a ADR-005).
- **Tracker:** projeto [nx-base-template](https://linear.app/mmda/project/nx-base-template-db3f16eab767) no Linear (time DevTeam, chave `DEVS`). Cada spec tem uma issue, ligada pelo `ticket_url` do frontmatter; branch e título de PR levam a chave `DEVS-<n>`. Regras de sincronização: `docs/onboarding.md`.
- **Branches protegidas:** `master` e `develop`, por rulesets do GitHub. `develop` exige PR e o check do CI verde, sem deleção e sem force push. `master` fica sem deleção e sem force push; o bot de release empurra nela o commit de versão e as tags. `defaultBase` do Nx é `master`.
- **Fluxo (git-flow):** trabalho → `develop` (validação) → `release/X.Y.Z` (após validar) → `master`. Antes do PR para `develop`, a branch de trabalho mergeia `release/X.Y.Z` (updates já aprovados). PRs de feature comparam contra `origin/develop` por padrão.
- **Anti-drift (duro):** ao promover para `release`, usar a **mesma árvore** já mergeada em `develop` (mesmo tip da feature). Não abrir promoção paralela com resolução/conteúdo diferente — isso faz o merge `release` → branch de trabalho reabrir os mesmos conflitos. Detalhe em `CONTRIBUTING.md`.
- **Release:** Nx Release com versionamento **independente** por projeto, baseado em Conventional Commits; tag pattern `{projectName}@{version}`. O `nx.json` não declara `release.projects` nem grupos: os grupos `libs`, `apps` e `go-contract-*` nascem com o primeiro membro (ver "Release por grupos" em `docs/nx-reference/tasks.md`). A release é encadeada:
  - `create-release.yml` (dispatch, input `bump-type`) cria e empurra `release/X.Y.Z` a partir de `master` e avisa para abrir o PR para `master` **com a sua conta** — PR aberto pelo `GITHUB_TOKEN` não dispara o CI.
  - `nx-release.yml` roda no merge de `release/*` ou `hotfix/*` em `master` (ou por dispatch em `master`): `nx release --skip-publish`, push atômico do commit e só das tags criadas no run, release Docker no `ghcr.io` quando houver `type:app` fora de Go (`!tag:stack:go`) e o job `publish`, encadeado por `needs`, que chama o reutilizável `publish-libs.yaml`. O encadeamento é necessário porque tag empurrada com `GITHUB_TOKEN` não dispara outro workflow.
  - `nx-publish-libs.yml` é a republicação **manual** (dispatch com os inputs `tag` e `ref`), pelo mesmo `publish-libs.yaml`, para quando a publicação encadeada falhar.
  - A publicação das libs vai para o GitHub Packages (`https://npm.pkg.github.com`, escopo `@mateusmacedo`), autenticada no CI só com `secrets.GITHUB_TOKEN` (`packages: write`). Quem consome libs publicadas usa um token com `read:packages` no `~/.npmrc` (`//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}`) e `@mateusmacedo:registry=https://npm.pkg.github.com` no `.npmrc` do projeto.
  - A separação entre versionamento e publicação vem da ADR-004; o alvo de publicação e o runner atuais seguem a ADR-010, que a supersede em parte.
- **CI:** `.github/workflows/ci.yml` roda em PRs para `master`, `develop` e `release/**` (ignora mudanças só em `**/*.md` e em `.github/ISSUE_TEMPLATE/**`), no runner `ubuntu-26.04`: `biome ci`, depois `nx affected` de lint, typecheck, test (com `--ci --coverage`), build e e2e, excluindo `tag:stack:go`. Os projetos Go passam pelo `.github/workflows/dmpf-ci.yml`, saída do `init` que chama o reutilizável do platform fixado por SHA; no template, sem módulo Go, ele falha no gate de conformance e não é check obrigatório.
- **Demais workflows:** `publish-libs.yaml` e `detect-apps.yaml` são reutilizáveis **locais** (`on: workflow_call`), chamados por `uses: ./.github/workflows/...`. O `cd-dev-hmg.yml` é um **template de CD desligado**, com apenas `workflow_dispatch` e o job de deploy comentado (ver `docs/ci-cd/`); seu job `detect` usa o `detect-apps.yaml` local, e o deploy comentado aponta para `./.github/workflows/deploy-eks.yaml`, reutilizável que **não existe** neste repositório. Ele deve ser criado quando o CD for ligado, com roles AWS, registry e repositório de infraestrutura do próprio projeto; o mesmo vale para os modelos ECS, on-premises e Lambda.

---

## Padrões de código

Os itens abaixo são defaults **recomendados**. Divergências são aceitáveis quando o contexto justificar.

### Convenções gerais

Consulte as skills e guias específicos para detalhes. Em resumo:

- Favoreça código auto-explicativo; reserve comentários para decisões não óbvias.
- Prefira `type` a `interface` quando não houver extensão/merge envolvido.
- Prefira arrow functions com `const` para funções no nível de módulo.
- Use `import type` para imports exclusivamente de tipos — aqui isso é regra do linter.
- Nomes de variáveis, funções, tipos e constantes em inglês; evite misturar idiomas.
- Verifique reuso antes de criar algo novo.
- Quando houver dados estáticos de configuração, considere separá-los em módulo próprio se isso melhorar a leitura.

### Organização de código compartilhado

- Utilitários específicos de um consumidor podem ficar colocalizados com ele.
- Mova código para uma camada compartilhada (`libs/`) quando dois ou mais módulos de pastas/apps diferentes passarem a depender dele.

### Textos para usuário final

Ao escrever textos visíveis (labels, placeholders, mensagens), use a ortografia correta do idioma alvo. Em PT-BR, preserve acentuação.

### Consulta a skills e padrões existentes antes de implementar

Antes de implementar algo novo:

1. Identificar a categoria da tarefa.
2. Ler as skills relacionadas em `.claude/skills/` (domínio) e `.agents/skills/` (Nx/workspace).
3. Verificar documentação interna em `docs/` e ADRs.
4. Buscar implementações similares no próprio código.

Isso reduz retrabalho e inconsistência. Declarar explicitamente o que foi consultado ajuda na revisão.

### Testes

- Enquadre testes no padrão do projeto: Jest + SWC, arquivos `*.spec.ts` colocalizados (ou E2E sob `apps/*-e2e`).
- Funções com lógica não trivial costumam ter testes associados.
- Para componentes/UI, avalie o custo/benefício; quando em dúvida, pergunte.
- Não altere testes só para "passar" — corrija o código sob teste (regra dura).

### Qualidade e CI

Utilize Biome, typecheck, Jest e os hooks Lefthook configurados. O princípio — "todo commit sai verde" — é o que importa. A cadeia típica pós-tarefa:

```bash
pnpm biome check --write .
pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source
```

(Ajuste o conjunto de targets ao impacto da mudança.)

### Uso de APIs e ferramentas externas

Ao usar APIs, SDKs ou ferramentas externas pouco familiares, consulte a documentação oficial antes. Se algo não ficar claro, pergunte em vez de inferir.

---

## Workflows

### Fluxo recomendado

1. Suba o ambiente: `corepack enable` e `pnpm install` (o install registra os hooks do Lefthook).
2. Para criar a primeira app ou lib, invoque a skill `nx-generate` — ela cobre a descoberta de generators.
3. Após cada tarefa, rode a cadeia de validação.
4. Confie nos hooks de pré-commit/pré-push para reforçar a validação — não os contorne.

### Protocolo de alterações

1. **Análise** — entenda o impacto e os arquivos envolvidos (`pnpm nx show projects`, grafo, consumidores das libs).
2. **Execução** — aplique a alteração mínima suficiente.
3. **Validação** — verifique regressões e rode a cadeia de validação.

### Orquestração de agentes e skills

Delegue para agentes/skills quando a tarefa envolver múltiplos arquivos, decisões arquiteturais ou conhecimento específico (segurança, NestJS, testes E2E, etc.). Para ajustes pontuais, faça direto.

- Scaffolding (apps/libs): invoque a skill `nx-generate` **antes** de explorar ou gerar.
- Navegação do workspace: skill `nx-workspace`.
- Commits: skill `nx-commit` (scopes por projeto Nx).

### Melhoria contínua (opcional)

Antes de criar algo novo, considere:

- Reusar código existente.
- Identificar duplicações.
- Remover código morto, após confirmar usos.

---

## Referências

- `CONTRIBUTING.md` — entrada curta para contribuir, com o modelo anti-drift.
- `docs/onboarding.md` — setup local e primeiro PR.
- `docs/adr/` — decisões arquiteturais (baseline, tasks Nx, hardening, release, plataforma, fechamento, autonomia do template, migração para GitHub e Linear, consumo do DMPF por versão).
- `docs/adr/README.md` — índice e template de ADRs.
- `docs/adr/007-fronteira-plugin-template.md` — ADR supersedido; o template é autônomo (sem consumo de plugin externo).
- `docs/adr/010-migracao-github-linear.md` — plataforma GitHub, publicação no GitHub Packages e tracker Linear; supersede a ADR-005 e, em parte, as ADR-004 e ADR-008.
- `docs/adr/011-template-consome-dmpf-por-versao.md` — consumo do DMPF por versão: plugin, credencial do registry, smoke, release sem grupos e `dmpf-ci.yml`.
- `docs/guides/primeiro-bounded-context.md` — do template ao primeiro bounded context verde.
- `docs/nx-reference/tasks.md` — guia prático de configuração de tasks.
- `docs/specs/` — specs de produto/técnicas.
- `docs/specs/README.md` — catálogo e template de specs.
- `docs/rules/README.md` — índice das regras de domínio (negócio, aplicação, produto).
- `docs/guides/development-workflow.md` — guia detalhado do fluxo de trabalho.
- `docs/ci-cd/` — guia de adoção de CI/CD e deploy.
- `infra/docker/Dockerfile.node.example` — Dockerfile de referência para apps Node.
- `.agents/skills/` — skills de workspace (`nx-workspace`, `nx-generate`, `nx-commit`, `monitor-ci`, entre outras).
- `.claude/skills/` — skills de domínio (NestJS, Jest, TypeScript, Go, review, etc.).
- `.claude/agents/` — agentes especializados; índice em `.claude/README.md`.
- `.claude/rules/` — regras de conduta e de ferramentas.
- `README.md` — visão do monorepo; se divergir do inventário Nx, trate `pnpm nx show projects` e este arquivo como fonte da verdade.

---

## Glossário

| Termo | Definição |
| --- | --- |
| **Lógica complexa** | Critério ajustável; por exemplo, múltiplos estados derivados ou efeitos colaterais interligados |
| **Alteração mínima** | Mudança restrita ao necessário para cumprir o pedido |
| **Código morto** | Funções, componentes ou flags sem uso efetivo |
| **Regra dura** | Comportamento não negociável (segurança, integridade, trabalho alheio) |
| **Affected** | Conjunto de projetos Nx impactados pela mudança em relação à base Git |

<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

# General Guidelines for working with Nx

- For navigating/exploring the workspace, invoke the `nx-workspace` skill first - it has patterns for querying projects, targets, and dependencies
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- Prefix nx commands with the workspace's package manager (e.g., `pnpm nx build`, `npm exec nx test`) - avoids using globally installed CLI
- You have access to the Nx MCP server and its tools, use them to help the user
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.
- NEVER guess CLI flags - always check nx_docs or `--help` first when unsure

## Scaffolding & Generators

- For scaffolding tasks (creating apps, libs, project structure, setup), ALWAYS invoke the `nx-generate` skill FIRST before exploring or calling MCP tools

## When to use nx_docs

- USE for: advanced config options, unfamiliar flags, migration guides, plugin configuration, edge cases
- DON'T USE for: basic generator syntax (`nx g @nx/react:app`), standard commands, things you already know
- The `nx-generate` skill handles generator discovery internally - don't call nx_docs just to look up generator syntax

<!-- nx configuration end-->
