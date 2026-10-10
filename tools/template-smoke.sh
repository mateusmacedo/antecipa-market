#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
PLUGIN=@mateusmacedo/dmpf-plugin
KERNEL=github.com/mateusmacedo/dmpf
PREFIX=github.com/example/template-smoke
CONTEXT=shop
WORK=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --work)
      [[ $# -ge 2 ]] || { echo "--work exige um diretório" >&2; exit 2; }
      WORK="$2"; shift 2 ;;
    *) echo "argumento desconhecido: $1" >&2; exit 2 ;;
  esac
done
WORK="${WORK:-$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/template-smoke.XXXXXX")}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
case "$WORK/" in "$ROOT"/*) echo "--work precisa ficar fora do repositório" >&2; exit 2 ;; *) ;; esac
[[ -z "$(ls -A "$WORK")" ]] || { echo "$WORK precisa estar vazio" >&2; exit 2; }

passo() { printf '\n== %s\n' "$*"; }
falha() { echo "template-smoke: $*" >&2; exit 1; }
versao_instalada() { jq -r ".$1" "node_modules/$PLUGIN/versions.json"; }
kernel_ddl() {
  local postgres="$KERNEL/libs/backend/go/postgres"
  if go list -m "$postgres" > /dev/null 2>&1; then
    go mod download -json "$postgres" | jq -r .Dir
  else
    echo none
  fi
}
copiar_working_tree() {
  local arquivo
  while IFS= read -r -d '' arquivo; do
    [[ -e "$ROOT/$arquivo" || -L "$ROOT/$arquivo" ]] || continue
    mkdir -p "$WORK/$(dirname "$arquivo")"
    cp -pP "$ROOT/$arquivo" "$WORK/$arquivo"
  done < <(git -C "$ROOT" ls-files -z -co --exclude-standard)
}

passo "cópia do working tree em $WORK"
copiar_working_tree
cd "$WORK"
# Com CI definido, o tb.Env do kernel (libs/backend/go/testkit/tb/tb.go do platform)
# reprova os testes que dependem de infra em vez de pulá-los, e o smoke não sobe infra.
unset CI
export NX_DAEMON=false NX_NO_CLOUD=true NX_TUI=false
git init -q
git add -A
git -c user.name=template-smoke -c user.email=template-smoke@example.com \
  -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m "chore: template-smoke"
pnpm install --frozen-lockfile

passo "bounded-context recusa o prefixo reservado"
antes="$(git status --porcelain=v1 -uall)"
if saida="$(pnpm nx g "$PLUGIN:bounded-context" "$CONTEXT" --boundedContext="$CONTEXT" --no-interactive 2>&1)"; then
  falha "bounded-context aceitou o prefixo reservado"
fi
grep -q 'reserved modulePrefix' <<< "$saida" || { echo "$saida" >&2; falha "bounded-context falhou por outro motivo"; }
[[ "$(git status --porcelain=v1 -uall)" == "$antes" ]] || falha "a recusa do bounded-context escreveu arquivos"

passo "init com o prefixo $PREFIX"
pnpm nx g "$PLUGIN:init" --modulePrefix="$PREFIX" --npmScope=@mateusmacedo --composeProject=template-smoke --no-interactive
jq -e --arg p "$PREFIX" '.modulePrefix == $p' dmpf.json > /dev/null || falha "dmpf.json sem o prefixo $PREFIX"

passo "bounded-context $CONTEXT"
pnpm nx g "$PLUGIN:bounded-context" "$CONTEXT" --boundedContext="$CONTEXT" --no-interactive
pnpm nx run-many -t tidy
CONFORMANCE="$(versao_instalada conformance)"
KERNEL_VERSION="$(versao_instalada kernel)"
go run "$KERNEL/tools/dmpf-conformance/cmd/conformance@$CONFORMANCE" --root . --write-baseline
# O generator deixa o .proto para o autor do contexto, e contrato vazio reprova no buf
# (mesmo passo de tools/external-consumer.sh do platform).
proto="apps/backend/$CONTEXT/contract/proto/company/$CONTEXT/service/v1"
mkdir -p "$proto"
printf '%s\n' 'syntax = "proto3";' '' "package company.$CONTEXT.service.v1;" '' \
  'service ShopService {' '  rpc Ping(PingRequest) returns (PingResponse);' '}' '' \
  'message PingRequest {}' '' 'message PingResponse {}' > "$proto/${CONTEXT}_service.proto"

passo "gates (kernel $KERNEL_VERSION, conformance $CONFORMANCE)"
go list -m -f '{{.Path}}/...' | xargs go build
go list -m -f '{{.Path}}/...' | xargs go vet
go list -m -f '{{.Path}}/...' | xargs go test
while read -r dir; do
  (cd "$dir" && go mod edit -json) | jq -e --arg k "$KERNEL/" --arg v "$KERNEL_VERSION" \
    '([.Replace[]? | select(.Old.Path | startswith($k))] | length == 0)
     and all(.Require[]? | select(.Path | startswith($k)); .Version == $v)' > /dev/null \
    || falha "$dir: kernel fora de $KERNEL_VERSION ou com replace"
done < <(go list -m -f '{{.Dir}}')
go run "$KERNEL/tools/dmpf-conformance/cmd/conformance@$CONFORMANCE" --root .
go run "$KERNEL/tools/dmpf-conformance/cmd/modsync@$CONFORMANCE" --root . --check
go run "$KERNEL/tools/dmpf-conformance/cmd/infrasync@$CONFORMANCE" --root . --check
ddl="$(kernel_ddl)"
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" DMPF_KERNEL_DDL="$ddl" \
  bash "node_modules/$PLUGIN/scripts/dmpf-context-check.sh" --phase structural
pnpm nx run-many -t buf-lint
pnpm nx release --dry-run

passo "template conforme ($WORK)"
