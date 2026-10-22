# identity

Serviço de identidade e organização da Plataforma Multiativo de Antecipação de
Recebíveis (repositório `antecipa-market`): guarda, por
tenant, as organizações e seus estabelecimentos, as pessoas e seus poderes de
representação, as contas de usuário com seus vínculos e permissões, e as trocas
de conta bancária sob dupla aprovação.

É o bounded context `identity-organization`, especificado em
[`SPEC-7KW0HHF6`](../../../docs/specs/SPEC-7KW0HHF6-identity-organization.md) e
construído sobre o kernel DMPF `v1.0.0-rc.7`.

## Sumário

- [Visão geral](#visão-geral)
- [Stack](#stack)
- [Pré-requisitos](#pré-requisitos)
- [Primeiros passos](#primeiros-passos)
- [Configuração](#configuração)
- [Uso](#uso)
- [Arquitetura](#arquitetura)
- [Testes](#testes)
- [Contrato](#contrato)
- [Deploy](#deploy)
- [Contribuição](#contribuição)
- [Documentação relacionada](#documentação-relacionada)
- [Licença](#licença)

## Visão geral

O `identity` é a raiz da cadeia de contextos da plataforma: os demais contextos
consultam nele quem é a organização e quem a representa, e o `bff` consulta
nele, a cada requisição, o que cada usuário pode fazer.

- **Serve só gRPC interno** (`company.identity.service.v1.IdentityService`),
  com 11 comandos e 11 consultas. O REST público é do `bff`, que serve o
  OpenAPI publicado em
  [`contract/openapi/v1/openapi.yaml`](contract/openapi/v1/openapi.yaml).
- **Publica 7 eventos** de integração no canal Kafka `identity.events`, pela
  outbox, e não consome canal nenhum.
- **Isola por tenant**: toda leitura e escrita é escopada pelo tenant do token
  verificado; um identificador de outro tenant responde como inexistente.
- **Protege identificadores**: CNPJ, CPF e documento de pessoa ganham um hash
  HMAC-SHA256 com chave secreta, usado na unicidade, na busca e nos eventos,
  que levam só hash e máscara. O CNPJ e o CPF de organizações e
  estabelecimentos continuam guardados em claro, como PII sem envelope até a
  SPEC-CC5SHV1K; o documento da pessoa nunca é guardado.

### Funcionalidades

| Agregado | O que o contexto faz |
| --- | --- |
| `Organization` | Registra a pessoa jurídica (CNPJ alfanumérico aceito), único por identificador fiscal e jurisdição no tenant |
| `Establishment` | Acrescenta estabelecimentos com identificador fiscal próprio, únicos por organização |
| `Person` | Registra pessoas naturais por hash e máscara do documento, sem guardar o documento |
| `Representation` | Registra poderes de representação com fonte, evidência, alçada e vigência |
| `UserAccount` | Provisiona a conta do usuário autenticado (inicializa se ausente) e a suspende |
| `Membership` | Convida, concede e revoga o vínculo do usuário com a organização, com os papéis `operator` e `auditor` (organização cliente) e `platform_ops` (organização de plataforma) |
| `BankAccountChange` | Propõe e aprova a troca de conta bancária, com aprovador distinto do proponente |

`FindEffectivePermissions` devolve, a cada requisição do `bff`, as permissões
efetivas do usuário: a união ordenada das permissões dos papéis dos vínculos
ativos, ou lista vazia quando a conta não existe ou está suspensa.

### Papéis do processo

O mesmo binário roda em dois papéis, escolhidos por `--role`:

| Papel | O que faz |
| --- | --- |
| `api` | Serve o `IdentityService` por gRPC e purga as entradas de comando vencidas da inbox |
| `relay` | Drena a outbox para `identity.events` e purga a outbox publicada |

`--role consumer` é recusado, porque o contexto não consome canal.

## Stack

- **Go** `1.26.10`, a diretiva do `go.work`, que o CI usa.
- **Kernel DMPF** `v1.0.0-rc.7`, com UoW, outbox, inbox, gRPC e observabilidade.
- **gRPC e Protobuf**, com contrato gerado pelo Buf (`buf` `v1.72.0` e
  `protoc-gen-go` `v1.36.12`).
- **PostgreSQL**, por `pgx` v5, com persistência híbrida de colunas tipadas e
  `snapshot` `jsonb`.
- **Kafka**: Redpanda em desenvolvimento e teste, com envelope CloudEvents.
- **OpenTelemetry** para traces, métricas e logs.
- **Nx** e **pnpm** para tarefas, cache e scaffolding (`@mateusmacedo/dmpf-plugin`).

## Pré-requisitos

- Node.js `^24` e pnpm `^11`. A versão exata vem do campo `packageManager` da
  raiz e é ativada pelo `corepack enable`.
- Go `1.26.10`. Se o seu Go local for mais novo, rode os gates com
  `GOTOOLCHAIN=go1.26.10` para reproduzir o CI.
- Docker com o plugin Compose (`docker compose`) 2.20 ou mais novo, por causa
  do `include:` do compose local, para a infra local e a de testes.

## Primeiros passos

Na raiz do repositório:

```bash
corepack enable
pnpm install

# Postgres e Redpanda locais, com banco, role, tópicos e ACLs do identity
docker compose -f infra/local/docker-compose.yml \
  --profile postgres --profile redpanda --profile provisioning up -d

# deploy/.env a partir do exemplo, que traz uma chave sintética de desenvolvimento
pnpm nx run identity:deploy-env

pnpm nx run identity:serve-api     # gRPC em 127.0.0.1:9194 (GRPC_ADDR do deploy/.env)
pnpm nx run identity:serve-relay   # sobe o api antes, se preciso
```

Para validar o contexto contra banco, rode a suíte de integração. Ela sobe a
própria infra de testes (`test-infra-up`, portas 15432 e 19092), separada da
infra local acima:

```bash
pnpm nx run identity:test-race
```

## Configuração

A configuração vem só do ambiente, sem prefixo `DMPF_`, pela forma
`Defaults(role)`, `FromEnv` e `Validate` de [`app/config.go`](app/config.go).
Os targets `serve-*` leem `deploy/.env.example` e, por cima, `deploy/.env`.

| Variável | Papel | Efeito |
| --- | --- | --- |
| `PG_DSN` | todos, obrigatória | Banco do contexto |
| `GRPC_INSECURE` ou `GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE`, `GRPC_CLIENT_CA_FILE` e `GRPC_TRUSTED_CLIENTS` | `api`, obrigatória uma das políticas | `true` dispensa o TLS, só em desenvolvimento; senão, certificado do servidor e autenticação do chamador por mTLS (`IDN-03`) |
| `IDENTITY_IDENTIFIER_HMAC_KEY` | `api`, obrigatória | Chave HMAC dos identificadores e do digest de idempotência, com no mínimo 32 bytes |
| `IDENTITY_IDENTIFIER_HMAC_KEY_VERSION` | `api`, obrigatória | Inteiro positivo, gravado ao lado de cada hash |
| `GRPC_ADDR` | `api` | Default `:9090` no código; o `deploy/.env.example` usa `127.0.0.1:9194` |
| `MIGRATE` | `api` | Aplica o schema antes de servir |
| `METRIC_TENANTS` | `api` | Tenants com balde de admissão e rótulo próprios (`MET-07`) |
| `KAFKA_BROKERS`, `KAFKA_IDENTITY_TOPIC`, `KAFKA_IDENTITY_DLQ`, `KAFKA_GROUP` | `relay`, obrigatórias | Brokers, endereço, contenção e grupo do canal `identity.events` |
| `KAFKA_SASL_MECHANISM` (com `KAFKA_SASL_USERNAME` e `KAFKA_SASL_PASSWORD`) ou `KAFKA_CLIENT_CERT_FILE` (com `KAFKA_CLIENT_KEY_FILE`) | `relay`, obrigatória sem `KAFKA_INSECURE` | Autenticação do cliente no broker; `KAFKA_CA_FILE` fixa a CA |
| `KAFKA_INSECURE` | `relay` | `true` desliga o TLS e dispensa a autenticação, só em desenvolvimento |
| `LOG_LEVEL` | todos | Default `info` |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` | todos | Identidade do recurso OTel; sem `OTEL_SERVICE_NAME`, o serviço é `identity`. `service.version` e `service.instance.id` não têm default, e sem eles a partida falha com `ErrResourceIncomplete` (exit 1) |
| `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_LOGS_EXPORTER`, `OTEL_PROPAGATORS`, `OTEL_GO_X_OBSERVABILITY` | todos | Exportação OTLP; propagador diferente de `tracecontext` recusa a partida |

A recusa de configuração encerra com exit 2: variável obrigatória ausente
(`ErrMissingVariable`, nomeando a primeira que falta), valor inválido
(`ErrInvalidVariable`: chave curta, versão que não é inteiro positivo, booleano
malformado) ou papel desconhecido. A falha depois da partida encerra com exit 1.

O `api` abre dois pools sobre o `PG_DSN`, um para as transações e outro para
as consultas, que correm fora de transação e assim não disputam conexão com os
comandos (a unicidade é a restrição declarada, sem leitura antes do `Save`,
desde o kernel `1.0.0-rc.7`); cada pool
segue o `pool_max_conns` do DSN, ou o default do pgx, `max(4, NumCPU)`, então
o processo abre até o dobro desse valor. O kernel nomeia o pool só por
`host:porta/banco`, então as séries `db.client.connection.*` do `api` somam os
dois pools, e a saturação de um deles não aparece sozinha nos painéis até o
kernel aceitar nome de pool (DEVS-108).

Os prazos de idempotência e de purga vêm de `Defaults()`, sem variável de
ambiente:

| Campo | Padrão | Uso |
| --- | --- | --- |
| `IdempotencyWait` | 1s | Espera máxima por comando concorrente da mesma chave |
| `IdempotencyRetention` | 24h | Vida da entrada de comando na inbox |
| `OutboxRetention` | 168h | Idade a partir da qual a outbox publicada é purgada |
| `PurgeInterval`, `PurgeBatch` | 15min, 1000 | Intervalo e lote de cada purga |

O relay tenta publicar cada evento até 24 vezes, com backoff exponencial de
200 ms até o teto de 5 min. No kernel, toda falha de publicação conta como
tentativa, inclusive o broker fora do ar; essa política aguenta pelo menos
cerca de 33 minutos de indisponibilidade antes de o evento ir para `failed`,
estado terminal que exige retomada manual. Vigie a outbox com `failed` acima
de zero (DEVS-109).

### Chave de identificador

- A chave tem no mínimo 32 bytes e é conferida na partida do `api`.
- A versão é gravada ao lado de cada hash (`taxIdHashKeyVersion`,
  `documentHashKeyVersion`).
- **A rotação é proibida nesta versão**: sem o documento da pessoa em claro, o
  `document_hash` guardado não pode ser recalculado sob outra chave, e um hash
  novo deixaria de casar com os antigos na busca e na unicidade.
- A chave nunca aparece em log nem em saída formatada: o `Config` a guarda num
  `Secret`, e a telemetria registra só a presença dela.

## Uso

### Serviço gRPC

O `IdentityService` tem 22 métodos. A rota é a do OpenAPI publicado para o
`bff`, e a permissão é a que [`app/authorization.go`](app/authorization.go)
declara para a operação.

| Agregado | Método | Tipo | Rota no OpenAPI | Permissão |
| --- | --- | --- | --- | --- |
| `Organization` | `RegisterOrganization` | comando | `POST /identity/organizations` | `identity:register-organization` |
| | `FindOrganization` | consulta | `GET /identity/organizations/{organizationId}` | `identity:find-organization` |
| | `FindOrganizationByTaxId` | consulta | `POST /identity/organization-lookups` | `identity:find-organization-by-tax-id` |
| `Establishment` | `AddEstablishment` | comando | `POST /identity/organizations/{organizationId}/establishments` | `identity:add-establishment` |
| | `FindEstablishmentsByOrganization` | consulta | `GET /identity/organizations/{organizationId}/establishments` | `identity:find-establishments-by-organization` |
| `Person` | `RegisterPerson` | comando | `POST /identity/persons` | `identity:register-person` |
| | `FindPerson` | consulta | `GET /identity/persons/{personId}` | `identity:find-person` |
| `Representation` | `RegisterRepresentation` | comando | `POST /identity/organizations/{organizationId}/representations` | `identity:register-representation` |
| | `FindRepresentation` | consulta | `GET /identity/representations/{representationId}` | `identity:find-representation` |
| | `FindRepresentationsByOrganization` | consulta | `GET /identity/organizations/{organizationId}/representations` | `identity:find-representations-by-organization` |
| `UserAccount` | `ProvisionUserAccount` | comando | `POST /identity/users` | `identity:provision-user-account` |
| | `SuspendUser` | comando | `POST /identity/users/{userId}/suspend` | `identity:suspend-user` |
| `Membership` | `InviteUser` | comando | `POST /identity/organizations/{organizationId}/memberships` | `identity:invite-user` |
| | `GrantMembership` | comando | `POST /identity/memberships/{membershipId}/grant` | `identity:grant-membership` |
| | `RevokeMembership` | comando | `POST /identity/memberships/{membershipId}/revoke` | `identity:revoke-membership` |
| | `FindMembershipsByUser` | consulta | `GET /identity/users/{userId}/memberships` | `identity:find-memberships-by-user` |
| | `FindMembershipsByOrganization` | consulta | `GET /identity/organizations/{organizationId}/memberships` | `identity:find-memberships-by-organization` |
| | `FindEffectivePermissions` | consulta | `GET /identity/users/{userId}/permissions` | `identity:find-effective-permissions` |
| `BankAccountChange` | `RequestBankAccountChange` | comando | `POST /identity/organizations/{organizationId}/bank-account-changes` | `identity:request-bank-account-change` |
| | `ApproveBankAccountChange` | comando | `POST /identity/bank-account-changes/{bankAccountChangeId}/approve` | `identity:approve-bank-account-change` |
| | `FindBankAccountChange` | consulta | `GET /identity/bank-account-changes/{bankAccountChangeId}` | `identity:find-bank-account-change` |
| | `FindBankAccountChangesByOrganization` | consulta | `GET /identity/organizations/{organizationId}/bank-account-changes` | `identity:find-bank-account-changes-by-organization` |

**Respostas.** Os handlers validam só a forma antes do domínio: ids de
agregado (`organization_id`, `membership_id` e os demais) em
`^[A-Za-z0-9._:-]{1,128}$`, ids de usuário que agem (`*_by_user_id`) com até
256 bytes, `tax_id` e `document` com até 32 bytes e nomes com até 256
caracteres. A entrada malformada volta como `INVALID_ARGUMENT`. Uma recusa de
domínio volta como `rejection` (`code`, `message`) na própria resposta. Uma
falha do caso de uso vira status gRPC por `kernelgrpc.StatusOf`, entre eles
`NOT_FOUND`, `ABORTED` para conflito de versão, `PERMISSION_DENIED`,
`DEADLINE_EXCEEDED`, `CANCELLED` e `INTERNAL`; os de idempotência estão na
tabela abaixo. Numa corrida pelo mesmo identificador fiscal
ou documento no mesmo tenant, o índice único decide: uma chamada grava e a
outra recebe a `rejection` de duplicidade (`duplicate-tax-id` ou
`duplicate-document`), sem linha nem evento.

### Idempotência dos comandos

Os 11 comandos exigem a metadata `idempotency-key`, no formato
`^[A-Za-z0-9._-]{1,128}$`; as consultas não leem chave.

| Situação | Resposta |
| --- | --- |
| Metadata ausente | `INVALID_ARGUMENT`, reason `MISSING_IDEMPOTENCY_KEY` |
| Chave fora do formato | `INVALID_ARGUMENT`, reason `INVALID_IDEMPOTENCY_KEY` |
| Mesma chave, mesmo pedido | A resposta gravada, aceite ou recusa, com o header `idempotent-replayed: true` |
| Mesma chave, outro pedido | `FAILED_PRECONDITION`, reason `REUSED_IDEMPOTENCY_KEY` |
| Mesma chave em andamento além da espera | `ABORTED`, reason `IN_FLIGHT_IDEMPOTENCY_KEY` |
| Comando de criação sobre id que já existe, com outra chave | `ALREADY_EXISTS` |

`ProvisionUserAccount` inicializa a conta se ela não existir: com chave nova
sobre conta já existente, aceita sem evento. A inbox guarda só o digest do
fingerprint, calculado por HMAC-SHA256 com a chave de identificador
([`provider.NewIdempotencyDigest`](provider/idempotency_digest.go)), porque o
fingerprint inclui campos em claro, como o `document` de `RegisterPerson`.

### Eventos publicados

Os eventos ficam em `contract/proto/company/identity/event/v1/`, package
`company.identity.event.v1`. Vão para a outbox na mesma transação do estado, e
o relay monta o envelope CloudEvents na publicação, com `partitionkey` igual ao
id do agregado.

| Evento | Tipo CloudEvents | Emitido por |
| --- | --- | --- |
| `OrganizationRegistered` | `com.company.identity.organization-registered.v1` | `RegisterOrganization` |
| `EstablishmentAdded` | `com.company.identity.establishment-added.v1` | `AddEstablishment` |
| `UserSuspended` | `com.company.identity.user-suspended.v1` | `SuspendUser` |
| `MembershipGranted` | `com.company.identity.membership-granted.v1` | `GrantMembership` |
| `MembershipRevoked` | `com.company.identity.membership-revoked.v1` | `RevokeMembership` |
| `BankAccountChangeRequested` | `com.company.identity.bank-account-change-requested.v1` | `RequestBankAccountChange` |
| `BankAccountChangeApproved` | `com.company.identity.bank-account-change-approved.v1` | `ApproveBankAccountChange` |

`RegisterPerson`, `RegisterRepresentation`, `ProvisionUserAccount` e
`InviteUser` não publicam evento. Nenhum evento leva identificador fiscal ou
documento em claro: só o hash e, quando a spec pede, a máscara. O canal usa os
tópicos `identity.events` e `identity.events.dlq`, declarados em
[`deploy/infra.json`](deploy/infra.json).

### Autorização e tenant

A autorização é por papel, na borda: o `bff` verifica o token, resolve as
permissões efetivas por `FindEffectivePermissions` e confere a permissão da
rota. No salto gRPC, o contexto chega sem sujeito e sem permissões, e o
`identity` exige só o tenant. Uma operação sem linha em `permissionOf` é
negada.

O tenant vem só do token verificado. A unicidade de identificador fiscal e de
documento é por tenant: o mesmo CNPJ pode existir em dois tenants. O papel
`api` só atende o workload que a CA assinou e que está na allowlist
(`GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS`), salvo com `GRPC_INSECURE=true`.

## Arquitetura

### Estrutura do projeto

```text
apps/backend/identity/
├── domain/        agregados, UPRs, códigos de rejeição, catálogo de papéis
├── ports/         repositórios, leitores e a porta IdentifierProtector
├── application/   casos de uso (comandos e consultas)
├── provider/      Postgres, mapper de eventos, HMAC e schema.sql
├── app/           composition root: config, wiring, autorização, catálogo
│   └── rpc/       borda gRPC
├── appkit/        harness de teste borda a borda sobre Postgres
├── distkit/       harness de teste distribuído (relay)
├── cmd/           binário (--role api|relay)
├── contract/      módulo do contrato: .proto, gen/go, OpenAPI e fixtures
└── deploy/        compose, Kustomize, .env.example e infra.json
```

Cada package de produção pertence a uma unidade do manifesto `dmpf-units.json`,
e as unidades caem em seis blocos DMPF: `domain`, `port`, `application`,
`provider`, `app` (que abrange `app/`, `app/rpc`, `cmd/`, `appkit/` e
`distkit/`) e `contract` (módulo próprio em `contract/`). `deploy/` não é
bloco. O `conformance` verifica as dependências entre blocos sobre o grafo
real de imports, e o `depguard` reforça as regras por bloco no lint. O bloco e
o bounded context vêm do manifesto, nunca do nome do diretório. Onde um arquivo
importa o kernel e o contexto, o kernel recebe alias pelo papel (`kernel`,
`usecase`, `port`, `kernelgrpc`, `kernelapp`).

### Banco de dados

Banco próprio (database e role `identity`, schema `public`), com `tenant_id` à
frente da chave e persistência híbrida: coluna tipada só para o que uma
consulta filtra, o resto do estado em `snapshot` `jsonb`. O DDL vive em
[`provider/schema.sql`](provider/schema.sql) e é aplicado na partida do `api`
quando `MIGRATE=true`.

| Tabela | Colunas tipadas além da chave | Unicidade por tenant |
| --- | --- | --- |
| `organizations` | `tax_id_hash`, `jurisdiction` | `(tenant_id, tax_id_hash, jurisdiction)` |
| `establishments` | `organization_id`, `tax_id_hash` | `(tenant_id, organization_id, tax_id_hash)` |
| `persons` | `document_hash` | `(tenant_id, document_hash)` |
| `representations` | `organization_id`, `person_id` | — |
| `user_accounts` | — (estado inteiro no `snapshot`) | — |
| `memberships` | `organization_id`, `user_id` (nulo até a concessão) | — |
| `bank_account_changes` | `organization_id` | — |

Do kernel vêm `outbox`, `inbox` e `quarantine`. O documento da pessoa não é
guardado: `persons` tem só o hash, a máscara e a versão da chave. Os dados
bancários também não entram aqui: `bank_account_changes` guarda só o id da
evidência do destino.

### Proteção de identificadores

`taxIdHash` e `documentHash` são HMAC-SHA256 sobre
`v1 | tenant | tipo | valor normalizado`, com cada campo prefixado pelo próprio
tamanho, em hexadecimal
([`provider/identifier_protector.go`](provider/identifier_protector.go)). O
tenant na entrada separa os hashes entre tenants. O CNPJ é normalizado em
maiúsculas e sem pontuação, aceitando o formato alfanumérico; CPF e documento
de pessoa, só dígitos.

### Dependências

O `go.mod` declara o kernel por versão (`v1.0.0-rc.7`) e o módulo de contrato
irmão em `v0.1.0`. Dentro do workspace, quem resolve é o `go.work` da raiz,
com o `replace` do contrato. O `require` do irmão é gravado pelo `modsync`, que
deve rodar de novo a cada import novo de módulo do workspace:

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/modsync@v1.0.0-rc.7 --root . --write
```

O target `tidy` mantém o `go.sum`, aplicando os `replace` do `go.work` só
durante o `go mod tidy`. Ele lê apenas o `replace` em bloco (`replace ( ... )`)
do `go.work` (DEVS-105).

## Testes

| Target | O que prova |
| --- | --- |
| `test` | Unitários sem banco: UPRs e determinismo do `domain`, sequência canônica, autorização, idempotência e instrumentação da `application`, serviço gRPC por `bufconn` e configuração |
| `test-race` | `go test -race -tags=integration ./...`: `provider` (escopo de tenant, acesso cruzado, unicidade e projeções), `appkit` e o e2e gRPC até a outbox (aceites, recusas sem escrita, replay, corridas, outro tenant e digest com chave na inbox) |
| `test-distributed` | `distkit` com as tags `integration,distributed`: duas drenagens concorrentes sobre a mesma outbox publicam cada registro exatamente uma vez |
| `e2e` | O mesmo comando do `test-distributed`, sem depender do `test-race` |

```bash
pnpm nx run identity:test
pnpm nx run identity:test-race
pnpm nx run identity:test-distributed
```

Os targets com banco sobem a infra de testes (`test-infra-up`) e preenchem
`PG_DSN` (15432) e `KAFKA_BROKERS` (19092) quando o ambiente não os declara.
Cada teste usa um banco `identity_test_<id>` próprio. Sem `KAFKA_BROKERS`, o
cenário distribuído é pulado localmente e reprovado no CI.

As fixtures golden do contrato são o fio publicado: as dos eventos
(`contract/fixtures/event/v1/`) rodam no `identity-contract:test-race`, e as das
projeções (`contract/fixtures/projection/v1/`), nos testes do `provider`.
`GOLDEN_UPDATE=1` regrava as dos eventos, e a diferença resultante é mudança de
contrato, não de teste.

Gates completos do contexto, os mesmos de qualquer módulo:

```bash
pnpm nx run-many -t fmt-check,vet,build,lint,test-race,test-distributed,govulncheck -p identity,identity-contract
DMPF_APPS_DIR=apps/backend bash node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh --context apps/backend/identity
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@v1.0.0-rc.7 --root .
```

## Contrato

O contrato é o módulo [`contract/`](contract/) (projeto `identity-contract`): o
serviço em `proto/company/identity/service/v1/identity_service.proto`, os
eventos em `proto/company/identity/event/v1/` e o OpenAPI em
`openapi/v1/openapi.yaml`. O `.proto` publicado não é regravado; evoluir o
contrato é um rito próprio, conferido pelo `buf-breaking` com as regras `FILE`.

O código gerado em `contract/gen/go/` só nasce pelo rito Buf, nunca à mão,
depois de mudar um `.proto`:

```bash
(cd apps/backend/identity/contract && bash ../../../../node_modules/@mateusmacedo/dmpf-plugin/scripts/buf.sh generate)
pnpm nx run identity-contract:tidy
pnpm nx run-many -t buf-lint,buf-pins,buf-generate-check -p identity-contract
NX_BASE=origin/master pnpm nx run identity-contract:buf-breaking
```

## Deploy

- **Imagem.** O build parte da raiz do repositório:
  `docker build -f apps/backend/identity/Dockerfile -t identity:local .`.
- **Compose local.** No `--profile dmpf` de `infra/local`,
  [`deploy/compose.yml`](deploy/compose.yml) declara os dois papéis em
  container, com mTLS no `api`. A allowlist `GRPC_TRUSTED_CLIENTS` admite só
  o `bff` (`spiffe://dmpf/bff`).
- **Kubernetes.** Os manifestos Kustomize ficam em
  [`deploy/k8s/`](deploy/k8s/), com a base e os overlays `dev` e `hmg`. A chave
  de identificador vem do Secret `identity-identifier-key`, lido só pelo `api`;
  a versão fica no ConfigMap. Nenhum overlay gera esse Secret: crie-o no
  namespace antes do `apply`, na forma do `secrets.example.yaml.tmpl` do
  overlay (`dev` ou `hmg`). No `dev`, com uma chave aleatória de no mínimo 32
  bytes:

  ```bash
  kubectl -n <namespace> create secret generic identity-identifier-key \
    --from-literal=IDENTITY_IDENTIFIER_HMAC_KEY="$(openssl rand -hex 32)"
  ```
- **Infra agregada.** Os arquivos de `infra/` que agregam as apps derivam de
  [`deploy/infra.json`](deploy/infra.json). Mudou o `infra.json`? Regrave:

  ```bash
  go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/infrasync@v1.0.0-rc.7 --root . --write
  ```

## Contribuição

Siga o [`CONTRIBUTING.md`](../../../CONTRIBUTING.md) da raiz (git-flow,
anti-drift) e o [`docs/onboarding.md`](../../../docs/onboarding.md).

- **Commits** em Conventional Commits e PT-BR, com escopo pelo projeto Nx:
  `identity` ou `identity-contract`, um projeto por commit.
- **Antes do PR**, rode os gates completos da seção [Testes](#testes).
- **Passos que são de pessoa, não de agente:**
  - o rito Buf ao mudar `.proto`;
  - o `conformance --write-baseline` quando uma unidade nova muda a
    classificação (`DMPF-T001`);
  - o `infrasync --write` ao mudar o `deploy/infra.json`.
- **Nunca edite à mão** `project.json`, `go.mod`, `go.work`, `dmpf-units.json`
  (só o `include`, por merge), o baseline ou `contract/gen/go/`.

## Documentação relacionada

- Mapa diagramático completo do contexto: [arquitetura, jornadas, estados e sequências](docs/architecture/README.md).
- Spec do contexto:
  [`docs/specs/SPEC-7KW0HHF6-identity-organization.md`](../../../docs/specs/SPEC-7KW0HHF6-identity-organization.md).
- Decisões arquiteturais: [`docs/adr/`](../../../docs/adr/), em especial o
  ADR-013 (tenant), o ADR-016 (identidade e Keycloak) e o ADR-022 (precedência
  do DMPF).
- Normas do bounded context:
  [`.claude/rules/dmpf-bounded-context.md`](../../../.claude/rules/dmpf-bounded-context.md).

## Licença

Distribuído sob a licença MIT. Veja o [`LICENSE`](../../../LICENSE) na raiz do
repositório.
