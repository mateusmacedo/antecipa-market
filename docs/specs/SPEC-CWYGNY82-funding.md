---
id: SPEC-CWYGNY82
slug: funding
title: Bounded context — funding
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-ZFDEPFWC]
ticket_url: https://linear.app/mmda/issue/DEVS-91/spec-cwygny82-bounded-context-funding
subtask_urls: []
created: 2026-10-12
---

# SPEC-CWYGNY82: Bounded context — funding

## Identidade

- **bounded_context**: `funding`
- **name**: `funding` (módulo `apps/backend/funding`, package Protobuf `company.funding.event.v1`)
- **Linguagem ubíqua**:
  - *Instrução de desembolso* (`FundingInstruction`) - autorização lógica, vinculada ao contrato e ao efeito, para o financiador pagar o cedente.
  - *Desembolso* (`Disbursement`) - execução informada pelo financiador; a plataforma registra o fato, nunca executa a transferência (10 §2).
  - *Destino* (`DisbursementDestination`) - conta validada do cedente; nunca uma conta da plataforma.
  - *Comprovante* (`FundingProof`) - registro do que o financiador informou, não atestado da plataforma (10 §7).
  - *Divergência* (`FundingMismatch`) - diferença de valor, destino ou situação entre o ordenado e o confirmado.
  - *Quarentena* - prazo em que conta nova ou alterada não recebe desembolso (94 C2.2).
  - *Resultado desconhecido* (`UNKNOWN_OUTCOME`) - não se sabe se o pedido ao financiador surtiu efeito; consulta, nunca reenvio.
  - `10:Dn` - lacunas locais de `docs/design/10-design-funding.md` (D1 a D11); não são as decisões D1 a D10 do doc 00.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/10-prd-funding.md`; `docs/design/10-design-funding.md` §2 a §9; `docs/design/90-modelo-de-dados.md` §1.4 e §4; `docs/design/92-schemas-eventos.md` §3.5 e §4; `docs/design/91-contratos-api.md` §4.3; `docs/design/94-seguranca-threat-model.md` T2 (C2.1 a C2.7) e T9; `docs/design/09-design-effects-orchestrator.md` §5; `docs/prd/14-catalogo-contratos-eventos.md` (sagas).
- **Situação**: `parcial` - o ciclo da instrução, as pré-condições, a comparação de valor e destino, o estorno e o cancelamento são construíveis; a camada 2 da validação de destino (titularidade por serviço) e a consulta de estado do financiador dependem de fornecedor.
- **Fase de entrega**: 1.
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-ZFDEPFWC`, Effects); a cadeia transitiva (contracting, gateway, partner, identity) fica implícita. Upstream duro: Effects (`EffectConfirmed`, `EffectPartiallyConfirmed`, `EffectOutcomeUnknown`), Contracting (contrato assinado, 10 §6.1), Gateway (`ExternalCommandConfirmed`, `ExternalOutcomeUnknown`), Partner Management (protocolo) e Identity (`BankAccountChangeRequested`, `BankAccountChangeApproved`). Upstream brando: KYB (`CHK-BNK-001`) e Fraude e Risco (pré-condição 5). As arestas de retroalimentação não entram em `depends_on`.
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
|---|---|---|
| #11 (C1; 10:D7) | serviço de validação de titularidade de conta | `ValidateDestination`, método `ownership_service` (camada 2) |
| 10:D8 | prazo de quarentena de conta | `RegisterDestination.quarantineUntil` |
| 10:D9 | financiadores oferecem consulta de estado de desembolso? | resolução automática de `unknown_outcome` |
| A11 (10:D10) | regra de recálculo proporcional (jurídico) | `OpenFundingInstruction` com `partialMode = proportional_adjustment` |
| 10:D11 | formato de comprovante aceito pelos financiadores | `FundingProof` |
| #1 (A1) | autorização do BCB (transversal) | o contexto inteiro |
| #12 | volumetria (dimensiona, não bloqueia) | persistência e relay |

- **Decisões de produto abertas**: caminho padrão da confirmação parcial (09 §5.2); regra de recálculo proporcional de deságio e tarifas (10 §5, A11); prazo de quarentena (10:D8); quem declara o destino da operação (conta padrão da organização ou destino do contrato) [ABERTA: 10 não o define].
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída e `SPEC-ZFDEPFWC` com `EffectConfirmed` publicado. Do que trava o núcleo, #11 não impede `planning` (as camadas 1, 3 e 4 de 10 §3 bastam, e a camada 5 é recusada); 10:D9 deve estar decidido como critério de homologação do financiador (13), senão todo `unknown_outcome` vira caso manual.

## Agregados

`Disbursement`, `FundingProof` e `FundingMismatch` são entidades internas de `FundingInstruction`: as divergências e o estado da instrução mudam juntos. Instantes `*At` valem 0 antes do fato; dinheiro é `*Minor: int` com `currency: string`.

### FundingInstruction

- **Identidade**: `natural` por `effectId` (0..1 instrução por efeito, 90 §4).
- **Campos**:
  - Origem: `effectId: id`, `contractId: id`, `cedentOrganizationId: id`, `funderOrganizationId: id`, `destinationHash: string`.
  - Valores: `currency: string`, `contractedGrossAmountMinor: int`, `confirmedGrossAmountMinor: int`, `expectedNetAmountMinor: int`, `partialMode: full | proportional_adjustment`, `deductions: list<Deduction>` (coleção indispensável, req. 3 do PRD: exibir bruto, deduções e líquido) com `Deduction` = `code: string`, `kind: percentage | fixed`, `amountMinor: int`.
  - Fluxo: `status`, `unmetPrerequisites: list<string>`, `effectOutcomeUnknown: bool`, `accountChangePending: bool`, `idempotencyKey: string`, `payloadVersion: int`, `rejectionCode: string`, `rejectionReason: string`, `createdAt`, `requestedAt`, `closedAt: instant`.
  - `disbursements: list<Disbursement>` (N por instrução, 90 §4) com `Disbursement` = `disbursementId: id`, `funderReference: string`, `amountMinor: int`, `executedAt: instant`, `destinationHash: string`, `paymentMethod: string`, `paymentReference: string`, `amountMatches: bool`, `destinationMatches: bool`, `status: confirmed | reversed`, `reversalReason: funder_error | beneficiary_return | other`, e `FundingProof` = `proofEvidenceId: id`, `proofEvidenceHash: string`, `funderDocumentEvidenceId: id`, `destinationMasked: string` (10 §7; o conteúdo do comprovante vive no armazenamento [ABERTA: provider de object storage]).
  - `mismatches: list<FundingMismatch>` com `FundingMismatch` = `mismatchId: id`, `kind: amount | destination | both | status`, `expectedAmountMinor: int`, `reportedAmountMinor: int`, `expectedDestinationHash: string`, `reportedDestinationHash: string`, `status: open | resolved`, `resolution: string`, `openedAt`, `resolvedAt: instant`.
- **Estados**: `pending_prerequisites`, `ready`, `requested`, `processing`, `disbursed`, `rejected`, `cancelled`, `unknown_outcome`, `mismatch`, `reversed`.
- **Transições permitidas**:
  - (novo) → `pending_prerequisites` - por `OpenFundingInstruction`
  - `pending_prerequisites`, `ready` → `ready` ou `pending_prerequisites` - por `EvaluatePrerequisites`; `ready` → `pending_prerequisites` - por `HoldFundingInstruction` e `BlockInstructionOnAccountChange`; `pending_prerequisites` → `pending_prerequisites` (limpa o bloqueio de conta) - por `ReconfirmInstructionDestination`
  - `ready` → `requested` - por `RequestDisbursement`
  - `requested`, `unknown_outcome` → `processing` - por `RecordDisbursementProcessing`; → `rejected` - por `RecordDisbursementRejection`
  - `requested`, `processing` → `unknown_outcome` - por `RecordDisbursementOutcomeUnknown`
  - `requested`, `processing`, `unknown_outcome` → `disbursed` ou `mismatch` - por `RecordDisbursementConfirmation`; confirmação recebida em `cancelled`, `rejected` ou `disbursed` (outra referência) → `mismatch` com `kind = status`
  - `requested`, `processing` → `ready` e `disbursed`, `mismatch` → `reversed` - por `RecordDisbursementReversal`
  - `mismatch` → `disbursed` - por `ResolveFundingMismatch`
  - `pending_prerequisites`, `ready` → `cancelled`; `requested` → `cancelled` só com confirmação do financiador - por `CancelFundingInstruction`
  - `reversed` → `pending_prerequisites` - por `ReopenFundingInstruction`
- **Invariantes de estado**:
  - A plataforma nunca é conta de destino nem de trânsito: o destino da instrução é um `DisbursementDestination` aceito pela regra abaixo.
  - O valor confirmado vem do retorno do financiador, nunca da ordem enviada; `amountMatches` compara os dois.
  - Uma referência do financiador não produz dois desembolsos: repetida, é aceita sem alterar.
  - Instrução nunca nasce com valor inferido: confirmação parcial só gera instrução com `proportional_adjustment` - `funding/funding-instruction/partial-path-undefined`.
  - Mudança de conta depois da assinatura bloqueia a operação até a reconfirmação por canal independente (10 §4) - `funding/funding-instruction/account-change-pending`.
  - Cancelar a instrução com efeito vigente exige o pedido de liberação no mesmo fluxo - `funding/funding-instruction/effect-release-not-requested`.
  - O estorno não libera o efeito: a instrução vai a `reversed` e o efeito permanece (10 §8).

### DisbursementDestination

- **Identidade**: `natural` por `destinationHash` (hash com sal secreto da conta, 90 §2.2; dentro do tenant).
- **Campos**: `destinationHash: string`, `organizationId: id` (cedente), `accountMasked: string`, `holderTaxIdHash: string`, `kind: own | third_party`, `thirdPartyAuthorizationEvidenceId: id`, `status: pending_validation | validated | rejected`, `validationMethod: fiscal_id_match | ownership_service | micro_deposit | document_declaration | declaration_only`, `validatedAt: instant`, `validationEvidenceId: id`, `secondApproverId: id`, `quarantineUntil: instant`, `registeredAt: instant`. Os dados bancários completos são `PII_SENSITIVE` e não entram em evento nem em log (90 §1.4) [ABERTA: origem do dado completo para a ordem ao financiador].
- **Estados**: `pending_validation`, `validated`, `rejected`; "em quarentena" é derivado (`validated` e `at < quarantineUntil`).
- **Transições permitidas**: (novo) → `pending_validation` - por `RegisterDestination`; `pending_validation` → `validated` ou `rejected` - por `ValidateDestination`.
- **Invariantes de estado**:
  - Nunca referencia conta de titularidade da plataforma, imposto na validação - `funding/disbursement-destination/platform-account`.
  - Declaração sem comprovação (camada 5) não valida - `funding/disbursement-destination/insufficient-validation-method`.
  - Destino de terceiro exige a autorização contratual e aprovação reforçada, e fica marcado como terceiro - `funding/disbursement-destination/third-party-requires-reinforced-approval`.
  - O histórico de contas é preservado; o tratamento "menos restritivo" do retorno a conta anterior (94 C2.7) não tem regra definida [ABERTA: 94:C2.7].

## Comandos (UPRs)

O disparo vem de evento consumido, de chamada gRPC (cedente, financiador pela Partner API via `bff`, backoffice) ou de processo do contexto. Os fatos de outros contextos (contrato, efeito, risco, destino) são lidos pelo caso de uso, fora da UoW, e entram como entrada. Fatos de avaliação: `contractValid: bool` (assinado e não cancelado), `effectConfirmed: bool`, `relatedEffectUnknown: bool`, `destinationState: validated | quarantined | pending_validation | rejected | unknown`, `riskBlockActive: bool`, `effectDisputeOpen: bool`.

### OpenFundingInstruction

- **Agregado**: `FundingInstruction`; **inicializa se ausente**
- **Disparo**: consumo de `EffectConfirmed`; consumo de `EffectPartiallyConfirmed` (a instrução só nasce no caminho proporcional)
- **Entrada**: `effectId: id`, `contractId: id`, `cedentOrganizationId: id`, `funderOrganizationId: id`, `destinationHash: string`, `currency: string`, `contractedGrossAmountMinor: int`, `confirmedGrossAmountMinor: int`, `deductions: list<code, kind, amountMinor>`, `partialMode`, `effectConfirmation: none | partial | full`, `contractValid: bool`
- **Pré-condições**:
  - `contractValid` → `funding/funding-instruction/contract-not-valid`
  - `effectConfirmation` diferente de `none` → `funding/funding-instruction/effect-not-confirmed`
  - `partial` só com `partialMode = proportional_adjustment` (contrato omisso ou tudo-ou-nada não gera instrução, 10 §5) → `funding/funding-instruction/partial-path-undefined`
  - `0 < confirmedGrossAmountMinor ≤ contractedGrossAmountMinor` e soma das deduções ≤ bruto confirmado → `funding/funding-instruction/invalid-amounts`
- **Efeitos sobre o estado**: quando ausente: `status ← pending_prerequisites`; campos `← cmd`; `expectedNetAmountMinor ←` bruto confirmado menos deduções, sendo que no modo proporcional as deduções `percentage` escalam por `confirmedGross / contractedGross` e as `fixed` não (10 §5) [A CONFIRMAR: A11, 10:D10] [ABERTA: regra de arredondamento]; `createdAt ← at`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: `FundingInstructionCreated` (só quando inicializa)
- **Resposta**: `fundingInstructionId: id` (igual a `effectId`)

### EvaluatePrerequisites

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: processo do contexto, após criação, validação de destino ou mudança de qualquer fato (10 §6.1)
- **Entrada**: os fatos de avaliação
- **Pré-condições**: `status` em `pending_prerequisites` ou `ready` → `funding/funding-instruction/not-evaluable`
- **Efeitos sobre o estado**: `unmetPrerequisites ←` lista das seis pré-condições de 10 §6.1 que não valem (contrato, efeito `EFFECTIVE`, nenhum efeito relacionado em `UNKNOWN_OUTCOME`, destino validado e fora de quarentena, nenhum bloqueio de risco, nenhuma disputa aberta) mais `account-change-pending`; `effectOutcomeUnknown ← relatedEffectUnknown`; `status ← ready` se a lista é vazia, senão `pending_prerequisites`. Não rejeita por pré-condição não atendida: o resultado é o próprio estado
- **Eventos emitidos**: nenhum
- **Resposta**: `status`, `unmetPrerequisites: list<string>`

### RequestDisbursement

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: processo do contexto; chamada gRPC do backoffice. Chave de idempotência obrigatória na borda (91 §2.2)
- **Entrada**: os fatos de avaliação, `payloadVersion: int`, `idempotencyKey: string`
- **Pré-condições** (reavaliadas na emissão, 10 §6.1):
  - `status = ready` → `funding/funding-instruction/not-ready`
  - `contractValid` → `funding/funding-instruction/contract-not-valid`
  - `effectConfirmed` → `funding/funding-instruction/effect-not-confirmed`
  - `relatedEffectUnknown` falso → `funding/funding-instruction/effect-outcome-unknown`
  - `destinationState ≠ quarantined` → `funding/funding-instruction/destination-in-quarantine`
  - `destinationState = validated` → `funding/funding-instruction/destination-not-validated`
  - `riskBlockActive` falso → `funding/funding-instruction/risk-block-active`
  - `effectDisputeOpen` falso → `funding/funding-instruction/effect-dispute-open`
  - `accountChangePending` falso → `funding/funding-instruction/account-change-pending`
- **Efeitos sobre o estado**: `status ← requested`; `idempotencyKey` e `payloadVersion ← cmd`; `requestedAt ← at`
- **Eventos emitidos**: `DisbursementRequested`
- **Resposta**: `fundingInstructionId: id`, `idempotencyKey: string`

### RecordDisbursementProcessing

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: consumo de `ExternalCommandConfirmed` (o financiador aceitou o pedido)
- **Entrada**: `externalCommandId: id`, `acknowledgedAt: instant`
- **Pré-condições**: `status` em `requested` ou `unknown_outcome` → `funding/funding-instruction/not-awaiting-processing`
- **Efeitos sobre o estado**: `status ← processing`
- **Eventos emitidos**: `DisbursementProcessing`
- **Resposta**: `fundingInstructionId: id`

### RecordDisbursementRejection

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: consumo de `ExternalCommandFailed` [ABERTA: 92:§4 não lista Funding como consumidor; sem ele `REJECTED` (10 PRD) não tem gatilho]
- **Entrada**: `rejectionCode: string`, `rejectionReason: string`
- **Pré-condições**: `status` em `requested` ou `unknown_outcome` → `funding/funding-instruction/not-awaiting-processing`
- **Efeitos sobre o estado**: `status ← rejected`; código e texto do financiador preservados
- **Eventos emitidos**: nenhum [ABERTA: o PRD não tem evento de rejeição de desembolso]
- **Resposta**: `fundingInstructionId: id`

### RecordDisbursementOutcomeUnknown

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: consumo de `ExternalOutcomeUnknown`
- **Entrada**: `stateQuerySupported: bool`, `lastAttemptAt: instant`
- **Pré-condições**: `status` em `requested` ou `processing` → `funding/funding-instruction/not-in-flight`
- **Efeitos sobre o estado**: `status ← unknown_outcome`. Sem consulta de estado do financiador (10:D9), a ambiguidade vira caso manual; nunca há reenvio automático
- **Eventos emitidos**: nenhum (o Ops recebe `ExternalOutcomeUnknown` do Gateway) [ABERTA: `UNKNOWN_OUTCOME` do PRD sem evento próprio]
- **Resposta**: `fundingInstructionId: id`

### RecordDisbursementConfirmation

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: chamada gRPC, vinda da Partner API (91 §4.3, `POST /v1/partner/funding-instructions/{id}/disbursements`) pelo `bff`. O pagamento já ocorreu: a divergência é registrada, não recusada
- **Entrada**: `disbursementId: id`, `funderReference: string`, `amountMinor: int`, `executedAt: instant`, `destinationHash: string`, `paymentMethod: string`, `paymentReference: string`, `proofEvidenceId: id`, `proofEvidenceHash: string`, `funderDocumentEvidenceId: id`, `destinationMasked: string`
- **Pré-condições**:
  - `funderReference` e `proofEvidenceHash` não vazios (a confirmação contém referência e comprovante) → `funding/funding-instruction/proof-incomplete`
  - `amountMinor > 0` → `funding/funding-instruction/invalid-amount`
- **Efeitos sobre o estado**: `funderReference` já registrada: aceita sem alterar. Senão acrescenta o desembolso `confirmed`, com `amountMatches ← amountMinor = expectedNetAmountMinor` e `destinationMatches ← destinationHash = instrução.destinationHash`. Ambos verdadeiros e `status` em `requested`, `processing` ou `unknown_outcome`: `status ← disbursed`. Caso contrário: `status ← mismatch` e acrescenta `FundingMismatch` `open` com o `kind` correspondente
- **Eventos emitidos**: `DisbursementConfirmed` (sempre que registra, com as flags de comparação) e `FundingMismatchDetected` (quando há divergência) [ABERTA: 10 PRD × 92 §3.5 - emissão de `DisbursementConfirmed` em divergência é implícita nas flags]
- **Resposta**: `fundingInstructionId: id`, `amountMatches: bool`, `destinationMatches: bool`

### RecordDisbursementReversal

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: chamada gRPC, vinda da Partner API (estorno informado pelo financiador)
- **Entrada**: `funderReference: string`, `reversedAmountMinor: int`, `reversalReason`, `reversedAt: instant`
- **Pré-condições**:
  - `status` em `requested`, `processing`, `disbursed` ou `mismatch` → `funding/funding-instruction/nothing-to-reverse`
  - após a confirmação, `funderReference` igual à de um desembolso registrado → `funding/funding-instruction/unknown-disbursement`
- **Efeitos sobre o estado**: antes da confirmação (`requested`, `processing`): `status ← ready`, a reavaliar. Depois (`disbursed`, `mismatch`): `desembolso.status ← reversed`, `status ← reversed`; o efeito permanece e o caso é crítico (10 §8)
- **Eventos emitidos**: `DisbursementReversed`
- **Resposta**: `fundingInstructionId: id`

### ResolveFundingMismatch

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: chamada gRPC do backoffice (caso `FUNDING_MISMATCH`, 17 §4)
- **Entrada**: `mismatchId: id`, `resolution: string`, `justification: string`, `secondApproverId: id`
- **Pré-condições**:
  - `status = mismatch` e a divergência `open` → `funding/funding-instruction/no-open-mismatch`
  - justificativa e segundo aprovador informados → `funding/funding-instruction/justification-missing`
- **Efeitos sobre o estado**: `mismatch.status ← resolved`; `status ← disbursed` [ABERTA: 10 não define como a exceção se encerra; mínimo coerente com o caso do Ops]
- **Eventos emitidos**: nenhum
- **Resposta**: `fundingInstructionId: id`

### CancelFundingInstruction

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: chamada gRPC (cedente, financiador ou backoffice)
- **Entrada**: `cancelledBy: cedent | funder | backoffice`, `reason: string`, `funderConfirmedCancellation: bool`, `effectInForce: bool`, `effectReleaseRequestId: id`
- **Pré-condições**:
  - `status` em `pending_prerequisites` ou `ready`, ou `requested` com `funderConfirmedCancellation` (10 §9) → `funding/funding-instruction/not-cancellable`
  - `effectInForce` exige `effectReleaseRequestId` (pedido de liberação aceito pelo Effects antes) → `funding/funding-instruction/effect-release-not-requested`
- **Efeitos sobre o estado**: `status ← cancelled`; `closedAt ← at`
- **Eventos emitidos**: nenhum [ABERTA: o PRD não tem evento de cancelamento]
- **Resposta**: `fundingInstructionId: id`

### HoldFundingInstruction

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: consumo de `EffectOutcomeUnknown` (92 §3.4, `blocksFunding`)
- **Entrada**: `effectId: id`, `contractId: id`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: em `pending_prerequisites` ou `ready`: `effectOutcomeUnknown ← true`, `status ← pending_prerequisites`. Em outro status: aceita sem alterar (sem instrução ainda, nada a bloquear; com desembolso em curso, o Ops já recebe o evento)
- **Eventos emitidos**: nenhum
- **Resposta**: `fundingInstructionId: id`

### BlockInstructionOnAccountChange

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: consumo de `BankAccountChangeRequested`, um comando por instrução em aberto do cedente
- **Entrada**: `changeRequestId: id`, `requestedAt: instant`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**: em `pending_prerequisites` ou `ready`: `accountChangePending ← true`, `status ← pending_prerequisites` (a operação com contrato fechado e dinheiro ainda não pago é a janela do ataque, 10 §4). Em outro status: aceita sem alterar [ABERTA: 10 §4 não trata `requested` e `processing`]
- **Eventos emitidos**: nenhum
- **Resposta**: `fundingInstructionId: id`

### ReconfirmInstructionDestination

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: chamada gRPC do backoffice (reconfirmação por canal independente; a aprovação na plataforma não basta)
- **Entrada**: `destinationHash: string`, `independentChannelReference: string`, `confirmedBy: id`
- **Pré-condições**:
  - `accountChangePending` → `funding/funding-instruction/no-account-change-pending`
  - `independentChannelReference` não vazia → `funding/funding-instruction/independent-confirmation-missing`
- **Efeitos sobre o estado**: `destinationHash ← cmd`; `accountChangePending ← false`; `status ← pending_prerequisites` (a nova conta ainda precisa estar validada e fora de quarentena)
- **Eventos emitidos**: nenhum
- **Resposta**: `fundingInstructionId: id`

### ReopenFundingInstruction

- **Agregado**: `FundingInstruction`; exige existente
- **Disparo**: chamada gRPC do backoffice (estorno por erro do financiador, "nova instrução se as partes concordarem", 10 §8)
- **Entrada**: `agreementEvidenceId: id`, `idempotencyKey: string`
- **Pré-condições**:
  - `status = reversed` e `reversalReason = funder_error` → `funding/funding-instruction/not-reversed`
  - `agreementEvidenceId` não vazio → `funding/funding-instruction/agreement-missing`
- **Efeitos sobre o estado**: `status ← pending_prerequisites`; `payloadVersion ← payloadVersion + 1`; `idempotencyKey ← cmd`; histórico de desembolsos preservado [ABERTA: 10 §8 "nova instrução" como reabertura ou instância nova; a identidade por `effectId` admite uma só]
- **Eventos emitidos**: `FundingInstructionCreated`
- **Resposta**: `fundingInstructionId: id`

### RegisterDestination

- **Agregado**: `DisbursementDestination`; **inicializa se ausente**
- **Disparo**: consumo de `BankAccountChangeApproved`
- **Entrada**: `destinationHash: string`, `organizationId: id`, `accountMasked: string`, `holderTaxIdHash: string`, `kind`, `thirdPartyAuthorizationEvidenceId: id`, `isPlatformAccount: bool` (fato: o hash coincide com conta da plataforma [ABERTA: origem da lista de contas da plataforma]), `quarantineUntil: instant` [A CONFIRMAR: 10:D8]
- **Pré-condições**: `isPlatformAccount` falso → `funding/disbursement-destination/platform-account`
- **Efeitos sobre o estado**: quando ausente: `status ← pending_validation`; campos `← cmd`; `registeredAt ← at`. Quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: nenhum
- **Resposta**: `destinationHash: string`

### ValidateDestination

- **Agregado**: `DisbursementDestination`; exige existente
- **Disparo**: chamada gRPC (processo de validação ou backoffice)
- **Entrada**: `method`, `result: passed | failed`, `evidenceId: id`, `secondApproverId: id`, `validatedAt: instant`
- **Pré-condições**:
  - `status = pending_validation` → `funding/disbursement-destination/not-pending-validation`
  - `method ≠ declaration_only` (camada 5, 10 §3) → `funding/disbursement-destination/insufficient-validation-method`
  - `kind = third_party` exige `secondApproverId` e a autorização contratual → `funding/disbursement-destination/third-party-requires-reinforced-approval`
- **Efeitos sobre o estado**: `result = passed`: `status ← validated`; `failed`: `status ← rejected`; `validationMethod`, `validatedAt`, evidência `← cmd` [A CONFIRMAR: #11, C1 para `ownership_service`]
- **Eventos emitidos**: nenhum
- **Resposta**: `destinationHash: string`, `status`

## Eventos de domínio

### FundingInstructionCreated

- **Campos**: `fundingInstructionId: id ← FundingInstruction.effectId`, `contractId: id ← FundingInstruction.contractId`, `effectId: id ← FundingInstruction.effectId`, `funderOrganizationId: id ← FundingInstruction.funderOrganizationId`, `currency: string ← FundingInstruction.currency`, `grossAmountMinor: int ← FundingInstruction.confirmedGrossAmountMinor`, `deductionsMinor: int ← soma(deductions)`, `expectedNetAmountMinor: int ← FundingInstruction.expectedNetAmountMinor`, `partialMode ← FundingInstruction.partialMode`, `at: instant ← at`

### DisbursementRequested

- **Campos**: `fundingInstructionId: id ← FundingInstruction.effectId`, `contractId: id ← FundingInstruction.contractId`, `funderOrganizationId: id ← FundingInstruction.funderOrganizationId`, `expectedNetAmountMinor: int ← FundingInstruction.expectedNetAmountMinor`, `destinationHash: string ← FundingInstruction.destinationHash`, `idempotencyKey: string ← FundingInstruction.idempotencyKey`, `at: instant ← at`

### DisbursementProcessing

- **Campos**: `fundingInstructionId: id ← FundingInstruction.effectId`, `contractId: id ← FundingInstruction.contractId`, `acknowledgedAt: instant ← cmd.acknowledgedAt`, `at: instant ← at`

### DisbursementConfirmed

- **Campos**: `fundingInstructionId: id ← FundingInstruction.effectId`, `disbursementId: id ← Disbursement.disbursementId`, `contractId: id ← FundingInstruction.contractId`, `effectId: id ← FundingInstruction.effectId`, `funderOrganizationId: id ← FundingInstruction.funderOrganizationId`, `confirmedAt: instant ← Disbursement.executedAt`, `amountMinor: int ← Disbursement.amountMinor`, `expectedAmountMinor: int ← FundingInstruction.expectedNetAmountMinor`, `amountMatches: bool ← Disbursement.amountMatches`, `destinationHash: string ← Disbursement.destinationHash`, `destinationMatches: bool ← Disbursement.destinationMatches`, `funderReference: string ← Disbursement.funderReference`, `proofEvidenceId: id ← FundingProof.proofEvidenceId`, `at: instant ← at`. Dados bancários não entram no evento; `destinationHash` permite conferir o destino sem trafegar a conta (92 §3.5). Diverge do design: 92 usa `destinationId` com prefixo `dst_`; aqui o destino é identificado pelo próprio hash.

### FundingMismatchDetected

- **Campos**: `fundingInstructionId: id ← FundingInstruction.effectId`, `mismatchId: id ← FundingMismatch.mismatchId`, `contractId: id ← FundingInstruction.contractId`, `kind ← FundingMismatch.kind`, `expectedAmountMinor: int ← FundingMismatch.expectedAmountMinor`, `reportedAmountMinor: int ← FundingMismatch.reportedAmountMinor`, `destinationMatches: bool ← Disbursement.destinationMatches`, `at: instant ← at`

### DisbursementReversed

- **Campos**: `fundingInstructionId: id ← FundingInstruction.effectId`, `contractId: id ← FundingInstruction.contractId`, `effectId: id ← FundingInstruction.effectId`, `funderReference: string ← cmd.funderReference`, `reversedAmountMinor: int ← cmd.reversedAmountMinor`, `reversalReason ← cmd.reversalReason`, `phase: before_confirmation | after_confirmation ← status anterior`, `effectRemainsInForce: bool ← true`, `at: instant ← at`

## Consultas

- `FindFundingInstruction` - filtros: `fundingInstructionId`; cardinalidade: `one` (inclui desembolsos e divergências)
- `FindFundingInstructionsByContract` - filtros: `contractId`; cardinalidade: `many`
- `FindFundingInstructionsByCedent` - filtros: `cedentOrganizationId`, `status`; cardinalidade: `many` (fan-out de `BlockInstructionOnAccountChange`)
- `FindFundingProof` - filtros: `fundingInstructionId`, `disbursementId`; cardinalidade: `one` (comprovante ao cedente, req. 9 do PRD)
- `FindDisbursementDestination` - filtros: `destinationHash`; cardinalidade: `one`
- `FindDisbursementDestinationsByOrganization` - filtros: `organizationId`, `status`; cardinalidade: `many` (histórico de contas, 94 C2.7)

## Relações entre agregados

- `FundingInstruction.effectId` → `RegulatoryEffect` (Effects)
- `FundingInstruction.contractId` → `ContractCase` (Contracting)
- `FundingInstruction.cedentOrganizationId`, `FundingInstruction.funderOrganizationId` → `Organization` (Identity)
- `FundingInstruction.destinationHash` → `DisbursementDestination`
- `DisbursementDestination.organizationId` → `Organization` (Identity)

## Integração

- **Publica** (integration events): `FundingInstructionCreated`, `DisbursementRequested`, `DisbursementProcessing`, `DisbursementConfirmed`, `FundingMismatchDetected` e `DisbursementReversed` - viram `funding_instruction_created.proto`, `disbursement_requested.proto`, `disbursement_processing.proto`, `disbursement_confirmed.proto`, `funding_mismatch_detected.proto` e `disbursement_reversed.proto` em `apps/backend/funding/contract/proto/company/funding/event/v1/`, package `company.funding.event.v1`. Consumidores declarados em 92 §4: Settlement (`DisbursementConfirmed`, `DisbursementReversed`), Ops (`FundingMismatchDetected`, `DisbursementReversed`), Risco (`FundingMismatchDetected`) e Ledger. `DisbursementProcessing` consta do PRD e não de 92 §4 [ABERTA: 92:§4].
- **Consome** (disposições de consumo por FND-04 §6.4; os schemas de Effects não cobrem `contractedGross` nem deduções, e os de Gateway e Identity não estão em 92 [ABERTA: 92:§3]):
  - `company.effects.event.v1.EffectConfirmed` → `OpenFundingInstruction` com `effectConfirmation = full`. Mapa: `effectId ← effectId`, `contractId ← contractId`, `funderOrganizationId ← beneficiaryOrganizationId`, `confirmedGrossAmountMinor ← confirmedTotalMinor`, `contractedGrossAmountMinor ← requestedTotalMinor`; cedente, deduções, destino, `contractValid` ← consulta gRPC a Contracting (termos do contrato, 10 §6.1) [ABERTA: 92:§4 não lista Funding como consumidor de `ContractSigned`].
  - `company.effects.event.v1.EffectPartiallyConfirmed` → `OpenFundingInstruction` com `effectConfirmation = partial` e `partialMode ← partialConfirmationClause`. Só `proportional_adjustment` gera instrução; contrato omisso ou tudo-ou-nada segue a disposição de descarte de FND-04 §6.4 e o Ops abre o caso `EFFECT_PARTIAL`.
  - `company.effects.event.v1.EffectOutcomeUnknown` → `HoldFundingInstruction`. Mapa: `effectId ← effectId`, `contractId ← contractId`.
  - `company.gateway.event.v1.ExternalCommandConfirmed` → `RecordDisbursementProcessing`. Mapa: `fundingInstructionId ← businessKey`, `acknowledgedAt ← at`.
  - `company.gateway.event.v1.ExternalCommandFailed` → `RecordDisbursementRejection`. Mapa: `rejectionCode`, `rejectionReason` do evento, sem tradução [ABERTA: 92:§4].
  - `company.gateway.event.v1.ExternalOutcomeUnknown` → `RecordDisbursementOutcomeUnknown`. Mapa: `stateQuerySupported`, `lastAttemptAt` do evento.
  - `company.identity.event.v1.BankAccountChangeRequested` → `BlockInstructionOnAccountChange`, um por instrução em aberto do cedente (`FindFundingInstructionsByCedent`). Mapa: `changeRequestId ← changeRequestId`, `requestedAt ← at`.
  - `company.identity.event.v1.BankAccountChangeApproved` → `RegisterDestination`. Mapa: `destinationHash ← accountHash`, `organizationId ← organizationId`, `accountMasked`, `holderTaxIdHash`, `kind`, `thirdPartyAuthorizationEvidenceId` do evento; `quarantineUntil ← at + prazo` [A CONFIRMAR: 10:D8]. [ABERTA: os eventos `BankAccountChange*` foram propostos em 92 §4 sem schema nem consumidores]
- **Consultas e comandos de saída**: gRPC a Contracting (termos do contrato), Effects (status do efeito, disputa aberta e efeito relacionado em `UNKNOWN_OUTCOME`), Fraude e Risco (bloqueio vigente [ABERTA: 92:§4 não declara consumidores de `OperationBlockedByRisk`]), KYB (`CHK-BNK-001`) e Partner Management (protocolo e consulta de estado do financiador). A solicitação de desembolso ao financiador é comando externo [ABERTA: 04 §3 não tem porta canônica para o pedido de desembolso, embora o PRD do Gateway inclua financiadores] [ABERTA: ciclo de comando externo sem suporte no kernel (`UNKNOWN_OUTCOME` e idempotência de saída; ADR-056 cobre só comando de entrada)].

## Borda e persistência

- **Borda**: só gRPC, `company.funding.service.v1.FundingService`, com um RPC por comando (`OpenFundingInstruction`, `EvaluatePrerequisites`, `RequestDisbursement`, `RecordDisbursementProcessing`, `RecordDisbursementRejection`, `RecordDisbursementOutcomeUnknown`, `RecordDisbursementConfirmation`, `RecordDisbursementReversal`, `ResolveFundingMismatch`, `CancelFundingInstruction`, `HoldFundingInstruction`, `BlockInstructionOnAccountChange`, `ReconfirmInstructionDestination`, `ReopenFundingInstruction`, `RegisterDestination`, `ValidateDestination`) e por consulta (`FindFundingInstruction`, `FindFundingInstructionsByContract`, `FindFundingInstructionsByCedent`, `FindFundingProof`, `FindDisbursementDestination`, `FindDisbursementDestinationsByOrganization`); os disparados só por consumo ou por processo não são chamados pelo bff. O REST é do `bff`: a confirmação de desembolso do financiador (`POST /v1/partner/funding-instructions/{id}/disbursements`, 91 §4.3), a visão do cedente (valor líquido, situação e comprovante) e os webhooks de saída (91 §5.2).
- **Banco**: database e role `funding`, schema `public`, tabelas `funding_instructions` e `disbursement_destinations`, com `tenant_id` à frente da chave. Persistência híbrida: em `funding_instructions`, colunas tipadas `contract_id` (`FindFundingInstructionsByContract`), `cedent_organization_id` e `status` (`FindFundingInstructionsByCedent`); em `disbursement_destinations`, `organization_id` e `status` (`FindDisbursementDestinationsByOrganization`); o resto vai em `snapshot` `jsonb`. O comprovante e os documentos do financiador ficam por referência [ABERTA: provider de object storage].
- **Instante**: inteiro de nanossegundos no domínio.
- Diverge do design: 90 §1.3 fala em milissegundos e 92 §1 em envelope JSON com `schemaUri`; vale o DMPF (nanossegundos; Protobuf com CloudEvents montado pelo relay).
- Diverge do design: multi-tenancy por `tenant_id` à frente da chave e escopo injetado pelo `ExecutionContext` (ADR-051 do DMPF), não por RLS (00 §4.1). A visão do financiador sobre a instrução e a leitura do backoffice entre tenants [ABERTA: requer extensão do kernel - ADR-051 DMPF].
- Diverge do design: broker Kafka (default do DMPF; confirmação na decisão D4 da fundação), não SQS.

## Políticas transversais

- **Idempotência**: todo comando de entrada corre em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); os disparados por evento têm a deduplicação da inbox (ADR-036). O pedido ao financiador usa `idempotencyKey = hash(tenantId, partnerId, operationType, businessKey, payloadVersion)` com `businessKey` igual ao `fundingInstructionId` (00 §7.3); `unknown_outcome` se resolve por consulta e nunca por reenvio, e a chave nunca produz dois desembolsos.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Mudança de conta e destino de terceiro exigem dupla aprovação (94 C2.1; 10 §3) [ABERTA: dupla aprovação no kernel]. A confirmação do financiador entra pela Partner API autenticada por mTLS (91 §1).
- **Auditoria**: pela trilha de auditoria do kernel (FND-08) e pelo Ledger por outbox (12 §2); divergência de destino é de severidade máxima (94 C2.6).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `funding` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO um efeito EffectConfirmed, contrato válido e valores coerentes
QUANDO OpenFundingInstruction chega
ENTÃO a instrução nasce pending_prerequisites com expectedNetAmountMinor = bruto menos deduções e FundingInstructionCreated vai à outbox
DADO um efeito parcialmente confirmado e partialMode proportional_adjustment
QUANDO OpenFundingInstruction chega
ENTÃO a instrução nasce sobre o bruto confirmado, com deduções percentage escaladas e fixed intactas
DADO uma instrução já aberta para o efeito
QUANDO OpenFundingInstruction chega de novo
ENTÃO aceita sem alterar e sem emitir evento
DADO contractValid falso
QUANDO OpenFundingInstruction chega
ENTÃO rejeita com funding/funding-instruction/contract-not-valid e nada é escrito
DADO effectConfirmation none
QUANDO OpenFundingInstruction chega
ENTÃO rejeita com funding/funding-instruction/effect-not-confirmed
DADO confirmação parcial e partialMode full
QUANDO OpenFundingInstruction chega
ENTÃO rejeita com funding/funding-instruction/partial-path-undefined
DADO soma das deduções maior que o bruto confirmado
QUANDO OpenFundingInstruction chega
ENTÃO rejeita com funding/funding-instruction/invalid-amounts

DADO uma instrução com todas as pré-condições válidas
QUANDO EvaluatePrerequisites chega
ENTÃO a instrução vai a ready com unmetPrerequisites vazia
DADO uma instrução ready cujo destino entrou em quarentena
QUANDO EvaluatePrerequisites chega
ENTÃO a instrução volta a pending_prerequisites listando a pré-condição não atendida
DADO uma instrução requested
QUANDO EvaluatePrerequisites chega
ENTÃO rejeita com funding/funding-instruction/not-evaluable

DADO uma instrução ready e todas as pré-condições válidas na emissão
QUANDO RequestDisbursement chega com idempotencyKey
ENTÃO a instrução vai a requested e DisbursementRequested vai à outbox
DADO uma instrução pending_prerequisites
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/not-ready
DADO relatedEffectUnknown verdadeiro
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/effect-outcome-unknown
DADO destinationState quarantined
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/destination-in-quarantine
DADO destinationState pending_validation
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/destination-not-validated
DADO riskBlockActive verdadeiro
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/risk-block-active
DADO effectDisputeOpen verdadeiro
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/effect-dispute-open
DADO accountChangePending verdadeiro
QUANDO RequestDisbursement chega
ENTÃO rejeita com funding/funding-instruction/account-change-pending

DADO uma instrução requested
QUANDO RecordDisbursementProcessing chega
ENTÃO a instrução vai a processing e DisbursementProcessing é emitido
DADO uma instrução ready
QUANDO RecordDisbursementProcessing chega
ENTÃO rejeita com funding/funding-instruction/not-awaiting-processing
DADO uma instrução requested
QUANDO RecordDisbursementRejection chega com o código do financiador
ENTÃO a instrução vai a rejected, preservando código e texto originais
DADO uma instrução requested
QUANDO RecordDisbursementOutcomeUnknown chega
ENTÃO a instrução vai a unknown_outcome e nada é reenviado
DADO uma instrução ready
QUANDO RecordDisbursementOutcomeUnknown chega
ENTÃO rejeita com funding/funding-instruction/not-in-flight

DADO uma instrução processing e um retorno com valor e destino iguais aos esperados
QUANDO RecordDisbursementConfirmation chega com referência e comprovante
ENTÃO a instrução vai a disbursed com amountMatches e destinationMatches verdadeiros e DisbursementConfirmed é emitido
DADO uma instrução processing e um valor creditado diferente do esperado
QUANDO RecordDisbursementConfirmation chega
ENTÃO a instrução vai a mismatch com divergência amount e DisbursementConfirmed e FundingMismatchDetected são emitidos
DADO uma instrução processing e um destinationHash diferente do validado
QUANDO RecordDisbursementConfirmation chega
ENTÃO a instrução vai a mismatch com divergência destination e FundingMismatchDetected é emitido
DADO uma instrução cancelled
QUANDO RecordDisbursementConfirmation chega
ENTÃO o pagamento é registrado, a instrução vai a mismatch com divergência status e nada é recusado
DADO um desembolso já registrado com a mesma funderReference
QUANDO RecordDisbursementConfirmation chega de novo
ENTÃO aceita sem alterar e sem emitir
DADO uma confirmação sem proofEvidenceHash
QUANDO RecordDisbursementConfirmation chega
ENTÃO rejeita com funding/funding-instruction/proof-incomplete
DADO uma confirmação com amountMinor zero
QUANDO RecordDisbursementConfirmation chega
ENTÃO rejeita com funding/funding-instruction/invalid-amount

DADO uma instrução processing
QUANDO RecordDisbursementReversal chega
ENTÃO a instrução volta a ready e DisbursementReversed é emitido com phase before_confirmation
DADO uma instrução disbursed
QUANDO RecordDisbursementReversal chega com a funderReference registrada
ENTÃO a instrução vai a reversed, o efeito não é tocado e DisbursementReversed é emitido com phase after_confirmation
DADO uma instrução pending_prerequisites
QUANDO RecordDisbursementReversal chega
ENTÃO rejeita com funding/funding-instruction/nothing-to-reverse
DADO uma instrução disbursed
QUANDO RecordDisbursementReversal chega com funderReference desconhecida
ENTÃO rejeita com funding/funding-instruction/unknown-disbursement

DADO uma instrução mismatch com divergência open
QUANDO ResolveFundingMismatch chega com justificativa e segundo aprovador
ENTÃO a divergência vai a resolved e a instrução a disbursed
DADO uma instrução disbursed
QUANDO ResolveFundingMismatch chega
ENTÃO rejeita com funding/funding-instruction/no-open-mismatch
DADO uma instrução mismatch
QUANDO ResolveFundingMismatch chega sem justificativa
ENTÃO rejeita com funding/funding-instruction/justification-missing

DADO uma instrução ready sem efeito em vigor
QUANDO CancelFundingInstruction chega
ENTÃO a instrução vai a cancelled
DADO uma instrução processing
QUANDO CancelFundingInstruction chega
ENTÃO rejeita com funding/funding-instruction/not-cancellable
DADO uma instrução ready com efeito em vigor
QUANDO CancelFundingInstruction chega sem effectReleaseRequestId
ENTÃO rejeita com funding/funding-instruction/effect-release-not-requested e a instrução segue ready

DADO uma instrução ready
QUANDO HoldFundingInstruction chega
ENTÃO a instrução volta a pending_prerequisites com effectOutcomeUnknown verdadeiro
DADO uma instrução ready
QUANDO BlockInstructionOnAccountChange chega
ENTÃO a instrução volta a pending_prerequisites com accountChangePending verdadeiro
DADO uma instrução com accountChangePending e reconfirmação por canal independente
QUANDO ReconfirmInstructionDestination chega
ENTÃO accountChangePending é limpo e o destino passa a ser o reconfirmado
DADO uma instrução sem mudança de conta pendente
QUANDO ReconfirmInstructionDestination chega
ENTÃO rejeita com funding/funding-instruction/no-account-change-pending
DADO uma instrução com accountChangePending
QUANDO ReconfirmInstructionDestination chega sem independentChannelReference
ENTÃO rejeita com funding/funding-instruction/independent-confirmation-missing

DADO uma instrução reversed por erro do financiador
QUANDO ReopenFundingInstruction chega com a evidência do acordo
ENTÃO a instrução volta a pending_prerequisites com payloadVersion maior e FundingInstructionCreated é emitido
DADO uma instrução disbursed
QUANDO ReopenFundingInstruction chega
ENTÃO rejeita com funding/funding-instruction/not-reversed
DADO uma instrução reversed por erro do financiador
QUANDO ReopenFundingInstruction chega sem agreementEvidenceId
ENTÃO rejeita com funding/funding-instruction/agreement-missing

DADO um destino ainda desconhecido que não é conta da plataforma
QUANDO RegisterDestination chega
ENTÃO o destino nasce pending_validation com quarantineUntil e nenhum evento é emitido
DADO um destino já registrado
QUANDO RegisterDestination chega de novo
ENTÃO aceita sem alterar e sem emitir
DADO um hash que coincide com conta da plataforma
QUANDO RegisterDestination chega
ENTÃO rejeita com funding/disbursement-destination/platform-account
DADO um destino pending_validation e validação por ownership_service aprovada
QUANDO ValidateDestination chega
ENTÃO o destino vai a validated
DADO um destino validated
QUANDO ValidateDestination chega
ENTÃO rejeita com funding/disbursement-destination/not-pending-validation
DADO um destino pending_validation
QUANDO ValidateDestination chega com method declaration_only
ENTÃO rejeita com funding/disbursement-destination/insufficient-validation-method
DADO um destino de terceiro pending_validation
QUANDO ValidateDestination chega sem secondApproverId
ENTÃO rejeita com funding/disbursement-destination/third-party-requires-reinforced-approval

DADO duas instruções em aberto do cedente A e uma do cedente B
QUANDO FindFundingInstructionsByCedent consulta o cedente A
ENTÃO devolve exatamente as duas, fora de qualquer UoW
```

## Escopo fora

- **Movimentação de recursos**: manter saldo ou conta de pagamento, executar Pix ou TED, custodiar ou compensar valores. O desembolso é do financiador; a plataforma só autoriza e registra (regra 6 do PRD de produto).
- **REST do bff**: a confirmação de desembolso do financiador (91 §4.3), a visão do cedente e os webhooks de saída ficam no `bff`.
- **Decisão de crédito e contabilidade do financiador**: fora do PRD (10).
- **Liberação do efeito**: o estorno e o cancelamento não liberam o efeito; o pedido de liberação é do Effects, por decisão das partes (10 §8, §9).
- **Revisão contratual na confirmação parcial**: o caso `EFFECT_PARTIAL` e a decisão conjunta são do Ops (17) e do Contracting; aqui só o caminho proporcional, quando o contrato o prevê.
- **Acompanhamento da liquidação**: do Settlement (11), a partir de `DisbursementConfirmed` e `DisbursementReversed`.
- **Cobrança**: o fato gerador recomendado é `DisbursementConfirmed` (18 §2.3), mas o Billing só nasce após #7 (D1 a D3).
- **Validação por serviço de titularidade**: só existe após resolver #11 (C1); até lá valem as camadas 1, 3 e 4.
