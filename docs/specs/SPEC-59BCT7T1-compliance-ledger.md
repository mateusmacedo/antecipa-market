---
id: SPEC-59BCT7T1
slug: compliance-ledger
title: Bounded context — compliance-ledger
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24]
ticket_url: https://linear.app/mmda/issue/DEVS-80/spec-59bct7t1-bounded-context-compliance-ledger
subtask_urls: []
created: 2026-10-12
---

# SPEC-59BCT7T1: Bounded context — compliance-ledger

## Identidade

- **bounded_context**: `compliance-ledger`
- **name**: `ledger`
- **Linguagem ubíqua**:
  - *Registro de auditoria* (`AuditRecord`) — registro imutável de uma ação ou fato; correção é um novo registro, nunca alteração.
  - *Sujeito* (`AuditSubject`) — entidade (organização, ativo, pedido, contrato, parceiro) à qual um registro se relaciona.
  - *Referência de evidência* (`EvidenceReference`) — localização, hash, tipo e política de acesso de um artefato; nunca o conteúdo.
  - *Dossiê* (`EvidenceBundle`) — conjunto exportável de evidências para uma finalidade, com manifesto, hashes e assinatura.
  - *Política de retenção* (`RetentionPolicy`) — prazo por categoria com fundamento jurídico obrigatório.
  - *Legal hold* (`LegalHold`) — suspensão controlada de descarte; prevalece sobre qualquer política de retenção.
  - *Partição* — `(tenantId, recordType)`; `sequence` e encadeamento por hash valem dentro dela.
  - *Instante do produtor* (`occurredAt`) e *instante de ingestão* (`recordedAt`) — mantidos separados.
  - *Lacuna* — sequência ausente, duplicada ou inválida, ou evento produzido e não registrado.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/12-prd-compliance-ledger.md` (inteiro); `docs/design/12-design-compliance-ledger.md` §2–§8; `docs/design/90-modelo-de-dados.md` §2.3 e §3.5; `docs/design/92-schemas-eventos.md` §4; `docs/prd/14-catalogo-contratos-eventos.md`; `docs/design/93-requisitos-nao-funcionais.md` §4.2 e §6; `docs/design/94-seguranca-threat-model.md` T6; `docs/design/95-personas-jornadas.md` (P8 e matriz de papéis).
- **Situação**: `desbloqueado` — nenhuma lacuna `#1`–`#12` trava o núcleo (registro, consulta, dossiê, legal hold); `#8` só define prazos, e o default é não expurgar.
- **Fase de entrega**: 1.
- **Dependências**: sem upstream duro — o ledger consome de todos os contextos por outbox, de forma assíncrona, e nunca bloqueia o produtor (12 §2). Upstream brando: `identity` (controle de acesso) e `partner` (políticas de retenção). `depends_on` declara só a fundação `SPEC-GXGASP24`; os produtores alimentam o ledger (aresta de retroalimentação) e não entram em `depends_on`.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| `#8`, A13, 12:L8 | Prazos de retenção por categoria e o fundamento que os sustenta | `RetentionPolicy.retentionRule`; expurgo (fora desta versão) |
| A9 | Elementos de evidência necessários para oponibilidade | Composição do `EvidenceBundle` |
| 12:L9 (90:L7, 94:S3) | Notarização externa do hash da cabeça de partição | Proteção contra recálculo da cadeia (fora desta versão) |
| 12:L10 | Tecnologia de armazenamento imutável (decisão D7 da fundação) | `EvidenceReference.storageUri`; camada 3 de imutabilidade |
| 12:L11 | Formato do pacote do dossiê | `EvidenceBundle` (manifesto e pacote) |
| 12:L12 | Fluxo de aprovação do legal hold | `LegalHold` |
| `#1`, A1 | Autorização do BCB (existencial, transversal) | Contexto inteiro |
| `#12` | Volumetria real | Dimensionamento das partições; não bloqueia a construção |

- **Decisões de produto abertas**: notarização externa (90:L7); quem solicita e quem aprova o dossiê (`95` matriz × `17` §5); aplicação automática de legal hold preventivo (12 §7.1).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída, incluindo a decisão D7 da fundação (tecnologia de imutabilidade) e a D4 da fundação (broker). Nenhum bloqueio externo trava o núcleo; a retenção opera com o default "não expurgar" até `#8`.

## Agregados

Nota de modelagem: `tenantId` não é campo dos agregados — o escopo vem do `ExecutionContext` (ADR-051). Diverge do design: `90` §3.5 lista `tenantId` em `AuditRecord`.

### AuditRecord

- **Identidade**: `generated`
- **Campos**:
  - `origin: event | ledger`
  - `sourceEventId: string` — `id` do CloudEvent consumido; ausente quando `origin = ledger`
  - `sequence: int` — monotônico por partição, sem intervalo
  - `recordType: string` — tipo versionado do evento (ex.: `ConsentGranted.v1`)
  - `subjects: list<AuditSubject>` — coleção indispensável: um registro relaciona N entidades (90 §3.5)
  - `actorType: string`, `actorId: id`, `actorRepresentationId: id` (opcional) — ator e representação usada
  - `correlationId: id`, `causationId: id` (opcional)
  - `occurredAt: instant` — relógio do produtor
  - `recordedAt: instant` — relógio do ledger (ingestão)
  - `payloadHash: string` — sha256 do payload original
  - `payloadRef: id` (opcional) — `EvidenceReference` quando o payload é preservado
  - `previousRecordHash: string`, `recordHash: string`
  - `retentionPolicyId: id`
- **Estados**: nenhum — o registro é imutável.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Nenhum comando altera ou remove um `AuditRecord`; correção é novo registro (PRD 12, invariante).
  - `recordHash` é função determinística dos demais campos e de `previousRecordHash`; o algoritmo e a canonicalização não estão definidos em `90` §3.5 **[ABERTA: 90:§3.5 — algoritmo e canonicalização do recordHash]**.
  - `occurredAt` posterior a `recordedAt` é aceito e sinalizado na reconstrução (desvio de relógio do produtor, 12 §5), não rejeitado.
  - O payload entra por hash e referência; nenhum campo é `PII_SENSITIVE` (90 §1.4).
  - `legalHoldIds` (90 §3.5) não é campo do agregado, porque o registro é imutável; a associação é derivada do escopo do `LegalHold` na consulta **[ABERTA: 90:§3.5 lista `legalHoldIds` em registro imutável; confirmar a derivação por escopo]**.
  - A cabeça da partição (última `sequence` e `recordHash`) é resolvida pelo caso de uso, dentro da UoW, pela porta `AuditChainHead`; a unicidade `(tenant_id, record_type, sequence)` serializa concorrentes **[ABERTA: 90:§3.5 define o encadeamento mas não onde vive o estado da cabeça, nem o `previousRecordHash` do primeiro registro da partição]**.

### AuditSubject

- **Identidade**: objeto de valor de `AuditRecord`, sem identidade própria.
- **Campos**:
  - `subjectType: organization | asset | request | contract | partner` — tipos de linha do tempo do PRD 12 (organização, ativo, pedido, contrato, parceiro)
  - `subjectId: id` — identidade na fonte, nunca o objeto de outro contexto
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: referência só por identidade; o ledger não resolve nem armazena o estado do sujeito (12 §9).

### EvidenceReference

- **Identidade**: `natural` por `evidenceId`
- **Campos**:
  - `evidenceId: id`
  - `contentHash: string` — sha256 sobre os bytes originais, antes de qualquer transformação (90 §2.3)
  - `contentType: string`
  - `storageUri: string` — **[ABERTA: provider de object storage]**
  - `accessPolicyId: id`
  - `integrity: unverified | verified | missing | hash-mismatch`
  - `lastVerifiedAt: instant` (opcional)
  - `registeredAt: instant`
- **Estados**: `unverified` (estado inicial), `verified`, `missing`, `hash-mismatch`.
- **Transições permitidas**:
  - (novo) → `unverified` — por `RegisterEvidenceReference`
  - `unverified | verified | missing | hash-mismatch` → `verified | missing | hash-mismatch` — por `RecordEvidenceVerification` (o estado reflete a última verificação; cada falha é incidente, ver Políticas transversais)
- **Invariantes de estado**:
  - `contentHash` é obrigatório — `RegisterEvidenceReference` rejeita com `compliance-ledger/evidence-reference/content-hash-missing`.
  - `accessPolicyId` é obrigatório — rejeita com `compliance-ledger/evidence-reference/access-policy-missing`.
  - Um `evidenceId` já registrado não aceita `contentHash` diferente — rejeita com `compliance-ledger/evidence-reference/hash-conflict`.
  - A referência nunca carrega o conteúdo; "hash sem referência recuperável não constitui dossiê completo" (PRD 12).

### EvidenceBundle

- **Identidade**: `generated`
- **Campos**:
  - `purpose: string` — finalidade declarada
  - `scopeCorrelationId: id`, `scopeSubjectType: string`, `scopeSubjectId: id`, `scopeFrom: instant`, `scopeTo: instant` — seletores de escopo, todos opcionais, ao menos um presente
  - `documentsAuthorized: bool` — documentos referenciados entram "conforme autorização" (12 §6.2)
  - `recipient: string` (opcional) — destinatário, para marca d'água quando o formato permitir (12 §6.3)
  - `requestedBy: id`, `approvedBy: id` (opcional)
  - `status: requested | approved | rejected | ready | expired`
  - `requestedAt: instant`, `decidedAt: instant` (opcional), `generatedAt: instant` (opcional), `validUntil: instant` (opcional)
  - `manifestHash: string`, `packageHash: string`, `signature: string` — preenchidos ao concluir
  - `packageRef: id` (opcional) — `EvidenceReference` do pacote gerado
  - `accessCount: int`, `lastAccessedAt: instant` (opcional)
- **Estados**: `requested`, `approved`, `rejected`, `ready`, `expired`.
- **Transições permitidas**:
  - (novo) → `requested` — por `RequestEvidenceBundle`
  - `requested` → `approved` — por `ApproveEvidenceBundle`
  - `requested` → `rejected` — por `RejectEvidenceBundle` **[ABERTA: 12:§6.1 — o fluxo não define recusa da solicitação]**
  - `approved` → `ready` — por `CompleteEvidenceBundle` (geração assíncrona: coleta, manifesto, assinatura, disponibilização)
  - `ready` → `expired` — por `ExpireEvidenceBundle`; a expiração não apaga o registro da emissão (12 §6.3)
- **Invariantes de estado**:
  - Solicitação sem finalidade rejeita com `compliance-ledger/evidence-bundle/purpose-missing`; sem nenhum seletor de escopo, com `compliance-ledger/evidence-bundle/scope-missing` (minimização, 12 §6.3).
  - Quem aprova é um segundo responsável: `approvedBy ≠ requestedBy` — `compliance-ledger/evidence-bundle/self-approval`.
  - `ready` exige `manifestHash`, `packageHash`, `signature`, `packageRef`, `generatedAt` e `validUntil` — `compliance-ledger/evidence-bundle/package-incomplete`.
  - Dossiê com referência cujo conteúdo não é recuperável não é completo — `compliance-ledger/evidence-bundle/unrecoverable-reference`.
  - Formato do pacote: **[ABERTA: 12:L11 — formato do pacote do dossiê]**; conteúdo mínimo em 12 §6.2 (manifesto, hashes, linha do tempo, registros, versões de política, texto e método, solicitante e finalidade, instante e validade, assinatura).

### RetentionPolicy

- **Identidade**: `generated`
- **Campos**:
  - `category: string` — categoria de 93 §6 (dossiê contratual, evidência de consentimento, dossiê cadastral, registro de efeito, registro de liquidação, trilha de acesso, payload bruto de parceiro, projeção de leitura, snapshot não referenciado)
  - `legalBasis: string` — fundamento jurídico, obrigatório
  - `retentionRule: string` (opcional) — expressão do prazo (ex.: "vigência + prazo prescricional"); ausente significa "não expurgar" **[A CONFIRMAR: #8, A13]**
  - `effectiveFrom: instant`
- **Estados**: nenhum. Mudança de política é nova política com nova `effectiveFrom`; cada registro preserva o `retentionPolicyId` da sua época. Versionamento explícito (`version`, `supersedesId`) não é definido para este agregado **[ABERTA: 12:§7 — versionamento de RetentionPolicy; 90 §1.5 oferece o padrão]**.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Política sem fundamento não é aplicável — `compliance-ledger/retention-policy/legal-basis-missing`.
  - `category` não vazia — `compliance-ledger/retention-policy/category-missing`.
  - O mapeamento `recordType` → categoria não está definido **[ABERTA: 12:L8/93 §6 — mapeamento de recordType para categoria de retenção]**. Enquanto isso, a política padrão `no-purge` (sem `retentionRule`, com a pendência `#8` como fundamento declarado) é a única aplicável.

### LegalHold

- **Identidade**: `generated`
- **Campos**:
  - `scopeKind: subject | period | correlation`
  - `scopeSubjectType: string`, `scopeSubjectId: id`, `scopeFrom: instant`, `scopeTo: instant`, `scopeCorrelationId: id` — conforme o `scopeKind`
  - `basis: string`, `authorizedBy: id`, `placedAt: instant`
  - `status: active | released`
  - `releasedAt: instant`, `releasedBy: id`, `releaseBasis: string` (opcionais até a liberação)
- **Estados**: `active`, `released`.
- **Transições permitidas**:
  - (novo) → `active` — por `PlaceLegalHold`
  - `active` → `released` — por `ReleaseLegalHold`
- **Invariantes de estado**:
  - Exatamente um escopo, coerente com o `scopeKind` — `compliance-ledger/legal-hold/scope-invalid`.
  - Fundamento obrigatório — `compliance-ledger/legal-hold/basis-missing`.
  - Só `active` é liberável — `compliance-ledger/legal-hold/not-active`; a liberação exige autor e fundamento — `compliance-ledger/legal-hold/release-basis-missing`.
  - Hold prevalece sobre qualquer política de descarte (12 §7.1); esta versão não tem comando de descarte.

## Comandos (UPRs)

Nenhum comando emite evento (ver "Eventos de domínio").

### AppendAuditRecord

- **Agregado**: `AuditRecord`; **cria**
- **Entrada**: `origin: event | ledger`; `sourceEventId: string`; `recordType: string`; `subjects: list<AuditSubject>`; `actorType: string`; `actorId: id`; `actorRepresentationId: id`; `correlationId: id`; `causationId: id`; `occurredAt: instant`; `payloadHash: string`; `payloadRef: id`; `sequence: int`; `previousRecordHash: string`; `retentionPolicyId: id`
- **Pré-condições**:
  - `payloadHash` não vazio → `compliance-ledger/audit-record/payload-hash-missing`
  - `subjects` com ao menos um item → `compliance-ledger/audit-record/subjects-missing`
  - `recordType` e `correlationId` não vazios → `compliance-ledger/audit-record/identification-missing`
  - `sequence ≥ 1` → `compliance-ledger/audit-record/sequence-invalid`
- **Efeitos sobre o estado**: demais campos ← entrada; `recordedAt ← at`; `recordHash ← hash(campos canônicos, previousRecordHash)`
- **Eventos emitidos**: nenhum
- **Resposta**: `auditRecordId: id`, `sequence: int`, `recordHash: string`

### RegisterEvidenceReference

- **Agregado**: `EvidenceReference`; **inicializa se ausente**
- **Entrada**: `evidenceId: id`; `contentHash: string`; `contentType: string`; `storageUri: string`; `accessPolicyId: id`
- **Pré-condições**:
  - `contentHash` não vazio → `compliance-ledger/evidence-reference/content-hash-missing`
  - `accessPolicyId` presente → `compliance-ledger/evidence-reference/access-policy-missing`
  - quando presente, `contentHash` igual ao armazenado → `compliance-ledger/evidence-reference/hash-conflict`
- **Efeitos sobre o estado**: quando ausente: demais campos ← entrada; `integrity ← unverified`; `registeredAt ← at`. Quando presente e igual: nenhum — aceita sem alterar.
- **Eventos emitidos**: nenhum
- **Resposta**: `evidenceId: id`

### RecordEvidenceVerification

- **Agregado**: `EvidenceReference`; exige existente
- **Entrada**: `result: verified | missing | hash-mismatch`
- **Pré-condições**: nenhuma além da existência.
- **Efeitos sobre o estado**: `integrity ← cmd.result`; `lastVerifiedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `evidenceId: id`, `integrity`

### RequestEvidenceBundle

- **Agregado**: `EvidenceBundle`; **cria**
- **Entrada**: `purpose: string`; `scopeCorrelationId: id`; `scopeSubjectType: string`; `scopeSubjectId: id`; `scopeFrom: instant`; `scopeTo: instant`; `documentsAuthorized: bool`; `recipient: string`; `requestedBy: id`
- **Pré-condições**:
  - `purpose` não vazio → `compliance-ledger/evidence-bundle/purpose-missing`
  - ao menos um seletor de escopo → `compliance-ledger/evidence-bundle/scope-missing`
- **Efeitos sobre o estado**: `status ← requested`; demais campos ← entrada; `requestedAt ← at`; `accessCount ← 0`
- **Eventos emitidos**: nenhum
- **Resposta**: `bundleId: id`

### ApproveEvidenceBundle

- **Agregado**: `EvidenceBundle`; exige existente
- **Entrada**: `approvedBy: id`
- **Pré-condições**:
  - `status = requested` → `compliance-ledger/evidence-bundle/not-requested`
  - `approvedBy ≠ requestedBy` → `compliance-ledger/evidence-bundle/self-approval`
- **Efeitos sobre o estado**: `status ← approved`; `approvedBy ← cmd.approvedBy`; `decidedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `bundleId: id`

### RejectEvidenceBundle

- **Agregado**: `EvidenceBundle`; exige existente
- **Entrada**: `decidedBy: id`
- **Pré-condições**: `status = requested` → `compliance-ledger/evidence-bundle/not-requested`
- **Efeitos sobre o estado**: `status ← rejected`; `decidedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `bundleId: id`

### CompleteEvidenceBundle

- **Agregado**: `EvidenceBundle`; exige existente
- **Entrada**: `manifestHash: string`; `packageHash: string`; `signature: string`; `packageRef: id`; `validUntil: instant`; `unrecoverableReferences: list<id>` (coleção: as referências sem conteúdo recuperável apuradas na coleta)
- **Pré-condições**:
  - `status = approved` → `compliance-ledger/evidence-bundle/not-approved`
  - `unrecoverableReferences` vazia → `compliance-ledger/evidence-bundle/unrecoverable-reference`
  - `manifestHash`, `packageHash`, `signature`, `packageRef` presentes e `validUntil > at` → `compliance-ledger/evidence-bundle/package-incomplete`
- **Efeitos sobre o estado**: `status ← ready`; campos do pacote ← entrada; `generatedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `bundleId: id`, `validUntil: instant`

### AccessEvidenceBundle

- **Agregado**: `EvidenceBundle`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**: `status = ready` e `at < validUntil` → `compliance-ledger/evidence-bundle/not-available`
- **Efeitos sobre o estado**: `accessCount ← accessCount + 1`; `lastAccessedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `packageRef: id`, `validUntil: instant`
- **Auto-registro**: a emissão e cada acesso geram `AuditRecord` com `origin = ledger` (req. 6 do PRD 12; 12 §6.3). O ledger não publica eventos (14) e um comando decide um agregado; o RPC de acesso chama `AppendAuditRecord` **antes** de entregar a referência, e falha fechado se o registro falhar **[ABERTA: 12:§6.3 — mecanismo de auto-registro de emissão e acesso]**.

### ExpireEvidenceBundle

- **Agregado**: `EvidenceBundle`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**: `status = ready` e `at ≥ validUntil` → `compliance-ledger/evidence-bundle/not-yet-expired`
- **Efeitos sobre o estado**: `status ← expired`
- **Eventos emitidos**: nenhum
- **Resposta**: `bundleId: id`

### DefineRetentionPolicy

- **Agregado**: `RetentionPolicy`; **cria**
- **Entrada**: `category: string`; `legalBasis: string`; `retentionRule: string`; `effectiveFrom: instant`
- **Pré-condições**:
  - `category` não vazia → `compliance-ledger/retention-policy/category-missing`
  - `legalBasis` não vazio → `compliance-ledger/retention-policy/legal-basis-missing`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `retentionPolicyId: id`

### PlaceLegalHold

- **Agregado**: `LegalHold`; **cria**
- **Entrada**: `scopeKind: subject | period | correlation`; campos de escopo; `basis: string`; `authorizedBy: id`
- **Pré-condições**:
  - escopo único e coerente com `scopeKind` (`subject` → tipo e id; `period` → `scopeFrom ≤ scopeTo`; `correlation` → `scopeCorrelationId`) → `compliance-ledger/legal-hold/scope-invalid`
  - `basis` não vazio → `compliance-ledger/legal-hold/basis-missing`
- **Efeitos sobre o estado**: `status ← active`; demais campos ← entrada; `placedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `legalHoldId: id`

### ReleaseLegalHold

- **Agregado**: `LegalHold`; exige existente
- **Entrada**: `releasedBy: id`; `releaseBasis: string`
- **Pré-condições**:
  - `status = active` → `compliance-ledger/legal-hold/not-active`
  - `releaseBasis` não vazio → `compliance-ledger/legal-hold/release-basis-missing`
- **Efeitos sobre o estado**: `status ← released`; `releasedAt ← at`; `releasedBy`, `releaseBasis` ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `legalHoldId: id`

## Eventos de domínio

Este contexto não publica eventos de domínio: o catálogo compartilhado declara que o ledger "consome os de todos os contextos" (`14`, tabela "Eventos por contexto"), e `92` §4 não lista evento do ledger. Por isso não há `.proto` de evento, `Outbox` nem tópico próprio. Consequência a registrar: a falha de integridade ou de reconciliação (12 §4, §8) deveria virar "incidente de conformidade", mas nenhum evento a leva ao `ops` **[ABERTA: 12:§4 — destino do incidente de conformidade; 17 §4 só tem `OUTBOX_STALLED` como caso técnico]**.

## Consultas

Todas correm fora da UoW.

- `FindAuditRecord` — filtros: `auditRecordId`; cardinalidade: `one`
- `FindAuditRecordBySubject` — filtros: `subjectType`, `subjectId`, `recordedFrom` e `recordedTo` (opcionais); cardinalidade: `many`
- `FindAuditTimelineByCorrelation` — filtros: `correlationId`; cardinalidade: `many`. Reconstrução de 12 §5: carrega os registros da correlação, monta o grafo por `causationId`, ordena topologicamente e desempata por `occurredAt` e depois `recordedAt`. Registro sem `causationId` é raiz; ciclo e `occurredAt > recordedAt` são sinalizados, nunca ordenados arbitrariamente. Cada registro devolve se está sob `LegalHold` ativo, derivado do escopo.
- `FindEvidenceReference` — filtros: `evidenceId`; cardinalidade: `one`
- `FindEvidenceBundle` — filtros: `bundleId`; cardinalidade: `one`
- `FindLegalHold` — filtros: `legalHoldId`; cardinalidade: `one`
- `FindRetentionPolicyByCategory` — filtros: `category`, `asOf: instant`; cardinalidade: `one` (a de maior `effectiveFrom` não posterior a `asOf`)

## Relações entre agregados

- `AuditRecord.payloadRef` → `EvidenceReference`
- `AuditRecord.retentionPolicyId` → `RetentionPolicy`
- `AuditRecord.subjects[].subjectId` → entidade de outro contexto (só identidade; sem join)
- `EvidenceBundle.packageRef` → `EvidenceReference`
- `LegalHold.scope*` e `EvidenceBundle.scope*` → seleção de `AuditRecord` por `AuditSubject`, `correlationId` ou período (na consulta, não por referência armazenada)

## Integração

- **Publica**: nenhum evento (ver "Eventos de domínio").
- **Consome**: eventos de todos os contextos, via inbox e Kafka (D4 da fundação). Todos disparam o mesmo comando local, `AppendAuditRecord`, com o mesmo mapa de campos:
  - `id` do CloudEvent → `sourceEventId`; `type` → `recordType`; `time` → `occurredAt`
  - `correlationid` → `correlationId`; `causationid` → `causationId`
  - bytes do payload (`Any.value`) → `payloadHash` (sha256 do original); `payloadRef` só quando o payload é preservado em object storage
  - `subject` do envelope e referências do payload → `subjects` **[ABERTA: 92:§4 — mapeamento por tipo de evento para sujeitos (organização, ativo, pedido, contrato, parceiro)]**
  - `actorType`, `actorId`, `actorRepresentationId`: o envelope CloudEvents do DMPF (ENV-08) não traz `actor` nem `dataClassification` do envelope de `92` §1; o ator chega no payload de cada evento ou o kernel é estendido **[ABERTA: ator no envelope CloudEvents do DMPF]**
  - `tenantid` → `ExecutionContext`, não campo; `recordedAt ← at`; `sequence`, `previousRecordHash` e `retentionPolicyId` são resolvidos pelo caso de uso
  - Payload com `EvidenceReference` (consentimento, instrumento, comprovante): o registro das referências usa `RegisterEvidenceReference`; um comando por evento consumido decide um único agregado **[ABERTA: FND-04 §6.4 — consumo que registra `AuditRecord` e N `EvidenceReference`]**
- **Disposições de consumo**: as sete de FND-04 §6.4; reentrega do mesmo `id` não gera novo registro; `ErrSchemaMismatch` e `ErrMalformed` viram `Failure(Validation)` sem abrir UoW e o evento vai à quarentena, contando em "eventos rejeitados por schema ou integridade" (PRD 12, métricas). Perda de registro é inaceitável: a quarentena é incidente.
- **Eventos consumidos** (59: os 55 que `92` §4 lista com o ledger como consumidor e os 4 que a SPEC-N1X148AW publica além do catálogo, pelo PRD 12, req. 1):
  - **identity**: `company.identity.event.v1.OrganizationRegistered`, `company.identity.event.v1.RepresentationVerified`, `company.identity.event.v1.RepresentationRevoked`, `company.identity.event.v1.UserSuspended`
  - **kyb**: `company.kyb.event.v1.VerificationReady`, `company.kyb.event.v1.DiscrepancyDetected`, `company.kyb.event.v1.PartnerApproved`, `company.kyb.event.v1.PartnerRejected`, `company.kyb.event.v1.ReverificationRequired`
  - **consent**: `company.consent.event.v1.ConsentGranted`, `company.consent.event.v1.ExternalOptInActivated`, `company.consent.event.v1.ConsentRevoked`, `company.consent.event.v1.ConsentExpired`, `company.consent.event.v1.AuthorizationSyncFailed`
  - **gateway**: `company.gateway.event.v1.ExternalCommandSubmitted`, `company.gateway.event.v1.ExternalCommandConfirmed`, `company.gateway.event.v1.ExternalCommandFailed`, `company.gateway.event.v1.ExternalOutcomeUnknown`, `company.gateway.event.v1.ExternalSchemaMismatchDetected`, `company.gateway.event.v1.ExternalCommandAcknowledged`, `company.gateway.event.v1.ExternalInteractionUncorrelated`, `company.gateway.event.v1.ExternalInteractionReprocessed`, `company.gateway.event.v1.ExternalCommandAmbiguityExpired` (os quatro últimos faltam em `92` §4; o contrato é o `.proto` da SPEC-N1X148AW, ADR-022 linha 6)
  - **portfolio**: `company.portfolio.event.v1.PortfolioSnapshotCreated`, `company.portfolio.event.v1.AssetPositionChanged`, `company.portfolio.event.v1.ExternalPositionDiverged`
  - **eligibility**: `company.eligibility.event.v1.EligibleLotCreated`, `company.eligibility.event.v1.AssetReserved`, `company.eligibility.event.v1.EligibilityInvalidated`
  - **marketplace**: `company.marketplace.event.v1.FundingRequestOpened`, `company.marketplace.event.v1.PartnerInvited`, `company.marketplace.event.v1.OfferReceived`, `company.marketplace.event.v1.OffersPresented`, `company.marketplace.event.v1.OfferSelected`, `company.marketplace.event.v1.FundingRequestClosed`
  - **contracting**: `company.contracting.event.v1.ContractValidated`, `company.contracting.event.v1.ContractMismatchDetected`, `company.contracting.event.v1.ContractSigned`, `company.contracting.event.v1.ContractSubmittedForEffect`, `company.contracting.event.v1.ContractCancelled`
  - **effects**: `company.effects.event.v1.RegulatoryEffectPlanned`, `company.effects.event.v1.EffectSubmitted`, `company.effects.event.v1.EffectConfirmed`, `company.effects.event.v1.EffectPartiallyConfirmed`, `company.effects.event.v1.EffectRejected`, `company.effects.event.v1.EffectOutcomeUnknown`, `company.effects.event.v1.EffectReleased`, `company.effects.event.v1.EffectDisputed`
  - **funding**: `company.funding.event.v1.FundingInstructionCreated`, `company.funding.event.v1.DisbursementRequested`, `company.funding.event.v1.DisbursementConfirmed`, `company.funding.event.v1.FundingMismatchDetected`, `company.funding.event.v1.DisbursementReversed`
  - **settlement**: `company.settlement.event.v1.SettlementExpected`, `company.settlement.event.v1.SettlementReported`, `company.settlement.event.v1.AssetSettled`, `company.settlement.event.v1.ReconciliationMatched`, `company.settlement.event.v1.ReconciliationMismatchDetected`, `company.settlement.event.v1.EffectReleaseRequested`
- **Eventos propostos (92 §4, "ausentes do catálogo original")**: `company.risk.event.v1.RiskSignalRaised`, `company.risk.event.v1.OperationBlockedByRisk`, `company.notify.event.v1.NotificationDispatched`, `company.ops.event.v1.OperationalCaseOpened`, `company.ops.event.v1.OperationalCaseResolved`, `company.identity.event.v1.BankAccountChangeRequested`, `company.identity.event.v1.BankAccountChangeApproved`. Os designs de origem mandam registrá-los no ledger (`15` §7, `16` §6.2, `17` §7), mas `92` §4 não declara consumidores para eles **[ABERTA: 92:§4 — consumidores dos eventos propostos]**.
- **Ressalvas de consistência**:
  - `92` §4 não lista o ledger como consumidor de `AssetBecameStale`, `ReservationExpired`, `CapabilityEnabled`, `CapabilitySuspended`, `PartnerProductChanged` e `PartnerCertificateExpiring`, embora o PRD 12 (req. 1) peça "eventos de todos os bounded contexts" **[ABERTA: 92:§4 × prd/12:req.1 — incluir ou não esses seis eventos]**.
  - `ObligationFulfilled` aparece na saga de `14` e não existe em `92` §4.
  - O emissor de `RepresentationVerified` é disputado entre Identity e KYB; o FQN acima segue `92` §4 (Identity) **[ABERTA: emissor de RepresentationVerified]**.

## Borda e persistência

- **Borda**: só gRPC, `company.ledger.service.v1.LedgerService`, um RPC por comando e por consulta (12 comandos, 7 consultas). `AppendAuditRecord` e `RegisterEvidenceReference` só aceitam a identidade de serviço do consumidor do próprio ledger; nenhum papel humano escreve a trilha (94 T6). REST do `bff` fica no Escopo fora.
- **Banco**: database e role `ledger`, schema `public`, tabelas `audit_records`, `evidence_references`, `evidence_bundles`, `retention_policies`, `legal_holds` e `inbox` (kernel; o contexto consome e não publica, então pede `Inbox` e não `Outbox`), com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas e a consulta que filtra por cada uma:
  - `audit_records`: `record_type` + `sequence` (unicidade da partição e cadeia); `correlation_id` (`FindAuditTimelineByCorrelation`); `source_event_id` (unicidade, detecção de duplicidade); `recorded_at` (`FindAuditRecordBySubject` por período). Consulta por sujeito sobre lista **[ABERTA: persistência do filtro por `subjects` (lista) sem tabela auxiliar]**.
  - `evidence_references`: `integrity`, `last_verified_at` (amostragem de verificação).
  - `evidence_bundles`: `status`, `valid_until` (expiração).
  - `legal_holds`: `status`. `retention_policies`: `category`, `effective_from`.
- **Imutabilidade**: a tecnologia é a decisão D7 da fundação (`SPEC-GXGASP24`), referenciada aqui e não decidida. No DMPF a role da app é dona do banco (tem `UPDATE` e `DELETE`); a camada "permissão de banco sem UPDATE/DELETE" (12 §3, C6.2) exige uma role separada, só de `INSERT`, para `audit_records`. Diverge do design: C6.2 supõe que a role da aplicação não tenha esses privilégios.
- **Divergências do design**: sem RLS como mecanismo primário (`tenant_id` + `ExecutionContext`, ADR-051); o envelope JSON/`schemaUri` de `92` §1 é substituído por Protobuf/CloudEvents; `instant` é inteiro de nanossegundos (o doc 00 §8 fala em milissegundos); a reconciliação "outbox dos módulos × ledger" (12 §4) exigiria ler o banco de outro contexto, o que o DMPF proíbe **[ABERTA: mecanismo de reconciliação produtor × ledger sem acesso cruzado a banco]**.
- **Acesso cross-tenant** (auditor do financiador, dossiê sobre operação de outro tenant): **[ABERTA: a capacidade existe no kernel desde a `1.0.0-rc.6` (DEVS-100, ADR-068 do DMPF); o uso aqui depende da adoção da rc.6 (DEVS-136)]**.

## Políticas transversais

- **Idempotência**: consumo idempotente por `id` do CloudEvent (inbox, ADR-036; FND-04 §6.4) e unicidade `(tenant_id, source_event_id)`; os comandos de escrita correm em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Papéis: `AUDITOR` solicita e `PLATFORM_OPS` aprova dossiê (`95`, matriz) — diverge de `17` §5 (supervisor com segundo aprovador) **[ABERTA: 95 matriz × 17 §5 — quem solicita e quem aprova o dossiê]**. Legal hold é ato do jurídico, fora da rotina (12 §7.1) **[ABERTA: 12:L12 — fluxo de aprovação]**.
- **Auditoria**: o ledger é a trilha; a emissão e o acesso aos seus dossiês se registram nele mesmo (ver `AccessEvidenceBundle`). O restante pela trilha de auditoria do kernel (FND-08; `observability/audit`).
- **Disponibilidade**: a escrita é assíncrona por outbox dos produtores; a indisponibilidade do ledger atrasa o registro e não impede a operação (93 §4.2). RPO zero; a profundidade e a idade da outbox dos produtores são métrica de conformidade.
- **Integridade e verificação** (12 §4, §8): duplicidade e sequência são contínuas na ingestão; cadeia por partição (diária, incremental), cadeia completa, existência e hash das evidências (por amostragem, via `RecordEvidenceVerification`) e restauração testada com verificação de cadeia são rotinas de leitura. Falha de cadeia ou de hash é incidente crítico. O limite do encadeamento é declarado: sem notarização externa, quem controla a infraestrutura pode recalcular a cadeia.
- **Classificação**: dado pessoal minimizado (90 §1.4); o ledger referencia conteúdo protegido e nunca o replica.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `ledger` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO um evento consumido com payload, correlação e sujeito válidos
QUANDO AppendAuditRecord chega com sequence 1
ENTÃO o registro nasce com recordedAt = at, recordHash calculado e nada é emitido

DADO o mesmo evento entregue duas vezes
QUANDO a segunda entrega chega à inbox
ENTÃO nenhum novo AuditRecord é criado e a recepção é concluída como repetida

DADO qualquer entrada
QUANDO AppendAuditRecord chega com payloadHash vazio
ENTÃO rejeita com compliance-ledger/audit-record/payload-hash-missing

DADO qualquer entrada
QUANDO AppendAuditRecord chega sem sujeitos
ENTÃO rejeita com compliance-ledger/audit-record/subjects-missing

DADO qualquer entrada
QUANDO AppendAuditRecord chega com recordType ou correlationId vazio
ENTÃO rejeita com compliance-ledger/audit-record/identification-missing

DADO qualquer entrada
QUANDO AppendAuditRecord chega com sequence 0
ENTÃO rejeita com compliance-ledger/audit-record/sequence-invalid

DADO uma evidência ainda não registrada
QUANDO RegisterEvidenceReference chega com contentHash e accessPolicyId
ENTÃO a referência nasce unverified com registeredAt = at

DADO uma evidência já registrada com o mesmo contentHash
QUANDO RegisterEvidenceReference chega
ENTÃO aceita sem alterar a referência

DADO qualquer entrada
QUANDO RegisterEvidenceReference chega sem contentHash
ENTÃO rejeita com compliance-ledger/evidence-reference/content-hash-missing

DADO qualquer entrada
QUANDO RegisterEvidenceReference chega sem accessPolicyId
ENTÃO rejeita com compliance-ledger/evidence-reference/access-policy-missing

DADO uma evidência registrada com um contentHash
QUANDO RegisterEvidenceReference chega com o mesmo evidenceId e outro contentHash
ENTÃO rejeita com compliance-ledger/evidence-reference/hash-conflict e o estado não muda

DADO uma evidência registrada
QUANDO RecordEvidenceVerification chega com result hash-mismatch
ENTÃO integrity vai a hash-mismatch e lastVerifiedAt = at

DADO uma solicitação com finalidade e escopo por correlação
QUANDO RequestEvidenceBundle chega
ENTÃO o dossiê nasce requested com requestedAt = at

DADO qualquer entrada
QUANDO RequestEvidenceBundle chega sem finalidade
ENTÃO rejeita com compliance-ledger/evidence-bundle/purpose-missing

DADO qualquer entrada
QUANDO RequestEvidenceBundle chega sem nenhum seletor de escopo
ENTÃO rejeita com compliance-ledger/evidence-bundle/scope-missing

DADO um dossiê requested
QUANDO ApproveEvidenceBundle chega com um aprovador diferente do solicitante
ENTÃO o dossiê vai a approved

DADO um dossiê requested
QUANDO ApproveEvidenceBundle chega com o próprio solicitante
ENTÃO rejeita com compliance-ledger/evidence-bundle/self-approval

DADO um dossiê já approved
QUANDO ApproveEvidenceBundle ou RejectEvidenceBundle chega
ENTÃO rejeita com compliance-ledger/evidence-bundle/not-requested

DADO um dossiê requested
QUANDO RejectEvidenceBundle chega
ENTÃO o dossiê vai a rejected

DADO um dossiê approved
QUANDO CompleteEvidenceBundle chega com manifesto, hashes, assinatura, packageRef e validade futura
ENTÃO o dossiê vai a ready com generatedAt = at

DADO um dossiê requested
QUANDO CompleteEvidenceBundle chega
ENTÃO rejeita com compliance-ledger/evidence-bundle/not-approved

DADO um dossiê approved
QUANDO CompleteEvidenceBundle chega com referências irrecuperáveis
ENTÃO rejeita com compliance-ledger/evidence-bundle/unrecoverable-reference e o dossiê permanece approved

DADO um dossiê approved
QUANDO CompleteEvidenceBundle chega sem assinatura ou com validade passada
ENTÃO rejeita com compliance-ledger/evidence-bundle/package-incomplete

DADO um dossiê ready dentro da validade
QUANDO AccessEvidenceBundle chega
ENTÃO accessCount incrementa, lastAccessedAt = at e a referência do pacote é devolvida

DADO um dossiê que não está ready ou cuja validade passou
QUANDO AccessEvidenceBundle chega
ENTÃO rejeita com compliance-ledger/evidence-bundle/not-available

DADO um dossiê ready cuja validade passou
QUANDO ExpireEvidenceBundle chega
ENTÃO o dossiê vai a expired

DADO um dossiê ready ainda dentro da validade
QUANDO ExpireEvidenceBundle chega
ENTÃO rejeita com compliance-ledger/evidence-bundle/not-yet-expired

DADO uma categoria e um fundamento
QUANDO DefineRetentionPolicy chega
ENTÃO a política nasce com effectiveFrom informado

DADO qualquer entrada
QUANDO DefineRetentionPolicy chega sem fundamento
ENTÃO rejeita com compliance-ledger/retention-policy/legal-basis-missing

DADO qualquer entrada
QUANDO DefineRetentionPolicy chega sem categoria
ENTÃO rejeita com compliance-ledger/retention-policy/category-missing

DADO um escopo por sujeito e um fundamento
QUANDO PlaceLegalHold chega
ENTÃO o hold nasce active com placedAt = at

DADO qualquer entrada
QUANDO PlaceLegalHold chega com dois escopos ou com período invertido
ENTÃO rejeita com compliance-ledger/legal-hold/scope-invalid

DADO qualquer entrada
QUANDO PlaceLegalHold chega sem fundamento
ENTÃO rejeita com compliance-ledger/legal-hold/basis-missing

DADO um hold active
QUANDO ReleaseLegalHold chega com autor e fundamento
ENTÃO o hold vai a released com releasedAt = at

DADO um hold já released
QUANDO ReleaseLegalHold chega
ENTÃO rejeita com compliance-ledger/legal-hold/not-active

DADO um hold active
QUANDO ReleaseLegalHold chega sem fundamento
ENTÃO rejeita com compliance-ledger/legal-hold/release-basis-missing

DADO registros da mesma correlação ligados por causationId e um com occurredAt posterior a recordedAt
QUANDO FindAuditTimelineByCorrelation consulta a correlação
ENTÃO devolve a ordem causal desempatada por tempo, com o desvio de relógio sinalizado, fora de qualquer UoW
```

## Escopo fora

- **REST do `bff`**: `GET /audit/timelines/{correlationId}`, `POST /audit/evidence-bundles`, `GET /audit/evidence-bundles/{id}` e `POST /audit/legal-holds` (PRD 12, "APIs principais") ficam no `bff`; `docs/design/91-contratos-api.md` não detalha rotas de auditoria.
- **Expurgo por retenção**: sem prazos (`#8`, A13), o default é não expurgar; não há comando de descarte. Só existe após resolver o bloqueio.
- **Notarização externa** do hash da cabeça de partição (12:L9): decisão de produto e jurídica (90:L7).
- **Hold preventivo automático** em disputa ou caso crítico (12 §7.1, "deveria aplicar"): gatilho e autorização indefinidos **[ABERTA: 12:§7.1 — gatilho e consumidor do hold preventivo]**.
- **Tecnologia de armazenamento imutável** e provider de object storage: decisão D7 da fundação e **[ABERTA: provider de object storage]**.
- **Observabilidade, banco de leitura, relatórios e estado atual**: o ledger responde "o que aconteceu", não "qual o estado atual" (12 §9).
- **Reconciliação produtor × ledger e verificação de "causationId sem comando"**: dependem do mecanismo em aberto acima.
- **Restauração testada e infraestrutura de backup**: exercício de operação, fora do domínio do contexto.
