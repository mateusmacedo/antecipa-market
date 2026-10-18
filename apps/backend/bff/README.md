# bff

Borda REST/JSON pública do workspace, portada do `bff` de referência do kernel DMPF `1.0.0-rc.5` (ADR-044 do kernel). Sem banco, outbox nem consumer: autentica o sujeito, recusa o token de uma sessão revogada consultando a deny-list guardada no Redis de `REDIS_URL`, autoriza por permissão e traduz cada rota do contrato OpenAPI em uma chamada gRPC a um bounded context (`RST-01`, `GRP-01`, `IDN-16`). Especificado pela `docs/specs/SPEC-A62BGCD4-bff-borda-rest.md`.

Nesta versão (DEVS-111) a borda ainda não monta rota de negócio: o processo sobe a cadeia HTTP completa, a autenticação, a saúde e o OpenAPI do `identity` servido. As 21 rotas do `identity` chegam na DEVS-112 (consultas) e na DEVS-113 (comandos).

## Saúde

Fora do contrato e sem autenticação, numa porta de administração própria (`ADMIN_ADDR`, default `:8090`). A porta pública não serve as duas rotas e responde `404` a elas.

| Rota | Responde |
| --- | --- |
| `GET /livez` | `204` enquanto o processo serve HTTP |
| `GET /readyz` | `204`; `503` a partir do `SIGTERM`, durante a drenagem. Não depende do Redis; a dependência da conexão gRPC com o `identity` entra na DEVS-112 |

O subcomando `bff healthcheck` consulta o `/readyz` no endereço de `ADMIN_ADDR` e sai com `0` ou `1`. `GET` e `HEAD` de `/livez` e `/readyz` não abrem span SERVER nem ponto de `http.server.request.duration`; o registro de acesso delas sai no máximo uma vez a cada 5 min por rota quando a resposta é 2xx, e sempre fora de 2xx.

## Unidade do manifesto

`bff/app`, bloco `app`, `bounded_context` `bff`, com os pacotes `app`, `app/api`, `app/rpc` e `cmd`. O `bff` alcança só o shared kernel (`http`, `grpc`, `transport`, `observability`, `authn`, `redis`, `ports`) e, a partir da DEVS-112, o contrato gerado do `identity`. Importar domínio ou aplicação de um contexto reprova no verificador com `DMPF-D002`.

## Cadeia de uma requisição

1. O `otelhttp` abre um span SERVER por requisição, com pai no `traceparent` recebido; o `tracestate` do cliente é descartado na extração. O registro de acesso fica logo abaixo e envolve o CORS e o `ServeMux`: toda resposta com span SERVER sai com exatamente um `http request`.
2. Em cada rota montada: `withRecover` (pânico vira 500 `internal-failure`), `withRouteDeadline` (prazo da requisição a partir do `Budget` da rota, `GRP-05`), autenticação e montagem do contexto de execução (`CTX-01`), admissão por rota e tenant (429 `admission-refused` antes de ler o corpo, `RES-16`), exigência de `Idempotency-Key` e o handler.
3. `X-Correlation-ID` válido é preservado, senão cunhado, e volta no mesmo header (`CTX-07`).
4. `ResolveIdentity`: sem credencial ou com credencial inválida → 401 `unauthenticated`; tenant não resolvido → 403 `tenant-unresolved`; sem a permissão → 403 `permission-denied`. `RefuseAssertedIdentity` recusa com 403 `identity-mismatch` quando `X-Subject-ID`, `X-Tenant-ID` ou o query `tenant_id` divergem do token (`CTX-06`).
5. `Idempotency-Key` obrigatório nos `POST`, no formato `^[A-Za-z0-9._-]{1,128}$`: sem ele, 400 `missing-idempotency-key`; fora do formato, 400 `invalid-idempotency-key`. O log de acesso registra a chave do cliente e a derivada `hex(sha256(sujeito ‖ 0x00 ‖ chave))`.
6. Corpo de até 64 KiB, um único objeto JSON e sem campo desconhecido (`TRP-49`, `TRP-51`).

`ValidateEdge` recusa a partida com rota sem `Permission` ou `POST` sem chave de idempotência declarada (`IDN-16`, `RST-02`).

## Autenticação

O `bff` resolve identidade pelo pacote `authn` do kernel: o verificador OIDC (`OIDC_*`) ou o mock de desenvolvimento (`AUTH_DEV_MOCK`), nunca os dois. Com o verificador, cada token aceito é consultado na deny-list guardada no Redis de `REDIS_URL`; sessão ou sujeito revogado vira 401, e o Redis fora ou lento além de `OIDC_REVOCATION_TIMEOUT_MS` recusa o token (falha fechada). `REDIS_URL` é obrigatório inclusive com `AUTH_DEV_MOCK`, que não consulta a deny-list. Um `SIGTERM` durante a descoberta do issuer encerra a partida com exit 0. O registro `process configured` traz `authn.revocation=enabled` e as chaves `redis.*` sem a senha.

Com o realm de referência, o tenant vem da claim nativa `organization`, sem mapper que a achate:

| Variável | Valor |
| --- | --- |
| `OIDC_ISSUER` | `http://keycloak.localhost:8180/realms/dmpf` (local) |
| `OIDC_AUDIENCE` | `dmpf-bff` |
| `OIDC_TENANT_CLAIM` | `organization.*.tenant_id` |

## Configuração

| Variável | Obrigatória | Efeito |
| --- | --- | --- |
| `IDENTITY_GRPC_TARGET` | sim | Alvo gRPC do `identity` (`dns:///host:porta`) |
| `GRPC_INSECURE` | uma das duas | `true` só em desenvolvimento |
| `GRPC_CA_FILE`, `GRPC_SERVER_NAME` | uma das duas | CA que valida o `identity` e nome esperado no certificado |
| `GRPC_CLIENT_CERT_FILE`, `GRPC_CLIENT_KEY_FILE` | com `GRPC_CA_FILE` | Certificado de cliente do `bff` (`spiffe://dmpf/bff`); carregado na partida, que falha com material ilegível |
| `OIDC_ISSUER`, `OIDC_AUDIENCE`, `OIDC_TENANT_CLAIM` | uma das duas linhas de autenticação | Emissor, audiência e claim de tenant do access token |
| `AUTH_DEV_MOCK` | uma das duas linhas de autenticação | `true` lê a identidade declarada no próprio Bearer, sem verificação; não pode coexistir com `OIDC_ISSUER` |
| `REDIS_URL` | sim | Redis compartilhado em que o `bff` consulta a deny-list, banco 0 (`redis://<usuário>:<senha>@host:porta/0`); ausente, inválido ou com outro banco, a partida falha com exit 2 sem ecoar a URL |
| `REDIS_DIAL_TIMEOUT_MS`, `REDIS_COMMAND_TIMEOUT_MS`, `REDIS_REQUIRE_TLS` | não | Prazos do cliente Redis e exigência de `rediss://` |
| `OIDC_REVOCATION_TIMEOUT_MS` | não | Prazo da consulta à deny-list |
| `OPENAPI_IDENTITY_PATH` | não | Serve o contrato do `identity` em `/openapi/identity/v1/openapi.yaml`, com `ETag`, `304` a `If-None-Match` e gzip quando o cliente o aceita; arquivo ilegível ou vazio recusa a partida |
| `HTTP_ADDR`, `ADMIN_ADDR` | não | Defaults `:8080` e `:8090`; fora de `host:porta`, ou na mesma porta com host igual ou curinga, a partida falha com exit 2. A comparação normaliza a porta (nome de serviço, zero à esquerda) e o IP literal, mas não resolve nome de host |
| `DRAIN_DELAY` | não | Default `0`, máximo `10s` (o `terminationGracePeriodSeconds` de 30 s menos as duas janelas de 10 s do desligamento, a do HTTP e a da telemetria); no `SIGTERM`, o `/readyz` passa a `503` e o listener público só fecha depois desse tempo. Um segundo `SIGTERM` ou `SIGINT` encerra o processo na hora, sem o desligamento gracioso |
| `CORS_ORIGINS` | não | Origens aceitas pelo navegador; vazio por padrão |
| `METRIC_TENANTS` | não | Tenants com balde de admissão e rótulo de métrica próprios (`MET-07`) |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_*` | ver `deploy/.env.example` | Identidade do recurso e exportação OTLP |

Variável obrigatória ausente, ou nenhuma política de transporte gRPC ou de autenticação, encerra a partida com exit 2 nomeando o que falta.

## Rodar localmente

```bash
pnpm nx run bff:deploy-env
set -a; . apps/backend/bff/deploy/.env; set +a
pnpm nx run bff:serve
```

O `deploy-env` copia o `deploy/.env.example` para `deploy/.env` quando a cópia não existe. O `serve` depende do `redis-up`, que sobe só o Redis do profile `redis` do Compose local, e roda o binário no host com HTTP em 8080 e administração em 8090. Para conferir:

```bash
curl -i localhost:8080/openapi/identity/v1/openapi.yaml
curl -i localhost:8090/readyz
```

## Targets Nx

`tidy`, `fmt-check`, `vet`, `build`, `test-race`, `govulncheck`, `serve`, `redis-up`, `test-infra-up`, `test-infra-down` e `deploy-env`; `lint` e `test` são inferidos pelo plugin.

## Testes

Unitários sobre a cadeia real, com rotas sintéticas montadas por uma costura de teste (`app/api/export_test.go`), sem nenhuma rota de negócio em produção: ordem da cadeia, `ValidateEdge`, CORS, 401 (inclusive com o verificador OIDC contra um issuer falso), 403, 429, pânico, prazo, access log, span, idempotência, saúde, configuração, telemetria e partida do binário. O teste estrutural do OpenAPI compara os status por tipo de rota (comando, consulta por `POST`, consulta por id, listagem) e prova o verificador com rotas sintéticas sobre operações reais do contrato do `identity`.

```bash
pnpm nx run bff:test-race
```

## O que fica para as próximas tasks

- DEVS-112: decorador de permissões efetivas, dial gRPC e `ClientOptions`, `/readyz` dependente do `identity`, as 10 rotas de consulta e o teste HTTP de revogação.
- DEVS-113: as 11 rotas de comando e o replay de idempotência.
- DEVS-114: papel `session-events`, Dockerfile, `deploy/compose.yml`, `deploy/k8s`, `edge: bff` no `dmpf.json` e o e2e em container.

## Limitações até a DEVS-114

- Os agregados `infra/k8s/overlays/{dev,hmg}/apps`, gerados pelo `infrasync`, já referenciam `apps/backend/bff/deploy/k8s/overlays/{dev,hmg}`, que só nascem na DEVS-114; até lá esses agregados não renderizam.
- O certificado de cliente `bff-client` é emitido pelo serviço `pki` do Compose local; num ambiente em que o volume `pki` já existia antes, é preciso recriá-lo para que o certificado seja gerado.
