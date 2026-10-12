# Catálogo Compartilhado — Contratos, Eventos e Regras de Integração

## Objetivo

Definir a linguagem de integração entre bounded contexts sem compartilhar modelos internos ou banco de dados.

## Envelope de evento

```json
{
  "eventId": "evt_01",
  "eventType": "RegulatoryEffectConfirmed.v1",
  "occurredAt": "2026-09-16T15:00:00Z",
  "recordedAt": "2026-09-16T15:00:01Z",
  "tenantId": "tenant_01",
  "correlationId": "corr_01",
  "causationId": "cmd_01",
  "aggregateType": "RegulatoryEffect",
  "aggregateId": "effect_01",
  "aggregateVersion": 7,
  "actor": {
    "type": "PARTNER_SYSTEM",
    "id": "partner_01"
  },
  "data": {}
}
```

## Regras de publicação e consumo

1. Eventos representam fatos concluídos, nunca intenções.
2. Comandos são enviados a um único proprietário do agregado.
3. Cada evento tem versão explícita e schema compatível durante a janela acordada.
4. Consumidores implementam inbox e idempotência por `eventId`.
5. Produtores usam transactional outbox.
6. Ordenação é garantida somente por agregado, não globalmente.
7. Payload não transporta documentos sensíveis; usa referências autorizadas.
8. Correções geram novo evento, sem alteração do evento original.

## Eventos por contexto

| Contexto | Eventos principais |
| --- | --- |
| Identity | `OrganizationRegistered`, `RepresentationVerified`, `RepresentationRevoked` |
| KYB/KYC | `VerificationReady`, `PartnerApproved`, `ReverificationRequired` |
| Consent | `ConsentGranted`, `ConsentRevoked`, `AuthorizationSyncFailed` |
| Gateway | `ExternalCommandConfirmed`, `ExternalOutcomeUnknown`, `ExternalSchemaMismatchDetected` |
| Portfolio | `PortfolioSnapshotCreated`, `AssetPositionChanged`, `ExternalPositionDiverged` |
| Eligibility | `EligibleLotCreated`, `AssetReserved`, `EligibilityInvalidated` |
| Marketplace | `OfferReceived`, `OffersPresented`, `OfferSelected` |
| Contracting | `ContractSigned`, `ContractMismatchDetected`, `ContractSubmittedForEffect` |
| Effects | `EffectConfirmed`, `EffectPartiallyConfirmed`, `EffectRejected`, `EffectReleased` |
| Funding | `DisbursementConfirmed`, `FundingMismatchDetected`, `DisbursementReversed` |
| Settlement | `AssetSettled`, `ReconciliationMismatchDetected`, `EffectReleaseRequested` |
| Partners | `CapabilityEnabled`, `CapabilitySuspended`, `PartnerProductChanged` |
| Compliance Ledger | Não publica eventos de domínio — consome os de todos os contextos |

## Tipos compartilhados mínimos

### Money

```json
{ "currency": "BRL", "amountMinor": 150000 }
```

### ExternalReference

```json
{
  "providerId": "provider_01",
  "system": "REGISTRY",
  "referenceType": "EFFECT",
  "reference": "external-reference",
  "asOf": "2026-09-16T15:00:00Z"
}
```

### AssetRef

```json
{
  "assetType": "BOOK_ENTRY_INVOICE",
  "internalId": "asset_01",
  "externalReferences": []
}
```

## Sagas principais

### Contratação e desembolso

`OfferSelected → ContractSigned → EffectSubmitted → EffectConfirmed → DisbursementRequested → DisbursementConfirmed`

Compensações são de negócio: expirar seleção, cancelar contrato antes de eficácia ou solicitar liberação do efeito. Não existe rollback técnico de um efeito já constituído.

### Liquidação e liberação

`SettlementReported → ReconciliationMatched → ObligationFulfilled → EffectReleaseRequested → EffectReleased`

## Política de falhas

- `RETRYABLE`: backoff com limite e mesma chave idempotente.
- `FINAL`: encerra tentativa e abre ação de negócio.
- `UNKNOWN_OUTCOME`: consultar estado externo antes de qualquer reenvio.
- `SCHEMA_MISMATCH`: suspender operação incompatível e alertar Partner Management.
- `RECONCILIATION_REQUIRED`: preservar fluxo e abrir caso operacional.

## Governança

- Schemas publicados em registry versionado.
- Mudança incompatível cria nova versão maior.
- Contexto produtor é proprietário semântico do evento.
- Dados derivados não retornam ao proprietário como se fossem fatos autoritativos.
- Testes de contrato são obrigatórios para parceiros e consumidores internos.

