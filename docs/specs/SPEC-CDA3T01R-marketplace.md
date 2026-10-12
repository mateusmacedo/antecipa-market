---
id: SPEC-CDA3T01R
slug: marketplace
title: Bounded context — marketplace
stage: backlog
priority: P1
depends_on: [SPEC-GXGASP24, SPEC-GM5GH5BF]
ticket_url: https://linear.app/mmda/issue/DEVS-88/spec-cda3t01r-bounded-context-marketplace
subtask_urls: []
created: 2026-10-12
---

# SPEC-CDA3T01R: Bounded context — marketplace

## Identidade

- **bounded_context**: `marketplace`
- **name**: `marketplace`
- **Convenções de tipo**: dinheiro é `…Minor: int` (unidade menor) mais `currency: string`;
  taxas são `decimal`; `instant` é inteiro de nanossegundos. Decisões numeradas do resumo
  executivo aparecem como `resumo:Dn`.
- **Linguagem ubíqua**:
  - *Pedido* (`FundingRequest`) - intenção do cedente sobre um lote elegível.
  - *Convite* (`PartnerInvitation`) - convite a um financiador autorizado. Silêncio é `expired`,
    distinto de recusa explícita (`declined`).
  - *Oferta* (`FinancingOffer`) - proposta de um financiador, guardada como recebida (original)
    e normalizada (`pricing`), sem reprecificar.
  - *Taxa efetiva mensal* - métrica primária de comparação: `(bruto / líquido) ^ (30 / prazo
    médio ponderado) - 1`.
  - *Grupo de comparabilidade* - ofertas que só se comparam entre si: mesma cobertura, mesmo
    prazo médio, mesma estrutura de coobrigação e garantia.
  - *Política de ordenação* (`RankingPolicy`) - critério versionado e auditável de apresentação.
  - *Apresentação* (`OfferPresentation`) - registro imutável do que foi mostrado ao cedente.
  - *Seleção* (`OfferSelection`) - escolha autenticada do cliente, com revalidação.
  - *Patrocínio* - posição marcada e com remuneração exibida; altera posição, nunca valores.
  - *Contraproposta* - oferta sobre subconjunto dos ativos; não é negociação iterativa.
  - A plataforma não aprova crédito, não estima preço e não recomenda.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/07-prd-marketplace.md`; `docs/design/07-design-marketplace.md` §2-§7;
  `docs/design/90-modelo-de-dados.md` §3.3 e §4; `docs/design/92-schemas-eventos.md` §3.3 e §4;
  `docs/design/91-contratos-api.md` §3.2-§3.4 e §4.1-§4.2;
  `docs/design/06-design-eligibility.md` §6; `docs/reference/glossario.md`.
- **Situação**: parcial. Com um único financiador o contexto roda sem remuneração visível e sem
  patrocínio; competição, remuneração e patrocínio dependem de #7 e de decisão de produto.
- **Fase de entrega**: 1 (um financiador); 3 (competição entre financiadores).
- **Dependências**: duro: eligibility (`EligibleLotCreated`, `ReservationExpired`,
  `EligibilityInvalidated`; SPEC-GM5GH5BF), partner (`CapabilityEnabled`, `CapabilitySuspended`,
  `PartnerProductChanged`) e consent (`ConsentRevoked`, 07 §4.2). `depends_on` declara só a
  fundação e a aresta dura reduzida (eligibility); partner e consent ficam implícitos pela cadeia
  transitiva. Brando: kyb (`VerificationReady`), risk (REVIEW/BLOCK, 15 §5) e billing
  (`platformRemuneration`, 90 §3.3). Contracting é jusante: retroalimentação, fora de `depends_on`.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
|---|---|---|
| #7 (D1-D3) | Modelo de monetização | `pricing.platformRemuneration` e sua exibição (R2) |
| resumo:D7 (07:M10) | Patrocínio pode inverter mérito (07 §3.3 R6) | `RankingPolicy.sponsored*` e a regra de posição |
| A15 (07:M11) | CET para tomador PJ | campo de CET em `pricing` |
| E3 (07:M9) | Validação do agrupamento de ofertas não comparáveis | `groupId` e a apresentação |
| 07:M12 | Janelas de supressão por reprovação | item 6 da elegibilidade do convite |
| 07:M13 | Formato do documento de oferta | `FinancingOffer.documentRef` |
| #1; #12 | Autorização do BCB (transversal); volumetria | existencial; dimensionamento |

- **Decisões de produto abertas**: patrocínio e inversão de mérito (resumo:D7; 07 §3.3 R6);
  modo `waves` (07 §4.1, que contraria o objetivo de competição); janelas de supressão (07:M12).
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída e SPEC-GM5GH5BF em
  `planning` ou adiante. Para um financiador sem patrocínio, nenhum bloqueio externo restante
  trava o núcleo; a fase 3 exige #7 e resumo:D7.

## Agregados

### FundingRequest

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`, `lotId: id`, `evaluationId: id`, `snapshotId: id`, `version: int`
  - `assetCount: int`, `lotNetTotalMinor: int`, `currency: string`
  - `status: draft | open | collecting_offers | offers_available | offer_selected | contracting |
    closed | no_offer | expired | cancelled | eligibility_invalidated`
  - `invitationMode: all_authorized | selected | waves` (opcional até `OpenFundingRequest`)
  - `offerWindowClosesAt: instant`, `reservationExpiresAt: instant` (opcionais)
  - `dataSharingConsentId: id`, `openedByRepresentationId: id` (opcionais)
  - `createdAt: instant`, `openedAt: instant`, `closedAt: instant`, `closeReason: string` (opcionais)
- **Estados**: os onze de `status`; terminais: `closed`, `no_offer`, `expired`, `cancelled`,
  `eligibility_invalidated`.
- **Transições permitidas** (07 §6.1):
  - (novo) → `draft` - por `DraftFundingRequest`
  - `draft` → `open` - por `OpenFundingRequest`
  - `open` → `collecting_offers` → `offers_available` → `offer_selected` → `contracting` - por
    `AdvanceFundingRequest`
  - `offer_selected` → `offers_available` e `contracting` → `offers_available` - por
    `AdvanceFundingRequest`, na anulação da seleção e na reabertura de 07 §6.2.
    **[ABERTA: 07:§6.1 não lista a reabertura que 07 §6.2 exige]**
  - qualquer não terminal → terminal - por `CloseFundingRequest`
- **Invariantes de estado**:
  - Terminal não sai do estado: `CloseFundingRequest` repetido rejeita com
    `marketplace/funding-request/already-closed`.
  - Uma oferta pertence a um financiador e a uma versão do pedido; `version` vale 1 e nenhum
    comando desta spec a altera. **[ABERTA: 07-prd - versão do pedido sem comando que a mude]**
  - O pedido só se abre com lote vigente: o rascunho nasce de `EligibleLotCreated` e
    `EligibilityInvalidated` o encerra. Diverge do design: 91 §3.2 cria e abre o pedido num
    único `POST`; aqui o `bff` traduz o `POST` em `OpenFundingRequest` sobre o rascunho.
    **[ABERTA: 91-contratos-api:§3.2 x 90:§4 (EligibleLot 1:1 FundingRequest)]**

### PartnerInvitation

- **Identidade**: `natural` por `invitationKey` (`<fundingRequestId>:<funderOrganizationId>`)
- **Campos**:
  - `fundingRequestId: id`, `requestVersion: int`, `funderOrganizationId: id`, `productId: id`
  - `disclosureLevel: string` (`screening` no mínimo, 91 §4.1; demais níveis
    **[ABERTA: 95:P4 e 91:A2 - validação LGPD]**)
  - `status: created | sent | delivered | offered | declined | expired`
  - `createdAt: instant`, `expiresAt: instant`, `sentAt: instant`, `deliveredAt: instant`
    (os dois últimos opcionais)
  - `declineReason: string`, `closeReason: window_ended | consent_revoked | capability_suspended |
    product_changed | request_closed` (opcionais)
- **Estados**: `created`, `sent`, `delivered`, `offered`, `declined`, `expired`
- **Transições permitidas** (07 §4.3):
  - (novo) → `created` - por `InviteFunder`
  - `created` → `sent`, `sent` → `delivered` - por `RecordInvitationDelivery`
  - `sent` → `offered` e `delivered` → `offered` - por `RecordInvitationOffered`
  - `sent` → `declined` e `delivered` → `declined` - por `RecordInvitationDeclined`
  - `created`, `sent` ou `delivered` → `expired` - por `CloseInvitation`
- **Invariantes de estado**:
  - Um convite por pedido e financiador: repetir `InviteFunder` rejeita com
    `marketplace/partner-invitation/already-invited`.
  - `offered`, `declined` e `expired` são terminais; silêncio é `expired`, nunca `declined`.
  - Convite não entregue não derruba o pedido: os demais seguem.

### FinancingOffer

- **Identidade**: `generated`
- **Campos**:
  - `fundingRequestId: id`, `requestVersion: int`, `invitationKey: string`
  - `funderOrganizationId: id`, `productId: id`, `funderOfferReference: string`
  - `originalPayload: string` (a proposta exatamente como recebida; `CONFIDENTIAL`; vai no
    `snapshot`), `originalPayloadHash: string`
  - `documentRef: string` (opcional; documento bruto do financiador
    **[ABERTA: provider de object storage]**)
  - `coveredAssetIds: list<id>` (nota: pode ser subconjunto do lote; indispensável para a
    cobertura e a comparabilidade)
  - `isCounterOffer: bool` (verdadeiro quando `coveredAssetIds` é subconjunto)
  - `conditions`: `recourse: bool`, `coobligation: bool`, `additionalGuarantees: list<string>`
  - `pricing` (objeto embutido `PricingBreakdown`, abaixo)
  - `validUntil: instant`, `receivedAt: instant`
  - `status: received | expired | rejected_as_invalid | withdrawn`
  - `rejectionReason: request_version_stale | request_closed | asset_not_in_request |
    capability_not_enabled | incomplete_pricing | pricing_inconsistent | valid_until_invalid`
    (opcional, só em `rejected_as_invalid`)
- **`pricing`** (90 §3.3): `grossAmountMinor`, `discountAmountMinor`, `netAmountMinor`,
  `totalCostAmountMinor` (`bruto - líquido`), `weightedAverageTermDays: int`,
  `effectiveMonthlyRate: decimal`, `calculationMethodId: string`, `currency`;
  `feeItems: list<FeeItem>` (`code`, `description`, `amountMinor`, `chargedBy`) e
  `taxItems: list<TaxItem>` (`code`, `baseMinor`, `amountMinor`); `platformRemuneration:
  int` opcional (valor; **[A CONFIRMAR: #7]**). Escala do `decimal` da taxa
  **[ABERTA: 90:§2.3 só exemplifica 4 casas]**. CET **[A CONFIRMAR: A15]**. Diverge do insumo,
  que lista `PricingBreakdown` como agregado: em 90 §3.3 é objeto embutido da oferta.
- **Estados**: `received`, `expired`, `rejected_as_invalid`, `withdrawn`
- **Transições permitidas**:
  - (novo) → `received` ou `rejected_as_invalid` - por `ReceiveOffer`
  - `received` → `expired` - por `ExpireOffer`
- **Invariantes de estado**:
  - `originalPayload` e seu hash nunca mudam; a normalização é derivada e a ordenação nunca
    altera os valores originais.
  - `netAmountMinor = bruto - deságio - tarifas - tributos`, senão a oferta é gravada com
    `rejectionReason = pricing_inconsistent`. A plataforma valida a aritmética, nunca a
    razoabilidade do preço.
  - Oferta inválida é gravada com o motivo, visível ao financiador; não se rejeita em silêncio.
  - Diverge do design: 90 §3.3 lista `PRESENTED` e `SELECTED` como status; aqui são derivados de
    `OfferPresentation` e `OfferSelection`. `withdrawn` não tem comando em 91.
    **[ABERTA: 90:§3.3 status withdrawn sem rota]**

### RankingPolicy

- **Identidade**: `natural` por `policyKey` (`<policyId>:<version>`)
- **Campos**:
  - `policyId: string`, `version: int`, `status: published | retired`, `effectiveFrom: instant`
  - `criteria: list<RankingCriterion>` (nota: `field`, `direction: asc | desc`, `weight:
    decimal` opcional; ordem de aplicação indispensável)
  - `grouping: string`, `tieBreakers: list<string>`
  - `sponsoredEnabled: bool`, `sponsoredMaxPositions: int`, `disclosureRequired: bool`
  - `policyHash: string`
- **Estados**: `published`, `retired`
- **Transições permitidas**: (novo) → `published` por `PublishRankingPolicy`; `published` →
  `retired` por `RetireRankingPolicy`
- **Invariantes de estado**:
  - Versão publicada nunca muda; `policyHash` identifica o conteúdo e torna a ordenação
    reproduzível.
  - Política padrão proposta (07 §3.2): grupo de comparabilidade, taxa efetiva crescente, valor
    líquido decrescente, `validUntil` decrescente, `receivedAt` crescente (determinismo).
  - Patrocínio habilitado exige `disclosureRequired` e `sponsoredMaxPositions` declarados (R1,
    R2, R4). Por default patrocínio não ocupa a primeira posição quando há oferta com taxa
    melhor. **[ABERTA: resumo:D7 - 07:M10]**

### OfferPresentation

- **Identidade**: `generated`
- **Campos**:
  - `fundingRequestId: id`, `requestVersion: int`, `presentedAt: instant`,
    `presentedToUserId: id`
  - `rankingPolicyKey: string`, `rankingPolicyHash: string`, `calculationMethodId: string`
  - `entries: list<PresentedOffer>` (nota: indispensável para reconstruir a apresentação; cada
    uma com `offerId`, `position: int`, `groupId: string`, `rankingScore: decimal`,
    `sponsored: bool`, `platformRemunerationMinor: int` opcional, `originalPayloadHash`,
    `netAmountMinor`, `totalCostAmountMinor`, `effectiveMonthlyRate`,
    `weightedAverageTermDays`, `validUntil`)
  - `unsponsoredOrder: list<id>` (a ordenação sem patrocínio, R5)
  - `excluded: list<ExcludedOffer>` (nota: `offerId` e `reason`)
  - `presentationHash: string`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - Imutável e reconstruível: com a política (pelo hash) e o método, recalcular dá a mesma
    sequência (teste de reprodutibilidade, 07 §3.4).
  - Ofertas não comparáveis aparecem em grupos separados, nunca numa lista única (07 §2.1);
    `excluded` registra o que não foi mostrado e por quê.

### OfferSelection

- **Identidade**: `natural` por `fundingRequestId` (uma seleção vigente por pedido)
- **Campos**:
  - `fundingRequestId: id`, `offerId: id`, `presentationId: id`, `offerVersionHash: string`
  - `selectedAt: instant`, `selectedByUserId: id`, `selectedByRepresentationId: id`
  - `revalidation`: `snapshotId: id`, `eligibilityEvaluationId: id`, `consentIds: list<id>`,
    `kybCaseId: id`, `revalidatedAt: instant`, `outcome: valid | invalid`, `failedCheck: string`
    (opcional)
  - `materialConditionsDocumentHash: string`, `materialConditionsAcceptedAt: instant`
  - `status: selected | voided`, `voidReason: string` (opcional)
- **Estados**: `selected`, `voided`
- **Transições permitidas**:
  - (novo) → `selected` e `voided` → `selected` - por `SelectOffer`
  - `selected` → `voided` - por `VoidSelection`
- **Invariantes de estado**:
  - A seleção referencia a versão e o hash da oferta exibida, e só vale para oferta vigente.
  - Há uma seleção vigente por pedido: `SelectOffer` sobre `selected` rejeita com
    `marketplace/offer-selection/already-selected`.
  - A revalidação feita no momento da seleção fica guardada: sem ela a revalidação não é
    comprovável.

## Comandos (UPRs)

### DraftFundingRequest
- **Agregado**: `FundingRequest`; **cria**
- **Entrada**: `lotId: id`, `evaluationId: id`, `organizationId: id`, `snapshotId: id`,
  `assetCount: int`, `lotNetTotalMinor: int`, `currency: string`
- **Pré-condições**:
  - `assetCount > 0` → `marketplace/funding-request/lot-empty`
- **Efeitos sobre o estado**: `status ← draft`, `version ← 1`, `createdAt ← at`, demais campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `fundingRequestId: id`

### OpenFundingRequest
- **Agregado**: `FundingRequest`; exige existente
- **Entrada**: `invitationMode`, `offerWindowClosesAt: instant`, `reservationExpiresAt: instant`,
  `dataSharingConsentId: id`, `representationId: id`, mais os fatos resolvidos pelo caso de uso
  por consulta gRPC antes da UoW: `sharingConsentValid: bool` (Consent), `representationValid:
  bool` (Identity, com alçada), `kybReady: bool` (KYB), `eligibleFunderCount: int` (Partner). As
  reservas dos ativos já foram feitas no Eligibility, um comando por ativo.
- **Pré-condições**:
  - `status = draft` → `marketplace/funding-request/not-draft`
  - `offerWindowClosesAt > at` → `marketplace/funding-request/window-in-past`
  - `reservationExpiresAt >= offerWindowClosesAt` → `marketplace/funding-request/reservation-shorter-than-window`
  - `sharingConsentValid` → `marketplace/funding-request/consent-invalid`
  - `representationValid` → `marketplace/funding-request/representation-invalid`
  - `kybReady` → `marketplace/funding-request/kyb-not-ready`
  - `eligibleFunderCount > 0` → `marketplace/funding-request/no-eligible-funder`
- **Efeitos sobre o estado**: `status ← open`, `openedAt ← at`, demais campos ← entrada
- **Eventos emitidos**: `FundingRequestOpened`
- **Resposta**: `fundingRequestId: id`, `status`, `version: int`

### AdvanceFundingRequest
- **Agregado**: `FundingRequest`; exige existente
- **Entrada**: `target: collecting_offers | offers_available | offer_selected | contracting`
- **Pré-condições**:
  - a transição está na lista de Agregados (incluindo as duas reaberturas) →
    `marketplace/funding-request/transition-not-allowed`
- **Efeitos sobre o estado**: `status ← cmd.target` (alvo igual ao estado atual: aceita sem alterar)
- **Eventos emitidos**: nenhum
- **Resposta**: `fundingRequestId: id`, `status`
- **[ABERTA: 92-schemas-eventos:§4 - nenhum evento sinaliza a criação do caso de contrato que
  leva a `contracting`]**

### CloseFundingRequest
- **Agregado**: `FundingRequest`; exige existente
- **Entrada**: `reason: contracted | no_offer | expired | cancelled | eligibility_invalidated`
- **Pré-condições**:
  - `status` não terminal → `marketplace/funding-request/already-closed`
  - `contracted` só de `contracting`; `no_offer` só de `open` ou `collecting_offers`; os demais
    de qualquer não terminal → `marketplace/funding-request/reason-not-allowed`
- **Efeitos sobre o estado**: `status ←` `closed` (`contracted`), `no_offer`, `expired`,
  `cancelled` ou `eligibility_invalidated`; `closeReason ← cmd.reason`; `closedAt ← at`
- **Eventos emitidos**: `FundingRequestClosed`
- **Resposta**: `fundingRequestId: id`, `status`

### InviteFunder
- **Agregado**: `PartnerInvitation`; **cria**
- **Entrada**: `fundingRequestId: id`, `requestVersion: int`, `funderOrganizationId: id`,
  `productId: id`, `disclosureLevel: string`, `expiresAt: instant`, `funderEligible: bool`
  (os seis itens de 07 §4.2, resolvidos pelo caso de uso por consulta ao Partner, ao Consent e ao
  histórico de reprovações; o sexto **[ABERTA: 07:M12 - janela sem parâmetro]**)
- **Pré-condições**:
  - `funderEligible` → `marketplace/partner-invitation/funder-not-eligible`
  - `expiresAt > at` → `marketplace/partner-invitation/expiry-in-past`
  - convite ainda inexistente para o par → `marketplace/partner-invitation/already-invited`
- **Efeitos sobre o estado**: `status ← created`, `createdAt ← at`, demais campos ← entrada
- **Eventos emitidos**: `PartnerInvited`
- **Resposta**: `invitationKey: string`

### RecordInvitationDelivery
- **Agregado**: `PartnerInvitation`; exige existente
- **Entrada**: `stage: sent | delivered`
- **Pré-condições**:
  - `created` → `sent` ou `sent` → `delivered` → `marketplace/partner-invitation/invalid-delivery-stage`
- **Efeitos sobre o estado**: `status ← cmd.stage`; `sentAt` ou `deliveredAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `invitationKey: string`

### RecordInvitationOffered
- **Agregado**: `PartnerInvitation`; exige existente
- **Entrada**: `offerId: id`
- **Pré-condições**:
  - `status` é `sent` ou `delivered` → `marketplace/partner-invitation/not-open`
- **Efeitos sobre o estado**: `status ← offered`
- **Eventos emitidos**: nenhum (`OfferReceived` sai de `ReceiveOffer`)
- **Resposta**: `invitationKey: string`

### RecordInvitationDeclined
- **Agregado**: `PartnerInvitation`; exige existente
- **Entrada**: `declineReason: string`
- **Pré-condições**:
  - `status` é `sent` ou `delivered` → `marketplace/partner-invitation/not-open`
  - `declineReason` não vazio → `marketplace/partner-invitation/decline-reason-empty`
- **Efeitos sobre o estado**: `status ← declined`, `declineReason ← cmd`
- **Eventos emitidos**: nenhum
- **Resposta**: `invitationKey: string`

### CloseInvitation
- **Agregado**: `PartnerInvitation`; exige existente
- **Entrada**: `reason: window_ended | consent_revoked | capability_suspended | product_changed |
  request_closed`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: `status ← expired` e `closeReason ← cmd.reason` quando `created`,
  `sent` ou `delivered`; já terminal: aceita sem alterar
- **Eventos emitidos**: nenhum
- **Resposta**: `invitationKey: string`

### ReceiveOffer
- **Agregado**: `FinancingOffer`; **cria**
- **Entrada**: `fundingRequestId`, `requestVersion`, `invitationKey`, `funderOrganizationId`,
  `productId`, `funderOfferReference`, `originalPayload`, `originalPayloadHash`, `documentRef`,
  `coveredAssetIds`, `isCounterOffer`, `conditions`, `pricing` declarado, `validUntil`, mais os
  fatos do caso de uso: `requestVersionCurrent`, `requestAcceptingOffers`, `assetsInLot`,
  `capabilityEnabled` (todos `bool`)
- **Pré-condições**:
  - `originalPayload` não vazio → `marketplace/financing-offer/payload-empty`
- **Efeitos sobre o estado**: campos ← entrada; `receivedAt ← at`; `totalCostAmountMinor ←
  bruto - líquido`; `effectiveMonthlyRate ←` fórmula de 90 §3.3. `status ← received` quando todas
  as verificações de 07 §7 passam; senão `status ← rejected_as_invalid` com a primeira falha, na
  ordem `request_version_stale`, `request_closed`, `asset_not_in_request`,
  `capability_not_enabled`, `incomplete_pricing`, `pricing_inconsistent`, `valid_until_invalid`
  (`validUntil <= at`; máximo permitido **[ABERTA: 07:§7 sem valor]**)
- **Eventos emitidos**: `OfferReceived` (válida) ou `OfferRejectedAsInvalid` (inválida)
- **Resposta**: `offerId: id`, `status`, `rejectionReason` (opcional)

### ExpireOffer
- **Agregado**: `FinancingOffer`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status = received` e `validUntil <= at` → `marketplace/financing-offer/not-expirable`
- **Efeitos sobre o estado**: `status ← expired`
- **Eventos emitidos**: nenhum
- **Resposta**: `offerId: id`

### PublishRankingPolicy
- **Agregado**: `RankingPolicy`; **cria**
- **Entrada**: `policyId`, `version`, `effectiveFrom`, `criteria`, `grouping`, `tieBreakers`,
  `sponsoredEnabled`, `sponsoredMaxPositions`, `disclosureRequired`, `policyHash`
- **Pré-condições**:
  - `criteria` não vazio → `marketplace/ranking-policy/criteria-empty`
  - `sponsoredEnabled` implica `disclosureRequired` → `marketplace/ranking-policy/disclosure-required`
  - `sponsoredEnabled` implica `sponsoredMaxPositions > 0` → `marketplace/ranking-policy/sponsored-max-positions-missing`
  - `policyKey` ainda não publicada → `marketplace/ranking-policy/already-published`
- **Efeitos sobre o estado**: campos ← entrada; `status ← published`
- **Eventos emitidos**: nenhum
- **Resposta**: `policyKey: string`

### RetireRankingPolicy
- **Agregado**: `RankingPolicy`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status = published` → `marketplace/ranking-policy/already-retired`
- **Efeitos sobre o estado**: `status ← retired`
- **Eventos emitidos**: nenhum
- **Resposta**: `policyKey: string`

### PresentOffers
- **Agregado**: `OfferPresentation`; **cria**
- **Entrada**: `fundingRequestId`, `requestVersion`, `presentedToUserId`, `rankingPolicyKey`,
  `rankingPolicyHash`, `calculationMethodId`, `offers: list<OfferInput>` (as ofertas `received`
  e vigentes, com `pricing` e `receivedAt`), `sponsoredOfferIds: list<id>`, `excluded:
  list<ExcludedOffer>`, `policyPublished: bool`. O `GET .../comparable-offers` do `bff` traduz-se
  neste comando.
- **Pré-condições**:
  - `offers` não vazio → `marketplace/offer-presentation/no-offers-to-present`
  - `policyPublished` → `marketplace/offer-presentation/policy-not-published`
  - posições patrocinadas no máximo `sponsoredMaxPositions` → `marketplace/offer-presentation/sponsored-limit-exceeded`
- **Efeitos sobre o estado**: `entries` e `unsponsoredOrder` calculados pela política (grupo,
  taxa efetiva, valor líquido, validade, recebimento); `presentedAt ← at`; `presentationHash ←`
  hash do conteúdo
- **Eventos emitidos**: `OffersPresented`
- **Resposta**: `presentationId: id`, `presentationHash: string`

### SelectOffer
- **Agregado**: `OfferSelection`; **inicializa se ausente** (ou re-seleciona quando `voided`)
- **Entrada**: `fundingRequestId`, `offerId`, `presentationId`, `offerVersionHash`,
  `offerCurrentHash: string`, `offerValidUntil: instant` (os dois últimos lidos pelo caso de uso
  na oferta), `selectedByUserId`, `selectedByRepresentationId`, `materialConditionsAccepted:
  bool`, `materialConditionsDocumentHash`, `revalidation` (resultado de Eligibility, Consent,
  Identity e KYB, feito antes da UoW)
- **Pré-condições**:
  - `offerVersionHash = offerCurrentHash` → `marketplace/offer-selection/offer-changed`
  - `offerValidUntil > at` → `marketplace/offer-selection/offer-expired`
  - `materialConditionsAccepted` → `marketplace/offer-selection/material-conditions-not-accepted`
  - `revalidation.outcome = valid` → `marketplace/offer-selection/revalidation-failed` (a
    verificação reprovada acompanha a rejeição, 91 §3.4)
  - sem seleção `selected` → `marketplace/offer-selection/already-selected`
- **Efeitos sobre o estado**: `status ← selected`, `selectedAt ← at`,
  `materialConditionsAcceptedAt ← at`, demais campos ← entrada
- **Eventos emitidos**: `OfferSelected`
- **Resposta**: `selectionId: id` (igual a `fundingRequestId`), `offerId: id`
- **[ABERTA: a seleção e o avanço do pedido são duas UPRs em dois agregados do mesmo contexto,
  sem process manager no kernel]**

### VoidSelection
- **Agregado**: `OfferSelection`; exige existente
- **Entrada**: `reason: contracting_cancelled | revalidation_failed_later`
- **Pré-condições**:
  - `status = selected` → `marketplace/offer-selection/not-selected`
- **Efeitos sobre o estado**: `status ← voided`, `voidReason ← cmd.reason`
- **Eventos emitidos**: nenhum
- **Resposta**: `selectionId: id`

## Eventos de domínio

92 §3.3 define `OffersPresented.v1` e `OfferSelected.v1`. Os demais têm campos derivados dos
agregados. **[ABERTA: 92-schemas-eventos:§3 sem schema de FundingRequestOpened, PartnerInvited,
OfferReceived e FundingRequestClosed]**

### FundingRequestOpened
- **Campos**: `fundingRequestId: id`, `organizationId: id`, `lotId: id`, `evaluationId: id`,
  `assetCount: int`, `lotNetTotalMinor: int`, `currency: string`, `invitationMode`,
  `offerWindowClosesAt: instant`, `version: int`, `openedAt: instant ← at`

### PartnerInvited
- **Campos**: `fundingRequestId: id`, `requestVersion: int`, `funderOrganizationId: id`,
  `productId: id`, `disclosureLevel: string`, `expiresAt: instant`, `invitedAt: instant ← at`

### OfferReceived
- **Campos**: `offerId: id ← FinancingOffer.id`, `fundingRequestId: id`, `requestVersion: int`,
  `funderOrganizationId: id`, `productId: id`, `isCounterOffer: bool`, `coveredAssetCount: int`,
  `netAmountMinor: int`, `totalCostAmountMinor: int`, `effectiveMonthlyRate: decimal`,
  `weightedAverageTermDays: int`, `validUntil: instant`, `originalPayloadHash: string`,
  `receivedAt: instant ← at`

### OfferRejectedAsInvalid
- **Campos**: `offerId: id`, `fundingRequestId: id`, `funderOrganizationId: id`,
  `rejectionReason`, `originalPayloadHash: string`, `rejectedAt: instant ← at`. Só evento de
  domínio. **[ABERTA: 92-schemas-eventos:§4 sem OfferRejectedAsInvalid e FundingRequestExpired,
  que o PRD 07 lista; a expiração sai como FundingRequestClosed com reason expired]**

### OffersPresented
- **Campos** (92 §3.3): `fundingRequestId`, `presentationId ← OfferPresentation.id`,
  `presentedAt`, `presentedToUserId`, `rankingPolicyId`, `rankingPolicyHash`,
  `calculationMethodId`, `offers[]` (por entrada: `offerId`, `position`, `funderOrganizationId`,
  `netAmount`, `totalCostAmount`, `effectiveMonthlyRate`, `weightedAverageTermDays`, `validUntil`,
  `rankingScore`, `sponsored`, `platformRemuneration`, `originalPayloadHash`),
  `excludedOffers[]` (`offerId`, `reason`), `presentationHash`
- Acrescenta `unsponsoredOrder: list<id>`. **[ABERTA: 92-schemas-eventos:§3.3 não carrega a
  ordenação sem patrocínio que 07 §3.3 R5 exige]**

### OfferSelected
- **Campos** (92 §3.3): `fundingRequestId`, `offerId`, `selectionId`, `presentationId`,
  `selectedAt`, `selectedByUserId`, `selectedByRepresentationId`, `offerVersionHash`,
  `revalidation` (`snapshotId`, `eligibilityEvaluationId`, `consentIds`, `kybCaseId`,
  `revalidatedAt`, `outcome`), `materialConditionsAcceptance` (`documentHash`, `acceptedAt`)

### FundingRequestClosed
- **Campos**: `fundingRequestId: id`, `organizationId: id`, `lotId: id`,
  `reason ← cmd.reason`, `closedAt: instant ← at`

## Consultas

Consultas correm fora da UoW.

- `FindFundingRequest` - filtros: `fundingRequestId`; cardinalidade: `one`
- `FindFundingRequestByLot` - filtros: `lotId`; cardinalidade: `one`
- `FindFundingRequestsByEvaluation` - filtros: `evaluationId`, `status`; cardinalidade: `many`
- `FindFundingRequestsByOrganization` - filtros: `organizationId`, `status`; cardinalidade: `many`
- `FindInvitation` - filtros: `fundingRequestId`, `funderOrganizationId`; cardinalidade: `one`
- `FindInvitationsByRequest` - filtros: `fundingRequestId`, `status`; cardinalidade: `many`
- `FindInvitationsByFunder` - filtros: `funderOrganizationId`, `status`; cardinalidade: `many`
  (serve ao `GET /v1/partner/invitations` do `bff`)
- `FindOffer` - filtros: `offerId`; cardinalidade: `one`
- `FindOffersByRequest` - filtros: `fundingRequestId`, `requestVersion`, `status`; cardinalidade: `many`
- `FindPresentation` - filtros: `presentationId`; cardinalidade: `one`
- `FindLatestPresentation` - filtros: `fundingRequestId`; cardinalidade: `one`
- `ReproducePresentation` - filtros: `presentationId`; cardinalidade: `one` (recalcula a
  sequência com a política referenciada e a compara com a gravada)
- `FindOfferSelection` - filtros: `fundingRequestId`; cardinalidade: `one`
- `FindRankingPolicy` - filtros: `policyKey`; cardinalidade: `one`
- `FindEffectiveRankingPolicy` - filtros: `at`; cardinalidade: `one`

## Relações entre agregados

- `PartnerInvitation.fundingRequestId` → `FundingRequest`
- `FinancingOffer.fundingRequestId` → `FundingRequest`
- `FinancingOffer.invitationKey` → `PartnerInvitation`
- `OfferPresentation.fundingRequestId` → `FundingRequest`
- `OfferPresentation.rankingPolicyKey` → `RankingPolicy`
- `OfferPresentation.entries[].offerId` → `FinancingOffer`
- `OfferSelection.fundingRequestId` → `FundingRequest`
- `OfferSelection.offerId` → `FinancingOffer`
- `OfferSelection.presentationId` → `OfferPresentation`
- Para outros contextos, só por identidade: `FundingRequest.lotId` → `EligibleLot`
  (eligibility); `FundingRequest.organizationId` e `funderOrganizationId` → `Organization`
  (identity); `FundingRequest.dataSharingConsentId` → `ConsentGrant` (consent);
  `OfferSelection.revalidation.*` → snapshot, avaliação, consentimentos e dossiê.

## Integração

- **Publica** (integration events), em
  `apps/backend/marketplace/contract/proto/company/marketplace/event/v1/`, package
  `company.marketplace.event.v1`:
  - `FundingRequestOpened` - `funding_request_opened.proto` (Partners, Risco, Ledger)
  - `PartnerInvited` - `partner_invited.proto` (Ledger)
  - `OfferReceived` - `offer_received.proto` (Ledger)
  - `OffersPresented` - `offers_presented.proto` (Ledger)
  - `OfferSelected` - `offer_selected.proto` (Contracting, Ledger)
  - `FundingRequestClosed` - `funding_request_closed.proto` (Eligibility, Ledger)
  - `OfferRejectedAsInvalid` fica só como evento de domínio.
- **Consome** (por contrato; disposições de consumo conforme FND-04 §6.4). Um evento que alcança
  vários convites ou pedidos dispara um comando por agregado. **[ABERTA: fan-out de consumo sem
  suporte verificado no kernel]**
  - `company.eligibility.event.v1.EligibleLotCreated` → `DraftFundingRequest`. Mapa: `lotId ←
    lotId`, `evaluationId ← evaluationId`, `organizationId ← organizationId`, `snapshotId ←
    snapshotId`, `assetCount ← assetCount`, `lotNetTotalMinor ← netTotalMinor`, `currency ← currency`.
  - `company.eligibility.event.v1.EligibilityInvalidated` → `CloseFundingRequest`
    (`eligibility_invalidated`) dos pedidos de `evaluationId` (`FindFundingRequestsByEvaluation`)
    e `CloseInvitation` (`request_closed`) de cada convite deles.
  - `company.eligibility.event.v1.ReservationExpired` → `CloseFundingRequest` (`expired`) do
    pedido `holderId`, enquanto não estiver em `contracting`. Mapa: `fundingRequestId ← holderId`.
  - `company.partner.event.v1.CapabilitySuspended` → `CloseInvitation`
    (`capability_suspended`) dos convites abertos do financiador (`FindInvitationsByFunder`);
    `PartnerProductChanged` → `CloseInvitation` (`product_changed`). Payloads
    **[ABERTA: 92:§3 sem schema]**.
  - `company.consent.event.v1.ConsentRevoked` → `CloseInvitation` (`consent_revoked`) dos
    convites dos pedidos de `organizationId` cujo `dataSharingConsentId = consentId` (07 §4.2).
  - `company.contracting.event.v1.ContractSigned` → `CloseFundingRequest` (`contracted`) e
    `CloseInvitation` (`request_closed`); `ContractCancelled` antes da assinatura →
    `VoidSelection` e `AdvanceFundingRequest` (`offers_available`), reabrindo convites se a
    janela permitir. **[ABERTA: 92-schemas-eventos:§4 não lista Marketplace como consumidor de
    ContractSigned nem de ContractCancelled, mas 07 §6.1 e §6.2 dependem deles]**
- **Não subscritos**: `CapabilityEnabled` e `VerificationReady` (92 §4 lista Marketplace, mas 07
  não define reação; o estado vem por consulta gRPC), `OperationBlockedByRisk` (REVIEW/BLOCK, 15
  §5, sem consumidor declarado em 92 §4) e eventos de billing. **[ABERTA:
  92-schemas-eventos:§4]**
- **Consultas de saída (gRPC)**: Eligibility (`FindLot`, reservas), Partner (capacidade efetiva,
  produto, `CommercialRule`), Consent (escopo de compartilhamento), KYB (dossiê), Identity
  (representação e alçada) e Portfolio (snapshot na revalidação).
- O financiador recebe só os dados autorizados pelo nível de divulgação do convite; a projeção
  para o tenant do financiador é cópia sob concessão (00 §4.2), e o `organizationId` do cedente
  fica nulo no nível de triagem. **[ABERTA: requer extensão do kernel - ADR-051 DMPF]**
- Nota de inconsistência: 07 §3.3 R5 pede duas ordenações no `OffersPresented`; 92 §3.3 traz uma.

## Borda e persistência

- **Borda**: só gRPC, `company.marketplace.service.v1.MarketplaceService`, um RPC por comando e
  por consulta, atrás da cadeia de interceptors do kernel. Client API, Partner API e webhooks são
  REST do `bff` (Escopo fora).
- **Banco**: database e role `marketplace`, schema `public`, tabelas `funding_requests`,
  `partner_invitations`, `financing_offers`, `ranking_policies`, `offer_presentations` e
  `offer_selections`, com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas
  só onde uma consulta filtra:
  - `funding_requests`: `organization_id`, `lot_id`, `evaluation_id`, `status`
  - `partner_invitations`: `funding_request_id`, `funder_organization_id`, `status`
  - `financing_offers`: `funding_request_id`, `request_version`, `status`, `valid_until`
  - `ranking_policies`: `policy_id`, `effective_from`
  - `offer_presentations`: `funding_request_id`, `presented_at`
  - o resto do estado vai em `snapshot` `jsonb`, inclusive `originalPayload`
- **Instante**: inteiro de nanossegundos no domínio.

## Políticas transversais

- **Idempotência**: os comandos de criação entram pelo `bff` por `POST` com chave de idempotência
  declarada, obrigatória em criação de solicitação, envio de oferta e seleção (91 §2.2; ADR-056;
  FND-04 §7.6); os demais são idempotentes por identidade natural ou por estado de destino. O
  consumo usa inbox por `eventId` (FND-04 §6.4).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação
  (FND-04 §3.2). Ato do cedente exige representação válida e com alçada. Cada financiador vê só
  as próprias ofertas; revelar preço de concorrente é vazamento (94:T4). O acesso entre tenants
  **[ABERTA: requer extensão do kernel - ADR-051 DMPF]**.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); o Ledger
  preserva a apresentação. A reprodutibilidade da ordenação é teste de CI a cada mudança de
  política (07 §3.4).
- **Conflito de interesse**: remuneração da plataforma é exibida em valor, patrocínio altera só a
  posição e nunca as métricas (R1 a R3).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `marketplace` (`fmt-check`, `vet`, `build`, `lint`,
  `test-race` e `test-distributed` com Postgres, verificador com `--base` e
  `dmpf-context-check`).
- [ ] Cenários:

```text
DADO um lote elegível com ativos
QUANDO DraftFundingRequest chega por EligibleLotCreated
ENTÃO o pedido nasce draft com version 1 e createdAt = at, sem evento
DADO qualquer entrada
QUANDO DraftFundingRequest chega com assetCount 0
ENTÃO rejeita com marketplace/funding-request/lot-empty
DADO um pedido draft e os fatos válidos
QUANDO OpenFundingRequest chega com janela futura e reserva que cobre a janela
ENTÃO o pedido vai a open com openedAt = at e FundingRequestOpened vai à outbox
DADO um pedido já open
QUANDO OpenFundingRequest chega
ENTÃO rejeita com marketplace/funding-request/not-draft
DADO um pedido draft
QUANDO OpenFundingRequest chega com offerWindowClosesAt menor ou igual a at
ENTÃO rejeita com marketplace/funding-request/window-in-past
DADO um pedido draft
QUANDO OpenFundingRequest chega com reservationExpiresAt menor que offerWindowClosesAt
ENTÃO rejeita com marketplace/funding-request/reservation-shorter-than-window
DADO um pedido draft
QUANDO OpenFundingRequest chega com sharingConsentValid falso
ENTÃO rejeita com marketplace/funding-request/consent-invalid
DADO um pedido draft
QUANDO OpenFundingRequest chega com representationValid falso
ENTÃO rejeita com marketplace/funding-request/representation-invalid
DADO um pedido draft
QUANDO OpenFundingRequest chega com kybReady falso
ENTÃO rejeita com marketplace/funding-request/kyb-not-ready
DADO um pedido draft
QUANDO OpenFundingRequest chega com eligibleFunderCount 0
ENTÃO rejeita com marketplace/funding-request/no-eligible-funder
DADO um pedido open
QUANDO AdvanceFundingRequest chega com target collecting_offers
ENTÃO o pedido vai a collecting_offers; o mesmo alvo de novo aceita sem alterar
DADO um pedido draft
QUANDO AdvanceFundingRequest chega com target offer_selected
ENTÃO rejeita com marketplace/funding-request/transition-not-allowed
DADO um pedido contracting
QUANDO AdvanceFundingRequest chega com target offers_available
ENTÃO o pedido volta a offers_available (reabertura)
DADO um pedido contracting
QUANDO CloseFundingRequest chega com reason contracted
ENTÃO o pedido vai a closed com closedAt = at e FundingRequestClosed é emitido
DADO um pedido collecting_offers sem oferta válida
QUANDO CloseFundingRequest chega com reason no_offer
ENTÃO o pedido vai a no_offer e FundingRequestClosed é emitido com reason no_offer
DADO um pedido já terminal
QUANDO CloseFundingRequest chega
ENTÃO rejeita com marketplace/funding-request/already-closed e o estado não muda
DADO um pedido offers_available
QUANDO CloseFundingRequest chega com reason contracted
ENTÃO rejeita com marketplace/funding-request/reason-not-allowed
DADO um pedido open e um financiador elegível
QUANDO InviteFunder chega com expiresAt futuro
ENTÃO o convite nasce created e PartnerInvited vai à outbox
DADO funderEligible falso
QUANDO InviteFunder chega
ENTÃO rejeita com marketplace/partner-invitation/funder-not-eligible
DADO qualquer entrada
QUANDO InviteFunder chega com expiresAt menor ou igual a at
ENTÃO rejeita com marketplace/partner-invitation/expiry-in-past
DADO um convite já criado para o par pedido e financiador
QUANDO InviteFunder chega de novo
ENTÃO rejeita com marketplace/partner-invitation/already-invited
DADO um convite created
QUANDO RecordInvitationDelivery chega com stage sent e depois com stage delivered
ENTÃO o convite passa por sent e delivered com sentAt e deliveredAt gravados
DADO um convite created
QUANDO RecordInvitationDelivery chega com stage delivered
ENTÃO rejeita com marketplace/partner-invitation/invalid-delivery-stage
DADO um convite delivered
QUANDO RecordInvitationOffered chega
ENTÃO o convite vai a offered
DADO um convite created ou terminal
QUANDO RecordInvitationOffered ou RecordInvitationDeclined chega
ENTÃO rejeita com marketplace/partner-invitation/not-open
DADO um convite delivered
QUANDO RecordInvitationDeclined chega com declineReason
ENTÃO o convite vai a declined, distinto de expired
DADO um convite delivered
QUANDO RecordInvitationDeclined chega com declineReason vazio
ENTÃO rejeita com marketplace/partner-invitation/decline-reason-empty
DADO um convite delivered sem resposta
QUANDO CloseInvitation chega com reason window_ended
ENTÃO o convite vai a expired com closeReason; um segundo CloseInvitation aceita sem alterar
DADO um pedido open e uma oferta aritmeticamente consistente dentro da validade
QUANDO ReceiveOffer chega com os fatos do pedido verdadeiros
ENTÃO a oferta nasce received, com effectiveMonthlyRate calculado e originalPayload intacto, e OfferReceived é emitido
DADO uma oferta cujo netAmountMinor não fecha com bruto, deságio, tarifas e tributos
QUANDO ReceiveOffer chega
ENTÃO a oferta é gravada rejected_as_invalid com pricing_inconsistent e OfferRejectedAsInvalid é emitido
DADO uma oferta para uma versão antiga do pedido
QUANDO ReceiveOffer chega com requestVersionCurrent falso
ENTÃO a oferta é gravada rejected_as_invalid com request_version_stale
DADO uma oferta com validUntil menor ou igual a at
QUANDO ReceiveOffer chega
ENTÃO a oferta é gravada rejected_as_invalid com valid_until_invalid
DADO uma oferta de ativo fora do lote, de capacidade suspensa ou com preço incompleto
QUANDO ReceiveOffer chega
ENTÃO a oferta é gravada rejected_as_invalid com asset_not_in_request, capability_not_enabled ou incomplete_pricing
DADO qualquer entrada
QUANDO ReceiveOffer chega com originalPayload vazio
ENTÃO rejeita com marketplace/financing-offer/payload-empty
DADO uma oferta received com validUntil menor ou igual a at
QUANDO ExpireOffer chega
ENTÃO a oferta vai a expired
DADO uma oferta received com validUntil no futuro
QUANDO ExpireOffer chega
ENTÃO rejeita com marketplace/financing-offer/not-expirable
DADO uma política nova sem patrocínio
QUANDO PublishRankingPolicy chega com criteria
ENTÃO a política nasce published com policyHash
DADO qualquer entrada
QUANDO PublishRankingPolicy chega com criteria vazio
ENTÃO rejeita com marketplace/ranking-policy/criteria-empty
DADO qualquer entrada
QUANDO PublishRankingPolicy chega com sponsoredEnabled e disclosureRequired falso
ENTÃO rejeita com marketplace/ranking-policy/disclosure-required
DADO qualquer entrada
QUANDO PublishRankingPolicy chega com sponsoredEnabled e sponsoredMaxPositions 0
ENTÃO rejeita com marketplace/ranking-policy/sponsored-max-positions-missing
DADO uma policyKey já publicada
QUANDO PublishRankingPolicy chega
ENTÃO rejeita com marketplace/ranking-policy/already-published
DADO uma política published
QUANDO RetireRankingPolicy chega
ENTÃO a política vai a retired; uma segunda chamada rejeita com marketplace/ranking-policy/already-retired
DADO duas ofertas received de grupos diferentes e uma política published
QUANDO PresentOffers chega
ENTÃO a apresentação nasce com posições por grupo, unsponsoredOrder e presentationHash, e OffersPresented é emitido
DADO nenhuma oferta vigente
QUANDO PresentOffers chega com offers vazio
ENTÃO rejeita com marketplace/offer-presentation/no-offers-to-present
DADO policyPublished falso
QUANDO PresentOffers chega
ENTÃO rejeita com marketplace/offer-presentation/policy-not-published
DADO mais ofertas patrocinadas que sponsoredMaxPositions
QUANDO PresentOffers chega
ENTÃO rejeita com marketplace/offer-presentation/sponsored-limit-exceeded
DADO uma apresentação gravada
QUANDO a consulta ReproducePresentation roda sobre ela
ENTÃO a sequência recalculada com a política referenciada é igual à gravada, fora de qualquer UoW
DADO uma oferta vigente apresentada, condições aceitas e revalidação valid
QUANDO SelectOffer chega com o hash exibido igual ao atual
ENTÃO a seleção nasce selected com selectedAt = at e OfferSelected vai à outbox com a revalidação
DADO o hash exibido diferente do hash atual da oferta
QUANDO SelectOffer chega
ENTÃO rejeita com marketplace/offer-selection/offer-changed
DADO uma oferta com offerValidUntil menor ou igual a at
QUANDO SelectOffer chega
ENTÃO rejeita com marketplace/offer-selection/offer-expired
DADO materialConditionsAccepted falso
QUANDO SelectOffer chega
ENTÃO rejeita com marketplace/offer-selection/material-conditions-not-accepted
DADO uma revalidação com outcome invalid
QUANDO SelectOffer chega
ENTÃO rejeita com marketplace/offer-selection/revalidation-failed e nada é emitido
DADO uma seleção já selected
QUANDO SelectOffer chega
ENTÃO rejeita com marketplace/offer-selection/already-selected
DADO uma seleção voided
QUANDO SelectOffer chega com nova oferta válida
ENTÃO a seleção volta a selected
DADO uma seleção selected
QUANDO VoidSelection chega com reason contracting_cancelled
ENTÃO a seleção vai a voided
DADO uma seleção voided
QUANDO VoidSelection chega
ENTÃO rejeita com marketplace/offer-selection/not-selected
```

## Escopo fora

- **REST do bff**: a Client API (`POST /v1/funding-requests`, `GET .../comparable-offers`,
  `POST .../selection`) e a Partner API (`GET /v1/partner/invitations`,
  `POST /v1/partner/funding-requests/{id}/offers`) ficam no `bff`, com erros HTTP mapeados dos
  códigos de rejeição (referência: `docs/design/91-contratos-api.md` §3 e §4). O webhook de
  oferta e os webhooks enviados ao cedente também são do `bff`.
- **Crédito, preço e recomendação**: a plataforma não aprova crédito, não estima preço, não
  calcula probabilidade de aprovação e não valida razoabilidade de preço.
- **Negociação iterativa**: contraproposta é oferta nova sobre subconjunto, sem troca de
  mensagens entre cedente e financiador (07 §5).
- **Reserva dos ativos**: é do Eligibility; aqui só se guarda `reservationExpiresAt`.
- **Contratação, assinatura e efeito**: de Contracting e Effects; o Marketplace entrega o
  `OfferSelected`.
- **Remuneração, patrocínio e cobrança**: dependem de #7 e de resumo:D7; com um financiador não
  existem. A inversão de mérito por patrocínio só entra depois da decisão.
- **Distribuição `waves`**: o valor existe no modelo, mas a agenda de ondas fica fora até a
  decisão de produto (07 §4.1).
- **CET** (A15), **formato do documento de oferta** (07:M13) e **janelas de supressão por
  reprovação** (07:M12): fora até apuração ou parâmetro.
- **Notificação ao cedente e ao financiador não selecionado**: de Notificações; o texto não
  revela condições da oferta vencedora (07 §6.2).
- **Gatilho temporal** de `ExpireOffer`, `CloseInvitation` por fim de janela e `CloseFundingRequest`
  por `no_offer`: **[ABERTA: sem agendador verificado no kernel]**.
