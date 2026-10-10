# ADR-010: Migrar a manutenção para o GitHub e o Linear

## Status

Aceito — 2026-10-09. Implementa SPEC-317RXZ73 (DEVS-72).

Supersede o [ADR-005](./005-plataforma-gitea.md) por inteiro e, do
[ADR-004](./004-workflows-verdaccio-release.md), o alvo de publicação e o
runner; o split entre versionamento e publicação segue válido. Supersede do
[ADR-008](./008-identidade-automacao-release.md) a identidade do bot e a forma
do push; a remoção das proteções que bloqueavam o push do release segue válida.

## Contexto

O template nasceu hospedado no Gitea de uma organização contratante e herdou
dela a cadeia inteira de infraestrutura: o host e a API `/api/v1`, o runner
`gitea-runner`, o Verdaccio com credencial obtida por OIDC numa role da AWS, os
workflows reutilizáveis de um repositório `actions-templates` e o Jira como
rastreador. Encerrado o vínculo, a manutenção passa a `mateusmacedo` e nada
disso continua acessível.

Quatro consequências práticas:

1. **Os workflows não rodam.** Três jobs pedem um runner autogerido que não
   existe no GitHub, e o `create-release.yml` chama a API do Gitea.
2. **Os reutilizáveis somem.** Um `uses: <org>/actions-templates/...@main` falha
   no momento do run, não no push, quando o repositório deixa de existir.
3. **A publicação perde o registry e a credencial.** Registry, role e parâmetro
   pertencem à contratante.
4. **O gatilho da publicação deixa de disparar.** No GitHub, uma tag empurrada
   com o `GITHUB_TOKEN` não dispara outro workflow; o `push: tags` do
   `nx-publish-libs.yml` nunca rodaria depois do `nx-release.yml`.

O repositório `dmpf`, derivado deste template, já fez a mesma migração (ADR-043
dele) e roda o fluxo resultante no GitHub com o mesmo Nx e o mesmo pnpm.

## Decisão

**Plataforma: GitHub**, em `github.com/mateusmacedo/dmpf-nx-template`, marcado
como repositório template. O `gh` volta a ser o caminho padrão de automação.
Os jobs rodam em `ubuntu-26.04`, o mesmo label do `dmpf`.

**Release sem PR do bot.** O `create-release.yml` cria e empurra
`release/X.Y.Z` e emite um aviso para o mantenedor abrir o PR com a própria
conta: um PR aberto pelo `GITHUB_TOKEN` não dispara o CI. O ciclo anterior é
lido do assunto dos merges em `master`, porque o GitHub apaga a branch mergeada.

**Publicação: GitHub Packages, encadeada ao release.** O `nx-release.yml`
versiona, empurra de forma atômica o commit e só as tags criadas no run, e
chama o reutilizável `publish-libs.yaml` num job `publish` com `needs`. A
autenticação é o `GITHUB_TOKEN` com `packages: write`; o escopo npm passa a
`@mateusmacedo`, porque o GitHub Packages exige que o escopo seja o owner do
repositório. O `nx-publish-libs.yml` vira republicação manual. A identidade do
commit de versão passa a `github-actions[bot]`.

**Reutilizáveis internalizados.** `detect-apps.yaml` e `publish-libs.yaml`
passam a viver em `.github/workflows/`, portados do `dmpf`. O `deploy-eks.yaml`
não é trazido: o original depende de roles, registry e repositório de
infraestrutura da contratante, e generalizá-lo sem um alvo real seria código
não testado. O exemplo comentado do `cd-dev-hmg.yml` aponta para o caminho
local a criar quando o CD for ligado.

**Proteção por rulesets**, como no `dmpf`: `develop` exige PR e o check do CI,
sem deleção e sem force push; `master` só bloqueia deleção e force push, para
que o bot de release empurre o commit de versão e as tags.

**Rastreador: Linear**, projeto `nx-base-template` do time DevTeam (chave
`DEVS`). Cada spec tem uma issue, ligada pelo `ticket_url`; branch e título de
PR levam `DEVS-<n>`.

**Sem identificação da contratante.** Os registros históricos (ADRs e specs
concluídas) mantêm as decisões e os IDs `ARQ-<n>`, mas trocam o nome da
contratante e dos sistemas dela por termos neutros ("contratante", "monorepo de
origem", `dmpf-legacy`) e perdem as URLs do tracker dela e os caminhos de
máquina. Duas specs mudam de nome de arquivo pelo mesmo motivo.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Criar `mateusmacedo/actions-templates` e manter os `uses:` externos por `@main` | Reintroduz uma dependência que não se versiona com o template; projetos derivados herdariam um link para outro repositório |
| Manter o gatilho `push: tags` na publicação | Tag empurrada pelo `GITHUB_TOKEN` não dispara workflow; exigiria um PAT dedicado só para isso |
| Abrir o PR de release pelo bot com `gh pr create` | O PR nasce sem CI, pelo mesmo motivo; o check obrigatório da `develop` nunca ficaria verde |
| Internalizar o `deploy-eks.yaml` generalizado | Sem alvo real, seria código não exercitado; entra quando o CD for ligado |
| `ubuntu-latest` | O label muda de imagem sem aviso; o `ubuntu-26.04` é o que o `dmpf` já valida |
| Reescrever os ADRs da era Gitea no lugar de supersedê-los | Um ADR registra a decisão daquele momento; reescrevê-lo apaga o histórico que ele existe para preservar |
| Reescrever a autoria dos commits | Muda todos os SHAs; o `dmpf` aceitou o mesmo custo de manter o e-mail antigo no histórico |

## Consequências

**Positivas:**

- Nenhuma dependência de infraestrutura de terceiro permanece na esteira:
  host, registry, runner, reutilizáveis e credenciais são do próprio
  repositório ou do GitHub.
- A publicação depende só do token que o run já tem.
- Projetos derivados herdam workflows completos, sem link para outro
  repositório.
- O trabalho fica rastreado num projeto Linear próprio, separado do DMPF.

**Negativas:**

- O PR de release passa a ser aberto à mão pelo mantenedor.
- Projetos derivados precisam trocar o escopo `@mateusmacedo` pelo owner deles
  antes da primeira publicação.
- O `cd-dev-hmg.yml` continua sem o reutilizável de deploy; ligar o CD exige
  criá-lo.
- O histórico de commits mantém o e-mail corporativo antigo do autor.

## Referências

- SPEC-317RXZ73 — `docs/specs/SPEC-317RXZ73-migrar-mantenedor-github-linear.md`
- ADR-043 do `dmpf` — `github.com/mateusmacedo/dmpf/blob/develop/docs/adr/043-migracao-para-github-licenca-e-autoria.md`
