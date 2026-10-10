# Primeiro bounded context

Este guia cobre só o que é próprio de um projeto criado a partir do template. A
referência completa de consumo do DMPF é o guia do platform na versão fixada pelo
plugin:
[`docs/guides/dmpf-consumo-externo.md`](https://github.com/mateusmacedo/dmpf/blob/5def4d2d749bf8d5b2ef683f3cdc97eadf354c26/docs/guides/dmpf-consumo-externo.md)
(commit `5def4d2`, o `workflowRef` do `@mateusmacedo/dmpf-plugin` `1.0.0-rc.3`).

## Antes de começar

- Credencial do GitHub Packages no `~/.npmrc` e `NODE_AUTH_TOKEN` com `read:packages`
  (ver "Pré-requisitos" no `README.md`).
- Go na diretiva do `go.work`, mais `bash`, `jq` e `git`.
- `pnpm install` já rodado.

## 1. Configurar o projeto

O template sai com o module path reservado `example.com/change-me`, e o
`bounded-context` recusa rodar enquanto ele estiver no `dmpf.json`.

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=github.com/<org>/<repo> \
  --composeProject=<repo> --no-interactive
```

O `init` reescreve os arquivos gerenciados que você não editou, mas não toca a semente
`infra/local/docker-compose.yml`. Troque nela o `name: nx-base-template-local` por
`<repo>-local`.

## 2. Gerar o contexto

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <contexto> --boundedContext=<contexto>
pnpm nx run-many -t tidy
CONFORMANCE="$(jq -r .conformance node_modules/@mateusmacedo/dmpf-plugin/versions.json)"
go run "github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@$CONFORMANCE" \
  --root . --write-baseline
```

O generator não escreve o `.proto` do contrato, e contrato vazio reprova no buf. Um
serviço mínimo, em
`apps/backend/<contexto>/contract/proto/company/<contexto>/service/v1/<contexto>_service.proto`:

```proto
syntax = "proto3";

package company.<contexto>.service.v1;

service <Contexto>Service {
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {}

message PingResponse {}
```

## 3. Rodar os gates

A mesma sequência do `tools/template-smoke.sh`:

```bash
go list -m -f '{{.Path}}/...' | xargs go build
go list -m -f '{{.Path}}/...' | xargs go vet
go list -m -f '{{.Path}}/...' | xargs go test

KIT="github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd"
go run "$KIT/conformance@$CONFORMANCE" --root .
go run "$KIT/modsync@$CONFORMANCE" --root . --check
go run "$KIT/infrasync@$CONFORMANCE" --root . --check

DDL="$(go mod download -json github.com/mateusmacedo/dmpf/libs/backend/go/postgres | jq -r .Dir)"
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" DMPF_KERNEL_DDL="$DDL" \
  bash node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh --phase structural
pnpm nx run-many -t buf-lint
```

O contexto gerado já tem o bloco `provider`, que depende do módulo Postgres do kernel; o
context-check lê o DDL desse módulo no cache de módulos do Go.

No PR, o `ci.yml` cobre as stacks JS e o `template-smoke`; o `dmpf-ci.yml` cobre os
projetos Go e passa a valer a partir deste primeiro contexto.

## 4. Release

O `bounded-context` grava no `nx.json` o grupo `go-contract-<contexto>`, com a tag
`apps/backend/<contexto>/contract/v{version}`. Libs TS e apps com imagem Docker precisam
do próprio grupo quando o primeiro membro nascer: ver "Release por grupos" em
[`docs/nx-reference/tasks.md`](../nx-reference/tasks.md).

## 5. Atualizar o DMPF

Kernel e plugin sobem juntos, só pelo `nx migrate`; o Renovate não mexe neles.

```bash
pnpm nx migrate @mateusmacedo/dmpf-plugin@<nova-versão>
pnpm install
pnpm nx migrate --run-migrations
```

O `nx migrate` atualiza a versão no `catalog:` do `pnpm-workspace.yaml`. Versão publicada
há menos de um dia entra no `minimumReleaseAgeExclude` do pnpm, que o próprio `pnpm install`
acrescenta.
