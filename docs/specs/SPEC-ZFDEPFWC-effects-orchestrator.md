---
id: SPEC-ZFDEPFWC
slug: effects-orchestrator
title: Bounded context — effects-orchestrator
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-GA4FYX4Z]
ticket_url: https://linear.app/mmda/issue/DEVS-90/spec-zfdepfwc-bounded-context-effects-orchestrator
subtask_urls: []
created: 2026-10-12
---

# SPEC-ZFDEPFWC: Bounded context — effects-orchestrator

## Identidade

- **bounded_context**: `effects-orchestrator`
- **name**: `effects` (módulo `apps/backend/effects`, package Protobuf `company.effects.event.v1`)
- **Linguagem ubíqua**:
  - *Efeito regulatório* (`RegulatoryEffect`) - intenção de constituir, alterar ou extinguir uma consequência jurídica sobre um conjunto de ativos, registrada na infraestrutura competente.
  - *Alvo* (`EffectTarget`) - um ativo (ou parte dele) alcançado pelo efeito; a confirmação é por alvo.
  - *Eficácia* - o efeito vale perante terceiros só com a confirmação da infraestrutura, nunca com o aceite técnico do comando.
  - *Janela crítica* - passos 9 a 13 da sequência de submissão (09 §3): revalidação, persistência de `SUBMITTED`, envio e registro da resposta.
  - *Resultado desconhecido* (`UNKNOWN_OUTCOME`) - não se sabe se o comando externo surtiu efeito; só se resolve por consulta de estado, nunca por reenvio.
  - *Resolução de tipo* (`EffectTypeResolution`) - tabela versionada que traduz a natureza do contrato no tipo e no código de efeito aceitos pela infraestrutura.
  - *Disputa* (`EffectDispute`) - divergência entre contrato, efeito e posição oficial; a plataforma registra e acompanha, não decide prioridade jurídica.
  - *Liberação* - comando que extingue o efeito; o cancelamento interno não remove o efeito externo sem a confirmação dela.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/09-prd-effects-orchestrator.md`; `docs/design/09-design-effects-orchestrator.md` §2 a §8; `docs/design/90-modelo-de-dados.md` §3.4 e §4; `docs/design/92-schemas-eventos.md` §3.4 e §4; `docs/prd/14-catalogo-contratos-eventos.md` (sagas, política de falhas); `docs/design/00-arquitetura-tecnica.md` §7.2 e §7.3; `docs/design/04-design-receivables-gateway.md` §3.3 e §4; `docs/design/94-seguranca-threat-model.md` T1 e T9.
- **Situação**: `bloqueado` - sem os códigos de efeito registráveis (#2, B3) nenhum comando de constituição é enviável (09 §4); o modelo, o ciclo de vida e a sequência da janela crítica são construíveis.
- **Fase de entrega**: 1 (cartão); 2 (duplicata); 4 (futuros e múltiplas infraestruturas).
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-GA4FYX4Z`, Contracting); a cadeia transitiva (identity, partner, gateway, consent, portfolio) fica implícita. Upstream duro: Contracting (`ContractSubmittedForEffect`, `ContractCancelled`), Gateway (`RegulatoryEffectsPort`; `ExternalCommandConfirmed`, `ExternalCommandFailed`, `ExternalOutcomeUnknown`), Partner Management (capacidade), Portfolio (posição) e Consent (`OperationalMandate`). Upstream brando: Settlement (`EffectReleaseRequested`, `AssetSettled`); a aresta de retroalimentação não entra em `depends_on`.
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
|---|---|---|
| #2 (B3; 09:F8) | tipos e códigos de efeito por infraestrutura; trava de domicílio × cessão | `EffectTypeResolution`, `PlanRegulatoryEffect`; o envio inteiro |
| B1 (04:G9; 09:F11) | existe consulta de estado por operação de escrita? | `RecordEffectOutcomeUnknown`, `ResubmitEffect`, recuperação automática |
| B6 (09:F11) | prazo típico e máximo de confirmação | limiar de `UNKNOWN_OUTCOME` |
| A11 (09:F9; 90:L4) | cláusula de confirmação parcial no instrumento | `partialConfirmationClause`, `RecordEffectConfirmation` |
| B10 (09:F10) | suporte a confirmação parcial por alvo | `partialResultSupported` do Gateway; estado `partially_effective` |
| B11, D6 (09:F10) | efeito sobre recebíveis futuros | `scopeMode = rule_based`; extensão automática (req. 8 do PRD) |
| B16 (09:F12) | procedimento e prazo de disputa por infraestrutura | comandos de `EffectDispute` |
| B13 | quem informa o rateio com efeitos concorrentes | prioridade entre efeitos (a plataforma não a decide) |
| #1 (A1) | autorização do BCB (transversal) | o contexto inteiro |

- **Decisões de produto abertas**: caminho padrão da confirmação parcial (09 §5.2, proposta: revisão contratual); recebíveis futuros no MVP (D6; a estratégia os põe na fase 4); granularidade da liberação, por efeito ou por alvo (11 §7 × 09) [ABERTA: 09:§2].
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída; `SPEC-GA4FYX4Z` com `ContractSubmittedForEffect` e `ContractCancelled` publicados; #2 (B3) resolvido para ao menos a infraestrutura da fase 1, porque trava o envio inteiro; B1 e B6 decididos ou assumidos como `stateQuerySupported = false` com caso manual. Sem #2, o modelo e o ciclo avançam contra o simulador do Gateway (04:G12).

## Agregados

`EffectTarget`, `EffectTerms`, `ExternalEffectReference`, `EffectConfirmation` e `EffectDispute` são entidades ou valores internos de `RegulatoryEffect`: o status do efeito é derivado dos alvos e não pode divergir deles (90 §3.4), e a disputa muda o status do efeito. Instantes `*At` valem 0 antes do fato; datas puras são `string` `AAAA-MM-DD`; dinheiro é `*Minor: int` com `currency: string`.

### RegulatoryEffect

- **Identidade**: `generated` (N por contrato, um por infraestrutura de destino, 90 §4).
- **Campos**:
  - Origem: `contractId: id`, `registryPartnerId: id`, `beneficiaryOrganizationId: id`, `cedentOrganizationId: id`.
  - Tipo: `effectType: string` e `providerEffectCode: string` [A CONFIRMAR: #2, B3] (os tipos conceituais de 09 PRD - desconto ou aquisição definitiva, coobrigação, cessão, cessão fiduciária, garantia, mudança de titularidade, endosso e endosso-penhor, cancelamento, liberação, portabilidade - ainda não têm código), `resolutionId: id`, `resolutionVersion: int`.
  - Termos (`EffectTerms`): `termsPriority: int`, `termsAmountMinor: int`, `currency: string`, `termsValidFrom: string`, `termsValidUntil: string`.
  - Alcance: `scopeMode: enumerated | rule_based`; no modo `rule_based` [A CONFIRMAR: B11, F10]: `ruleAssetTypes: list<string>`, `rulePeriodFrom: string`, `rulePeriodUntil: string`, `ruleAcquirerIds: list<id>`, `ruleSchemeIds: list<id>`, `ruleMaxAmountMinor: int`.
  - `targets: list<EffectTarget>` (coleção indispensável: confirmação parcial por alvo; milhares de itens, 92 §3.4 cita 1284) com `EffectTarget` = `targetId: id`, `assetType: string`, `assetId: id`, `requestedAmountMinor: int`, `confirmedAmountMinor: int`, `status: pending | confirmed | rejected | unknown`, `rejectionCode: string`, `rejectionReason: string` (originais do parceiro, 92 §2), `rejectionClass: unavailability | validation | other`, `externalReference: string`.
  - Execução: `idempotencyKey: string`, `payloadVersion: int`, `externalReference: string` e `externalReferenceAsOf: instant` (`ExternalEffectReference`), `confirmationSource: callback | polled_query | file`, `confirmedAt: instant`, `confirmationEvidenceId: id`, `confirmationEvidenceHash: string` (`EffectConfirmation`; o histórico de interações fica no Gateway e no Ledger).
  - Parcial: `partialConfirmationClause: absent | proportional_adjustment | all_or_nothing`, `minConfirmedRatio: decimal` [ABERTA: 09:§5.1 "mínimo do produto" sem valor nem dono; 13 não o define].
  - Fluxo: `status`, `statusBeforeDispute: string`, `rejectionOrigin: none | partner | prevalidation`, `cancellationPending: bool`, `releaseCause: string`, `plannedAt`, `submittedAt`, `acknowledgedAt`, `releasedAt: instant`.
  - `disputes: list<EffectDispute>` (coleção indispensável) com `EffectDispute` = `disputeId: id`, `trigger: holder_divergence | unrequested_effect_confirmed | third_party_effect | release_not_confirmed | external_position_divergence`, `severity: critical | high` (derivada do gatilho, 09 §8), `status: opened | investigating | resolved_internal | escalated_to_partner | partner_review | resolved_partner | unresolved`, `evidenceId: id`, `resolution: string`, `openedAt`, `resolvedAt: instant`.
- **Estados**: `planned`, `prevalidating`, `submitted`, `acknowledged`, `effective`, `partially_effective`, `release_pending`, `released`, `rejected`, `unknown_outcome`, `disputed`, `cancelled`.
- **Transições permitidas**:
  - (novo) → `planned` - por `PlanRegulatoryEffect`; `rejected`, `unknown_outcome` → `planned` - por `ResubmitEffect`
  - `planned` → `prevalidating` - por `StartPrevalidation`
  - `prevalidating` → `submitted` ou `rejected` (origem `prevalidation`) - por `RecordPrevalidation`
  - `submitted` → `acknowledged` - por `RecordEffectAcknowledged`
  - `submitted`, `acknowledged`, `unknown_outcome`, `partially_effective` → `effective`, `partially_effective`, `rejected` ou `unknown_outcome` - por `RecordEffectConfirmation`, derivado dos alvos
  - `submitted`, `acknowledged`, `unknown_outcome` → `rejected` - por `RecordEffectRejection`
  - `submitted`, `acknowledged` → `unknown_outcome` - por `RecordEffectOutcomeUnknown`
  - `effective`, `partially_effective` → `release_pending` - por `RequestEffectRelease`; `release_pending` → `released` - por `RecordEffectReleased`
  - `planned` → `cancelled` - por `ApplyContractCancellation`
  - `acknowledged`, `effective`, `partially_effective`, `release_pending`, `disputed` → `disputed` - por `OpenEffectDispute`; `disputed` → `statusBeforeDispute` - por `ResolveEffectDispute` quando nenhuma disputa segue aberta ou em revisão [ABERTA: 09:§8 não define o retorno de `DISPUTED`]
- **Invariantes de estado**:
  - O status é derivado dos alvos: `effective` só com todos `confirmed`; com ao menos um `confirmed` e mistura, `partially_effective`; sem nenhum `confirmed`, `rejected` se todos `rejected`, senão `unknown_outcome`. Confirmação parcial divide o resultado por alvo e nunca promove o conjunto (09 PRD).
  - Eficácia exige referência externa e confirmação - `effects-orchestrator/regulatory-effect/external-reference-missing`.
  - Só `effective` autoriza o desembolso padrão; Funding consome `EffectConfirmed`, não `EffectAcknowledged`.
  - `unknown_outcome` nunca é reenviado sem consulta de estado - `effects-orchestrator/regulatory-effect/state-query-missing`.
  - Cancelamento interno não remove o efeito externo: o status só chega a `released` pela confirmação da infraestrutura.
  - A plataforma não decide prioridade jurídica entre efeitos conflitantes: a disputa é registrada e escalada, nunca resolvida por regra interna.

### EffectTypeResolution

- **Identidade**: `natural` por `resolutionKey` (composta de `resolutionId` e `version`, `<resolutionId>:<version>`).
- **Campos**: `resolutionId: id`, `version: int`, condições (`contractNature: string`, `assetType: string`, `registryPartnerId: id`, `hasRecourse: bool`, `scopeMode: enumerated | rule_based`), `canonicalEffectType: string` e `providerEffectCode: string` [A CONFIRMAR: #2, B3], `requiredFields: list<string>` (campos que o parceiro exige para o tipo), `effectiveFrom: instant`, `publishedAt: instant`.
- **Estados**: nenhum; cada versão é imutável e a mudança regulatória publica versão nova (09 §4).
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: `providerEffectCode` nunca é vazio - `effects-orchestrator/effect-type-resolution/provider-code-missing`; uma versão não é republicada - `effects-orchestrator/effect-type-resolution/version-already-published`.

## Comandos (UPRs)

Cada passo da saga é um comando local; o disparo vem de evento consumido, de retorno de porta, de chamada gRPC ou de processo do contexto. Os fatos de outros contextos (capacidade, mandato, posição, tipo resolvido) são lidos pelo caso de uso, fora da UoW, e entram como entrada do comando. O encadeamento entre os passos do mesmo agregado é do processo do contexto [ABERTA: encadeamento intra-contexto não normatizado no kernel].

### PlanRegulatoryEffect

- **Agregado**: `RegulatoryEffect`; **cria**
- **Disparo**: consumo de `ContractSubmittedForEffect`, um comando por infraestrutura de destino (09 §3, passos 1 a 8)
- **Entrada**: `contractId: id`, `registryPartnerId: id`, `beneficiaryOrganizationId: id`, `cedentOrganizationId: id`, `scopeMode`, `targets: list<id, assetType, assetId, requestedAmountMinor>`, termos (`termsPriority`, `termsAmountMinor`, `currency`, `termsValidFrom`, `termsValidUntil`), `partialConfirmationClause`, `minConfirmedRatio`, `idempotencyKey: string`, `payloadVersion: int`, regra (`rule*`, só `rule_based`); fatos: `resolutionId`, `resolutionVersion`, `canonicalEffectType`, `providerEffectCode`, `requiredFieldsSatisfied: bool`, `contractTechnicallyComplete: bool`, `capabilityEnabled: bool`, `futureReceivablesSupported: bool`, `operationalMandateActive: bool`
- **Pré-condições**:
  - `contractTechnicallyComplete` → `effects-orchestrator/regulatory-effect/contract-incomplete`
  - `providerEffectCode` não vazio (mapeamento resolvido; nunca um tipo "parecido") → `effects-orchestrator/regulatory-effect/effect-type-unresolved`
  - `requiredFieldsSatisfied` → `effects-orchestrator/regulatory-effect/required-fields-missing`
  - `capabilityEnabled` → `effects-orchestrator/regulatory-effect/capability-not-enabled`
  - `operationalMandateActive` (03 §"registrar efeito": mandato mais contrato assinado) → `effects-orchestrator/regulatory-effect/mandate-not-active`
  - `enumerated` com ao menos um alvo de valor positivo → `effects-orchestrator/regulatory-effect/no-targets`
  - `rule_based` só com `futureReceivablesSupported` → `effects-orchestrator/regulatory-effect/rule-based-scope-unsupported`
  - `idempotencyKey` não vazia → `effects-orchestrator/regulatory-effect/idempotency-key-missing`
- **Efeitos sobre o estado**: `status ← planned`; alvos `← pending`; demais campos `← cmd`; `plannedAt ← at`
- **Eventos emitidos**: `RegulatoryEffectPlanned`
- **Resposta**: `effectId: id`

### StartPrevalidation

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: processo do contexto, após `RegulatoryEffectPlanned`
- **Entrada**: nenhuma além da identidade
- **Pré-condições**: `status = planned` → `effects-orchestrator/regulatory-effect/not-planned`
- **Efeitos sobre o estado**: `status ← prevalidating`
- **Eventos emitidos**: nenhum (trilha de auditoria do kernel)
- **Resposta**: `effectId: id`

### RecordPrevalidation

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: processo do contexto, com o resultado da revalidação da posição de todos os ativos junto à fonte (09 §3, passos 9 a 11); nenhum cache ou consulta reaproveitada
- **Entrada**: `checkedAt: instant`, `results: list<targetId, consistent: bool, divergence: string>`
- **Pré-condições**:
  - `status = prevalidating` → `effects-orchestrator/regulatory-effect/not-prevalidating`
  - um resultado por alvo → `effects-orchestrator/regulatory-effect/prevalidation-incomplete`
- **Efeitos sobre o estado**: todos consistentes: `status ← submitted`, `submittedAt ← at` (persistido antes do envio, 09 §3). Qualquer divergência aborta sem ajustar o lote nem enviar parcial: `status ← rejected`, `rejectionOrigin ← prevalidation`, alvos divergentes `← rejected` com `rejectionClass ← unavailability` [ABERTA: 09:§3 passo 10 não define estado nem evento do aborto]
- **Eventos emitidos**: `EffectSubmitted` ou `EffectRejected`
- **Resposta**: `effectId: id`, `status`

### RecordEffectAcknowledged

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: processo do contexto, com o retorno síncrono de `submitEffect` (aceite técnico, não eficácia)
- **Entrada**: `externalReference: string` (quando o parceiro a devolve), `acknowledgedAt: instant`
- **Pré-condições**: `status = submitted` → `effects-orchestrator/regulatory-effect/not-submitted`
- **Efeitos sobre o estado**: `status ← acknowledged`; `acknowledgedAt ← at`; `externalReference ←` valor informado, se houver
- **Eventos emitidos**: `EffectAcknowledged`
- **Resposta**: `effectId: id`

### RecordEffectConfirmation

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: consumo de `ExternalCommandConfirmed` com `operation = submit`
- **Entrada**: `externalReference: string`, `externalReferenceAsOf: instant`, `confirmationSource`, `confirmedAt: instant`, `corroboratedByStateQuery: bool`, `evidenceId: id`, `evidenceHash: string`, `outcomes: list<targetId, status, confirmedAmountMinor, rejectionCode, rejectionReason, rejectionClass>`
- **Pré-condições**:
  - `status` em `submitted`, `acknowledged`, `unknown_outcome` ou `partially_effective` → `effects-orchestrator/regulatory-effect/not-awaiting-confirmation`
  - `corroboratedByStateQuery` (94 C1.3: a confirmação nunca deriva só da mensagem) → `effects-orchestrator/regulatory-effect/confirmation-not-corroborated`
  - `externalReference` não vazia → `effects-orchestrator/regulatory-effect/external-reference-missing`
  - todo `targetId` existe → `effects-orchestrator/regulatory-effect/unknown-target`
  - alvo `confirmed` com `0 < confirmedAmountMinor ≤ requestedAmountMinor` → `effects-orchestrator/regulatory-effect/invalid-confirmed-amount`
- **Efeitos sobre o estado**: aplica o resultado a cada alvo, preservando código e texto originais; `status ←` derivado dos alvos (ver invariantes); `externalReference`, `confirmationSource`, `confirmedAt` e evidência `← cmd`. Alvo sem resultado segue `unknown` ou `pending` (consultar, não reenviar)
- **Eventos emitidos**: `EffectConfirmed` (todos confirmados), `EffectPartiallyConfirmed` (mistura; carrega `belowProductMinimum` quando a razão confirmada fica abaixo de `minConfirmedRatio`), `EffectRejected` (todos rejeitados); nenhum quando segue em `unknown_outcome`
- **Resposta**: `effectId: id`, `status`

### RecordEffectRejection

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: consumo de `ExternalCommandFailed` com `operation = submit` (categorias `FINAL_*`, 04 §4)
- **Entrada**: `failureCategory: final_validation | final_business | final_authorization`, `rejectionCode: string`, `rejectionReason: string`, `rejectionClass`, `failedAt: instant`
- **Pré-condições**: `status` em `submitted`, `acknowledged` ou `unknown_outcome` → `effects-orchestrator/regulatory-effect/not-awaiting-confirmation`
- **Efeitos sobre o estado**: alvos não confirmados `← rejected` com código e texto originais; `status ← rejected`; `rejectionOrigin ← partner`
- **Eventos emitidos**: `EffectRejected` (rejeição por indisponibilidade é sinal de risco, 09 §5.3 [ABERTA: 92:§4 não lista Risco como consumidor])
- **Resposta**: `effectId: id`

### RecordEffectOutcomeUnknown

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: consumo de `ExternalOutcomeUnknown` com `operation = submit`; ou processo do contexto quando `submitted` fica sem resposta além do limiar [A CONFIRMAR: B6, F11]
- **Entrada**: `failureCategory: timeout | no_response | unparseable`, `lastAttemptAt: instant`, `attemptCount: int`, `stateQuerySupported: bool`, `nextQueryScheduledAt: instant`
- **Pré-condições**: `status` em `submitted` ou `acknowledged` → `effects-orchestrator/regulatory-effect/not-awaiting-confirmation`
- **Efeitos sobre o estado**: `status ← unknown_outcome`; alvos `pending ← unknown`
- **Eventos emitidos**: `EffectOutcomeUnknown` (com `blocksFunding = true`; `stateQuerySupported = false` leva direto a tratamento humano)
- **Resposta**: `effectId: id`

### ResubmitEffect

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: chamada gRPC do backoffice (reenvio manual, 17 §5: supervisor mais segundo aprovador)
- **Entrada**: `reason: validation_corrected | not_received_confirmed`, `priorStateQueryId: id`, `payloadVersion: int`, `idempotencyKey: string`
- **Pré-condições**:
  - `status` em `rejected` ou `unknown_outcome` → `effects-orchestrator/regulatory-effect/not-resubmittable`
  - `validation_corrected` só após rejeição de classe `validation`; `not_received_confirmed` só em `unknown_outcome` → `effects-orchestrator/regulatory-effect/rejection-not-correctable`
  - `priorStateQueryId` não vazio (94 C9.3) → `effects-orchestrator/regulatory-effect/state-query-missing`
  - `validation_corrected` exige chave nova e `payloadVersion` maior; `not_received_confirmed` mantém a chave → `effects-orchestrator/regulatory-effect/idempotency-key-reused`
- **Efeitos sobre o estado**: `status ← planned` (a janela crítica recomeça, com nova revalidação); `payloadVersion` e `idempotencyKey ← cmd`; alvos `← pending`; `rejectionOrigin ← none`
- **Eventos emitidos**: `RegulatoryEffectPlanned`
- **Resposta**: `effectId: id`

### RequestEffectRelease

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: consumo de `EffectReleaseRequested`; chamada gRPC (cancelamento de instrução, ação do backoffice); processo do contexto quando `cancellationPending` e o efeito confirma
- **Entrada**: `cause: obligation_fulfilled | contract_cancelled | partial_all_or_nothing | partial_below_minimum | funding_cancelled | operator_request`, `requestedAt: instant`
- **Pré-condições**: `status` em `effective` ou `partially_effective` → `effects-orchestrator/regulatory-effect/not-releasable`
- **Efeitos sobre o estado**: `status ← release_pending`; `releaseCause ← cmd.cause`; `cancellationPending ← false`
- **Eventos emitidos**: nenhum (o pedido ao parceiro sai pelo Gateway, `requestRelease`; `EffectReleased` só na confirmação)
- **Resposta**: `effectId: id`

### RecordEffectReleased

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: consumo de `ExternalCommandConfirmed` com `operation = release`
- **Entrada**: `releaseReference: string`, `confirmedAt: instant`, `corroboratedByStateQuery: bool`, `evidenceId: id`
- **Pré-condições**:
  - `status = release_pending` → `effects-orchestrator/regulatory-effect/not-release-pending`
  - `corroboratedByStateQuery` → `effects-orchestrator/regulatory-effect/confirmation-not-corroborated`
- **Efeitos sobre o estado**: `status ← released`; `releasedAt ← at`
- **Eventos emitidos**: `EffectReleased`
- **Resposta**: `effectId: id`

### ApplyContractCancellation

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: consumo de `ContractCancelled` (09 §7)
- **Entrada**: `terminalState: cancelled | rejected | expired`, `reason: string`
- **Pré-condições**: nenhuma; o efeito depende do status
- **Efeitos sobre o estado**: `planned`: `status ← cancelled`. `prevalidating`, `submitted`, `acknowledged`, `unknown_outcome`: `cancellationPending ← true` e o comando em trânsito não é interrompido (aguarda o desfecho). `effective`, `partially_effective`: `cancellationPending ← true` e o processo pede `RequestEffectRelease` com causa `contract_cancelled`. Demais status: nenhum
- **Eventos emitidos**: nenhum [ABERTA: sem evento para `CANCELLED` no PRD nem em 92]
- **Resposta**: `effectId: id`

### OpenEffectDispute

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: chamada gRPC (backoffice); processo do contexto quando a liberação não é confirmada no prazo [A CONFIRMAR: B16, F12]
- **Entrada**: `disputeId: id`, `trigger`, `evidenceId: id`
- **Pré-condições**: `status` em `acknowledged`, `effective`, `partially_effective`, `release_pending` ou `disputed` → `effects-orchestrator/regulatory-effect/not-disputable`
- **Efeitos sobre o estado**: acrescenta a disputa `opened` com `severity` derivada do gatilho (09 §8); `statusBeforeDispute ← status` se ainda não estava em `disputed`; `status ← disputed`
- **Eventos emitidos**: `EffectDisputed`
- **Resposta**: `disputeId: id`

### InvestigateEffectDispute

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: chamada gRPC (backoffice)
- **Entrada**: `disputeId: id`
- **Pré-condições**: a disputa existe → `effects-orchestrator/regulatory-effect/dispute-not-found`; `opened` → `investigating` → `effects-orchestrator/regulatory-effect/invalid-dispute-transition`
- **Efeitos sobre o estado**: `dispute.status ← investigating`
- **Eventos emitidos**: nenhum
- **Resposta**: `disputeId: id`

### EscalateEffectDispute

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: chamada gRPC (backoffice)
- **Entrada**: `disputeId: id`, `partnerContactRef: string` [A CONFIRMAR: B16]
- **Pré-condições**: a disputa existe → `effects-orchestrator/regulatory-effect/dispute-not-found`; `investigating` → `escalated_to_partner` → `effects-orchestrator/regulatory-effect/invalid-dispute-transition`
- **Efeitos sobre o estado**: `dispute.status ← escalated_to_partner`
- **Eventos emitidos**: nenhum
- **Resposta**: `disputeId: id`

### RecordPartnerReview

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: chamada gRPC (backoffice), quando o parceiro assume a revisão
- **Entrada**: `disputeId: id`, `partnerReference: string`
- **Pré-condições**: a disputa existe → `effects-orchestrator/regulatory-effect/dispute-not-found`; `escalated_to_partner` → `partner_review` → `effects-orchestrator/regulatory-effect/invalid-dispute-transition`
- **Efeitos sobre o estado**: `dispute.status ← partner_review`
- **Eventos emitidos**: nenhum
- **Resposta**: `disputeId: id`

### ResolveEffectDispute

- **Agregado**: `RegulatoryEffect`; exige existente
- **Disparo**: chamada gRPC (backoffice)
- **Entrada**: `disputeId: id`, `outcome: resolved_internal | resolved_partner | unresolved`, `resolution: string`
- **Pré-condições**: a disputa existe → `effects-orchestrator/regulatory-effect/dispute-not-found`; `resolved_internal` só de `investigating`, `resolved_partner` e `unresolved` só de `partner_review` → `effects-orchestrator/regulatory-effect/invalid-dispute-transition`
- **Efeitos sobre o estado**: `dispute.status ← outcome`; sem disputa aberta, escalada ou em revisão, `status ← statusBeforeDispute`; `unresolved` mantém `disputed`
- **Eventos emitidos**: nenhum [ABERTA: PRD e 92 não têm evento de resolução de disputa]
- **Resposta**: `disputeId: id`

### PublishEffectTypeResolution

- **Agregado**: `EffectTypeResolution`; **cria**
- **Disparo**: chamada gRPC (backoffice, com aval regulatório)
- **Entrada**: `resolutionId: id`, `version: int`, condições, `canonicalEffectType: string`, `providerEffectCode: string`, `requiredFields: list<string>`, `effectiveFrom: instant`
- **Pré-condições**:
  - `providerEffectCode` não vazio → `effects-orchestrator/effect-type-resolution/provider-code-missing`
  - versão ainda não publicada → `effects-orchestrator/effect-type-resolution/version-already-published`
- **Efeitos sobre o estado**: campos `← cmd`; `publishedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `resolutionKey: string`

## Eventos de domínio

Os schemas do doc 92 §3.4 cobrem só `EffectConfirmed`, `EffectPartiallyConfirmed` e `EffectOutcomeUnknown`; os demais campos abaixo são derivados do agregado. `targets` e `outcomes` são `list<tipo>` (mesma nota de volume do agregado).

### RegulatoryEffectPlanned

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `registryPartnerId: id ← RegulatoryEffect.registryPartnerId`, `effectType: string ← RegulatoryEffect.effectType`, `payloadVersion: int ← RegulatoryEffect.payloadVersion`, `targetCount: int ← size(targets)`, `at: instant ← at`

### EffectSubmitted

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `registryPartnerId: id ← RegulatoryEffect.registryPartnerId`, `idempotencyKey: string ← RegulatoryEffect.idempotencyKey`, `at: instant ← at`

### EffectAcknowledged

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `externalReference: string ← RegulatoryEffect.externalReference`, `at: instant ← at`

### EffectConfirmed

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `effectType: string ← RegulatoryEffect.effectType`, `registryPartnerId: id ← RegulatoryEffect.registryPartnerId`, `beneficiaryOrganizationId: id ← RegulatoryEffect.beneficiaryOrganizationId`, `externalReference: string ← RegulatoryEffect.externalReference`, `confirmedAt: instant ← RegulatoryEffect.confirmedAt`, `confirmationSource ← RegulatoryEffect.confirmationSource`, `targets: list<targetId, assetType, assetId, requestedAmountMinor, confirmedAmountMinor, status> ← RegulatoryEffect.targets`, `requestedTotalMinor: int ← soma(requestedAmountMinor)`, `confirmedTotalMinor: int ← soma(confirmedAmountMinor)`, `evidenceId: id ← RegulatoryEffect.confirmationEvidenceId`, `at: instant ← at`

### EffectPartiallyConfirmed

- **Campos**: os de `EffectConfirmed`, com `targets` em status misto, mais `belowProductMinimum: bool ← razão confirmada < minConfirmedRatio` e `partialConfirmationClause ← RegulatoryEffect.partialConfirmationClause`. Evento separado porque o Funding precisa de rota distinta (92 §3.4).

### EffectRejected

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `rejectionOrigin ← RegulatoryEffect.rejectionOrigin`, `rejectionClass ← RegulatoryEffect.targets.rejectionClass`, `targets: list<targetId, rejectionCode, rejectionReason> ← RegulatoryEffect.targets`, `at: instant ← at`

### EffectOutcomeUnknown

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `registryPartnerId: id ← RegulatoryEffect.registryPartnerId`, `idempotencyKey: string ← RegulatoryEffect.idempotencyKey`, `lastAttemptAt: instant ← cmd.lastAttemptAt`, `attemptCount: int ← cmd.attemptCount`, `failureCategory ← cmd.failureCategory`, `stateQuerySupported: bool ← cmd.stateQuerySupported`, `nextQueryScheduledAt: instant ← cmd.nextQueryScheduledAt`, `blocksFunding: bool ← true`, `at: instant ← at`

### EffectReleased

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `releaseCause: string ← RegulatoryEffect.releaseCause`, `releasedAt: instant ← RegulatoryEffect.releasedAt`, `at: instant ← at`

### EffectDisputed

- **Campos**: `effectId: id ← RegulatoryEffect.id`, `contractId: id ← RegulatoryEffect.contractId`, `disputeId: id ← EffectDispute.disputeId`, `trigger ← EffectDispute.trigger`, `severity ← EffectDispute.severity`, `evidenceId: id ← EffectDispute.evidenceId`, `at: instant ← at`

## Consultas

- `FindRegulatoryEffect` - filtros: `effectId`; cardinalidade: `one` (inclui alvos e disputas)
- `FindRegulatoryEffectsByContract` - filtros: `contractId`; cardinalidade: `many` (Funding e Ops)
- `FindRegulatoryEffectsByStatus` - filtros: `status`, `submittedAt` (varredura de `submitted` sem resposta e de `unknown_outcome`); cardinalidade: `many`
- `FindRegulatoryEffectsByAsset` - filtros: `assetId`, `status`; cardinalidade: `many` (Eligibility, regra `AST-ENC-001` com efeitos baseados em regra, 09 §6) [ABERTA: índice por ativo dentro de `targets`]
- `FindEffectTypeResolution` - filtros: `contractNature`, `assetType`, `registryPartnerId`, `hasRecourse`, `scopeMode`, instante de vigência; cardinalidade: `one`

## Relações entre agregados

- `RegulatoryEffect.contractId` → `ContractCase` (Contracting)
- `RegulatoryEffect.registryPartnerId` → `Partner` (Partner Management)
- `RegulatoryEffect.beneficiaryOrganizationId`, `RegulatoryEffect.cedentOrganizationId` → `Organization` (Identity)
- `RegulatoryEffect.resolutionId` e `RegulatoryEffect.resolutionVersion` → `EffectTypeResolution`
- `RegulatoryEffect.targets.assetId` → `FinancialAsset` (Portfolio)

## Integração

- **Publica** (integration events): `RegulatoryEffectPlanned`, `EffectSubmitted`, `EffectAcknowledged`, `EffectConfirmed`, `EffectPartiallyConfirmed`, `EffectRejected`, `EffectOutcomeUnknown`, `EffectReleased` e `EffectDisputed` - viram `regulatory_effect_planned.proto`, `effect_submitted.proto`, `effect_acknowledged.proto`, `effect_confirmed.proto`, `effect_partially_confirmed.proto`, `effect_rejected.proto`, `effect_outcome_unknown.proto`, `effect_released.proto` e `effect_disputed.proto` em `apps/backend/effects/contract/proto/company/effects/event/v1/`, package `company.effects.event.v1`. Consumidores declarados em 92 §4: Funding, Portfolio, Settlement, Contracting, Ops, Risco e Ledger. `EffectAcknowledged` consta do PRD e não do catálogo de 92 §4 [ABERTA: 92:§4].
- **Consome** (disposições de consumo por FND-04 §6.4; recusa de negócio segue a disposição de falha definitiva; os schemas dos eventos de Contracting, Gateway e Settlement não estão em 92 [ABERTA: 92:§3]):
  - `company.contracting.event.v1.ContractSubmittedForEffect` → `PlanRegulatoryEffect` (um por infraestrutura de destino). Mapa: `contractId ← contractId`, `cedentOrganizationId ← cedentOrganizationId`, `beneficiaryOrganizationId ← funderOrganizationId`, `scopeMode ← scheduleMode`, `partialConfirmationClause ← partialConfirmationClause`; alvos ← consulta gRPC `FindAssetSchedule` ao Contracting, conferida por `assetSetHash` (lista grande: a exceção de 92 §3.2 à convenção "autocontido"); `registryPartnerId` ← Partner Management por tipo de ativo (09 §4); tipo e código ← `EffectTypeResolution`. [ABERTA: 92:§4 lista Effects como consumidor de `ContractSigned` e de `ContractSubmittedForEffect`; esta spec usa só o segundo como gatilho]
  - `company.contracting.event.v1.ContractCancelled` → `ApplyContractCancellation`. Mapa: `terminalState ← terminalState`, `reason ← reason`.
  - `company.gateway.event.v1.ExternalCommandConfirmed` → `RecordEffectConfirmation` (`operation = submit`) ou `RecordEffectReleased` (`operation = release`). Mapa: `effectId ← businessKey`, `externalReference ← externalReference`, `confirmationSource ← confirmationSource`, `corroboratedByStateQuery ← corroboratedByStateQuery`, `outcomes ← resultado por alvo`, `evidenceId ← evidenceId`. [ABERTA: o discriminador `operation` e o resultado por alvo não constam de 92]
  - `company.gateway.event.v1.ExternalCommandFailed` → `RecordEffectRejection`. Mapa: `failureCategory`, `rejectionCode`, `rejectionReason` do evento, sem tradução.
  - `company.gateway.event.v1.ExternalOutcomeUnknown` → `RecordEffectOutcomeUnknown`. Mapa: `failureCategory`, `lastAttemptAt`, `attemptCount`, `stateQuerySupported`, `nextQueryScheduledAt`.
  - `company.settlement.event.v1.EffectReleaseRequested` → `RequestEffectRelease` com `cause = obligation_fulfilled`. Mapa: `effectId ← effectId`, `requestedAt ← at`.
  - `AssetSettled` (Settlement): 92 §4 lista Effects como consumidor e 09 não define o efeito; sem comando nesta versão [ABERTA: 92:§4].
- **Consultas e comandos de saída**: pelo Gateway, `RegulatoryEffectsPort` (`submitEffect`, `queryEffect`, `requestRelease`, `queryEffectsByAsset`) e `ReceivablesQueryPort.queryAsset` na revalidação; consulta gRPC a Partner Management (capacidade) e a Consent (`OperationalMandate`). O envio e a liberação são comandos externos com resultado ambíguo possível [ABERTA: ciclo de comando externo sem suporte no kernel (`UNKNOWN_OUTCOME` e idempotência de saída; ADR-056 cobre só comando de entrada)].

## Borda e persistência

- **Borda**: só gRPC, `company.effects.service.v1.EffectsService`, com um RPC por comando (`PlanRegulatoryEffect`, `StartPrevalidation`, `RecordPrevalidation`, `RecordEffectAcknowledged`, `RecordEffectConfirmation`, `RecordEffectRejection`, `RecordEffectOutcomeUnknown`, `ResubmitEffect`, `RequestEffectRelease`, `RecordEffectReleased`, `ApplyContractCancellation`, `OpenEffectDispute`, `InvestigateEffectDispute`, `EscalateEffectDispute`, `RecordPartnerReview`, `ResolveEffectDispute`, `PublishEffectTypeResolution`) e por consulta (`FindRegulatoryEffect`, `FindRegulatoryEffectsByContract`, `FindRegulatoryEffectsByStatus`, `FindRegulatoryEffectsByAsset`, `FindEffectTypeResolution`); os comandos disparados só por consumo ou por processo não são chamados pelo bff. Não há rota REST de Effects: os webhooks de entrada de infraestruturas (91 §5.1) são do `bff` e chegam ao Gateway.
- **Banco**: database e role `effects`, schema `public`, tabelas `regulatory_effects` e `effect_type_resolutions`, com `tenant_id` à frente da chave. Persistência híbrida: em `regulatory_effects`, colunas tipadas `contract_id` (`FindRegulatoryEffectsByContract`), `status` e `submitted_at` (`FindRegulatoryEffectsByStatus`); em `effect_type_resolutions`, as colunas de condição e `effective_from` (`FindEffectTypeResolution`); o resto vai em `snapshot` `jsonb`, inclusive `targets` [ABERTA: volume dos alvos e índice por ativo para `FindRegulatoryEffectsByAsset`]. Evidências brutas (payload, resposta, posição) ficam por referência [ABERTA: provider de object storage].
- **Instante**: inteiro de nanossegundos no domínio.
- Diverge do design: 90 §1.3 fala em milissegundos e 92 §1 em envelope JSON com `schemaUri`; vale o DMPF (nanossegundos; Protobuf com CloudEvents montado pelo relay). Os nomes de evento seguem 92 §4.
- Diverge do design: multi-tenancy por `tenant_id` à frente da chave e escopo injetado pelo `ExecutionContext` (ADR-051 do DMPF), não por RLS (00 §4.1). O financiador acompanha o efeito por projeção no tenant dele e o backoffice lê entre tenants [ABERTA: requer extensão do kernel - ADR-051 DMPF].
- Diverge do design: broker Kafka (default do DMPF; confirmação na decisão D4 da fundação), não SQS.

## Políticas transversais

- **Idempotência**: todo comando de entrada corre em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); os disparados por evento têm a deduplicação da inbox (ADR-036). O comando externo de saída usa `idempotencyKey = hash(tenantId, partnerId, operationType, businessKey, payloadVersion)` (00 §7.3), derivada de dados de negócio e nunca de tentativa; reenvio idêntico gera a mesma chave.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Reenvio e pedido de liberação pelo backoffice exigem justificativa, segundo aprovador e consulta prévia (17 §5; 94 C9.3) [ABERTA: dupla aprovação no kernel].
- **Auditoria**: pela trilha de auditoria do kernel (FND-08) e pelo Ledger por outbox (12 §2); contrato, payload, retorno e posição final ficam reconstruíveis por `correlationId` (critério do PRD).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `effects` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO um contrato submetido, tipo resolvido, capacidade e mandato ativos
QUANDO PlanRegulatoryEffect chega com alvos de valor positivo
ENTÃO o efeito nasce planned com alvos pending e RegulatoryEffectPlanned vai à outbox
DADO contractTechnicallyComplete falso
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/contract-incomplete e nada é escrito
DADO providerEffectCode vazio na resolução
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/effect-type-unresolved
DADO requiredFieldsSatisfied falso
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/required-fields-missing
DADO capabilityEnabled falso
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/capability-not-enabled
DADO operationalMandateActive falso
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/mandate-not-active
DADO escopo enumerated sem alvo de valor positivo
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/no-targets
DADO escopo rule_based e futureReceivablesSupported falso
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/rule-based-scope-unsupported
DADO idempotencyKey vazia
QUANDO PlanRegulatoryEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/idempotency-key-missing

DADO um efeito planned
QUANDO StartPrevalidation chega
ENTÃO o efeito vai a prevalidating
DADO um efeito fora de planned
QUANDO StartPrevalidation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-planned

DADO um efeito prevalidating e todos os alvos consistentes
QUANDO RecordPrevalidation chega
ENTÃO o efeito vai a submitted, submittedAt = at, e EffectSubmitted vai à outbox
DADO um efeito prevalidating com um alvo divergente
QUANDO RecordPrevalidation chega
ENTÃO o efeito vai a rejected com rejectionOrigin prevalidation, nada é enviado e EffectRejected é emitido
DADO um efeito fora de prevalidating
QUANDO RecordPrevalidation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-prevalidating
DADO um efeito prevalidating e um alvo sem resultado
QUANDO RecordPrevalidation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/prevalidation-incomplete

DADO um efeito submitted
QUANDO RecordEffectAcknowledged chega
ENTÃO o efeito vai a acknowledged e EffectAcknowledged é emitido
DADO um efeito fora de submitted
QUANDO RecordEffectAcknowledged chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-submitted

DADO um efeito acknowledged e confirmação corroborada de todos os alvos
QUANDO RecordEffectConfirmation chega
ENTÃO o efeito vai a effective com confirmedAmountMinor por alvo e EffectConfirmed é emitido
DADO um efeito acknowledged e confirmação de parte dos alvos
QUANDO RecordEffectConfirmation chega
ENTÃO o efeito vai a partially_effective, os alvos mantêm o resultado individual e EffectPartiallyConfirmed é emitido
DADO um efeito submitted e todos os alvos rejeitados
QUANDO RecordEffectConfirmation chega
ENTÃO o efeito vai a rejected com código e texto originais do parceiro e EffectRejected é emitido
DADO um efeito em planned
QUANDO RecordEffectConfirmation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-awaiting-confirmation
DADO corroboratedByStateQuery falso
QUANDO RecordEffectConfirmation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/confirmation-not-corroborated
DADO externalReference vazia
QUANDO RecordEffectConfirmation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/external-reference-missing
DADO um resultado para targetId inexistente
QUANDO RecordEffectConfirmation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/unknown-target
DADO um alvo confirmed com confirmedAmountMinor maior que requestedAmountMinor
QUANDO RecordEffectConfirmation chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/invalid-confirmed-amount

DADO um efeito acknowledged
QUANDO RecordEffectRejection chega com final_validation
ENTÃO os alvos vão a rejected, o efeito vai a rejected com origem partner e EffectRejected é emitido
DADO um efeito submitted sem resposta
QUANDO RecordEffectOutcomeUnknown chega
ENTÃO o efeito vai a unknown_outcome, os alvos a unknown e EffectOutcomeUnknown é emitido com blocksFunding verdadeiro

DADO um efeito rejected por validação
QUANDO ResubmitEffect chega com validation_corrected, consulta prévia e chave nova
ENTÃO o efeito volta a planned com payloadVersion maior e RegulatoryEffectPlanned é emitido
DADO um efeito unknown_outcome cuja consulta de estado provou que o comando não chegou
QUANDO ResubmitEffect chega com not_received_confirmed e a mesma chave
ENTÃO o efeito volta a planned, sem nova chave
DADO um efeito effective
QUANDO ResubmitEffect chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-resubmittable
DADO um efeito rejected por indisponibilidade
QUANDO ResubmitEffect chega com validation_corrected
ENTÃO rejeita com effects-orchestrator/regulatory-effect/rejection-not-correctable
DADO um efeito unknown_outcome
QUANDO ResubmitEffect chega sem priorStateQueryId
ENTÃO rejeita com effects-orchestrator/regulatory-effect/state-query-missing
DADO um efeito rejected por validação
QUANDO ResubmitEffect chega com validation_corrected e a chave antiga
ENTÃO rejeita com effects-orchestrator/regulatory-effect/idempotency-key-reused

DADO um efeito effective
QUANDO RequestEffectRelease chega com obligation_fulfilled
ENTÃO o efeito vai a release_pending e nenhum evento é emitido
DADO um efeito submitted
QUANDO RequestEffectRelease chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-releasable
DADO um efeito release_pending e confirmação corroborada
QUANDO RecordEffectReleased chega
ENTÃO o efeito vai a released e EffectReleased é emitido
DADO um efeito effective
QUANDO RecordEffectReleased chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-release-pending

DADO um efeito planned
QUANDO ApplyContractCancellation chega
ENTÃO o efeito vai a cancelled sem enviar nada
DADO um efeito submitted
QUANDO ApplyContractCancellation chega
ENTÃO o efeito segue submitted com cancellationPending verdadeiro e o comando em trânsito não é interrompido

DADO um efeito effective
QUANDO OpenEffectDispute chega com o gatilho holder_divergence
ENTÃO a disputa nasce opened com severidade critical, o efeito vai a disputed guardando o status anterior e EffectDisputed é emitido
DADO um efeito planned
QUANDO OpenEffectDispute chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/not-disputable
DADO uma disputa opened
QUANDO InvestigateEffectDispute chega
ENTÃO a disputa vai a investigating
DADO uma disputa investigating
QUANDO EscalateEffectDispute chega
ENTÃO a disputa vai a escalated_to_partner
DADO uma disputa escalated_to_partner
QUANDO RecordPartnerReview chega
ENTÃO a disputa vai a partner_review
DADO uma disputa partner_review e nenhuma outra aberta
QUANDO ResolveEffectDispute chega com resolved_partner
ENTÃO a disputa é encerrada e o efeito volta ao status anterior
DADO uma disputa partner_review
QUANDO ResolveEffectDispute chega com unresolved
ENTÃO o efeito segue disputed
DADO um disputeId inexistente
QUANDO InvestigateEffectDispute chega
ENTÃO rejeita com effects-orchestrator/regulatory-effect/dispute-not-found
DADO uma disputa opened
QUANDO ResolveEffectDispute chega com resolved_partner
ENTÃO rejeita com effects-orchestrator/regulatory-effect/invalid-dispute-transition

DADO uma resolução com providerEffectCode preenchido
QUANDO PublishEffectTypeResolution chega
ENTÃO a versão nasce imutável com publishedAt = at
DADO providerEffectCode vazio
QUANDO PublishEffectTypeResolution chega
ENTÃO rejeita com effects-orchestrator/effect-type-resolution/provider-code-missing
DADO uma versão já publicada
QUANDO PublishEffectTypeResolution chega com a mesma versão
ENTÃO rejeita com effects-orchestrator/effect-type-resolution/version-already-published

DADO um efeito unknown_outcome e dois efeitos effective
QUANDO FindRegulatoryEffectsByStatus consulta unknown_outcome
ENTÃO devolve exatamente um efeito, fora de qualquer UoW
```

## Escopo fora

- **REST do bff**: sem rota de Effects na Client API nem na Partner API (91); os webhooks de entrada de infraestruturas (91 §5.1) são do `bff`.
- **Protocolos e adaptadores**: tradução de formatos, taxonomia de erros, `CapabilityProfile` e simulador são do Receivables Gateway (04); aqui só a orquestração do ciclo do efeito.
- **Prioridade jurídica entre efeitos conflitantes**: a plataforma não a decide (PRD, invariante); a disputa é registrada e escalada.
- **Desembolso e confirmação parcial a jusante**: o caminho de instrução proporcional ou de revisão contratual é do Funding (10) e do caso `EFFECT_PARTIAL` do Ops (17); aqui só o evento `EffectPartiallyConfirmed`.
- **Quitação e conciliação**: a decisão de pedir a liberação é do Settlement (11); aqui só a execução, por `RequestEffectRelease`.
- **Recebíveis futuros** (`rule_based`, extensão automática, req. 8 do PRD): fase 4; o modelo admite o modo, mas o comando de extensão não é especificado até B11/D6 [A CONFIRMAR: B11, F10].
- **Tipos sem código** (portabilidade, endosso-penhor, trava de domicílio): só existem após #2 (B3).
- **Caso operacional, fila e notificação**: do Ops (17) e de Notificações (16), a partir dos eventos publicados.
- **Dossiê**: preservação e composição reproduzível são do Compliance Ledger (12).
