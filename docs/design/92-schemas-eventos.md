# Schemas de Eventos

> **Camada**: construtiva
> **Origem**: lacuna identificada — `14-catalogo-contratos-eventos.md` define o
> envelope e lista ~50 nomes de evento, mas o campo `data` do envelope está
> literalmente vazio (`"data": {}`). Sem schema, não há contrato entre contextos,
> não há validação e não há teste de contrato — embora o próprio catálogo os
> declare obrigatórios.
> **Status**: proposta

## 1. Envelope

Reafirmado de `14-catalogo`, com os campos que faltavam especificar.

```json
{
  "eventId": "evt_01HN4K…",
  "eventType": "EffectConfirmed.v1",
  "schemaUri": "https://schemas.<dominio>/effects/EffectConfirmed/1.2.0",
  "occurredAt": "2026-09-16T15:00:00.000Z",
  "recordedAt": "2026-09-16T15:00:01.117Z",
  "tenantId": "ten_01HN4K…",
  "correlationId": "cor_01HN4K…",
  "causationId": "cmd_01HN4K…",
  "aggregateType": "RegulatoryEffect",
  "aggregateId": "eff_01HN4K…",
  "aggregateVersion": 7,
  "actor": {
    "type": "PARTNER_SYSTEM",
    "id": "prt_01HN4K…",
    "representationId": null,
    "onBehalfOfOrganizationId": null
  },
  "dataClassification": "CONFIDENTIAL",
  "data": {}
}
```

### 1.1 Campos acrescentados ao envelope original

| Campo | Razão |
| --- | --- |
| `schemaUri` | O catálogo exige "schemas publicados em registry versionado" e "cada evento tem versão explícita". Sem URI no envelope, o consumidor precisa inferir o schema do `eventType` — o que quebra quando há versão menor compatível. |
| `actor.representationId` | `01-prd-identity-organization.md` exige que toda ação em nome da empresa referencie uma `Representation` válida. Se o evento não carrega qual, a trilha do ledger não consegue provar sob qual poder a ação ocorreu. |
| `actor.onBehalfOfOrganizationId` | Distingue quem agiu de por quem agiu — necessário para o contador que opera para múltiplos clientes (persona P3). |
| `dataClassification` | Permite que o consumidor e o ledger apliquem política de mascaramento e retenção sem conhecer o schema específico. |

### 1.2 Versionamento

`eventType` carrega a versão maior (`.v1`). `schemaUri` carrega a versão completa
(`1.2.0`).

| Mudança | Efeito na versão | Compatibilidade |
| --- | --- | --- |
| Campo opcional acrescentado | Menor (`1.1.0` -> `1.2.0`) | Compatível |
| Campo opcional removido | Maior (`v1` -> `v2`) | Incompatível |
| Campo obrigatório acrescentado | Maior | Incompatível |
| Tipo de campo alterado | Maior | Incompatível |
| Semântica de campo alterada com o mesmo tipo | Maior | Incompatível — e é o caso mais perigoso, porque a validação não detecta |

**Regra derivada**: mudança de semântica sem mudança de tipo exige renomear o
campo. O schema não detecta essa classe de quebra, então a convenção precisa
impedi-la.

**Janela de convivência**: duas versões maiores coexistem por um período acordado
(`14-catalogo`, regra 3, fala em "janela acordada" sem defini-la). Proposta:
90 dias após a publicação da nova versão, com métrica de consumo da versão antiga
para confirmar que ninguém a lê antes de removê-la.

---

## 2. Convenções de payload

1. **Fato, não intenção.** O `data` descreve o que passou a ser verdade, não o
   que se pretende fazer.
2. **Referência, não conteúdo.** Documentos e dados `PII_SENSITIVE` entram como
   `EvidenceReference`. Nunca o valor.
3. **Autocontido para o consumidor pretendido.** O evento carrega o que o
   consumidor precisa para agir sem consultar o produtor de volta. Buscar de volta
   reintroduz acoplamento.
4. **Valor monetário sempre com moeda.** Nunca número solto.
5. **Sem campo derivável trivialmente.** Se o consumidor calcula, não se envia —
   exceto quando o cálculo depende de versão de política, caso em que se envia o
   resultado e o identificador da política.
6. **Motivo de falha preserva o original.** Código e texto do parceiro entram sem
   tradução, acompanhados da classificação canônica.

---

## 3. Schemas por contexto

### 3.1 Consent & Mandates

#### `ConsentGranted.v1`

```json
{
  "consentId": "cns_01HN4K…",
  "organizationId": "org_01HN4K…",
  "consentType": "RECEIVABLES_OPT_IN",
  "version": 1,
  "scope": {
    "assetTypes": ["CARD_RECEIVABLE_UNIT"],
    "purposes": ["ADVANCE_QUOTATION", "ADVANCE_CONTRACTING"],
    "recipientOrganizationIds": ["org_funder_1", "org_funder_2"],
    "dataCategories": ["SCHEDULE_POSITION", "REGISTRY_ENCUMBRANCES"],
    "periodFrom": "2026-09-16",
    "periodUntil": "2027-09-15"
  },
  "grantedByRepresentationId": "rep_01HN4K…",
  "acceptance": {
    "channel": "WEB",
    "documentVersion": "termo-optin-2.3",
    "documentHash": "sha256:…",
    "authenticationMethod": "MFA_TOTP",
    "acceptedAt": "2026-09-16T14:58:22.004Z",
    "evidenceRef": { "evidenceId": "evd_…", "contentHash": "sha256:…" }
  },
  "externalAuthorization": null
}
```

**Notas**:

- `consentType` distingue os quatro instrumentos que `03-prd` separa
  (`DataProcessingBasis`, `DataSharingConsent`, `ReceivablesOptIn`,
  `OperationalMandate`). Um evento por instrumento — o PRD é explícito em que
  "um aceite de termos não deve representar implicitamente todos esses
  instrumentos", e um evento genérico reintroduziria exatamente isso.
- `externalAuthorization` é nulo aqui e preenchido por `ExternalOptInActivated`.
  O consentimento interno existe antes da autorização externa; o escopo efetivo é
  a interseção (invariante do PRD).
- `documentHash` viabiliza "o texto exibido e o aceite são preservados por hash e
  versão".

#### `ConsentRevoked.v1`

```json
{
  "consentId": "cns_01HN4K…",
  "organizationId": "org_01HN4K…",
  "requestedAt": "2026-10-01T10:00:00.000Z",
  "effectiveAt": "2026-10-01T10:00:03.221Z",
  "requestedByRepresentationId": "rep_01HN4K…",
  "reason": "CLIENT_REQUEST",
  "propagation": [
    { "recipientOrganizationId": "org_funder_1", "status": "CONFIRMED", "confirmedAt": "2026-10-01T10:00:12.000Z" },
    { "recipientOrganizationId": "org_funder_2", "status": "PENDING",   "confirmedAt": null }
  ],
  "preservedObligations": {
    "activeContractIds": ["ctr_01HN4K…"],
    "note": "Contratos vigentes e obrigações de retenção não são afetados"
  }
}
```

**Nota**: `preservedObligations` materializa a invariante "revogação não desfaz
contratos válidos nem obrigações de retenção" no próprio evento. Um consumidor que
receba a revogação e apague dados violaria a invariante; com o campo, ele sabe o
que preservar.

### 3.2 Portfolio

#### `PortfolioSnapshotCreated.v1`

```json
{
  "snapshotId": "snp_01HN4K…",
  "portfolioId": "prt_01HN4K…",
  "organizationId": "org_01HN4K…",
  "discoveryId": "dsc_01HN4K…",
  "asOf": "2026-09-16T14:30:00.000Z",
  "sources": [
    {
      "providerId": "prv_registry_a",
      "system": "REGISTRY",
      "asOf": "2026-09-16T14:29:58.000Z",
      "coverage": "COMPLETE",
      "encumbranceCoverage": "COMPLETE"
    }
  ],
  "summary": {
    "assetCount": 1284,
    "grossTotal": { "currency": "BRL", "amountMinor": 48250000 },
    "byAssetType": [
      { "assetType": "CARD_RECEIVABLE_UNIT", "count": 1284, "grossTotal": { "currency": "BRL", "amountMinor": 48250000 } }
    ]
  },
  "contentHash": "sha256:…",
  "supersedesSnapshotId": "snp_01HN4J…",
  "staleAfter": "2026-09-16T18:30:00.000Z"
}
```

**Campo `encumbranceCoverage`**: `COMPLETE`, `PARTIAL` ou `UNAVAILABLE`.
Materializa a invariante "ausência de informação não significa ausência de
gravame". Um consumidor que receba `PARTIAL` e trate a disponibilidade como
confiável está errado — e o campo torna o erro detectável em teste de contrato.

**Ausência deliberada**: o evento não carrega a lista de ativos. Um snapshot de
milhares de unidades excederia qualquer limite razoável de mensagem. O consumidor
obtém os ativos pela API do Portfolio usando `snapshotId`. Isso contraria a
convenção 3 da seção 2 e a exceção precisa estar declarada — é o caso legítimo de
buscar de volta.

### 3.3 Marketplace

#### `OffersPresented.v1`

Evento de maior relevância probatória do contexto: `07-prd` exige que "a
apresentação pode ser reconstruída exatamente para auditoria".

```json
{
  "fundingRequestId": "fdr_01HN4K…",
  "presentationId": "prs_01HN4K…",
  "presentedAt": "2026-09-16T15:10:00.000Z",
  "presentedToUserId": "usr_01HN4K…",
  "rankingPolicyId": "rkp_v3",
  "rankingPolicyHash": "sha256:…",
  "calculationMethodId": "clc_v2",
  "offers": [
    {
      "offerId": "ofr_01HN4K…",
      "position": 1,
      "funderOrganizationId": "org_funder_1",
      "netAmount": { "currency": "BRL", "amountMinor": 46800000 },
      "totalCostAmount": { "currency": "BRL", "amountMinor": 1450000 },
      "effectiveMonthlyRate": { "value": "0.0198", "scale": 4, "basis": "MONTHLY" },
      "weightedAverageTermDays": 47,
      "validUntil": "2026-09-16T19:00:00.000Z",
      "rankingScore": "0.9412",
      "sponsored": false,
      "platformRemuneration": { "currency": "BRL", "amountMinor": 23400 },
      "originalPayloadHash": "sha256:…"
    }
  ],
  "excludedOffers": [
    { "offerId": "ofr_01HN4L…", "reason": "EXPIRED_BEFORE_PRESENTATION" }
  ],
  "presentationHash": "sha256:…"
}
```

**Notas**:

- `rankingPolicyHash` e `calculationMethodId` tornam a ordenação reproduzível.
  Sem eles, "auditável" significa apenas "registrado", não "verificável".
- `sponsored` e `platformRemuneration` atendem a "ordenação auditável, inclusive
  quando houver patrocínio" e "remuneração comercial não pode ser escondida no
  ranking". Se o campo existir, existiu remuneração — o silêncio deixa de ser
  possível.
- `excludedOffers` registra o que não foi mostrado e por quê. Sem isso, a
  reconstrução mostra o que apareceu mas não prova que nada foi omitido.

#### `OfferSelected.v1`

```json
{
  "fundingRequestId": "fdr_01HN4K…",
  "offerId": "ofr_01HN4K…",
  "selectionId": "sel_01HN4K…",
  "presentationId": "prs_01HN4K…",
  "selectedAt": "2026-09-16T15:14:33.000Z",
  "selectedByUserId": "usr_01HN4K…",
  "selectedByRepresentationId": "rep_01HN4K…",
  "offerVersionHash": "sha256:…",
  "revalidation": {
    "snapshotId": "snp_01HN4K…",
    "eligibilityEvaluationId": "elg_01HN4K…",
    "consentIds": ["cns_01HN4K…"],
    "kybCaseId": "kyb_01HN4K…",
    "revalidatedAt": "2026-09-16T15:14:30.000Z",
    "outcome": "VALID"
  },
  "materialConditionsAcceptance": {
    "documentHash": "sha256:…",
    "acceptedAt": "2026-09-16T15:14:32.000Z"
  }
}
```

**Nota**: o bloco `revalidation` materializa o requisito 8 de `07-prd`
("revalidar lote e consentimentos no momento da seleção"). Sem ele no evento, a
revalidação aconteceu mas não é provável — e "provável" é justamente o requisito.

### 3.4 Effects Orchestrator

#### `EffectConfirmed.v1`

```json
{
  "effectId": "eff_01HN4K…",
  "contractId": "ctr_01HN4K…",
  "effectType": "ASSIGNMENT",
  "registryPartnerId": "prv_registry_a",
  "beneficiaryOrganizationId": "org_funder_1",
  "externalReference": {
    "providerId": "prv_registry_a",
    "system": "REGISTRY",
    "referenceType": "EFFECT",
    "reference": "…",
    "asOf": "2026-09-16T16:02:11.000Z"
  },
  "confirmedAt": "2026-09-16T16:02:11.000Z",
  "confirmationSource": "CALLBACK",
  "targets": [
    {
      "targetId": "tgt_01HN4K…",
      "assetRef": { "assetType": "CARD_RECEIVABLE_UNIT", "internalId": "ast_…", "externalReferences": [] },
      "requestedAmount": { "currency": "BRL", "amountMinor": 1000000 },
      "confirmedAmount": { "currency": "BRL", "amountMinor": 1000000 },
      "status": "CONFIRMED"
    }
  ],
  "totals": {
    "requestedAmount": { "currency": "BRL", "amountMinor": 48000000 },
    "confirmedAmount": { "currency": "BRL", "amountMinor": 48000000 },
    "targetCount": 1284,
    "confirmedTargetCount": 1284
  },
  "evidenceRef": { "evidenceId": "evd_…", "contentHash": "sha256:…" }
}
```

**`confirmationSource`**: `CALLBACK`, `POLLED_QUERY` ou `FILE`. Distingue
confirmação recebida de confirmação buscada — relevante porque
`UNKNOWN_OUTCOME` só se resolve por consulta, e a trilha precisa mostrar qual
caminho produziu a certeza.

#### `EffectPartiallyConfirmed.v1`

Mesmo schema, com `status` misto nos alvos e `confirmedTargetCount <
targetCount`. Evento separado — e não um `EffectConfirmed` com totais
divergentes — porque o consumidor Funding precisa de rota de tratamento distinta,
e um consumidor que ignore a diferença desembolsaria valor integral sobre efeito
parcial.

#### `EffectOutcomeUnknown.v1`

```json
{
  "effectId": "eff_01HN4K…",
  "contractId": "ctr_01HN4K…",
  "registryPartnerId": "prv_registry_a",
  "commandId": "cmd_01HN4K…",
  "idempotencyKey": "…",
  "lastAttemptAt": "2026-09-16T16:00:00.000Z",
  "attemptCount": 3,
  "failureCategory": "TIMEOUT",
  "stateQuerySupported": true,
  "nextQueryScheduledAt": "2026-09-16T16:05:00.000Z",
  "blocksFunding": true,
  "operationalCaseId": null
}
```

**`stateQuerySupported`**: quando `false`, o evento indica que a resolução
automática é impossível e o caso vai direto para tratamento humano. Esse campo
depende do `CapabilityProfile` do parceiro — e expõe a lacuna apontada em
`00-arquitetura-tecnica.md`, 7.2: `13-prd-partner-management.md` não exige
operação de consulta de estado na homologação.

### 3.5 Funding

#### `DisbursementConfirmed.v1`

```json
{
  "fundingInstructionId": "fni_01HN4K…",
  "disbursementId": "dsb_01HN4K…",
  "contractId": "ctr_01HN4K…",
  "effectId": "eff_01HN4K…",
  "funderOrganizationId": "org_funder_1",
  "confirmedAt": "2026-09-16T17:30:00.000Z",
  "amount": { "currency": "BRL", "amountMinor": 46800000 },
  "expectedAmount": { "currency": "BRL", "amountMinor": 46800000 },
  "amountMatches": true,
  "destination": {
    "destinationId": "dst_01HN4K…",
    "destinationHash": "sha256:…",
    "validatedAt": "2026-09-15T11:00:00.000Z",
    "validationMethod": "OWNERSHIP_CHECK"
  },
  "destinationMatches": true,
  "funderReference": "…",
  "proofRef": { "evidenceId": "evd_…", "contentHash": "sha256:…" }
}
```

**Nota**: dados bancários não entram no evento. `destinationHash` permite ao
consumidor confirmar que o destino é o esperado sem que a conta trafegue. Isso
atende à convenção 2 e à classificação `PII_SENSITIVE` de
`90-modelo-de-dados.md`.

**Campos `amountMatches` e `destinationMatches`**: derivados, contrariando a
convenção 5. A exceção é deliberada — a comparação é a razão de ser do evento
(`10-prd`, requisito 7), e deixá-la implícita permitiria que um consumidor
esquecesse de compará-la.

### 3.6 Settlement & Reconciliation

#### `ReconciliationMismatchDetected.v1`

```json
{
  "reconciliationRunId": "rcr_01HN4K…",
  "itemId": "rci_01HN4K…",
  "caseId": "rcc_01HN4K…",
  "assetRef": { "assetType": "CARD_RECEIVABLE_UNIT", "internalId": "ast_…", "externalReferences": [] },
  "contractId": "ctr_01HN4K…",
  "mismatchType": "AMOUNT_DIVERGENCE",
  "expected": {
    "amount": { "currency": "BRL", "amountMinor": 1000000 },
    "date": "2026-10-15",
    "beneficiaryOrganizationId": "org_funder_1"
  },
  "reported": {
    "amount": { "currency": "BRL", "amountMinor": 987500 },
    "date": "2026-10-15",
    "beneficiaryOrganizationId": "org_funder_1",
    "sourceProviderId": "prv_registry_a",
    "sourceAsOf": "2026-10-15T23:00:00.000Z"
  },
  "difference": { "currency": "BRL", "amountMinor": -12500 },
  "severity": "LOW",
  "detectedAt": "2026-10-16T02:00:00.000Z"
}
```

**`mismatchType`**: `AMOUNT_DIVERGENCE`, `DATE_DIVERGENCE`,
`BENEFICIARY_DIVERGENCE`, `UNEXPECTED_SETTLEMENT`, `MISSING_SETTLEMENT`,
`DUPLICATE_SETTLEMENT`. `BENEFICIARY_DIVERGENCE` é a de maior gravidade — indica
pagamento a quem não deveria receber e é sinal de fraude ou falha de efeito.

---

## 4. Catálogo completo

| Contexto | Evento | Versão | Consumidores principais | Classificação |
| --- | --- | --- | --- | --- |
| Identity | `OrganizationRegistered` | v1 | KYB, Ledger | `INTERNAL` |
| Identity | `RepresentationVerified` | v1 | Consent, Contracting, Ledger | `INTERNAL` |
| Identity | `RepresentationRevoked` | v1 | Consent, Contracting, Ledger | `INTERNAL` |
| Identity | `UserSuspended` | v1 | Ledger | `INTERNAL` |
| KYB/KYC | `VerificationReady` | v1 | Eligibility, Marketplace, Ledger | `CONFIDENTIAL` |
| KYB/KYC | `DiscrepancyDetected` | v1 | Risco, Ledger | `CONFIDENTIAL` |
| KYB/KYC | `PartnerApproved` | v1 | Eligibility, Ledger | `CONFIDENTIAL` |
| KYB/KYC | `PartnerRejected` | v1 | Eligibility, Ledger | `CONFIDENTIAL` |
| KYB/KYC | `ReverificationRequired` | v1 | Eligibility, Ledger | `INTERNAL` |
| Consent | `ConsentGranted` | v1 | Gateway, Eligibility, Ledger | `INTERNAL` |
| Consent | `ExternalOptInActivated` | v1 | Portfolio, Ledger | `INTERNAL` |
| Consent | `ConsentRevoked` | v1 | Gateway, Eligibility, Marketplace, Ledger | `INTERNAL` |
| Consent | `ConsentExpired` | v1 | Eligibility, Ledger | `INTERNAL` |
| Consent | `AuthorizationSyncFailed` | v1 | Ops, Ledger | `INTERNAL` |
| Gateway | `ExternalCommandSubmitted` | v1 | Ledger | `INTERNAL` |
| Gateway | `ExternalCommandConfirmed` | v1 | Effects, Funding, Ledger | `INTERNAL` |
| Gateway | `ExternalCommandFailed` | v1 | Effects, Ops, Ledger | `INTERNAL` |
| Gateway | `ExternalOutcomeUnknown` | v1 | Effects, Funding, Ops, Ledger | `INTERNAL` |
| Gateway | `ExternalSchemaMismatchDetected` | v1 | Partners, Ops, Ledger | `INTERNAL` |
| Portfolio | `PortfolioSnapshotCreated` | v1 | Eligibility, Ledger | `CONFIDENTIAL` |
| Portfolio | `AssetPositionChanged` | v1 | Eligibility, Settlement, Ledger | `CONFIDENTIAL` |
| Portfolio | `AssetBecameStale` | v1 | Eligibility | `INTERNAL` |
| Portfolio | `ExternalPositionDiverged` | v1 | Settlement, Risco, Ledger | `CONFIDENTIAL` |
| Eligibility | `EligibleLotCreated` | v1 | Marketplace, Ledger | `CONFIDENTIAL` |
| Eligibility | `AssetReserved` | v1 | Portfolio, Ledger | `INTERNAL` |
| Eligibility | `ReservationExpired` | v1 | Marketplace, Portfolio | `INTERNAL` |
| Eligibility | `EligibilityInvalidated` | v1 | Marketplace, Contracting, Ledger | `INTERNAL` |
| Marketplace | `FundingRequestOpened` | v1 | Partners, Risco, Ledger | `CONFIDENTIAL` |
| Marketplace | `PartnerInvited` | v1 | Ledger | `INTERNAL` |
| Marketplace | `OfferReceived` | v1 | Ledger | `CONFIDENTIAL` |
| Marketplace | `OffersPresented` | v1 | Ledger | `CONFIDENTIAL` |
| Marketplace | `OfferSelected` | v1 | Contracting, Ledger | `CONFIDENTIAL` |
| Marketplace | `FundingRequestClosed` | v1 | Eligibility, Ledger | `INTERNAL` |
| Contracting | `ContractValidated` | v1 | Ledger | `CONFIDENTIAL` |
| Contracting | `ContractMismatchDetected` | v1 | Ops, Ledger | `CONFIDENTIAL` |
| Contracting | `ContractSigned` | v1 | Effects, Ledger | `CONFIDENTIAL` |
| Contracting | `ContractSubmittedForEffect` | v1 | Effects, Ledger | `INTERNAL` |
| Contracting | `ContractCancelled` | v1 | Effects, Eligibility, Ledger | `INTERNAL` |
| Effects | `RegulatoryEffectPlanned` | v1 | Ledger | `INTERNAL` |
| Effects | `EffectSubmitted` | v1 | Ledger | `INTERNAL` |
| Effects | `EffectConfirmed` | v1 | Funding, Portfolio, Settlement, Ledger | `INTERNAL` |
| Effects | `EffectPartiallyConfirmed` | v1 | Funding, Contracting, Ops, Ledger | `INTERNAL` |
| Effects | `EffectRejected` | v1 | Contracting, Ops, Ledger | `INTERNAL` |
| Effects | `EffectOutcomeUnknown` | v1 | Funding, Ops, Ledger | `INTERNAL` |
| Effects | `EffectReleased` | v1 | Portfolio, Ledger | `INTERNAL` |
| Effects | `EffectDisputed` | v1 | Ops, Risco, Ledger | `INTERNAL` |
| Funding | `FundingInstructionCreated` | v1 | Ledger | `CONFIDENTIAL` |
| Funding | `DisbursementRequested` | v1 | Ledger | `CONFIDENTIAL` |
| Funding | `DisbursementConfirmed` | v1 | Settlement, Ledger | `CONFIDENTIAL` |
| Funding | `FundingMismatchDetected` | v1 | Ops, Risco, Ledger | `CONFIDENTIAL` |
| Funding | `DisbursementReversed` | v1 | Settlement, Ops, Ledger | `CONFIDENTIAL` |
| Settlement | `SettlementExpected` | v1 | Ledger | `INTERNAL` |
| Settlement | `SettlementReported` | v1 | Portfolio, Ledger | `INTERNAL` |
| Settlement | `AssetSettled` | v1 | Portfolio, Effects, Ledger | `INTERNAL` |
| Settlement | `ReconciliationMatched` | v1 | Ledger | `INTERNAL` |
| Settlement | `ReconciliationMismatchDetected` | v1 | Ops, Risco, Ledger | `CONFIDENTIAL` |
| Settlement | `EffectReleaseRequested` | v1 | Effects, Ledger | `INTERNAL` |
| Partners | `CapabilityEnabled` | v1 | Gateway, Eligibility, Marketplace | `INTERNAL` |
| Partners | `CapabilitySuspended` | v1 | Gateway, Eligibility, Marketplace, Ops | `INTERNAL` |
| Partners | `PartnerProductChanged` | v1 | Eligibility, Marketplace | `INTERNAL` |
| Partners | `PartnerCertificateExpiring` | v1 | Ops | `INTERNAL` |

**Eventos propostos, ausentes do catálogo original:**

| Contexto | Evento | Razão |
| --- | --- | --- |
| Risco | `RiskSignalRaised` | Não existe contexto de fraude nos PRDs |
| Risco | `OperationBlockedByRisk` | Idem |
| Notificação | `NotificationDispatched` | Nenhum contexto trata comunicação |
| Ops | `OperationalCaseOpened` | Casos são criados sem evento correspondente |
| Ops | `OperationalCaseResolved` | Idem |
| Identity | `BankAccountChangeRequested` | Mudança de conta é vetor de fraude e não tem evento |
| Identity | `BankAccountChangeApproved` | Idem |

---

## 5. Governança

### 5.1 Registry

Schemas publicados em registry versionado, conforme `14-catalogo`. Requisitos
construtivos:

- o schema é publicado **antes** do código que o produz chegar a produção;
- publicação incompatível com a versão anterior sem incremento de versão maior é
  rejeitada pelo registry, não pela revisão humana;
- o produtor valida contra o registry no momento da publicação e falha a
  transação em caso de divergência — evento inválido nunca é emitido.

### 5.2 Testes de contrato

`14-catalogo` os declara obrigatórios sem definir escopo.

| Teste | Quem executa | O que verifica |
| --- | --- | --- |
| Produtor gera evento válido | Produtor, em CI | Evento emitido valida contra o schema publicado |
| Consumidor tolera campo desconhecido | Consumidor, em CI | Campo opcional novo não quebra o consumo |
| Consumidor rejeita campo obrigatório ausente | Consumidor, em CI | Falha visível, não silenciosa |
| Compatibilidade entre versões | CI do registry | Nova versão menor é compatível com a anterior |
| Exemplo canônico por evento | Ambos | Um exemplo por schema, versionado junto |

### 5.3 O que não fazer

- **Evento genérico com discriminador interno.** `ConsentChanged` com um campo
  `changeType` reintroduz o acoplamento que o catálogo evita e impede
  versionamento independente.
- **Payload com o agregado inteiro.** O evento carrega o fato, não o estado
  completo. Publicar o agregado transforma o evento em replicação de banco.
- **Reusar `eventId` em republicação.** Republicar após correção exige novo
  `eventId` — `14-catalogo`, regra 8: "correções geram novo evento".

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 1 | `14-catalogo-contratos-eventos.md` | Envelope sem `schemaUri`, representação e classificação |
| 3.1 | `03-prd-consent-mandates.md` | Prova de escopo e de aceite |
| 3.2 | `05-prd-portfolio.md` | Cobertura de gravame não representável |
| 3.3 | `07-prd-marketplace.md` | Reconstrução da apresentação |
| 3.4 | `09-prd-effects-orchestrator.md` | Confirmação parcial e resultado desconhecido |
| 3.5 | `10-prd-funding.md` | Comparação de valor e destino |
| 3.6 | `11-prd-settlement-reconciliation.md` | Tipos de divergência |
| 4 | `14-catalogo-contratos-eventos.md` | Catálogo sem consumidores nem classificação |
| 5 | `14-catalogo-contratos-eventos.md` | Testes de contrato sem escopo |
