# ADR-011: O template consome o DMPF por versão

## Status

Aceito — 2026-10-10. Implementa SPEC-TXJ6A4XX (DEVS-71).

## Contexto

O template não tinha nada de DMPF: nenhum `go.work` útil, nenhum gate, nenhuma
infra e nenhum ativo de IA de bounded context. Quem quisesse um projeto DMPF
copiava arquivos do repositório `dmpf` (o platform) e passava a mantê-los à
mão.

A SPEC-P846PH1P do platform publicou o plugin `@mateusmacedo/dmpf-plugin` no
GitHub Packages, com os generators `init` e `bounded-context`, executors,
migrations, ativos de IA e o workflow reutilizável `dmpf-go-ci.yml`. Ainda não
há versão estável: a mais recente é a `1.0.0-rc.3`, de 2026-10-09.

A análise do plugin nessa versão encontrou cinco atritos com o template:

1. **Arquivos que colidem.** O `.env.example` da raiz é um arquivo gerenciado
   pelo `init`, e o `infra/local/docker-compose.yml` é uma semente sem a lista
   `include:` que o `bounded-context` exige.
2. **Release incompatível.** O `bounded-context` grava `release.groups` no
   `nx.json`, e o Nx 23 recusa `release.projects` junto de `release.groups`.
   Além disso, grupo sem projeto é erro, inclusive em `--dry-run`, e o bloco
   `docker` do topo de `release` é herdado por todos os grupos.
3. **Credencial do registry.** Desde a 11.5.3, o pnpm ignora `${VAR}` em
   credencial do `.npmrc` do projeto (GHSA-3qhv-2rgh-x77r), e nenhum workflow
   do template autenticava o GitHub Packages antes do `pnpm install`.
4. **CI Go num workspace sem Go.** O reutilizável roda o gate de conformance
   sem condição, e o `conformance` reprova workspace sem módulo Go.
5. **Contexto que não nasce verde.** Depois do `bounded-context`, ainda faltam
   `tidy`, baseline de conformidade e um `.proto`.

## Decisão

**O plugin é a âncora única e versionada do DMPF no template.** O
`@mateusmacedo/dmpf-plugin` entra pelo `catalog:` do `pnpm-workspace.yaml`, na
`1.0.0-rc.3`, e o template versiona a saída de
`init --modulePrefix=example.com/change-me --npmScope=@mateusmacedo --composeProject=nx-base-template`.
Nenhum arquivo DMPF é escrito ou editado à mão. Kernel e plugin só se
atualizam juntos, por `nx migrate`, que atualiza o catálogo; o Renovate fica
desligado para o plugin, o `@nx-go/nx-go`, os módulos do platform e o workflow
reutilizável. O `.env.example` e o compose antigos dão lugar à saída do
`init`.

**GitHub Packages com credencial de usuário.** O `.npmrc` versionado só mapeia o
escopo `@mateusmacedo` para o registry. A credencial vive num npmrc de usuário:
`~/.npmrc` localmente; no CI, um arquivo em `$RUNNER_TEMP` apontado por
`NPM_CONFIG_USERCONFIG`, gravado pela composite `setup-node-pnpm` e pelo
`publish-libs.yaml` antes do install; no `Dockerfile.node.example`, dois
secrets do BuildKit.

**Ativos de IA pelo plugin.** Os agentes, comandos, rules e skills `dmpf-*`
chegam pelo `init` e se atualizam com ele; os ativos genéricos de `.claude/` e
`.agents/` continuam do template.

**Smoke em vez de app de exemplo.** O `tools/template-smoke.sh` copia o working
tree para um repositório descartável e repete ali a sequência do
`external-consumer.sh` do platform: recusa do prefixo reservado, `init`,
`bounded-context`, gates e `nx release --dry-run`. O job `template-smoke` roda o
script em todo PR.

**Release sem grupos no template.** O `nx.json` perde `release.projects` e o
bloco `docker`; o topo de `release` guarda só as opções comuns, como no
platform. Cada grupo nasce com o primeiro membro: `go-contract-<contexto>` pelo
`bounded-context`, `libs` e `apps` pelos passos de
`docs/nx-reference/tasks.md`. O publish npm e o release Docker filtram
`!tag:stack:go`.

**`dmpf-ci.yml` vermelho no template.** O chamador gerado pelo `init` não é
editado e fica fora dos checks obrigatórios. No template ele falha no gate de
conformance; num projeto derivado, passa a valer a partir do primeiro contexto.
O `ci.yml` exclui `tag:stack:go` do `nx affected`, porque os projetos Go são
cobertos pelo `dmpf-ci.yml`.

**Biome fora dos arquivos do DMPF.** `dmpf.json`, `dmpf.rendered.json`, `infra/`
e `tools/dmpf-baseline/` saem do Biome, para que o pre-commit nunca reescreva
um arquivo gerado e quebre o registro de hashes do `init` ou o
`infrasync --check`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Template sem a saída do `init`, que roda no primeiro uso | O template deixaria de provar que a versão fixada funciona |
| Instalar com `nx add` | Roda o `init` com os defaults (`@nx-base-template` e `source`), e o `init` seguinte não reescreve a semente do compose |
| App de exemplo versionada | Contraria a ADR-009 e vira código a manter |
| Grupo `libs` explícito no template | Fica vazio num derivado só com Go, e o `nx release` falha |
| Manter `release.projects` | Todo derivado fica com o `nx.json` inválido no primeiro contexto |
| Credencial no `.npmrc` do projeto | O pnpm ignora a variável e o arquivo versionado carregaria o segredo |
| Esperar a `1.0.0` estável | Não tem data; a `1.0.0-rc.3` é a única com o `workflowRef` e a diretiva Go atuais |
| Editar o `dmpf-ci.yml` para pular o gate sem módulo Go | Viola a regra de não editar arquivo DMPF; a correção pertence ao platform |

## Consequências

**Positivas:**

- O projeto derivado nasce com infra, gates, CI e ativos de IA da versão fixada
  e chega ao primeiro contexto em poucos comandos.
- Cada PR do template prova a versão fixada de ponta a ponta.
- O `nx migrate` mantém kernel, ferramentas e chamador de CI na mesma versão.

**Negativas:**

- O `dmpf-ci.yml` fica vermelho em todo PR do template até o platform aceitar
  workspace sem módulo Go.
- Toda instalação exige token com `read:packages`, inclusive o build Docker das
  apps JS.
- O template depende de uma release candidate, e cada versão recém-publicada do
  plugin entra no `minimumReleaseAgeExclude` do pnpm.
- Os manifestos de infra gerados herdam escolhas de desenvolvimento do platform
  (Redpanda e Loki sem autenticação, Prometheus com `--web.enable-lifecycle`,
  senha padrão do Postgres no `secretGenerator` do base do k8s), que só mudam no
  plugin; um projeto derivado precisa revê-las antes de qualquer ambiente
  compartilhado.
- A primeira lib TS ou app de um projeto derivado exige declarar o grupo de
  release.

## Referências

- SPEC-TXJ6A4XX — `docs/specs/SPEC-TXJ6A4XX-template-greenfield-dmpf.md`
- Guia de consumo do platform —
  `github.com/mateusmacedo/dmpf/blob/5def4d2d749bf8d5b2ef683f3cdc97eadf354c26/docs/guides/dmpf-consumo-externo.md`
- ADR-047 do platform (versão dos módulos Go pelo `package.json` privado) —
  `github.com/mateusmacedo/dmpf/blob/5def4d2d749bf8d5b2ef683f3cdc97eadf354c26/docs/adr/047-tags-de-modulo-go-e-consumo-fora-do-workspace.md`
- [ADR-009](./009-remocao-libs-exemplo.md) e [ADR-010](./010-migracao-github-linear.md)
