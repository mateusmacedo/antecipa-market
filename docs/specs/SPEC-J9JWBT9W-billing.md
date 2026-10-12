---
id: SPEC-J9JWBT9W
slug: billing
title: Bounded context — billing
stage: backlog
priority: P3
depends_on: [SPEC-GXGASP24, SPEC-CWYGNY82]
ticket_url: https://linear.app/mmda/issue/DEVS-93/spec-j9jwbt9w-bounded-context-billing
subtask_urls: []
created: 2026-10-12
---

# SPEC-J9JWBT9W: Bounded context — billing

## Identidade

- **bounded_context**: `billing`
- **name**: `billing`
- **Linguagem ubíqua**:
  - *Plano de preços* (`PricingPlan`) — plano versionado: modelo, base de cálculo, alíquotas, faixas e vigência.
  - *Atribuição de plano* (`PlanAssignment`) — vínculo de um plano a uma organização pagadora, com vigência.
  - *Fato gerador* (`BillableEvent`) — o fato registrado de que nasce a cobrança, com a base de cálculo congelada.
  - *Cobrança* (`Charge`) — valor calculado, com valor do serviço, tributos retidos e tributos incidentes em campos separados.
  - *Fatura* (`Invoice`) e *linha* (`InvoiceLine`) — agrupamento por período e pagador; cada linha aponta para a operação de origem.
  - *Regra tributária* (`TaxRule`) — regra por natureza do serviço e localidade.
  - *Pagador* — a organização que paga: cedente, financiador ou ambos.
  - *Base congelada* — o valor de referência capturado no fato gerador; o estorno é `REVERSED` com registro, nunca recálculo silencioso (18 §3.1).
  - *Extrato* — a consulta do que foi cobrado, por operação (18 §6).
  - As decisões comerciais D1 a D3 (`#7`) ainda não foram tomadas; o modelo parte do desenho recomendado e cada ponto que depende delas leva **[A CONFIRMAR: #7, D1, D2, D3]**. Os valores enumerados do design são preservados literalmente.

## Origem e bloqueios externos

- **Fontes**: `docs/design/18-design-billing.md` §1 a §8 (contexto sem PRD); `docs/prd/13-prd-partner-management.md` e `docs/design/13-design-partner-management.md` §3 (`CommercialRule`); `docs/design/90-modelo-de-dados.md` §3.3 (`platformRemuneration`); `docs/design/92-schemas-eventos.md` §3.3 e §3.5; `docs/design/07-design-marketplace.md` (R5, R6); `docs/design/93-requisitos-nao-funcionais.md` §11; `docs/resumo-executivo-apuracao.md` (D1 a D3, A16).
- **Situação**: `bloqueado` — `#7` (D1 a D3; 18:G1 a G3) trava o contexto inteiro, e 18 §8 diz que ele "não deve ser construído antes". O modelo abaixo vem do desenho recomendado (fato gerador = confirmação do desembolso, 18 §2.3) para a spec ficar pronta quando a decisão sair; não autoriza construir antes dela.
- **Fase de entrega**: 3 (inferida).
- **Dependências**: upstream duro `funding` (`SPEC-CWYGNY82`: `DisbursementConfirmed`, o fato gerador recomendado). Upstream brando: `partner` (`CommercialRule`), `identity` (pagador) e `effects` (`EffectConfirmed`, se o fato gerador for o efeito). `depends_on` declara só a fundação e a aresta dura reduzida (`funding`); as arestas de retroalimentação não entram. O `marketplace` consome uma consulta deste contexto (`platformRemuneration`) e é aresta de retroalimentação.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| `#7`, D1, 18:G1 | Quem paga: cedente, financiador, ambos, assinatura ou misto | `PricingPlan.model`; `PlanAssignment`; `Charge` |
| `#7`, D2, 18:G2 | Base de cálculo: face, líquido, custo da operação, fixo ou escalonado | `PricingPlan.calculationBasis`; `BillableEvent`; `CalculateCharge` |
| `#7`, D3, 18:G3 | Fato gerador: apresentação, seleção, assinatura, efeito, desembolso ou liquidação | `PricingPlan.triggerEvent`; consumo de eventos |
| A16, 18:G4 | Enquadramento tributário: natureza do serviço, município, retenções, obrigação acessória, financiador pagador | `TaxRule`; componentes de `Charge` |
| B17, C3, 18:G5 | Custos unitários dos provedores desconhecidos | Margem por operação (fora desta versão) |
| 18:G6 | Integração com emissão fiscal não definida | Fora desta versão |
| `#1`, A1 | Autorização do BCB (existencial, transversal) | Contexto inteiro |

- **Decisões de produto abertas**: D1 a D3 (só o negócio decide); se a comissão variar por parceiro, a divulgação por oferta e a ordenação sem consideração comercial registrada em `OffersPresented` (controle R5 de 18 §2.1) são condição, não opção.
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída e D1 a D3 decididas e registradas (`#7`). A16 precisa estar resolvida, ao menos no enquadramento do serviço, antes de `TaxRule` e `Charge` saírem de `planning`. `SPEC-CWYGNY82` (funding) precisa publicar `DisbursementConfirmed`.

## Agregados

Nota de modelagem: `tenantId` não é campo dos agregados — o escopo vem do `ExecutionContext` (ADR-051). Valor monetário é `amountMinor: int` mais `currency: string` (90 §1.2); percentual e taxa são `decimal`. O doc 90 não detalha este contexto; os campos são o mínimo derivado de 18 §3 a §6.

### PricingPlan

- **Identidade**: `generated`
- **Campos**:
  - `version: int`, `supersedesId: id` (opcional) — versionamento explícito (90 §1.5)
  - `model: CEDENT_PAYS | FUNDER_PAYS | BOTH | SUBSCRIPTION | MIXED` — as cinco opções de 18 §2.1 **[A CONFIRMAR: #7, D1]**
  - `calculationBasis: FACE_VALUE | NET_DISBURSED | OPERATION_COST | FIXED_PER_OPERATION | VOLUME_TIERED` — 18 §2.2 **[A CONFIRMAR: #7, D2]**
  - `triggerEvent: string` — a recomendação é `DisbursementConfirmed` (18 §2.3) **[A CONFIRMAR: #7, D3]**
  - `rates: list<PricingRate>` — coleção indispensável: cada faixa com `rate: decimal` ou `fixedAmountMinor: int` e, em `VOLUME_TIERED`, os limites da faixa; os valores não existem **[A CONFIRMAR: #7, D1, D2]**
  - `commercialRuleId: id` (opcional) — **[ABERTA: 13:§3 × 18:§3 — `CommercialRule` (modelo, pagador, base, taxas, evento) e `PricingPlan` carregam os mesmos atributos; o dono da taxa não é definido]**
  - `effectiveFrom: instant`, `effectiveUntil: instant` (opcional)
- **Estados**: nenhum — nova redação do plano é nova versão.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - `rates` não vazia — `billing/pricing-plan/rates-missing`.
  - `effectiveUntil`, quando presente, é posterior a `effectiveFrom` — `billing/pricing-plan/period-invalid`.
  - Nenhuma cobrança existe fora do plano vigente (18 §6); `SUBSCRIPTION` e `MIXED` exigem fato gerador periódico, que 18 não define e esta versão não modela **[A CONFIRMAR: #7, D1]**.

### PlanAssignment

- **Identidade**: `generated`
- **Campos**:
  - `planId: id`, `payerOrganizationId: id`
  - `effectiveFrom: instant`, `effectiveUntil: instant` (opcional)
  - `status: active | ended`
- **Estados**: `active`, `ended`.
- **Transições permitidas**:
  - (novo) → `active` — por `AssignPricingPlan`
  - `active` → `ended` — por `EndPlanAssignment`
- **Invariantes de estado**:
  - O pagador é obrigatório — `billing/plan-assignment/payer-missing`; quem paga depende de D1 **[A CONFIRMAR: #7, D1]**.
  - Só `active` é encerrável — `billing/plan-assignment/not-active`.

### BillableEvent

- **Identidade**: `natural` por `sourceEventId`
- **Campos**:
  - `sourceEventId: string` — `id` do CloudEvent que originou o fato
  - `payerOrganizationId: id`, `planAssignmentId: id`
  - `calculationBasisMinor: int`, `calculationBasisCurrency: string` — base congelada **[A CONFIRMAR: #7, D2]**
  - `contractId: id`, `effectId: id` (opcional), `disbursementId: id` (opcional) — as referências da operação
  - `occurredAt: instant`
  - `status: PENDING | CHARGED | WAIVED | REVERSED`
  - `waiverReason: string` (opcional), `reversalReason: string` (opcional), `reversedAt: instant` (opcional)
- **Estados**: os quatro de `status`.
- **Transições permitidas**:
  - (novo) → `PENDING` — por `RegisterBillableEvent`
  - `PENDING` → `CHARGED` — por `MarkBillableEventCharged`
  - `PENDING` → `WAIVED` — por `WaiveBillableEvent` (isenção ou franquia)
  - `PENDING` | `CHARGED` → `REVERSED` — por `ReverseBillableEvent`
- **Invariantes de estado**:
  - A base é congelada: nenhum comando a recalcula; se o desembolso é estornado, o tratamento é `REVERSED` com registro (18 §3.1).
  - Base não negativa — `billing/billable-event/basis-invalid`.
  - Sem atribuição de plano vigente não há fato gerador (nada se cobra fora do plano) — `billing/billable-event/assignment-missing`.
  - O estorno de cobrança já faturada **[ABERTA: 18:§3.1 — a fatura emitida não muda, e o design só diz `REVERSED` com registro; falta o tratamento na fatura seguinte]**.

### Charge

- **Identidade**: `generated`
- **Campos**:
  - `billableEventId: id`, `payerOrganizationId: id`
  - `planId: id`, `planVersion: int`, `taxRuleId: id` (opcional)
  - `serviceAmountMinor: int`, `withheldTaxAmountMinor: int`, `incidentTaxAmountMinor: int`, `currency: string` — componentes separados (18 §5)
  - `calculatedAt: instant`
- **Estados**: nenhum — a cobrança é imutável; a correção é estorno do fato gerador.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Um valor único não basta: serviço, tributos retidos e tributos incidentes são campos próprios, para o documento fiscal sair correto e o cliente poder escriturar (18 §5).
  - Componentes não negativos — `billing/charge/component-negative`.
  - O cálculo exige a regra tributária aplicável — `billing/charge/tax-rule-missing` **[A CONFIRMAR: A16, 18:G4]**.
  - Arredondamento do cálculo sobre `amountMinor` **[A CONFIRMAR: #7, D2]**.

### Invoice

- **Identidade**: `natural` por `invoiceKey` (`payerOrganizationId` e o período)
- **Campos**:
  - `payerOrganizationId: id`, `periodFrom: instant`, `periodTo: instant`
  - `lines: list<InvoiceLine>` — coleção indispensável: cada linha com `chargeId: id`, `contractId: id`, `serviceAmountMinor: int`, `withheldTaxAmountMinor: int`, `incidentTaxAmountMinor: int`
  - `currency: string`
  - `status: OPEN | ISSUED`
  - `issuedAt: instant` (opcional)
- **Estados**: `OPEN` (acumulando), `ISSUED`. **[ABERTA: 18:§4 — o design nomeia "fechamento e emissão", "cobrança" e "conciliação do recebimento" sem estados; só os dois primeiros têm modelo aqui]**
- **Transições permitidas**:
  - (novo) → `OPEN` — por `AccumulateCharge`
  - `OPEN` → `ISSUED` — por `IssueInvoice`
- **Invariantes de estado**:
  - Fatura emitida não muda depois (18 §3.1) — `billing/invoice/already-issued`.
  - A mesma cobrança não entra duas vezes — `billing/invoice/duplicate-charge`.
  - Toda linha aponta para a operação de origem (18 §6) — `billing/invoice/line-untraceable`.
  - Fatura sem linhas não é emitida — `billing/invoice/empty`.

### TaxRule

- **Identidade**: `generated`
- **Campos**:
  - `serviceNature: string`, `jurisdiction: string` — natureza do serviço e município de incidência
  - `rate: decimal`, `withholdingApplies: bool`
  - `appliesToPayer: CEDENT | FUNDER | ANY` — o serviço prestado a instituição financeira pode ter regra específica (18 §5)
  - `effectiveFrom: instant`, `effectiveUntil: instant` (opcional)
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Natureza e município presentes — `billing/tax-rule/identification-missing`.
  - Alíquota não negativa — `billing/tax-rule/rate-negative`.
  - Todo o conteúdo depende de apuração externa **[A CONFIRMAR: A16, 18:G4]**.

## Comandos (UPRs)

### DefinePricingPlan

- **Agregado**: `PricingPlan`; **cria**
- **Entrada**: `version: int`; `supersedesId: id`; `model`; `calculationBasis`; `triggerEvent: string`; `rates: list<PricingRate>`; `commercialRuleId: id`; `effectiveFrom: instant`; `effectiveUntil: instant`
- **Pré-condições**:
  - `rates` com ao menos uma faixa → `billing/pricing-plan/rates-missing`
  - `effectiveUntil` ausente ou posterior a `effectiveFrom` → `billing/pricing-plan/period-invalid`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `planId: id`

### AssignPricingPlan

- **Agregado**: `PlanAssignment`; **cria**
- **Entrada**: `planId: id`; `payerOrganizationId: id`; `effectiveFrom: instant`; `effectiveUntil: instant`
- **Pré-condições**:
  - `payerOrganizationId` presente → `billing/plan-assignment/payer-missing`
- **Efeitos sobre o estado**: `status ← active`; demais campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `planAssignmentId: id`

### EndPlanAssignment

- **Agregado**: `PlanAssignment`; exige existente
- **Entrada**: `effectiveUntil: instant`
- **Pré-condições**:
  - `status = active` → `billing/plan-assignment/not-active`
- **Efeitos sobre o estado**: `status ← ended`; `effectiveUntil ← cmd.effectiveUntil`
- **Eventos emitidos**: nenhum
- **Resposta**: `planAssignmentId: id`

### RegisterBillableEvent

- **Agregado**: `BillableEvent`; **inicializa se ausente**
- **Entrada**: `sourceEventId: string`; `payerOrganizationId: id`; `planAssignmentId: id`; `calculationBasisMinor: int`; `calculationBasisCurrency: string`; `contractId: id`; `effectId: id`; `disbursementId: id`; `occurredAt: instant`
- **Pré-condições**:
  - `planAssignmentId` presente → `billing/billable-event/assignment-missing`
  - `calculationBasisMinor ≥ 0` → `billing/billable-event/basis-invalid`
- **Efeitos sobre o estado**: quando ausente: `status ← PENDING`; demais campos ← entrada. Quando presente: nenhum — aceita sem alterar e sem emitir.
- **Eventos emitidos**: nenhum
- **Resposta**: `billableEventId: string`

### WaiveBillableEvent

- **Agregado**: `BillableEvent`; exige existente
- **Entrada**: `waiverReason: string`
- **Pré-condições**:
  - `status = PENDING` → `billing/billable-event/not-pending`
- **Efeitos sobre o estado**: `status ← WAIVED`; `waiverReason ← cmd.waiverReason`
- **Eventos emitidos**: nenhum
- **Resposta**: `billableEventId: string`

### MarkBillableEventCharged

- **Agregado**: `BillableEvent`; exige existente
- **Entrada**: `chargeId: id`
- **Pré-condições**:
  - `status = PENDING` → `billing/billable-event/not-pending`
- **Efeitos sobre o estado**: `status ← CHARGED`
- **Eventos emitidos**: nenhum
- **Resposta**: `billableEventId: string`

### ReverseBillableEvent

- **Agregado**: `BillableEvent`; exige existente
- **Entrada**: `reversalReason: string`
- **Pré-condições**:
  - `status` em `PENDING` ou `CHARGED` → `billing/billable-event/not-reversible`
- **Efeitos sobre o estado**: `status ← REVERSED`; `reversalReason ← cmd.reversalReason`; `reversedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `billableEventId: string`

### CalculateCharge

- **Agregado**: `Charge`; **cria**
- **Entrada**: `billableEventId: id`; `payerOrganizationId: id`; `planId: id`; `planVersion: int`; `rates: list<PricingRate>` e `calculationBasis` (instantâneo do plano, resolvido pelo caso de uso); `calculationBasisMinor: int`; `currency: string`; `taxRuleId: id`; `tax` (instantâneo da `TaxRule`)
- **Pré-condições**:
  - `taxRuleId` presente → `billing/charge/tax-rule-missing`
  - componentes calculados não negativos → `billing/charge/component-negative`
- **Efeitos sobre o estado**: campos ← entrada; `serviceAmountMinor ←` aplicação da regra do plano sobre a base congelada **[A CONFIRMAR: #7, D2]**; `withheldTaxAmountMinor` e `incidentTaxAmountMinor ←` aplicação da `TaxRule` **[A CONFIRMAR: A16, 18:G4]**; `calculatedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `chargeId: id`

### AccumulateCharge

- **Agregado**: `Invoice`; **inicializa se ausente**
- **Entrada**: `payerOrganizationId: id`; `periodFrom: instant`; `periodTo: instant`; `chargeId: id`; `contractId: id`; `serviceAmountMinor: int`; `withheldTaxAmountMinor: int`; `incidentTaxAmountMinor: int`; `currency: string`
- **Pré-condições**:
  - `status = OPEN` → `billing/invoice/already-issued`
  - `chargeId` ainda fora das linhas → `billing/invoice/duplicate-charge`
  - `contractId` presente → `billing/invoice/line-untraceable`
- **Efeitos sobre o estado**: quando ausente: `status ← OPEN`; campos de identidade ← entrada. Em ambos os casos: acrescenta a linha à coleção `lines`.
- **Eventos emitidos**: nenhum
- **Resposta**: `invoiceKey: string`

### IssueInvoice

- **Agregado**: `Invoice`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status = OPEN` → `billing/invoice/already-issued`
  - `lines` não vazia → `billing/invoice/empty`
- **Efeitos sobre o estado**: `status ← ISSUED`; `issuedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `invoiceKey: string`

### DefineTaxRule

- **Agregado**: `TaxRule`; **cria**
- **Entrada**: `serviceNature: string`; `jurisdiction: string`; `rate: decimal`; `withholdingApplies: bool`; `appliesToPayer`; `effectiveFrom: instant`; `effectiveUntil: instant`
- **Pré-condições**:
  - `serviceNature` e `jurisdiction` presentes → `billing/tax-rule/identification-missing`
  - `rate ≥ 0` → `billing/tax-rule/rate-negative`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `taxRuleId: id`

## Eventos de domínio

Este contexto não publica eventos de domínio: nem `14` nem `92` §4 listam evento de faturamento, e 18 o descreve como consumidor que "não participa do fluxo de operação" (18 §4). Por isso não há `.proto` de evento, `Outbox` nem tópico próprio **[ABERTA: 92:§4 — eventos de cobrança (por exemplo, fatura emitida para Notificações e ledger) não são definidos]**.

## Consultas

Todas correm fora da UoW.

- `FindPricingPlan` — filtros: `planId`; cardinalidade: `one`
- `FindPlanAssignmentByPayer` — filtros: `payerOrganizationId`, `asOf`; cardinalidade: `one` (a atribuição `active` vigente em `asOf`)
- `FindBillableEvent` — filtros: `sourceEventId`; cardinalidade: `one`
- `FindBillableEventByOperation` — filtros: `disbursementId`, `effectId` ou `contractId`; cardinalidade: `many`
- `FindChargeByPayer` — filtros: `payerOrganizationId`, `calculatedFrom`, `calculatedTo`; cardinalidade: `many` (o extrato por operação, 18 §6)
- `FindInvoice` — filtros: `invoiceKey`; cardinalidade: `one`
- `FindInvoiceByPayer` — filtros: `payerOrganizationId`, `periodFrom`, `periodTo`; cardinalidade: `many`
- `FindTaxRule` — filtros: `serviceNature`, `jurisdiction`, `asOf`; cardinalidade: `one`
- `QuotePlatformRemuneration` — filtros: `payerOrganizationId`, `calculationBasisMinor`, `calculationBasisCurrency`, `asOf`; cardinalidade: `one`. Cálculo puro, sem estado: devolve a remuneração em valor, com o percentual como acompanhante e nunca como substituto (18 §6). É o que alimenta `platformRemuneration` de `PricingBreakdown` (90 §3.3) **[A CONFIRMAR: #7, D1, D2]**.

## Relações entre agregados

- `PlanAssignment.planId` → `PricingPlan`
- `PlanAssignment.payerOrganizationId` → organização (`identity`, só identidade)
- `PricingPlan.commercialRuleId` → `CommercialRule` (`partner`, só identidade)
- `BillableEvent.planAssignmentId` → `PlanAssignment`
- `BillableEvent.contractId`, `effectId` e `disbursementId` → contrato, efeito e desembolso (outros contextos, só identidade)
- `Charge.billableEventId` → `BillableEvent`
- `Charge.planId` → `PricingPlan`
- `Charge.taxRuleId` → `TaxRule`
- `Invoice.lines[].chargeId` → `Charge`
- `Invoice.payerOrganizationId` → organização (`identity`, só identidade)
- A cadeia fato gerador → cobrança → fatura é um comando por UoW, encadeada pelo caso de uso; cada comando é idempotente.

## Integração

- **Publica**: nenhum evento (ver "Eventos de domínio").
- **Consome** (inbox e Kafka, D4 da fundação; disposições de consumo de FND-04 §6.4), segundo o desenho recomendado **[A CONFIRMAR: #7, D3]**:
  - `company.funding.event.v1.DisbursementConfirmed` → `RegisterBillableEvent`: `id` → `sourceEventId`; `confirmedAt` → `occurredAt`; `contractId`, `effectId` e `disbursementId` → referências da operação; `amount.amountMinor` e `amount.currency` → `calculationBasisMinor` e `calculationBasisCurrency` quando a base é `NET_DISBURSED` **[A CONFIRMAR: #7, D2]**; `planAssignmentId` resolvido pelo caso de uso com `FindPlanAssignmentByPayer`. O pagador vem de `funderOrganizationId` quando o financiador paga; o payload não traz a organização do cedente **[ABERTA: 92:§3.5 — a organização cedente não está em `DisbursementConfirmed`; consulta a `contracting`]**. A base `FACE_VALUE` exige o valor de face, que o evento também não traz **[A CONFIRMAR: #7, D2]**.
  - `company.funding.event.v1.DisbursementReversed` → `ReverseBillableEvent`: localiza o fato gerador por `disbursementId` (`FindBillableEventByOperation`) e registra o estorno, sem recálculo (18 §3.1).
  - Alternativa, se D3 escolher o efeito como fato gerador: `company.effects.event.v1.EffectConfirmed` → `RegisterBillableEvent` **[A CONFIRMAR: #7, D3]**.
  - **Inconsistência registrada**: `92` §4 não lista o `billing` como consumidor de `DisbursementConfirmed` nem de `DisbursementReversed`, e 18 §2.3 e §3.1 os exigem **[ABERTA: 92:§4 × 18:§2.3 — consumidores de Funding]**.
- **Consultas gRPC a outros contextos**: `partner` (`CommercialRule` do parceiro), `identity` (organização pagadora) e `contracting` (valor de face, se a base for `FACE_VALUE`); os contratos são os das specs desses contextos.
- **Consultas gRPC servidas**: `QuotePlatformRemuneration`, ao `marketplace`, para divulgar a remuneração em valor por oferta (18 §6). O isolamento de 18 §4 diz que falha de faturamento nunca bloqueia contratação, e a divulgação é obrigatória **[ABERTA: 18:§4 × 07 — comportamento de `platformRemuneration` quando o billing está indisponível]**.

## Borda e persistência

- **Borda**: só gRPC, `company.billing.service.v1.BillingService`, um RPC por comando (11) e por consulta (9), atrás da cadeia de interceptors do kernel. O extrato do cliente e a divulgação no contrato são do `bff` e do `contracting`, e ficam no Escopo fora.
- **Banco**: database e role `billing`, schema `public`, tabelas `pricing_plans`, `plan_assignments`, `billable_events`, `charges`, `invoices`, `tax_rules` e `inbox` (kernel; o contexto consome e não publica, então pede `Inbox` e não `Outbox`), com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra:
  - `plan_assignments(payer_organization_id, effective_from)` — `FindPlanAssignmentByPayer`
  - `billable_events(disbursement_id, effect_id, contract_id, status)` — `FindBillableEventByOperation`; o `sourceEventId` é a chave natural
  - `charges(payer_organization_id, calculated_at)` — `FindChargeByPayer`
  - `invoices(payer_organization_id, period_from)` — `FindInvoiceByPayer`; as linhas ficam no `snapshot`
  - `tax_rules(service_nature, jurisdiction, effective_from)` — `FindTaxRule`
  - O resto vai em `snapshot` `jsonb`.
- **Divergências do design**: sem RLS como mecanismo primário (`tenant_id` e `ExecutionContext`, ADR-051); `instant` é inteiro de nanossegundos (o doc 00 §8 fala em milissegundos); dinheiro é inteiro em unidade menor, sem ponto flutuante (90 §1.2).
- **Isolamento** (18 §4): o contexto só consome eventos e atende consultas; a falha dele não participa do fluxo de contratação nem de desembolso.

## Políticas transversais

- **Idempotência**: os comandos correm em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); o consumo é idempotente por `id` do CloudEvent (inbox, ADR-036; FND-04 §6.4); `BillableEvent` tem identidade natural por `sourceEventId`, e a repetição do evento não cobra duas vezes.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). O cliente consulta o próprio extrato; planos e regras tributárias são de plataforma, e a matriz de 95 não tem papel de faturamento **[ABERTA: 95 matriz — papel que administra planos e regras tributárias]**.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`). Mudança de plano e estorno de fato gerador não têm evento para o ledger **[ABERTA: 92:§4 — registro de planos, estornos e faturas no ledger]**.
- **Transparência** (18 §6): remuneração divulgada em valor na apresentação e no contrato; extrato por operação; toda linha de fatura aponta para a operação de origem; nenhuma cobrança fora do plano vigente. `disclosureRequired` é sempre verdadeiro (13 §3).
- **Base congelada**: o valor de referência é capturado no fato gerador e não é recalculado; a fatura emitida não muda.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `billing` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários, válidos para o desenho recomendado e sujeitos a revisão quando `#7` decidir **[A CONFIRMAR: #7, D1, D2, D3]**:

```text
DADO um plano com ao menos uma faixa
QUANDO DefinePricingPlan chega com triggerEvent DisbursementConfirmed
ENTÃO o plano nasce com a versão informada

DADO qualquer entrada
QUANDO DefinePricingPlan chega sem faixas
ENTÃO rejeita com billing/pricing-plan/rates-missing e nada é escrito

DADO qualquer entrada
QUANDO DefinePricingPlan chega com effectiveUntil igual ou anterior a effectiveFrom
ENTÃO rejeita com billing/pricing-plan/period-invalid

DADO um plano e uma organização pagadora
QUANDO AssignPricingPlan chega
ENTÃO a atribuição nasce active

DADO qualquer entrada
QUANDO AssignPricingPlan chega sem payerOrganizationId
ENTÃO rejeita com billing/plan-assignment/payer-missing

DADO uma atribuição active
QUANDO EndPlanAssignment chega com effectiveUntil
ENTÃO a atribuição vai a ended

DADO uma atribuição já ended
QUANDO EndPlanAssignment chega
ENTÃO rejeita com billing/plan-assignment/not-active

DADO DisbursementConfirmed consumido pela primeira vez e uma atribuição vigente
QUANDO RegisterBillableEvent chega com a base congelada
ENTÃO o fato gerador nasce PENDING com a base informada e nada é emitido

DADO um fato gerador já registrado para o mesmo sourceEventId
QUANDO RegisterBillableEvent chega de novo
ENTÃO aceita sem alterar o fato gerador, e nenhuma cobrança nova é criada

DADO qualquer entrada
QUANDO RegisterBillableEvent chega sem planAssignmentId
ENTÃO rejeita com billing/billable-event/assignment-missing

DADO qualquer entrada
QUANDO RegisterBillableEvent chega com base negativa
ENTÃO rejeita com billing/billable-event/basis-invalid

DADO um fato gerador PENDING
QUANDO WaiveBillableEvent chega com waiverReason
ENTÃO o fato vai a WAIVED

DADO um fato gerador CHARGED
QUANDO WaiveBillableEvent ou MarkBillableEventCharged chega
ENTÃO rejeita com billing/billable-event/not-pending

DADO um fato gerador PENDING
QUANDO MarkBillableEventCharged chega com chargeId
ENTÃO o fato vai a CHARGED

DADO um fato gerador CHARGED
QUANDO ReverseBillableEvent chega com reversalReason
ENTÃO o fato vai a REVERSED com reversedAt = at e a base permanece a original

DADO um fato gerador WAIVED
QUANDO ReverseBillableEvent chega
ENTÃO rejeita com billing/billable-event/not-reversible

DADO um fato gerador PENDING, um plano e uma regra tributária
QUANDO CalculateCharge chega
ENTÃO a cobrança nasce com serviço, tributos retidos e tributos incidentes em campos separados e calculatedAt = at

DADO qualquer entrada
QUANDO CalculateCharge chega sem taxRuleId
ENTÃO rejeita com billing/charge/tax-rule-missing

DADO qualquer entrada
QUANDO CalculateCharge produz um componente negativo
ENTÃO rejeita com billing/charge/component-negative

DADO nenhuma fatura aberta para o pagador no período
QUANDO AccumulateCharge chega com uma cobrança e contractId
ENTÃO a fatura nasce OPEN com uma linha rastreável ao contrato

DADO uma fatura OPEN que já contém a cobrança
QUANDO AccumulateCharge chega com a mesma chargeId
ENTÃO rejeita com billing/invoice/duplicate-charge

DADO qualquer entrada
QUANDO AccumulateCharge chega sem contractId
ENTÃO rejeita com billing/invoice/line-untraceable

DADO uma fatura ISSUED
QUANDO AccumulateCharge ou IssueInvoice chega
ENTÃO rejeita com billing/invoice/already-issued e a fatura não muda

DADO uma fatura OPEN com linhas
QUANDO IssueInvoice chega
ENTÃO a fatura vai a ISSUED com issuedAt = at

DADO uma fatura OPEN sem linhas
QUANDO IssueInvoice chega
ENTÃO rejeita com billing/invoice/empty

DADO natureza do serviço e município
QUANDO DefineTaxRule chega com alíquota não negativa
ENTÃO a regra nasce com a vigência informada

DADO qualquer entrada
QUANDO DefineTaxRule chega sem município
ENTÃO rejeita com billing/tax-rule/identification-missing

DADO qualquer entrada
QUANDO DefineTaxRule chega com alíquota negativa
ENTÃO rejeita com billing/tax-rule/rate-negative

DADO DisbursementReversed consumido para um desembolso já cobrado
QUANDO a inbox o recebe pela primeira vez
ENTÃO ReverseBillableEvent roda sobre o fato gerador do desembolso e a recepção termina como processada

DADO um pagador com atribuição vigente
QUANDO QuotePlatformRemuneration consulta uma base de cálculo
ENTÃO devolve a remuneração em valor, fora de qualquer UoW e sem alterar estado
```

## Escopo fora

- **Construção antes de `#7`**: 18 §8 veda construir o contexto antes de G1 a G3; um motor de cobrança genérico para acomodar qualquer modelo é o caminho mais caro e o menos provável de acertar.
- **REST do `bff`**: o extrato do cliente e as telas de planos e faturas ficam no `bff`; este contexto serve só gRPC.
- **Cobrança e conciliação do recebimento** (18 §4): o design as nomeia sem desenho, e a plataforma não toca recurso de terceiros **[ABERTA: 18:§4 — como a fatura é paga e conciliada]**.
- **Emissão de documento fiscal** (18:G6): integração com emissão fiscal não definida; o contexto só separa os componentes para que ela seja possível.
- **Custo por operação e margem** (18 §7; 18:G5, B17, C3): depende dos custos unitários dos provedores e de eventos de custo que nenhum contexto publica.
- **Planos `SUBSCRIPTION` e `MIXED` com fato gerador periódico**: dependem de D1 (`#7`).
- **Divulgação no contrato e ordenação das ofertas**: são do `contracting` e do `marketplace`; este contexto só fornece a remuneração.
- **Tributação definitiva**: depende de A16 (18:G4); as regras entram como dado, não como código.
