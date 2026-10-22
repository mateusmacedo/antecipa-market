# partner

Serviço de gestão de parceiros da Plataforma Multiativo de Antecipação de
Recebíveis (repositório `antecipa-market`): guarda, por tenant, os parceiros e
seus estágios, os acordos com sua vigência, as capacidades homologadas e o
rollout de cada uma, os produtos financeiros, os perfis de integração e as
políticas operacionais, e responde se uma capacidade está efetiva para um
pedido.

É o bounded context `partner-management`, especificado em
[`SPEC-9TEQ9EV9`](../../../docs/specs/SPEC-9TEQ9EV9-partner-management.md) e
construído sobre o kernel DMPF `v1.0.0-rc.7`.

## Sumário

- [Visão geral](#visão-geral)
- [Configuração](#configuração)
- [Uso](#uso)
- [Arquitetura](#arquitetura)
- [Testes](#testes)
- [Contrato](#contrato)
- [Deploy](#deploy)
- [Contribuição](#contribuição)
- [Documentação relacionada](#documentação-relacionada)

## Visão geral

- **Serve só gRPC interno** (`company.partner.service.v1.PartnerService`),
  com 14 comandos e 10 consultas. O REST público é do `bff`, que servirá o
  OpenAPI publicado em
  [`contract/openapi/v1/openapi.yaml`](contract/openapi/v1/openapi.yaml).
- **Publica 11 eventos** de integração no canal Kafka `partner.events`, pela
  outbox, e não consome canal nenhum.
- **Isola por tenant**: toda leitura e escrita é escopada pelo tenant do token
  verificado; um identificador de outro tenant responde como inexistente.
  Nesta versão a configuração de parceiros vive num tenant único da
  plataforma, e o `tenantId` dos filtros é o tenant avaliado no rollout.
- **Versiona** acordos, capacidades, produtos, perfis de integração e
  políticas operacionais: cada versão é uma linha nova que aponta a anterior
  por `supersedesId`, e as anteriores permanecem.
- **Nega em caso de dúvida**: a capacidade efetiva só é concedida quando todas
  as condições se confirmam, e a negação devolve a condição que falhou.

### Agregados

| Agregado | O que o contexto faz | Série de versões |
| --- | --- | --- |
| `Partner` | Registra o parceiro de uma organização do Identity num papel e conduz o estágio, de `prospect` a `offboarding` | — (único por organização e papel) |
| `PartnerAgreement` | Registra o acordo em rascunho e o ativa com a evidência do contrato e os critérios contratuais H13 a H18 | `partnerId` |
| `Capability` | Declara a operação sobre um tipo de ativo num ambiente, registra a homologação H1 a H12, habilita, suspende e controla o rollout por tenant | `partnerId`, `operation`, `assetType`, `environment` |
| `FinancialProduct` | Define o produto com faixas de valor e prazo, ativos e regiões, e o retira por vencimento ou retirada | `partnerId`, `code` |
| `IntegrationProfile` | Registra protocolos, endpoint, schema e as referências de segredo e certificado por ambiente, e alerta o vencimento do certificado | `partnerId`, `environment` |
| `OperationalPolicy` | Define limite de taxa, calendário, janela, SLA, contingência e contatos | `partnerId` |

Sem `supersedesId`, o comando abre a série e só é aceito se ela ainda não tem
versão; com ele, só se apontar a última versão da mesma série. Fora disso, a
recusa é `<agregado>/supersedes-invalid`.

### Papéis do processo

O mesmo binário roda em dois papéis, escolhidos por `--role`:

| Papel | O que faz |
| --- | --- |
| `api` | Serve o `PartnerService` por gRPC e purga as entradas de comando vencidas da inbox |
| `relay` | Drena a outbox para `partner.events` e purga a outbox publicada |

`--role consumer` é recusado, porque o contexto não consome canal.

## Configuração

A configuração vem só do ambiente, sem prefixo `DMPF_`, pela forma
`Defaults(role)`, `FromEnv` e `Validate` de [`app/config.go`](app/config.go).
Os targets `serve-*` leem `deploy/.env.example` e, por cima, `deploy/.env`.

| Variável | Papel | Efeito |
| --- | --- | --- |
| `PG_DSN` | todos, obrigatória | Banco do contexto, com a role de runtime `partner`. No `api`, ganha `lock_timeout=2s` e `idle_in_transaction_session_timeout=30s` quando não os traz (guarda do ADR-067; o valor da DSN prevalece) |
| `GRPC_INSECURE` ou `GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE`, `GRPC_CLIENT_CA_FILE` e `GRPC_TRUSTED_CLIENTS` | `api`, obrigatória uma das políticas | `true` dispensa o TLS, só em desenvolvimento; senão, certificado do servidor e autenticação do chamador por mTLS |
| `GRPC_ADDR` | `api` | Default `:9090` no código; o `deploy/.env.example` usa `127.0.0.1:9195` |
| `MIGRATE` e `PG_MIGRATE_DSN` | `api` | Com `MIGRATE=true`, a role migradora `partner_migrator` de `PG_MIGRATE_DSN`, obrigatória nesse caso, aplica o schema e os grants antes de servir |
| `METRIC_TENANTS` | `api` | Tenants com balde de admissão e rótulo próprios |
| `KAFKA_BROKERS`, `KAFKA_PARTNER_TOPIC`, `KAFKA_PARTNER_DLQ`, `KAFKA_GROUP` | `relay`, obrigatórias | Brokers, tópico, contenção e grupo do canal `partner.events` |
| `KAFKA_SASL_MECHANISM` (com `KAFKA_SASL_USERNAME` e `KAFKA_SASL_PASSWORD`) ou `KAFKA_CLIENT_CERT_FILE` (com `KAFKA_CLIENT_KEY_FILE`) | `relay`, obrigatória sem `KAFKA_INSECURE` | Autenticação do cliente no broker; `KAFKA_CA_FILE` fixa a CA |
| `KAFKA_INSECURE` | `relay` | `true` desliga o TLS e dispensa a autenticação, só em desenvolvimento |
| `LOG_LEVEL` | todos | Default `info` |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` | todos | Identidade do recurso OTel; sem `OTEL_SERVICE_NAME`, o serviço é `partner`. `service.version` e `service.instance.id` não têm default, e sem eles a partida falha com `ErrResourceIncomplete` (exit 1) |
| `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_LOGS_EXPORTER`, `OTEL_PROPAGATORS`, `OTEL_GO_X_OBSERVABILITY` | todos | Exportação OTLP; propagador diferente de `tracecontext` recusa a partida |

A recusa de configuração encerra com exit 2: variável obrigatória ausente
(`ErrMissingVariable`, nomeando a primeira que falta), valor inválido
(`ErrInvalidVariable`) ou papel desconhecido. A falha depois da partida
encerra com exit 1. O `PG_MIGRATE_DSN` vem do `deploy/.env.migrator.example`,
que só o `serve-api` e o `docker:run` carregam: a credencial da migradora não
chega ao `relay`.

O `api` abre dois pools sobre o `PG_DSN`: um para as transações e outro para
as leituras fora delas, onde correm as consultas e a pré-leitura da cabeça de
cada série de versões. As condições que dependem de outro agregado (†) leem na
transação, sob a guarda. Cada pool segue o `pool_max_conns`
do DSN, ou o default do pgx, então o processo abre até o dobro desse valor.

Os prazos de idempotência e de purga vêm de `Defaults()`, sem variável de
ambiente:

| Campo | Padrão | Uso |
| --- | --- | --- |
| `IdempotencyWait` | 1s | Espera máxima por comando concorrente da mesma chave |
| `IdempotencyRetention` | 24h | Vida da entrada de comando na inbox |
| `OutboxRetention` | 168h | Idade a partir da qual a outbox publicada é purgada |
| `PurgeInterval`, `PurgeBatch` | 15min, 1000 | Intervalo e lote de cada purga |

O relay tenta publicar cada evento até 24 vezes, com backoff exponencial de
200 ms até o teto de 5 min; toda falha de publicação conta como tentativa,
inclusive o broker fora do ar. Esgotadas as tentativas, o registro vai para
`failed`, estado terminal que exige retomada manual.

### Diretório de organizações (fail-closed)

`RegisterPartner` consulta a organização no Identity pela porta
`ports.OrganizationDirectory`, no `Run` do caso de uso, antes de
`usecase.Decide`. O cliente gRPC do Identity ainda não existe, e o composition
root liga em produção o
[`provider.UnconfiguredOrganizationDirectory`](provider/organization_directory.go),
que responde `ErrOrganizationDirectoryUnconfigured` a toda organização.

Por isso, em produção, todo `RegisterPartner` falha como `INTERNAL`, sem linha
nem evento: nenhum parceiro é registrado sobre uma organização que ninguém
conferiu. Os demais comandos e todas as consultas funcionam normalmente. Os
testes injetam um diretório próprio por `app.NewService`. O provider real é
item do Escopo fora da spec.

## Uso

### Serviço gRPC

O `PartnerService` tem 24 métodos. A rota é a do OpenAPI publicado para o
`bff`, e a permissão é a que [`app/authorization.go`](app/authorization.go)
declara para a operação.

| Agregado | Método | Tipo | Rota no OpenAPI | Permissão |
| --- | --- | --- | --- | --- |
| `Partner` | `RegisterPartner` | comando | `POST /partner/partners` | `partner:register-partner` |
| | `ChangePartnerStage` | comando | `POST /partner/partners/{partnerId}/stage` | `partner:change-partner-stage` |
| | `FindPartner` | consulta | `GET /partner/partners/{partnerId}` | `partner:find-partner` |
| | `FindPartnersByOrganization` | consulta | `GET /partner/partners` | `partner:find-partners-by-organization` |
| | `FindEligiblePartners` | consulta | `GET /partner/eligible-partners` | `partner:find-eligible-partners` |
| `PartnerAgreement` | `RegisterAgreement` | comando | `POST /partner/partners/{partnerId}/agreements` | `partner:register-agreement` |
| | `ActivateAgreement` | comando | `POST /partner/agreements/{agreementId}/activate` | `partner:activate-agreement` |
| | `FindAgreementsByPartner` | consulta | `GET /partner/partners/{partnerId}/agreements` | `partner:find-agreements-by-partner` |
| `Capability` | `DeclareCapability` | comando | `POST /partner/partners/{partnerId}/capabilities` | `partner:declare-capability` |
| | `RecordHomologation` | comando | `POST /partner/capabilities/{capabilityId}/homologation` | `partner:record-homologation` |
| | `EnableCapability` | comando | `POST /partner/capabilities/{capabilityId}/enable` | `partner:enable-capability` |
| | `SuspendCapability` | comando | `POST /partner/capabilities/{capabilityId}/suspend` | `partner:suspend-capability` |
| | `ChangeRollout` | comando | `POST /partner/capabilities/{capabilityId}/rollout` | `partner:change-rollout` |
| | `FindCapabilitiesByPartner` | consulta | `GET /partner/partners/{partnerId}/capabilities` | `partner:find-capabilities-by-partner` |
| | `FindEffectiveCapability` | consulta | `GET /partner/effective-capability` | `partner:find-effective-capability` |
| `FinancialProduct` | `DefineProduct` | comando | `POST /partner/partners/{partnerId}/products` | `partner:define-product` |
| | `RetireProduct` | comando | `POST /partner/products/{productId}/retire` | `partner:retire-product` |
| | `FindProductsByPartner` | consulta | `GET /partner/partners/{partnerId}/products` | `partner:find-products-by-partner` |
| `IntegrationProfile` | `RegisterIntegrationProfile` | comando | `POST /partner/partners/{partnerId}/integration-profiles` | `partner:register-integration-profile` |
| | `RaiseCertificateExpiryAlert` | comando | `POST /partner/integration-profiles/{profileId}/certificate-expiry-alerts` | `partner:raise-certificate-expiry-alert` |
| | `FindIntegrationProfile` | consulta | `GET /partner/partners/{partnerId}/integration-profiles/{environment}` | `partner:find-integration-profile` |
| | `FindCertificatesExpiring` | consulta | `GET /partner/expiring-certificates` | `partner:find-certificates-expiring` |
| `OperationalPolicy` | `DefineOperationalPolicy` | comando | `POST /partner/partners/{partnerId}/operational-policies` | `partner:define-operational-policy` |
| | `FindOperationalPolicy` | consulta | `GET /partner/partners/{partnerId}/operational-policy` | `partner:find-operational-policy` |

`SuspendCapability` não recebe `trigger`: o RPC suspende sempre como
`manual`, e `schema_mismatch` e `offboarding` ficam para fluxos internos.
`RaiseCertificateExpiryAlert` e `RetireProduct` com `reason = expired` vêm de
chamador externo, com chave determinística que leva o dia UTC da avaliação
(`cert-alert.<profileId>.<thresholdDays>.<yyyymmdd>` e
`product-expired.<productId>.<yyyymmdd>`): a repetição no mesmo dia devolve a
resposta gravada, e a do dia seguinte decide de novo sobre o estado.

**Respostas.** Os handlers validam só a forma antes do domínio: ids em
`^[A-Za-z0-9._:-]{1,128}$`, salvo `profileId` de `RegisterIntegrationProfile` e
`RaiseCertificateExpiryAlert` e `productId` de `DefineProduct` e
`RetireProduct`, em `^[A-Za-z0-9._-]{1,100}$` para caber na chave
determinística; enums conhecidos, instantes em nanossegundos não negativos e
listas dentro do teto. A entrada malformada volta como
`INVALID_ARGUMENT`. Uma recusa de domínio volta como `rejection` (`code`,
`message`) na própria resposta, com código `partner-management/<agregado>/<rejeicao>`.
Uma falha do caso de uso vira status gRPC por `kernelgrpc.StatusOf`, entre eles
`NOT_FOUND`, `ABORTED` para conflito de versão e para a espera pela guarda além
do `lock_timeout` (contorno até a DEVS-157), `PERMISSION_DENIED`,
`DEADLINE_EXCEEDED`, `CANCELLED` e `INTERNAL`. Numa corrida pela mesma
organização e papel, ou pela mesma posição de uma série de versões, o índice
único decide: uma chamada grava e a outra recebe a `rejection` declarada
(`already-registered` ou `supersedes-invalid`), sem linha nem evento.

### Consultas

| Consulta | Filtros | Resposta |
| --- | --- | --- |
| `FindPartner` | `partnerId` | O parceiro |
| `FindPartnersByOrganization` | `organizationId` | Os parceiros da organização, um por papel |
| `FindAgreementsByPartner` | `partnerId` | Todas as versões de acordo do parceiro |
| `FindCapabilitiesByPartner` | `partnerId`, `environment` e `status` opcionais | As capacidades do parceiro |
| `FindProductsByPartner` | `partnerId`, `asOf` | A versão vigente de cada produto |
| `FindIntegrationProfile` | `partnerId`, `environment`, `asOf` | A versão vigente do perfil no ambiente |
| `FindOperationalPolicy` | `partnerId`, `asOf` | A versão vigente da política |
| `FindCertificatesExpiring` | `environment`, `expiresBefore` | A versão vigente de cada série `partnerId+environment` no instante da consulta cujo certificado vence antes de `expiresBefore`, refinada em memória |
| `FindEffectiveCapability` | `partnerId`, `operation`, `assetType`, `environment`, `tenantId`, `asOf` e, opcionais, `productId`, `amountMinor`, `termDays` | Permitido, ou negado com a `failedCondition` |
| `FindEligiblePartners` | `assetType`, `operation`, `environment`, `tenantId`, `amountMinor`, `termDays`, `region`, `asOf` | Os financiadores elegíveis ao pedido |

A capacidade efetiva é: parceiro `active`, acordo vigente no instante, versão
vigente da capacidade em `enabled`, produto vigente (só com `productId`),
tenant incluído no rollout e parâmetros compatíveis. A avaliação segue a ordem
de `failedCondition`: `capability_not_found`, `partner_not_active`,
`agreement_not_effective`, `capability_not_enabled`, `product_not_effective`,
`not_in_rollout` e `parameters_incompatible`. Uma leitura que falha devolve o
erro, nunca uma permissão, e o consumidor nega o que não confirmou. O tenant
cai num de 100 baldes por SHA-256 de `capabilityId:tenantId`
([`provider/rollout_bucket.go`](provider/rollout_bucket.go)), então a mesma
avaliação dá sempre o mesmo resultado.

### Idempotência dos comandos

Os 14 comandos exigem a metadata `idempotency-key`; as consultas não leem
chave.

| Situação | Resposta |
| --- | --- |
| Metadata ausente | `INVALID_ARGUMENT`, reason `MISSING_IDEMPOTENCY_KEY` |
| Chave fora do formato | `INVALID_ARGUMENT`, reason `INVALID_IDEMPOTENCY_KEY` |
| Mesma chave, mesmo pedido | A resposta gravada, aceite ou recusa, com o header `idempotent-replayed: true` |
| Mesma chave, outro pedido | `FAILED_PRECONDITION`, reason `REUSED_IDEMPOTENCY_KEY` |
| Mesma chave em andamento além da espera | `ABORTED`, reason `IN_FLIGHT_IDEMPOTENCY_KEY` |
| Comando de criação sobre id que já existe, com outra chave | `ALREADY_EXISTS` |

A repetição com chave nova decide de novo sobre o estado atual.
`ActivateAgreement` trata `satisfiedCriteria` como conjunto: os mesmos
critérios em outra ordem são o mesmo pedido.

### Eventos publicados

Os eventos ficam em `contract/proto/company/partner/event/v1/`, package
`company.partner.event.v1`. Vão para a outbox na mesma transação do estado, e
o relay monta o envelope CloudEvents na publicação, com `partitionkey` igual ao
id do agregado de origem.

| Evento | Tipo CloudEvents | Emitido por |
| --- | --- | --- |
| `PartnerRegistered` | `com.company.partner.partner-registered.v1` | `RegisterPartner` |
| `PartnerOffboardingStarted` | `com.company.partner.partner-offboarding-started.v1` | `ChangePartnerStage`, ao entrar em `offboarding` |
| `PartnerAgreementActivated` | `com.company.partner.partner-agreement-activated.v1` | `ActivateAgreement` |
| `CapabilityEnabled` | `com.company.partner.capability-enabled.v1` | `EnableCapability` |
| `CapabilitySuspended` | `com.company.partner.capability-suspended.v1` | `SuspendCapability` |
| `CapabilityRolloutChanged` | `com.company.partner.capability-rollout-changed.v1` | `ChangeRollout` |
| `PartnerProductDefined` | `com.company.partner.partner-product-defined.v1` | `DefineProduct`, na primeira versão |
| `PartnerProductChanged` | `com.company.partner.partner-product-changed.v1` | `DefineProduct`, nas versões seguintes |
| `PartnerProductExpired` | `com.company.partner.partner-product-expired.v1` | `RetireProduct` com `reason = expired` |
| `PartnerProductRetired` | `com.company.partner.partner-product-retired.v1` | `RetireProduct` com `reason = withdrawn` |
| `PartnerCertificateExpiring` | `com.company.partner.partner-certificate-expiring.v1` | `RaiseCertificateExpiryAlert` |

`RegisterAgreement`, `DeclareCapability`, `RecordHomologation`,
`RegisterIntegrationProfile` e `DefineOperationalPolicy` não publicam evento,
nem a suspensão do parceiro, que só aparece em `FindEffectiveCapability`
(`partner_not_active`). Nenhum evento leva segredo nem os contatos da política
operacional: segredo e certificado entram só por referência (`secretRef`,
`certificateRef`). O canal usa os tópicos `partner.events` e
`partner.events.dlq`, declarados em [`deploy/infra.json`](deploy/infra.json).

### Autorização e tenant

A permissão `partner:<operação>` é conferida no `bff`. No salto gRPC, o
contexto chega sem sujeito, e o `partner` exige só o tenant. Uma operação sem
linha em `permissionOf` é negada. O tenant vem só do token verificado. O papel
`api` só atende o workload que a CA assinou e que está na allowlist
(`GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS`), salvo com `GRPC_INSECURE=true`.

## Arquitetura

### Estrutura do projeto

```text
apps/backend/partner/
├── domain/        agregados, UPRs, códigos de rejeição e a capacidade efetiva
├── ports/         leitores por relação, OrganizationDirectory e RolloutBucket
├── application/   casos de uso (comandos e consultas)
├── provider/      Postgres, mapper de eventos, balde de rollout, diretório e schema.sql
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
real de imports, e o `depguard` reforça as regras por bloco no lint. Onde um
arquivo importa o kernel e o contexto, o kernel recebe alias pelo papel
(`kernel`, `usecase`, `port`, `kernelgrpc`, `kernelapp`).

### Banco de dados

Banco próprio: database `partner`, schema `public`, com a role migradora
`partner_migrator` como dona e a role de runtime `partner` só com os grants
de [`provider.Grants`](provider/schema.go) (`postgres.Mutable` por tabela).
`tenant_id` vai à frente da chave, e a persistência é híbrida: coluna tipada
só para o que uma consulta filtra ou para uma restrição única, o resto do
estado em `snapshot` `jsonb`, onde vive também a versão de domínio. O DDL vive
em [`provider/schema.sql`](provider/schema.sql).

| Tabela | Colunas tipadas além da chave | Unicidade declarada por tenant |
| --- | --- | --- |
| `partners` | `organization_id`, `role`, `status` | `(organization_id, role)` → `partner/already-registered` |
| `partner_agreements` | `partner_id`, `status`, `supersedes_id` | v1 única por `partner_id` |
| `capabilities` | `partner_id`, `operation`, `asset_type`, `environment`, `status`, `supersedes_id` | v1 única por `partner_id`, `operation`, `asset_type`, `environment` |
| `financial_products` | `partner_id`, `code`, `status`, `valid_until`, `supersedes_id` | v1 única por `partner_id`, `code` |
| `integration_profiles` | `partner_id`, `environment`, `certificate_expires_at`, `supersedes_id` | v1 única por `partner_id`, `environment` |
| `operational_policies` | `partner_id`, `supersedes_id` | v1 única por `partner_id` |

Cada `CONSTRAINT <tabela>_<colunas>_key UNIQUE` tem um `postgres.Unique` de
mesmo nome no provider (ADR-066 do kernel), e a partida confere a declaração
no catálogo (`VerifyUniques`). O `Save` devolve a violação como a rejeição
declarada, e a requisição perdedora de uma corrida recebe a recusa, sem linha
nem evento. Nas cinco tabelas versionadas, `UNIQUE (tenant_id, supersedes_id)`
impede duas versões sobre a mesma anterior, e a coluna gerada `series_origin`,
com a chave da série na v1 e `NULL` nas demais, impede duas v1 na mesma série
com `UNIQUE (tenant_id, series_origin)`. `partner_agreements`,
`integration_profiles` e `capabilities` declaram `Guard: partner_id`: as
pré-condições (†) leem a série do parceiro na UoW sob lock compartilhado, e
todo `Save` dessas tabelas toma o exclusivo da mesma chave (ADR-067 do
kernel). Do kernel vêm `outbox`, `inbox` e `quarantine`.

O `postgres.Migrate` é idempotente e não tem tabela de versão: um database
`partner` criado antes do kernel `1.0.0-rc.7` mantém os índices parciais
antigos e não tem `series_origin`, e a partida recusa com
`ErrUniqueDivergent`. Fora de produção, faça backup e recrie só o database
`partner`, com a role `partner_migrator` como dona, sem apagar o volume do
Postgres, que identity e bff também usam. A próxima partida com `MIGRATE=true`
aplica o schema novo.

### Dependências

O `go.mod` declara o kernel por versão (`v1.0.0-rc.7`). Dentro do workspace,
quem resolve é o `go.work` da raiz. O `require` do módulo de contrato irmão é
gravado pelo `modsync`, que deve rodar de novo a cada import novo de módulo do
workspace:

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/modsync@v1.0.0-rc.7 --root . --write
```

## Testes

| Target | O que prova |
| --- | --- |
| `test-race` | `go test -race -tags=integration ./...`: UPRs e determinismo do `domain`, sequência canônica, autorização, idempotência e instrumentação da `application`, serviço gRPC, repositórios, leitores e projeções do `provider` e o e2e gRPC até a outbox sobre Postgres (aceites, recusas sem escrita, replay, corridas e outro tenant) |
| `test-distributed` | `distkit` com as tags `integration,distributed`: duas drenagens concorrentes sobre a mesma outbox publicam cada registro exatamente uma vez |
| `e2e` | O mesmo comando do `test-distributed`, sem depender do `test-race` |

```bash
pnpm nx run partner:test-race
pnpm nx run partner:test-distributed
```

Os targets com banco sobem a infra de testes (`test-infra-up`) e preenchem
`PG_DSN` (15432) e `KAFKA_BROKERS` (19092) quando o ambiente não os declara.
Cada teste usa um banco `partner_test_<id>` próprio.

As fixtures golden do contrato são o fio publicado: as dos eventos
(`contract/fixtures/event/v1/`) rodam no `partner-contract:test-race`, e as das
projeções (`contract/fixtures/projection/v1/`), nos testes do `provider`.

Gates completos do contexto, os mesmos de qualquer módulo:

```bash
pnpm nx run-many -t fmt-check,vet,build,lint,test-race,test-distributed -p partner,partner-contract
DMPF_APPS_DIR=apps/backend bash node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh --context apps/backend/partner
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@v1.0.0-rc.7 --root .
```

## Contrato

O contrato é o módulo [`contract/`](contract/) (projeto `partner-contract`): o
serviço em `proto/company/partner/service/v1/partner_service.proto`, os
eventos em `proto/company/partner/event/v1/` e o OpenAPI em
`openapi/v1/openapi.yaml`. O `.proto` publicado não é regravado; evoluir o
contrato é um rito próprio, conferido pelo `buf-breaking`.

O código gerado em `contract/gen/go/` só nasce pelo rito Buf, nunca à mão,
depois de mudar um `.proto`:

```bash
(cd apps/backend/partner/contract && bash ../../../../node_modules/@mateusmacedo/dmpf-plugin/scripts/buf.sh generate)
pnpm nx run partner-contract:tidy
pnpm nx run-many -t buf-lint,buf-pins,buf-generate-check -p partner-contract
NX_BASE=origin/master pnpm nx run partner-contract:buf-breaking
```

## Deploy

- **Imagem.** O build parte da raiz do repositório:
  `docker build -f apps/backend/partner/Dockerfile -t partner:local .`.
- **Compose local.** [`deploy/compose.yml`](deploy/compose.yml) declara os
  dois papéis em container; o `PG_MIGRATE_DSN` vai só ao serviço
  `partner-api`.
- **Kubernetes.** Os manifestos Kustomize ficam em
  [`deploy/k8s/`](deploy/k8s/), com a base e os overlays `dev` e `hmg`; a
  credencial da migradora vem do Secret `partner-migrator`, que só o
  Deployment `partner-api` referencia.
- **Infra agregada.** Os arquivos de `infra/` que agregam as apps derivam de
  [`deploy/infra.json`](deploy/infra.json). Mudou o `infra.json`? Regrave:

  ```bash
  go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/infrasync@v1.0.0-rc.7 --root . --write
  ```

## Contribuição

Siga o [`CONTRIBUTING.md`](../../../CONTRIBUTING.md) da raiz e o
[`docs/onboarding.md`](../../../docs/onboarding.md).

- **Commits** em Conventional Commits e PT-BR, com escopo pelo projeto Nx:
  `partner` ou `partner-contract`, um projeto por commit.
- **Passos que são de pessoa, não de agente:** o rito Buf ao mudar `.proto`;
  o `conformance --write-baseline` quando uma unidade nova muda a
  classificação (`DMPF-T001`); o `infrasync --write` ao mudar o
  `deploy/infra.json`.
- **Nunca edite à mão** `project.json`, `go.mod`, `go.work`, `dmpf-units.json`
  (só o `include`, por merge), o baseline ou `contract/gen/go/`.

## Documentação relacionada

- Spec do contexto:
  [`docs/specs/SPEC-9TEQ9EV9-partner-management.md`](../../../docs/specs/SPEC-9TEQ9EV9-partner-management.md).
- Decisões arquiteturais: [`docs/adr/`](../../../docs/adr/), em especial o
  ADR-013 (tenant) e o ADR-022 (precedência do DMPF).
- Normas do bounded context:
  [`.claude/rules/dmpf-bounded-context.md`](../../../.claude/rules/dmpf-bounded-context.md).
