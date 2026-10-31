---
id: SPEC-7KH5JJMV
slug: backoffice-operations
title: Bounded context — backoffice-operations
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6]
ticket_url: https://linear.app/mmda/issue/DEVS-83/spec-7kh5jjmv-bounded-context-backoffice-operations
subtask_urls: []
created: 2026-10-12
---

# SPEC-7KH5JJMV: Bounded context — backoffice-operations

## Identidade

- **bounded_context**: `backoffice-operations`
- **name**: `ops`
- **Linguagem ubíqua**:
  - *Caso operacional* (`OperationalCase`) — unidade de trabalho nascida de uma exceção: origem, tipo, severidade, responsável, prazo e estado.
  - *Ação do caso* (`CaseAction`) — ação executada no caso, que invoca o comando de domínio do contexto dono; tem autor, justificativa e resultado.
  - *Evidência do caso* (`CaseEvidence`) — material anexado: consulta, resposta ou comunicação.
  - *Regra de escalonamento* (`EscalationRule`) — quando e para quem escalar.
  - *Fila de trabalho* (`WorkQueue`) — agrupamento por tipo e severidade, com política de atribuição.
  - *Alçada* — o papel mínimo exigido por tipo de ação: operador, supervisor ou analista de risco.
  - *Segundo aprovador* — aprovação por pessoa distinta do autor, exigida em ação sensível.
  - *Consulta prévia* — consulta de estado externo, com resultado registrado, exigida antes de reenviar um comando.
  - *Pausa de prazo* — o relógio do prazo pausa em `PENDING_EXTERNAL`, porque o caso espera terceiro.
  - Princípios de 17 §2: nada se resolve fora da ferramenta; toda ação é comando de domínio; ajuste nunca sobrescreve; acesso a dado de cliente exige motivo; a ferramenta não inventa capacidade. Os valores enumerados do design são preservados literalmente.

## Origem e bloqueios externos

- **Fontes**: `docs/design/17-design-backoffice.md` §1 a §9 (contexto sem PRD); `docs/design/94-seguranca-threat-model.md` T7 e T9 (C7.5, C9.3); `docs/design/95-personas-jornadas.md` (P6, J3, J4 e matriz de papéis); `docs/design/92-schemas-eventos.md` §3 e §4; `docs/design/00-arquitetura-tecnica.md` §4.1 (acesso cross-tenant); `docs/design/93-requisitos-nao-funcionais.md` §4.1 e §8.
- **Situação**: `desbloqueado` — nenhum bloqueio externo; as lacunas 17:B1 a B4 são decisões internas e de operação. Cada tipo de caso só nasce quando o contexto produtor existe (`effects`, `funding`, `settlement` e os demais estão em fases e specs próprias).
- **Fase de entrega**: 1 (inferida).
- **Dependências**: upstream duro `identity` (`SPEC-7KW0HHF6`: papéis de plataforma e acesso cross-tenant, 00 §4.1). Upstream brando: `consent`, `gateway`, `kyb`, `contracting`, `effects`, `funding`, `settlement` e `partner`, cujos eventos abrem casos e cujos comandos as ações chamam. `depends_on` declara só a fundação e a aresta dura reduzida (`identity`); as arestas de retroalimentação, em que os demais contextos alimentam o backoffice, não entram.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| 17:B1 | Dimensionamento da equipe depende do volume de casos, desconhecido | `WorkQueue.assignmentPolicy` |
| 17:B2, E2 | Limiares de alçada por valor sem definição | `EscalationRule`; ajuste de conciliação acima do limiar; severidade de `RECONCILIATION_MISMATCH` |
| 17:B3 | Prazos alvo são propostas sem base histórica | `OperationalCase.dueAt` |
| 17:B4 | Integração com ferramenta externa de atendimento não decidida | Fora desta versão |
| `#12`, E4 | Volumetria real (não bloqueia a construção) | Dimensionamento da fila e do painel |
| `#1`, A1 | Autorização do BCB (existencial, transversal) | Contexto inteiro |

- **Decisões de produto abertas**: quem solicita e quem aprova o dossiê de auditoria (`95` matriz × `17` §5); o papel de "supervisor" que 17 usa e a matriz de 95 não tem.
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída e decisão sobre o acesso cross-tenant (a fila mostra casos de todos os tenants, 17 §6.1), capacidade que existe no kernel desde a `1.0.0-rc.6` (DEVS-100, ADR-068 do DMPF) e cujo uso aqui depende da adoção da rc.6 (DEVS-136). Nenhum bloqueio externo trava o núcleo; B2 e E2 travam só a faixa fina da alçada por valor.

## Agregados

Nota de modelagem: `tenantId` não é campo dos agregados — o escopo vem do `ExecutionContext` (ADR-051), e o caso fica no escopo do tenant afetado. O doc 90 não detalha este contexto; os campos são o mínimo derivado de 17 §3 a §5.

### OperationalCase

- **Identidade**: `generated`
- **Campos**:
  - `caseType: EXTERNAL_OUTCOME_UNKNOWN | EFFECT_REJECTED | EFFECT_PARTIAL | EFFECT_DISPUTED | FUNDING_MISMATCH | DISBURSEMENT_REVERSED | RECONCILIATION_MISMATCH | BENEFICIARY_DIVERGENCE | CONSENT_SYNC_FAILED | KYB_DISCREPANCY | SCHEMA_MISMATCH | EXTERNAL_INTERACTION_UNCORRELATED | EXTERNAL_AMBIGUITY_EXPIRED | EXTERNAL_REPROCESS_TERMINAL | CERTIFICATE_EXPIRING | DLQ_MESSAGE | OUTBOX_STALLED` — o catálogo de 17 §4, mais os três do Gateway (SPEC-N1X148AW, decisões 5, 7 e 10)
  - `severity: LOW | MEDIUM | HIGH | CRITICAL`
  - `sourceEventId: string` (opcional) — ausente em `DLQ_MESSAGE` e `OUTBOX_STALLED`, que nascem de fila morta e de alarme técnico
  - `subjects: list<CaseSubject>` — coleção indispensável: entidades relacionadas; cada uma com `subjectType: contract | effect | asset | organization` e `subjectId: id`
  - `partnerId: id` (opcional) — filtro "parceiro" da fila (17 §6.1)
  - `financialImpactMinor: int`, `financialImpactCurrency: string` (opcionais, nulos quando não se aplica)
  - `status: OPEN | ASSIGNED | IN_PROGRESS | PENDING_EXTERNAL | PENDING_APPROVAL | RESOLVED | CLOSED`
  - `assigneeUserId: id` (opcional)
  - `dueAt: instant`, `pausedAt: instant` (opcional)
  - `escalatedToRole: SUPERVISOR | PLATFORM_RISK` (opcional), `escalatedAt: instant` (opcional)
  - `resolutionCode: string`, `resolutionJustification: string`, `resolutionApproverIds: list<id>` (opcionais até a resolução) — coleção: os aprovadores que a alçada exigiu
  - `openedAt: instant`, `resolvedAt: instant` (opcional)
- **Estados**: os sete de `status`.
- **Transições permitidas** **[ABERTA: 17:§3.1 — o design lista os estados e não as transições; as abaixo são o mínimo coerente]**:
  - (novo) → `OPEN` — por `OpenOperationalCase`
  - `OPEN` → `ASSIGNED` — por `AssignOperationalCase`; nos estados `ASSIGNED`, `IN_PROGRESS` e `PENDING_EXTERNAL` a atribuição só troca o responsável
  - `ASSIGNED` → `IN_PROGRESS`; `IN_PROGRESS` → `PENDING_EXTERNAL` | `PENDING_APPROVAL`; `PENDING_EXTERNAL` → `IN_PROGRESS`; `PENDING_APPROVAL` → `IN_PROGRESS`; `RESOLVED` → `CLOSED` — por `ChangeOperationalCaseStatus`
  - `IN_PROGRESS` | `PENDING_APPROVAL` → `RESOLVED` — por `ResolveOperationalCase`
- **Invariantes de estado**:
  - O prazo nasce depois da abertura — `backoffice-operations/operational-case/due-in-past`.
  - `BENEFICIARY_DIVERGENCE` é sempre `CRITICAL`, qualquer que seja o valor (17 §4) — `backoffice-operations/operational-case/severity-mismatch`.
  - Transição fora da tabela — `backoffice-operations/operational-case/invalid-transition`.
  - O relógio pausa em `PENDING_EXTERNAL`: ao entrar, `pausedAt ← at`; ao sair, `dueAt ← dueAt + (at − pausedAt)` e `pausedAt` é limpo.
  - A resolução exige código e justificativa — `backoffice-operations/operational-case/resolution-incomplete`; os aprovadores seguem a alçada da ação que a motivou.
  - O caso nunca altera dado de negócio: o estado de outro contexto muda só pelo comando de domínio daquele contexto (17 §2, princípio 2).

### CaseAction

- **Identidade**: `generated`
- **Campos**:
  - `caseId: id` (opcional) — ausente só em `ACCESS_CUSTOMER_DATA` sem caso aberto
  - `actionType: QUERY_EXTERNAL_STATE | REPROCESS_DLQ_MESSAGE | RESEND_EXTERNAL_COMMAND | SUSPEND_PARTNER_CAPABILITY | RECORD_RECONCILIATION_ADJUSTMENT | CANCEL_CONTRACT_BEFORE_EFFECT | REQUEST_EFFECT_RELEASE | BLOCK_ORGANIZATION_FOR_RISK | ISSUE_AUDIT_DOSSIER | ACCESS_CUSTOMER_DATA` — as ações de 17 §5, com identificadores em inglês derivados do nome
  - `authorId: id`, `authorRole: operator | supervisor | risk-analyst`
  - `justification: string` (opcional)
  - `priorQueryId: id` (opcional)
  - `aboveThreshold: bool`
  - `secondApproverId: id` (opcional)
  - `status: PENDING_APPROVAL | APPROVED | EXECUTED | FAILED`
  - `resultCode: string` (opcional)
  - `withoutCase: bool`
  - `requestedAt: instant`, `approvedAt: instant` (opcional), `completedAt: instant` (opcional)
- **Estados**: os quatro de `status`.
- **Transições permitidas**:
  - (novo) → `PENDING_APPROVAL` (quando a ação exige segundo aprovador) | `APPROVED` (quando não exige) — por `RequestCaseAction`
  - `PENDING_APPROVAL` → `APPROVED` — por `ApproveCaseAction`
  - `APPROVED` → `EXECUTED` | `FAILED` — por `RecordCaseActionOutcome`
- **Invariantes de estado** (a alçada de 17 §5):

| Ação | Alçada | Justificativa | Segundo aprovador |
| --- | --- | --- | --- |
| `QUERY_EXTERNAL_STATE` | operator | não | não |
| `REPROCESS_DLQ_MESSAGE` | operator | sim | não |
| `RESEND_EXTERNAL_COMMAND` | operator | sim | sim, com `priorQueryId` |
| `SUSPEND_PARTNER_CAPABILITY` | supervisor | sim | não |
| `RECORD_RECONCILIATION_ADJUSTMENT` | operator abaixo do limiar; supervisor acima | sim | só acima do limiar |
| `CANCEL_CONTRACT_BEFORE_EFFECT` | supervisor | sim | sim |
| `REQUEST_EFFECT_RELEASE` | supervisor | sim | sim |
| `BLOCK_ORGANIZATION_FOR_RISK` | risk-analyst | sim | não |
| `ISSUE_AUDIT_DOSSIER` | supervisor | sim | sim |
| `ACCESS_CUSTOMER_DATA` | operator | sim, com motivo | não; sem caso aberto gera registro destacado |

  - O limiar do ajuste de conciliação não existe **[ABERTA: 17:B2 e E2 — limiar de alçada por valor]**.
  - Alçada insuficiente do autor — `backoffice-operations/case-action/insufficient-authority`; ação sem caso, exceto `ACCESS_CUSTOMER_DATA` — `backoffice-operations/case-action/case-required`.
  - Justificativa obrigatória onde a tabela pede — `backoffice-operations/case-action/justification-missing`.
  - O reenvio de comando externo sem `priorQueryId` é recusado mecanicamente, o que impõe C9.3 de 94 — `backoffice-operations/case-action/prior-query-missing`.
  - O segundo aprovador é pessoa distinta do autor — `backoffice-operations/case-action/self-approval`.
  - Reenvio, `UNKNOWN_OUTCOME` e idempotência de saída pertencem ao `gateway`, sobre o ciclo de comando externo do kernel (ADR-070 do DMPF, na `1.0.0-rc.7`; SPEC-N1X148AW).

### CaseEvidence

- **Identidade**: `generated`
- **Campos**:
  - `caseId: id`
  - `kind: query | response | communication`
  - `evidenceId: id`, `contentHash: string`
  - `attachedById: id`, `attachedAt: instant`
- **Estados**: nenhum — a evidência é imutável.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - `contentHash` é obrigatório — `backoffice-operations/case-evidence/content-hash-missing`.
  - O conteúdo bruto mora em armazenamento de evidência, nunca no caso **[ABERTA: provider de object storage]**.

### EscalationRule

- **Identidade**: `generated`
- **Campos**:
  - `caseType` (opcional), `minSeverity: LOW | MEDIUM | HIGH | CRITICAL` (opcional)
  - `whenOverdue: bool`
  - `financialImpactThresholdMinor: int` (opcional)
  - `targetRole: SUPERVISOR | PLATFORM_RISK`
  - `effectiveFrom: instant`
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Ao menos um critério (`caseType`, `minSeverity`, `whenOverdue` ou limiar) — `backoffice-operations/escalation-rule/criteria-missing`.
  - A regra de `BENEFICIARY_DIVERGENCE` escala a `PLATFORM_RISK` independentemente do valor (95, J4). Os valores dos limiares e dos prazos de escalonamento **[ABERTA: 17:B2 e E2]**.

### WorkQueue

- **Identidade**: `generated`
- **Campos**:
  - `name: string`
  - `caseTypes: list<string>`, `severities: list<string>` — coleções: os critérios de agrupamento
  - `assignmentPolicy: string` — **[ABERTA: 17:§3 — os valores da política de atribuição não são definidos; depende de 17:B1]**
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: ao menos um tipo ou uma severidade — `backoffice-operations/work-queue/criteria-missing`.

## Comandos (UPRs)

### OpenOperationalCase

- **Agregado**: `OperationalCase`; **cria**
- **Entrada**: `caseType`; `severity`; `sourceEventId: string`; `subjects: list<CaseSubject>`; `partnerId: id`; `financialImpactMinor: int`; `financialImpactCurrency: string`; `dueAt: instant` (calculado pelo caso de uso a partir do catálogo de prazos, com o calendário de dias úteis **[ABERTA: 17:§4 — calendário de dias úteis]**)
- **Pré-condições**:
  - `dueAt > at` → `backoffice-operations/operational-case/due-in-past`
  - `caseType = BENEFICIARY_DIVERGENCE` exige `severity = CRITICAL` → `backoffice-operations/operational-case/severity-mismatch`
- **Efeitos sobre o estado**: `status ← OPEN`; demais campos ← entrada; `openedAt ← at`
- **Eventos emitidos**: `OperationalCaseOpened`
- **Resposta**: `caseId: id`

### AssignOperationalCase

- **Agregado**: `OperationalCase`; exige existente
- **Entrada**: `assigneeUserId: id`
- **Pré-condições**:
  - `status` em `OPEN`, `ASSIGNED`, `IN_PROGRESS` ou `PENDING_EXTERNAL` → `backoffice-operations/operational-case/invalid-transition`
  - `assigneeUserId` presente → `backoffice-operations/operational-case/assignee-missing`
- **Efeitos sobre o estado**: `assigneeUserId ← cmd.assigneeUserId`; quando `OPEN`: `status ← ASSIGNED`
- **Eventos emitidos**: nenhum
- **Resposta**: `caseId: id`

### ChangeOperationalCaseStatus

- **Agregado**: `OperationalCase`; exige existente
- **Entrada**: `target: IN_PROGRESS | PENDING_EXTERNAL | PENDING_APPROVAL | CLOSED`
- **Pré-condições**:
  - `target` alcançável do `status` atual → `backoffice-operations/operational-case/invalid-transition`
- **Efeitos sobre o estado**: `status ← cmd.target`; ao entrar em `PENDING_EXTERNAL`: `pausedAt ← at`; ao sair dele: `dueAt ← dueAt + (at − pausedAt)` e `pausedAt` limpo
- **Eventos emitidos**: nenhum
- **Resposta**: `caseId: id`, `status`

### ResolveOperationalCase

- **Agregado**: `OperationalCase`; exige existente
- **Entrada**: `resolutionCode: string`; `resolutionJustification: string`; `resolutionApproverIds: list<id>`
- **Pré-condições**:
  - `status` em `IN_PROGRESS` ou `PENDING_APPROVAL` → `backoffice-operations/operational-case/invalid-transition`
  - `resolutionCode` e `resolutionJustification` não vazios → `backoffice-operations/operational-case/resolution-incomplete`
- **Efeitos sobre o estado**: `status ← RESOLVED`; campos de resolução ← entrada; `resolvedAt ← at`
- **Eventos emitidos**: `OperationalCaseResolved`
- **Resposta**: `caseId: id`

### EscalateOperationalCase

- **Agregado**: `OperationalCase`; exige existente
- **Entrada**: `targetRole: SUPERVISOR | PLATFORM_RISK`
- **Pré-condições**:
  - `status` fora de `RESOLVED` e `CLOSED` → `backoffice-operations/operational-case/invalid-transition`
- **Efeitos sobre o estado**: `escalatedToRole ← cmd.targetRole`; `escalatedAt ← at`
- **Eventos emitidos**: nenhum **[ABERTA: 17:§4 — escalar a risco deve abrir `RiskCase` em `risk` por gRPC; o contrato é da spec desse contexto]**
- **Resposta**: `caseId: id`

### RequestCaseAction

- **Agregado**: `CaseAction`; **cria**
- **Entrada**: `caseId: id`; `actionType`; `authorId: id`; `authorRole: operator | supervisor | risk-analyst`; `justification: string`; `priorQueryId: id`; `aboveThreshold: bool`
- **Pré-condições**:
  - `caseId` presente, salvo `ACCESS_CUSTOMER_DATA` → `backoffice-operations/case-action/case-required`
  - `authorRole` atende a alçada do `actionType` → `backoffice-operations/case-action/insufficient-authority`
  - justificativa presente onde a tabela pede → `backoffice-operations/case-action/justification-missing`
  - `RESEND_EXTERNAL_COMMAND` com `priorQueryId` → `backoffice-operations/case-action/prior-query-missing`
- **Efeitos sobre o estado**: `status ← PENDING_APPROVAL` quando a ação exige segundo aprovador, senão `APPROVED`; demais campos ← entrada; `withoutCase ← (caseId ausente)`; `requestedAt ← at`
- **Eventos emitidos**: nenhum **[ABERTA: 92:§4 — 17 §7 exige `AuditRecord` por ação, mas só existem `OperationalCaseOpened` e `OperationalCaseResolved`]**
- **Resposta**: `caseActionId: id`, `status`

### ApproveCaseAction

- **Agregado**: `CaseAction`; exige existente
- **Entrada**: `secondApproverId: id`
- **Pré-condições**:
  - `status = PENDING_APPROVAL` → `backoffice-operations/case-action/not-pending-approval`
  - `secondApproverId ≠ authorId` → `backoffice-operations/case-action/self-approval`
- **Efeitos sobre o estado**: `status ← APPROVED`; `secondApproverId ← cmd.secondApproverId`; `approvedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `caseActionId: id`

### RecordCaseActionOutcome

- **Agregado**: `CaseAction`; exige existente
- **Entrada**: `outcome: EXECUTED | FAILED`; `resultCode: string`
- **Pré-condições**:
  - `status = APPROVED` → `backoffice-operations/case-action/not-approved`
- **Efeitos sobre o estado**: `status ← cmd.outcome`; `resultCode ← cmd.resultCode`; `completedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `caseActionId: id`, `status`

### AttachCaseEvidence

- **Agregado**: `CaseEvidence`; **cria**
- **Entrada**: `caseId: id`; `kind: query | response | communication`; `evidenceId: id`; `contentHash: string`; `attachedById: id`
- **Pré-condições**:
  - `contentHash` não vazio → `backoffice-operations/case-evidence/content-hash-missing`
- **Efeitos sobre o estado**: campos ← entrada; `attachedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `caseEvidenceId: id`

### DefineEscalationRule

- **Agregado**: `EscalationRule`; **cria**
- **Entrada**: `caseType`; `minSeverity`; `whenOverdue: bool`; `financialImpactThresholdMinor: int`; `targetRole`; `effectiveFrom: instant`
- **Pré-condições**:
  - ao menos um critério presente → `backoffice-operations/escalation-rule/criteria-missing`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `escalationRuleId: id`

### DefineWorkQueue

- **Agregado**: `WorkQueue`; **cria**
- **Entrada**: `name: string`; `caseTypes: list<string>`; `severities: list<string>`; `assignmentPolicy: string`
- **Pré-condições**:
  - ao menos um tipo ou uma severidade → `backoffice-operations/work-queue/criteria-missing`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `workQueueId: id`

## Eventos de domínio

Os dois eventos são os propostos em `92` §4 ("Casos são criados sem evento correspondente"); o schema não está em `92` §3 **[ABERTA: 92:§3 — schemas de `OperationalCaseOpened` e `OperationalCaseResolved`; os campos são o mínimo derivado de 17 §3.1]**.

### OperationalCaseOpened

- **Campos**:
  - `caseId: id ← OperationalCase.id`
  - `caseType ← OperationalCase.caseType`, `severity ← OperationalCase.severity`
  - `sourceEventId: string ← OperationalCase.sourceEventId`
  - `subjects: list<CaseSubject> ← OperationalCase.subjects`
  - `partnerId: id ← OperationalCase.partnerId`
  - `dueAt: instant ← OperationalCase.dueAt`
  - `at: instant ← at`

### OperationalCaseResolved

- **Campos**:
  - `caseId: id ← OperationalCase.id`
  - `resolutionCode: string ← OperationalCase.resolutionCode`
  - `at: instant ← at`
- Nenhum consumidor é declarado em `92` §4; `EffectOutcomeUnknown` traz `operationalCaseId` (92 §3.4), o que sugere vínculo de volta ao efeito, sem consumidor definido **[ABERTA: 92:§4 — consumidores dos eventos de caso]**.

## Consultas

Todas correm fora da UoW.

- `FindOperationalCase` — filtros: `caseId`; cardinalidade: `one`
- `FindOperationalCaseByQueue` — filtros: `workQueueId`; opcionais `caseType`, `severity`, `assigneeUserId`, `partnerId`, `tenant`, faixa de `financialImpactMinor`, idade (`openedAt`); cardinalidade: `many`, em ordem de severidade decrescente e depois de `dueAt` crescente, com os vencidos no topo (17 §6.1)
- `FindRelatedOperationalCase` — filtros: `caseId`; cardinalidade: `many` (mesmo cliente, mesmo parceiro e mesma janela, para enxergar o incidente por trás de casos isolados, 17 §6.2) **[ABERTA: 17:§6.2 — a janela de "mesma janela" não é definida]**
- `FindCaseActionByCase` — filtros: `caseId`; cardinalidade: `many`
- `FindCaseEvidenceByCase` — filtros: `caseId`; cardinalidade: `many`
- `FindWorkQueue` — filtros: `workQueueId`; cardinalidade: `one`
- `FindEscalationRuleByCaseType` — filtros: `caseType`; cardinalidade: `many`
- `SummarizeOperationalCase` — filtros: `tenant` (opcional); cardinalidade: `one` (casos abertos por tipo e severidade, vencidos e idade do mais antigo por tipo, 17 §6.3)

## Relações entre agregados

- `OperationalCase.subjects[].subjectId` → contrato, efeito, ativo ou organização (outro contexto, só identidade)
- `OperationalCase.partnerId` → parceiro (`partner`, só identidade)
- `OperationalCase.sourceEventId` → evento de origem (`id` do CloudEvent, sem objeto)
- `CaseAction.caseId` → `OperationalCase`
- `CaseAction.priorQueryId` → `CaseEvidence` (de `kind` `query`)
- `CaseEvidence.caseId` → `OperationalCase`
- `WorkQueue` e `EscalationRule` alcançam `OperationalCase` por critério (tipo, severidade), nunca por referência armazenada.

## Integração

- **Publica** (integration events), em `apps/backend/ops/contract/proto/company/ops/event/v1/`, package `company.ops.event.v1`: `OperationalCaseOpened` (`operational_case_opened.proto`) e `OperationalCaseResolved` (`operational_case_resolved.proto`).
- **Consome** (inbox e Kafka, D4 da fundação; disposições de consumo de FND-04 §6.4). Todo evento abaixo dispara `OpenOperationalCase` com o mesmo mapa: `id` → `sourceEventId`; `caseType`, `severity` e `dueAt` do catálogo de 17 §4, e da tabela abaixo para os três tipos `EXTERNAL_*` que a SPEC-N1X148AW acrescenta (o prazo alvo parte de `at`); `subjects`, `partnerId` e `financialImpact*` do payload do evento. Os prazos são propostas sem base histórica **[ABERTA: 17:B3 — prazos alvo]**.

| messageFQN | caseType | severity | prazo alvo |
| --- | --- | --- | --- |
| `company.gateway.event.v1.ExternalOutcomeUnknown` | `EXTERNAL_OUTCOME_UNKNOWN` | `HIGH` | 4 h |
| `company.effects.event.v1.EffectRejected` | `EFFECT_REJECTED` | `HIGH` | 4 h |
| `company.effects.event.v1.EffectPartiallyConfirmed` | `EFFECT_PARTIAL` | `HIGH` | 4 h |
| `company.effects.event.v1.EffectDisputed` | `EFFECT_DISPUTED` | `CRITICAL` | 2 h |
| `company.funding.event.v1.FundingMismatchDetected` | `FUNDING_MISMATCH` | `CRITICAL` | 2 h |
| `company.funding.event.v1.DisbursementReversed` | `DISBURSEMENT_REVERSED` | `CRITICAL` | 2 h |
| `company.settlement.event.v1.ReconciliationMismatchDetected` | `RECONCILIATION_MISMATCH` | por valor (limiar configurável, 17:B2) | 1 dia útil |
| `company.settlement.event.v1.ReconciliationMismatchDetected` com `mismatchType = BENEFICIARY_DIVERGENCE` (92 §3.6) | `BENEFICIARY_DIVERGENCE` | `CRITICAL` | 2 h |
| `company.consent.event.v1.AuthorizationSyncFailed` | `CONSENT_SYNC_FAILED` | `MEDIUM` | 1 dia útil |
| `company.kyb.event.v1.DiscrepancyDetected` | `KYB_DISCREPANCY` | `MEDIUM` | 2 dias úteis |
| `company.gateway.event.v1.ExternalSchemaMismatchDetected` | `SCHEMA_MISMATCH` | `HIGH` | 4 h |
| `company.gateway.event.v1.ExternalInteractionUncorrelated` | `EXTERNAL_INTERACTION_UNCORRELATED` | `HIGH` | 4 h |
| `company.gateway.event.v1.ExternalCommandAmbiguityExpired` | `EXTERNAL_AMBIGUITY_EXPIRED` | `CRITICAL` | 2 h |
| `company.gateway.event.v1.ExternalInteractionReprocessed` com `terminalCommandAffected = true` | `EXTERNAL_REPROCESS_TERMINAL` | `HIGH` | 4 h |
| `company.partner.event.v1.PartnerCertificateExpiring` | `CERTIFICATE_EXPIRING` | `MEDIUM` | antes do vencimento |

- **Casos sem evento de domínio**: `DLQ_MESSAGE` (fila morta, `MEDIUM`, 1 dia útil) e `OUTBOX_STALLED` (alarme técnico, `HIGH`, 1 h) entram por RPC de serviço, sem `sourceEventId` **[ABERTA: 17:§4 — origem de fila morta e de alarme técnico]**.
- **Inconsistências registradas**:
  - `92` §4 não lista o `ops` como consumidor de `DiscrepancyDetected`, mas 17 §1 e §4 o consomem; este contexto segue 17 **[ABERTA: 92:§4 × 17:§4]**.
  - `92` §4 lista o `ops` como consumidor de `ExternalCommandFailed`, `ContractMismatchDetected`, `EffectOutcomeUnknown` e `CapabilitySuspended`, e 17 §4 não define `caseType` para eles **[ABERTA: 17:§4 — `caseType` desses quatro eventos]**.
  - `ExternalOutcomeUnknown` (gateway) e `EffectOutcomeUnknown` (effects) descrevem a mesma condição em dois contextos, e o mesmo problema abriria dois casos **[ABERTA: 17:§4 — deduplicação entre os dois eventos]**.
  - O prazo de 17 §3.1 é "derivado da severidade", e 17 §4 o dá por tipo (`MEDIUM` tem 1 e 2 dias úteis).
  - 17 §4 não define `EXTERNAL_INTERACTION_UNCORRELATED`, `EXTERNAL_AMBIGUITY_EXPIRED` nem `EXTERNAL_REPROCESS_TERMINAL`. Os três vêm da SPEC-N1X148AW (decisões 5, 7 e 8; linha 18 do ADR-022), com severidade e prazo propostos aqui, sob o mesmo 17:B3.
- **Chama (gRPC, como identidade de plataforma)**: as ações de `CaseAction` invocam o comando de domínio do contexto dono, que as specs desses contextos nomeiam:

| actionType | Contexto-alvo |
| --- | --- |
| `QUERY_EXTERNAL_STATE`, `RESEND_EXTERNAL_COMMAND` | `gateway` |
| `SUSPEND_PARTNER_CAPABILITY` | `partner` |
| `RECORD_RECONCILIATION_ADJUSTMENT` | `settlement` |
| `CANCEL_CONTRACT_BEFORE_EFFECT` | `contracting` |
| `REQUEST_EFFECT_RELEASE` | `effects` |
| `BLOCK_ORGANIZATION_FOR_RISK` | `risk` (`ImposeRiskBlock`, escopo `organization`) |
| `ISSUE_AUDIT_DOSSIER` | `ledger` (`RequestEvidenceBundle` e `ApproveEvidenceBundle`) |
| `ACCESS_CUSTOMER_DATA` | contexto dono do dado |
| `REPROCESS_DLQ_MESSAGE` | mensageria do kernel **[ABERTA: reprocesso de fila morta — sem contexto dono]** |

- **Consultas gRPC para o detalhe do caso** (17 §6.2): linha do tempo do ledger por `correlationId` (`FindAuditTimelineByCorrelation`), payloads bruto e normalizado do `gateway` e o estado atual de cada contexto envolvido.

## Borda e persistência

- **Borda**: só gRPC, `company.ops.service.v1.OpsService`, um RPC por comando (11) e por consulta (8), atrás da cadeia de interceptors do kernel. A fila, o detalhe do caso e o painel são telas do `bff` e ficam no Escopo fora.
- **Banco**: database e role `ops`, schema `public`, tabelas `operational_cases`, `case_actions`, `case_evidences`, `escalation_rules`, `work_queues`, `inbox` e `outbox` (kernel), com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra:
  - `operational_cases(status, severity, case_type, assignee_user_id, due_at, opened_at, partner_id, financial_impact_minor)` — `FindOperationalCaseByQueue` e `SummarizeOperationalCase`
  - `case_actions(case_id, status)` e `case_evidences(case_id)` — consultas por caso
  - `escalation_rules(case_type)` — `FindEscalationRuleByCaseType`
  - O resto vai em `snapshot` `jsonb`.
- **Divergências do design**: sem RLS como mecanismo primário (`tenant_id` e `ExecutionContext`, ADR-051); o envelope JSON e o `schemaUri` de `92` §1 são substituídos por Protobuf em CloudEvents; `instant` é inteiro de nanossegundos (o doc 00 §8 fala em milissegundos).
- **Acesso cross-tenant**: a fila mostra casos de todos os tenants (17 §6.1) e o analista de plataforma lê dados de qualquer cliente (00 §4.1): **[ABERTA: a capacidade existe no kernel desde a `1.0.0-rc.6` (DEVS-100, ADR-068 do DMPF); o uso aqui depende da adoção da rc.6 (DEVS-136)]**. Todo acesso cross-tenant é registrado no ledger (00 §4.1).

## Políticas transversais

- **Idempotência**: os comandos correm em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); o consumo é idempotente por `id` do CloudEvent (inbox, ADR-036; FND-04 §6.4). A entrega repetida do mesmo evento não abre dois casos; eventos distintos sobre o mesmo problema são o ponto aberto de deduplicação.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Papéis `PLATFORM_OPS` e `PLATFORM_RISK` (95, matriz); a alçada por tipo de ação é regra de domínio. MFA e sessão curta para papéis de plataforma (C7.1) e revisão periódica de acessos privilegiados (C7.7) são da identidade. A matriz de 95 não tem "supervisor" **[ABERTA: 95 matriz × 17 §5 — papel de supervisor]**.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`). 17 §7 exige `AuditRecord` por ação, com ator, papel, caso, ação, justificativa, estado anterior e posterior e aprovadores, e registro de acesso a dado de cliente, mesmo somente leitura (12, req. 6); esses registros não têm evento em `92` §4 **[ABERTA: 92:§4 — evento de ação e de acesso para o ledger]**.
- **Acesso a dado de cliente**: exige motivo associado a caso aberto (C7.5); sem caso, é possível e produz registro destacado e revisável.
- **Prazo**: o relógio pausa em `PENDING_EXTERNAL` (17 §3.1); casos vencidos sobem na fila.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `ops` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO ExternalOutcomeUnknown consumido pela primeira vez
QUANDO OpenOperationalCase chega com caseType EXTERNAL_OUTCOME_UNKNOWN, severity HIGH e dueAt futuro
ENTÃO o caso nasce OPEN com openedAt = at e OperationalCaseOpened vai à outbox com caseId, caseType e dueAt

DADO qualquer entrada
QUANDO OpenOperationalCase chega com dueAt igual ou anterior a at
ENTÃO rejeita com backoffice-operations/operational-case/due-in-past e nada é escrito

DADO qualquer entrada
QUANDO OpenOperationalCase chega com caseType BENEFICIARY_DIVERGENCE e severity HIGH
ENTÃO rejeita com backoffice-operations/operational-case/severity-mismatch

DADO um caso OPEN
QUANDO AssignOperationalCase chega com assigneeUserId
ENTÃO o caso vai a ASSIGNED com o responsável definido

DADO um caso ASSIGNED
QUANDO AssignOperationalCase chega sem assigneeUserId
ENTÃO rejeita com backoffice-operations/operational-case/assignee-missing

DADO um caso ASSIGNED
QUANDO ChangeOperationalCaseStatus chega com target IN_PROGRESS
ENTÃO o caso vai a IN_PROGRESS

DADO um caso OPEN
QUANDO ChangeOperationalCaseStatus chega com target PENDING_EXTERNAL
ENTÃO rejeita com backoffice-operations/operational-case/invalid-transition e o estado não muda

DADO um caso IN_PROGRESS com dueAt D
QUANDO ChangeOperationalCaseStatus leva o caso a PENDING_EXTERNAL e, depois de um intervalo T, de volta a IN_PROGRESS
ENTÃO o dueAt passa a D + T e pausedAt fica limpo

DADO um caso IN_PROGRESS
QUANDO ResolveOperationalCase chega com código, justificativa e aprovadores
ENTÃO o caso vai a RESOLVED com resolvedAt = at e OperationalCaseResolved vai à outbox

DADO um caso IN_PROGRESS
QUANDO ResolveOperationalCase chega sem justificativa
ENTÃO rejeita com backoffice-operations/operational-case/resolution-incomplete

DADO um caso RESOLVED
QUANDO ChangeOperationalCaseStatus chega com target CLOSED
ENTÃO o caso vai a CLOSED

DADO um caso IN_PROGRESS
QUANDO EscalateOperationalCase chega com targetRole PLATFORM_RISK
ENTÃO escalatedToRole = PLATFORM_RISK e escalatedAt = at

DADO um caso CLOSED
QUANDO EscalateOperationalCase chega
ENTÃO rejeita com backoffice-operations/operational-case/invalid-transition

DADO um operador e um caso aberto
QUANDO RequestCaseAction chega com QUERY_EXTERNAL_STATE
ENTÃO a ação nasce APPROVED com requestedAt = at

DADO um operador, um caso aberto e uma consulta prévia registrada
QUANDO RequestCaseAction chega com RESEND_EXTERNAL_COMMAND, justificativa e priorQueryId
ENTÃO a ação nasce PENDING_APPROVAL

DADO qualquer entrada
QUANDO RequestCaseAction chega com RESEND_EXTERNAL_COMMAND sem priorQueryId
ENTÃO rejeita com backoffice-operations/case-action/prior-query-missing

DADO qualquer entrada
QUANDO RequestCaseAction chega com CANCEL_CONTRACT_BEFORE_EFFECT e authorRole operator
ENTÃO rejeita com backoffice-operations/case-action/insufficient-authority

DADO qualquer entrada
QUANDO RequestCaseAction chega com REPROCESS_DLQ_MESSAGE sem justificativa
ENTÃO rejeita com backoffice-operations/case-action/justification-missing

DADO qualquer entrada
QUANDO RequestCaseAction chega com SUSPEND_PARTNER_CAPABILITY sem caseId
ENTÃO rejeita com backoffice-operations/case-action/case-required

DADO um operador sem caso aberto
QUANDO RequestCaseAction chega com ACCESS_CUSTOMER_DATA e motivo
ENTÃO a ação nasce APPROVED com withoutCase verdadeiro

DADO uma ação PENDING_APPROVAL
QUANDO ApproveCaseAction chega com um segundo aprovador diferente do autor
ENTÃO a ação vai a APPROVED com approvedAt = at

DADO uma ação PENDING_APPROVAL
QUANDO ApproveCaseAction chega com o próprio autor
ENTÃO rejeita com backoffice-operations/case-action/self-approval

DADO uma ação já APPROVED
QUANDO ApproveCaseAction chega
ENTÃO rejeita com backoffice-operations/case-action/not-pending-approval

DADO uma ação APPROVED
QUANDO RecordCaseActionOutcome chega com EXECUTED e resultCode
ENTÃO a ação vai a EXECUTED com completedAt = at

DADO uma ação PENDING_APPROVAL
QUANDO RecordCaseActionOutcome chega
ENTÃO rejeita com backoffice-operations/case-action/not-approved

DADO um caso aberto
QUANDO AttachCaseEvidence chega com kind query, evidenceId e contentHash
ENTÃO a evidência nasce com attachedAt = at

DADO qualquer entrada
QUANDO AttachCaseEvidence chega sem contentHash
ENTÃO rejeita com backoffice-operations/case-evidence/content-hash-missing

DADO um caso do tipo BENEFICIARY_DIVERGENCE
QUANDO DefineEscalationRule chega com caseType e targetRole PLATFORM_RISK
ENTÃO a regra nasce sem limiar de valor

DADO qualquer entrada
QUANDO DefineEscalationRule chega sem nenhum critério
ENTÃO rejeita com backoffice-operations/escalation-rule/criteria-missing

DADO uma fila para casos HIGH e CRITICAL
QUANDO DefineWorkQueue chega com severities e assignmentPolicy
ENTÃO a fila nasce

DADO qualquer entrada
QUANDO DefineWorkQueue chega sem tipos nem severidades
ENTÃO rejeita com backoffice-operations/work-queue/criteria-missing

DADO ReconciliationMismatchDetected com mismatchType BENEFICIARY_DIVERGENCE
QUANDO a inbox o recebe pela primeira vez
ENTÃO OpenOperationalCase roda com BENEFICIARY_DIVERGENCE, CRITICAL e prazo de 2 h, independentemente do valor

DADO o mesmo evento entregue duas vezes
QUANDO a segunda entrega chega à inbox
ENTÃO nenhum novo caso nasce e a recepção termina como repetida

DADO casos de severidades e prazos diferentes, um deles vencido
QUANDO FindOperationalCaseByQueue consulta a fila
ENTÃO devolve severidade decrescente e depois prazo crescente, com o vencido no topo, fora de qualquer UoW
```

## Escopo fora

- **REST e telas do `bff`**: a fila de trabalho, o detalhe do caso e o painel (17 §6) são do `bff` e do front; este contexto serve só gRPC.
- **Indicadores que vêm de observabilidade**: taxa de erro por parceiro, profundidade e idade da outbox e comandos em `UNKNOWN_OUTCOME` por faixa de idade (17 §6.3) são métricas de FND-08. "Casos por mil operações" exige a contagem de operações dos outros contextos **[ABERTA: 17:§6.3 — fonte do denominador]**.
- **O que o backoffice não faz** (17 §8): alterar dado de negócio diretamente; executar transição que o domínio não permite; aprovar crédito ou alterar condição de contrato; alterar dado bruto de parceiro; remover registro de auditoria; operar em nome do cliente sem mandato.
- **Tipos de caso de contextos que ainda não existem**: cada tipo nasce quando o produtor existe; os de `effects`, `funding` e `settlement` seguem as fases e specs desses contextos.
- **Integração com ferramenta externa de atendimento** (17:B4) e **dimensionamento da equipe** (17:B1): decisões de operação, fora do domínio.
- **Contato ativo ao cliente** (terceiro limiar do escalonamento de 16 §5.1): ação humana a partir do caso, que `notify` só dispara como passo.
