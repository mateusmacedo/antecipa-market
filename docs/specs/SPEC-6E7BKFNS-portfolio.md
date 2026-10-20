---
id: SPEC-6E7BKFNS
slug: portfolio
title: Bounded context — portfolio
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-Z3YR96YS]
ticket_url: https://linear.app/mmda/issue/DEVS-86/spec-6e7bkfns-bounded-context-portfolio
subtask_urls: []
created: 2026-10-12
---

# SPEC-6E7BKFNS: Bounded context — portfolio

## Identidade

- **bounded_context**: `portfolio`
- **name**: `portfolio`
- **Convenções de tipo**: dinheiro é `…Minor: int` (unidade menor) mais `currency: string`;
  datas puras (`…Date`) são `string` ISO 8601; `instant` é inteiro de nanossegundos.
- **Linguagem ubíqua**:
  - *Portfólio* (`Portfolio`) - conjunto lógico dos recebíveis de uma organização, 1:1 com ela.
  - *Descoberta* - execução que consulta a fonte oficial, via Gateway, e produz uma posição.
  - *Snapshot* - fotografia imutável da posição num instante, com origem e `asOf`.
  - *Ativo financeiro* (`FinancialAsset`) - item de um snapshot: `book_entry_invoice` (duplicata
    escritural) ou `card_receivable_unit` (unidade de recebível, UR).
  - *Chave do ativo* (`assetKey`) - identidade estável do ativo na plataforma, independente do
    snapshot: `<assetType>:<sourceIdentity>`. É o `assetId` dos demais contextos.
  - *Posição externa* (`ExternalPosition`) - o que a fonte oficial informou por último sobre um
    ativo. A fonte prevalece; a projeção local é corrigida, nunca o contrário.
  - *Compromisso* (`CommitmentProjection`) - efeito regulatório conhecido pela plataforma.
  - *Disponibilidade* (`AvailabilityProjection`) - valor livre derivado e não autoritativo; é
    desconhecida (nula) quando a cobertura de gravame é incompleta.
  - *Alteração material* - mudança que altera o `contentHash` do ativo; só ela gera nova versão.
  - *Frescor* - `staleAfter = sourceAsOf + positionFreshnessSla` da fonte.
  - *Cobertura de gravame* - `complete | partial | unavailable`. Ausência de informação não
    significa ausência de gravame.
  - *Divergência* - diferença entre a projeção local e a posição externa.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/05-prd-portfolio.md`; `docs/design/05-design-portfolio.md` §2-§7;
  `docs/design/90-modelo-de-dados.md` §3.2; `docs/design/92-schemas-eventos.md` §3.2 e §4;
  `docs/design/93-requisitos-nao-funcionais.md` §2 (RNF-V1 a RNF-V5);
  `docs/design/00-arquitetura-tecnica.md` §5.3; `docs/design/04-design-receivables-gateway.md`
  §2.1; `docs/reference/glossario.md`.
- **Situação**: parcial. Hash de alteração material, frescor e disponibilidade são
  construíveis; identidade do ativo, campos canônicos, ingestão e diff dependem de #3 e #4.
- **Fase de entrega**: 1 (cartão); 2 (duplicata escritural).
- **Dependências**: duro: consent (`ExternalOptInActivated`; SPEC-Z3YR96YS) e gateway
  (`ReceivablesQueryPort`: `discoverSchedule` e `queryAsset`; SPEC-N1X148AW, implícito pela
  cadeia transitiva). `depends_on` declara só a fundação e a aresta dura reduzida (consent).
  Brando: eligibility (`AssetReserved`, `ReservationExpired`), effects (`EffectConfirmed`,
  `EffectReleased`), settlement (`SettlementReported`, `AssetSettled`) e partner (default de
  frescor, 05 §4). Essas quatro são jusante: arestas de retroalimentação, fora de `depends_on`.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
|---|---|---|
| #3 (B4; 05:P8) | Esquema de identidade da UR e da duplicata | `assetKey`, correspondência entre versões, `CompareSnapshots` |
| #4 (B7; 05:P9, 90:L1/L2) | Campos canônicos dos ativos | campos de `FinancialAsset`, ingestão |
| B5 (05:P10) | Validade da posição por fonte | `staleAfter` (hoje só o default conservador) |
| 05:P11 | Tolerância de valor na comparação | divergência `value_differs_beyond_tolerance` |
| 05:P12 | Retenção de snapshot não referenciado | expurgo de snapshots |
| #1; #12 | Autorização do BCB (transversal); volumetria | existencial; dimensionamento |

- **Decisões de produto abertas**: nenhuma. Técnicas: a definição de "alteração material" é a
  decisão D6 da fundação (SPEC-GXGASP24; `00-arquitetura-tecnica.md` §12). Esta spec a
  referencia e usa a proposta de 05 §2 como default até a decisão.
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída, SPEC-Z3YR96YS em
  `planning` ou adiante e #3/#4 resolvidos para cartão. Sem #3/#4 só o núcleo (hash, frescor,
  disponibilidade) pode ser planejado.

## Agregados

### Portfolio
- **Identidade**: `natural` por `organizationId`
- **Campos**:
  - `organizationId: id`
  - `openedAt: instant`
  - `discoveryStatus: idle | requested`
  - `activeDiscoveryId: id` (presente só em `requested`)
  - `lastDiscoveryRequestedAt: instant` (opcional)
  - `lastConfirmedAsOf: instant` (opcional)
  - `latestSnapshotId: id` (opcional)
  - `latestContentHash: string` (opcional)
- **Estados**: `idle`, `requested`
- **Transições permitidas**:
  - (novo) → `idle` - por `OpenPortfolio`
  - `idle` → `requested` - por `RequestDiscovery`
  - `requested` → `idle` - por `CompleteDiscovery`
- **Invariantes de estado**:
  - Há no máximo uma descoberta em curso: `RequestDiscovery` sobre `requested` rejeita com
    `portfolio/portfolio/discovery-in-progress`.
  - `latestSnapshotId` e `latestContentHash` só mudam quando uma descoberta termina com
    snapshot publicado.
  - `lastConfirmedAsOf` guarda o `asOf` de uma consulta que devolveu posição idêntica.
    Diverge do design: 05 §2 diz que a consulta idêntica "atualiza o `asOf` do snapshot
    vigente", mas o PRD e 90 §3.2 exigem snapshot publicado imutável; o `asOf` vai para o
    `Portfolio`. **[ABERTA: 05-design:§2 x 90:§3.2]**

### PortfolioSnapshot
- **Identidade**: `generated`
- **Campos**:
  - `portfolioId: id`
  - `discoveryId: id`
  - `purpose: contracting | simulation`
  - `status: building | published`
  - `windowFromDate: string`, `windowToDate: string` (janela consultada)
  - `assetTypes: list<string>` (nota: tipos de ativo da consulta, para comparar só a interseção
    de janelas e tipos)
  - `sources: list<SnapshotSource>` (nota: uma entrada por fonte consultada, indispensável por
    90 §3.2 e 92 §3.2; cada uma com `providerId: id`, `system: string`, `asOf: instant`,
    `coverage: complete | partial | unavailable`,
    `encumbranceCoverage: complete | partial | unavailable`)
  - `asOf: instant` (instante da posição, informado pela fonte)
  - `createdAt: instant`
  - `assetCount: int`, `grossTotalMinor: int`, `currency: string`
  - `byAssetType: list<AssetTypeSummary>` (nota: `assetType`, `count`, `grossTotalMinor`; é o
    `summary.byAssetType` do evento)
  - `contentHash: string` (hash do conjunto ordenado dos hashes dos ativos)
  - `staleAfter: instant`
  - `publishedAt: instant` (opcional)
  - `supersedesId: id` (opcional)
- **Estados**: `building`, `published`
- **Transições permitidas**:
  - (novo) → `building` - por `BeginSnapshot`
  - `building` → `published` - por `PublishSnapshot`
- **Invariantes de estado**:
  - Snapshot `published` é imutável: nenhum comando o altera, e `PublishSnapshot` sobre ele
    rejeita com `portfolio/portfolio-snapshot/not-building`.
  - Snapshot de simulação nunca é publicado (RNF-V3): `portfolio/portfolio-snapshot/simulation-snapshot-not-publishable`.
  - Não se publica posição já vencida: `staleAfter > at`, senão
    `portfolio/portfolio-snapshot/stale-position`.
  - Totais e `contentHash` são derivados dos itens da mesma versão (invariante do PRD). O caso
    de uso os calcula sobre os `FinancialAsset` do snapshot antes de chamar `PublishSnapshot`.
    **[ABERTA: atomicidade snapshot x ativos - uma UPR altera um agregado, e um snapshot tem
    até 5.000 ativos (93:§2.1)]**

### FinancialAsset
- **Identidade**: `natural` por `snapshotAssetKey` (`<snapshotId>:<assetKey>`)
- **Campos comuns**:
  - `snapshotId: id`, `assetKey: string`, `assetType: book_entry_invoice | card_receivable_unit`
  - `sourceIdentity: string` **[A CONFIRMAR: #3, B4]**
  - `providerId: id`
  - `externalReferences: list<ExternalReference>` (nota: N por ativo, 90 §2.1; cada uma com
    `providerId`, `system`, `referenceType`, `reference` exato como recebido, `asOf`,
    `observedVersion`)
  - `contentHash: string`
  - `grossAmountMinor: int`, `netAmountMinor: int`, `currency: string`
  - `situation: available | settled | unavailable` (mapeamento dos códigos da fonte
    **[A CONFIRMAR: #4, B7]**)
  - `encumbranceCoverage: complete | partial | unavailable`
  - `knownEncumbrances: list<Encumbrance>` (nota: `effectType: string`, `amountMinor: int`,
    `holderHash: string`; tipos de efeito **[A CONFIRMAR: #2, B3]**)
  - `adjustments: list<Adjustment>` (nota: `kind: string`, `amountMinor: int`, `origin: string`)
  - `sourceAsOf: instant`, `staleAfter: instant`
- **Campos de `card_receivable_unit`**: `holderHash`, `acquirerId: id`, `subAcquirerId: id`
  (opcional), `paymentScheme: string`, `settlementDate: string`.
- **Campos de `book_entry_invoice`**: `drawerHash`, `draweeHash`, `holderHash`,
  `fiscalDocumentRef: string`, `issueDate`, `dueDate`, `accepted: bool`, `bookkeeperId: id`.
  Lista de campos **[A CONFIRMAR: #4, B7]**.
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - Imutável depois de gravado; identificadores e referências da origem nunca se perdem.
  - `sourceIdentity` nunca é vazio - `portfolio/financial-asset/source-identity-empty`.
  - Valores não negativos - `portfolio/financial-asset/amount-negative`.
  - Diverge do design: 90 §3.2 põe `availableAmount` na `CardReceivableUnit`. Aqui ele vive só
    na `AvailabilityProjection`, porque o campo muda com reservas e o snapshot é imutável.
  - `staleAfter` vem do `positionFreshnessSla` do perfil do Gateway; sem SLA declarado vale o
    prazo conservador padrão, com alerta de configuração. **[ABERTA: 05:P10 - valor do prazo
    padrão]**

### ExternalPosition
- **Identidade**: `natural` por `assetKey`
- **Campos**:
  - `assetKey: string`, `assetType: string`, `providerId: id`
  - `contentHash: string`, `holderHash: string`
  - `situation: available | settled | unavailable`
  - `netAmountMinor: int`, `currency: string`
  - `encumbranceCoverage: complete | partial | unavailable`
  - `encumberedAmountMinor: int` (soma dos gravames conhecidos)
  - `settlementDate: string`
  - `sourceAsOf: instant`, `lastConfirmedAsOf: instant`, `staleAfter: instant`
  - `freshness: fresh | stale`
  - `latestSnapshotId: id`
  - `lastDivergenceKind: string` (opcional), `lastDivergenceAt: instant` (opcional)
- **Estados**: `fresh`, `stale`
- **Transições permitidas**:
  - (novo) → `fresh` - por `RecordExternalPosition`
  - `fresh` → `stale` - por `MarkPositionStale`
  - `stale` → `fresh` - por `RecordExternalPosition` com `sourceAsOf` mais novo
- **Invariantes de estado**:
  - `sourceAsOf` nunca regride: observação mais antiga rejeita com
    `portfolio/external-position/stale-observation`.
  - Há alteração material quando o `contentHash` muda. Entram no hash: identificador, valor
    líquido, data de liquidação, titular, situação, gravames, valor bruto, ajustes, credenciador
    e arranjo; ficam fora `asOf`, paginação e ordem (05 §2). **[ABERTA: 00:D6 - decisão da
    fundação]**

### CommitmentProjection
- **Identidade**: `natural` por `assetKey`
- **Campos**:
  - `assetKey: string`, `currency: string`
  - `committedAmountMinor: int`, `settledAmountMinor: int`, `reportedAmountMinor: int`
  - `effects: list<CommittedEffect>` (nota: vários efeitos concorrentes sobre o mesmo ativo,
    B13; cada um com `effectId: id`, `amountMinor: int`, `status: active | released`,
    `since: instant`)
  - `lastChangeAt: instant`, `correctedAt: instant` (opcional)
- **Estados**: nenhum no agregado (o estado vive em cada `CommittedEffect`)
- **Transições permitidas**: `active` → `released`, por efeito, em `ApplyEffectReleased`
- **Invariantes de estado**:
  - Um `effectId` aparece uma vez em `effects`.
  - `committedAmountMinor` é derivado dos efeitos `active` e pode ser substituído pela fonte em
    `CorrectCommitment`. A projeção é derivada e não autoritativa.

### AvailabilityProjection
- **Identidade**: `natural` por `assetKey`
- **Campos**:
  - `assetKey: string`, `currency: string`
  - `availability: unknown | known`
  - `availableAmountMinor: int` (presente só quando `known`)
  - `netAmountMinor: int`, `encumberedAmountMinor: int`, `reservedAmountMinor: int`
  - `asOf: instant`, `sourceProviderId: id`, `policyVersion: string`, `staleAfter: instant`
- **Estados**: `unknown`, `known`
- **Transições permitidas**: qualquer → qualquer, a cada recálculo
- **Invariantes de estado**:
  - Política (05 §3): cobertura diferente de `complete` → `unknown`; situação diferente de
    `available` → `known` com zero; senão `max(0, net - gravames - reservas)`.
  - Sempre carrega `asOf`, fonte e `policyVersion` (invariante do PRD) -
    `portfolio/availability-projection/policy-version-empty`.
  - Nunca fundamenta registro de efeito: este exige revalidação na fonte. O frescor é derivado
    na leitura: com `staleAfter <= agora` a resposta sai marcada como vencida.

## Comandos (UPRs)

### OpenPortfolio
- **Agregado**: `Portfolio`; **inicializa se ausente**
- **Entrada**: `organizationId: id`, `consentId: id`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**:
  - quando ausente: `openedAt ← at`, `discoveryStatus ← idle`
  - quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: nenhum
- **Resposta**: `organizationId: id`

### RequestDiscovery
- **Agregado**: `Portfolio`; exige existente
- **Entrada**: `discoveryId: id` (gerado antes da UoW), `purpose: contracting | simulation`,
  `assetTypes: list<string>`, `windowFromDate: string`, `windowToDate: string`
- **Pré-condições**:
  - `assetTypes` não vazio → `portfolio/portfolio/asset-types-empty`
  - `windowFromDate <= windowToDate` → `portfolio/portfolio/window-invalid`
  - `discoveryStatus = idle` → `portfolio/portfolio/discovery-in-progress`
- **Efeitos sobre o estado**: `discoveryStatus ← requested`, `activeDiscoveryId ←
  cmd.discoveryId`, `lastDiscoveryRequestedAt ← at`
- **Eventos emitidos**: `PortfolioDiscoveryRequested`
- **Resposta**: `discoveryId: id`

### CompleteDiscovery
- **Agregado**: `Portfolio`; exige existente
- **Entrada**: `discoveryId: id`, `outcome: snapshot_created | unchanged | simulated | failed`,
  `snapshotId: id`, `contentHash: string`, `asOf: instant` (os três opcionais conforme o
  desfecho), `failureCode: string` (opcional)
- **Pré-condições**:
  - `discoveryStatus = requested` → `portfolio/portfolio/no-discovery-in-progress`
  - `discoveryId = activeDiscoveryId` → `portfolio/portfolio/discovery-id-mismatch`
  - `outcome = snapshot_created` exige `snapshotId` e `contentHash` →
    `portfolio/portfolio/snapshot-required`
- **Efeitos sobre o estado**:
  - `snapshot_created`: `latestSnapshotId ← cmd.snapshotId`, `latestContentHash ←
    cmd.contentHash`, `lastConfirmedAsOf ← cmd.asOf`
  - `unchanged`: `lastConfirmedAsOf ← cmd.asOf`
  - `simulated` e `failed`: nenhum além do abaixo
  - sempre: `discoveryStatus ← idle`, `activeDiscoveryId` limpo
- **Eventos emitidos**: nenhum (`PortfolioSnapshotCreated` sai de `PublishSnapshot`)
- **Resposta**: `discoveryId: id`, `outcome`

### BeginSnapshot
- **Agregado**: `PortfolioSnapshot`; **cria**
- **Entrada**: `portfolioId: id`, `discoveryId: id`, `purpose`, `windowFromDate: string`,
  `windowToDate: string`, `assetTypes: list<string>`, `sources: list<SnapshotSource>`,
  `asOf: instant`
- **Pré-condições**:
  - `sources` não vazio → `portfolio/portfolio-snapshot/sources-empty`
  - `windowFromDate <= windowToDate` → `portfolio/portfolio-snapshot/window-invalid`
- **Efeitos sobre o estado**: `status ← building`, `createdAt ← at`, demais campos ← entrada,
  `assetCount ← 0`
- **Eventos emitidos**: nenhum
- **Resposta**: `snapshotId: id`

### PublishSnapshot
- **Agregado**: `PortfolioSnapshot`; exige existente
- **Entrada**: `assetCount: int`, `grossTotalMinor: int`, `currency: string`,
  `byAssetType: list<AssetTypeSummary>`, `contentHash: string`, `staleAfter: instant`,
  `supersedesId: id` (opcional)
- **Pré-condições**:
  - `status = building` → `portfolio/portfolio-snapshot/not-building`
  - `purpose = contracting` → `portfolio/portfolio-snapshot/simulation-snapshot-not-publishable`
  - `contentHash` não vazio → `portfolio/portfolio-snapshot/content-hash-empty`
  - `staleAfter > at` → `portfolio/portfolio-snapshot/stale-position`
- **Efeitos sobre o estado**: `status ← published`, `assetCount`, `grossTotalMinor`,
  `currency`, `byAssetType`, `contentHash`, `staleAfter`, `supersedesId ← cmd`;
  `publishedAt ← at`
- **Eventos emitidos**: `PortfolioSnapshotCreated`
- **Resposta**: `snapshotId: id`, `publishedAt: instant`

### RecordAsset
- **Agregado**: `FinancialAsset`; **inicializa se ausente**
- **Entrada**: todos os campos comuns e os do tipo (ver Agregados), mais `snapshotId: id`
- **Pré-condições**:
  - `sourceIdentity` não vazio → `portfolio/financial-asset/source-identity-empty`
  - `grossAmountMinor >= 0` e `netAmountMinor >= 0` → `portfolio/financial-asset/amount-negative`
  - quando presente, mesmo `contentHash` → `portfolio/financial-asset/content-conflict`
- **Efeitos sobre o estado**:
  - quando ausente: todos os campos ← entrada
  - quando presente: nenhum - aceita sem alterar (reenvio seguro da ingestão)
- **Eventos emitidos**: nenhum
- **Resposta**: `snapshotAssetKey: string`

### RecordExternalPosition
- **Agregado**: `ExternalPosition`; **inicializa se ausente**
- **Entrada**: `assetKey`, `assetType`, `providerId`, `contentHash`, `holderHash`,
  `situation`, `netAmountMinor`, `currency`, `encumbranceCoverage`, `encumberedAmountMinor`,
  `settlementDate`, `sourceAsOf`, `staleAfter`, `snapshotId`
- **Pré-condições**:
  - quando presente, `sourceAsOf >= sourceAsOf` atual → `portfolio/external-position/stale-observation`
- **Efeitos sobre o estado**:
  - ausente: campos ← entrada, `freshness ← fresh`, `lastConfirmedAsOf ← cmd.sourceAsOf`
  - presente, mesmo `contentHash`: `lastConfirmedAsOf ← cmd.sourceAsOf`, `staleAfter ←
    cmd.staleAfter`, `freshness ← fresh`; sem evento
  - presente, `contentHash` diferente (alteração material): campos ← entrada, `freshness ←
    fresh`, `latestSnapshotId ← cmd.snapshotId`
- **Eventos emitidos**: `AssetPositionChanged` (só na alteração material sobre posição que já existia)
- **Resposta**: `assetKey: string`, `materialChange: bool`

### MarkPositionStale
- **Agregado**: `ExternalPosition`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `staleAfter <= at` → `portfolio/external-position/still-fresh`
- **Efeitos sobre o estado**: `freshness ← stale` (já `stale`: aceita sem alterar e sem emitir)
- **Eventos emitidos**: `AssetBecameStale` (só na transição)
- **Resposta**: `assetKey: string`

### ReportDivergence
- **Agregado**: `ExternalPosition`; exige existente
- **Entrada**: `kind: local_encumbrance_missing_at_source | source_encumbrance_unknown_locally |
  holder_differs | value_differs_beyond_tolerance | situation_differs`, `localValue: string`,
  `externalValue: string`, `hasActivePlatformEffect: bool` (lido pelo caso de uso na
  `CommitmentProjection`)
- **Pré-condições**:
  - `localValue` diferente de `externalValue` → `portfolio/external-position/no-divergence`
- **Efeitos sobre o estado**: `lastDivergenceKind ← cmd.kind`, `lastDivergenceAt ← at`
- **Eventos emitidos**: `ExternalPositionDiverged`. A severidade sai da tabela de 05 §7:
  `high`, `critical`, `critical`, `medium`, `medium`, na ordem de `kind`.
  Tolerância de valor **[ABERTA: 05:P11]**
- **Resposta**: `assetKey: string`, `severity: medium | high | critical`

### ApplyEffectConfirmed
- **Agregado**: `CommitmentProjection`; **inicializa se ausente**
- **Entrada**: `assetKey`, `effectId: id`, `amountMinor: int`, `currency`, `confirmedAt: instant`
- **Pré-condições**:
  - `amountMinor > 0` → `portfolio/commitment-projection/amount-invalid`
- **Efeitos sobre o estado**:
  - ausente: valores zerados
  - `effects ←` acrescenta `{effectId, amountMinor, active, since: confirmedAt}` e
    `committedAmountMinor ← +amountMinor`; `lastChangeAt ← at`
  - `effectId` já presente: aceita sem alterar
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`

### ApplyEffectReleased
- **Agregado**: `CommitmentProjection`; exige existente
- **Entrada**: `assetKey`, `effectId: id`
- **Pré-condições**:
  - `effectId` presente em `effects` → `portfolio/commitment-projection/effect-unknown`
- **Efeitos sobre o estado**: efeito `status ← released` e `committedAmountMinor ← -amountMinor`
  do efeito; `lastChangeAt ← at` (já `released`: aceita sem alterar)
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`

### ApplySettlementReport
- **Agregado**: `CommitmentProjection`; exige existente
- **Entrada**: `assetKey`, `amountMinor: int`, `reportedAt: instant`
- **Pré-condições**:
  - `amountMinor > 0` → `portfolio/commitment-projection/amount-invalid`
- **Efeitos sobre o estado**: `reportedAmountMinor ← +amountMinor`, `lastChangeAt ← at`
  (pagamento reportado não extingue a obrigação; só `AssetSettled` o faz)
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`

### ApplyAssetSettled
- **Agregado**: `CommitmentProjection`; exige existente
- **Entrada**: `assetKey`, `settledAmountMinor: int`, `settledAt: instant`
- **Pré-condições**:
  - `settledAmountMinor > 0` → `portfolio/commitment-projection/amount-invalid`
- **Efeitos sobre o estado**: `settledAmountMinor ← +settledAmountMinor`, `lastChangeAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`

### CorrectCommitment
- **Agregado**: `CommitmentProjection`; exige existente
- **Entrada**: `assetKey`, `committedAmountMinor: int` (valor da fonte), `reason: string`
- **Pré-condições**:
  - `committedAmountMinor >= 0` → `portfolio/commitment-projection/amount-negative`
- **Efeitos sobre o estado**: `committedAmountMinor ← cmd`, `correctedAt ← at`,
  `lastChangeAt ← at`
- **Eventos emitidos**: nenhum (a divergência já saiu em `ReportDivergence`)
- **Resposta**: `assetKey: string`

### ProjectAvailability
- **Agregado**: `AvailabilityProjection`; **inicializa se ausente**
- **Entrada**: `assetKey`, `netAmountMinor: int`, `currency`, `situation`,
  `encumbranceCoverage`, `encumberedAmountMinor: int`, `asOf: instant`, `sourceProviderId: id`,
  `policyVersion: string`, `staleAfter: instant`
- **Pré-condições**:
  - `policyVersion` não vazio → `portfolio/availability-projection/policy-version-empty`
  - `netAmountMinor >= 0` → `portfolio/availability-projection/amount-negative`
- **Efeitos sobre o estado**: campos ← entrada; `availability` e `availableAmountMinor` ← política
  de 05 §3, com o `reservedAmountMinor` atual (zero quando ausente)
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`, `availability`

### ApplyAssetReserved
- **Agregado**: `AvailabilityProjection`; exige existente
- **Entrada**: `assetKey`, `reservedAmountMinor: int`
- **Pré-condições**:
  - `reservedAmountMinor > 0` → `portfolio/availability-projection/amount-invalid`
- **Efeitos sobre o estado**: `reservedAmountMinor ← +reservedAmountMinor`; recalcula
  `availability` e `availableAmountMinor`
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`

### ApplyReservationExpired
- **Agregado**: `AvailabilityProjection`; exige existente
- **Entrada**: `assetKey`, `releasedAmountMinor: int`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: `reservedAmountMinor ← max(0, reservedAmountMinor -
  releasedAmountMinor)`; recalcula `availability` e `availableAmountMinor`
- **Eventos emitidos**: nenhum
- **Resposta**: `assetKey: string`

## Eventos de domínio

Só `PortfolioSnapshotCreated.v1` tem schema em 92 §3.2. Os demais têm campos derivados do
agregado. **[ABERTA: 92-schemas-eventos:§3 sem schema de AssetPositionChanged,
AssetBecameStale e ExternalPositionDiverged]**

### PortfolioDiscoveryRequested
- **Campos**:
  - `discoveryId: id ← cmd.discoveryId`
  - `organizationId: id ← Portfolio.organizationId`
  - `purpose ← cmd.purpose`, `assetTypes: list<string> ← cmd.assetTypes`
  - `windowFromDate: string ← cmd.windowFromDate`, `windowToDate: string ← cmd.windowToDate`
  - `at: instant ← at`

### PortfolioSnapshotCreated
- **Campos**:
  - `snapshotId: id ← PortfolioSnapshot.id`, `portfolioId: id ← PortfolioSnapshot.portfolioId`
  - `organizationId: id ← Portfolio.organizationId` (via `portfolioId`)
  - `discoveryId: id ← PortfolioSnapshot.discoveryId`, `asOf: instant ← PortfolioSnapshot.asOf`
  - `sources: list<SnapshotSource> ← PortfolioSnapshot.sources`
  - `assetCount`, `grossTotalMinor`, `currency`, `byAssetType ← PortfolioSnapshot` (o `summary` de 92)
  - `contentHash: string ← PortfolioSnapshot.contentHash`
  - `supersedesSnapshotId: id ← PortfolioSnapshot.supersedesId`
  - `staleAfter: instant ← PortfolioSnapshot.staleAfter`
- O evento não leva a lista de ativos (exceção declarada em 92 §3.2 à convenção 3 de 92 §2): o
  consumidor a obtém por `ListAssetsBySnapshot`.

### AssetPositionChanged
- **Campos**:
  - `assetKey: string ← ExternalPosition.assetKey`, `assetType`, `providerId`
  - `previousContentHash: string`, `contentHash: string ← cmd.contentHash`
  - `changeKinds: list<value_lower | value_higher | new_encumbrance | holder_changed |
    situation_changed | settlement_date_changed | other_material>` (derivado da comparação
    entre o valor anterior e o novo)
  - `previous` e `current`: `netAmountMinor`, `situation`, `holderHash`, `settlementDate`,
    `encumberedAmountMinor`
  - `snapshotId: id ← cmd.snapshotId`, `sourceAsOf: instant ← cmd.sourceAsOf`
- `new_encumbrance` é sinal de risco de dupla negociação (05 §6).

### AssetBecameStale
- **Campos**: `assetKey: string`, `providerId: id`, `sourceAsOf: instant`,
  `staleAfter: instant`, `at: instant ← at`

### ExternalPositionDiverged
- **Campos**: `assetKey: string`, `kind ← cmd.kind`, `severity ←` tabela de 05 §7,
  `localValue: string`, `externalValue: string`, `hasActivePlatformEffect: bool`, `at: instant ← at`

## Consultas

Consultas correm fora da UoW. As de ativo paginam por cursor (91 §2.3).

- `FindPortfolio` - filtros: `organizationId`; cardinalidade: `one`
- `FindLatestPublishedSnapshot` - filtros: `organizationId`; cardinalidade: `one`
- `FindSnapshot` - filtros: `snapshotId`; cardinalidade: `one`
- `FindSnapshotsByPortfolio` - filtros: `portfolioId`, `status`, período de `createdAt`;
  cardinalidade: `many` (histórico consultável)
- `ListAssetsBySnapshot` - filtros: `snapshotId`, `assetType`, intervalo de `settlementDate`,
  `acquirerId`, `paymentScheme`; cardinalidade: `many`
- `GroupAssetsBySnapshot` - filtros: `snapshotId`, `dimension: settlement_date | drawee |
  payment_scheme | acquirer | risk`; cardinalidade: `many` (`risk` **[ABERTA: 05-prd - dimensão
  sem definição]**)
- `FindAssetInSnapshot` - filtros: `snapshotId`, `assetKey`; cardinalidade: `one`
- `CompareSnapshots` - filtros: `snapshotIdA`, `snapshotIdB`; cardinalidade: `one` (resultado
  `added`, `removed`, `changed` com campos, valor anterior e novo, e a contagem de `unchanged`;
  considera só a interseção das janelas e dos tipos de ativo, porque `removed` é ambíguo - 05 §6)
- `FindExternalPosition` - filtros: `assetKey`; cardinalidade: `one`
- `FindCommitmentProjection` - filtros: `assetKey`; cardinalidade: `one`
- `FindAvailability` - filtros: `assetKey`; cardinalidade: `one`
- `FindAvailabilityBySnapshot` - filtros: `snapshotId`, `assetType`; cardinalidade: `many`
  (leitura para seleção e simulação; cada item sai com `stale` derivado de `staleAfter`)

## Relações entre agregados

- `Portfolio.latestSnapshotId` → `PortfolioSnapshot`
- `PortfolioSnapshot.portfolioId` → `Portfolio`
- `FinancialAsset.snapshotId` → `PortfolioSnapshot`
- `FinancialAsset.assetKey` → `ExternalPosition`
- `ExternalPosition.latestSnapshotId` → `PortfolioSnapshot`
- `CommitmentProjection.assetKey` → `ExternalPosition`
- `AvailabilityProjection.assetKey` → `ExternalPosition`
- Para outros contextos, só por identidade: `Portfolio.organizationId` → `Organization`
  (identity); `CommitmentProjection.effects[].effectId` → `RegulatoryEffect` (effects).

## Integração

- **Publica** (integration events), em
  `apps/backend/portfolio/contract/proto/company/portfolio/event/v1/`, package
  `company.portfolio.event.v1`:
  - `PortfolioSnapshotCreated` - `portfolio_snapshot_created.proto` (Eligibility, Ledger)
  - `AssetPositionChanged` - `asset_position_changed.proto` (Eligibility, Settlement, Ledger)
  - `AssetBecameStale` - `asset_became_stale.proto` (Eligibility)
  - `ExternalPositionDiverged` - `external_position_diverged.proto` (Settlement, Risco, Ledger)
  - `PortfolioDiscoveryRequested` fica só como evento de domínio: 92 §4 não o lista.
    **[ABERTA: 92-schemas-eventos:§4 sem PortfolioDiscoveryRequested, que o PRD 05 lista]**
- **Consome** (por contrato; disposições de consumo conforme FND-04 §6.4):
  - `company.consent.event.v1.ExternalOptInActivated` → `OpenPortfolio`. Mapa:
    `organizationId ← organizationId`, `consentId ← consentId`. **[ABERTA: 92:§3.1 não define o
    payload de ExternalOptInActivated]**
  - `company.eligibility.event.v1.AssetReserved` → `ApplyAssetReserved`. Mapa: `assetKey ←
    assetId`, `reservedAmountMinor ← reservedAmountMinor`. **[ABERTA: 06 reserva por ativo sem
    valor; o valor reservado é o líquido do ativo]**
  - `company.eligibility.event.v1.ReservationExpired` → `ApplyReservationExpired`. Mapa:
    `assetKey ← assetId`, `releasedAmountMinor ← reservedAmountMinor`.
  - `company.effects.event.v1.EffectConfirmed` → `ApplyEffectConfirmed`, uma vez por alvo
    `CONFIRMED`. Mapa: `assetKey ← targets[].assetRef.internalId`, `effectId ← effectId`,
    `amountMinor ← targets[].confirmedAmount.amountMinor`, `confirmedAt ← confirmedAt`.
    **[ABERTA: um evento com N alvos (1.284 no exemplo de 92 §3.4) dispara N UPRs; fan-out de
    consumo sem suporte verificado no kernel]**
  - `company.effects.event.v1.EffectReleased` → `ApplyEffectReleased`, por alvo. Mapa:
    `assetKey ← targets[].assetRef.internalId`, `effectId ← effectId`. **[ABERTA: payload não
    definido em 92:§3]**
  - `company.settlement.event.v1.SettlementReported` → `ApplySettlementReport` e
    `company.settlement.event.v1.AssetSettled` → `ApplyAssetSettled`. Mapa: `assetKey ←
    assetRef.internalId`, valor ← valor do evento. **[ABERTA: payloads não definidos em 92:§3]**
  - `EffectPartiallyConfirmed` não é consumido: 92 §4 lista Portfolio só em `EffectConfirmed`,
    mas os alvos confirmados de um efeito parcial também são compromisso. **[ABERTA:
    92-schemas-eventos:§4]**
  - Liberação de reserva por conclusão, cancelamento ou invalidação não tem evento em 92 §4
    (só `ReservationExpired`): o Portfolio não zera `reservedAmountMinor` nesses casos.
    **[ABERTA: 92-schemas-eventos:§4 x 06:§6.2]**
- **Consultas de saída (gRPC)**: ao Gateway, `DiscoverSchedule` e `QueryAsset`
  (`ReceivablesQueryPort`), e `FindEffectiveCapabilityProfile` para o `positionFreshnessSla`.
- **Orquestração da descoberta** (caso de uso): `RequestDiscovery` → `DiscoverSchedule` →
  normalização e hash → se igual a `latestContentHash`, `CompleteDiscovery(unchanged)`; senão
  `BeginSnapshot`, N vezes `RecordAsset`, `RecordExternalPosition` e `ProjectAvailability`,
  `PublishSnapshot` e `CompleteDiscovery(snapshot_created)`. **[ABERTA: ciclo sem rotina de
  abandono de snapshot `building` e sem lote multiagregado no kernel]**
- Nota de inconsistência: o PRD 05 lista `AssetSettled` entre os eventos do Portfolio, mas 92 §4
  o atribui ao Settlement e o Portfolio apenas o consome.

## Borda e persistência

- **Borda**: só gRPC, `company.portfolio.service.v1.PortfolioService`, um RPC por comando e por
  consulta, atrás da cadeia de interceptors do kernel. O REST público é do `bff` (Escopo fora).
- **Banco**: database e role `portfolio`, schema `public`, tabelas `portfolios`,
  `portfolio_snapshots`, `financial_assets`, `external_positions`, `commitment_projections` e
  `availability_projections`, com `tenant_id` à frente da chave. Persistência híbrida; colunas
  tipadas só onde uma consulta filtra:
  - `portfolio_snapshots`: `portfolio_id`, `status`, `created_at` (`FindSnapshotsByPortfolio`)
  - `financial_assets`: `snapshot_id`, `asset_type`, `settlement_date`, `acquirer_id`,
    `payment_scheme`, `drawee_hash` (`ListAssetsBySnapshot`, `GroupAssetsBySnapshot`)
  - `external_positions` e `availability_projections`: `stale_after`
  - o resto do estado vai em `snapshot` `jsonb`
- Particionar `financial_assets` por `settlement_date` (RNF-V4) e impor a imutabilidade de
  snapshot publicado por permissão de banco (90 §3.2). **[ABERTA: suporte do `postgres.Table`
  do kernel a partição declarativa e a UPDATE restrito por estado]**
- **Instante**: inteiro de nanossegundos no domínio.

## Políticas transversais

- **Idempotência**: `RequestDiscovery` entra pelo `bff` por `POST` com chave de idempotência
  declarada (ADR-056; FND-04 §7.6). Os demais comandos são idempotentes por identidade natural
  do agregado (`RecordAsset`, `RecordExternalPosition`, `OpenPortfolio`, `Apply*`), e o consumo
  por inbox com `eventId` (FND-04 §6.4).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação
  (FND-04 §3.2). A projeção do portfólio para o financiador é cópia sob concessão no tenant dele,
  e o backoffice lê entre tenants. **[ABERTA: a capacidade existe no kernel desde a `1.0.0-rc.6`
  (DEVS-100, ADR-068 do DMPF); o uso aqui depende da adoção da rc.6 (DEVS-136)]**
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); o Ledger
  preserva os snapshots por `PortfolioSnapshotCreated`.
- **Classificação de dados**: valores `CONFIDENTIAL`; identificadores fiscais só como hash
  (90 §1.4 e §2.2); nada `PII_SENSITIVE` em evento.
- **Retenção**: snapshot não referenciado por oferta, contrato ou efeito tem retenção reduzida
  (RNF-V5). **[ABERTA: 05:P12 - prazo e política]**

## Critérios de aceite

- [ ] Os gates do workspace verdes para `portfolio` (`fmt-check`, `vet`, `build`, `lint`,
  `test-race` e `test-distributed` com Postgres, verificador com `--base` e
  `dmpf-context-check`).
- [ ] Cenários:

```text
DADO nenhum portfólio da organização
QUANDO OpenPortfolio chega por ExternalOptInActivated
ENTÃO o portfólio nasce idle com openedAt = at e nada é emitido; um segundo OpenPortfolio aceita sem alterar
DADO um portfólio idle
QUANDO RequestDiscovery chega com assetTypes e janela válidos
ENTÃO discoveryStatus vai a requested, activeDiscoveryId recebe o discoveryId e PortfolioDiscoveryRequested é emitido
DADO um portfólio idle
QUANDO RequestDiscovery chega com assetTypes vazio
ENTÃO rejeita com portfolio/portfolio/asset-types-empty
DADO um portfólio idle
QUANDO RequestDiscovery chega com windowFromDate depois de windowToDate
ENTÃO rejeita com portfolio/portfolio/window-invalid
DADO um portfólio requested
QUANDO RequestDiscovery chega
ENTÃO rejeita com portfolio/portfolio/discovery-in-progress e o estado não muda
DADO um portfólio requested
QUANDO CompleteDiscovery chega com outcome snapshot_created, snapshotId e contentHash
ENTÃO latestSnapshotId e latestContentHash são gravados e discoveryStatus volta a idle
DADO um portfólio requested
QUANDO CompleteDiscovery chega com outcome unchanged
ENTÃO lastConfirmedAsOf é atualizado, latestSnapshotId não muda e discoveryStatus volta a idle
DADO um portfólio idle
QUANDO CompleteDiscovery chega
ENTÃO rejeita com portfolio/portfolio/no-discovery-in-progress
DADO um portfólio requested
QUANDO CompleteDiscovery chega com discoveryId diferente do ativo
ENTÃO rejeita com portfolio/portfolio/discovery-id-mismatch
DADO um portfólio requested
QUANDO CompleteDiscovery chega com outcome snapshot_created sem snapshotId
ENTÃO rejeita com portfolio/portfolio/snapshot-required
DADO uma descoberta de contratação
QUANDO BeginSnapshot chega com uma fonte
ENTÃO o snapshot nasce building com createdAt = at e assetCount 0
DADO qualquer entrada
QUANDO BeginSnapshot chega com sources vazio
ENTÃO rejeita com portfolio/portfolio-snapshot/sources-empty
DADO qualquer entrada
QUANDO BeginSnapshot chega com janela invertida
ENTÃO rejeita com portfolio/portfolio-snapshot/window-invalid
DADO um snapshot building de contratação
QUANDO PublishSnapshot chega com contentHash e staleAfter no futuro
ENTÃO o snapshot vai a published com publishedAt = at e PortfolioSnapshotCreated vai à outbox
DADO um snapshot já published
QUANDO PublishSnapshot chega
ENTÃO rejeita com portfolio/portfolio-snapshot/not-building e o snapshot não muda
DADO um snapshot building de simulação
QUANDO PublishSnapshot chega
ENTÃO rejeita com portfolio/portfolio-snapshot/simulation-snapshot-not-publishable
DADO um snapshot building
QUANDO PublishSnapshot chega com contentHash vazio
ENTÃO rejeita com portfolio/portfolio-snapshot/content-hash-empty
DADO um snapshot building
QUANDO PublishSnapshot chega com staleAfter menor ou igual a at
ENTÃO rejeita com portfolio/portfolio-snapshot/stale-position
DADO um snapshot building
QUANDO RecordAsset chega com sourceIdentity e valores válidos
ENTÃO o ativo é gravado imutável, sem availableAmount; o mesmo contentHash de novo aceita sem alterar
DADO um ativo já gravado no snapshot
QUANDO RecordAsset chega com outro contentHash
ENTÃO rejeita com portfolio/financial-asset/content-conflict
DADO qualquer entrada
QUANDO RecordAsset chega com sourceIdentity vazia
ENTÃO rejeita com portfolio/financial-asset/source-identity-empty
DADO qualquer entrada
QUANDO RecordAsset chega com netAmountMinor negativo
ENTÃO rejeita com portfolio/financial-asset/amount-negative
DADO nenhuma posição externa do ativo
QUANDO RecordExternalPosition chega
ENTÃO a posição nasce fresh, lastConfirmedAsOf = sourceAsOf e nada é emitido
DADO uma posição externa com o mesmo contentHash
QUANDO RecordExternalPosition chega com sourceAsOf mais novo
ENTÃO lastConfirmedAsOf avança, freshness volta a fresh e nada é emitido
DADO uma posição externa
QUANDO RecordExternalPosition chega com contentHash diferente
ENTÃO os campos são substituídos e AssetPositionChanged vai à outbox com changeKinds, valor anterior e novo
DADO uma posição externa com sourceAsOf T
QUANDO RecordExternalPosition chega com sourceAsOf anterior a T
ENTÃO rejeita com portfolio/external-position/stale-observation
DADO uma posição fresh com staleAfter vencido
QUANDO MarkPositionStale chega
ENTÃO freshness vai a stale e AssetBecameStale é emitido; repetir aceita sem alterar e sem emitir
DADO uma posição com staleAfter no futuro
QUANDO MarkPositionStale chega
ENTÃO rejeita com portfolio/external-position/still-fresh
DADO uma posição externa
QUANDO ReportDivergence chega com kind source_encumbrance_unknown_locally
ENTÃO ExternalPositionDiverged é emitido com severity critical e lastDivergenceKind é gravado
DADO uma posição externa
QUANDO ReportDivergence chega com localValue igual a externalValue
ENTÃO rejeita com portfolio/external-position/no-divergence
DADO nenhuma projeção de compromisso do ativo
QUANDO ApplyEffectConfirmed chega com amountMinor 1000000
ENTÃO a projeção nasce com o efeito active e committedAmountMinor 1000000; o mesmo effectId de novo aceita sem alterar
DADO qualquer entrada
QUANDO ApplyEffectConfirmed chega com amountMinor 0
ENTÃO rejeita com portfolio/commitment-projection/amount-invalid
DADO uma projeção com um efeito active
QUANDO ApplyEffectReleased chega para esse effectId
ENTÃO o efeito vai a released e committedAmountMinor diminui do valor dele
DADO uma projeção sem o effectId
QUANDO ApplyEffectReleased chega
ENTÃO rejeita com portfolio/commitment-projection/effect-unknown
DADO uma projeção existente
QUANDO ApplySettlementReport chega com amountMinor positivo
ENTÃO reportedAmountMinor aumenta e committedAmountMinor não muda
DADO uma projeção existente
QUANDO ApplyAssetSettled chega com settledAmountMinor positivo
ENTÃO settledAmountMinor aumenta
DADO uma projeção existente
QUANDO ApplySettlementReport ou ApplyAssetSettled chega com valor zero
ENTÃO rejeita com portfolio/commitment-projection/amount-invalid
DADO uma projeção existente
QUANDO CorrectCommitment chega com o valor da fonte
ENTÃO committedAmountMinor é substituído e correctedAt = at
DADO qualquer entrada
QUANDO CorrectCommitment chega com committedAmountMinor negativo
ENTÃO rejeita com portfolio/commitment-projection/amount-negative
DADO um ativo com cobertura de gravame partial
QUANDO ProjectAvailability chega
ENTÃO availability é unknown, sem availableAmountMinor, com asOf, fonte e policyVersion
DADO cobertura complete, situação available, líquido 1000, gravame 300 e reserva 0
QUANDO ProjectAvailability chega
ENTÃO availability é known com availableAmountMinor 700; com situação settled seria known com 0
DADO qualquer entrada
QUANDO ProjectAvailability chega com policyVersion vazia
ENTÃO rejeita com portfolio/availability-projection/policy-version-empty
DADO qualquer entrada
QUANDO ProjectAvailability chega com netAmountMinor negativo
ENTÃO rejeita com portfolio/availability-projection/amount-negative
DADO uma disponibilidade known de 700
QUANDO ApplyAssetReserved chega com reservedAmountMinor 700
ENTÃO reservedAmountMinor vai a 700 e availableAmountMinor a 0
DADO qualquer entrada
QUANDO ApplyAssetReserved chega com reservedAmountMinor 0
ENTÃO rejeita com portfolio/availability-projection/amount-invalid
DADO uma disponibilidade com reserva de 700
QUANDO ApplyReservationExpired chega com releasedAmountMinor 700
ENTÃO reservedAmountMinor volta a 0 e availableAmountMinor é recalculado
```

## Escopo fora

- **REST do bff**: `POST /portfolios/{id}/discoveries`, `GET /portfolios/{id}/snapshots/latest`,
  `GET /portfolio-snapshots/{id}/assets` e `GET /portfolio-snapshots/{id}/changes` ficam no
  `bff` (PRD 05; referência: `docs/design/91-contratos-api.md` §2.3 e §6).
- **Limite de frequência de descoberta** (6 por hora por organização, 91 §6; RNF-V2) e a
  sobreposição mínima entre execuções: política da borda e do `PartnerAgreement`, não do domínio.
- **Consulta à fonte, protocolo e idempotência de saída**: do Gateway; o Portfolio só consome a
  porta de consulta.
- **Reserva autoritativa**: de Eligibility (unicidade por ativo); o Portfolio só projeta a
  disponibilidade a partir dos eventos.
- **Decidir a alteração material**: é a decisão D6 da fundação (SPEC-GXGASP24). Aqui vale a
  proposta de 05 §2 até lá.
- **Elegibilidade, preço e crédito**: de Eligibility e Marketplace.
- **Conciliação de liquidação e escalada de divergência crítica a disputa**: de Settlement e de
  Effects (09 §8).
- **Recebíveis futuros** (fase 4) e **sacado da duplicata**: fora desta versão; a duplicata
  escritural é só modelada, e sua ingestão depende de #3 e #4.
- **Gatilho temporal do frescor** (quem chama `MarkPositionStale` ao vencer `staleAfter`):
  **[ABERTA: sem agendador verificado no kernel]**. Até lá, a leitura deriva o frescor.
- **Retenção e expurgo de snapshot não referenciado**: depende de 05:P12.
