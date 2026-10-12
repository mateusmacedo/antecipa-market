---
id: SPEC-GM5GH5BF
slug: eligibility
title: Bounded context — eligibility
stage: backlog
priority: P1
depends_on: [SPEC-GXGASP24, SPEC-6E7BKFNS, SPEC-R25V2A8C]
ticket_url: https://linear.app/mmda/issue/DEVS-87/spec-gm5gh5bf-bounded-context-eligibility
subtask_urls: []
created: 2026-10-12
---

# SPEC-GM5GH5BF: Bounded context — eligibility

## Identidade

- **bounded_context**: `eligibility`
- **name**: `eligibility`
- **Convenções de tipo**: dinheiro é `…Minor: int` (unidade menor) mais `currency: string`;
  `instant` é inteiro de nanossegundos. Decisões numeradas do resumo executivo aparecem como
  `resumo:Dn`; as da fundação, como `00:Dn`.
- **Linguagem ubíqua**:
  - *Política* (`EligibilityPolicy`) - conjunto versionado de parâmetros das regras. As regras
    são código versionado (06 §2); só os parâmetros ficam em dados.
  - *Avaliação* (`EligibilityEvaluation`) - execução da política sobre um snapshot do Portfolio.
  - *Resultado de regra* (`RuleResult`) - aprovação, rejeição ou pendência, com razão legível.
  - *Severidade* - `blocking` exclui; `warning` inclui com alerta; `info` só registra.
  - *Lote elegível* (`EligibleLot`) - ativos selecionáveis para um produto e financiador.
  - *Reserva* (`Reservation`) - bloqueio interno e temporário de um ativo, com expiração. Não
    produz efeito perante terceiros: só o registro na infraestrutura impede dupla negociação.
  - *Simulação* - avaliação sem reserva, sem evento e sem poder de fundamentar solicitação.
  - *Invalidação* - perda de validade de uma avaliação por mudança de snapshot, consentimento,
    dossiê, capacidade ou risco. Mudança de política não invalida.
  - *Remediável* - exclusão que o usuário pode corrigir (consentimento expirado), ao contrário
    da que não pode (ativo vencido).
  - Elegível não significa crédito aprovado: o contexto não decide crédito nem preço.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/06-prd-eligibility.md`; `docs/design/06-design-eligibility.md` §2-§9;
  `docs/design/90-modelo-de-dados.md` §4; `docs/design/92-schemas-eventos.md` §4;
  `docs/design/91-contratos-api.md` §3.1-§3.2; `docs/design/93-requisitos-nao-funcionais.md`
  §3.1 (5.000 ativos em menos de 2 s, p95); `docs/design/05-design-portfolio.md` §3-§4.
- **Situação**: parcial. O motor de regras, a estrutura da política, a justificativa legível e
  a reserva são construíveis. Parâmetros comerciais e três regras dependem de decisão e apuração.
- **Fase de entrega**: 1.
- **Dependências**: duro: portfolio (SPEC-6E7BKFNS), kyb (SPEC-R25V2A8C), consent, partner e
  identity (consulta `ORG-REP-001` e `ORG-STS-001`). `depends_on` declara só a fundação e a
  aresta dura reduzida (portfolio e kyb); consent, partner e identity ficam implícitos pela
  cadeia transitiva. Brando: risk (`OperationBlockedByRisk` → `ORG-RSK-001`), marketplace
  (`FundingRequestClosed`) e contracting (`ContractCancelled`), que liberam reserva. Marketplace e
  contracting são jusante: retroalimentação, fora de `depends_on`.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
|---|---|---|
| resumo:D4 (06:E8) | Parâmetros comerciais: faixas de valor e prazo, concentração, listas | `EligibilityPolicy.parameters`; regras `AST-VAL`, `AST-MAT-002`, `AST-ARR`, `AST-ACQ`, `LOT-*` |
| B3 (06:E9) | Tipos de efeito incompatíveis | regra `AST-ENC-001` |
| B5, B2 (06:E10) | Validade da posição e cobertura de gravame por fonte | regras `AST-FRS-001`, `AST-ENC-002` |
| 06:E11 | Extensão máxima da reserva | `Reservation.maxExpiresAt` |
| 06:E12 | Invalidação depois da assinatura | tratada em 09; fora desta spec |
| #1; #12 | Autorização do BCB (transversal); volumetria | existencial; dimensionamento |

- **Decisões de produto abertas**: parâmetros comerciais do primeiro produto (resumo:D4;
  06:E8) e extensão máxima da reserva (06:E11).
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída; SPEC-6E7BKFNS e
  SPEC-R25V2A8C em `planning` ou adiante; parâmetros mínimos do primeiro financiador definidos
  (resumo:D4). `AST-FRS-001` e `AST-ENC-002` podem nascer com defaults conservadores até B5 e B2.

## Agregados

### EligibilityPolicy

- **Identidade**: `natural` por `policyKey` (`<policyId>:<version>`)
- **Campos**:
  - `policyId: string`, `version: int`, `status: published | retired`
  - `effectiveFrom: instant`, `effectiveUntil: instant` (opcional)
  - `assetTypes: list<string>` (nota: escopo da política, 06 §3)
  - `partnerId: id`, `productId: id` (ambos opcionais)
  - `ruleSetVersion: string` (versão do artefato de código que contém as regras)
  - `parameters: string` (JSON por `ruleId`: faixas, percentuais e listas)
- **Estados**: `published`, `retired`
- **Transições permitidas**:
  - (novo) → `published` - por `PublishEligibilityPolicy`
  - `published` → `retired` - por `RetireEligibilityPolicy`
- **Invariantes de estado**:
  - Versão publicada nunca muda: regra alterada não modifica avaliação histórica; mudança é
    nova versão.
  - `assetTypes` nunca é vazio - `eligibility/eligibility-policy/scope-empty`.
  - `effectiveUntil > effectiveFrom` - `eligibility/eligibility-policy/effective-window-invalid`.
  - O catálogo normativo de regras (`ORG-*`, `AST-*`, `LOT-*`) é o de 06 §4 e é referenciado por
    `ruleId`, não copiado.

### EligibilityEvaluation

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`, `snapshotId: id`, `snapshotAsOf: instant`
  - `policyKey: string`, `purpose: simulation | contracting`
  - `productId: id`, `partnerId: id` (opcionais)
  - `status: requested | evaluated | invalidated`
  - `requestedAt: instant`, `evaluatedAt: instant` (opcional)
  - `assetCount: int`, `eligibleCount: int`, `ineligibleCount: int`
  - `results: list<RuleResult>` (nota: agregado por regra e `reasonCode`, não por ativo -
    06 §9; cada entrada com `ruleId: string`, `ruleVersion: int`,
    `severity: blocking | warning | info`, `outcome: pass | fail | pending`,
    `reasonCode: string`, `explanation: string`, `observed: string`, `expected: string`,
    `remediable: bool`, `remediationHint: string` (opcional), `assetCount: int`)
  - `invalidationReason: string`, `invalidatedAt: instant` (opcionais)
- **Estados**: `requested`, `evaluated`, `invalidated`
- **Transições permitidas**:
  - (novo) → `requested` - por `RequestEligibilityEvaluation`
  - `requested` → `evaluated` - por `CompleteEligibilityEvaluation`
  - `requested` → `invalidated` e `evaluated` → `invalidated` - por `InvalidateEvaluation`
- **Invariantes de estado**:
  - Toda avaliação referencia a versão da política e o snapshot (invariante do PRD).
  - Qualquer resultado `blocking` com `fail` exclui o ativo ou o lote; `warning` não exclui,
    mas consta da avaliação e do lote. A ordem é `ORG`, depois `ASSET`, depois `LOT` (06 §3.2).
  - `explanation` traz o valor observado e o esperado em linguagem do usuário (06 §5).
  - Avaliação de simulação não gera evento, tem retenção curta e não vale para criar
    solicitação. **[ABERTA: 93:RNF-V5 - prazo da retenção curta]**
  - Mudança de política não invalida avaliação existente -
    `eligibility/eligibility-evaluation/policy-change-not-invalidating`.

### EligibleLot

- **Identidade**: `generated`
- **Campos**:
  - `evaluationId: id`, `organizationId: id`, `snapshotId: id`, `productId: id`, `partnerId: id`
  - `assetIds: list<id>` (nota: a seleção do lote, até 5.000 ativos - 93 §2.1; indispensável
    porque a reserva é por ativo)
  - `assetCount: int`, `netTotalMinor: int`, `currency: string`
  - `warnings: list<string>` (nota: `ruleId` dos `warning` que falharam, repassados ao financiador)
  - `status: eligible | invalidated | closed`, `createdAt: instant`
- **Estados**: `eligible`, `invalidated`, `closed`
- **Transições permitidas**:
  - (novo) → `eligible` - por `CreateEligibleLot`
  - `eligible` → `invalidated` e `eligible` → `closed` - por `CloseEligibleLot`
- **Invariantes de estado**:
  - O lote só nasce de avaliação `evaluated` de contratação, sem resultado `blocking` falho.
  - O conteúdo do lote não muda; só o `status`.
  - **[ABERTA: 06-prd (lote por produto e financiador) x 90:§4 (EligibleLot 1:1
    FundingRequest, 1:N PartnerInvitation)]**

### Reservation

- **Identidade**: `natural` por `assetId` (a `assetKey` do Portfolio)
- **Campos**:
  - `assetId: string`, `lotId: id`, `holderId: id` (a `FundingRequest` que reserva)
  - `reservedAmountMinor: int`, `currency: string`
  - `status: active | released | expired`
  - `reservedAt: instant`, `expiresAt: instant`, `maxExpiresAt: instant`
  - `releaseReason: concluded | cancelled | invalidated` (opcional)
- **Estados**: `active`, `released`, `expired`
- **Transições permitidas**:
  - (novo) → `active`, `released` → `active` e `expired` → `active` - por `ReserveAsset`
  - `active` → `released` - por `ReleaseReservation`
  - `active` → `expired` - por `ExpireReservation`
- **Invariantes de estado**:
  - Há no máximo uma reserva vigente por ativo: `(assetId, ACTIVE)` de 06 §6.2 vira a identidade
    natural por `assetId`, e a chave primária do `postgres.Table` resolve a corrida; a segunda
    reserva rejeita com `eligibility/reservation/asset-already-reserved`. Diverge do design:
    06 §6.2 pede índice único parcial; o histórico de reservas anteriores vive nos eventos e no
    Ledger.
  - Reserva com `expiresAt <= agora` não bloqueia nova reserva, mesmo antes de
    `ExpireReservation` rodar (verificação preguiçosa, 06 §6.2).
  - `expiresAt <= maxExpiresAt`; além do limite exige nova avaliação.
  - A reserva é por ativo, nunca por lote: o ativo sai do lote sem liberar os demais.

## Comandos (UPRs)

### PublishEligibilityPolicy
- **Agregado**: `EligibilityPolicy`; **cria**
- **Entrada**: `policyId: string`, `version: int`, `effectiveFrom: instant`, `effectiveUntil:
  instant` (opcional), `assetTypes: list<string>`, `partnerId: id`, `productId: id`
  (opcionais), `ruleSetVersion: string`, `parameters: string`
- **Pré-condições**:
  - `assetTypes` não vazio → `eligibility/eligibility-policy/scope-empty`
  - `effectiveUntil > effectiveFrom` quando presente → `eligibility/eligibility-policy/effective-window-invalid`
  - `policyKey` ainda não publicada → `eligibility/eligibility-policy/already-published`
- **Efeitos sobre o estado**: campos ← entrada; `status ← published`
- **Eventos emitidos**: nenhum
- **Resposta**: `policyKey: string`

### RetireEligibilityPolicy
- **Agregado**: `EligibilityPolicy`; exige existente
- **Entrada**: `effectiveUntil: instant`
- **Pré-condições**:
  - `status = published` → `eligibility/eligibility-policy/already-retired`
- **Efeitos sobre o estado**: `status ← retired`, `effectiveUntil ← cmd.effectiveUntil`
- **Eventos emitidos**: nenhum
- **Resposta**: `policyKey: string`

### RequestEligibilityEvaluation
- **Agregado**: `EligibilityEvaluation`; **cria**
- **Entrada**: `organizationId: id`, `snapshotId: id`, `snapshotAsOf: instant`,
  `snapshotStaleAfter: instant`, `policyKey: string`, `policyEffective: bool`,
  `purpose: simulation | contracting`, `productId: id`, `partnerId: id` (opcionais). O caso de
  uso lê o snapshot no Portfolio e a política em `FindEffectivePolicy` antes da UoW.
- **Pré-condições**:
  - `snapshotStaleAfter > at` → `eligibility/eligibility-evaluation/snapshot-stale`
  - `policyEffective` → `eligibility/eligibility-evaluation/policy-not-in-effect`
- **Efeitos sobre o estado**: `status ← requested`, `requestedAt ← at`, demais campos ← entrada
- **Eventos emitidos**: `EligibilityEvaluationRequested` (só `contracting`)
- **Resposta**: `evaluationId: id`

### CompleteEligibilityEvaluation
- **Agregado**: `EligibilityEvaluation`; exige existente
- **Entrada**: `assetCount: int`, `eligibleCount: int`, `ineligibleCount: int`,
  `results: list<RuleResult>`. As regras rodam no caso de uso, sobre os ativos lidos do
  Portfolio, com pré-filtro em consulta e agregação por `reasonCode` (06 §9).
  **[ABERTA: fronteira domínio/aplicação para executar regras sobre 5.000 ativos de outro contexto]**
- **Pré-condições**:
  - `status = requested` → `eligibility/eligibility-evaluation/not-requested`
  - `results` não vazio → `eligibility/eligibility-evaluation/results-empty`
  - `eligibleCount + ineligibleCount = assetCount` → `eligibility/eligibility-evaluation/counts-inconsistent`
- **Efeitos sobre o estado**: `status ← evaluated`, `evaluatedAt ← at`, contagens e `results ← cmd`
- **Eventos emitidos**: nenhum
- **Resposta**: `evaluationId: id`, `eligibleCount: int`

### InvalidateEvaluation
- **Agregado**: `EligibilityEvaluation`; exige existente
- **Entrada**: `reason: snapshot_changed | stale_position | consent_revoked | consent_expired |
  reverification_required | partner_rejected | capability_suspended | product_expired |
  risk_block | policy_changed`, `affectedAssetIds: list<id>` (opcional, informativo)
- **Pré-condições**:
  - `reason` diferente de `policy_changed` → `eligibility/eligibility-evaluation/policy-change-not-invalidating`
- **Efeitos sobre o estado**: `status ← invalidated`, `invalidationReason ← cmd.reason`,
  `invalidatedAt ← at` (já `invalidated`: aceita sem alterar e sem emitir)
- **Eventos emitidos**: `EligibilityInvalidated` (só `contracting`, na transição)
- **Resposta**: `evaluationId: id`

### CreateEligibleLot
- **Agregado**: `EligibleLot`; **cria**
- **Entrada**: `evaluationId: id`, `organizationId: id`, `snapshotId: id`, `productId: id`,
  `partnerId: id`, `assetIds: list<id>`, `netTotalMinor: int`, `currency: string`,
  `warnings: list<string>`, `evaluationPurpose`, `evaluationValid: bool`, `blockingFailures: int`
  (os três últimos lidos pelo caso de uso na `EligibilityEvaluation`)
- **Pré-condições**:
  - `evaluationPurpose = contracting` → `eligibility/eligible-lot/simulation-cannot-create-lot`
  - `evaluationValid` (avaliação `evaluated`) → `eligibility/eligible-lot/evaluation-not-valid`
  - `assetIds` não vazio → `eligibility/eligible-lot/assets-empty`
  - `blockingFailures = 0` → `eligibility/eligible-lot/blocking-rule-failed`
- **Efeitos sobre o estado**: `status ← eligible`, `createdAt ← at`, `assetCount ← tamanho de
  assetIds`, demais campos ← entrada
- **Eventos emitidos**: `EligibleLotCreated`
- **Resposta**: `lotId: id`

### CloseEligibleLot
- **Agregado**: `EligibleLot`; exige existente
- **Entrada**: `reason: closed | invalidated`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: `status ← cmd.reason` quando `eligible`; já terminal: aceita sem alterar
- **Eventos emitidos**: nenhum
- **Resposta**: `lotId: id`

### ReserveAsset
- **Agregado**: `Reservation`; **inicializa se ausente**
- **Entrada**: `assetId: string`, `lotId: id`, `holderId: id`, `reservedAmountMinor: int`,
  `currency: string`, `expiresAt: instant`, `maxExpiresAt: instant` (derivado do parâmetro de
  política **[ABERTA: 06:E11]**)
- **Pré-condições**:
  - `expiresAt > at` → `eligibility/reservation/expiry-in-past`
  - `maxExpiresAt >= expiresAt` → `eligibility/reservation/max-expiry-before-expiry`
  - sem reserva `active` e não expirada de outro `holderId` → `eligibility/reservation/asset-already-reserved`
- **Efeitos sobre o estado**:
  - ausente, `released`, `expired` ou `active` vencida: `status ← active`, `reservedAt ← at`,
    demais campos ← entrada, `releaseReason` limpo
  - `active` e vigente do mesmo `holderId`: aceita sem alterar e sem emitir
- **Eventos emitidos**: `AssetReserved` (só quando reserva)
- **Resposta**: `assetId: string`, `expiresAt: instant`

### ExtendReservation
- **Agregado**: `Reservation`; exige existente
- **Entrada**: `holderId: id`, `newExpiresAt: instant`
- **Pré-condições**:
  - `status = active` e `expiresAt > at` → `eligibility/reservation/reservation-not-active`
  - `holderId` é o do agregado → `eligibility/reservation/holder-mismatch`
  - `newExpiresAt > expiresAt` → `eligibility/reservation/extension-not-later`
  - `newExpiresAt <= maxExpiresAt` → `eligibility/reservation/extension-limit-exceeded`
- **Efeitos sobre o estado**: `expiresAt ← cmd.newExpiresAt`
- **Eventos emitidos**: nenhum
- **Resposta**: `assetId: string`, `expiresAt: instant`

### ReleaseReservation
- **Agregado**: `Reservation`; exige existente
- **Entrada**: `holderId: id`, `reason: concluded | cancelled | invalidated`
- **Pré-condições**:
  - `holderId` é o do agregado → `eligibility/reservation/holder-mismatch`
- **Efeitos sobre o estado**: `status ← released`, `releaseReason ← cmd.reason` quando `active`;
  já `released` ou `expired`: aceita sem alterar
- **Eventos emitidos**: nenhum. **[ABERTA: 92-schemas-eventos:§4 só tem ReservationExpired;
  a liberação por conclusão, cancelamento ou invalidação não chega ao Portfolio]**
- **Resposta**: `assetId: string`

### ExpireReservation
- **Agregado**: `Reservation`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status = active` e `expiresAt <= at` → `eligibility/reservation/not-expirable`
- **Efeitos sobre o estado**: `status ← expired`
- **Eventos emitidos**: `ReservationExpired`
- **Resposta**: `assetId: string`

## Eventos de domínio

92 §3 não traz schema dos eventos deste contexto: os campos são derivados dos agregados.
**[ABERTA: 92-schemas-eventos:§3 sem schema de EligibleLotCreated, AssetReserved,
ReservationExpired e EligibilityInvalidated]**

### EligibleLotCreated
- **Campos**:
  - `lotId: id ← EligibleLot.id`, `evaluationId: id`, `organizationId: id`, `snapshotId: id`
  - `productId: id`, `partnerId: id`, `assetCount: int`, `netTotalMinor: int`, `currency: string`
  - `warnings: list<string> ← EligibleLot.warnings`, `policyKey: string ← EligibilityEvaluation`
  - `createdAt: instant ← at`
- Não leva a lista de ativos (até 5.000, 93 §2.1): o consumidor a obtém por `FindLot`.
  Exceção à convenção 3 de 92 §2, como a de `PortfolioSnapshotCreated`.

### AssetReserved
- **Campos**: `assetId: string ← Reservation.assetId`, `lotId: id`, `holderId: id`,
  `reservedAmountMinor: int`, `currency: string`, `reservedAt: instant ← at`,
  `expiresAt: instant ← Reservation.expiresAt`

### ReservationExpired
- **Campos**: `assetId: string`, `lotId: id`, `holderId: id`, `reservedAmountMinor: int`,
  `expiredAt: instant ← at`

### EligibilityInvalidated
- **Campos**: `evaluationId: id`, `organizationId: id`, `snapshotId: id`,
  `reason ← cmd.reason`, `affectedAssetIds: list<id> ← cmd.affectedAssetIds`,
  `invalidatedAt: instant ← at`

### EligibilityEvaluationRequested
- **Campos**: `evaluationId: id`, `organizationId: id`, `snapshotId: id`, `policyKey: string`,
  `purpose`, `at: instant ← at`. Só evento de domínio: 92 §4 não o lista.
  **[ABERTA: 92-schemas-eventos:§4 sem EligibilityEvaluationRequested, AssetEligible e
  AssetIneligible, que o PRD 06 lista; os dois últimos contrariam o registro agregado de 06 §9]**

## Consultas

Consultas correm fora da UoW.

- `FindPolicy` - filtros: `policyKey`; cardinalidade: `one`
- `FindEffectivePolicy` - filtros: `assetType`, `partnerId`, `productId`, `at`; cardinalidade: `one`
- `FindEvaluation` - filtros: `evaluationId`; cardinalidade: `one`
- `FindEvaluationsBySnapshot` - filtros: `snapshotId`, `status`; cardinalidade: `many`
- `FindEvaluationsByOrganization` - filtros: `organizationId`, `status`, `partnerId`; cardinalidade: `many`
- `FindLot` - filtros: `lotId`; cardinalidade: `one` (devolve também `assetIds`)
- `FindLotsByEvaluation` - filtros: `evaluationId`; cardinalidade: `many`
- `FindLotsByAsset` - filtros: `assetId`, `status`; cardinalidade: `many` (serve à invalidação
  por ativo). **[ABERTA: `assetIds` fica no `snapshot` jsonb; a consulta exige coluna auxiliar]**
- `FindReservationByAsset` - filtros: `assetId`; cardinalidade: `one` (devolve `expired` quando
  `expiresAt <= agora`, mesmo que `ExpireReservation` ainda não tenha rodado)
- `FindReservationsByHolder` - filtros: `holderId`; cardinalidade: `many`
- `ExplainAssetExclusion` - filtros: `evaluationId`, `assetId`; cardinalidade: `one` (recalcula
  o detalhe por ativo com a mesma versão de política, lendo o ativo no Portfolio - 06 §9)

## Relações entre agregados

- `EligibilityEvaluation.policyKey` → `EligibilityPolicy`
- `EligibleLot.evaluationId` → `EligibilityEvaluation`
- `Reservation.lotId` → `EligibleLot`
- Para outros contextos, só por identidade: `EligibilityEvaluation.snapshotId` →
  `PortfolioSnapshot` (portfolio); `EligibilityEvaluation.organizationId` → `Organization`
  (identity); `EligibleLot.partnerId` → `Partner` (partner); `EligibleLot.assetIds[]` e
  `Reservation.assetId` → ativo (portfolio, por `assetKey`); `Reservation.holderId` →
  `FundingRequest` (marketplace).

## Integração

- **Publica** (integration events), em
  `apps/backend/eligibility/contract/proto/company/eligibility/event/v1/`, package
  `company.eligibility.event.v1`:
  - `EligibleLotCreated` - `eligible_lot_created.proto` (Marketplace, Ledger)
  - `AssetReserved` - `asset_reserved.proto` (Portfolio, Ledger)
  - `ReservationExpired` - `reservation_expired.proto` (Marketplace, Portfolio)
  - `EligibilityInvalidated` - `eligibility_invalidated.proto` (Marketplace, Contracting, Ledger)
- **Consome** (por contrato; disposições de consumo conforme FND-04 §6.4). Cada evento de
  invalidação alcança N avaliações ou reservas: o consumidor consulta e dispara um comando por
  agregado. **[ABERTA: fan-out de consumo sem suporte verificado no kernel; uma UPR altera um
  agregado]**
  - `company.portfolio.event.v1.PortfolioSnapshotCreated` → `InvalidateEvaluation`
    (`snapshot_changed`) para cada avaliação com `snapshotId = supersedesSnapshotId`.
  - `company.portfolio.event.v1.AssetPositionChanged` → `InvalidateEvaluation`
    (`snapshot_changed`) das avaliações cujo lote contém `assetKey` (`FindLotsByAsset`);
    `affectedAssetIds ← [assetKey]`.
  - `company.portfolio.event.v1.AssetBecameStale` → `InvalidateEvaluation` (`stale_position`),
    mesma busca.
  - `company.kyb.event.v1.ReverificationRequired` → `InvalidateEvaluation`
    (`reverification_required`) de todas as avaliações de `organizationId`.
  - `company.kyb.event.v1.PartnerRejected` → `InvalidateEvaluation` (`partner_rejected`) das
    avaliações da organização para o `partnerId`. **[ABERTA: 06 §7 não lista este gatilho]**
  - `company.consent.event.v1.ConsentRevoked` → `InvalidateEvaluation` (`consent_revoked`) de
    todas as avaliações de `organizationId` (92 §3.1); `ConsentExpired` → `consent_expired`.
  - `company.partner.event.v1.CapabilitySuspended` → `InvalidateEvaluation`
    (`capability_suspended`) e `CloseEligibleLot` (`invalidated`) para avaliações e lotes do
    `partnerId`; `PartnerProductChanged` → `product_expired`.
  - `company.risk.event.v1.OperationBlockedByRisk` → `InvalidateEvaluation` (`risk_block`) de
    todas as avaliações de `organizationId` (`ORG-RSK-001`).
  - `company.marketplace.event.v1.FundingRequestClosed` → `CloseEligibleLot` (`closed`) e
    `ReleaseReservation` para cada reserva de `FindReservationsByHolder(fundingRequestId)`;
    `reason ←` `concluded` (contratado), `cancelled` ou `invalidated` conforme o motivo.
  - `company.contracting.event.v1.ContractCancelled` → `ReleaseReservation` (`cancelled`).
    **[ABERTA: o evento traz `contractId`; a ligação com `holderId` não está em 92:§3]**
  - Payloads de `ConsentExpired`, `PartnerProductChanged`, `PartnerRejected`,
    `FundingRequestClosed` e `ContractCancelled` **[ABERTA: 92:§3 sem schema]**.
- **Não subscritos**: `ConsentGranted`, `VerificationReady`, `PartnerApproved` e
  `CapabilityEnabled`. 92 §4 lista Eligibility como consumidor, mas 06 §7 não define reação:
  as regras `ORG-*` leem o estado atual por consulta gRPC. **[ABERTA: 92-schemas-eventos:§4]**
  Também não há subscrição de `RepresentationRevoked`, que 06 §7 lista como gatilho.
- **Consultas de saída (gRPC)**: Portfolio (`FindSnapshot`, `ListAssetsBySnapshot`,
  `FindAvailabilityBySnapshot`), Identity (`ORG-REP-001`, `ORG-STS-001`), Consent (escopo
  efetivo, `ORG-CNS-*`), KYB (dossiê, `ORG-KYB-*`) e Partner (capacidade efetiva e produto). A
  fonte de dado de `ORG-BNK-001` não é declarada em 06. **[ABERTA: 06:§4.1 x 94 C2.2]**

## Borda e persistência

- **Borda**: só gRPC, `company.eligibility.service.v1.EligibilityService`, um RPC por comando e
  por consulta, atrás da cadeia de interceptors do kernel. O REST público é do `bff` (Escopo fora).
- **Banco**: database e role `eligibility`, schema `public`, tabelas `eligibility_policies`,
  `eligibility_evaluations`, `eligible_lots` e `reservations`, com `tenant_id` à frente da
  chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra:
  - `eligibility_policies`: `policy_id`, `effective_from` (`FindEffectivePolicy`)
  - `eligibility_evaluations`: `organization_id`, `snapshot_id`, `status`
  - `eligible_lots`: `evaluation_id`, `partner_id`, `status`
  - `reservations`: `holder_id`, `expires_at` (a chave é `asset_id`)
  - o resto do estado vai em `snapshot` `jsonb`
- **Instante**: inteiro de nanossegundos no domínio.
- **Expiração**: o gatilho periódico que chama `ExpireReservation` não tem agendador verificado
  no kernel. **[ABERTA: gatilho temporal sem suporte no kernel]** A leitura preguiçosa cobre o
  intervalo.

## Políticas transversais

- **Idempotência**: `RequestEligibilityEvaluation` entra pelo `bff` por `POST` com chave de
  idempotência declarada (ADR-056; FND-04 §7.6). `ReserveAsset` é idempotente por identidade
  natural do ativo e do `holderId`; os demais, por estado de destino; o consumo, por inbox com
  `eventId` (FND-04 §6.4).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação
  (FND-04 §3.2); o ato em nome do cedente exige representação válida (`ORG-REP-001`). O acesso do
  backoffice entre tenants **[ABERTA: requer extensão do kernel - ADR-051 DMPF]**.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); o Ledger
  recebe os quatro eventos publicados. A avaliação é reproduzível: mesma entrada e mesma versão
  de política dão o mesmo resultado.
- **Determinismo e desempenho**: avaliar 5.000 ativos em menos de 2 s (p95, 93 §3.1).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `eligibility` (`fmt-check`, `vet`, `build`, `lint`,
  `test-race` e `test-distributed` com Postgres, verificador com `--base` e
  `dmpf-context-check`).
- [ ] Cenários:

```text
DADO uma política nova com escopo e janela válidos
QUANDO PublishEligibilityPolicy chega
ENTÃO a política nasce published com ruleSetVersion e parameters e nada é emitido
DADO qualquer entrada
QUANDO PublishEligibilityPolicy chega com assetTypes vazio
ENTÃO rejeita com eligibility/eligibility-policy/scope-empty
DADO qualquer entrada
QUANDO PublishEligibilityPolicy chega com effectiveUntil menor ou igual a effectiveFrom
ENTÃO rejeita com eligibility/eligibility-policy/effective-window-invalid
DADO uma policyKey já publicada
QUANDO PublishEligibilityPolicy chega com a mesma chave
ENTÃO rejeita com eligibility/eligibility-policy/already-published e a política não muda
DADO uma política published
QUANDO RetireEligibilityPolicy chega com effectiveUntil
ENTÃO a política vai a retired com effectiveUntil gravado
DADO uma política retired
QUANDO RetireEligibilityPolicy chega
ENTÃO rejeita com eligibility/eligibility-policy/already-retired
DADO um snapshot com staleAfter no futuro e uma política em vigor
QUANDO RequestEligibilityEvaluation chega para contratação
ENTÃO a avaliação nasce requested com requestedAt = at e EligibilityEvaluationRequested é emitido
DADO uma avaliação de simulação
QUANDO RequestEligibilityEvaluation chega com purpose simulation
ENTÃO a avaliação nasce requested e nenhum evento é emitido
DADO um snapshot com staleAfter menor ou igual a at
QUANDO RequestEligibilityEvaluation chega
ENTÃO rejeita com eligibility/eligibility-evaluation/snapshot-stale
DADO policyEffective falso
QUANDO RequestEligibilityEvaluation chega
ENTÃO rejeita com eligibility/eligibility-evaluation/policy-not-in-effect
DADO uma avaliação requested
QUANDO CompleteEligibilityEvaluation chega com contagens coerentes e results
ENTÃO a avaliação vai a evaluated com evaluatedAt = at e nada é emitido
DADO uma avaliação já evaluated
QUANDO CompleteEligibilityEvaluation chega
ENTÃO rejeita com eligibility/eligibility-evaluation/not-requested
DADO uma avaliação requested
QUANDO CompleteEligibilityEvaluation chega com results vazio
ENTÃO rejeita com eligibility/eligibility-evaluation/results-empty
DADO uma avaliação requested
QUANDO CompleteEligibilityEvaluation chega com eligibleCount + ineligibleCount diferente de assetCount
ENTÃO rejeita com eligibility/eligibility-evaluation/counts-inconsistent
DADO uma avaliação evaluated de contratação
QUANDO InvalidateEvaluation chega com reason consent_revoked
ENTÃO a avaliação vai a invalidated com invalidationReason e EligibilityInvalidated é emitido
DADO uma avaliação já invalidated
QUANDO InvalidateEvaluation chega de novo
ENTÃO aceita sem alterar e sem emitir
DADO uma avaliação evaluated
QUANDO InvalidateEvaluation chega com reason policy_changed
ENTÃO rejeita com eligibility/eligibility-evaluation/policy-change-not-invalidating e o estado não muda
DADO uma avaliação de contratação evaluated, sem resultado blocking falho
QUANDO CreateEligibleLot chega com assetIds
ENTÃO o lote nasce eligible com assetCount = tamanho de assetIds e EligibleLotCreated vai à outbox
DADO uma avaliação de simulação
QUANDO CreateEligibleLot chega
ENTÃO rejeita com eligibility/eligible-lot/simulation-cannot-create-lot
DADO evaluationValid falso
QUANDO CreateEligibleLot chega
ENTÃO rejeita com eligibility/eligible-lot/evaluation-not-valid
DADO qualquer entrada
QUANDO CreateEligibleLot chega com assetIds vazio
ENTÃO rejeita com eligibility/eligible-lot/assets-empty
DADO blockingFailures maior que zero
QUANDO CreateEligibleLot chega
ENTÃO rejeita com eligibility/eligible-lot/blocking-rule-failed
DADO um lote eligible
QUANDO CloseEligibleLot chega com reason closed
ENTÃO o lote vai a closed; um segundo CloseEligibleLot aceita sem alterar
DADO um ativo sem reserva
QUANDO ReserveAsset chega com expiresAt no futuro e maxExpiresAt maior ou igual
ENTÃO a reserva nasce active com reservedAt = at e AssetReserved vai à outbox
DADO uma reserva active vigente de outro holderId
QUANDO ReserveAsset chega para o mesmo assetId
ENTÃO rejeita com eligibility/reservation/asset-already-reserved e a primeira reserva não muda
DADO uma reserva active com expiresAt menor ou igual a at
QUANDO ReserveAsset chega de outro holderId
ENTÃO a nova reserva é aceita: o ativo volta a active para o novo holder
DADO uma reserva active vigente do mesmo holderId
QUANDO ReserveAsset chega de novo
ENTÃO aceita sem alterar e sem emitir
DADO qualquer entrada
QUANDO ReserveAsset chega com expiresAt menor ou igual a at
ENTÃO rejeita com eligibility/reservation/expiry-in-past
DADO qualquer entrada
QUANDO ReserveAsset chega com maxExpiresAt menor que expiresAt
ENTÃO rejeita com eligibility/reservation/max-expiry-before-expiry
DADO uma reserva active vigente
QUANDO ExtendReservation chega com newExpiresAt entre expiresAt e maxExpiresAt
ENTÃO expiresAt avança e nada é emitido
DADO uma reserva released ou vencida
QUANDO ExtendReservation chega
ENTÃO rejeita com eligibility/reservation/reservation-not-active
DADO uma reserva active
QUANDO ExtendReservation ou ReleaseReservation chega com outro holderId
ENTÃO rejeita com eligibility/reservation/holder-mismatch
DADO uma reserva active
QUANDO ExtendReservation chega com newExpiresAt menor ou igual a expiresAt
ENTÃO rejeita com eligibility/reservation/extension-not-later
DADO uma reserva active
QUANDO ExtendReservation chega com newExpiresAt maior que maxExpiresAt
ENTÃO rejeita com eligibility/reservation/extension-limit-exceeded
DADO uma reserva active do holder
QUANDO ReleaseReservation chega com reason cancelled
ENTÃO a reserva vai a released com releaseReason cancelled e nada é emitido
DADO uma reserva active com expiresAt menor ou igual a at
QUANDO ExpireReservation chega
ENTÃO a reserva vai a expired e ReservationExpired é emitido
DADO uma reserva active com expiresAt no futuro
QUANDO ExpireReservation chega
ENTÃO rejeita com eligibility/reservation/not-expirable
```

## Escopo fora

- **REST do bff**: a simulação (`POST /v1/organizations/{id}/simulations`) e a criação de
  solicitação, com erros como `ELIGIBILITY_EXPIRED` e `ASSET_ALREADY_RESERVED`, ficam no `bff`
  (referência: `docs/design/91-contratos-api.md` §3.1 e §3.2).
- **Crédito, rating, score e precificação**: fora do contexto (PRD 06); elegível não é crédito
  aprovado.
- **Confirmação de titularidade e gravame**: permanece externa. A revalidação antes do registro
  do efeito (req. 8) é de Contracting e Effects; a reserva interna não a substitui.
- **Reserva atômica de lote**: a reserva é por ativo e um RPC por ativo. Reservar um lote de N
  ativos é N comandos, com compensação por `ReleaseReservation`. **[ABERTA: sem UPR
  multiagregado no kernel]**
- **Motor de regras configurável**: rejeitado por 06 §2; revisar se a frequência de mudança
  exceder a cadência de publicação.
- **Parâmetros concretos** das regras (resumo:D4), tipos de efeito incompatíveis (B3) e SLA por
  fonte (B5): dependem de decisão ou apuração.
- **Invalidação depois da assinatura** (06:E12): tratada em `09-design-effects-orchestrator.md`.
- **Eventos por ativo** (`AssetEligible`, `AssetIneligible`) e a retenção curta da simulação
  (93:RNF-V5): fora desta versão.
