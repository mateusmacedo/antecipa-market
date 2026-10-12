---
id: SPEC-3AAD37HR
slug: settlement-reconciliation
title: Bounded context — settlement-reconciliation
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-CWYGNY82]
ticket_url: https://linear.app/mmda/issue/DEVS-92/spec-3aad37hr-bounded-context-settlement-reconciliation
subtask_urls: []
created: 2026-10-12
---

# SPEC-3AAD37HR: Bounded context — settlement-reconciliation

## Identidade

- **bounded_context**: `settlement-reconciliation`
- **name**: `settlement` (módulo `apps/backend/settlement`, package Protobuf `company.settlement.event.v1`)
- **Linguagem ubíqua**:
  - *Expectativa de liquidação* (`SettlementExpectation`) - o fluxo esperado de um ativo sob um efeito confirmado.
  - *Ocorrência* (`SettlementOccurrence`) - fato bruto informado por fonte externa; só se acrescenta, nunca se altera (append-only).
  - *Pago, baixado, liquidado e repassado* - quatro fatos distintos (11 §2): o devedor pagou; a fonte oficial registrou a quitação do ativo; os recursos foram compensados; os recursos chegaram ao beneficiário do efeito. O repasse, não a baixa, é a condição de liberação.
  - *Execução de conciliação* (`ReconciliationRun`) - comparação versionada de uma janela e de uma fonte; reprocessar gera nova execução.
  - *Item* (`ReconciliationItem`) - correspondência ou divergência, com a camada de correspondência usada.
  - *Caso* (`ReconciliationCase`) - tratamento de divergência, com ajustes e resolução.
  - *Ajuste* (`ReconciliationAdjustment`) - registro vinculado, com justificativa e aprovadores; nunca altera o dado bruto.
  - *Alocação* (`SettlementAllocation`) - rateio entre principal, encargos e saldo livre, informado pela fonte competente; a plataforma não o calcula (11 §4).
  - *Janela de contestação* - prazo em que uma liquidação ainda pode ser estornada por chargeback.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/11-prd-settlement-reconciliation.md`; `docs/design/11-design-settlement-reconciliation.md` §2 a §8; `docs/design/90-modelo-de-dados.md` §4; `docs/design/92-schemas-eventos.md` §3.6 e §4; `docs/prd/14-catalogo-contratos-eventos.md` (sagas); `docs/design/17-design-backoffice.md` §4 e §5; `docs/design/94-seguranca-threat-model.md` T7; `docs/design/05-design-portfolio.md` §6 e §7.
- **Situação**: `parcial` - expectativas, ocorrências append-only, correspondência em camadas, casos e ajustes são construíveis; a liberação do efeito depende da janela de contestação (#10), a ingestão do formato dos arquivos (B8) e a correspondência determinística do esquema de identidade da UR (#3).
- **Fase de entrega**: 1.
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-CWYGNY82`, Funding); a cadeia transitiva (effects, contracting, portfolio, gateway) fica implícita. Upstream duro: Portfolio (`AssetPositionChanged`, `ExternalPositionDiverged`), Effects (`EffectConfirmed`), Funding (`DisbursementConfirmed`, `DisbursementReversed`) e Gateway (`SettlementQueryPort`). Upstream brando: Partner Management (fontes e janelas). As arestas de retroalimentação (Effects consome `EffectReleaseRequested`; Portfolio consome `SettlementReported` e `AssetSettled`) não entram em `depends_on`.
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
|---|---|---|
| #10 (B9; 11:R9) | janela de contestação por arranjo | `RequestEffectRelease` (`contestationWindowEndsAt`) |
| B8 (11:R10) | formato dos arquivos de liquidação por fonte | ingestão em `RecordSettlementOccurrence` |
| B13 (11:R13) | quem informa o rateio com efeitos concorrentes | `reportedAllocation`; casos de múltiplos efeitos |
| #3 (B4; 05:P8) | esquema de identidade da UR | correspondência das camadas 1 e 2 |
| E2 (11:R11) | limiares de alçada de ajuste e de severidade | `ProposeAdjustment`, `approvalTier` |
| 11:R12 | tolerância de data e de valor | `AdvanceExpectationSchedule`, severidade das divergências |
| #1 (A1) | autorização do BCB (transversal) | o contexto inteiro |

- **Decisões de produto abertas**: granularidade da liberação, por efeito ou por ativo (11 §7 × 09 §2) [ABERTA: 11:§7]; o esquema de alçada de ajuste (11 §6, §6.1 e 17 §5 divergem); janela de contestação como parâmetro obrigatório, sem valor padrão.
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída e `SPEC-CWYGNY82` com `DisbursementConfirmed` publicado. Do que trava o núcleo, #3 (B4) vale para as camadas 1 e 2; as camadas 3 a 5 e o restante do modelo avançam sem ele. #10 não impede `planning`: sem janela configurada, `RequestEffectRelease` nunca é aceito (falha fechada, a postura conservadora de 11 §7).

## Agregados

`SettlementAllocation` é valor de `SettlementOccurrence`; `ReconciliationAdjustment` e `Approval` são entidades internas de `ReconciliationCase`. Instantes `*At` valem 0 antes do fato; datas puras são `string` `AAAA-MM-DD`; dinheiro é `*Minor: int` com `currency: string`.

### SettlementExpectation

- **Identidade**: `natural` por `targetId` (um fluxo por alvo do efeito, ou seja, por ativo e contrato).
- **Campos**:
  - Origem: `targetId: id`, `effectId: id`, `contractId: id`, `assetType: string`, `assetId: id`, `assetReference: string` (identificador oficial, camada 1), `effectReference: string` (camada 2), `registryPartnerId: id` (fonte), `beneficiaryOrganizationId: id`, `currency: string`, `expectedAmountMinor: int`, `expectedDate: string`.
  - Acumuladores (11 §3.3): `paidAmountMinor: int`, `liquidatedAmountMinor: int`, `reversedAmountMinor: int`, `netSettledAmountMinor: int`, `transferredAmountMinor: int`, `writtenOffReported: bool`, `occurrences: list<OccurrenceRef>` (append-only, coleção indispensável) com `OccurrenceRef` = `occurrenceKey: string`, `kind`, `reversal: bool`, `amountMinor: int`, `appliedAt: instant`.
  - `status`, `statusBeforeMismatch: string`, `createdAt: instant`.
- **Estados**: `expected`, `due`, `partially_settled`, `settled`, `reconciled`, `overdue`, `mismatch`, `reversed`, `disputed`, `write_off_reported`.
- **Transições permitidas**:
  - (novo) → `expected` - por `CreateSettlementExpectation`
  - `expected` → `due`; `expected`, `due` → `overdue` - por `AdvanceExpectationSchedule`
  - qualquer status, exceto `mismatch`, `disputed` → `partially_settled`, `settled`, `reversed` ou `write_off_reported` - por `ApplyOccurrence`, derivado dos acumuladores
  - `settled` → `reconciled` - por `ConfirmReconciliation`
  - qualquer status, exceto `mismatch` → `mismatch` - por `FlagExpectationMismatch`; `mismatch` → `statusBeforeMismatch` - por `ClearExpectationMismatch`
  - `disputed` não tem gatilho no design [ABERTA: 11 PRD lista `DISPUTED` sem origem]
- **Invariantes de estado**:
  - `netSettledAmountMinor = liquidatedAmountMinor - reversedAmountMinor`; `0 < net < expectedAmountMinor` é `partially_settled`; `net ≥ expectedAmountMinor` é `settled`; estorno com `reversedAmountMinor > 0` depois de `settled` ou `reconciled` leva a `reversed` e reabre a expectativa (11 §3.3, §8). [ABERTA: 11:§2 × §3.3 - qual `kind` alimenta `netSettled`; proposto: `liquidated`]
  - Pagamento relatado (`paid`) não altera `netSettledAmountMinor`; a baixa (`written_off`) só marca `writtenOffReported`; o repasse acumula `transferredAmountMinor`, base da liberação.
  - Ocorrências só se acrescentam; estorno é ocorrência de sinal oposto, nunca subtração (11 §3.3) - `settlement-reconciliation/settlement-expectation/invalid-amount`.
  - `reconciled` exige todas as ocorrências conciliadas - `settlement-reconciliation/settlement-expectation/unmatched-occurrences`.

### SettlementOccurrence

- **Identidade**: `natural` por `occurrenceKey` (identificador da linha na fonte, ou hash do conteúdo bruto; a ingestão repetida não duplica, 11 §5).
- **Campos**: `occurrenceKey: string`, `providerId: id`, `sourceSystem: acquirer | registry | bookkeeper | depository | position`, `kind: paid | written_off | liquidated | transferred`, `reversal: bool`, `amountMinor: int`, `currency: string`, `settlementDate: string`, `beneficiaryOrganizationId: id`, `assetReference: string`, `effectReference: string`, `reportedAllocation` (`SettlementAllocation` = `allocationReported: bool`, `principalMinor: int`, `chargesMinor: int`, `freeBalanceMinor: int`; todos 0 quando a fonte informa só o total), `sourceAsOf: instant`, `rawPayloadEvidenceId: id`, `rawPayloadHash: string`, `ingestedAt: instant`. O bruto fica no armazenamento [ABERTA: provider de object storage].
- **Estados**: nenhum; imutável.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: o bruto é persistido antes de qualquer correspondência - `settlement-reconciliation/settlement-occurrence/raw-payload-missing`; `amountMinor > 0`, o sinal vem de `reversal` - `settlement-reconciliation/settlement-occurrence/invalid-amount`.

### ReconciliationRun

- **Identidade**: `generated`.
- **Campos**: `registryPartnerId: id` (escopo por parceiro, isola o atraso de uma fonte), `windowFrom: string`, `windowTo: string`, `version: int`, `reprocessOf: id`, `status: open | closed`, `expectationCount: int`, `matchedCount: int`, `mismatchCount: int`, `unmatchedOccurrenceCount: int`, `openedAt: instant`, `closedAt: instant`.
- **Estados**: `open`, `closed`.
- **Transições permitidas**: (novo) → `open` - por `OpenReconciliationRun`; `open` → `closed` - por `CloseReconciliationRun`.
- **Invariantes de estado**: reprocessar cria nova execução com `version` maior e preserva a anterior (req. 10 do PRD) - `settlement-reconciliation/reconciliation-run/invalid-reprocess-version`; `windowFrom` não passa de `windowTo` - `settlement-reconciliation/reconciliation-run/invalid-window`; só se fecha uma execução aberta - `settlement-reconciliation/reconciliation-run/run-not-open`.

### ReconciliationItem

- **Identidade**: `generated`.
- **Campos**: `runId: id`, `targetId: id`, `occurrenceKey: string`, `outcome: matched | mismatch`, `correspondenceLayer: int` (1 a 4; 0 sem correspondência, 11 §3.2), `detailPending: bool` (camada 4: total conciliado, detalhamento pendente), `mismatchType: AMOUNT_DIVERGENCE | DATE_DIVERGENCE | BENEFICIARY_DIVERGENCE | UNEXPECTED_SETTLEMENT | MISSING_SETTLEMENT | DUPLICATE_SETTLEMENT | CHARGEBACK` (o último é proposto, 11 §8 [ABERTA: 92:§3.6 enumera seis tipos]), `severity: low | medium | high | critical`, `expectedAmountMinor: int`, `expectedDate: string`, `expectedBeneficiaryId: id`, `reportedAmountMinor: int`, `reportedDate: string`, `reportedBeneficiaryId: id`, `sourceProviderId: id`, `sourceAsOf: instant`, `differenceMinor: int`, `caseId: id`, `recordedAt: instant`.
- **Estados**: nenhum; imutável.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: o item registra a camada usada, e a camada 3 só vale com combinação única (11 §3.2) - `settlement-reconciliation/reconciliation-item/invalid-layer`; divergência referencia um caso - `settlement-reconciliation/reconciliation-item/mismatch-without-case`; `BENEFICIARY_DIVERGENCE` é sempre `critical`, qualquer valor (11 §6) - `settlement-reconciliation/reconciliation-item/beneficiary-divergence-must-be-critical`.

### ReconciliationCase

- **Identidade**: `generated`.
- **Campos**: `origin: reconciliation_item | external_position_divergence`, `mismatchType`, `severity`, `financialImpactMinor: int`, `effectId: id`, `contractId: id`, `targetId: id`, `status: open | pending_approval | escalated | resolved`, `resolutionCode: string`, `resolutionJustification: string`, `openedAt`, `escalatedAt`, `resolvedAt: instant`, `adjustments: list<ReconciliationAdjustment>` (coleção indispensável, 11 §6.1) com `ReconciliationAdjustment` = `adjustmentId: id`, `amountMinor: int`, `justification: string`, `proposedBy: id`, `approvalTier: operator | supervisor | financial_owner`, `requiredRoles: list<string>`, `approvals: list<Approval>` (`Approval` = `role: string`, `approverId: id`, `approvedAt: instant`), `status: pending | approved`, `proposedAt: instant`. Responsável e prazo do caso vivem no `OperationalCase` do Ops [ABERTA: 11 PRD dá ao `ReconciliationCase` "responsável e SLA"; 17 §4 cria `RECONCILIATION_MISMATCH` no Ops].
- **Estados**: `open`, `pending_approval`, `escalated`, `resolved`.
- **Transições permitidas**: (novo) → `open` - por `OpenReconciliationCase`; `open` → `pending_approval` - por `ProposeAdjustment`; `pending_approval` → `open` - por `ApproveAdjustment` quando todos os papéis exigidos aprovaram; `open`, `pending_approval` → `escalated` - por `EscalateReconciliationCase`; `open`, `escalated` → `resolved` - por `ResolveReconciliationCase`.
- **Invariantes de estado**: caso resolvido não se reabre - `settlement-reconciliation/reconciliation-case/case-closed`; ajuste exige justificativa - `settlement-reconciliation/reconciliation-case/justification-missing`; suspeita de fraude escala a risco em vez de ajustar - `settlement-reconciliation/reconciliation-case/fraud-suspected-escalate`; quem propõe não aprova - `settlement-reconciliation/reconciliation-case/self-approval`; só se resolve sem ajuste pendente - `settlement-reconciliation/reconciliation-case/adjustment-pending`. Ajuste nunca altera o dado bruto; a visão conciliada é bruto mais ajustes aprovados (11 §6.1).

### EffectRelease

- **Identidade**: `natural` por `effectId` (acompanhamento da liberação de um efeito, do desembolso ao pedido; não consta do PRD [ABERTA: 11:§7 não nomeia o agregado que avalia a condição composta]).
- **Campos**: `effectId: id`, `contractId: id`, `status: tracking | requested`, `disbursementConfirmedAt: instant`, `disbursementReversed: bool`, `obligationFulfilledAt: instant`, `transferConfirmedAt: instant`, `contestationWindowEndsAt: instant`, `requestCause: automatic | operator`, `requestedAt: instant`.
- **Estados**: `tracking`, `requested`.
- **Transições permitidas**: (novo) → `tracking` - por `TrackEffectRelease`; `tracking` → `requested` - por `RequestEffectRelease`.
- **Invariantes de estado**: a liberação só é pedida com obrigação cumprida, repasse confirmado, janela de contestação decorrida, sem estorno e sem disputa aberta (11 §7), e nunca depois de estorno do desembolso. Liberar por baixa liberaria a garantia antes de o credor receber - `settlement-reconciliation/effect-release/transfer-not-confirmed`.

## Comandos (UPRs)

O disparo vem de evento consumido, de ingestão de fonte (arquivo, API ou webhook, pelo `bff` ou pelo Gateway), de processo da execução de conciliação (passos 1 a 10 de 11 §5) ou de chamada gRPC do backoffice. Os fatos de outros contextos entram como entrada, lidos fora da UoW. A correspondência em camadas (11 §3.2) é decidida pelo processo da execução: o domínio registra a camada usada, não a infere. O encadeamento entre agregados do contexto é do processo [ABERTA: encadeamento intra-contexto não normatizado no kernel].

### CreateSettlementExpectation

- **Agregado**: `SettlementExpectation`; **inicializa se ausente**
- **Disparo**: consumo de `EffectConfirmed`, um comando por alvo confirmado
- **Entrada**: `targetId: id`, `effectId: id`, `contractId: id`, `assetType: string`, `assetId: id`, `assetReference: string`, `effectReference: string`, `registryPartnerId: id`, `beneficiaryOrganizationId: id`, `currency: string`, `expectedAmountMinor: int`, `expectedDate: string`
- **Pré-condições**: `expectedAmountMinor > 0` → `settlement-reconciliation/settlement-expectation/invalid-expected-amount`
- **Efeitos sobre o estado**: quando ausente: `status ← expected`; acumuladores `← 0`; campos `← cmd`; `createdAt ← at`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: `SettlementExpected` (só quando inicializa)
- **Resposta**: `targetId: id`

### ApplyOccurrence

- **Agregado**: `SettlementExpectation`; exige existente
- **Disparo**: processo da execução, após a correspondência das camadas 1 a 4
- **Entrada**: `occurrenceKey: string`, `kind`, `reversal: bool`, `amountMinor: int`, `appliedAt: instant`
- **Pré-condições**: `amountMinor > 0` → `settlement-reconciliation/settlement-expectation/invalid-amount`
- **Efeitos sobre o estado**: `occurrenceKey` já aplicada: aceita sem alterar. Senão acrescenta a ocorrência e atualiza o acumulador do `kind` (`paid`, `liquidated` com `reversal` somando em `reversedAmountMinor`, `transferred`, `written_off`); `netSettledAmountMinor ← liquidated - reversed`; `status ←` derivado (`partially_settled`, `settled`, `reversed`, `write_off_reported`) conforme os invariantes
- **Eventos emitidos**: `AssetPartiallySettled` (ao entrar em `partially_settled`) ou `AssetSettled` (ao entrar em `settled`); nenhum nos demais casos [ABERTA: `REVERSED` (chargeback) sem evento no PRD nem em 92]
- **Resposta**: `targetId: id`, `status`

### AdvanceExpectationSchedule

- **Agregado**: `SettlementExpectation`; exige existente
- **Disparo**: processo periódico do contexto
- **Entrada**: `today: string`, `toleranceDays: int` [A CONFIRMAR: E2, 11:R12]
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: `expected` com `today ≥ expectedDate`: `status ← due`. `expected` ou `due` com `net = 0` e `today` além de `expectedDate + toleranceDays`: `status ← overdue`. Demais status: aceita sem alterar
- **Eventos emitidos**: nenhum (a ausência de liquidação além da tolerância vira item `MISSING_SETTLEMENT` na execução)
- **Resposta**: `targetId: id`, `status`

### ConfirmReconciliation

- **Agregado**: `SettlementExpectation`; exige existente
- **Disparo**: processo da execução, quando todas as ocorrências da expectativa estão conciliadas
- **Entrada**: `allOccurrencesMatched: bool`
- **Pré-condições**:
  - `status = settled` → `settlement-reconciliation/settlement-expectation/not-settled`
  - `allOccurrencesMatched` → `settlement-reconciliation/settlement-expectation/unmatched-occurrences`
- **Efeitos sobre o estado**: `status ← reconciled`
- **Eventos emitidos**: nenhum (cada correspondência já emitiu `ReconciliationMatched`)
- **Resposta**: `targetId: id`

### FlagExpectationMismatch

- **Agregado**: `SettlementExpectation`; exige existente
- **Disparo**: processo da execução, ao registrar um item `mismatch`
- **Entrada**: `itemId: id`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: fora de `mismatch`: `statusBeforeMismatch ← status`, `status ← mismatch`. Em `mismatch`: aceita sem alterar
- **Eventos emitidos**: nenhum (o evento é do item)
- **Resposta**: `targetId: id`

### ClearExpectationMismatch

- **Agregado**: `SettlementExpectation`; exige existente
- **Disparo**: processo do contexto, após `ReconciliationCaseResolved`
- **Entrada**: `caseId: id`
- **Pré-condições**: `status = mismatch` → `settlement-reconciliation/settlement-expectation/not-in-mismatch`
- **Efeitos sobre o estado**: `status ← statusBeforeMismatch`
- **Eventos emitidos**: nenhum
- **Resposta**: `targetId: id`

### RecordSettlementOccurrence

- **Agregado**: `SettlementOccurrence`; **inicializa se ausente**
- **Disparo**: ingestão da fonte (arquivo, API ou webhook; 91 §5.1, persistir o bruto antes de interpretar); consumo de `AssetPositionChanged` com `sourceSystem = position` [ABERTA: sem schema em 92; a ambiguidade de `removed` em 05 §6 exige a janela da consulta]
- **Entrada**: `occurrenceKey: string`, `providerId: id`, `sourceSystem`, `kind`, `reversal: bool`, `amountMinor: int`, `currency: string`, `settlementDate: string`, `beneficiaryOrganizationId: id`, `assetReference: string`, `effectReference: string`, `reportedAllocation`, `sourceAsOf: instant`, `rawPayloadEvidenceId: id`, `rawPayloadHash: string`
- **Pré-condições**:
  - `rawPayloadHash` não vazio → `settlement-reconciliation/settlement-occurrence/raw-payload-missing`
  - `amountMinor > 0` → `settlement-reconciliation/settlement-occurrence/invalid-amount`
- **Efeitos sobre o estado**: quando ausente: campos `← cmd`; `ingestedAt ← at`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: `SettlementReported` (só quando inicializa)
- **Resposta**: `occurrenceKey: string`

### OpenReconciliationRun

- **Agregado**: `ReconciliationRun`; **cria**
- **Disparo**: processo diário, após a janela de corte de cada fonte (11 §5); chamada gRPC do backoffice para reprocessar
- **Entrada**: `registryPartnerId: id`, `windowFrom: string`, `windowTo: string`, `reprocessOf: id`, `version: int`, `previousVersion: int` (fato lido da execução anterior)
- **Pré-condições**:
  - `windowFrom ≤ windowTo` → `settlement-reconciliation/reconciliation-run/invalid-window`
  - com `reprocessOf`, `version = previousVersion + 1` → `settlement-reconciliation/reconciliation-run/invalid-reprocess-version`
- **Efeitos sobre o estado**: `status ← open`; `openedAt ← at`; campos `← cmd`
- **Eventos emitidos**: nenhum
- **Resposta**: `runId: id`

### CloseReconciliationRun

- **Agregado**: `ReconciliationRun`; exige existente
- **Disparo**: processo da execução (passo 9)
- **Entrada**: `expectationCount: int`, `matchedCount: int`, `mismatchCount: int`, `unmatchedOccurrenceCount: int`
- **Pré-condições**: `status = open` → `settlement-reconciliation/reconciliation-run/run-not-open`
- **Efeitos sobre o estado**: `status ← closed`; sumário `← cmd`; `closedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `runId: id`

### RecordReconciliationItem

- **Agregado**: `ReconciliationItem`; **cria**
- **Disparo**: processo da execução (passos 5 a 7)
- **Entrada**: `runId: id`, `targetId: id`, `occurrenceKey: string`, `outcome`, `correspondenceLayer: int`, `detailPending: bool`, `mismatchType`, `severity`, valores esperados e informados, `differenceMinor: int`, `caseId: id`, `layer3Unique: bool` (fato: combinação única no conjunto)
- **Pré-condições**:
  - camada entre 1 e 4 em `matched`, e a camada 3 só com `layer3Unique` → `settlement-reconciliation/reconciliation-item/invalid-layer`
  - `mismatch` com `mismatchType` e `caseId` → `settlement-reconciliation/reconciliation-item/mismatch-without-case`
  - `BENEFICIARY_DIVERGENCE` com `severity = critical` → `settlement-reconciliation/reconciliation-item/beneficiary-divergence-must-be-critical`
- **Efeitos sobre o estado**: campos `← cmd`; `recordedAt ← at`
- **Eventos emitidos**: `ReconciliationMatched` (`matched`) ou `ReconciliationMismatchDetected` (`mismatch`)
- **Resposta**: `itemId: id`

### OpenReconciliationCase

- **Agregado**: `ReconciliationCase`; **cria**
- **Disparo**: processo da execução (passo 8); consumo de `ExternalPositionDiverged` com `origin = external_position_divergence`
- **Entrada**: `origin`, `mismatchType`, `severity`, `financialImpactMinor: int`, `effectId: id`, `contractId: id`, `targetId: id`
- **Pré-condições**: `BENEFICIARY_DIVERGENCE` com `severity = critical` → `settlement-reconciliation/reconciliation-case/beneficiary-divergence-must-be-critical`
- **Efeitos sobre o estado**: `status ← open`; `openedAt ← at`; campos `← cmd`
- **Eventos emitidos**: nenhum [ABERTA: caso de origem `external_position_divergence` não tem item, logo não tem `ReconciliationMismatchDetected`]
- **Resposta**: `caseId: id`

### ProposeAdjustment

- **Agregado**: `ReconciliationCase`; exige existente
- **Disparo**: chamada gRPC do backoffice (17 §5)
- **Entrada**: `adjustmentId: id`, `amountMinor: int`, `justification: string`, `approvalTier: operator | supervisor | financial_owner` [A CONFIRMAR: E2, R11], `proposedBy: id`, `suspectedFraud: bool`
- **Pré-condições**:
  - `status = open` → `settlement-reconciliation/reconciliation-case/case-not-open`
  - `suspectedFraud` falso (suspeita escala a risco, qualquer faixa) → `settlement-reconciliation/reconciliation-case/fraud-suspected-escalate`
  - `justification` não vazia → `settlement-reconciliation/reconciliation-case/justification-missing`
- **Efeitos sobre o estado**: acrescenta o ajuste `pending` com `requiredRoles` da faixa (`operator`; `operator` e `supervisor`; os dois mais `financial_owner`, 11 §6.1); `status ← pending_approval`. O dado bruto não é tocado
- **Eventos emitidos**: nenhum (trilha de auditoria do kernel)
- **Resposta**: `adjustmentId: id`

### ApproveAdjustment

- **Agregado**: `ReconciliationCase`; exige existente
- **Disparo**: chamada gRPC do backoffice
- **Entrada**: `adjustmentId: id`, `approverId: id`, `approverRole: string`
- **Pré-condições**:
  - o ajuste existe e está `pending` → `settlement-reconciliation/reconciliation-case/adjustment-not-pending`
  - `approverId` diferente de `proposedBy` → `settlement-reconciliation/reconciliation-case/self-approval`
  - `approverRole` exigido e ainda sem aprovação → `settlement-reconciliation/reconciliation-case/approver-role-not-required`
- **Efeitos sobre o estado**: acrescenta a aprovação; com todos os papéis exigidos, `ajuste.status ← approved` e `status ← open`
- **Eventos emitidos**: nenhum
- **Resposta**: `adjustmentId: id`, `status`

### EscalateReconciliationCase

- **Agregado**: `ReconciliationCase`; exige existente
- **Disparo**: chamada gRPC do backoffice (suspeita de fraude ou `BENEFICIARY_DIVERGENCE`)
- **Entrada**: `reason: string`
- **Pré-condições**: `status` em `open` ou `pending_approval` → `settlement-reconciliation/reconciliation-case/case-closed`
- **Efeitos sobre o estado**: `status ← escalated`; `escalatedAt ← at`
- **Eventos emitidos**: `ReconciliationCaseEscalated` [ABERTA: evento proposto; o PRD e 92 §4 não têm sinal de escalonamento a risco além de `ReconciliationMismatchDetected`]
- **Resposta**: `caseId: id`

### ResolveReconciliationCase

- **Agregado**: `ReconciliationCase`; exige existente
- **Disparo**: chamada gRPC do backoffice
- **Entrada**: `resolutionCode: string`, `justification: string`
- **Pré-condições**:
  - `status` diferente de `resolved` → `settlement-reconciliation/reconciliation-case/case-closed`
  - `status` diferente de `pending_approval` → `settlement-reconciliation/reconciliation-case/adjustment-pending`
  - `justification` não vazia → `settlement-reconciliation/reconciliation-case/justification-missing`
- **Efeitos sobre o estado**: `status ← resolved`; resolução `← cmd`; `resolvedAt ← at`
- **Eventos emitidos**: `ReconciliationCaseResolved`
- **Resposta**: `caseId: id`

### TrackEffectRelease

- **Agregado**: `EffectRelease`; **inicializa se ausente**
- **Disparo**: consumo de `DisbursementConfirmed`
- **Entrada**: `effectId: id`, `contractId: id`, `confirmedAt: instant`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: quando ausente: `status ← tracking`; `disbursementConfirmedAt ← cmd.confirmedAt`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: nenhum
- **Resposta**: `effectId: id`

### RecordFundingReversal

- **Agregado**: `EffectRelease`; exige existente
- **Disparo**: consumo de `DisbursementReversed`
- **Entrada**: `effectId: id`, `phase: before_confirmation | after_confirmation`, `reversedAt: instant`
- **Pré-condições**: acompanhamento existente → `settlement-reconciliation/effect-release/not-tracked`
- **Efeitos sobre o estado**: `disbursementReversed ← true`
- **Eventos emitidos**: nenhum
- **Resposta**: `effectId: id`

### RequestEffectRelease

- **Agregado**: `EffectRelease`; exige existente
- **Disparo**: processo do contexto, quando as expectativas do efeito chegam a `reconciled`; chamada gRPC do backoffice (17 §5: supervisor mais segundo aprovador), sujeita às mesmas condições
- **Entrada**: `obligationFulfilled: bool`, `obligationFulfilledAt: instant`, `transferConfirmed: bool`, `transferConfirmedAt: instant`, `contestationWindowEndsAt: instant` [A CONFIRMAR: #10, B9] (0 = janela não levantada, tratada como aberta), `chargebackSeen: bool`, `effectDisputeOpen: bool`, `requestCause: automatic | operator`
- **Pré-condições**:
  - `status = tracking` → `settlement-reconciliation/effect-release/already-requested`
  - `disbursementReversed` falso → `settlement-reconciliation/effect-release/disbursement-reversed`
  - `obligationFulfilled` (termos do contrato; na cessão definitiva, a liquidação dos ativos) → `settlement-reconciliation/effect-release/obligation-not-fulfilled`
  - `transferConfirmed` (repasse, não baixa) → `settlement-reconciliation/effect-release/transfer-not-confirmed`
  - `contestationWindowEndsAt > 0` e `at ≥ contestationWindowEndsAt` → `settlement-reconciliation/effect-release/contestation-window-open`
  - `chargebackSeen` falso → `settlement-reconciliation/effect-release/chargeback-seen`
  - `effectDisputeOpen` falso → `settlement-reconciliation/effect-release/effect-dispute-open`
- **Efeitos sobre o estado**: `status ← requested`; instantes e `requestCause ← cmd`; `requestedAt ← at`
- **Eventos emitidos**: `EffectReleaseRequested`
- **Resposta**: `effectId: id`

## Eventos de domínio

Só `ReconciliationMismatchDetected` tem schema em 92 §3.6; os demais campos são derivados dos agregados. A condição `ObligationFulfilled`, citada nas sagas de 14, é avaliada dentro de `RequestEffectRelease` e não vira evento [ABERTA: 14 "Sagas principais" cita `ObligationFulfilled`, ausente do catálogo e de 92].

### SettlementExpected

- **Campos**: `targetId: id ← SettlementExpectation.targetId`, `effectId: id ← SettlementExpectation.effectId`, `contractId: id ← SettlementExpectation.contractId`, `assetType: string ← SettlementExpectation.assetType`, `assetId: id ← SettlementExpectation.assetId`, `registryPartnerId: id ← SettlementExpectation.registryPartnerId`, `beneficiaryOrganizationId: id ← SettlementExpectation.beneficiaryOrganizationId`, `currency: string ← SettlementExpectation.currency`, `expectedAmountMinor: int ← SettlementExpectation.expectedAmountMinor`, `expectedDate: string ← SettlementExpectation.expectedDate`, `at: instant ← at`

### SettlementReported

- **Campos**: `occurrenceKey: string ← SettlementOccurrence.occurrenceKey`, `providerId: id ← SettlementOccurrence.providerId`, `sourceSystem ← SettlementOccurrence.sourceSystem`, `kind ← SettlementOccurrence.kind`, `reversal: bool ← SettlementOccurrence.reversal`, `amountMinor: int ← SettlementOccurrence.amountMinor`, `currency: string ← SettlementOccurrence.currency`, `settlementDate: string ← SettlementOccurrence.settlementDate`, `assetReference: string ← SettlementOccurrence.assetReference`, `sourceAsOf: instant ← SettlementOccurrence.sourceAsOf`, `rawPayloadEvidenceId: id ← SettlementOccurrence.rawPayloadEvidenceId`, `at: instant ← at`

### AssetPartiallySettled

- **Campos**: `targetId: id ← SettlementExpectation.targetId`, `effectId: id ← SettlementExpectation.effectId`, `assetId: id ← SettlementExpectation.assetId`, `expectedAmountMinor: int ← SettlementExpectation.expectedAmountMinor`, `netSettledAmountMinor: int ← SettlementExpectation.netSettledAmountMinor`, `at: instant ← at`

### AssetSettled

- **Campos**: `targetId: id ← SettlementExpectation.targetId`, `effectId: id ← SettlementExpectation.effectId`, `contractId: id ← SettlementExpectation.contractId`, `assetId: id ← SettlementExpectation.assetId`, `netSettledAmountMinor: int ← SettlementExpectation.netSettledAmountMinor`, `transferredAmountMinor: int ← SettlementExpectation.transferredAmountMinor`, `at: instant ← at`

### ReconciliationMatched

- **Campos**: `itemId: id ← ReconciliationItem.id`, `runId: id ← ReconciliationItem.runId`, `targetId: id ← ReconciliationItem.targetId`, `occurrenceKey: string ← ReconciliationItem.occurrenceKey`, `correspondenceLayer: int ← ReconciliationItem.correspondenceLayer`, `detailPending: bool ← ReconciliationItem.detailPending`, `at: instant ← at`

### ReconciliationMismatchDetected

- **Campos**: `reconciliationRunId: id ← ReconciliationItem.runId`, `itemId: id ← ReconciliationItem.id`, `caseId: id ← ReconciliationItem.caseId`, `targetId: id ← ReconciliationItem.targetId`, `mismatchType ← ReconciliationItem.mismatchType`, `expected: amountMinor, date, beneficiaryOrganizationId ← ReconciliationItem.expected*`, `reported: amountMinor, date, beneficiaryOrganizationId, sourceProviderId, sourceAsOf ← ReconciliationItem.reported*`, `differenceMinor: int ← ReconciliationItem.differenceMinor`, `severity ← ReconciliationItem.severity`, `detectedAt: instant ← at`. `contractId` e `assetRef` do schema de 92 §3.6 saem do alvo.

### ReconciliationCaseResolved

- **Campos**: `caseId: id ← ReconciliationCase.id`, `resolutionCode: string ← ReconciliationCase.resolutionCode`, `adjustmentCount: int ← size(adjustments)`, `resolvedAt: instant ← ReconciliationCase.resolvedAt`, `at: instant ← at`

### ReconciliationCaseEscalated

- **Campos**: `caseId: id ← ReconciliationCase.id`, `severity ← ReconciliationCase.severity`, `mismatchType ← ReconciliationCase.mismatchType`, `reason: string ← cmd.reason`, `at: instant ← at`

### EffectReleaseRequested

- **Campos**: `effectId: id ← EffectRelease.effectId`, `contractId: id ← EffectRelease.contractId`, `requestCause ← EffectRelease.requestCause`, `obligationFulfilledAt: instant ← EffectRelease.obligationFulfilledAt`, `transferConfirmedAt: instant ← EffectRelease.transferConfirmedAt`, `contestationWindowEndsAt: instant ← EffectRelease.contestationWindowEndsAt`, `requestedAt: instant ← EffectRelease.requestedAt`, `at: instant ← at`

## Consultas

- `FindSettlementExpectation` - filtros: `targetId`; cardinalidade: `one`
- `FindSettlementExpectationsByEffect` - filtros: `effectId`; cardinalidade: `many` (avaliação da condição de liberação)
- `FindSettlementExpectationsForWindow` - filtros: `registryPartnerId`, intervalo de `expectedDate`, `status`; cardinalidade: `many` (passo 2 da execução)
- `FindSettlementOccurrence` - filtros: `occurrenceKey`; cardinalidade: `one`
- `FindSettlementOccurrencesForWindow` - filtros: `providerId`, intervalo de `settlementDate`; cardinalidade: `many` (inclui as sem item, para `UNEXPECTED_SETTLEMENT`)
- `FindReconciliationRun` - filtros: `runId`; cardinalidade: `one`
- `FindReconciliationItemsByRun` - filtros: `runId`, `outcome`; cardinalidade: `many`
- `FindReconciliationItemsByExpectation` - filtros: `targetId`; cardinalidade: `many`
- `FindReconciliationCase` - filtros: `caseId`; cardinalidade: `one` (com ajustes)
- `FindReconciliationCasesByStatus` - filtros: `status`, `severity`; cardinalidade: `many`
- `FindReconciledView` - filtros: `targetId`; cardinalidade: `one` (bruto mais ajustes aprovados, nunca bruto alterado)
- `FindEffectRelease` - filtros: `effectId`; cardinalidade: `one`

## Relações entre agregados

- `SettlementExpectation.targetId` → `EffectTarget` (Effects, dentro de `RegulatoryEffect`)
- `SettlementExpectation.effectId` → `RegulatoryEffect` (Effects); `SettlementExpectation.contractId` → `ContractCase` (Contracting)
- `SettlementExpectation.assetId` → `FinancialAsset` (Portfolio)
- `SettlementExpectation.registryPartnerId`, `SettlementOccurrence.providerId`, `ReconciliationRun.registryPartnerId` → `Partner` (Partner Management)
- `ReconciliationRun.reprocessOf` → `ReconciliationRun`
- `ReconciliationItem.runId` → `ReconciliationRun`; `ReconciliationItem.targetId` → `SettlementExpectation`; `ReconciliationItem.occurrenceKey` → `SettlementOccurrence`; `ReconciliationItem.caseId` → `ReconciliationCase`
- `ReconciliationCase.targetId` → `SettlementExpectation`
- `EffectRelease.effectId` → `RegulatoryEffect` (Effects)

## Integração

- **Publica** (integration events): `SettlementExpected`, `SettlementReported`, `AssetPartiallySettled`, `AssetSettled`, `ReconciliationMatched`, `ReconciliationMismatchDetected`, `ReconciliationCaseResolved`, `ReconciliationCaseEscalated` e `EffectReleaseRequested` - viram `settlement_expected.proto`, `settlement_reported.proto`, `asset_partially_settled.proto`, `asset_settled.proto`, `reconciliation_matched.proto`, `reconciliation_mismatch_detected.proto`, `reconciliation_case_resolved.proto`, `reconciliation_case_escalated.proto` e `effect_release_requested.proto` em `apps/backend/settlement/contract/proto/company/settlement/event/v1/`, package `company.settlement.event.v1`. Consumidores declarados em 92 §4: Portfolio (`SettlementReported`, `AssetSettled`), Effects (`AssetSettled`, `EffectReleaseRequested`), Ops e Risco (`ReconciliationMismatchDetected`) e Ledger. `AssetPartiallySettled` e `ReconciliationCaseResolved` constam do PRD e não de 92 §4 [ABERTA: 92:§4]. O PRD do Portfolio também lista `AssetSettled` entre os seus eventos; esta spec segue 92 §4, que o atribui ao Settlement [ABERTA: 05 PRD × 92:§4].
- **Consome** (disposições de consumo por FND-04 §6.4; os schemas de Effects, Funding e Portfolio não cobrem todos os campos abaixo [ABERTA: 92:§3]):
  - `company.effects.event.v1.EffectConfirmed` → `CreateSettlementExpectation`, um por alvo confirmado. Mapa: `targetId ← targets[].targetId`, `effectId ← effectId`, `contractId ← contractId`, `assetType ← targets[].assetType`, `assetId ← targets[].assetId`, `registryPartnerId ← registryPartnerId`, `beneficiaryOrganizationId ← beneficiaryOrganizationId`, `expectedAmountMinor ← targets[].confirmedAmountMinor`, `effectReference ← externalReference`; `assetReference` e `expectedDate` ← consulta gRPC ao Portfolio (o evento não traz a data de liquidação nem o identificador oficial do ativo). O volume é de um comando por alvo, até milhares por efeito [ABERTA: volume, #12].
  - `company.funding.event.v1.DisbursementConfirmed` → `TrackEffectRelease`. Mapa: `effectId ← effectId`, `contractId ← contractId`, `confirmedAt ← confirmedAt`.
  - `company.funding.event.v1.DisbursementReversed` → `RecordFundingReversal`. Mapa: `effectId ← effectId`, `phase ← phase`, `reversedAt ← at`.
  - `company.portfolio.event.v1.AssetPositionChanged` → `RecordSettlementOccurrence` com `sourceSystem = position`, quando a mudança indica liquidação, baixa ou redução de um ativo com expectativa aberta [ABERTA: sem schema em 92; a regra de mapeamento depende da janela da consulta, 05 §6].
  - `company.portfolio.event.v1.ExternalPositionDiverged` → `OpenReconciliationCase` com `origin = external_position_divergence`. Mapa: `severity ← severity`, `effectId`, `contractId` e `targetId` do evento.
- **Consultas de saída**: pelo Gateway, `SettlementQueryPort` (`querySettlements`, `querySettlementByAsset`) para a ingestão; consulta gRPC a Partner Management (fontes e janelas), a Portfolio (data de liquidação) e a Effects (disputa aberta).

## Borda e persistência

- **Borda**: só gRPC, `company.settlement.service.v1.SettlementService`, com um RPC por comando (`CreateSettlementExpectation`, `ApplyOccurrence`, `AdvanceExpectationSchedule`, `ConfirmReconciliation`, `FlagExpectationMismatch`, `ClearExpectationMismatch`, `RecordSettlementOccurrence`, `OpenReconciliationRun`, `CloseReconciliationRun`, `RecordReconciliationItem`, `OpenReconciliationCase`, `ProposeAdjustment`, `ApproveAdjustment`, `EscalateReconciliationCase`, `ResolveReconciliationCase`, `TrackEffectRelease`, `RecordFundingReversal`, `RequestEffectRelease`) e por consulta (as doze de Consultas); os disparados só por consumo ou por processo não são chamados pelo bff. O REST é do `bff`: os webhooks de entrada de liquidação (91 §5.1) e a visão do cedente, que mostra a fonte e o instante da confirmação.
- **Banco**: database e role `settlement`, schema `public`, tabelas `settlement_expectations`, `settlement_occurrences`, `reconciliation_runs`, `reconciliation_items`, `reconciliation_cases` e `effect_releases`, com `tenant_id` à frente da chave. Persistência híbrida: colunas tipadas só para o que uma consulta filtra - `effect_id`, `registry_partner_id`, `expected_date` e `status` em `settlement_expectations`; `provider_id` e `settlement_date` em `settlement_occurrences`; `run_id`, `target_id` e `outcome` em `reconciliation_items`; `status` e `severity` em `reconciliation_cases`; o resto vai em `snapshot` `jsonb`. A `occurrences` da expectativa guarda só referências e valores. O bruto das fontes fica por referência [ABERTA: provider de object storage].
- **Instante**: inteiro de nanossegundos no domínio.
- Diverge do design: 90 §1.3 fala em milissegundos e 92 §1 em envelope JSON com `schemaUri`; vale o DMPF (nanossegundos; Protobuf com CloudEvents montado pelo relay).
- Diverge do design: multi-tenancy por `tenant_id` à frente da chave e escopo injetado pelo `ExecutionContext` (ADR-051 do DMPF), não por RLS (00 §4.1). A fila do backoffice vê casos de todos os tenants (17 §6.1) [ABERTA: requer extensão do kernel - ADR-051 DMPF].
- Diverge do design: broker Kafka (default do DMPF; confirmação na decisão D4 da fundação), não SQS.

## Políticas transversais

- **Idempotência**: todo comando de entrada corre em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); os disparados por evento têm a deduplicação da inbox (ADR-036). A ingestão é idempotente por `occurrenceKey` (ocorrência já ingerida não duplica, 11 §5); o reprocessamento gera nova execução versionada e nunca apaga a anterior.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Ajuste e pedido de liberação pelo backoffice seguem a alçada de 17 §5 e de 11 §6.1, com segregação entre quem propõe e quem aprova (94 C7.3, C7.6) [ABERTA: dupla aprovação no kernel].
- **Auditoria**: pela trilha de auditoria do kernel (FND-08) e pelo Ledger por outbox (12 §2); toda divergência mantém dado original, decisão e responsável.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `settlement` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO um alvo confirmado de um efeito
QUANDO CreateSettlementExpectation chega com expectedAmountMinor positivo
ENTÃO a expectativa nasce expected com acumuladores zerados e SettlementExpected vai à outbox
DADO expectedAmountMinor zero
QUANDO CreateSettlementExpectation chega
ENTÃO rejeita com settlement-reconciliation/settlement-expectation/invalid-expected-amount

DADO uma expectativa expected de 1000
QUANDO ApplyOccurrence chega com liquidated de 400
ENTÃO a expectativa vai a partially_settled com netSettled 400 e AssetPartiallySettled é emitido
DADO uma expectativa partially_settled com net 400
QUANDO ApplyOccurrence chega com liquidated de 600
ENTÃO a expectativa vai a settled com net 1000 e AssetSettled é emitido
DADO uma expectativa expected
QUANDO ApplyOccurrence chega com kind paid
ENTÃO netSettled não muda e a expectativa não vai a settled
DADO uma expectativa settled
QUANDO ApplyOccurrence chega com liquidated e reversal verdadeiro
ENTÃO a ocorrência é acrescentada, a original permanece e a expectativa vai a reversed
DADO uma occurrenceKey já aplicada
QUANDO ApplyOccurrence chega de novo
ENTÃO aceita sem alterar e sem emitir
DADO qualquer expectativa
QUANDO ApplyOccurrence chega com amountMinor zero
ENTÃO rejeita com settlement-reconciliation/settlement-expectation/invalid-amount

DADO uma expectativa expected com expectedDate vencida além da tolerância e net zero
QUANDO AdvanceExpectationSchedule chega
ENTÃO a expectativa vai a overdue
DADO uma expectativa settled com todas as ocorrências conciliadas
QUANDO ConfirmReconciliation chega
ENTÃO a expectativa vai a reconciled
DADO uma expectativa partially_settled
QUANDO ConfirmReconciliation chega
ENTÃO rejeita com settlement-reconciliation/settlement-expectation/not-settled
DADO uma expectativa settled com ocorrência sem item conciliado
QUANDO ConfirmReconciliation chega com allOccurrencesMatched falso
ENTÃO rejeita com settlement-reconciliation/settlement-expectation/unmatched-occurrences
DADO uma expectativa settled
QUANDO FlagExpectationMismatch chega
ENTÃO a expectativa vai a mismatch guardando o status anterior
DADO uma expectativa mismatch
QUANDO ClearExpectationMismatch chega
ENTÃO a expectativa volta ao status anterior
DADO uma expectativa settled
QUANDO ClearExpectationMismatch chega
ENTÃO rejeita com settlement-reconciliation/settlement-expectation/not-in-mismatch

DADO uma linha de liquidação nova com payload bruto
QUANDO RecordSettlementOccurrence chega com rawPayloadHash
ENTÃO a ocorrência é persistida sem correspondência e SettlementReported vai à outbox
DADO uma occurrenceKey já ingerida
QUANDO RecordSettlementOccurrence chega de novo
ENTÃO aceita sem alterar e sem emitir
DADO qualquer ocorrência
QUANDO RecordSettlementOccurrence chega sem rawPayloadHash
ENTÃO rejeita com settlement-reconciliation/settlement-occurrence/raw-payload-missing
DADO qualquer ocorrência
QUANDO RecordSettlementOccurrence chega com amountMinor zero
ENTÃO rejeita com settlement-reconciliation/settlement-occurrence/invalid-amount

DADO uma janela válida e um parceiro
QUANDO OpenReconciliationRun chega
ENTÃO a execução nasce open com version 1
DADO windowFrom posterior a windowTo
QUANDO OpenReconciliationRun chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-run/invalid-window
DADO um reprocessamento com version igual à da execução anterior
QUANDO OpenReconciliationRun chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-run/invalid-reprocess-version e a execução anterior fica intacta
DADO uma execução open
QUANDO CloseReconciliationRun chega com o sumário
ENTÃO a execução vai a closed
DADO uma execução closed
QUANDO CloseReconciliationRun chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-run/run-not-open

DADO uma ocorrência que casa pelo identificador oficial do ativo
QUANDO RecordReconciliationItem chega com matched e camada 1
ENTÃO o item registra a camada usada e ReconciliationMatched é emitido
DADO uma divergência de valor com caso aberto
QUANDO RecordReconciliationItem chega com mismatch AMOUNT_DIVERGENCE
ENTÃO o item referencia o caso e ReconciliationMismatchDetected é emitido
DADO uma correspondência por composição sem combinação única
QUANDO RecordReconciliationItem chega com camada 3
ENTÃO rejeita com settlement-reconciliation/reconciliation-item/invalid-layer
DADO uma divergência sem caseId
QUANDO RecordReconciliationItem chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-item/mismatch-without-case
DADO uma divergência de beneficiário com severidade medium
QUANDO RecordReconciliationItem chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-item/beneficiary-divergence-must-be-critical

DADO uma divergência detectada
QUANDO OpenReconciliationCase chega
ENTÃO o caso nasce open
DADO uma divergência de beneficiário com severidade low
QUANDO OpenReconciliationCase chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/beneficiary-divergence-must-be-critical
DADO um caso open
QUANDO ProposeAdjustment chega com justificativa e faixa supervisor
ENTÃO o ajuste nasce pending exigindo operator e supervisor, o caso vai a pending_approval e o dado bruto não muda
DADO um caso pending_approval
QUANDO ProposeAdjustment chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/case-not-open
DADO um caso open
QUANDO ProposeAdjustment chega com suspectedFraud verdadeiro
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/fraud-suspected-escalate
DADO um caso open
QUANDO ProposeAdjustment chega sem justificativa
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/justification-missing
DADO um ajuste pending exigindo operator e supervisor
QUANDO ApproveAdjustment chega do supervisor e depois do operator, ambos diferentes de quem propôs
ENTÃO o ajuste vai a approved e o caso volta a open
DADO um ajuste já approved
QUANDO ApproveAdjustment chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/adjustment-not-pending
DADO um ajuste pending
QUANDO ApproveAdjustment chega do próprio proponente
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/self-approval
DADO um ajuste pending que não exige o papel financial_owner
QUANDO ApproveAdjustment chega com esse papel
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/approver-role-not-required
DADO um caso open
QUANDO EscalateReconciliationCase chega
ENTÃO o caso vai a escalated e ReconciliationCaseEscalated é emitido
DADO um caso resolved
QUANDO EscalateReconciliationCase chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/case-closed
DADO um caso open
QUANDO ResolveReconciliationCase chega com código e justificativa
ENTÃO o caso vai a resolved e ReconciliationCaseResolved é emitido
DADO um caso pending_approval
QUANDO ResolveReconciliationCase chega
ENTÃO rejeita com settlement-reconciliation/reconciliation-case/adjustment-pending

DADO um desembolso confirmado
QUANDO TrackEffectRelease chega
ENTÃO o acompanhamento nasce tracking com disbursementConfirmedAt
DADO um efeito sem acompanhamento
QUANDO RecordFundingReversal chega
ENTÃO rejeita com settlement-reconciliation/effect-release/not-tracked
DADO um acompanhamento tracking e todas as condições atendidas
QUANDO RequestEffectRelease chega
ENTÃO o acompanhamento vai a requested e EffectReleaseRequested vai à outbox
DADO um acompanhamento requested
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/already-requested
DADO um desembolso estornado
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/disbursement-reversed
DADO obligationFulfilled falso
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/obligation-not-fulfilled
DADO ativos baixados mas repasse não confirmado
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/transfer-not-confirmed
DADO contestationWindowEndsAt zero ou ainda no futuro
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/contestation-window-open
DADO chargebackSeen verdadeiro
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/chargeback-seen
DADO effectDisputeOpen verdadeiro
QUANDO RequestEffectRelease chega
ENTÃO rejeita com settlement-reconciliation/effect-release/effect-dispute-open

DADO duas expectativas do parceiro A na janela e uma do parceiro B
QUANDO FindSettlementExpectationsForWindow consulta o parceiro A
ENTÃO devolve exatamente as duas, fora de qualquer UoW
```

## Escopo fora

- **Movimentação de recursos**: a plataforma não redistribui, não retém nem repassa valores (regra 6 do PRD de produto e invariante do PRD 11); só registra o que a fonte informa.
- **Cálculo de rateio**: a alocação é a informada pela fonte competente; com efeitos concorrentes e fonte que não detalha, não se infere e abre-se caso (11 §4).
- **REST do bff**: webhooks de entrada de liquidação (91 §5.1) e visão do cedente ficam no `bff`.
- **Adaptadores das fontes**: protocolos, formatos de arquivo e tradução são do Receivables Gateway (04); aqui só a ingestão canônica.
- **Execução da liberação**: o comando ao parceiro e o estado do efeito são do Effects (09); aqui só a decisão de pedir (`EffectReleaseRequested`).
- **Caso operacional, fila e SLA**: do Ops (17); o `ReconciliationCase` guarda só o tratamento de domínio.
- **Cobrança**: o Billing não existe antes de #7 (D1 a D3) e usa `DisbursementConfirmed`, não a liquidação (18 §2.3).
- **`DISPUTED` e `WRITE_OFF_REPORTED` completos**: sem gatilho nem tratamento próprio no design de 11; o estado existe no modelo [ABERTA: 11 PRD].
- **Camada 1 e 2 de correspondência em produção**: só após #3 (B4); até lá valem as camadas 3 a 5.
