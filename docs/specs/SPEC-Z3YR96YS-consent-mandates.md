---
id: SPEC-Z3YR96YS
slug: consent-mandates
title: Bounded context — consent-mandates
stage: backlog
priority: P1
depends_on: [SPEC-GXGASP24, SPEC-N1X148AW]
ticket_url: https://linear.app/mmda/issue/DEVS-85/spec-z3yr96ys-bounded-context-consent-mandates
subtask_urls: []
created: 2026-10-12
---

# SPEC-Z3YR96YS: Bounded context — consent-mandates

## Identidade

- **bounded_context**: `consent-mandates`
- **name**: `consent`
- **Linguagem ubíqua**:
  - *Instrumento* — um dos quatro do PRD: `DataProcessingBasis` (base a registrar, não consentimento), `DataSharingConsent`, `ReceivablesOptIn` e `OperationalMandate`; cada um com ciclo de vida próprio.
  - *Concessão* (`ConsentGrant`) — autorização versionada de um instrumento; cada renovação é uma nova versão.
  - *Escopo* (`ConsentScope`) — tipos de ativo, finalidades, destinatários nominais, categorias de dados e período.
  - *Escopo efetivo* — interseção do escopo interno com a autorização externa; vazio enquanto a autorização externa não existe.
  - *Aceite forte* — aceite do representante com poder vigente, reautenticação, texto integral apresentado e ação afirmativa.
  - *Revogação* — bloqueio interno imediato, propagação a cada destinatário e acompanhamento da confirmação.
  - *Autorização externa* (`ExternalAuthorizationReference`) — identificador mantido pela infraestrutura para o opt-in.
  - *Posição de representação* (`RepresentationStanding`) — projeção local da `Representation` de Identity, usada para validar o poder do concedente.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/03-prd-consent-mandates.md`; `docs/design/03-design-consent-mandates.md` §2 (instrumentos), §3 (aceite forte), §4 (escopo efetivo), §5 (sincronização e revogação), §6 (renovação), §7 (estrutura do instrumento); `docs/design/01-design-identity-organization.md` §6 (consentimento por estabelecimento recebedor); `docs/design/04-design-receivables-gateway.md` §3.2 (`AuthorizationPort`); `docs/design/92-schemas-eventos.md` §3.1 e §4; `docs/prd/14-catalogo-contratos-eventos.md`; `docs/reference/glossario.md` (Opt-in).
- **Situação**: `parcial` — consentimento interno, aceite forte, escopo efetivo e revogação interna são construíveis; a mecânica externa do opt-in, os prazos e os textos jurídicos dependem de apuração.
- **Fase de entrega**: 1.
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-N1X148AW`, Receivables Gateway: `AuthorizationPort` com `activateOptIn` e `revokeOptIn`); a cadeia identity → partner → gateway → consent fica implícita, e Identity (`RepresentationVerified`, `RepresentationRevoked`) entra por ela. Branda: Partner Management (capacidades externas). A retroalimentação — Gateway consome `ConsentGranted` e `ConsentRevoked` — nunca entra em `depends_on`.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| #1 / A1 | autorização regulatória da plataforma (transversal) | existencial para o contexto inteiro |
| #6, A4, 03:K8 | natureza jurídica e mecânica de ativação do opt-in; prazos | `ActivateExternalOptIn`, prazos de propagação, `ExpireConsent` |
| #6, B12, 03:K12 | revogação parcial de opt-in é suportada? | renovação com escopo reduzido; revogação parcial |
| 03:K9 | prazo máximo de validade da autorização | limite de `periodUntil` |
| A12, 03:K10 | base legal e textos jurídicos dos instrumentos | `legalBasis`; conteúdo do documento apresentado |
| 03:K11 | viabilidade de consentimento por categoria de financiador | destinatários nominais (hoje obrigatórios) |

- **Decisões de produto abertas**: valor do limite de retomada da sincronização externa (`docs/design/03` §5.2, "após limite"); consentimento por estabelecimento recebedor × `ConsentGranted` do doc 92 (em Agregados); sinal de revogação sem confirmação além do prazo (em Integração).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída (inclui D4, o broker); `SPEC-N1X148AW` com a `AuthorizationPort` e o simulador disponíveis. O núcleo (instrumentos, aceite forte, escopo efetivo, revogação interna e propagação) não espera #6; a ativação real do opt-in espera.

## Agregados

Decisão de modelagem: o `Mandate` do PRD é uma `ConsentGrant` com `consentType = operational_mandate`, coerente com o doc 92 §3.1 (um `ConsentGranted` por instrumento, discriminado por `consentType`). `ConsentScope`, `ConsentEvidence`, `Revocation` e `ExternalAuthorizationReference` são partes da `ConsentGrant`: o escopo efetivo, a prova e o acompanhamento da revogação são decididos atomicamente.

### ConsentGrant

- **Identidade**: `generated`; versionada — cada renovação é uma linha nova com `version` e `supersedesId`, e a versão anterior nunca é alterada (prova do que valia em cada período).
- **Campos**:
  - `organizationId: id`
  - `establishmentId: id` **[ABERTA: docs/design/01-design-identity-organization.md §6 regra 4 manda o consentimento ser por estabelecimento recebedor, mas docs/design/92-schemas-eventos.md §3.1 não carrega o estabelecimento em `ConsentGranted`]**
  - `consentType: data_processing_basis | data_sharing | receivables_opt_in | operational_mandate`
  - `version: int`, `supersedesId: id` (opcional), `renewalMode: none | simplified | full`
  - `scope`: `assetTypes: list<string>`, `purposes: list<string>`, `recipientOrganizationIds: list<id>`, `dataCategories: list<string>`, `periodFrom: instant`, `periodUntil: instant` — as listas são coleções indispensáveis do escopo; as datas puras do design viram `instant` UTC
  - `powers: list<string>` — poderes operacionais, só em `operational_mandate`
  - `legalBasis: string` e `revocable: bool` — só em `data_processing_basis` **[A CONFIRMAR: A12, 03:K10]**
  - `evidence`: `documentVersion: string`, `documentHash: string`, `presentation: full | simplified`, `presentedAt: instant`, `channel: string`, `authenticationMethod: string`, `acceptedAt: instant`, `originAddress: string` (PII), `evidenceRef: id` **[ABERTA: provider de object storage]**
  - `grantedByRepresentationId: id`
  - `status: draft | pending_acceptance | active | revocation_pending | revoked | expired | rejected | external_sync_failed`
  - `externalAuthorization` (opcional, só `receivables_opt_in`): `providerId: id`, `system: string`, `reference: string`, `asOf: instant` e o `scope` externo, na mesma forma
  - `effectiveScope` — mesma forma de `scope`; vazio quando não há escopo efetivo
  - `syncRetryLimitAt: instant` (opcional)
  - `revocation` (opcional): `requestedAt: instant`, `effectiveAt: instant`, `requestedByRepresentationId: id`, `reason: string`, `confirmationDeadlineAt: instant`, `propagation: list<RecipientPropagation>` (`recipientKind: infrastructure | funder`, `recipientId: id`, `status: pending | confirmed`, `confirmedAt: instant` opcional), `activeContractIds: list<id>` — a lista por destinatário é indispensável: a revogação fica visível até a confirmação de cada um
- **Estados**: os oito de `status`.
- **Transições permitidas**:
  - (novo) → `draft` — por `DraftConsent`
  - `draft` → `pending_acceptance` — por `PresentConsent`
  - `pending_acceptance` → `active` — por `GrantConsent`
  - `pending_acceptance` → `rejected` — por `DeclineConsent`
  - `active` | `external_sync_failed` → `external_sync_failed` — por `RecordExternalSyncFailure`
  - `external_sync_failed` → `active` — por `ActivateExternalOptIn` (retomada sem novo aceite, texto inalterado)
  - `active` | `external_sync_failed` → `revocation_pending` — por `RequestRevocation`
  - `revocation_pending` → `revoked` — por `ConfirmRevocation` do último destinatário
  - `active` | `external_sync_failed` → `expired` — por `ExpireConsent`
- **Invariantes de estado**:
  - `effectiveScope` é a interseção de `scope` com o escopo da `externalAuthorization` para `receivables_opt_in`: conjuntos por interseção, período em `[max(inícios), min(fins)]`, vazio quando o início passa o fim. Sem autorização externa, em `external_sync_failed`, `revocation_pending`, `revoked`, `expired`, `rejected`, `draft` e `pending_acceptance`, é vazio. Escopo externo que o modelo não representa também o esvazia. Para os demais instrumentos, é o próprio `scope` enquanto `active`.
  - Durante a sincronização o consentimento interno existe, mas o escopo efetivo é vazio ("autorização em ativação").
  - O texto exibido e o aceite são preservados por hash e versão; o `documentHash` aceito é o apresentado.
  - O bloqueio interno da revogação é síncrono: `RequestRevocation` esvazia `effectiveScope` na própria transação, antes de qualquer propagação.
  - A revogação não desfaz contratos vigentes, não elimina evidência e não interrompe retenção.
  - Um mandato nunca concede poderes maiores que os do representante.
  - Autorização expirada não é renovada automaticamente; renovar é um novo aceite.

### RepresentationStanding

- **Identidade**: `natural` por `representationId`
- **Campos**:
  - `representationId: id`, `organizationId: id`, `powerType: string`
  - `scopeCurrency: string`, `scopeMaxAmountMinor: int`, `scopeActTypes: list<string>` — alçada, opcionais **[ABERTA: docs/design/90-modelo-de-dados.md §5 L3 — formato do limite e tratamento acima dos poderes]**
  - `validFrom: instant`, `validUntil: instant` (opcional)
  - `status: verified | revoked`, `revokedAt: instant` (opcional)
- **Estados**: `verified`, `revoked`
- **Transições permitidas**:
  - (novo) → `verified` | `revoked` — por `RecordRepresentationStanding`
  - `verified` → `revoked` — por `RecordRepresentationStanding`
- **Invariantes de estado**: projeção local dos eventos de Identity; uma revogada nunca volta a verificada. **[ABERTA: docs/design/92-schemas-eventos.md §3 — o schema de `RepresentationVerified` não está definido; o mapa de campos desta spec segue o mínimo derivado de docs/design/90 §3.1]**

## Comandos (UPRs)

(†) marca pré-condição que depende de outro agregado ou contexto: o caso de uso a verifica por leitura ou consulta gRPC antes de decidir. Ver Relações entre agregados.

### DraftConsent

- **Agregado**: `ConsentGrant`; **cria**
- **Comando**: `DraftConsent` — também é a renovação (`RenewConsent` do PRD): com `supersedesId` cria a nova versão; um único comando cria o agregado.
- **Entrada**:
  - `organizationId: id`, `establishmentId: id`
  - `consentType`
  - `scope` (os seis campos) e, quando couber, `powers`, `legalBasis`, `revocable`
  - `supersedesId: id` (opcional) e `renewalMode: simplified | full` — decididos pelo caso de uso: `simplified` com texto e escopo inalterados; `full` com texto alterado ou escopo ampliado
- **Pré-condições**:
  - `data_sharing` e `receivables_opt_in` exigem `assetTypes`, `purposes` e `recipientOrganizationIds` não vazios, destinatários nominais → `consent-mandates/consent-grant/scope-incomplete`
  - `periodFrom` anterior a `periodUntil` → `consent-mandates/consent-grant/invalid-period` (limite máximo de validade **[A CONFIRMAR: #6, 03:K9]**)
  - `operational_mandate` exige `powers` não vazio → `consent-mandates/consent-grant/powers-empty`
  - `data_processing_basis` exige `legalBasis` → `consent-mandates/consent-grant/legal-basis-missing`
  - `supersedesId`, quando informado, referencia uma concessão do mesmo estabelecimento e instrumento (†) → `consent-mandates/consent-grant/superseded-grant-not-found`
- **Efeitos sobre o estado**:
  - `status ← draft`; `effectiveScope ← vazio`
  - `version ← 1`, ou a da concessão substituída mais 1; `renewalMode ← none` sem `supersedesId`
- **Eventos emitidos**: nenhum
- **Resposta**: `consentId: id`, `version: int`

### PresentConsent

**[ABERTA: docs/prd/03-prd-consent-mandates.md Comandos — o PRD não lista o comando; docs/design/03 §3 exige o texto integral exibido antes do aceite e o estado `PENDING_ACCEPTANCE` o pressupõe]**

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `PresentConsent`
- **Entrada**:
  - `documentVersion: string`
  - `documentHash: string`
  - `presentation: full | simplified`
- **Pré-condições**:
  - `status = draft` → `consent-mandates/consent-grant/not-draft`
  - `documentHash` não vazio → `consent-mandates/consent-grant/document-hash-missing`
  - `renewalMode = full` exige `presentation = full` → `consent-mandates/consent-grant/full-presentation-required`
- **Efeitos sobre o estado**:
  - `evidence.documentVersion`, `evidence.documentHash`, `evidence.presentation` ← cmd; `evidence.presentedAt ← at`
  - `status ← pending_acceptance`
- **Eventos emitidos**: nenhum
- **Resposta**: `consentId: id`

### GrantConsent

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `GrantConsent`
- **Entrada**:
  - `documentHash: string`
  - `grantedByRepresentationId: id`
  - `channel: string`, `authenticationMethod: string`, `originAddress: string`, `evidenceRef: id`
  - `explicitAction: bool`, `reauthenticated: bool`
- **Pré-condições** (as de aceite não se aplicam a `data_processing_basis`, que registra base e não colhe aceite **[A CONFIRMAR: A12, 03:K8]**):
  - `status = pending_acceptance` → `consent-mandates/consent-grant/not-pending-acceptance`
  - `documentHash` igual ao apresentado → `consent-mandates/consent-grant/document-mismatch`
  - `explicitAction` verdadeiro (caixa pré-marcada não é aceite) → `consent-mandates/consent-grant/acceptance-not-explicit`
  - `reauthenticated` verdadeiro, mesmo com sessão ativa → `consent-mandates/consent-grant/reauthentication-missing`
  - representação verificada, vigente e com alçada para o ato (†) → `consent-mandates/consent-grant/representation-invalid`
  - `operational_mandate`: `powers` dentro dos poderes da representação (†) → `consent-mandates/consent-grant/mandate-exceeds-representation`
- **Efeitos sobre o estado**:
  - `status ← active`; `grantedByRepresentationId ← cmd`
  - `evidence` ← campos do comando; `evidence.acceptedAt ← at`
  - `effectiveScope ← scope` para os instrumentos sem autorização externa; vazio para `receivables_opt_in` até a ativação externa
- **Eventos emitidos**: `ConsentGranted`
- **Resposta**: `consentId: id`

### DeclineConsent

**[ABERTA: docs/prd/03-prd-consent-mandates.md Estados — `REJECTED` existe sem comando nem evento]**

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `DeclineConsent`
- **Entrada**: nenhuma além da identidade do agregado
- **Pré-condições**:
  - `status = pending_acceptance` → `consent-mandates/consent-grant/not-pending-acceptance`
- **Efeitos sobre o estado**:
  - `status ← rejected`
- **Eventos emitidos**: nenhum
- **Resposta**: `consentId: id`

### ActivateExternalOptIn

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `ActivateExternalOptIn` — registra o resultado confirmado da `AuthorizationPort.activateOptIn` do Gateway.
- **Entrada**:
  - `providerId: id`, `system: string`, `reference: string`, `asOf: instant`
  - `externalScope` (mesma forma de `scope`)
- **Pré-condições**:
  - `consentType = receivables_opt_in` → `consent-mandates/consent-grant/not-an-opt-in`
  - `status = active` sem autorização externa, ou `external_sync_failed` → `consent-mandates/consent-grant/not-awaiting-external-activation`
- **Efeitos sobre o estado**:
  - `externalAuthorization ← cmd`
  - `effectiveScope ←` interseção de `scope` com `externalScope`
  - `status ← active`; `syncRetryLimitAt ← vazio`
- **Eventos emitidos**: `ExternalOptInActivated`
- **Resposta**: `consentId: id`

### RecordExternalSyncFailure

**[ABERTA: docs/prd/03-prd-consent-mandates.md Comandos — o estado `EXTERNAL_SYNC_FAILED` e o evento `AuthorizationSyncFailed` existem sem comando]**

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `RecordExternalSyncFailure` — registra a falha definitiva, ou o desconhecido não resolvido, da ativação externa.
- **Entrada**:
  - `reason: provider_failure | scope_not_representable | unknown_outcome_unresolved`
  - `retryLimitAt: instant` **[ABERTA: docs/design/03 §5.2 — o limite não tem valor definido]**
- **Pré-condições**:
  - `consentType = receivables_opt_in` → `consent-mandates/consent-grant/not-an-opt-in`
  - `status` em `active` sem autorização externa, ou `external_sync_failed` → `consent-mandates/consent-grant/not-awaiting-external-activation`
- **Efeitos sobre o estado**:
  - `status ← external_sync_failed`; `syncRetryLimitAt ← cmd.retryLimitAt`; `effectiveScope ← vazio`
- **Eventos emitidos**: `AuthorizationSyncFailed`
- **Resposta**: `consentId: id`

### RequestRevocation

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `RequestRevocation`
- **Entrada**:
  - `requestedByRepresentationId: id`, `reason: string`
  - `confirmationDeadlineAt: instant` (acordo com o destinatário; padrão de 1 dia útil, `docs/design/03` §5.3)
  - `targets: list<{recipientKind, recipientId}>` — a infraestrutura e os financiadores destinatários
- **Pré-condições**:
  - `status` em `active` ou `external_sync_failed` → `consent-mandates/consent-grant/not-revocable`
  - `data_processing_basis` só revoga quando `revocable` → `consent-mandates/consent-grant/basis-not-revocable`
  - representação do solicitante verificada e vigente (†) → `consent-mandates/consent-grant/representation-invalid`
- **Efeitos sobre o estado**:
  - `status ← revocation_pending`; `effectiveScope ← vazio` (bloqueio interno imediato)
  - `revocation ←` {`requestedAt ← at`, `effectiveAt ← at`, `requestedByRepresentationId`, `reason`, `confirmationDeadlineAt`, `propagation ←` um item `pending` por alvo}
- **Eventos emitidos**: `ConsentRevocationRequested`
- **Resposta**: `consentId: id`

### ConfirmRevocation

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `ConfirmRevocation` — registra a confirmação de um destinatário.
- **Entrada**:
  - `recipientKind: infrastructure | funder`, `recipientId: id`
  - `activeContractIds: list<id>` (preenchido pelo caso de uso **[ABERTA: docs/design/92 §3.1 `preservedObligations` exige dado de Contracting; Consent não consome evento de contrato]**)
- **Pré-condições**:
  - `status = revocation_pending` → `consent-mandates/consent-grant/not-revocation-pending`
  - o destinatário consta em `propagation` → `consent-mandates/consent-grant/recipient-unknown`
- **Efeitos sobre o estado**:
  - o item do destinatário: `status ← confirmed`, `confirmedAt ← at` (já confirmado: aceita sem alterar)
  - todos confirmados: `status ← revoked`; `revocation.activeContractIds ← cmd`
- **Eventos emitidos**: `ConsentRevoked` (só quando o último destinatário confirma)
- **Resposta**: `consentId: id`, `status`

### ExpireConsent

**[ABERTA: docs/prd/03-prd-consent-mandates.md Comandos — `ConsentExpired` existe sem comando que o emita]**

- **Agregado**: `ConsentGrant`; exige existente
- **Comando**: `ExpireConsent` — acionado por agendador; nunca renova.
- **Entrada**: nenhuma além da identidade do agregado
- **Pré-condições**:
  - `status` em `active` ou `external_sync_failed` → `consent-mandates/consent-grant/not-expirable`
  - `periodUntil` menor ou igual a `at`, ou `syncRetryLimitAt` menor ou igual a `at` → `consent-mandates/consent-grant/not-yet-expired`
- **Efeitos sobre o estado**:
  - `status ← expired`; `effectiveScope ← vazio`
- **Eventos emitidos**: `ConsentExpired`
- **Resposta**: `consentId: id`

### RecordRepresentationStanding

- **Agregado**: `RepresentationStanding`; **inicializa se ausente**
- **Comando**: `RecordRepresentationStanding`
- **Entrada**:
  - `representationId: id`, `organizationId: id`, `powerType: string`
  - `scopeCurrency`, `scopeMaxAmountMinor`, `scopeActTypes` (opcionais)
  - `validFrom: instant`, `validUntil: instant` (opcional)
  - `status: verified | revoked`, `revokedAt: instant` (opcional)
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**:
  - quando ausente: campos ← cmd
  - quando `verified` e o comando é `revoked`: `status ← revoked`, `revokedAt ← cmd`
  - demais casos, incluindo `revoked` → `verified`: aceita sem alterar
- **Eventos emitidos**: nenhum
- **Resposta**: `representationId: id`

## Eventos de domínio

`EventName()` é `consent-mandates.consent-grant.<evento>`.

### ConsentGranted

- **Campos**:
  - `consentId: id ← ConsentGrant.id`
  - `organizationId: id ← ConsentGrant.organizationId`
  - `establishmentId: id ← ConsentGrant.establishmentId`
  - `consentType ← ConsentGrant.consentType`
  - `version: int ← ConsentGrant.version`
  - `scope ← ConsentGrant.scope` (`assetTypes`, `purposes`, `recipientOrganizationIds`, `dataCategories`, `periodFrom`, `periodUntil`)
  - `grantedByRepresentationId: id ← ConsentGrant.grantedByRepresentationId`
  - `acceptance ←` `channel`, `documentVersion`, `documentHash`, `authenticationMethod`, `acceptedAt`, `evidenceRef` de `ConsentGrant.evidence`
  - `externalAuthorization ←` nulo; preenchido depois por `ExternalOptInActivated`

### ExternalOptInActivated

- **Campos**:
  - `consentId: id ← ConsentGrant.id`
  - `organizationId: id ← ConsentGrant.organizationId`
  - `externalAuthorization ←` `providerId`, `system`, `reference`, `asOf` de `ConsentGrant.externalAuthorization`
  - `effectiveScope ← ConsentGrant.effectiveScope`
  - `at: instant ← at`

### ConsentRevocationRequested

- **Campos**:
  - `consentId: id ← ConsentGrant.id`
  - `organizationId: id ← ConsentGrant.organizationId`
  - `requestedAt: instant ← ConsentGrant.revocation.requestedAt`
  - `requestedByRepresentationId: id ← ConsentGrant.revocation.requestedByRepresentationId`
  - `reason: string ← ConsentGrant.revocation.reason`
  - `propagation ← ConsentGrant.revocation.propagation`

### ConsentRevoked

- **Campos**:
  - `consentId: id ← ConsentGrant.id`
  - `organizationId: id ← ConsentGrant.organizationId`
  - `requestedAt: instant ← ConsentGrant.revocation.requestedAt`
  - `effectiveAt: instant ← ConsentGrant.revocation.effectiveAt`
  - `requestedByRepresentationId: id ← ConsentGrant.revocation.requestedByRepresentationId`
  - `reason: string ← ConsentGrant.revocation.reason`
  - `propagation ← ConsentGrant.revocation.propagation`
  - `preservedObligations.activeContractIds ← ConsentGrant.revocation.activeContractIds`

### ConsentExpired

- **Campos**:
  - `consentId: id ← ConsentGrant.id`
  - `organizationId: id ← ConsentGrant.organizationId`
  - `consentType ← ConsentGrant.consentType`
  - `at: instant ← at`

### AuthorizationSyncFailed

- **Campos**:
  - `consentId: id ← ConsentGrant.id`
  - `organizationId: id ← ConsentGrant.organizationId`
  - `reason ← cmd.reason`
  - `retryLimitAt: instant ← ConsentGrant.syncRetryLimitAt`
  - `at: instant ← at`

## Consultas

- `FindConsentGrant` — filtros: `consentId`; cardinalidade: `one`
- `FindConsentGrantsByOrganization` — filtros: `organizationId`, `establishmentId`, `consentType` e `status` (opcionais); cardinalidade: `many` — autorizações ativas e histórico do cedente
- `FindEffectiveScope` — filtros: `organizationId`, `establishmentId`, `consentType`, `asOf`; cardinalidade: `one` — a consulta de agenda é bloqueada antes da chamada externa quando vazio
- `FindConsentEvidence` — filtros: `consentId`; cardinalidade: `one`
- `FindPendingRevocations` — filtros: `confirmationDeadlineBefore`; cardinalidade: `many` — acompanhamento e escalonamento
- `FindRepresentationStanding` — filtros: `representationId`; cardinalidade: `one`

## Relações entre agregados

- `ConsentGrant.organizationId` → `Organization` (contexto `identity-organization`)
- `ConsentGrant.establishmentId` → `Establishment` (contexto `identity-organization`)
- `ConsentGrant.grantedByRepresentationId` → `RepresentationStanding`
- `ConsentGrant.supersedesId` → `ConsentGrant`
- `ConsentGrant.scope.recipientOrganizationIds` → `Organization` (financiadores)
- `RepresentationStanding.representationId` → `Representation` (contexto `identity-organization`)

As relações são só por identidade. A verificação (†) de existência e vigência do agregado referenciado é do caso de uso, por leitura ou consulta gRPC **[ABERTA: FND-04 §3.2 — o golden `bookings` não lê outro agregado dentro da decisão; validar o padrão na fundação]**.

## Integração

- **Publica** (integration events), em `apps/backend/consent/contract/proto/company/consent/event/v1/`, package `company.consent.event.v1`, no tópico do contexto (Kafka, default do DMPF; confirmação na decisão D4 da fundação):
  - `ConsentGranted` → `consent_granted.proto` — consumidores (doc 92 §4): Gateway, Eligibility, Ledger
  - `ExternalOptInActivated` → `external_opt_in_activated.proto` — Portfolio, Ledger
  - `ConsentRevoked` → `consent_revoked.proto` — Gateway, Eligibility, Marketplace, Ledger
  - `ConsentExpired` → `consent_expired.proto` — Eligibility, Ledger
  - `AuthorizationSyncFailed` → `authorization_sync_failed.proto` — Ops, Ledger
  - `ConsentRevocationRequested` → `consent_revocation_requested.proto` — **[ABERTA: docs/design/92-schemas-eventos.md §4 omite o evento declarado em docs/prd/03-prd-consent-mandates.md; sem consumidor declarado além do Ledger]**
  - Notificações consome `ConsentGranted` e `ConsentRevoked` (`docs/design/16` §4) **[ABERTA: docs/design/92 §4 não lista Notificações como consumidor]**
  - O acesso do financiador à concessão (projeção no tenant do financiador, `docs/design/00` §4.2) **[ABERTA: requer extensão do kernel — ADR-051 DMPF]**
- **Consome**:
  - `company.identity.event.v1.RepresentationVerified` → `RecordRepresentationStanding` com `status = verified`; mapa: `representationId`, `organizationId`, `powerType`, `scopeCurrency`, `scopeMaxAmountMinor`, `scopeActTypes`, `validFrom`, `validUntil` ← mesmos campos do evento.
  - `company.identity.event.v1.RepresentationRevoked` → `RecordRepresentationStanding` com `status = revoked`; mapa: `representationId`, `organizationId` ← mesmos campos; `revokedAt` ← `revokedAt`.
  - Disposições de consumo por referência a FND-04 §6.4; `ErrSchemaMismatch` e `ErrMalformed` do envelope viram `Failure(Validation)` sem abrir UoW. A revogação de uma representação impede novos consentimentos; não invalida os já concedidos.
  - Sincronização externa (saída): o provider chama a `AuthorizationPort` do Gateway (`activateOptIn`, `revokeOptIn`, `queryOptInStatus`) e traduz o resultado em `ActivateExternalOptIn`, `RecordExternalSyncFailure` ou `ConfirmRevocation`. Resultado desconhecido: consultar o estado, nunca reenviar às cegas. **[ABERTA: ciclo de comando externo sem suporte no kernel]**
  - Partner Management fornece capacidades externas (`docs/prd/03` Dependências) **[ABERTA: docs/design/92 §4 não declara Consent consumidor de `CapabilityEnabled` nem de `CapabilitySuspended`]**.
  - Revogação sem confirmação além do prazo abre caso operacional (`docs/design/03` §5.3) **[ABERTA: o catálogo não tem evento para esse sinal; `FindPendingRevocations` cobre a detecção]**.

## Borda e persistência

- **Borda**: só gRPC, `company.consent.service.v1.ConsentService`, um RPC por comando (`DraftConsent`, `PresentConsent`, `GrantConsent`, `DeclineConsent`, `ActivateExternalOptIn`, `RecordExternalSyncFailure`, `RequestRevocation`, `ConfirmRevocation`, `ExpireConsent`, `RecordRepresentationStanding`) e por consulta, atrás da cadeia de interceptors do kernel. As rotas REST (`POST` e `GET /organizations/{id}/consents`, `POST /consents/{id}/revocations`, `GET /consents/{id}/evidence`) ficam no `bff`.
- **Banco**: database e role `consent`, schema `public`, tabelas `consent_grants` e `representation_standings`, com `tenant_id` à frente da chave. Persistência híbrida: `organization_id`, `establishment_id`, `consent_type` e `status` são colunas tipadas em `consent_grants` (filtros de `FindConsentGrantsByOrganization` e `FindEffectiveScope`); `revocation_deadline_at` também (filtro de `FindPendingRevocations`); o resto vai em `snapshot` `jsonb`.
- Diverge do design: o isolamento por tenant é `tenant_id` injetado pelo `ExecutionContext` no `postgres.Table` (ADR-051 DMPF), não RLS como mecanismo primário (`docs/design/00` §4.1); `tenantId` não é campo do agregado.
- Diverge do design: eventos são Protobuf com CloudEvents montado pelo relay; o envelope JSON e o `schemaUri` do doc 92 §1 não se aplicam.
- Diverge do design: `instant` é inteiro de nanossegundos no domínio; as datas puras do escopo (`periodFrom`, `periodUntil`) viram `instant`.
- `originAddress` é `PII` e vai mascarado em log (`docs/design/90` §1.4).

## Políticas transversais

- **Idempotência**: `DraftConsent` e `RequestRevocation` (criação de recurso) entram pelo `bff` por `POST` com chave de idempotência declarada, que segue na metadata gRPC só para o log; os demais são idempotentes por identidade do agregado (RST-02; FND-04; ADR-056). O consumo de evento é idempotente pela inbox, por `eventId` (ADR-036).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Conceder e revogar são atos críticos, com reautenticação (`docs/design/01` §2.1); papéis conforme a matriz de `docs/design/95` (conceder: `APPROVER`; revogar: `APPROVER` e `ORG_ADMIN`).
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); os eventos vão ao Ledger e a prova do aceite é preservada com hash.
- **Prazos-alvo da revogação** (`docs/design/03` §5.3): bloqueio interno imediato e síncrono; comando à infraestrutura em menos de 1 minuto; notificação aos financiadores em menos de 5 minutos; confirmação pelo prazo acordado.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `consent` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários de aceite por comando:

```text
DADO nenhuma concessão anterior
QUANDO DraftConsent chega com consentType data_sharing, escopo completo e período válido
ENTÃO a concessão nasce draft, version 1, com effectiveScope vazio, e nada é emitido

DADO uma concessão ativa vigente
QUANDO DraftConsent chega com supersedesId dela e renewalMode full
ENTÃO nasce uma nova concessão draft com version 2, e a anterior permanece inalterada

DADO uma concessão draft
QUANDO PresentConsent chega com documentVersion, documentHash e presentation full
ENTÃO a concessão vai a pending_acceptance, com presentedAt = at

DADO uma concessão data_sharing pending_acceptance e uma representação vigente com alçada
QUANDO GrantConsent chega com o mesmo documentHash, explicitAction e reauthenticated verdadeiros
ENTÃO a concessão vai a active, acceptedAt = at, effectiveScope = scope e ConsentGranted vai à outbox

DADO uma concessão receivables_opt_in pending_acceptance
QUANDO GrantConsent chega com aceite válido
ENTÃO a concessão vai a active com effectiveScope vazio, até a ativação externa

DADO uma concessão pending_acceptance
QUANDO DeclineConsent chega
ENTÃO a concessão vai a rejected e nada é emitido

DADO um opt-in active sem autorização externa
QUANDO ActivateExternalOptIn chega com um escopo externo mais restrito que o interno
ENTÃO externalAuthorization é registrada, effectiveScope é a interseção e ExternalOptInActivated é emitido

DADO um opt-in external_sync_failed
QUANDO ActivateExternalOptIn chega com a autorização externa
ENTÃO a concessão volta a active sem novo aceite e ExternalOptInActivated é emitido

DADO um opt-in active sem autorização externa
QUANDO RecordExternalSyncFailure chega com reason provider_failure
ENTÃO a concessão vai a external_sync_failed, effectiveScope fica vazio e AuthorizationSyncFailed é emitido

DADO uma concessão active com dois destinatários
QUANDO RequestRevocation chega
ENTÃO a concessão vai a revocation_pending, effectiveScope fica vazio na mesma transação e ConsentRevocationRequested é emitido

DADO uma revogação pendente com dois destinatários, um já confirmado
QUANDO ConfirmRevocation chega do segundo
ENTÃO a concessão vai a revoked e ConsentRevoked é emitido com a propagação confirmada e os contratos preservados

DADO uma revogação pendente com dois destinatários
QUANDO ConfirmRevocation chega só do primeiro
ENTÃO a concessão segue revocation_pending e nenhum evento é emitido

DADO uma concessão active com periodUntil menor ou igual a at
QUANDO ExpireConsent chega
ENTÃO a concessão vai a expired, effectiveScope fica vazio e ConsentExpired é emitido

DADO uma representação ainda desconhecida
QUANDO RecordRepresentationStanding chega com status verified
ENTÃO a posição é inicializada verified, sem evento

DADO uma posição verified
QUANDO RecordRepresentationStanding chega com status revoked
ENTÃO a posição vai a revoked, com revokedAt do comando

DADO duas concessões da organização A e uma da B
QUANDO FindConsentGrantsByOrganization consulta A
ENTÃO devolve exatamente as duas de A, fora de qualquer UoW
```

- [ ] Um cenário por código de rejeição (nada é escrito e o estado não muda):

```text
DADO qualquer organização QUANDO DraftConsent chega com recipientOrganizationIds vazio ENTÃO rejeita com consent-mandates/consent-grant/scope-incomplete
DADO qualquer organização QUANDO DraftConsent chega com periodFrom igual ou posterior a periodUntil ENTÃO rejeita com consent-mandates/consent-grant/invalid-period
DADO qualquer organização QUANDO DraftConsent operational_mandate chega sem powers ENTÃO rejeita com consent-mandates/consent-grant/powers-empty
DADO qualquer organização QUANDO DraftConsent data_processing_basis chega sem legalBasis ENTÃO rejeita com consent-mandates/consent-grant/legal-basis-missing
DADO nenhuma concessão com o id informado QUANDO DraftConsent chega com supersedesId ENTÃO rejeita com consent-mandates/consent-grant/superseded-grant-not-found
DADO uma concessão pending_acceptance QUANDO PresentConsent chega ENTÃO rejeita com consent-mandates/consent-grant/not-draft
DADO uma concessão draft QUANDO PresentConsent chega com documentHash vazio ENTÃO rejeita com consent-mandates/consent-grant/document-hash-missing
DADO uma renovação com renewalMode full QUANDO PresentConsent chega com presentation simplified ENTÃO rejeita com consent-mandates/consent-grant/full-presentation-required
DADO uma concessão draft QUANDO GrantConsent ou DeclineConsent chega ENTÃO rejeita com consent-mandates/consent-grant/not-pending-acceptance
DADO uma concessão pending_acceptance QUANDO GrantConsent chega com documentHash diferente do apresentado ENTÃO rejeita com consent-mandates/consent-grant/document-mismatch
DADO uma concessão pending_acceptance QUANDO GrantConsent chega com explicitAction falso ENTÃO rejeita com consent-mandates/consent-grant/acceptance-not-explicit
DADO uma concessão pending_acceptance QUANDO GrantConsent chega com reauthenticated falso ENTÃO rejeita com consent-mandates/consent-grant/reauthentication-missing
DADO uma representação revogada QUANDO GrantConsent ou RequestRevocation chega citando-a ENTÃO rejeita com consent-mandates/consent-grant/representation-invalid
DADO um mandato com powers além da alçada da representação QUANDO GrantConsent chega ENTÃO rejeita com consent-mandates/consent-grant/mandate-exceeds-representation
DADO uma concessão data_sharing QUANDO ActivateExternalOptIn ou RecordExternalSyncFailure chega ENTÃO rejeita com consent-mandates/consent-grant/not-an-opt-in
DADO um opt-in já com autorização externa ativa QUANDO ActivateExternalOptIn chega ENTÃO rejeita com consent-mandates/consent-grant/not-awaiting-external-activation
DADO uma concessão revoked QUANDO RequestRevocation chega ENTÃO rejeita com consent-mandates/consent-grant/not-revocable
DADO uma data_processing_basis com revocable falso QUANDO RequestRevocation chega ENTÃO rejeita com consent-mandates/consent-grant/basis-not-revocable
DADO uma concessão active QUANDO ConfirmRevocation chega ENTÃO rejeita com consent-mandates/consent-grant/not-revocation-pending
DADO uma revogação pendente QUANDO ConfirmRevocation chega de um destinatário fora da propagação ENTÃO rejeita com consent-mandates/consent-grant/recipient-unknown
DADO uma concessão draft QUANDO ExpireConsent chega ENTÃO rejeita com consent-mandates/consent-grant/not-expirable
DADO uma concessão active com periodUntil no futuro QUANDO ExpireConsent chega ENTÃO rejeita com consent-mandates/consent-grant/not-yet-expired
```

## Escopo fora

- **REST do `bff`**: as rotas de `docs/prd/03` (Client API) e o canal pelo qual o financiador confirma a revogação são do `bff`; `docs/design/91-contratos-api.md` §3.2 só referencia `dataSharingConsentId` e o erro `CONSENT_INVALID`. O contexto serve só gRPC.
- **Execução do opt-in e das consultas**: Receivables Gateway; este contexto só decide a autorização e registra o resultado.
- **Identidade e poderes**: Identity mantém `Representation`; aqui só a projeção `RepresentationStanding`.
- **Textos dos instrumentos**: a redação jurídica (A12, 03:K10) e a estrutura de oito blocos (`docs/design/03` §7) ficam no documento apresentado; o contexto guarda versão e hash.
- **Revogação parcial** (B12, 03:K12) **e consentimento por categoria de financiador** (03:K11): fora até a apuração; hoje a revogação é total e os destinatários são nominais.
- **Renovação automática**: proibida; lembretes de vencimento são de Notificações.
- **Caso operacional por sincronização falha ou revogação sem confirmação**: Backoffice, a partir de `AuthorizationSyncFailed` e de `FindPendingRevocations`.
- **Acesso cross-tenant do financiador**: depende da extensão do kernel (ADR-051).
- **Armazenamento do texto e da prova bruta**: **[ABERTA: provider de object storage]**.
- **Composition root**: o esqueleto de `app/` e o `cmd/` com `--role` nascem do generator; o harness escreve o conteúdo de domínio dentro deles.
