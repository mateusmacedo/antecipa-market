---
id: SPEC-A62BGCD4
slug: bff-borda-rest
title: bff - borda REST pública sobre os contextos gRPC (fundação e identity)
stage: backlog
priority: P0
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6, SPEC-SBW7HT6Y, SPEC-STEJG347]
ticket_url: https://linear.app/mmda/issue/DEVS-110
subtask_urls: [https://linear.app/mmda/issue/DEVS-111, https://linear.app/mmda/issue/DEVS-112, https://linear.app/mmda/issue/DEVS-113, https://linear.app/mmda/issue/DEVS-114]
created: 2026-10-15
---

# SPEC-A62BGCD4: bff - borda REST pública sobre os contextos gRPC (fundação e identity)

## Resumo

O workspace não tem borda pública. O `identity` serve só gRPC, e o contrato REST
dele (`apps/backend/identity/contract/openapi/v1/openapi.yaml`, 22 operações) não
é servido por ninguém. Esta spec cria o app `bff` em `apps/backend/bff`, porte do
`bff` de referência do kernel DMPF (`1.0.0-rc.4`, `workflowRef` `487e9de`). O app
traz a cadeia completa da borda (OIDC, permissões efetivas resolvidas no
`identity` a cada requisição, tenant, idempotência, erros, admissão, CORS,
OpenAPI servido e mTLS de cliente) e 21 das 22 rotas do `identity`. Como usuário
autenticado de uma organização cliente, quero chamar o `identity` por HTTPS com o
meu token, para operar cadastro, vínculos e dados bancários sem acesso direto ao
gRPC interno.

## Contexto

- **Problema**: a ADR-012 põe todo REST externo no `bff` e os contextos em gRPC,
  mas o `bff` não existe (`apps/backend/identity/README.md:36-38`). Com
  `"edge": ""` em `dmpf.json:6`, a NetworkPolicy do `identity` tem `ingress: []`
  (`apps/backend/identity/deploy/k8s/base/networkpolicy.yaml`), o overlay hmg
  tem `GRPC_TRUSTED_CLIENTS: ''`, e `infra/local/compose/pki.yml` só emite o
  certificado de servidor `dmpf-identity-api`, sem cliente para a borda. Em
  container, o `identity` fica inalcançável.
- **Impacto**: abre o primeiro caminho externo verificável de ponta a ponta
  (token, borda, gRPC, contexto) e destrava o Swagger UI local, que já aponta
  para `http://localhost:8080/openapi/identity/v1/openapi.yaml`
  (`infra/local/compose/swagger-ui.yml`). Cada contexto futuro acrescenta as
  próprias rotas à borda, sem refazer a fundação.
- **Inspiração**: o `bff` de referência do kernel, em
  `lidercap-platform/apps/backend/bff` (repositório `mateusmacedo/dmpf`, sem
  mudanças em `apps/backend/bff` nem em `docs/adr` entre `487e9de` e o HEAD
  local). Decisão de origem: ADR-044 do kernel (bff REST e contextos gRPC de
  referência).
- **Links relevantes**:
  - [ADR-012](../adr/012-um-app-por-bounded-context.md): um app por contexto,
    REST só no `bff`.
  - [ADR-016](../adr/016-provedor-de-identidade-oidc.md): Keycloak, mapper de
    tenant, deny-list de sessão e origem das permissões.
  - [ADR-022](../adr/022-precedencia-dmpf-sobre-design.md): o kernel prevalece
    sobre `docs/design/`; extensão do kernel nasce como spec no kernel.
  - SPEC-7KW0HHF6 (identity-organization): `FindEffectivePermissions` chamada
    pelo `bff` a cada requisição, sem cache (:59, :413, :452, :590).
  - `docs/design/91-contratos-api.md`: superfícies Client API, Partner API e
    webhooks (requisito de produto; divergências reconciliadas abaixo).
- **Dependências do kernel (bloqueantes)**: SPEC-SBW7HT6Y (DEVS-97, Keycloak de
  referência, mapper `organization` para `tenant_id` e eventos de sessão) e
  SPEC-STEJG347 (DEVS-101, deny-list por `sub`/`sid` no `authn`), ambas em
  `backlog` no repositório do kernel. Decisão do discovery (2026-10-15): esta
  spec só entra em `building` quando as duas estiverem `done` e publicadas numa
  versão do kernel consumida por este workspace (ADR-011).
- **Referências externas**:
  - `lidercap-platform/apps/backend/bff/app/api/routes.go`: `bindings()`,
    `surface.command`/`surface.query`, `NewHandler` e ordem da cadeia
    (:156-179).
  - `lidercap-platform/apps/backend/bff/app/rpc/errors.go`, `metadata.go`,
    `clients.go`: mapeamento de erros, metadata gRPC e política por método.
  - `lidercap-platform/libs/backend/go/http/identity.go:29-51`:
    `ResolveIdentity` confere `route.Permission` contra
    `ports.Identity.Permissions`.
  - `lidercap-platform/libs/backend/go/ports/authn.go:25-48`: `Identity`
    (`Subject`, `Permissions`) e interface `Authenticator`.

<constraints>
- [P0] O `bff` não importa domínio nem aplicação de contexto: só módulos
  `contract` dos contextos e bibliotecas do kernel (ADR-012; ADR-044 do kernel;
  regra `DMPF-D002`).
- [P0] Uma rota encaminha para exatamente uma chamada gRPC. Sem orquestração,
  sem saga, sem banco.
- [P0] Toda rota declara `Permission` e `ContractRef`; `ValidateEdge` recusa na
  partida rota sem permissão ou `POST` sem exigência de `Idempotency-Key`
  (IDN-16, IDN-17, RST-04).
- [P0] Permissões efetivas vêm só de `FindEffectivePermissions` do `identity`,
  a cada requisição e sem cache (SPEC-7KW0HHF6:59, CTX-16). Claims de
  permissão do token são descartadas.
- [P0] Sujeito e tenant vêm só do token verificado; `X-Subject-ID`,
  `X-Tenant-ID` ou `?tenant_id` divergentes recusam com 403 (CTX-06). Todo
  campo `*ByUserId` da mensagem gRPC recebe o `sub`.
- [P0] Nenhuma extensão do kernel é escrita neste workspace: deny-list, mapper
  de tenant e verificador OIDC vêm do kernel (ADR-022, item 3).
- [P0] Arquivos gerados pelo `dmpf-plugin` e pelo `infrasync` mudam só por
  regeneração, nunca por edição manual.
- [P1] Corpo de erro, headers e status de idempotência seguem o kernel, não o
  doc 91 (ADR-022, item 1).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] App `bff`**: criar o módulo Go
  `github.com/mateusmacedo/antecipa-market/apps/backend/bff` (`go 1.26.9`,
  bibliotecas do kernel na versão consumida), projeto Nx `bff` com tags
  `type:app`, `scope:backend`, `stack:go`, `layer:apps`, `dmpf-units.json` com
  a unidade `bff/app` (`bounded_context: bff`) e entrada no `go.work`.
- [ ] **[P0] Cadeia da borda**: montar, de fora para dentro, a ordem da
  referência: `otelhttp`, rota, access log, CORS, mux, recover, prazo da rota,
  `ExecutionContext` (`ResolveIdentity`), admissão, `Idempotency-Key`,
  handler.
- [ ] **[P0] Autenticação OIDC**: `authn.NewVerifier` com `OIDC_ISSUER`,
  `OIDC_AUDIENCE` e `OIDC_TENANT_CLAIM` apontando para o claim achatado do
  Keycloak de referência (SPEC-SBW7HT6Y). `AUTH_DEV_MOCK` só em dev e testes,
  nunca junto de issuer. Token ausente ou inválido responde 401
  `unauthenticated`.
- [ ] **[P0] Permissões efetivas**: um `ports.Authenticator` decorador que
  verifica o token pelo `authn`, chama `IdentityService.FindEffectivePermissions`
  com `userId` igual ao `sub` e `x-tenant-id` do token, e substitui
  `Identity.Permissions` pelo resultado.
  - Lista vazia: a rota responde 403 `permission-denied`.
  - `identity` indisponível ou prazo esgotado: responde 503 ou 504, nunca
    segue com permissões das claims nem com lista vazia silenciosa.
  - Sem cache de resultado nem de ausência.
- [ ] **[P0] Revogação de sessão**: ligar o provider de deny-list do `authn`
  (SPEC-STEJG347) no composition root, com a fonte de eventos de sessão da
  SPEC-SBW7HT6Y. Sessão revogada responde 401 em até 60 s (ADR-016, condição 2).
- [ ] **[P0] Rotas do identity**: registrar as 21 rotas da tabela abaixo, cada
  uma com `ContractRef` para a operação do OpenAPI, `Permission`
  `identity:<operação-em-kebab>` (`apps/backend/identity/app/authorization.go`)
  e, nos `POST`, `Idempotency-Key` obrigatório.
- [ ] **[P0] Preenchimento do ator**: preencher todo campo `*ByUserId` das
  mensagens gRPC com o `sub` do token; o corpo externo não carrega esses campos
  (OpenAPI do identity, `info.description`).
- [ ] **[P0] Erros**: traduzir a resposta gRPC para HTTP pela tabela do kernel
  (`grpc.HTTPStatus` e `app/rpc/errors.go` da referência). Rejeição no
  `oneof result` responde 422 com o código de domínio; o `reason` do
  `ErrorInfo` é lido antes do código gRPC; corpo `{"code","message"}` em
  `application/json; charset=utf-8`.
- [ ] **[P0] Metadata gRPC**: enviar `x-correlation-id`, `x-causation-id`,
  `x-tenant-id`, `traceparent`, `idempotency-key` derivado
  (`hex(sha256(subject || 0x00 || key))`) e `x-locale`. Sujeito e permissões não
  atravessam o salto (CTX-12, IDN-02).
- [ ] **[P0] Idempotência**: `Idempotency-Key` no formato
  `^[A-Za-z0-9._-]{1,128}$`; ausência ou formato inválido responde 400. Replay
  devolve o mesmo status e corpo com `Idempotent-Replayed: true`, só abaixo de
  500 (ADR-056 do kernel, §2 e §6). Em `POST /identity/organization-lookups` a
  chave é exigida, mas nada é armazenado nem repetido.
- [ ] **[P0] Cliente gRPC**: `kernelgrpc.Config` com `MethodPolicy` para cada
  método do descriptor de `company.identity.service.v1.IdentityService`; prazo
  do método menor que o da rota (GRP-17); retry só em `UNAVAILABLE`; breaker
  contando só indisponibilidade. Teste reprova método do descriptor sem
  política.
- [ ] **[P0] mTLS de cliente**: `GRPC_CA_FILE`, `GRPC_CLIENT_CERT_FILE`,
  `GRPC_CLIENT_KEY_FILE` e `GRPC_SERVER_NAME`, com identidade
  `spiffe://dmpf/bff` (ADR-052 do kernel). `GRPC_INSECURE` só no host local.
- [ ] **[P0] Borda declarada**: `dmpf.json` com `"edge": "bff"`; regenerar pelo
  `bounded-context` os arquivos do `identity` que dependem de `edgeApp`
  (NetworkPolicy, `GRPC_TRUSTED_CLIENTS` do overlay hmg, `project.json`,
  `README.md`); `deploy/infra.json` do `bff` com `grpc.client: bff` e rodar
  `infrasync --write` para emitir o certificado de cliente em `pki.yml`.
- [ ] **[P0] OpenAPI servido**: servir `/openapi/identity/v1/openapi.yaml` a
  partir de `OPENAPI_IDENTITY_PATH`. Teste estrutural garante que cada rota
  registrada existe no contrato com o método e o caminho declarados.
- [ ] **[P1] Admissão**: limite por rota e tenant, recusa 429
  `admission-refused`; tenants com balde próprio por `METRIC_TENANTS`
  (RES-16, RES-17).
- [ ] **[P1] CORS**: allowlist `CORS_ORIGINS`, vazia por padrão; no compose
  local, a origem do Swagger UI (`http://localhost:8082`).
- [ ] **[P1] Limites de forma**: corpo até 64 KiB e `DisallowUnknownFields`
  (TRP-49, TRP-51).
- [ ] **[P1] Saúde**: `/livez` e `/readyz` em `ADMIN_ADDR` (`:8090`), fora do
  contrato e sem autenticação; `/readyz` falha enquanto a conexão gRPC com o
  `identity` não estiver pronta.
- [ ] **[P1] Observabilidade**: `otelboot` próprio, access log, span por
  requisição e `X-Correlation-ID` propagado ou gerado e exposto na resposta.
- [ ] **[P1] Deploy**: `Dockerfile`, `deploy/compose.yml` estendendo
  `dmpf-http-edge`, `deploy/.env.example` com `IDENTITY_GRPC_TARGET`, porta
  HTTP 8080 (`HTTP_PORT`, `infra/local/env.platform.example`), `deploy/k8s/base`
  e overlays `dev` e `hmg`, e targets Nx equivalentes aos da referência
  (`build`, `vet`, `fmt-check`, `tidy`, `test-race`, `govulncheck`, `serve`,
  `docker:run`, `deploy-env`, `k8s-render`).

#### Rotas do identity nesta versão

| Rota | RPC | Permissão | Chave |
| --- | --- | --- | --- |
| `POST /identity/organizations` | `RegisterOrganization` | `identity:register-organization` | sim |
| `POST /identity/organization-lookups` | `FindOrganizationByTaxId` | `identity:find-organization-by-tax-id` | sim, sem replay |
| `GET /identity/organizations/{organizationId}` | `FindOrganization` | `identity:find-organization` | não |
| `POST /identity/organizations/{organizationId}/establishments` | `AddEstablishment` | `identity:add-establishment` | sim |
| `GET /identity/organizations/{organizationId}/establishments` | `FindEstablishmentsByOrganization` | `identity:find-establishments-by-organization` | não |
| `POST /identity/organizations/{organizationId}/representations` | `RegisterRepresentation` | `identity:register-representation` | sim |
| `GET /identity/organizations/{organizationId}/representations` | `FindRepresentationsByOrganization` | `identity:find-representations-by-organization` | não |
| `POST /identity/organizations/{organizationId}/memberships` | `InviteUser` | `identity:invite-user` | sim |
| `GET /identity/organizations/{organizationId}/memberships` | `FindMembershipsByOrganization` | `identity:find-memberships-by-organization` | não |
| `POST /identity/organizations/{organizationId}/bank-account-changes` | `RequestBankAccountChange` | `identity:request-bank-account-change` | sim |
| `GET /identity/organizations/{organizationId}/bank-account-changes` | `FindBankAccountChangesByOrganization` | `identity:find-bank-account-changes-by-organization` | não |
| `POST /identity/persons` | `RegisterPerson` | `identity:register-person` | sim |
| `GET /identity/persons/{personId}` | `FindPerson` | `identity:find-person` | não |
| `GET /identity/representations/{representationId}` | `FindRepresentation` | `identity:find-representation` | não |
| `POST /identity/users` | `ProvisionUserAccount` | `identity:provision-user-account` | sim |
| `POST /identity/users/{userId}/suspend` | `SuspendUser` | `identity:suspend-user` | sim |
| `GET /identity/users/{userId}/memberships` | `FindMembershipsByUser` | `identity:find-memberships-by-user` | não |
| `GET /identity/users/{userId}/permissions` | `FindEffectivePermissions` | `identity:find-effective-permissions` | não |
| `POST /identity/memberships/{membershipId}/revoke` | `RevokeMembership` | `identity:revoke-membership` | sim |
| `GET /identity/bank-account-changes/{bankAccountChangeId}` | `FindBankAccountChange` | `identity:find-bank-account-change` | não |
| `POST /identity/bank-account-changes/{bankAccountChangeId}/approve` | `ApproveBankAccountChange` | `identity:approve-bank-account-change` | sim |

No catálogo congelado da SPEC-7KW0HHF6, só `operator` concede permissões
`identity:*`, e apenas três: `request-bank-account-change`,
`find-bank-account-change` e `find-bank-account-changes-by-organization`. As
outras 18 rotas ficam registradas e respondem 403 a todo usuário até a fatia de
papéis seguinte. É o comportamento esperado (fail-closed), não um defeito.

### Não-funcionais

- [ ] Segurança: toda rota de negócio exige token verificado e permissão
  declarada; nenhuma permissão é lida de claim; o acesso a id de outro tenant
  volta como 404 (ADR-051 do kernel).
- [ ] Segurança: sessão revogada é recusada em até 60 s (ADR-016).
- [ ] Tempo: o prazo de cada rota é declarado no `Budget` da rota, e o de cada
  método gRPC é estritamente menor (GRP-17); o deadline é propagado, nunca
  reiniciado (ADR-024 do kernel).
- [ ] Resiliência: com o `identity` fora, a borda responde 503 em vez de travar,
  e `/readyz` reprova.
- [ ] Compatibilidade: Go `1.26.9`, kernel na versão consumida pelo
  `dmpf.json`/`go.work`, mesmo runner de CI dos projetos `stack:go`
  (`dmpf-ci.yml`).

### Pontos a verificar na implementação

- O `surface.query` da referência é `GET`. Confirmar como registrar
  `POST /identity/organization-lookups` (consulta por `POST` com chave exigida e
  sem replay) sem burlar `ValidateEdge`.
- O teste estrutural da referência (`app/api/openapi_test.go:77`) exige os
  status 400/404/409/422/429/503/504/default; as consultas do `identity` não
  declaram 409/422. Ajustar o teste ao tipo da rota sem relaxar o contrato.
- Confirmar o comando de regeneração do `identity` após mudar o `edge`
  (generator `bounded-context` ou `/dmpf-new-context SPEC-7KW0HHF6 --regen`) e
  a forma de rodar o `infrasync` (`tools/dmpf-conformance`, tag `conformance`
  do `versions.json` do plugin).
- Confirmar a versão do kernel que publica SPEC-SBW7HT6Y e SPEC-STEJG347 e o
  bump correspondente (ADR-011), antes de fixar `go.mod`.

## Camadas afetadas

| Camada | Afetada? | Descrição |
| --- | --- | --- |
| Borda HTTP (`bff/app/api`) | [x] | Rotas, cadeia de middleware, handlers, saúde |
| Cliente gRPC (`bff/app/rpc`) | [x] | Cliente do `IdentityService`, erros, metadata, replay, TLS |
| Autenticação (`bff/app`) | [x] | Verificador OIDC do kernel e decorador de permissões |
| Contexto `identity` | [x] | Só arquivos gerados que dependem de `edgeApp` (regeneração) |
| Infra local e k8s | [x] | `dmpf.json`, `go.work`, `pki.yml` (infrasync), compose e overlays do `bff` |
| Domínio e aplicação | [ ] | Nenhuma mudança; o `bff` não tem domínio |
| Banco | [ ] | O `bff` não declara banco |

## Localização de código

```text
apps/backend/bff/
  cmd/main.go                — binário único; sobe HTTP e porta de administração
  app/config.go              — leitura de variáveis (HTTP_ADDR, ADMIN_ADDR, OIDC_*, IDENTITY_GRPC_TARGET, OPENAPI_IDENTITY_PATH, GRPC_*, CORS_ORIGINS, METRIC_TENANTS)
  app/wiring.go              — composition root: authn, decorador, clientes, handler
  app/authn.go               — Authenticator decorador com FindEffectivePermissions
  app/telemetry.go, probe.go — otelboot e prontidão
  app/api/routes.go          — bindings das 21 rotas do identity
  app/api/handlers_identity.go — tradução JSON <-> mensagens do IdentityService
  app/api/middleware.go, handlers.go, health.go, traceparent.go — porte da referência
  app/rpc/clients.go, errors.go, metadata.go, replay.go, ready.go, tls.go — porte da referência
  deploy/                    — compose.yml, .env.example, infra.json, k8s/base, overlays dev e hmg
  dmpf-units.json, project.json, go.mod, Dockerfile, README.md
```

**Arquivos a modificar**:

- `dmpf.json`: `"edge": "bff"`.
- `go.work`: `use ./apps/backend/bff`.
- `apps/backend/identity/deploy/k8s/base/networkpolicy.yaml`,
  `.../overlays/hmg/configmap-identity-patch.yaml`,
  `apps/backend/identity/project.json`, `apps/backend/identity/README.md`: só
  por regeneração.
- `infra/local/compose/pki.yml` e demais arquivos do `infrasync`: só por
  `infrasync --write`.

## Design

### Arquitetura

```text
cliente HTTPS --(Bearer)--> bff :8080
                             | authn.Verifier (kernel) -> sub, tenant
                             | decorador -> IdentityService.FindEffectivePermissions (gRPC mTLS)
                             | ResolveIdentity confere route.Permission
                             | admissão, Idempotency-Key, prazo
                             v
                   IdentityService (gRPC :9090, mTLS, só spiffe://dmpf/bff)
```

### Fluxo principal

1. A requisição entra; `otelhttp` abre o span e a rota é resolvida no mux.
2. O prazo da rota é aplicado; `ResolveIdentity` chama o decorador.
3. O decorador verifica o token e consulta `FindEffectivePermissions` com o
   tenant do token.
4. `ResolveIdentity` confere a permissão da rota; a admissão por rota e tenant
   decide; nos `POST`, a `Idempotency-Key` é validada e derivada.
5. O handler monta a mensagem gRPC, preenche `*ByUserId` com o `sub` e chama o
   RPC com a metadata da borda.
6. A resposta vira JSON; rejeição vira 422; falha técnica vira o status da
   tabela do kernel.

### Pseudocódigo do decorador

```text
Authenticate(ctx, credential):
  identity = verifier.Authenticate(ctx, credential)      # 401 se falhar
  perms = identity_client.FindEffectivePermissions(ctx with x-tenant-id=identity.tenant, userId=identity.Subject)
  if erro técnico: devolve erro -> 503/504 (nunca segue)
  identity.Permissions = perms                           # claims descartadas
  return identity
```

## Decisões técnicas

- **Porte da referência do kernel, à mão**: o `@mateusmacedo/dmpf-plugin` só tem
  os generators `init` e `bounded-context`, e o agente `dmpf-context-author`
  não edita o `bff`. Alternativa descartada: pedir um generator de `bff` ao
  kernel antes, porque atrasa a borda sem mudar o resultado.
- **Permissões por decorador de `Authenticator`**: cumpre a SPEC-7KW0HHF6:59 sem
  alterar o kernel, porque `ResolveIdentity` só lê `Identity.Permissions`.
  Alternativa descartada: `OIDC_PERMISSION_CLAIMS`, já rejeitada na
  SPEC-7KW0HHF6 por criar duas fontes de verdade.
- **Falha do identity nega**: sem permissões confirmadas não há autorização.
  Alternativa descartada: seguir com lista vazia, que esconde a indisponibilidade
  como 403.
- **Convenções do kernel sobre o doc 91**: o corpo de erro `{code,message}`,
  `Idempotent-Replayed` e o 422 para chave reusada seguem o kernel (ADR-022,
  item 1). O contrato publicado do `identity` já usa essa forma.
- **Caminhos sem `/v1`**: mantém o contrato publicado do `identity`
  (`/identity/...`). O kernel não decide versionamento de rota (ADR-024 do
  kernel deixa sem dona), e mudar o caminho quebraria o contrato já aprovado.
- **Dependências do kernel bloqueantes**: revogação em 60 s e tenant achatado
  são condições da ADR-016; decisão do discovery foi não entregar sem elas.
- **`grantMembership` fora desta versão**: o contrato diz que o `bff` conduz a
  verificação do canal e dos dados pessoais e define `channelConfirmed` e
  `verificationLevel`, mas nenhum documento define como. Expor a rota exigiria
  inventar esse fluxo ou enviar valores não verificados.
- **e2e caixa-preta sem caminho feliz de negócio**: sem `org_admin` nem fluxo de
  primeiro vínculo (SPEC-7KW0HHF6:594), nenhum usuário obtém permissão pela API,
  e o e2e não lê nem semeia tabela de contexto (ADR-053 do kernel, §6). Os
  caminhos felizes ficam em testes com servidor gRPC falso (`bufconn`); o e2e
  real cobre autenticação, negação, contrato servido e saúde.

## Regras relacionadas

- `docs/adr/012-um-app-por-bounded-context.md`, `016-provedor-de-identidade-oidc.md`,
  `022-precedencia-dmpf-sobre-design.md`.
- SPEC-7KW0HHF6: catálogo de papéis, `FindEffectivePermissions`, Escopo fora.
- SPEC-GXGASP24: ADRs de fundação.
- `.claude/rules/process-enforcement.md`: cadeia de validação Go.

## Verificação e testes

### Critérios de aceite

- [ ] `pnpm nx run bff:build`, `vet`, `fmt-check`, `test-race` e `govulncheck`
  passam; `dmpf-ci.yml` verde para `bff` e `identity`.
- [ ] O `bff` recusa partir com rota sem permissão ou `POST` sem chave
  (`ValidateEdge`).
- [ ] As 21 rotas existem no OpenAPI do `identity` com o mesmo método e
  caminho (teste estrutural).
- [ ] Todo método do `IdentityService` tem `MethodPolicy` (teste pelo
  descriptor).
- [ ] Token ausente, expirado ou de issuer errado responde 401; usuário sem
  vínculo ativo responde 403 em todas as 21 rotas.
- [ ] `FindEffectivePermissions` é chamada uma vez por requisição autenticada e
  nunca em requisição sem token (teste com servidor falso).
- [ ] Com o `identity` fora, a rota responde 503 e `/readyz` reprova.
- [ ] Replay de `POST` com a mesma chave devolve o mesmo status e corpo com
  `Idempotent-Replayed: true`; chave reusada com corpo diferente responde 422.
- [ ] Sessão revogada no Keycloak de referência é recusada em até 60 s.
- [ ] `GET /openapi/identity/v1/openapi.yaml` devolve o contrato, e o Swagger UI
  local o carrega.
- [ ] Em hmg, a NetworkPolicy do `identity` admite só o pod do `bff` na porta
  9090, e `GRPC_TRUSTED_CLIENTS` é `spiffe://dmpf/bff`.

### Cenários de teste

1. **Caminho feliz (servidor falso)**: DADO um token válido de `sub` S no
   tenant T e `FindEffectivePermissions` devolvendo
   `identity:request-bank-account-change` QUANDO chega
   `POST /identity/organizations/{id}/bank-account-changes` com
   `Idempotency-Key` válida ENTÃO o RPC recebe `requestedByUserId = S`,
   `x-tenant-id = T` e a chave derivada, e a resposta é 201.
2. **Fail-closed**: DADO um token válido e `FindEffectivePermissions` devolvendo
   lista vazia QUANDO chega `GET /identity/organizations/{id}` ENTÃO a resposta
   é 403 `permission-denied` e o RPC `FindOrganization` não é chamado.
3. **Identidade forjada**: DADO um token do tenant T QUANDO a requisição traz
   `X-Tenant-ID` com outro tenant ENTÃO a resposta é 403 `identity-mismatch`.
4. **Rejeição de domínio**: DADO o RPC devolvendo `Rejection` com código
   `identity-organization/bank-account-change/...` QUANDO o handler traduz
   ENTÃO a resposta é 422 com esse código no corpo.
5. **Identity indisponível**: DADO o servidor gRPC fora QUANDO chega qualquer
   rota autenticada ENTÃO a resposta é 503 `unavailable` e nenhuma permissão de
   claim é usada.
6. **e2e real**: DADO `bff` e `identity` em container com mTLS QUANDO um usuário
   sem vínculo chama as 21 rotas ENTÃO todas respondem 403, e
   `GET /openapi/identity/v1/openapi.yaml` responde 200.

<critical_constraints>
- [P0] O `bff` não importa domínio nem aplicação de contexto: só módulos
  `contract` dos contextos e bibliotecas do kernel (ADR-012; ADR-044 do kernel;
  regra `DMPF-D002`).
- [P0] Uma rota encaminha para exatamente uma chamada gRPC. Sem orquestração,
  sem saga, sem banco.
- [P0] Toda rota declara `Permission` e `ContractRef`; `ValidateEdge` recusa na
  partida rota sem permissão ou `POST` sem exigência de `Idempotency-Key`
  (IDN-16, IDN-17, RST-04).
- [P0] Permissões efetivas vêm só de `FindEffectivePermissions` do `identity`,
  a cada requisição e sem cache (SPEC-7KW0HHF6:59, CTX-16). Claims de
  permissão do token são descartadas.
- [P0] Sujeito e tenant vêm só do token verificado; `X-Subject-ID`,
  `X-Tenant-ID` ou `?tenant_id` divergentes recusam com 403 (CTX-06). Todo
  campo `*ByUserId` da mensagem gRPC recebe o `sub`.
- [P0] Nenhuma extensão do kernel é escrita neste workspace: deny-list, mapper
  de tenant e verificador OIDC vêm do kernel (ADR-022, item 3).
- [P0] Arquivos gerados pelo `dmpf-plugin` e pelo `infrasync` mudam só por
  regeneração, nunca por edição manual.
- [P1] Corpo de erro, headers e status de idempotência seguem o kernel, não o
  doc 91 (ADR-022, item 1).
</critical_constraints>

## Escopo fora

- **Rotas dos outros 16 contextos**: cada contexto acrescenta as próprias rotas
  ao `bff` na sua spec ou task, quando o servidor gRPC dele existir.
- **`POST /identity/memberships/{membershipId}/grant`**: depende da verificação
  de canal e de dados pessoais conduzida pelo `bff`, sem definição em nenhum
  documento [ABERTA].
- **Partner API** (mTLS e credencial de cliente, doc 91 §4) e **webhooks de
  entrada e de saída** (doc 91 §5): sem ADR no kernel; a assinatura de saída
  depende de SPEC-5AQHHMJ0 e SPEC-MSA607GT do kernel.
- **Versionamento `/v1` nas rotas e paginação por cursor** (doc 91 §1): o kernel
  não decide; ficam como lacuna até uma norma.
- **Organização ativa por `X-Organization-Id`** (doc 91): o tenant vem só do
  token, com a escolha de organização ativa feita no Keycloak (SPEC-SBW7HT6Y).
- **Step-up e MFA por ação crítica** (`acr`, `auth_time`): o `authn` não lê esses
  claims [ABERTA: SPEC-7KW0HHF6:454].
- **Limites numéricos do doc 91** (600/min por usuário): a admissão do kernel é
  por rota e tenant; os valores ficam em configuração.
- **Leitura entre tenants do backoffice**: SPEC-QVDGV6J0 (DEVS-100).
- **Primeiro vínculo e papéis `approver`, `org_admin`, de financiador e de
  plataforma**: Escopo fora da SPEC-7KW0HHF6.
- **Ambiente de produção**: só dev e hmg pelo baseline (ADR-019).
- **Frontend e telas**: fora do workspace backend.
