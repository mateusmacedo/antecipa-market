---
id: SPEC-H4DR929G
slug: fraud-risk
title: Bounded context — fraud-risk
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6]
ticket_url: https://linear.app/mmda/issue/DEVS-81/spec-h4dr929g-bounded-context-fraud-risk
subtask_urls: []
created: 2026-10-12
---

# SPEC-H4DR929G: Bounded context — fraud-risk

## Identidade

- **bounded_context**: `fraud-risk`
- **name**: `risk`
- **Linguagem ubíqua**:
  - *Sinal* (`RiskSignal`) — indício objetivo e informativo, com código `RSK-*`, severidade, evidência e instante; nunca decisão de crédito.
  - *Avaliação* (`RiskAssessment`) — apuração do risco de uma organização ou operação em um instante, sob uma política.
  - *Política de risco* (`RiskPolicy`) — versionada: sinais, pesos, limiares e ações; opera em modo observação ou em modo de aplicação.
  - *Decisão* (`RiskDecision`) — `PROCEED`, `PROCEED_WITH_SIGNAL`, `REVIEW`, `BLOCK_OPERATION` ou `BLOCK_ORGANIZATION` (15 §4.1).
  - *Bloqueio* (`RiskBlock`) — bloqueio vigente com escopo, motivo, evidência, responsável e prazo.
  - *Caso de risco* (`RiskCase`) — investigação com responsável e desfecho; também recebe a contestação do cedente.
  - *Relação* (`EntityRelation`) — aresta do grafo: sócio, endereço, dispositivo ou representante em comum.
  - *Modo observação* — sinais sem ação automática; só severidade inequívoca bloqueia (15 §8).
  - *Limite de papel* — a plataforma produz sinal e bloqueia a própria operação por fraude; o financiador decide crédito. Bloquear por fraude é recusar-se a prestar o serviço, não decisão de crédito (15 §1).
  - Os valores enumerados que o design define (`PROCEED`, `REVIEW` e os demais) são preservados literalmente; os enumerados derivados usam kebab minúsculo.

## Origem e bloqueios externos

- **Fontes**: `docs/design/15-design-fraude-risco.md` §1 a §8 (contexto sem PRD); `docs/prd/02-prd-kyb-kyc.md` (`RiskSignal`); `docs/design/92-schemas-eventos.md` §4 (eventos propostos e consumidores); `docs/design/94-seguranca-threat-model.md` T2; `docs/design/95-personas-jornadas.md` (P7, matriz de papéis); `docs/design/06-design-eligibility.md` (`ORG-RSK-001`); `docs/design/10-design-funding.md` §6.1; `docs/resumo-executivo-apuracao.md` (E1, A2, A7, C1, C2).
- **Situação**: `parcial` — o modelo, o modo observação e os bloqueios de severidade inequívoca são construíveis; a calibragem (E1), a obrigação de lastro (A7), a verificação de documento fiscal (C2) e a titularidade de conta (C1) travam partes do contexto.
- **Fase de entrega**: 1 (inferida; em modo observação).
- **Dependências**: upstream duro `identity` (`SPEC-7KW0HHF6`: sujeito avaliado e `BankAccountChange*`). Upstream brando: `kyb` (`DiscrepancyDetected`), `portfolio` (`ExternalPositionDiverged`), `marketplace` (`FundingRequestOpened`), `effects` (`EffectDisputed`), `funding` (`FundingMismatchDetected`) e `settlement` (`ReconciliationMismatchDetected`). `depends_on` declara só a fundação e a aresta dura reduzida (`identity`); as arestas de retroalimentação, em que os demais contextos alimentam o risco, não entram.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| E1, 15:X1 | Calibragem dos limiares: não há dado histórico nem benchmark | Pesos e limiares de `RiskPolicy`; ação automática. Mitigação: modo observação (15 §8) |
| A2, 15:X2 | Obrigações de PLD/FT e de comunicação a autoridade | Governança (15 §7); desfecho de `RiskCase` |
| A7, 15:X3 | Obrigação de verificação de lastro da operação mercantil | Sinais documentais `RSK-DOC-*` |
| C2, 15:X4 | Verificação de documento fiscal por terceiro | Sinais documentais `RSK-DOC-*` |
| C1, 15:X5, `#11` | Serviço de validação de titularidade de conta | `RSK-BNK-002`; controle de maior valor contra o desvio de destino |
| 15:X6 | Confirmação com o sacado sem inviabilizar a operação | Sinais de conluio cedente-sacado |
| 15:X7 | Tipologias de mercado não levantadas | Completude do catálogo de sinais |
| `#1`, A1 | Autorização do BCB (existencial, transversal) | Contexto inteiro |

- **Decisões de produto abertas**: dono de `RiskSignal` (disputado com KYB); se e como comunicar ao cedente o motivo do bloqueio (15 §7, validação jurídica); alcance do grafo (15 §6 recomenda relação materializada com consulta de dois graus).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída e dono de `RiskSignal` definido. Nenhum bloqueio externo trava o núcleo em modo observação; `C1` (`#11`) trava só o sinal `RSK-BNK-002` e `A2` só a comunicação a autoridade.

## Agregados

Nota de modelagem: `tenantId` não é campo dos agregados — o escopo vem do `ExecutionContext` (ADR-051). O doc 90 não detalha atributos deste contexto; os campos abaixo são o mínimo derivado de 15 §3, §4 e §7.

### RiskSignal

**[ABERTA: dono de `RiskSignal` — docs/prd/02-prd-kyb-kyc.md (Modelo de domínio) o lista em KYB/KYC; docs/design/15-design-fraude-risco.md §4 o define como agregado do contexto Fraude e Risco; docs/design/92-schemas-eventos.md §4 só prevê o evento `RiskSignalRaised` sob Risco]**

- **Identidade**: `generated`
- **Campos**:
  - `code: string` — código do catálogo de 15 §3 (`RSK-CAD-*`, `RSK-BHV-*`, `RSK-REL-*`, `RSK-BNK-*`, `RSK-DOC-*`)
  - `category: registration | behavioral | relational | disbursement | documentary`
  - `severity: string` — **[ABERTA: 15:§4 — a escala de severidade do sinal não é definida; 91 §4.1 exemplifica `MEDIUM` e o código `NEW_BANK_ACCOUNT_RECENT`, que não pertence ao catálogo `RSK-*`]**
  - `subjectType: organization | operation`, `subjectId: id`
  - `evidenceId: id` (opcional)
  - `sourceEventId: string` (opcional) — evento de origem
  - `observedAt: instant`
- **Estados**: nenhum — o sinal é imutável.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - `code` pertence ao catálogo de 15 §3 — `fraud-risk/risk-signal/unknown-code` (o catálogo pode ter lacunas, 15:X7).
  - `RSK-BNK-005` não é sinal, é incidente tratado como `FUNDING_MISMATCH` crítico (15 §3.4) — `fraud-risk/risk-signal/incident-not-signal`; o caminho dele é `ImposeRiskBlock`.
  - Sinal é informativo: nenhum sinal, sozinho, bloqueia (02, `RiskSignal`).
  - Os sinais `RSK-DOC-*` dependem de a verificação de lastro ser obrigação da plataforma e de existir verificação de documento fiscal por terceiro **[A CONFIRMAR: A7, C2]**; `RSK-BNK-002` depende de serviço de validação de titularidade de conta **[A CONFIRMAR: C1, #11]**; os sinais de conluio cedente-sacado dependem de a confirmação com o sacado ser praticável **[A CONFIRMAR: 15:X6]**.

### RiskPolicy

- **Identidade**: `generated`
- **Campos**:
  - `version: int`, `supersedesId: id` (opcional) — versionamento explícito (90 §1.5)
  - `mode: observation | enforcing`
  - `rules: list<RiskRule>` — coleção indispensável. Cada regra: `signalCode: string`, `weight: decimal`, `threshold: decimal`, `action: none | signal | review | block-operation | block-organization`, `unequivocal: bool`, `exceptionEntityIds: list<id>` (só em regra relacional; 15 §6)
  - `effectiveFrom: instant`
- **Estados**: o campo `mode`; a política nasce em `observation` até a calibragem (15 §8).
- **Transições permitidas**: nenhuma. Mudar modo ou regra é nova versão com `supersedesId`.
- **Invariantes de estado**:
  - Em `observation`, regra com `action` além de `none` ou `signal` só vale se `unequivocal` (15 §8: destino divergente, representação revogada, gravame de terceiro) — `fraud-risk/risk-policy/observation-mode-action-forbidden`.
  - Os valores de `weight` e `threshold` dependem de dado histórico que só a operação produz **[A CONFIRMAR: E1, 15:X1]**, e a fórmula de combinação dos sinais não está definida **[ABERTA: 15:X1 — algoritmo de combinação]**.
  - "Representação revogada" e "gravame de terceiro" não têm código no catálogo de 15 §3 **[ABERTA: 15:§8 × §3 — códigos dos sinais de severidade inequívoca]**.

### RiskAssessment

- **Identidade**: `generated`
- **Campos**:
  - `subjectType: organization | operation`, `subjectId: id`
  - `trigger: onboarding | request | pre-signature | pre-disbursement | post-settlement | monitoring` — os momentos de 15 §5
  - `policyId: id`
  - `signalIds: list<id>` — coleção indispensável: os sinais considerados
  - `outcome: PROCEED | PROCEED_WITH_SIGNAL | REVIEW | BLOCK_OPERATION | BLOCK_ORGANIZATION`
  - `observationOnly: bool`
  - `assessedAt: instant`
- **Estados**: nenhum — cada reavaliação é uma nova avaliação.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - `outcome` é função da política e dos sinais; `PROCEED_WITH_SIGNAL` é o resultado padrão para sinal não conclusivo (15 §4.1).
  - Com `observationOnly`, `outcome` só é `PROCEED` ou `PROCEED_WITH_SIGNAL`, salvo regra `unequivocal`.
  - Avaliação sem política — `fraud-risk/risk-assessment/policy-missing`.

### RiskDecision

- **Identidade**: `generated`
- **Campos**:
  - `assessmentId: id` (opcional), `caseId: id` (opcional)
  - `decision: PROCEED | PROCEED_WITH_SIGNAL | REVIEW | BLOCK_OPERATION | BLOCK_ORGANIZATION`
  - `decidedBy: policy | analyst`, `deciderId: id` (opcional)
  - `justification: string`
  - `supersedesDecisionId: id` (opcional)
  - `decidedAt: instant`
- **Estados**: nenhum — a decisão é imutável; reverter é nova decisão.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Decisão manual (`decidedBy = analyst`) exige `deciderId` e `justification` — `fraud-risk/risk-decision/justification-missing`.
  - `REVIEW` retém a operação e precisa de prazo curto e de comunicação ao cedente (15 §4.1) **[ABERTA: 15:§4.1 — prazo da revisão]**.

### RiskBlock

- **Identidade**: `generated`
- **Campos**:
  - `scope: operation | organization`, `operationId: id` (opcional), `organizationId: id`
  - `reasonCode: string`, `evidenceId: id`, `responsibleId: id`
  - `decisionId: id` (opcional)
  - `expiresAt: instant` (opcional) — o prazo do bloqueio
  - `status: active | lifted`
  - `imposedAt: instant`, `liftedAt: instant`, `liftedById: id`, `liftReason: string` (opcionais até a liberação)
- **Estados**: `active`, `lifted`.
- **Transições permitidas**:
  - (novo) → `active` — por `ImposeRiskBlock`
  - `active` → `lifted` — por `LiftRiskBlock`
- **Invariantes de estado**:
  - Todo bloqueio tem código, evidência e responsável (15 §7) — `fraud-risk/risk-block/incomplete-justification`.
  - Escopo `operation` exige `operationId` — `fraud-risk/risk-block/scope-invalid`.
  - Bloqueio vigente é `status = active` com `expiresAt` ausente ou futuro; a expiração por prazo não é transição de estado.
  - `block-organization` só é revertido pela supervisão de risco (15 §4.1) — `fraud-risk/risk-block/supervisor-required`.
  - Quem investiga não aprova o próprio bloqueio acima de certo impacto (15 §7) **[ABERTA: 15:§7 — limiar de impacto da segregação]**.

### RiskCase

- **Identidade**: `generated`
- **Campos**:
  - `origin: review | contestation`
  - `subjectType: organization | operation`, `subjectId: id`
  - `assessmentId: id` (opcional), `blockId: id` (opcional)
  - `responsibleId: id` (opcional)
  - `status: open | resolved`
  - `dueAt: instant` (opcional) — **[ABERTA: 15:§7 — a contestação "tem prazo e resposta", sem valor definido]**
  - `openedAt: instant`, `resolvedAt: instant` (opcional)
  - `outcome: string`, `resolutionJustification: string` (opcionais até a resolução) — **[ABERTA: 15:§4 — o "desfecho" não tem domínio de valores]**
- **Estados**: `open`, `resolved`.
- **Transições permitidas**:
  - (novo) → `open` — por `OpenRiskCase`
  - `open` → `resolved` — por `ResolveRiskCase`
- **Invariantes de estado**:
  - Só `open` é resolvível — `fraud-risk/risk-case/not-open`.
  - A resolução exige desfecho e justificativa — `fraud-risk/risk-case/resolution-incomplete`.
  - Fraude confirmada alimenta o grafo (15 §5) **[ABERTA: 15:§5 — o ciclo de retorno do desfecho para `EntityRelation` não é definido; hoje seria um `RecordEntityRelation` manual]**.

### EntityRelation

- **Identidade**: `natural` por `relationKey` (`relationType` e as duas entidades, em ordem canônica)
- **Campos**:
  - `relationType: common-partner | common-address | common-device | common-representative | linked-to-confirmed-fraud` — derivados de `RSK-REL-001` a `RSK-REL-005`
  - `entityAId: id`, `entityBId: id`
  - `source: string` — origem da aresta
  - `confidence: decimal` — **[ABERTA: 15:§4 — escala da confiança]**
  - `observedAt: instant`
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - As duas pontas são entidades distintas — `fraud-risk/entity-relation/same-entity`.
  - A consulta é limitada a dois graus (15 §6); regra relacional leva lista de exceção mantida (escritórios de contabilidade concentram endereço e representante).
  - Fonte dos dados das arestas: nenhum evento de `92` §4 traz sócio, endereço ou dispositivo **[ABERTA: 15:§3.3 — fonte das arestas (KYB, Identity, sessão)]**.
  - Relação entre organizações de tenants distintos: **[ABERTA: requer extensão do kernel - ADR-051 DMPF]**.

## Comandos (UPRs)

### RaiseRiskSignal

- **Agregado**: `RiskSignal`; **cria**
- **Entrada**: `code: string`; `severity: string`; `subjectType: organization | operation`; `subjectId: id`; `evidenceId: id`; `sourceEventId: string`; `observedAt: instant`
- **Pré-condições**:
  - `code` no catálogo de 15 §3 → `fraud-risk/risk-signal/unknown-code`
  - `code ≠ RSK-BNK-005` → `fraud-risk/risk-signal/incident-not-signal`
- **Efeitos sobre o estado**: campos ← entrada; `category ←` categoria do prefixo de `code`
- **Eventos emitidos**: `RiskSignalRaised`
- **Resposta**: `signalId: id`

### DefineRiskPolicy

- **Agregado**: `RiskPolicy`; **cria**
- **Entrada**: `version: int`; `supersedesId: id`; `mode: observation | enforcing`; `rules: list<RiskRule>`; `effectiveFrom: instant`
- **Pré-condições**:
  - em `observation`, toda regra com `action` além de `none` ou `signal` é `unequivocal` → `fraud-risk/risk-policy/observation-mode-action-forbidden`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `policyId: id`

### AssessRisk

- **Agregado**: `RiskAssessment`; **cria**
- **Entrada**: `subjectType: organization | operation`; `subjectId: id`; `trigger`; `policyId: id`; `signalIds: list<id>`; `policy` (instantâneo da `RiskPolicy` vigente, resolvido pelo caso de uso)
- **Pré-condições**:
  - `policyId` presente → `fraud-risk/risk-assessment/policy-missing`
- **Efeitos sobre o estado**: demais campos ← entrada; `outcome ←` avaliação das regras da política sobre os sinais **[ABERTA: 15:X1 — algoritmo de combinação]**; `observationOnly ← (policy.mode = observation)`; `assessedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `assessmentId: id`, `outcome`

### DecideRisk

- **Agregado**: `RiskDecision`; **cria**
- **Entrada**: `assessmentId: id`; `caseId: id`; `decision`; `decidedBy: policy | analyst`; `deciderId: id`; `justification: string`; `supersedesDecisionId: id`
- **Pré-condições**:
  - quando `decidedBy = analyst`, `deciderId` e `justification` presentes → `fraud-risk/risk-decision/justification-missing`
- **Efeitos sobre o estado**: campos ← entrada; `decidedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `decisionId: id`

### ImposeRiskBlock

- **Agregado**: `RiskBlock`; **cria**
- **Entrada**: `scope: operation | organization`; `operationId: id`; `organizationId: id`; `reasonCode: string`; `evidenceId: id`; `responsibleId: id`; `decisionId: id`; `expiresAt: instant`
- **Pré-condições**:
  - `reasonCode`, `evidenceId` e `responsibleId` presentes → `fraud-risk/risk-block/incomplete-justification`
  - `scope = operation` exige `operationId` → `fraud-risk/risk-block/scope-invalid`
- **Efeitos sobre o estado**: `status ← active`; demais campos ← entrada; `imposedAt ← at`
- **Eventos emitidos**: `OperationBlockedByRisk`
- **Resposta**: `blockId: id`

### LiftRiskBlock

- **Agregado**: `RiskBlock`; exige existente
- **Entrada**: `liftedById: id`; `liftReason: string`; `actorRole: analyst | supervisor`
- **Pré-condições**:
  - `status = active` → `fraud-risk/risk-block/not-active`
  - `scope = organization` exige `actorRole = supervisor` → `fraud-risk/risk-block/supervisor-required` **[ABERTA: 95 — o papel "supervisão de risco" não existe na matriz, só `PLATFORM_RISK`]**
- **Efeitos sobre o estado**: `status ← lifted`; `liftedAt ← at`; `liftedById`, `liftReason` ← entrada
- **Eventos emitidos**: nenhum **[ABERTA: 92:§4 — não há evento de liberação; `ORG-RSK-001` (06) e a pré-condição 5 de Funding (10 §6.1) precisam saber quando o bloqueio termina]**
- **Resposta**: `blockId: id`

### OpenRiskCase

- **Agregado**: `RiskCase`; **cria**
- **Entrada**: `origin: review | contestation`; `subjectType`; `subjectId: id`; `assessmentId: id`; `blockId: id`; `responsibleId: id`; `dueAt: instant`
- **Pré-condições**: nenhuma além da forma da entrada.
- **Efeitos sobre o estado**: `status ← open`; demais campos ← entrada; `openedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `caseId: id`

### ResolveRiskCase

- **Agregado**: `RiskCase`; exige existente
- **Entrada**: `outcome: string`; `resolutionJustification: string`
- **Pré-condições**:
  - `status = open` → `fraud-risk/risk-case/not-open`
  - `outcome` e `resolutionJustification` não vazios → `fraud-risk/risk-case/resolution-incomplete`
- **Efeitos sobre o estado**: `status ← resolved`; `outcome`, `resolutionJustification` ← entrada; `resolvedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `caseId: id`

### RecordEntityRelation

- **Agregado**: `EntityRelation`; **inicializa se ausente**
- **Entrada**: `relationType`; `entityAId: id`; `entityBId: id`; `source: string`; `confidence: decimal`
- **Pré-condições**:
  - `entityAId ≠ entityBId` → `fraud-risk/entity-relation/same-entity`
- **Efeitos sobre o estado**: quando ausente: campos ← entrada; `observedAt ← at`. Quando presente: nenhum — aceita sem alterar e sem emitir.
- **Eventos emitidos**: nenhum
- **Resposta**: `relationKey: string`

## Eventos de domínio

Os dois eventos são os propostos em `92` §4 ("ausentes do catálogo original"); o schema não está em `92` §3, e os campos abaixo são o mínimo derivado de 15 §4 e §7 **[ABERTA: 92:§3 — schemas de `RiskSignalRaised` e `OperationBlockedByRisk`]**.

### RiskSignalRaised

- **Campos**:
  - `signalId: id ← RiskSignal.id`
  - `code: string ← RiskSignal.code`
  - `category ← RiskSignal.category`
  - `severity: string ← RiskSignal.severity`
  - `subjectType ← RiskSignal.subjectType`, `subjectId: id ← RiskSignal.subjectId`
  - `evidenceId: id ← RiskSignal.evidenceId`
  - `at: instant ← at`

### OperationBlockedByRisk

- **Campos**:
  - `blockId: id ← RiskBlock.id`
  - `scope ← RiskBlock.scope`, `operationId: id ← RiskBlock.operationId`, `organizationId: id ← RiskBlock.organizationId`
  - `reasonCode: string ← RiskBlock.reasonCode`
  - `evidenceId: id ← RiskBlock.evidenceId`
  - `expiresAt: instant ← RiskBlock.expiresAt`
  - `at: instant ← at`
- O mesmo evento cobre o escopo `organization`; a retenção por `REVIEW` e `BLOCK_ORGANIZATION` não tem evento próprio **[ABERTA: 92:§4 — evento para `REVIEW` e para bloqueio de organização]**.

## Consultas

Todas correm fora da UoW.

- `FindRiskSignalBySubject` — filtros: `subjectType`, `subjectId`; cardinalidade: `many`
- `FindRiskAssessment` — filtros: `assessmentId`; cardinalidade: `one`
- `FindRiskAssessmentBySubject` — filtros: `subjectType`, `subjectId`; cardinalidade: `many`
- `FindActiveRiskBlock` — filtros: `organizationId`, `operationId` (opcional), `asOf`; cardinalidade: `many` (usada por `ORG-RSK-001` e pela pré-condição 5 de Funding)
- `FindRiskCase` — filtros: `caseId`; cardinalidade: `one`
- `FindRiskCaseByStatus` — filtros: `status`, `responsibleId` (opcional); cardinalidade: `many`
- `FindActiveRiskPolicy` — filtros: `asOf`; cardinalidade: `one` (a de maior `version` com `effectiveFrom` não posterior)
- `FindEntityRelationByEntity` — filtros: `entityId`, `depth` (máximo 2); cardinalidade: `many`

## Relações entre agregados

- `RiskSignal.subjectId` → organização ou operação (outro contexto, só identidade)
- `RiskAssessment.policyId` → `RiskPolicy`
- `RiskAssessment.signalIds[]` → `RiskSignal`
- `RiskDecision.assessmentId` → `RiskAssessment`
- `RiskDecision.caseId` → `RiskCase`
- `RiskDecision.supersedesDecisionId` → `RiskDecision`
- `RiskBlock.decisionId` → `RiskDecision`
- `RiskCase.assessmentId` → `RiskAssessment`
- `RiskCase.blockId` → `RiskBlock`
- `EntityRelation.entityAId` e `entityBId` → organização ou pessoa (outro contexto, só identidade)
- A cadeia avaliação → decisão → bloqueio ou caso é um comando por UoW, encadeada pelo caso de uso chamador; cada comando é idempotente.

## Integração

- **Publica** (integration events), em `apps/backend/risk/contract/proto/company/risk/event/v1/`, package `company.risk.event.v1`: `RiskSignalRaised` (`risk_signal_raised.proto`) e `OperationBlockedByRisk` (`operation_blocked_by_risk.proto`). Consumidores: `92` §4 não os declara; os designs declaram Eligibility (`ORG-RSK-001`, 06), Funding (pré-condição 5, 10 §6.1), Marketplace (`REVIEW` e `BLOCK` antes da distribuição, 15 §5) e o ledger (15 §7) **[ABERTA: 92:§4 — consumidores dos eventos propostos]**.
- **Consome** (inbox e Kafka, D4 da fundação; disposições de consumo de FND-04 §6.4; rejeição de domínio vira recepção rejeitada, não retentativa):
  - `company.kyb.event.v1.DiscrepancyDetected` → `RaiseRiskSignal`: `code ← RSK-CAD-007`; `sourceEventId ← id`; `observedAt ← time`; `subjectType ← organization`; `subjectId ←` organização do payload **[ABERTA: 92:§3 — `DiscrepancyDetected` não tem schema; o código `RSK-CAD-007` é inferido do nome]**
  - `company.marketplace.event.v1.FundingRequestOpened` → `AssessRisk`: `trigger ← request`; `subjectType ← operation`; `subjectId ←` solicitação do payload
  - `company.portfolio.event.v1.ExternalPositionDiverged` → `RaiseRiskSignal`: `sourceEventId ← id`; `subjectType ← organization` **[ABERTA: 15:§3 — nenhum sinal do catálogo corresponde a divergência de posição externa]**
  - `company.effects.event.v1.EffectDisputed` → `RaiseRiskSignal`: `sourceEventId ← id`; `subjectType ← operation` **[ABERTA: 15:§3 — nenhum sinal do catálogo corresponde a efeito disputado]**
  - `company.funding.event.v1.FundingMismatchDetected` → `ImposeRiskBlock`: `scope ← operation`; `reasonCode ← RSK-BNK-005`; `responsibleId ←` identidade de serviço do consumidor. Bloqueio automático também em modo observação (15 §3.4, §8) **[ABERTA: 92:§3.5 — o payload de `FundingMismatchDetected` não tem schema]**
  - `company.settlement.event.v1.ReconciliationMismatchDetected` → `RaiseRiskSignal` quando `mismatchType = BENEFICIARY_DIVERGENCE` (92 §3.6): `subjectType ← operation`; `subjectId ← contractId` **[ABERTA: 15:§3 — sem código de sinal para divergência de beneficiário]**. Os demais `mismatchType` concluem a recepção sem comando.
  - `company.identity.event.v1.BankAccountChangeRequested` e `company.identity.event.v1.BankAccountChangeApproved` → `RaiseRiskSignal`: `code ← RSK-BNK-001`; `subjectType ← organization`; `subjectId ← organizationId`; `evidenceId ← destinationEvidenceId` (só no `Requested`). `RSK-BNK-004` exige correlacionar com `OfferSelected`, que o risco não consome **[ABERTA: 92:§4 — Risco não é consumidor declarado de `BankAccountChange*`; a diferença de efeito entre `Requested` e `Approved` não é definida em 15]**
- **Gatilhos sem evento** (15 §5): onboarding concluído, reavaliação antes da assinatura e antes do desembolso, avaliação retrospectiva após a liquidação e monitoramento contínuo chamam `AssessRisk` por RPC de serviço **[ABERTA: 15:§5 — esses gatilhos não têm evento consumido em 92 §4]**.
- **Consultas gRPC a outros contextos**: `identity` (`FindOrganization`) e `kyb` (perfil e estrutura societária) para os sinais cadastrais `RSK-CAD-*`, cujos contratos são os das specs desses contextos **[ABERTA: 15:§3.1 — fonte dos dados cadastrais dos sinais]**.

## Borda e persistência

- **Borda**: só gRPC, `company.risk.service.v1.RiskService`, um RPC por comando (9) e por consulta (8), atrás da cadeia de interceptors do kernel. O REST (inclusive `riskSignals` na Partner API, 91 §4.1) é do `bff` e fica no Escopo fora.
- **Banco**: database e role `risk`, schema `public`, tabelas `risk_signals`, `risk_policies`, `risk_assessments`, `risk_decisions`, `risk_blocks`, `risk_cases`, `entity_relations`, `inbox` e `outbox` (kernel), com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra:
  - `risk_signals(subject_type, subject_id)` e `risk_assessments(subject_type, subject_id)` — consultas por sujeito
  - `risk_blocks(organization_id, operation_id, status, expires_at)` — `FindActiveRiskBlock`
  - `risk_cases(status, responsible_id)` — `FindRiskCaseByStatus`
  - `risk_policies(effective_from)` — `FindActiveRiskPolicy`
  - `entity_relations(entity_a_id, entity_b_id)` — `FindEntityRelationByEntity`, dois graus por consultas sucessivas (relação materializada, 15 §6; estrutura de grafo dedicada fica fora)
  - O resto vai em `snapshot` `jsonb`.
- **Divergências do design**: sem RLS como mecanismo primário (`tenant_id` e `ExecutionContext`, ADR-051); o envelope JSON e o `schemaUri` de `92` §1 são substituídos por Protobuf em CloudEvents; `instant` é inteiro de nanossegundos (o doc 00 §8 fala em milissegundos).
- **Acesso cross-tenant** (analista de risco sobre casos e sinais de todos os tenants; grafo entre organizações de tenants distintos): a leitura de plataforma existe no kernel desde a `1.0.0-rc.6` (DEVS-100, ADR-068 do DMPF) e o uso aqui depende da adoção da rc.6 (DEVS-136); o grafo entre tenants é junção entre tenants, que o ADR-068 deixa fora **[ABERTA: requer extensão do kernel - ADR-051 DMPF]**.
- **Modo observação**: a política nasce em `observation` (15 §8); passar a `enforcing` é nova versão da política, decisão tomada após a calibragem (E1).

## Políticas transversais

- **Idempotência**: os comandos correm em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); o consumo é idempotente por `id` do CloudEvent (inbox, ADR-036; FND-04 §6.4). `RaiseRiskSignal` por evento herda a deduplicação da inbox.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). `PLATFORM_RISK` suspende operação por risco (95, matriz); a reversão de `block-organization` exige a supervisão. Segregação: quem investiga não aprova o próprio bloqueio acima de certo impacto (15 §7).
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`). O design pede que "toda avaliação, sinal e decisão vá ao ledger" (15 §7), mas só `RiskSignalRaised` e `OperationBlockedByRisk` existem em `92` §4 **[ABERTA: 92:§4 — avaliação e decisão sem evento para o ledger]**.
- **Governança**: bloqueio motivado (código, evidência, responsável); contestação aberta como `RiskCase` de origem `contestation`; comunicar ao cedente que houve bloqueio e oferecer canal de contestação, sem detalhar os sinais, posição que 15 §7 submete à validação jurídica **[ABERTA: 15:§7 — texto e fundamento da comunicação]**. A comunicação a autoridade depende das obrigações de PLD/FT **[A CONFIRMAR: A2, 15:X2]**.
- **Limite de papel**: o contexto informa sinais objetivos e nunca emite juízo sobre a idoneidade do cedente ao financiador (15 §7).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `risk` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO uma organização avaliada
QUANDO RaiseRiskSignal chega com um code do catálogo
ENTÃO o sinal nasce com observedAt informado e RiskSignalRaised vai à outbox com signalId, code, category e subjectId

DADO qualquer entrada
QUANDO RaiseRiskSignal chega com um code fora do catálogo
ENTÃO rejeita com fraud-risk/risk-signal/unknown-code e nada é escrito

DADO qualquer entrada
QUANDO RaiseRiskSignal chega com code RSK-BNK-005
ENTÃO rejeita com fraud-risk/risk-signal/incident-not-signal

DADO uma política em modo observation
QUANDO DefineRiskPolicy chega com regras de ação none, signal ou unequivocal
ENTÃO a política nasce com a versão informada

DADO uma política em modo observation
QUANDO DefineRiskPolicy chega com uma regra review ou block-operation que não é unequivocal
ENTÃO rejeita com fraud-risk/risk-policy/observation-mode-action-forbidden

DADO uma política vigente em observation
QUANDO AssessRisk chega com sinais não conclusivos
ENTÃO a avaliação nasce com outcome PROCEED_WITH_SIGNAL, observationOnly verdadeiro e assessedAt = at

DADO qualquer entrada
QUANDO AssessRisk chega sem policyId
ENTÃO rejeita com fraud-risk/risk-assessment/policy-missing

DADO uma avaliação com outcome REVIEW
QUANDO DecideRisk chega com decidedBy analyst, deciderId e justification
ENTÃO a decisão nasce com decidedAt = at

DADO qualquer entrada
QUANDO DecideRisk chega com decidedBy analyst sem justification
ENTÃO rejeita com fraud-risk/risk-decision/justification-missing

DADO uma operação com destino divergente
QUANDO ImposeRiskBlock chega com scope operation, reasonCode, evidenceId e responsibleId
ENTÃO o bloqueio nasce active e OperationBlockedByRisk vai à outbox

DADO qualquer entrada
QUANDO ImposeRiskBlock chega sem evidenceId
ENTÃO rejeita com fraud-risk/risk-block/incomplete-justification

DADO qualquer entrada
QUANDO ImposeRiskBlock chega com scope operation e sem operationId
ENTÃO rejeita com fraud-risk/risk-block/scope-invalid

DADO um bloqueio active de escopo operation
QUANDO LiftRiskBlock chega com actorRole analyst
ENTÃO o bloqueio vai a lifted com liftedAt = at e nenhum evento é emitido

DADO um bloqueio já lifted
QUANDO LiftRiskBlock chega
ENTÃO rejeita com fraud-risk/risk-block/not-active

DADO um bloqueio active de escopo organization
QUANDO LiftRiskBlock chega com actorRole analyst
ENTÃO rejeita com fraud-risk/risk-block/supervisor-required e o bloqueio continua active

DADO uma avaliação com outcome REVIEW
QUANDO OpenRiskCase chega com origin review
ENTÃO o caso nasce open com openedAt = at

DADO um caso open
QUANDO ResolveRiskCase chega com outcome e resolutionJustification
ENTÃO o caso vai a resolved com resolvedAt = at

DADO um caso já resolved
QUANDO ResolveRiskCase chega
ENTÃO rejeita com fraud-risk/risk-case/not-open

DADO um caso open
QUANDO ResolveRiskCase chega sem resolutionJustification
ENTÃO rejeita com fraud-risk/risk-case/resolution-incomplete

DADO duas organizações com sócio em comum
QUANDO RecordEntityRelation chega com relationType common-partner
ENTÃO a relação nasce com observedAt = at

DADO uma relação já registrada
QUANDO RecordEntityRelation chega com a mesma chave
ENTÃO aceita sem alterar a relação e sem emitir

DADO qualquer entrada
QUANDO RecordEntityRelation chega com entityAId igual a entityBId
ENTÃO rejeita com fraud-risk/entity-relation/same-entity

DADO FundingMismatchDetected consumido com a política em observation
QUANDO a inbox o recebe pela primeira vez
ENTÃO ImposeRiskBlock roda com reasonCode RSK-BNK-005 e a recepção termina como processada

DADO ReconciliationMismatchDetected com mismatchType AMOUNT_DIVERGENCE
QUANDO a inbox o recebe
ENTÃO a recepção termina como processada sem comando e sem sinal

DADO dois bloqueios de uma organização, um active e um lifted
QUANDO FindActiveRiskBlock consulta a organização
ENTÃO devolve só o active e vigente em asOf, fora de qualquer UoW
```

## Escopo fora

- **REST do `bff`**: o campo `riskSignals` da Partner API (91 §4.1, "presente, decisão ausente") e as telas do analista ficam no `bff`; este contexto serve só gRPC.
- **Decisão de crédito**: pertence ao financiador; o contexto bloqueia a própria operação por fraude e nunca comunica ao financiador que o cedente "é fraudador" (15 §1).
- **Comunicação ao cedente do bloqueio ou da revisão**: é do contexto `notify`; o texto depende de validação jurídica (15 §7).
- **Comunicação a autoridade**: depende de A2 (15:X2); só existe após resolver o bloqueio.
- **Verificação de lastro, de documento fiscal e de titularidade de conta**: dependem de A7, C2 e C1 (`#11`); os sinais `RSK-DOC-*` e `RSK-BNK-002` só ganham fonte depois.
- **Grafo dedicado e detecção de esquema com profundidade maior que dois graus**: capacidade a acrescentar quando houver volume (15 §6).
- **Calibragem dos limiares** (E1, 15:X1): só existe com dado de operação; até lá vale o modo observação.
- **Avaliação retrospectiva por chargeback**: depende da janela de contestação de Settlement (`#10`) e de fase posterior.
