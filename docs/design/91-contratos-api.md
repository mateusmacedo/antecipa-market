# Contratos de API

> **Camada**: construtiva
> **Origem**: lacuna identificada — os PRDs listam endpoints como
> `POST /funding-requests` sem corpo de requisição, resposta, código de erro,
> paginação, autenticação, versionamento ou idempotência. Cinco PRDs listam
> "APIs principais"; dez não listam nenhuma.
> **Status**: proposta

## 1. Superfícies de API

A plataforma expõe quatro superfícies com públicos, garantias e ritmos de
evolução diferentes. Tratá-las como uma só é erro estrutural: a API do parceiro
não pode quebrar quando a interface do cedente muda.

| Superfície | Consumidor | Autenticação | Estabilidade |
| --- | --- | --- | --- |
| **Client API** | Portal do cedente | OIDC, token de usuário | Interna; evolui com a interface |
| **Partner API** | Sistema do financiador | mTLS + credencial de cliente | Contratual; versionada; quebra exige aviso |
| **Inbound Webhooks** | Infraestruturas e provedores | Conforme o parceiro define | Ditada pelo parceiro |
| **Outbound Webhooks** | Sistema do cedente | Assinatura de payload | Contratual |

**Consequência de desenho**: `07-prd-marketplace.md` lista
`POST /funding-requests/{id}/offers` entre as "APIs principais" sem distinguir
quem chama. Essa rota pertence à Partner API — é o financiador que envia oferta.
Já `POST /funding-requests` pertence à Client API. Misturar as duas numa
superfície única acopla o ciclo de release do portal ao contrato com parceiros.

## 2. Convenções

### 2.1 Versionamento

Versão maior no caminho: `/v1/funding-requests`.

| Mudança | Versão | Aviso prévio |
| --- | --- | --- |
| Campo opcional acrescentado na resposta | Nenhuma | — |
| Campo opcional acrescentado na requisição | Nenhuma | — |
| Campo removido ou renomeado | Maior | 90 dias (Partner API) |
| Tipo alterado | Maior | 90 dias |
| Valor novo em enumeração | Nenhuma | Consumidor deve tolerar valor desconhecido |
| Semântica alterada | Maior | 90 dias |

**Regra sobre enumeração**: acrescentar valor a uma enumeração é compatível
apenas se os consumidores tratarem valor desconhecido sem falhar. Isso precisa
estar no contrato com o parceiro, não pressuposto.

### 2.2 Idempotência

Toda operação que cria ou altera estado aceita `Idempotency-Key` no cabeçalho.

- Chave repetida com corpo idêntico retorna a resposta original, com
  `Idempotency-Replayed: true`.
- Chave repetida com corpo diferente retorna `409`, código
  `IDEMPOTENCY_KEY_REUSE`.
- Janela de retenção da chave: 24 h, declarada na documentação.

**Obrigatória** — e não opcional — em: criação de solicitação, envio de oferta,
seleção de oferta, submissão de contrato para assinatura, criação de instrução de
desembolso.

### 2.3 Paginação

Baseada em cursor, nunca em deslocamento numérico.

```json
{
  "data": [],
  "pagination": {
    "nextCursor": "eyJ…",
    "hasMore": true,
    "limit": 50
  }
}
```

**Razão**: o conjunto de ativos muda entre páginas. Paginação por deslocamento
sobre dado que muda produz item repetido ou omitido — e omitir um ativo numa
listagem que fundamenta contratação é falha silenciosa. O cursor é estável
porque aponta para posição no conjunto ordenado, não para índice.

Limite padrão 50, máximo 200.

### 2.4 Erros

Estrutura única, em todas as superfícies.

```json
{
  "error": {
    "code": "ELIGIBILITY_INVALIDATED",
    "message": "A avaliação de elegibilidade não é mais válida",
    "category": "BUSINESS_RULE",
    "traceId": "cor_01HN4K…",
    "details": [
      { "field": "assetIds", "issue": "ASSET_NO_LONGER_AVAILABLE", "value": "ast_01HN4K…" }
    ],
    "retryable": false,
    "documentationUrl": "…"
  }
}
```

| Categoria | HTTP | Retentável | Significado |
| --- | --- | --- | --- |
| `VALIDATION` | 400 | não | Requisição malformada |
| `AUTHENTICATION` | 401 | não | Credencial ausente ou inválida |
| `AUTHORIZATION` | 403 | não | Sem permissão, ou representação inválida |
| `NOT_FOUND` | 404 | não | Recurso inexistente ou fora do tenant |
| `CONFLICT` | 409 | não | Estado incompatível com a operação |
| `BUSINESS_RULE` | 422 | não | Regra de negócio impede a operação |
| `RATE_LIMIT` | 429 | sim | Limite excedido; ver `Retry-After` |
| `INTERNAL` | 500 | sim | Falha da plataforma |
| `UPSTREAM_UNAVAILABLE` | 503 | sim | Parceiro indisponível |
| `UPSTREAM_UNKNOWN` | 502 | **não** | Resultado desconhecido no parceiro |

**`UPSTREAM_UNKNOWN` não é retentável, e essa é a decisão mais importante da
seção.** Reflete `UNKNOWN_OUTCOME`: o comando pode ter surtido efeito. Retentar
pode duplicar efeito regulatório. O cliente deve consultar o estado, não repetir.
Marcar como retentável seria coerente com a semântica usual de `502` e **errado**
neste domínio.

**`404` para recurso de outro tenant**: retornar `403` revelaria a existência do
recurso. `404` é a resposta correta — o recurso não existe, do ponto de vista de
quem pergunta.

### 2.5 Cabeçalhos

| Cabeçalho | Direção | Uso |
| --- | --- | --- |
| `Idempotency-Key` | entrada | 2.2 |
| `X-Correlation-Id` | ambos | Propagado ao envelope de evento; gerado se ausente |
| `X-Organization-Id` | entrada | Organização ativa, quando o usuário tem várias (persona P3) |
| `X-Representation-Id` | entrada | Representação invocada; obrigatório em ações que exigem poderes |
| `X-Api-Version` | saída | Versão exata que atendeu |
| `Retry-After` | saída | Em `429` e `503` |
| `X-RateLimit-Remaining` | saída | Partner API |

**`X-Representation-Id`**: fecha a lacuna entre a invariante de
`01-prd-identity-organization.md` ("uma ação em nome da empresa referencia uma
`Representation` válida") e a API, que hoje não tem onde carregar essa
referência. Sem o cabeçalho, o servidor teria de inferir a representação — e
inferir poder é o que a invariante proíbe.

---

## 3. Client API

Rotas que fecham lacunas. As demais seguem o padrão.

### 3.1 Simulação

Ausente dos PRDs, embora `06-prd-eligibility.md` requisito 9 exija "permitir
simulação sem criar reserva".

```http
POST /v1/organizations/{organizationId}/simulations
```

```json
{
  "assetSelection": {
    "mode": "BY_FILTER",
    "filter": {
      "assetTypes": ["CARD_RECEIVABLE_UNIT"],
      "settlementDateFrom": "2026-10-01",
      "settlementDateTo": "2026-12-31",
      "maxGrossAmount": { "currency": "BRL", "amountMinor": 50000000 }
    }
  },
  "productHints": { "funderOrganizationIds": [] }
}
```

Resposta `200`:

```json
{
  "simulationId": "sim_01HN4K…",
  "snapshotId": "snp_01HN4K…",
  "snapshotAsOf": "2026-09-16T14:30:00.000Z",
  "snapshotStaleAfter": "2026-09-16T18:30:00.000Z",
  "eligible": {
    "assetCount": 842,
    "grossTotal": { "currency": "BRL", "amountMinor": 31200000 }
  },
  "ineligible": {
    "assetCount": 442,
    "byReason": [
      { "reasonCode": "MATURITY_OUT_OF_RANGE", "assetCount": 310, "explanation": "Vencimento fora da faixa aceita" },
      { "reasonCode": "ENCUMBRANCE_PRESENT",   "assetCount": 132, "explanation": "Ativo com efeito registrado" }
    ]
  },
  "indicativeRange": null,
  "disclaimer": "Simulação não constitui oferta nem reserva de ativos"
}
```

**`indicativeRange` nulo**: intencional. `06-prd-eligibility.md` coloca
precificação fora de escopo e `07-prd-marketplace.md` diz que a plataforma "não
reprecifica". Exibir faixa indicativa exigiria a plataforma estimar preço — o que
contraria ambos. O campo existe no schema para o caso de os financiadores
passarem a publicar tabela de preço; até lá, nulo.

**`byReason` com explicação**: atende ao requisito 2 de `06-prd` ("produzir
justificativa legível para cada exclusão") e ao critério de aceitação "o usuário
entende por que um ativo não pode ser selecionado".

### 3.2 Criação de solicitação

```http
POST /v1/funding-requests
Idempotency-Key: <obrigatório>
X-Representation-Id: rep_01HN4K…
```

```json
{
  "organizationId": "org_01HN4K…",
  "eligibilityEvaluationId": "elg_01HN4K…",
  "assetIds": ["ast_01HN4K…"],
  "requestedAmount": { "currency": "BRL", "amountMinor": 30000000 },
  "invitationPolicy": {
    "mode": "ALL_AUTHORIZED",
    "funderOrganizationIds": [],
    "offerWindowMinutes": 120
  },
  "dataSharingConsentId": "cns_01HN4K…"
}
```

Resposta `201`:

```json
{
  "fundingRequestId": "fdr_01HN4K…",
  "status": "OPEN",
  "version": 1,
  "reservation": {
    "reservationId": "rsv_01HN4K…",
    "expiresAt": "2026-09-16T17:14:00.000Z"
  },
  "invitations": [
    { "funderOrganizationId": "org_funder_1", "status": "SENT", "sentAt": "2026-09-16T15:14:02.000Z" }
  ],
  "offerWindowClosesAt": "2026-09-16T17:14:00.000Z"
}
```

Erros específicos:

| Código | HTTP | Quando |
| --- | --- | --- |
| `ELIGIBILITY_EXPIRED` | 422 | Avaliação não é mais válida |
| `SNAPSHOT_STALE` | 422 | Snapshot ultrapassou o SLA da fonte |
| `CONSENT_INVALID` | 422 | Consentimento ausente, expirado ou fora de escopo |
| `KYB_INCOMPLETE` | 422 | Dossiê abaixo do mínimo exigido |
| `ASSET_ALREADY_RESERVED` | 409 | Ativo em outra contratação na plataforma |
| `NO_ELIGIBLE_FUNDER` | 422 | Nenhum financiador com capacidade compatível |
| `REPRESENTATION_INVALID` | 403 | Representação ausente, expirada ou sem alçada |

**`REPRESENTATION_INVALID` com detalhe de alçada**: expõe o requisito derivado de
`90-modelo-de-dados.md`, 3.1 — a representação pode ser vigente e ainda assim
insuficiente para o valor solicitado.

### 3.3 Comparação de ofertas

```http
GET /v1/funding-requests/{id}/comparable-offers
```

```json
{
  "fundingRequestId": "fdr_01HN4K…",
  "presentationId": "prs_01HN4K…",
  "presentedAt": "2026-09-16T15:20:00.000Z",
  "rankingPolicy": {
    "policyId": "rkp_v3",
    "criteria": ["NET_AMOUNT_DESC", "EFFECTIVE_RATE_ASC"],
    "description": "Ordenado por maior valor líquido; empate por menor taxa efetiva",
    "sponsoredPlacementUsed": false
  },
  "calculationMethod": {
    "methodId": "clc_v2",
    "effectiveRateFormula": "(gross/net)^(30/weightedAvgTermDays) - 1"
  },
  "offers": [
    {
      "offerId": "ofr_01HN4K…",
      "position": 1,
      "funder": { "organizationId": "org_funder_1", "legalName": "…" },
      "netAmount": { "currency": "BRL", "amountMinor": 46800000 },
      "grossAmount": { "currency": "BRL", "amountMinor": 48250000 },
      "totalCostAmount": { "currency": "BRL", "amountMinor": 1450000 },
      "effectiveMonthlyRate": { "value": "0.0198", "scale": 4, "basis": "MONTHLY" },
      "weightedAverageTermDays": 47,
      "pricingBreakdown": {
        "discountAmount": { "currency": "BRL", "amountMinor": 1380000 },
        "feeItems": [ { "code": "TAC", "description": "Tarifa de abertura", "amount": { "currency": "BRL", "amountMinor": 50000 }, "chargedBy": "FUNDER" } ],
        "taxItems": [ { "code": "IOF", "amount": { "currency": "BRL", "amountMinor": 20000 } } ]
      },
      "conditions": { "recourse": false, "coobligation": false, "additionalGuarantees": [] },
      "coveredAssetCount": 842,
      "validUntil": "2026-09-16T19:00:00.000Z",
      "sponsored": false,
      "platformRemunerationDisclosed": { "currency": "BRL", "amountMinor": 23400 },
      "originalDocumentRef": { "evidenceId": "evd_…", "contentHash": "sha256:…" }
    }
  ]
}
```

**`rankingPolicy` e `calculationMethod` na própria resposta**: atendem ao
requisito 6 de `07-prd` ("tornar critérios de ordenação auditáveis"). Documentar
o critério em outro lugar não é equivalente — o cliente vê a ordenação e o
critério juntos, no mesmo instante.

**`originalDocumentRef`**: dá acesso ao documento original do financiador,
cumprindo "sem perda do documento original".

### 3.4 Seleção de oferta

```http
POST /v1/funding-requests/{id}/selection
Idempotency-Key: <obrigatório>
X-Representation-Id: rep_01HN4K…
```

```json
{
  "offerId": "ofr_01HN4K…",
  "presentationId": "prs_01HN4K…",
  "offerVersionHash": "sha256:…",
  "materialConditionsAccepted": true,
  "materialConditionsDocumentHash": "sha256:…"
}
```

**`offerVersionHash` obrigatório**: o cliente devolve o hash da oferta que viu. Se
não corresponder à oferta atual, a resposta é `409 OFFER_CHANGED` — atendendo a
"seleção referencia versão e hash da oferta exibida" e a "bloquear seleção de
oferta expirada ou alterada". Sem isso, o cliente poderia selecionar uma oferta
diferente da que leu, sem que nada detectasse.

Resposta `201` inclui o resultado da revalidação:

```json
{
  "selectionId": "sel_01HN4K…",
  "contractCaseId": "ctr_01HN4K…",
  "status": "CONTRACTING",
  "revalidation": {
    "outcome": "VALID",
    "snapshotId": "snp_01HN4K…",
    "revalidatedAt": "2026-09-16T15:14:30.000Z",
    "checks": [
      { "check": "ASSET_AVAILABILITY", "result": "PASS" },
      { "check": "CONSENT_ACTIVE",     "result": "PASS" },
      { "check": "REPRESENTATION_VALID", "result": "PASS" },
      { "check": "KYB_STATUS",         "result": "PASS" }
    ]
  }
}
```

Se a revalidação falhar: `422` com `outcome: "INVALID"` e a verificação
específica que reprovou.

---

## 4. Partner API

### 4.1 Recepção de convite

```http
GET /v1/partner/invitations?status=PENDING&limit=50
```

O conteúdo devolvido depende do nível de dados autorizado no `PartnerAgreement`
(ver `95-personas-jornadas.md`, P4). Nível de triagem:

```json
{
  "data": [
    {
      "invitationId": "inv_01HN4K…",
      "fundingRequestId": "fdr_01HN4K…",
      "disclosureLevel": "SCREENING",
      "cedent": {
        "organizationId": null,
        "taxIdMasked": "**.***.***/0001-**",
        "segment": "RETAIL",
        "stateCode": "SP",
        "relationshipMonths": 14
      },
      "assets": {
        "assetType": "CARD_RECEIVABLE_UNIT",
        "count": 842,
        "grossTotal": { "currency": "BRL", "amountMinor": 31200000 },
        "weightedAverageTermDays": 47,
        "concentrationByAcquirer": [ { "acquirerId": "acq_1", "share": "0.62" } ]
      },
      "kybSummary": { "status": "READY_FOR_PARTNER", "completenessScore": "1.00", "lastVerifiedAt": "2026-09-10T…" },
      "riskSignals": [ { "code": "NEW_BANK_ACCOUNT_RECENT", "severity": "MEDIUM" } ],
      "expiresAt": "2026-09-16T17:14:00.000Z"
    }
  ]
}
```

**`organizationId` nulo no nível de triagem**: o financiador avalia a operação sem
identificar o cedente. Evita que a plataforma exponha a base de clientes a
concorrentes pela simples circulação de convites. A identificação vem com
`POST /v1/partner/invitations/{id}/disclosure-requests`, registrada como evento.

**`riskSignals` presente, decisão ausente**: mantém o limite de
`02-prd-kyb-kyc.md` — sinal informativo, nunca decisão da plataforma.

### 4.2 Envio de oferta

```http
POST /v1/partner/funding-requests/{id}/offers
Idempotency-Key: <obrigatório>
```

```json
{
  "invitationId": "inv_01HN4K…",
  "requestVersion": 1,
  "pricing": {
    "grossAmount": { "currency": "BRL", "amountMinor": 48250000 },
    "discountAmount": { "currency": "BRL", "amountMinor": 1380000 },
    "feeItems": [],
    "taxItems": [],
    "netAmount": { "currency": "BRL", "amountMinor": 46800000 }
  },
  "coveredAssetIds": ["ast_01HN4K…"],
  "conditions": { "recourse": false, "coobligation": false },
  "validUntil": "2026-09-16T19:00:00.000Z",
  "funderOfferReference": "…",
  "offerDocument": { "contentType": "application/pdf", "contentBase64": "…" }
}
```

Erros específicos:

| Código | HTTP | Quando |
| --- | --- | --- |
| `REQUEST_VERSION_STALE` | 409 | O pedido mudou; reenviar contra a versão atual |
| `REQUEST_CLOSED` | 409 | Janela encerrada ou pedido já contratado |
| `ASSET_NOT_IN_REQUEST` | 422 | Oferta cobre ativo fora do lote |
| `CAPABILITY_NOT_ENABLED` | 403 | Capacidade suspensa ou produto expirado |
| `PRICING_INCONSISTENT` | 422 | `netAmount` não corresponde aos componentes |

**`PRICING_INCONSISTENT`**: a plataforma valida a aritmética sem reprecificar.
Verificar que bruto menos deságio menos tarifas menos tributos resulta no líquido
declarado não é precificar — é impedir que uma oferta internamente inconsistente
seja apresentada como comparável.

**Oferta parcial**: `coveredAssetIds` pode ser subconjunto. Implicação não tratada
em `07-prd`: ofertas de escopos diferentes não são diretamente comparáveis por
valor líquido. A comparação precisa apresentar a cobertura junto do valor, e a
ordenação por valor líquido absoluto seria enganosa entre escopos distintos.

### 4.3 Confirmação de desembolso

```http
POST /v1/partner/funding-instructions/{id}/disbursements
Idempotency-Key: <obrigatório>
```

```json
{
  "amount": { "currency": "BRL", "amountMinor": 46800000 },
  "executedAt": "2026-09-16T17:28:00.000Z",
  "destinationHash": "sha256:…",
  "funderReference": "…",
  "paymentMethod": "PIX",
  "paymentReference": "E12345678202609161728…",
  "proof": { "contentType": "application/pdf", "contentBase64": "…" }
}
```

**`destinationHash` e não a conta**: o financiador confirma para onde pagou sem
que os dados bancários trafeguem novamente. A plataforma compara com o hash do
destino validado. Divergência produz `FundingMismatchDetected`, não erro de API —
o pagamento já ocorreu; recusar a informação não o desfaz.

---

## 5. Webhooks

### 5.1 Recebidos (de infraestruturas)

Formato ditado pelo parceiro. Requisitos da plataforma:

| Requisito | Definição |
| --- | --- |
| Verificação de origem | Assinatura ou mTLS, conforme o parceiro |
| Resposta rápida | Aceitar, persistir o bruto, enfileirar; responder em menos de 200 ms (p95) |
| Idempotência | Deduplicação pelo identificador do parceiro; repetição retorna sucesso |
| Preservação | Payload bruto persistido antes de qualquer interpretação |
| Ordem | Nunca presumida; a ordem vem do conteúdo, não da chegada |
| Rejeição | Somente por falha de autenticação. Payload malformado é aceito, persistido e encaminhado a tratamento — rejeitar faz o parceiro reenviar indefinidamente |

### 5.2 Enviados (ao cedente)

```json
{
  "webhookId": "whk_01HN4K…",
  "eventType": "DisbursementConfirmed.v1",
  "occurredAt": "2026-09-16T17:30:00.000Z",
  "organizationId": "org_01HN4K…",
  "data": {},
  "signature": "…"
}
```

- Assinatura sobre o corpo bruto, com segredo por organização.
- Repetição até 24 h, com espera crescente.
- Endpoint que falha persistentemente é desativado, com notificação.
- Sem `PII_SENSITIVE` no corpo; referências resolvidas por API autenticada.

---

## 6. Limites e cotas

| Superfície | Limite padrão | Escopo |
| --- | --- | --- |
| Client API | 600 req/min | Por usuário |
| Client API — descoberta | 6 req/hora | Por organização (ver RNF-V2) |
| Partner API — leitura | 1.200 req/min | Por parceiro |
| Partner API — escrita | 300 req/min | Por parceiro |
| Webhooks recebidos | Sem limite | Nunca limitar parceiro; absorver e enfileirar |

**Descoberta limitada a 6 por hora**: cada descoberta consome consulta paga à
infraestrutura e pode gerar snapshot. Sem limite, um cliente com atualização
automática consome custo de forma desproporcional. O limite é configurável por
`PartnerAgreement` e organização.

## 7. Documentação e conformidade

| Requisito | Definição |
| --- | --- |
| Especificação | OpenAPI 3.1 por superfície, gerada do código, não mantida à parte |
| Ambiente de teste | Partner API com sandbox e dados sintéticos |
| Exemplos | Requisição e resposta por rota, validados em CI |
| Compatibilidade | Teste que compara a especificação da versão anterior e falha em mudança incompatível não versionada |
| Changelog | Por superfície, com data de descontinuação quando aplicável |

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| A1 | Formato do documento de oferta (PDF? estruturado? ambos?) | Médio — afeta a comparabilidade |
| A2 | Níveis de divulgação precisam de validação jurídica quanto à LGPD | Alto |
| A3 | Comparação entre ofertas de escopos diferentes (4.2) | Alto — afeta a experiência central |
| A4 | Contrato de webhook para o cedente não validado com cliente real | Baixo |

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 1 | `07-prd-marketplace.md` | Superfícies não distinguidas |
| 2.2 | `04-prd-receivables-gateway.md` | Idempotência exigida sem mecanismo na API |
| 2.4 | `04`, `09` | `UNKNOWN_OUTCOME` sem representação em HTTP |
| 2.5 | `01-prd-identity-organization.md` | Representação sem canal na API |
| 3.1 | `06-prd-eligibility.md` | Simulação exigida sem contrato |
| 3.3 | `07-prd-marketplace.md` | Ordenação auditável sem exposição do critério |
| 3.4 | `07-prd-marketplace.md` | Hash da oferta exigido sem mecanismo |
| 4.1 | `07-prd-marketplace.md` | Escopo de dados por convite |
| 4.3 | `10-prd-funding.md` | Confirmação sem trafegar dados bancários |
