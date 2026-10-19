#!/usr/bin/env bash
# Renderiza os overlays de plataforma dev e hmg e confere o isolamento do papel
# session-events (ADR-063) sobre o YAML renderizado. Roda da raiz do workspace.
set -euo pipefail

failures=0

fail() {
  printf 'k8s-render: %s\n' "$1" >&2
  failures=$((failures + 1))
}

check() {
  local overlay=$1 rendered=$2 description=$3 filter=$4
  if [ "$(jq -r "$filter" <<<"$rendered")" != true ]; then
    fail "$overlay: $description"
  fi
}

for overlay in dev hmg; do
  rendered="$(kubectl kustomize "infra/k8s/overlays/$overlay" | yq ea -o=json '[.]')"

  check "$overlay" "$rendered" 'o Deployment bff não pode referenciar o Secret keycloak-session-events' '
    [.[] | select(.kind == "Deployment" and .metadata.name == "bff")] as $api
    | ($api | length) == 1
      and ($api | tostring | contains("keycloak-session-events") | not)'

  check "$overlay" "$rendered" 'só o container session-events lê keycloak-session-events/secret, sem optional' '
    [.[] | select(.kind == "Deployment" and .metadata.name == "bff-session-events")] as $role
    | ($role | length) == 1
      and ([$role[0].spec.template.spec.containers[]
            | select(tostring | contains("keycloak-session-events")) | .name] == ["session-events"])
      and ([$role[0].spec.template.spec.containers[].env[]?
            | select(.valueFrom.secretKeyRef.name == "keycloak-session-events")
            | .valueFrom.secretKeyRef | (.key == "secret" and (.optional // false) == false)] == [true])'

  for deployment in bff bff-session-events; do
    check "$overlay" "$rendered" "o Deployment $deployment lê REDIS_URL do Secret redis/url, sem optional" '
      [.[] | select(.kind == "Deployment" and .metadata.name == "'"$deployment"'")
       | .spec.template.spec.containers[].env[]? | select(.name == "REDIS_URL")] as $redis
      | ($redis | length) == 1
        and ($redis[0].valueFrom.secretKeyRef | (.name == "redis" and .key == "url" and (.optional // false) == false))'
  done

  check "$overlay" "$rendered" 'o container session-events expõe backchannel 8091 e admin 8092' '
    [.[] | select(.kind == "Deployment" and .metadata.name == "bff-session-events")
     | .spec.template.spec.containers[] | select(.name == "session-events") | .ports] as $ports
    | ($ports | length) == 1
      and ($ports[0] | map({(.name): .containerPort}) | add) == {"backchannel": 8091, "admin": 8092}'

  check "$overlay" "$rendered" 'o Service bff-session-events aponta a targetPort do back-channel' '
    [.[] | select(.kind == "Service" and .metadata.name == "bff-session-events") | .spec.ports[].targetPort]
    == ["backchannel"]'

  check "$overlay" "$rendered" 'o ConfigMap keycloak aponta o back-channel logout para o bff-session-events' '
    [.[] | select(.kind == "ConfigMap" and .metadata.name == "keycloak") | .data.DMPF_BACKCHANNEL_LOGOUT_URL]
    == ["http://bff-session-events:8091/backchannel-logout"]'

  check "$overlay" "$rendered" 'todo container do bff declara GOMEMLIMIT entre 70% e 100% do limit de memória' '
    def bytes: if test("^[0-9]+Mi$") then (.[:-2] | tonumber) * 1048576
      elif test("^[0-9]+Gi$") then (.[:-2] | tonumber) * 1073741824
      elif test("^[0-9]+MiB$") then (.[:-3] | tonumber) * 1048576
      elif test("^[0-9]+GiB$") then (.[:-3] | tonumber) * 1073741824
      else null end;
    [.[] | select(.kind == "Deployment" and (.metadata.name == "bff" or .metadata.name == "bff-session-events"))
     | .spec.template.spec.containers[]
     | {soft: ([.env[]? | select(.name == "GOMEMLIMIT") | .value | bytes] | first), hard: (.resources.limits.memory // "" | bytes)}] as $memory
    | ($memory | length) == 2 and all($memory[]; .soft != null and .hard != null and .soft < .hard and .soft >= .hard * 0.7)'

  check "$overlay" "$rendered" 'o Service bff-session-events expõe só a porta 8091' '
    [.[] | select(.kind == "Service" and .metadata.name == "bff-session-events")] as $svc
    | ($svc | length) == 1 and ([$svc[0].spec.ports[].port] == [8091])'

  check "$overlay" "$rendered" 'a NetworkPolicy bff-session-events admite só pods keycloak na porta backchannel' '
    [.[] | select(.kind == "NetworkPolicy" and .metadata.name == "bff-session-events")] as $np
    | ($np | length) == 1
      and ($np[0].spec.policyTypes == ["Ingress"])
      and ($np[0].spec.ingress == [{
            "from": [{"podSelector": {"matchExpressions": [
              {"key": "app.kubernetes.io/name", "operator": "In", "values": ["keycloak"]}]}}],
            "ports": [{"port": "backchannel", "protocol": "TCP"}]}])'
done

templates="$(yq ea -o=json '[.]' infra/k8s/overlays/hmg/secrets.example.yaml.tmpl apps/backend/bff/deploy/k8s/overlays/hmg/secrets.example.yaml.tmpl)"
check hmg "$templates" 'os templates hmg precisam declarar keycloak-session-events/secret e redis/url' '
  ([.[] | select(.kind == "Secret" and .metadata.name == "keycloak-session-events") | .stringData | has("secret")] == [true])
  and ([.[] | select(.kind == "Secret" and .metadata.name == "redis") | .stringData | has("url")] == [true])'

if [ "$failures" -gt 0 ]; then
  exit 1
fi
echo 'k8s-render: dev e hmg renderizados; isolamento do session-events conferido'
