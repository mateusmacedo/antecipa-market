---
id: SPEC-GA4FYX4Z
slug: contracting
title: Bounded context — contracting
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-CDA3T01R]
ticket_url: https://linear.app/mmda/issue/DEVS-89/spec-ga4fyx4z-bounded-context-contracting
subtask_urls: []
created: 2026-10-12
---

# SPEC-GA4FYX4Z: Bounded context — contracting

## Identidade

- **bounded_context**: `contracting`
- **name**: `contracting` (módulo `apps/backend/contracting`, package Protobuf `company.contracting.event.v1`)
- **Linguagem ubíqua**:
  - *Caso de contrato* (`ContractCase`) - processo de formalização de uma oferta selecionada.
  - *Instrumento* (`ContractInstrument`) - documento contratual versionado; cada versão é imutável.
  - *Parte* (`ContractParty`) - cedente, financiador ou garantidor do contrato.
  - *Cronograma de ativos* (`AssetSchedule`) - os ativos (ou a regra de alcance) a que o contrato se refere.
  - *Envelope de assinatura* (`SignatureEnvelope`) - uma rodada de assinatura: signatários, ordem e evidências.
  - *Condições materiais* - as 13 condições de 08 §3 cujo aceite explícito precede a assinatura; o aceite é um ato distinto da assinatura.
  - *Divergência bloqueante* (`MISMATCH`) - diferença entre oferta e instrumento com tolerância zero (08 §4).
  - *Alçada* - limite de valor ou de tipo de ato dos poderes de um representante (90 §3.1).
  - *Dossiê* (`ContractEvidenceBundle`) - pacote final e reproduzível de evidências, por referência e hash.
  - *Cláusula de confirmação parcial* - previsão contratual do caminho quando o efeito é confirmado só em parte.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/08-prd-contracting.md`; `docs/design/08-design-contracting.md` §2 (quem produz o instrumento), §3 (condições materiais), §4 (correspondência), §5 (assinatura, ordem e alçada), §6 (recebíveis futuros), §7 (dossiê); `docs/design/90-modelo-de-dados.md` §3.1 (`Representation.scope`) e §4; `docs/design/92-schemas-eventos.md` §3.3 e §4; `docs/prd/14-catalogo-contratos-eventos.md` (sagas); `docs/design/09-design-effects-orchestrator.md` §5 e §7 (confirmação parcial e invalidação após a assinatura).
- **Situação**: `parcial` - o modelo, a correspondência entre oferta e instrumento, a alçada e o dossiê são construíveis; o nível de assinatura (#9), quem produz o instrumento (08:T10) e o provedor de assinatura travam o fluxo de assinatura e a geração do documento.
- **Fase de entrega**: 1.
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-CDA3T01R`, Marketplace, fonte de `OfferSelected`); a cadeia transitiva fica implícita (identity, partner, gateway, consent, portfolio, eligibility). Upstream duro: Marketplace (`OfferSelected`), Eligibility (`EligibilityInvalidated`), Identity (`RepresentationVerified`, `RepresentationRevoked`), Portfolio (revalidação) e Partner Management (modelo do instrumento em `PartnerAgreement`, 08 §2). Upstream brando: Effects (`EffectRejected`, `EffectPartiallyConfirmed`), KYB e Consent (revalidação no dossiê, 08 §7). As arestas de retroalimentação (Effects) não entram em `depends_on`.
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
|---|---|---|
| #1 (A1) | autorização do BCB; premissa de não autorização (transversal, existencial) | o contexto inteiro |
| #9 (A8, C4; 08:T9, T12) | nível de assinatura eletrônica e provedor não escolhido | `SignatureEnvelope.signatureLevel`, `RequestSignature`, `RecordSignature` (evidências por nível), adaptador do provedor |
| A3 (08:T10) | responsabilidade da plataforma ao preencher o instrumento a partir do modelo do financiador | `ContractInstrument.mode`, `PrepareInstrument` no modo `platform_filled` |
| 08:T11 | textos contratuais | conteúdo do instrumento e do documento de condições materiais |
| A10 (08:T5; 90:L3) | alçada em procuração: como tratar assinatura acima dos poderes | `SigningAuthority`, rejeições de alçada em `RecordSignature` |
| A9 | elementos de evidência para oponibilidade em juízo | composição de `ContractEvidenceBundle` |
| A11 (08:T6; 09:F9) | cláusula de confirmação parcial: ajuste proporcional ou revisão contratual | `partialConfirmationClause`, `RecordEffectShortfall` |
| D6, B11 (08:T13) | estrutura de contrato sobre recebíveis futuros | `AssetSchedule` com `scheduleMode = rule_based` |

- **Decisões de produto abertas**: quem produz o instrumento (08 §2, recomendação híbrido; `docs/README.md` "Decisões abertas"); nível de assinatura por produto (08 §5.1, postura recomendada: atributo configurável); caminho padrão da confirmação parcial (09 §5.2, proposta: revisão contratual); recebíveis futuros no MVP (D6, `docs/resumo-executivo-apuracao.md`; a estratégia de entrega os põe na fase 4).
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída; `SPEC-CDA3T01R` com o contrato de `OfferSelected` publicado (hoje o evento não carrega ativos, preço nem financiador - ver Integração); e, do que trava o núcleo, A3 (08:T10) decidido ao menos como o modelo híbrido com `funder_supplied` de contingência, porque define o fluxo de `PrepareInstrument`. O nível de assinatura (#9) pode seguir como atributo configurável (08 §5.1) e o provedor atrás de uma porta, então não impede `planning`; impede a homologação do fluxo real de assinatura.

## Agregados

`ContractParty` e `SignatureEnvelope` são entidades internas de `ContractCase` (sem identidade nem ciclo de vida fora dele); `SignatureEnvelope` guarda só a rodada corrente, as anteriores ficam na trilha de auditoria e no Ledger. `SigningAuthority` não consta do PRD: é a projeção local que permite verificar o poder no instante da assinatura (08 §5.3) sem consulta síncrona. Instantes `*At` valem 0 enquanto o fato não ocorreu; datas puras são `string` no formato `AAAA-MM-DD`; dinheiro é `*Minor: int` (centavos) com `currency: string`.

### ContractCase

- **Identidade**: `natural` por `selectionId` (1:1 com a seleção, 90 §4; o `contractId` publicado é esse valor). Diverge do design: 90 §1.1 prevê id opaco gerado com prefixo `ctr_`; a identidade natural torna a abertura idempotente por seleção e deixa o bff devolver o id na resposta de seleção (91 §3.4).
- **Campos**:
  - Origem: `selectionId: id`, `offerId: id`, `fundingRequestId: id`, `presentationId: id`, `eligibilityEvaluationId: id`, `snapshotId: id`, `kybCaseId: id`, `consentIds: list<id>` (N consentimentos revalidados, indispensável ao dossiê).
  - Partes: `cedentOrganizationId: id`, `funderOrganizationId: id`, `productId: id`, `parties: list<ContractParty>` (PRD: cedente, financiador e garantidor; coleção indispensável), com `ContractParty` = `role: assignor | funder | guarantor`, `organizationId: id`.
  - Termos da oferta selecionada (base da correspondência, 08 §4): `offerVersionHash: string`, `currency: string`, `grossAmountMinor: int`, `netAmountMinor: int`, `totalCostMinor: int`, `effectiveMonthlyRate: decimal`, `weightedAverageTermDays: int`, `expectedCreditDate: string`, `assetSetHash: string`, `assetCount: int`, `scheduleMode: enumerated | rule_based`, `natureOfOperation: string`, `hasRecourse: bool`.
  - Instrumento e aceite: `currentInstrumentVersion: int` (0 sem instrumento), `presentedDocumentHash: string`, `presentedConditionsHash: string`, `signedDocumentHash: string`, `partialConfirmationClause: absent | proportional_adjustment | all_or_nothing`, `selectionAcceptanceHash: string`, `selectionAcceptedAt: instant` (aceite registrado pelo Marketplace na seleção), `materialConditionsAcceptanceHash: string`, `materialConditionsAcceptedVersion: int`, `materialConditionsAcceptedAt: instant`, `materialConditionsAcceptedBy: id`.
  - Fluxo: `status`, `mismatchFields: list<string>` (campos da divergência bloqueante vigente), `eligibilityInvalidated: bool`, `effectOutcome: none | rejected | partially_confirmed`, `evidenceBundleHash: string`, `closeReason: string`, `expiresAt: instant`, `createdAt`, `validatedAt`, `signedAt`, `submittedAt`, `closedAt: instant`.
  - `envelope: SignatureEnvelope`: `round: int`, `signatureLevel: string` [A CONFIRMAR: #9, A8], `signingOrder: list<string>` (papéis, na ordem; padrão cedente antes do financiador, 08 §5.3), `providerRef: string`, `expiresAt: instant`, `status: none | requested | partially_signed | completed | refused | expired | invalidated`, `signatories: list<Signatory>` com `Signatory` = `role: assignor | funder | guarantor`, `representationId: id`, `status: pending | signed | refused`, `signedAt: instant`, `authenticationMethod: string`, `signedDocumentHash: string`, `evidenceId: id`, `evidenceHash: string` (o conteúdo da evidência, 08 §5.2, vive no armazenamento [ABERTA: provider de object storage]; aqui só referência e hash).
- **Estados**: `preparing`, `pending_validation`, `ready_for_signature`, `signing`, `signed`, `submitted_for_effect`, `mismatch`, `expired`, `rejected`, `cancelled`, `signature_failed`.
- **Transições permitidas**:
  - (novo) → `preparing` - por `OpenContractCase`
  - `preparing`, `mismatch`, `ready_for_signature`, `signing`, `signature_failed` → `pending_validation` - por `SubmitInstrumentForValidation` (nova versão do instrumento; invalida as assinaturas coletadas)
  - `pending_validation` → `ready_for_signature` - por `ValidateContract`
  - `pending_validation` → `mismatch` - por `ValidateContract` (divergência bloqueante)
  - `ready_for_signature`, `signature_failed` → `signing` - por `RequestSignature`
  - `signing` → `signed` - por `RecordSignature` (última assinatura)
  - `signing` → `rejected` - por `RecordSignatureRefusal`
  - `signing` → `signature_failed` - por `RecordSigningFailure`
  - `signed` → `submitted_for_effect` - por `SubmitForEffect`
  - `preparing`, `pending_validation`, `mismatch`, `ready_for_signature`, `signing`, `signature_failed`, `signed` → `cancelled` - por `CancelContract`; `submitted_for_effect` → `cancelled` - por `CancelContract` ou `RecordEffectShortfall`, só com `effectOutcome` diferente de `none` [ABERTA: 08 não define cancelamento após a submissão; derivado de 09 §5.1 e 14 "Sagas principais"]
  - `preparing`, `pending_validation`, `mismatch`, `ready_for_signature`, `signing`, `signature_failed` → `expired` - por `ExpireContractCase` [ABERTA: 08 não define o prazo do caso; o PRD lista o estado `EXPIRED`]
- **Invariantes de estado**:
  - Só uma oferta válida abre caso - `OpenContractCase` rejeita com `contracting/contract-case/offer-not-validated`.
  - Valores, taxa, ativos e partes do instrumento coincidem com a oferta, tolerância zero; qualquer diferença leva a `mismatch` e exige nova apresentação e novo aceite, nunca correção silenciosa (08 §4).
  - A assinatura só conclui com poder vigente e dentro da alçada no instante dela (08 §5.3, §5.4) - `contracting/contract-case/power-not-in-force`.
  - Documento assinado não é sobrescrito: após `signed`, `signedDocumentHash` não muda e nova versão de instrumento é recusada (`not-open-for-instrument`).
  - Só um caso `submitted_for_effect` com dossiê selado alcança o Effects - `contracting/contract-case/evidence-bundle-not-sealed`.
  - `eligibilityInvalidated = true` bloqueia validação, assinatura e submissão (`contracting/contract-case/eligibility-invalidated`).

### ContractInstrument

- **Identidade**: `natural` por `instrumentKey` (composta de `contractId` e `version`, no formato `<contractId>:<version>`).
- **Campos**: `contractId: id`, `version: int`, `mode: funder_supplied | platform_filled` [ABERTA: 08:T10, A3], `templateRef: string` (modelo homologado em `PartnerAgreement` [ABERTA: 13 não define o modelo do instrumento]), `documentHash: string`, `documentEvidenceId: id`, `conditionsDocumentHash: string`, e os termos escritos no documento: `currency: string`, `grossAmountMinor: int`, `netAmountMinor: int`, `effectiveMonthlyRate: decimal`, `assetSetHash: string`, `partyOrganizationIds: list<id>`, `expectedCreditDate: string`, `partialConfirmationClause: absent | proportional_adjustment | all_or_nothing`, `preparedAt: instant`.
- **Estados**: nenhum. Cada versão é imutável; "substituída" é derivado de `ContractCase.currentInstrumentVersion` maior que `version`.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: `documentHash` nunca é vazio - `contracting/contract-instrument/document-hash-empty`; uma versão não é regravada - `contracting/contract-instrument/version-already-prepared`; no modo `platform_filled` há `templateRef` - `contracting/contract-instrument/template-missing`.

### AssetSchedule

- **Identidade**: `natural` por `contractId`.
- **Campos**: `contractId: id`, `scheduleMode: enumerated | rule_based`, `assetIds: list<id>` (modo `enumerated`; milhares de itens, 92 §3.4 cita 1284 alvos), `assetSetHash: string`, `assetCount: int`, `ruleAssetTypes: list<string>`, `rulePeriodFrom: string`, `rulePeriodUntil: string`, `ruleCeilingAmountMinor: int`, `disbursementMode: single_advance | successive` (modo `rule_based`, 08 §6) [A CONFIRMAR: B11, D6], `definedAt: instant`.
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma; o cronograma é definido uma vez por caso.
- **Invariantes de estado**: `enumerated` tem ao menos um ativo - `contracting/asset-schedule/schedule-empty`; `rule_based` só com suporte declarado - `contracting/asset-schedule/rule-based-unsupported`.

### SigningAuthority

- **Identidade**: `natural` por `representationId`.
- **Campos**: `representationId: id`, `organizationId: id`, `personId: id`, `powerType: legal_representative | attorney_in_fact | operator`, `validFrom: instant`, `validUntil: instant` (0 = indeterminada), `hasAmountLimit: bool`, `maxAmountMinor: int`, `actTypes: list<string>` (vazia = sem restrição), `status: verified | revoked`, `verifiedAt: instant`, `revokedAt: instant`, `revocationReason: string`.
- **Estados**: `verified`, `revoked`.
- **Transições permitidas**: (novo) → `verified` - por `RegisterSigningAuthority`; `verified` → `revoked` - por `RevokeSigningAuthority`. A `Representation` é imutável (90 §3.1): mudança de poderes revoga e cria outra.
- **Invariantes de estado**: registro exige `representationId`, `organizationId` e `powerType` - `contracting/signing-authority/representation-incomplete`; revogação de representação desconhecida rejeita com `contracting/signing-authority/unknown-representation`.

### ContractEvidenceBundle

- **Identidade**: `natural` por `contractId`.
- **Campos**: `contractId: id`, `items: list<EvidenceItem>` com `EvidenceItem` = `kind: funding_request | offers_presentation | selected_offer | material_conditions_acceptance | revalidation_result | portfolio_snapshot | representations | presented_instrument | signed_instrument | signature_evidence`, `referenceId: id`, `contentHash: string`, `origin: string` (08 §7; coleção indispensável), `bundleHash: string`, `sealedAt: instant`.
- **Estados**: nenhum; selado na criação e imutável.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: o dossiê contém todos os tipos exigidos por 08 §7 do lado do Contracting - `contracting/contract-evidence-bundle/incomplete`; todo item tem hash - `contracting/contract-evidence-bundle/item-without-hash`. O item "comando de efeito e confirmação" é do Effects; a composição final do dossiê reproduzível fica no Compliance Ledger [ABERTA: 08 §7 × 12, fronteira entre os dois dossiês].

## Comandos (UPRs)

O disparo vem de evento consumido, de chamada gRPC (portal, provedor de assinatura pelo `bff`, backoffice) ou de processo do contexto. Os fatos de outros agregados e contextos (instrumento, cronograma, poder, revalidação) são lidos pelo caso de uso, fora da UoW, e entram como entrada do comando [ABERTA: encadeamento intra-contexto e composição de fatos de um segundo agregado não normatizados no kernel]. A elegibilidade invalidada bloqueia o avanço em quatro comandos (`eligibility-invalidated`).

### OpenContractCase

- **Agregado**: `ContractCase`; **inicializa se ausente**
- **Disparo**: consumo de `OfferSelected`
- **Entrada**: `selectionId: id`, `offerId: id`, `fundingRequestId: id`, `presentationId: id`, `eligibilityEvaluationId: id`, `snapshotId: id`, `kybCaseId: id`, `consentIds: list<id>`, `cedentOrganizationId: id`, `funderOrganizationId: id`, `productId: id`, `offerVersionHash: string`, `currency: string`, `grossAmountMinor: int`, `netAmountMinor: int`, `totalCostMinor: int`, `effectiveMonthlyRate: decimal`, `weightedAverageTermDays: int`, `expectedCreditDate: string`, `assetSetHash: string`, `assetCount: int`, `scheduleMode`, `natureOfOperation: string`, `hasRecourse: bool`, `selectionAcceptanceHash: string`, `selectionAcceptedAt: instant`, `revalidationOutcome: valid | invalid`, `expiresAt: instant`
- **Pré-condições**:
  - `revalidationOutcome = valid` → `contracting/contract-case/offer-not-validated`
  - `grossAmountMinor > 0` e `grossAmountMinor - netAmountMinor = totalCostMinor` (90 §3.3) → `contracting/contract-case/inconsistent-offer-terms`
  - `assetCount ≥ 1` → `contracting/contract-case/no-assets`
- **Efeitos sobre o estado**: quando ausente: `status ← preparing`; campos `← cmd`; `parties ←` cedente (`assignor`) e financiador (`funder`); `currentInstrumentVersion ← 0`; `createdAt ← at`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: `ContractCaseCreated` (só quando inicializa)
- **Resposta**: `contractId: id` (igual a `selectionId`)

### SubmitInstrumentForValidation

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: processo do contexto, após `PrepareInstrument` (nova versão do instrumento)
- **Entrada**: `instrumentVersion: int`, `presentedDocumentHash: string`, `presentedConditionsHash: string`, `scheduleDefined: bool`
- **Pré-condições**:
  - `status` em `preparing`, `mismatch`, `ready_for_signature`, `signing` ou `signature_failed` (após `signed` o instrumento não muda) → `contracting/contract-case/not-open-for-instrument`
  - `instrumentVersion = currentInstrumentVersion + 1` → `contracting/contract-case/instrument-version-out-of-order`
  - `scheduleDefined` → `contracting/contract-case/schedule-not-defined`
  - `eligibilityInvalidated` falso → `contracting/contract-case/eligibility-invalidated`
- **Efeitos sobre o estado**: `currentInstrumentVersion` e os hashes `← cmd`; com envelope `requested` ou `partially_signed`: `envelope.status ← invalidated` e as assinaturas coletadas deixam de valer (08 §5.3); `materialConditionsAcceptance* ← zero` (nova apresentação exige novo aceite, 08 §4); `mismatchFields ← []`; `status ← pending_validation`
- **Eventos emitidos**: nenhum (trilha de auditoria do kernel)
- **Resposta**: `contractId: id`, `status`

### ValidateContract

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: processo do contexto, após `SubmitInstrumentForValidation`
- **Entrada**: `instrumentVersion: int`; termos do instrumento (fatos lidos de `ContractInstrument`): `grossAmountMinor`, `netAmountMinor`, `effectiveMonthlyRate`, `assetSetHash`, `partyOrganizationIds: list<id>`, `expectedCreditDate`, `partialConfirmationClause`; `creditDateDeviationBusinessDays: int`, `creditDateToleranceBusinessDays: int` [ABERTA: 08 §4 "tolerância em dias úteis" sem valor]; `revalidation: list<RevalidationCheck>` com `RevalidationCheck` = `check: asset_availability | consent_active | representation_valid | kyb_status | eligibility_valid`, `result: pass | fail`, `referenceId: id`, `checkedAt: instant`
- **Pré-condições**:
  - `status = pending_validation` → `contracting/contract-case/not-pending-validation`
  - `instrumentVersion = currentInstrumentVersion` → `contracting/contract-case/stale-instrument-version`
  - todos os `revalidation` em `pass` → `contracting/contract-case/revalidation-failed`
  - `eligibilityInvalidated` falso → `contracting/contract-case/eligibility-invalidated`
- **Efeitos sobre o estado**: compara bruto, líquido, taxa, conjunto de ativos e partes com os termos da oferta, tolerância zero (08 §4; o prazo médio é recalculado, não comparado). Qualquer diferença: `mismatchFields ←` campos divergentes, `status ← mismatch`. Sem diferença: `status ← ready_for_signature`, `partialConfirmationClause ← cmd`, `validatedAt ← at`. Desvio de data acima da tolerância não bloqueia: marca `creditDateAlert` no evento
- **Eventos emitidos**: `ContractMismatchDetected` (divergência) ou `ContractValidated`
- **Resposta**: `contractId: id`, `status`, `mismatchFields: list<string>`

### AcceptMaterialConditions

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: chamada gRPC (portal do cedente); ato distinto da assinatura (08 §3)
- **Entrada**: `instrumentVersion: int`, `conditionsDocumentHash: string`, `acceptedBy: id` (representação), `authenticationMethod: string`
- **Pré-condições**:
  - `status = ready_for_signature` → `contracting/contract-case/not-ready-for-acceptance`
  - `instrumentVersion = currentInstrumentVersion` → `contracting/contract-case/stale-instrument-version`
  - `conditionsDocumentHash = presentedConditionsHash` → `contracting/contract-case/conditions-hash-mismatch`
- **Efeitos sobre o estado**: `materialConditionsAcceptanceHash ← conditionsDocumentHash`, `materialConditionsAcceptedVersion ← instrumentVersion`, `materialConditionsAcceptedBy ← acceptedBy`, `materialConditionsAcceptedAt ← at`. Já aceito para a versão: aceita sem alterar [ABERTA: 08 §3 × 92 §3.3 - o aceite aparece no Marketplace (`OfferSelected`) e aqui (dossiê); esta spec guarda os dois]
- **Eventos emitidos**: nenhum (trilha de auditoria; o hash segue em `SignatureRequested`)
- **Resposta**: `contractId: id`

### RequestSignature

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: chamada gRPC (portal); chave de idempotência obrigatória na borda (91 §2.2, submissão para assinatura). O envelope é criado antes no provedor [ABERTA: sem porta canônica no Gateway para provedor de assinatura (04 §3); ciclo de comando externo sem suporte no kernel]. Provedor indisponível mantém o caso em `ready_for_signature` e comunica (93 §4.3), sem comando
- **Entrada**: `instrumentVersion: int`, `signatureLevel: string` [A CONFIRMAR: #9, A8], `signingOrder: list<string>`, `signatories: list<role, representationId>`, `providerRef: string`, `expiresAt: instant`
- **Pré-condições**:
  - `status` em `ready_for_signature` ou `signature_failed` → `contracting/contract-case/not-ready-for-signature`
  - `instrumentVersion = currentInstrumentVersion` → `contracting/contract-case/stale-instrument-version`
  - aceite das condições para essa versão → `contracting/contract-case/material-conditions-not-accepted`
  - signatários incluem `assignor` e `funder` → `contracting/contract-case/signatories-incomplete`
  - `eligibilityInvalidated` falso → `contracting/contract-case/eligibility-invalidated`
- **Efeitos sobre o estado**: `envelope ←` nova rodada (`round + 1`, `status ← requested`, signatários `pending`); `status ← signing`
- **Eventos emitidos**: `SignatureRequested`
- **Resposta**: `contractId: id`, `round: int`

### RecordSignature

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: chamada gRPC, vinda do webhook do provedor pelo `bff`, uma por assinatura
- **Entrada**: `round: int`, `representationId: id`, `signedAt: instant`, `signedDocumentHash: string`, `authenticationMethod: string`, `originAddress: string`, `deviceId: string`, `trustedTimestampRef: string`, `evidenceId: id`, `evidenceHash: string`, `actType: string`, `availabilityRevalidation: not_required | passed | failed`; fatos de `SigningAuthority`: `authorityStatus: verified | revoked | unknown`, `powerTypeCompatible: bool`, `validFrom`, `validUntil`, `hasAmountLimit`, `maxAmountMinor`, `authorityActTypes: list<string>` [ABERTA: matriz powerType × ato (08 §5.4) e verificação por consulta ao Identity no instante, dado o atraso de 60 s de 94 C3.4]
- **Pré-condições**:
  - `status = signing` e `round = envelope.round` com envelope aberto → `contracting/contract-case/no-open-signing-round`
  - o signatário está no envelope e pendente → `contracting/contract-case/signatory-not-expected`
  - é a vez dele na ordem (padrão cedente, depois financiador) → `contracting/contract-case/out-of-signing-order`
  - representação `verified` e `signedAt` dentro de `validFrom` e `validUntil` → `contracting/contract-case/power-not-in-force`
  - `powerTypeCompatible` → `contracting/contract-case/power-type-incompatible`
  - sem limite, ou `grossAmountMinor ≤ maxAmountMinor` (a rejeição informa o limite e o valor da operação, 08 §5.4) → `contracting/contract-case/amount-exceeds-authority`
  - `authorityActTypes` vazia ou contém `actType` → `contracting/contract-case/act-type-not-authorized`
  - hash assinado, método, `trustedTimestampRef` e evidência presentes (08 §5.2) → `contracting/contract-case/evidence-incomplete`
  - na última assinatura, `availabilityRevalidation ≠ failed` → `contracting/contract-case/assets-unavailable`
- **Efeitos sobre o estado**: signatário já `signed`: aceita sem alterar. Senão `signatory.status ← signed` com as evidências; `envelope.status ← partially_signed`; com todos assinados: `envelope.status ← completed`, `status ← signed`, `signedDocumentHash ← cmd`, `signedAt ← at`
- **Eventos emitidos**: `ContractSigned` (só na última assinatura)
- **Resposta**: `contractId: id`, `status`, `pendingSignatories: int`

### RecordSignatureRefusal

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: chamada gRPC, vinda do webhook do provedor pelo `bff`
- **Entrada**: `round: int`, `representationId: id`, `reason: string`
- **Pré-condições**:
  - rodada aberta → `contracting/contract-case/no-open-signing-round`
  - signatário esperado e pendente → `contracting/contract-case/signatory-not-expected`
  - `reason` não vazio (a recusa encerra o caso com motivo, 08 §5.3) → `contracting/contract-case/reason-missing`
- **Efeitos sobre o estado**: `signatory.status ← refused`; `envelope.status ← refused`; `status ← rejected`; `closeReason ← reason`; `closedAt ← at`
- **Eventos emitidos**: `ContractCancelled` (com `terminalState = rejected`)
- **Resposta**: `contractId: id`

### RecordSigningFailure

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: chamada gRPC, vinda do provedor pelo `bff`; processo do contexto na expiração do envelope
- **Entrada**: `round: int`, `cause: envelope_expired | signature_invalid | provider_reported_failure`
- **Pré-condições**: rodada aberta → `contracting/contract-case/no-open-signing-round`
- **Efeitos sobre o estado**: `envelope.status ← expired` (expiração) ou `invalidated`; `status ← signature_failed`. A expiração invalida o envelope, não o contrato (08 §5.3): nova `RequestSignature` é possível
- **Eventos emitidos**: nenhum [ABERTA: sem evento para `SIGNATURE_FAILED` no PRD nem em 92; Ops e Notificações não são avisados]
- **Resposta**: `contractId: id`

### SubmitForEffect

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: processo do contexto, após `SealEvidenceBundle`
- **Entrada**: `evidenceBundleHash: string` (fato lido de `ContractEvidenceBundle`)
- **Pré-condições**:
  - `status = signed` com `signedDocumentHash` preenchido → `contracting/contract-case/not-signed`
  - `evidenceBundleHash` não vazio → `contracting/contract-case/evidence-bundle-not-sealed`
  - `eligibilityInvalidated` falso → `contracting/contract-case/eligibility-invalidated`
- **Efeitos sobre o estado**: `status ← submitted_for_effect`; `evidenceBundleHash ← cmd`; `submittedAt ← at`
- **Eventos emitidos**: `ContractSubmittedForEffect`
- **Resposta**: `contractId: id`

### CancelContract

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: chamada gRPC (cedente; backoffice - cancelar contrato antes da eficácia é ação de supervisor com segundo aprovador, 17 §5)
- **Entrada**: `reason: string`
- **Pré-condições**:
  - `status` em qualquer estado não terminal até `signed`, ou `submitted_for_effect` com `effectOutcome` diferente de `none` → `contracting/contract-case/not-cancellable`
  - `reason` não vazio → `contracting/contract-case/reason-missing`
- **Efeitos sobre o estado**: `status ← cancelled`; envelope aberto `← invalidated`; `closeReason ← reason`; `closedAt ← at`
- **Eventos emitidos**: `ContractCancelled` (com `terminalState = cancelled`)
- **Resposta**: `contractId: id`

### ExpireContractCase

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: processo periódico do contexto (mesmo padrão da expiração de reserva, 06 §6.2) [ABERTA: agendador de expiração no kernel]
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status` em `preparing`, `pending_validation`, `mismatch`, `ready_for_signature`, `signing` ou `signature_failed` → `contracting/contract-case/not-expirable`
  - `at ≥ expiresAt` → `contracting/contract-case/not-yet-expired`
- **Efeitos sobre o estado**: `status ← expired`; envelope aberto `← invalidated`; `closeReason ← expired`; `closedAt ← at`
- **Eventos emitidos**: `ContractCancelled` (com `terminalState = expired`, que libera a reserva no Eligibility)
- **Resposta**: `contractId: id`

### BlockOnEligibilityInvalidation

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: consumo de `EligibilityInvalidated`, um comando por caso de `FindContractCasesByEligibilityEvaluation`
- **Entrada**: `eligibilityEvaluationId: id`, `invalidatedAt: instant`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: nos estados anteriores a `submitted_for_effect`: `eligibilityInvalidated ← true`. Em `submitted_for_effect` e nos terminais: aceita sem alterar (o Effects trata a invalidação após a submissão, 09 §7)
- **Eventos emitidos**: nenhum [ABERTA: o bloqueio não gera evento; o Ops não é avisado]
- **Resposta**: `contractId: id`

### RecordEffectShortfall

- **Agregado**: `ContractCase`; exige existente
- **Disparo**: consumo de `EffectRejected` ou de `EffectPartiallyConfirmed`
- **Entrada**: `outcome: rejected | partially_confirmed`, `effectId: id`, `belowProductMinimum: bool`
- **Pré-condições**: `status = submitted_for_effect` → `contracting/contract-case/not-submitted-for-effect`
- **Efeitos sobre o estado**: `effectOutcome ← outcome`. Parcial com `partialConfirmationClause = all_or_nothing` ou `belowProductMinimum`: `status ← cancelled`, `closeReason ← effect_shortfall` (09 §5.1: o Effects pede então a liberação do confirmado). Rejeição e parcial nos demais caminhos só registram; a decisão é humana (cancelar ou corrigir e reenviar; revisão contratual)
- **Eventos emitidos**: `ContractCancelled` (só no cancelamento automático)
- **Resposta**: `contractId: id`, `status`

### PrepareInstrument

- **Agregado**: `ContractInstrument`; **cria**
- **Disparo**: chamada gRPC (financiador pela Partner API, modo `funder_supplied`); processo do contexto (modo `platform_filled`)
- **Entrada**: `contractId: id`, `version: int`, `mode`, `templateRef: string`, `documentHash: string`, `documentEvidenceId: id`, `conditionsDocumentHash: string`, termos do documento (`currency`, `grossAmountMinor`, `netAmountMinor`, `effectiveMonthlyRate`, `assetSetHash`, `partyOrganizationIds`, `expectedCreditDate`, `partialConfirmationClause`)
- **Pré-condições**:
  - versão ainda não preparada → `contracting/contract-instrument/version-already-prepared`
  - `documentHash` não vazio → `contracting/contract-instrument/document-hash-empty`
  - `platform_filled` com `templateRef` → `contracting/contract-instrument/template-missing`
- **Efeitos sobre o estado**: campos `← cmd`; `preparedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `instrumentKey: string`

### DefineAssetSchedule

- **Agregado**: `AssetSchedule`; **inicializa se ausente**
- **Disparo**: processo do contexto, após `ContractCaseCreated`, com a cobertura da oferta lida do Marketplace
- **Entrada**: `contractId: id`, `scheduleMode`, `assetIds: list<id>`, `assetSetHash: string`, `assetCount: int`, regra (`ruleAssetTypes`, `rulePeriodFrom`, `rulePeriodUntil`, `ruleCeilingAmountMinor`, `disbursementMode`), `ruleBasedSupported: bool`
- **Pré-condições**:
  - `enumerated` com ao menos um ativo → `contracting/asset-schedule/schedule-empty`
  - `rule_based` só com `ruleBasedSupported` [A CONFIRMAR: B11, D6] → `contracting/asset-schedule/rule-based-unsupported`
- **Efeitos sobre o estado**: quando ausente: campos `← cmd`; `definedAt ← at`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: nenhum
- **Resposta**: `contractId: id`

### RegisterSigningAuthority

- **Agregado**: `SigningAuthority`; **inicializa se ausente**
- **Disparo**: consumo de `RepresentationVerified`
- **Entrada**: `representationId: id`, `organizationId: id`, `personId: id`, `powerType`, `validFrom: instant`, `validUntil: instant`, `hasAmountLimit: bool`, `maxAmountMinor: int`, `actTypes: list<string>`, `verifiedAt: instant`
- **Pré-condições**: `representationId`, `organizationId` e `powerType` informados → `contracting/signing-authority/representation-incomplete`
- **Efeitos sobre o estado**: quando ausente: `status ← verified`; campos `← cmd`. Quando presente: nenhum - a `Representation` é imutável (90 §3.1)
- **Eventos emitidos**: nenhum
- **Resposta**: `representationId: id`

### RevokeSigningAuthority

- **Agregado**: `SigningAuthority`; exige existente
- **Disparo**: consumo de `RepresentationRevoked`
- **Entrada**: `representationId: id`, `revokedAt: instant`, `revocationReason: string`
- **Pré-condições**: a representação é conhecida → `contracting/signing-authority/unknown-representation`
- **Efeitos sobre o estado**: `status ← revoked`; `revokedAt` e motivo `← cmd`. Já revogada: aceita sem alterar. Assinatura posterior é recusada em `RecordSignature`, mesmo com o envelope já criado (08 §5.3)
- **Eventos emitidos**: nenhum
- **Resposta**: `representationId: id`

### SealEvidenceBundle

- **Agregado**: `ContractEvidenceBundle`; **inicializa se ausente**
- **Disparo**: processo do contexto, após `ContractSigned`
- **Entrada**: `contractId: id`, `items: list<kind, referenceId, contentHash, origin>`, `bundleHash: string`
- **Pré-condições**:
  - `items` cobre todos os tipos de 08 §7 do lado do Contracting → `contracting/contract-evidence-bundle/incomplete`
  - todo item com `contentHash` → `contracting/contract-evidence-bundle/item-without-hash`
- **Efeitos sobre o estado**: quando ausente: campos `← cmd`; `sealedAt ← at`. Quando presente: nenhum - dossiê imutável
- **Eventos emitidos**: nenhum
- **Resposta**: `contractId: id`, `bundleHash: string`

## Eventos de domínio

92 §4 lista só cinco dos sete eventos do PRD (faltam `ContractCaseCreated` e `SignatureRequested`) e 92 §3 não traz schema de nenhum deles; os campos abaixo são derivados do agregado [ABERTA: 92:§3 e §4]. `contractId` é a identidade de `ContractCase`.

### ContractCaseCreated

- **Campos**: `contractId: id ← ContractCase.selectionId`, `offerId: id ← ContractCase.offerId`, `fundingRequestId: id ← ContractCase.fundingRequestId`, `cedentOrganizationId: id ← ContractCase.cedentOrganizationId`, `funderOrganizationId: id ← ContractCase.funderOrganizationId`, `productId: id ← ContractCase.productId`, `at: instant ← at`

### ContractValidated

- **Campos**: `contractId: id ← ContractCase.selectionId`, `instrumentVersion: int ← ContractCase.currentInstrumentVersion`, `presentedDocumentHash: string ← ContractCase.presentedDocumentHash`, `revalidation: list<check, result, referenceId, checkedAt> ← cmd.revalidation`, `creditDateAlert: bool ← cmd.creditDateDeviationBusinessDays > cmd.creditDateToleranceBusinessDays`, `at: instant ← at`

### ContractMismatchDetected

- **Campos**: `contractId: id ← ContractCase.selectionId`, `instrumentVersion: int ← ContractCase.currentInstrumentVersion`, `mismatchFields: list<string> ← ContractCase.mismatchFields`, `at: instant ← at`

### SignatureRequested

- **Campos**: `contractId: id ← ContractCase.selectionId`, `instrumentVersion: int ← ContractCase.currentInstrumentVersion`, `round: int ← ContractCase.envelope.round`, `signatureLevel: string ← ContractCase.envelope.signatureLevel`, `signatories: list<role, representationId> ← ContractCase.envelope.signatories`, `materialConditionsAcceptanceHash: string ← ContractCase.materialConditionsAcceptanceHash`, `materialConditionsAcceptedAt: instant ← ContractCase.materialConditionsAcceptedAt`, `envelopeExpiresAt: instant ← ContractCase.envelope.expiresAt`, `at: instant ← at`

### ContractSigned

- **Campos**: `contractId: id ← ContractCase.selectionId`, `instrumentVersion: int ← ContractCase.currentInstrumentVersion`, `presentedDocumentHash: string ← ContractCase.presentedDocumentHash`, `signedDocumentHash: string ← ContractCase.signedDocumentHash`, `signatoryCount: int ← size(envelope.signatories)`, `signedAt: instant ← ContractCase.signedAt`, `at: instant ← at`

### ContractSubmittedForEffect

- **Campos**: `contractId: id ← ContractCase.selectionId`, `cedentOrganizationId: id ← ContractCase.cedentOrganizationId`, `funderOrganizationId: id ← ContractCase.funderOrganizationId`, `natureOfOperation: string ← ContractCase.natureOfOperation`, `hasRecourse: bool ← ContractCase.hasRecourse`, `partialConfirmationClause ← ContractCase.partialConfirmationClause`, `scheduleMode ← ContractCase.scheduleMode`, `assetSetHash: string ← ContractCase.assetSetHash`, `assetCount: int ← ContractCase.assetCount`, `currency: string ← ContractCase.currency`, `grossAmountMinor: int ← ContractCase.grossAmountMinor`, `netAmountMinor: int ← ContractCase.netAmountMinor`, `signedDocumentHash: string ← ContractCase.signedDocumentHash`, `evidenceBundleHash: string ← ContractCase.evidenceBundleHash`, `at: instant ← at`. A lista de ativos não vai no evento: o Effects a lê por `FindAssetSchedule`, conferida pelo `assetSetHash` (a exceção de 92 §3.2 à convenção "autocontido"). O comando regulatório deriva desses dados versionados (critério do PRD).

### ContractCancelled

- **Campos**: `contractId: id ← ContractCase.selectionId`, `terminalState: cancelled | rejected | expired ← ContractCase.status`, `reason: string ← ContractCase.closeReason`, `previousStatus: string ← status anterior`, `at: instant ← at`. Cobre `CANCELLED`, `REJECTED` e `EXPIRED`, porque os três liberam a reserva e o Effects precisa distinguir o caso [ABERTA: o PRD lista só `ContractCancelled`]

## Consultas

- `FindContractCase` - filtros: `contractId`; cardinalidade: `one`
- `FindContractCasesByEligibilityEvaluation` - filtros: `eligibilityEvaluationId`; cardinalidade: `many` (fan-out de `BlockOnEligibilityInvalidation`)
- `FindContractCasesByCedent` - filtros: `cedentOrganizationId`, `status`; cardinalidade: `many` (portal do cedente)
- `FindContractInstrument` - filtros: `contractId`, `version`; cardinalidade: `one` (referência ao documento apresentado e ao assinado, para o download)
- `FindContractInstrumentsByContract` - filtros: `contractId`; cardinalidade: `many` (histórico de versões)
- `FindAssetSchedule` - filtros: `contractId`; cardinalidade: `one`
- `FindSigningAuthority` - filtros: `representationId`; cardinalidade: `one`
- `FindContractEvidenceBundle` - filtros: `contractId`; cardinalidade: `one`

## Relações entre agregados

- `ContractCase.selectionId` → `OfferSelection` (Marketplace); `ContractCase.eligibilityEvaluationId` → `EligibilityEvaluation` (Eligibility); `ContractCase.snapshotId` → `PortfolioSnapshot` (Portfolio); `ContractCase.kybCaseId` → `VerificationCase` (KYB/KYC)
- `ContractCase.cedentOrganizationId`, `ContractCase.funderOrganizationId` → `Organization` (Identity); `ContractCase.productId` → `FinancialProduct` (Partner Management)
- `ContractCase.envelope.signatories.representationId` → `SigningAuthority` → `Representation` (Identity)
- `ContractInstrument.contractId`, `AssetSchedule.contractId`, `ContractEvidenceBundle.contractId` → `ContractCase`

## Integração

- **Publica** (integration events): `ContractCaseCreated`, `ContractValidated`, `ContractMismatchDetected`, `SignatureRequested`, `ContractSigned`, `ContractSubmittedForEffect` e `ContractCancelled` - viram `contract_case_created.proto`, `contract_validated.proto`, `contract_mismatch_detected.proto`, `signature_requested.proto`, `contract_signed.proto`, `contract_submitted_for_effect.proto` e `contract_cancelled.proto` em `apps/backend/contracting/contract/proto/company/contracting/event/v1/`, package `company.contracting.event.v1`. Consumidores declarados em 92 §4: Effects (`ContractSigned`, `ContractSubmittedForEffect`, `ContractCancelled`), Eligibility (`ContractCancelled`), Ops (`ContractMismatchDetected`) e Ledger. Faltam consumidores para o que o desenho pede: o Marketplace move o pedido a `CONTRACTING` em `ContractCaseCreated` e a `CLOSED` em `ContractSigned` ou `ContractCancelled` (07 §6.1), e Notificações usa `ContractSigned` (16 §4) [ABERTA: 92:§4]. Effects consome `ContractSigned` e `ContractSubmittedForEffect` para o mesmo gatilho [ABERTA: 92:§4].
- **Consome** (disposições de consumo por FND-04 §6.4; recusa de negócio segue a disposição de falha definitiva):
  - `company.marketplace.event.v1.OfferSelected` → `OpenContractCase`. Mapa: `selectionId ← selectionId`, `offerId ← offerId`, `fundingRequestId ← fundingRequestId`, `presentationId ← presentationId`, `offerVersionHash ← offerVersionHash`, `eligibilityEvaluationId ← revalidation.eligibilityEvaluationId`, `snapshotId ← revalidation.snapshotId`, `kybCaseId ← revalidation.kybCaseId`, `consentIds ← revalidation.consentIds`, `revalidationOutcome ← revalidation.outcome`, `selectionAcceptanceHash ← materialConditionsAcceptance.documentHash`, `selectionAcceptedAt ← materialConditionsAcceptance.acceptedAt`. Financiador, produto, termos monetários, cobertura de ativos e natureza ← consulta gRPC à oferta no Marketplace, porque o evento não os carrega [ABERTA: 92:§3.3 × 08 §4; o evento é autocontido ou o contexto busca de volta]. O bff devolve `contractId` na resposta de seleção (91 §3.4), que é a identidade natural do caso.
  - `company.eligibility.event.v1.EligibilityInvalidated` → `BlockOnEligibilityInvalidation`. Mapa: `eligibilityEvaluationId ← evaluationId`, `invalidatedAt ← at` [ABERTA: sem schema em 92].
  - `company.identity.event.v1.RepresentationVerified` → `RegisterSigningAuthority`. Mapa: `representationId`, `organizationId`, `personId`, `powerType`, `validFrom`, `validUntil`, `scope.maxAmount`, `scope.actTypes` e `verifiedAt` do evento [ABERTA: 92 não especifica o payload; precisa carregar o escopo da alçada, 90 §3.1].
  - `company.identity.event.v1.RepresentationRevoked` → `RevokeSigningAuthority`. Mapa: `representationId`, `revokedAt`, `revocationReason` do evento.
  - `company.effects.event.v1.EffectRejected` → `RecordEffectShortfall` com `outcome = rejected`. Mapa: `contractId ← contractId`, `effectId ← effectId`.
  - `company.effects.event.v1.EffectPartiallyConfirmed` → `RecordEffectShortfall` com `outcome = partially_confirmed`. Mapa: `contractId`, `effectId`, `belowProductMinimum` do evento.
- **Consultas e comandos de saída**: consulta gRPC a Marketplace (oferta selecionada), Partner Management (modelo do instrumento em `PartnerAgreement` e nível de assinatura por produto [ABERTA: 13 não define o modelo do instrumento]), Portfolio, Eligibility, Consent e KYB (revalidação, 08 §7) e, se a decisão for essa, Identity (poder no instante). O provedor de assinatura é externo: envelope, callback e consulta ficam atrás de uma porta do contexto [ABERTA: ciclo de comando externo sem suporte no kernel; provider de assinatura não escolhido, #9].

## Borda e persistência

- **Borda**: só gRPC, `company.contracting.service.v1.ContractingService`, com um RPC por comando (`OpenContractCase`, `SubmitInstrumentForValidation`, `ValidateContract`, `AcceptMaterialConditions`, `RequestSignature`, `RecordSignature`, `RecordSignatureRefusal`, `RecordSigningFailure`, `SubmitForEffect`, `CancelContract`, `ExpireContractCase`, `BlockOnEligibilityInvalidation`, `RecordEffectShortfall`, `PrepareInstrument`, `DefineAssetSchedule`, `RegisterSigningAuthority`, `RevokeSigningAuthority`, `SealEvidenceBundle`) e por consulta (as oito de Consultas); os disparados só por consumo ou por processo não são chamados pelo bff. O REST é do `bff`: a seleção de oferta com a revalidação (`POST /v1/funding-requests/{id}/selection`, 91 §3.4), a recepção de instrumento do financiador, os webhooks do provedor de assinatura e o download do instrumento firmado.
- **Banco**: database e role `contracting`, schema `public`, tabelas `contract_cases`, `contract_instruments`, `asset_schedules`, `signing_authorities` e `contract_evidence_bundles`, com `tenant_id` à frente da chave. Persistência híbrida: em `contract_cases`, colunas tipadas `status` e `cedent_organization_id` (`FindContractCasesByCedent`), `eligibility_evaluation_id` (`FindContractCasesByEligibilityEvaluation`) e `expires_at` (varredura de `ExpireContractCase`); em `contract_instruments`, `contract_id` (`FindContractInstrumentsByContract`); as demais tabelas têm a identidade natural como chave e o resto em `snapshot` `jsonb`, inclusive `assetIds` [ABERTA: volume, 92 §3.4 cita 1284 alvos]. Documentos e evidências ficam por referência e hash [ABERTA: provider de object storage].
- **Instante**: inteiro de nanossegundos no domínio.
- Diverge do design: 90 §1.3 fala em milissegundos e 92 §1 em envelope JSON com `schemaUri`; vale o DMPF (nanossegundos; Protobuf com CloudEvents montado pelo relay).
- Diverge do design: multi-tenancy por `tenant_id` à frente da chave e escopo injetado pelo `ExecutionContext` (ADR-051 do DMPF), não por RLS (00 §4.1). O caso pertence ao tenant do cedente; o financiador o vê por projeção e o backoffice lê entre tenants [ABERTA: requer extensão do kernel - ADR-051 DMPF].
- Diverge do design: broker Kafka (default do DMPF; confirmação na decisão D4 da fundação), não SQS.

## Políticas transversais

- **Idempotência**: todo comando de entrada corre em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); os disparados por evento têm a deduplicação da inbox (ADR-036), e a abertura do caso é idempotente por `selectionId`. `RequestSignature` tem chave obrigatória na borda (91 §2.2).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). A alçada de assinatura é regra de domínio (`SigningAuthority`), não do gancho; cancelar contrato pelo backoffice exige supervisor e segundo aprovador (17 §5) [ABERTA: dupla aprovação no kernel].
- **Auditoria**: pela trilha de auditoria do kernel (FND-08) e pelo Ledger por outbox (12 §2); o dossiê guarda referência e hash de cada item, e o conteúdo recuperável fica no armazenamento.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `contracting` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO uma seleção sem caso e uma revalidação valid
QUANDO OpenContractCase chega com termos coerentes
ENTÃO o caso nasce preparing com as partes cedente e financiador, createdAt = at, e ContractCaseCreated vai à outbox
DADO um caso já aberto para a seleção
QUANDO OpenContractCase chega de novo
ENTÃO aceita sem alterar e sem emitir evento
DADO revalidationOutcome invalid
QUANDO OpenContractCase chega
ENTÃO rejeita com contracting/contract-case/offer-not-validated e nada é escrito
DADO bruto menos líquido diferente do custo total
QUANDO OpenContractCase chega
ENTÃO rejeita com contracting/contract-case/inconsistent-offer-terms
DADO assetCount zero
QUANDO OpenContractCase chega
ENTÃO rejeita com contracting/contract-case/no-assets

DADO um caso ready_for_signature com envelope parcialmente assinado
QUANDO SubmitInstrumentForValidation chega com a versão seguinte
ENTÃO o caso vai a pending_validation, o envelope é invalidado e o aceite das condições é zerado
DADO um caso signed
QUANDO SubmitInstrumentForValidation chega
ENTÃO rejeita com contracting/contract-case/not-open-for-instrument
DADO currentInstrumentVersion 1
QUANDO SubmitInstrumentForValidation chega com a versão 3
ENTÃO rejeita com contracting/contract-case/instrument-version-out-of-order
DADO scheduleDefined falso
QUANDO SubmitInstrumentForValidation chega
ENTÃO rejeita com contracting/contract-case/schedule-not-defined
DADO eligibilityInvalidated verdadeiro
QUANDO SubmitInstrumentForValidation chega
ENTÃO rejeita com contracting/contract-case/eligibility-invalidated

DADO um caso pending_validation e um instrumento igual à oferta
QUANDO ValidateContract chega com todas as revalidações em pass
ENTÃO o caso vai a ready_for_signature e ContractValidated é emitido
DADO um instrumento com líquido diferente de um centavo
QUANDO ValidateContract chega
ENTÃO o caso vai a mismatch com mismatchFields contendo netAmount e ContractMismatchDetected é emitido
DADO um caso em preparing
QUANDO ValidateContract chega
ENTÃO rejeita com contracting/contract-case/not-pending-validation
DADO uma versão diferente da corrente
QUANDO ValidateContract chega
ENTÃO rejeita com contracting/contract-case/stale-instrument-version
DADO uma revalidação com consent_active em fail
QUANDO ValidateContract chega
ENTÃO rejeita com contracting/contract-case/revalidation-failed e o caso segue pending_validation

DADO um caso ready_for_signature
QUANDO AcceptMaterialConditions chega com o hash do documento apresentado
ENTÃO o aceite fica registrado para a versão, com hash e instante, separado da assinatura
DADO um caso pending_validation
QUANDO AcceptMaterialConditions chega
ENTÃO rejeita com contracting/contract-case/not-ready-for-acceptance
DADO um hash diferente do apresentado
QUANDO AcceptMaterialConditions chega
ENTÃO rejeita com contracting/contract-case/conditions-hash-mismatch

DADO um caso ready_for_signature com aceite da versão corrente
QUANDO RequestSignature chega com cedente e financiador
ENTÃO o caso vai a signing com uma rodada requested e SignatureRequested é emitido
DADO um caso pending_validation
QUANDO RequestSignature chega
ENTÃO rejeita com contracting/contract-case/not-ready-for-signature
DADO um caso ready_for_signature sem aceite das condições
QUANDO RequestSignature chega
ENTÃO rejeita com contracting/contract-case/material-conditions-not-accepted
DADO uma lista de signatários sem o financiador
QUANDO RequestSignature chega
ENTÃO rejeita com contracting/contract-case/signatories-incomplete

DADO um envelope requested com dois signatários e o cedente na vez
QUANDO RecordSignature chega com poder verified dentro da alçada e evidência completa
ENTÃO o cedente vai a signed, o envelope a partially_signed e nenhum evento é emitido
DADO o financiador como último signatário pendente
QUANDO RecordSignature chega
ENTÃO o caso vai a signed com signedDocumentHash e ContractSigned é emitido
DADO um caso ready_for_signature
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/no-open-signing-round
DADO uma representação que não está no envelope
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/signatory-not-expected
DADO o financiador antes do cedente
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/out-of-signing-order
DADO uma representação revogada depois da criação do envelope
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/power-not-in-force
DADO powerTypeCompatible falso
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/power-type-incompatible
DADO maxAmountMinor menor que o bruto da operação
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/amount-exceeds-authority informando o limite e o valor da operação
DADO actTypes que não contém o ato
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/act-type-not-authorized
DADO uma assinatura sem trustedTimestampRef
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/evidence-incomplete
DADO a última assinatura com availabilityRevalidation failed
QUANDO RecordSignature chega
ENTÃO rejeita com contracting/contract-case/assets-unavailable

DADO um envelope requested
QUANDO RecordSignatureRefusal chega com motivo
ENTÃO o caso vai a rejected e ContractCancelled é emitido com terminalState rejected
DADO uma recusa sem motivo
QUANDO RecordSignatureRefusal chega
ENTÃO rejeita com contracting/contract-case/reason-missing
DADO um envelope requested que expirou
QUANDO RecordSigningFailure chega com envelope_expired
ENTÃO o caso vai a signature_failed e uma nova RequestSignature é aceita

DADO um caso signed com dossiê selado
QUANDO SubmitForEffect chega
ENTÃO o caso vai a submitted_for_effect e ContractSubmittedForEffect é emitido com o evidenceBundleHash
DADO um caso signing
QUANDO SubmitForEffect chega
ENTÃO rejeita com contracting/contract-case/not-signed
DADO um caso signed e evidenceBundleHash vazio
QUANDO SubmitForEffect chega
ENTÃO rejeita com contracting/contract-case/evidence-bundle-not-sealed

DADO um caso ready_for_signature
QUANDO CancelContract chega com motivo
ENTÃO o caso vai a cancelled e ContractCancelled é emitido com terminalState cancelled
DADO um caso submitted_for_effect com effectOutcome none
QUANDO CancelContract chega
ENTÃO rejeita com contracting/contract-case/not-cancellable
DADO um caso ready_for_signature com expiresAt vencido
QUANDO ExpireContractCase chega
ENTÃO o caso vai a expired e ContractCancelled é emitido com terminalState expired
DADO um caso signed
QUANDO ExpireContractCase chega
ENTÃO rejeita com contracting/contract-case/not-expirable
DADO um caso signing com expiresAt no futuro
QUANDO ExpireContractCase chega
ENTÃO rejeita com contracting/contract-case/not-yet-expired

DADO um caso ready_for_signature
QUANDO BlockOnEligibilityInvalidation chega
ENTÃO eligibilityInvalidated fica verdadeiro e o estado não muda
DADO um caso submitted_for_effect e EffectPartiallyConfirmed com all_or_nothing
QUANDO RecordEffectShortfall chega com partially_confirmed
ENTÃO o caso vai a cancelled com closeReason effect_shortfall e ContractCancelled é emitido
DADO um caso submitted_for_effect e EffectRejected
QUANDO RecordEffectShortfall chega com rejected
ENTÃO effectOutcome fica rejected e o caso segue submitted_for_effect
DADO um caso signing
QUANDO RecordEffectShortfall chega
ENTÃO rejeita com contracting/contract-case/not-submitted-for-effect

DADO uma versão nova no modo funder_supplied
QUANDO PrepareInstrument chega com documentHash
ENTÃO a versão nasce imutável com preparedAt = at
DADO uma versão já preparada
QUANDO PrepareInstrument chega com o mesmo contractId e version
ENTÃO rejeita com contracting/contract-instrument/version-already-prepared
DADO um documento sem hash
QUANDO PrepareInstrument chega
ENTÃO rejeita com contracting/contract-instrument/document-hash-empty
DADO o modo platform_filled sem templateRef
QUANDO PrepareInstrument chega
ENTÃO rejeita com contracting/contract-instrument/template-missing

DADO uma oferta com ativos enumerados
QUANDO DefineAssetSchedule chega
ENTÃO o cronograma nasce com assetSetHash e assetCount
DADO scheduleMode enumerated sem ativos
QUANDO DefineAssetSchedule chega
ENTÃO rejeita com contracting/asset-schedule/schedule-empty
DADO scheduleMode rule_based sem suporte
QUANDO DefineAssetSchedule chega
ENTÃO rejeita com contracting/asset-schedule/rule-based-unsupported

DADO uma representação verificada com limite de valor
QUANDO RegisterSigningAuthority chega
ENTÃO a autoridade nasce verified com o escopo do poder e nenhum evento é emitido
DADO uma representação sem powerType
QUANDO RegisterSigningAuthority chega
ENTÃO rejeita com contracting/signing-authority/representation-incomplete
DADO uma autoridade verified
QUANDO RevokeSigningAuthority chega
ENTÃO a autoridade vai a revoked
DADO uma representação nunca registrada
QUANDO RevokeSigningAuthority chega
ENTÃO rejeita com contracting/signing-authority/unknown-representation

DADO um contrato assinado com todos os itens de 08 §7 e hashes
QUANDO SealEvidenceBundle chega
ENTÃO o dossiê nasce selado com bundleHash e sealedAt = at
DADO um dossiê sem o instrumento assinado
QUANDO SealEvidenceBundle chega
ENTÃO rejeita com contracting/contract-evidence-bundle/incomplete
DADO um item sem contentHash
QUANDO SealEvidenceBundle chega
ENTÃO rejeita com contracting/contract-evidence-bundle/item-without-hash

DADO dois casos com a mesma avaliação de elegibilidade e um de outra
QUANDO FindContractCasesByEligibilityEvaluation consulta a primeira avaliação
ENTÃO devolve exatamente os dois, fora de qualquer UoW
```

## Escopo fora

- **REST do bff**: seleção de oferta, recepção do instrumento do financiador, webhooks do provedor de assinatura e download do instrumento firmado ficam no `bff` (91 §3.4, §4, §5).
- **Geração do texto contratual**: os textos (08:T11) e o modelo homologado em `PartnerAgreement` são jurídicos e de Partner Management; aqui só os dados versionados e a correspondência.
- **Integração com o provedor de assinatura**: criação do envelope, callbacks e consulta, e a escolha do provedor (#9, C4); o contexto só tem a porta e o estado da rodada.
- **Registro do efeito**: mapeamento a tipo e código, revalidação da posição e envio à infraestrutura são do Effects (09); aqui só `ContractSubmittedForEffect`.
- **Confirmação parcial a jusante**: instrução proporcional, tudo-ou-nada e revisão contratual são de Funding e Effects; aqui só a cláusula e `RecordEffectShortfall`.
- **Verificação de identidade e de poderes na origem**: do Identity (01); aqui só a projeção `SigningAuthority`.
- **Dossiê reproduzível completo**: a composição com o comando de efeito e a confirmação é do Compliance Ledger (12).
- **Recebíveis futuros** (`rule_based`): fase 4; o modelo admite o modo, mas o fluxo só existe após B11 e D6 [A CONFIRMAR: B11, D6].
- **Notificação do signatário e do cedente**: de Notificações (16), a partir dos eventos publicados.
