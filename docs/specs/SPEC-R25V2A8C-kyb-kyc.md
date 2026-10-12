---
id: SPEC-R25V2A8C
slug: kyb-kyc
title: Bounded context — kyb-kyc
stage: backlog
priority: P1
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6]
ticket_url: https://linear.app/mmda/issue/DEVS-79/spec-r25v2a8c-bounded-context-kyb-kyc
subtask_urls: []
created: 2026-10-12
---

# SPEC-R25V2A8C: Bounded context — kyb-kyc

## Identidade

- **bounded_context**: `kyb-kyc`
- **name**: `kyb`
- **Linguagem ubíqua**:
  - *Caso de verificação* (`VerificationCase`) — processo versionado de verificação de uma organização, para um parceiro e uma finalidade.
  - *Dossiê* — evidências, verificações e divergências de um caso; mede completude técnica, nunca aprovação.
  - *Evidência* (`Evidence`) — documento ou retorno de fonte, versionado com fonte, instante e hash.
  - *Verificação* (`Check`) — validação individual (`CHK-*`) com resultado e validade.
  - *Divergência* — conflito entre fontes ou falha de verificação, classificada `blocking`, `material` ou `informational`.
  - *Completude* — razão entre verificações obrigatórias válidas e verificações obrigatórias do produto, por par organização-parceiro.
  - *Prontidão* (`ready_for_partner`) — completude 1 sem divergência bloqueante; não é conformidade aprovada.
  - *Decisão do parceiro* (`PartnerDecision`) — decisão do financiador sobre o caso; nunca decisão global.
  - *Nível de divulgação* — `screening`, `analysis` ou `contracting`: o subconjunto do dossiê que o parceiro pode ver.
  - *Sinal de risco* (`RiskSignal`) — sinal informativo, nunca decisão automática; dono disputado (ver Agregados).

## Origem e bloqueios externos

- **Fontes**: `docs/prd/02-prd-kyb-kyc.md`; `docs/design/02-design-kyb-kyc.md` §2 (limite de responsabilidade), §3 (catálogo de verificações), §4 (completude), §5 (divergências), §6 (revalidação), §7 (níveis de compartilhamento), §8 (decisões divergentes); `docs/design/90-modelo-de-dados.md` §1.4 e §2.2; `docs/design/92-schemas-eventos.md` §4; `docs/prd/14-catalogo-contratos-eventos.md`; `docs/reference/glossario.md`.
- **Situação**: `parcial` — o núcleo (máquina de estados, completude, divergências, decisão do parceiro) é construível; fontes de dados, enquadramento de PLD/FT e verificação de titularidade de conta dependem de apuração externa.
- **Fase de entrega**: 1 (inferida — nenhum documento declara a fase do contexto).
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-7KW0HHF6`, Identity: `OrganizationRegistered`); a cadeia transitiva fica implícita. Brandas, fora de `depends_on`: Partner Management (`PartnerAgreement` define o conjunto obrigatório de verificações, 02 §4) e Consent & Mandates (níveis `analysis` e `contracting`, 02 §7).
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| #1 / A1 | autorização regulatória da plataforma (transversal) | existencial para o contexto inteiro |
| A2, 02:Y8 | se a plataforma é obrigada de PLD/FT, com comunicação de operação suspeita | limite de responsabilidade (02 §2); ponto de extensão de `RiskSignal` |
| C3, 02:Y11 | cobertura e custo dos provedores de dados cadastrais | fontes de `CHK-REG`, `CHK-OWN`, `CHK-LST`; custo por consulta |
| C1, 02:Y13, #11 | serviço de validação de titularidade de conta | `CHK-BNK-001` (controle antifraude de maior perda esperada) |
| 02:Y9 | critério de identificação de beneficiário final | `CHK-OWN-002` |
| 02:Y10 | listas restritivas obrigatórias | `CHK-LST-001` a `CHK-LST-004` |
| 02:Y12 | prazos de validade por verificação | `validUntil` de cada `Check`; gatilho `check_expiry` |

- **Decisões de produto abertas**: dono de `RiskSignal` (em Agregados); se divergência `material` aberta impede `ready_for_partner` (`docs/design/02` §5); política de tolerância para resultado expirado (`docs/prd/02`, Invariantes); valores de `purpose`; gatilhos de OPEN → COLLECTING e COLLECTING → VALIDATING (`docs/prd/02`, Estados).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída (inclui a decisão D4 do broker); `SPEC-7KW0HHF6` concluída (evento `OrganizationRegistered`); decisão das fontes mínimas (C3) para a fatia de verificações automáticas. O núcleo não espera A2, 02:Y9, 02:Y10 nem C1: um `CHK-BNK-001` obrigatório sem fonte mantém a completude abaixo de 1, de forma visível.

## Agregados

`Evidence`, `Check` e `PartnerDecision` do PRD são entidades internas de `VerificationCase`: a completude e a divergência bloqueante são invariantes que exigem leitura atômica do conjunto (`docs/design/02` §4). `BusinessProfile` e `OwnershipStructure` são visões derivadas das evidências do caso por regra de precedência (`docs/design/02` §5), servidas por consulta — não são agregados. A `PrecedenceRule` (`field`, `orderedSources`, `conflictBehavior: use_highest_precedence | flag_discrepancy`) é dado, não código **[ABERTA: docs/design/02-design-kyb-kyc.md §5 — onde vive a PrecedenceRule]**.

**RiskSignal** não é modelado aqui. **[ABERTA: dono de `RiskSignal` — docs/prd/02-prd-kyb-kyc.md (Modelo de domínio) o lista em KYB/KYC; docs/design/15-design-fraude-risco.md §4 o define como agregado do contexto Fraude e Risco; docs/design/92-schemas-eventos.md §4 só prevê o evento `RiskSignalRaised` sob Risco]**. O nível `screening` (02 §7), que expõe sinais de risco ao parceiro, depende dessa decisão.

**Comprovação de poderes.** **[ABERTA: emissor de RepresentationVerified e dono da prova de poderes - docs/prd/01-prd-identity-organization.md req. 4 e docs/design/92-schemas-eventos.md §4 atribuem o evento ao contexto Identity; docs/design/02-design-kyb-kyc.md §3.3 (CHK-REP-002, CHK-REP-003) atribui a comprovação documental dos poderes ao KYB/KYC; nenhum documento decide quem emite o evento nem quem aprova a transição declared para verified]**. Enquanto aberta, `CHK-REP-001` a `CHK-REP-003` são verificações comuns do caso e esta spec não emite `RepresentationVerified`.

**Catálogo de verificações** (`docs/design/02` §3): `CHK-REG-001` a `CHK-REG-005` (cadastro), `CHK-OWN-001` a `CHK-OWN-004` (estrutura societária; o critério de beneficiário final em `CHK-OWN-002` **[A CONFIRMAR: 02:Y9]**), `CHK-REP-001` a `CHK-REP-003` (representantes), `CHK-LST-001` a `CHK-LST-004` (listas restritivas; quais são obrigatórias **[A CONFIRMAR: 02:Y10]**) e `CHK-BNK-001`, `CHK-BNK-002` (conta; a titularidade em `CHK-BNK-001` **[A CONFIRMAR: C1, 02:Y13, #11]**). Se a plataforma for obrigada de PLD/FT, o limite de responsabilidade muda **[A CONFIRMAR: A2, 02:Y8]**; até lá o modelo assume que ela é auxiliar e mantém o `RiskSignal` como ponto de extensão.

### VerificationSubject

- **Identidade**: `natural` por `organizationId`
- **Campos**:
  - `organizationId: id`
  - `taxIdType: cnpj | cpf`
  - `taxIdHash: string`
  - `taxIdMasked: string`
  - `jurisdiction: string`
  - `kinds: list<string>` — espelha `Organization.kinds` de Identity (coleção indispensável: uma organização acumula papéis)
  - `registeredAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**: projeção local de `OrganizationRegistered`; não é alterada depois de inicializada. **[ABERTA: docs/prd/02-prd-kyb-kyc.md req. 1 e docs/design/92-schemas-eventos.md §4 declaram KYB consumidor de `OrganizationRegistered` sem dizer o efeito; este agregado é o mínimo coerente]**

### VerificationCase

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `partnerId: id` — a completude é por par organização-parceiro (`docs/design/02` §4)
  - `purpose: string` **[ABERTA: docs/prd/02-prd-kyb-kyc.md req. 1 — valores de finalidade não definidos]**
  - `version: int` e `supersedesId: id` (opcional)
  - `status: open | collecting | validating | pending_information | ready_for_partner | partner_review | approved | rejected | expired`
  - `requiredCheckIds: list<string>` — verificações `CHK-*` exigidas pelo produto e parceiro (`PartnerAgreement`); coleção indispensável para a completude
  - `checks: list<Check>` — `checkId: string`, `result: passed | failed | inconclusive`, `executedAt: instant`, `validUntil: instant`, `source: string`, `evidenceIds: list<id>`
  - `evidences: list<Evidence>` — `evidenceId: id`, `field: string`, `source: string`, `observedValue: string`, `contentHash: string`, `evidenceRef: id`, `contentType: string`, `collectedAt: instant`, `reusedFromCaseId: id` (opcional), `supersedesEvidenceId: id` (opcional). O conteúdo do documento nunca entra no agregado, só a `EvidenceReference` **[ABERTA: provider de object storage]**
  - `discrepancies: list<Discrepancy>` — `discrepancyId: id`, `field: string`, `class: blocking | material | informational`, `sources: list<string>`, `status: open | resolved | justified`, `detectedAt: instant`, `justification: string` (opcional)
  - `pendingRequests: list<InformationRequest>` — `requestId: id`, `description: string`, `requestedAt: instant`, `fulfilledAt: instant` (opcional)
  - `completeness: decimal` — de 0 a 1
  - `disclosureLevel: none | screening | analysis | contracting` e `disclosures: list<Disclosure>` (`level`, `at`) — histórico de escalonamento visível ao cedente (02 §7)
  - `partnerDecision` (opcional): `decision: approved | rejected`, `reasonCategory: cadastral | appetite` (opcional), `reasonDisclosureAuthorized: bool`, `decidedAt: instant`
  - `openedAt: instant`, `readyAt: instant` (opcional)
  - As cinco listas são coleções indispensáveis: sem elas a completude e a divergência bloqueante não são decididas atomicamente.
- **Estados**: os nove de `status`; `approved`, `rejected` e `expired` são finais.
- **Transições permitidas**:
  - (novo) → `open` — por `OpenVerificationCase`
  - `open` → `collecting` — por `AttachEvidence` **[ABERTA: docs/prd/02-prd-kyb-kyc.md Estados — o gatilho não é definido]**
  - `collecting` → `validating` — por `RunCheck` **[ABERTA: docs/prd/02-prd-kyb-kyc.md Estados — o gatilho não é definido]**
  - `collecting` | `validating` → `pending_information` — por `RequestInformation`
  - `pending_information` → `validating` — por `AttachEvidence` que cumpre a última pendência
  - `validating` → `ready_for_partner` — por `RunCheck`, `AttachEvidence` ou `ResolveDiscrepancy`, quando `completeness = 1` e nenhuma divergência `blocking` aberta
  - `ready_for_partner` → `partner_review` — por `SubmitToPartner`
  - `partner_review` → `approved` | `rejected` — por `RecordPartnerDecision`
  - qualquer estado, exceto `rejected` e `expired` → `expired` — por `ExpireVerification`
- **Invariantes de estado**:
  - `ready_for_partner` significa completude técnica, não conformidade aprovada; só é alcançado com `completeness = 1` e sem divergência `blocking` aberta.
  - `completeness` é recalculada a cada fato do caso: verificações obrigatórias com resultado `passed` e `validUntil` futuro, sobre o total de obrigatórias; divergência `blocking` aberta zera a completude (não pondera); verificação opcional não entra no denominador.
  - Nenhuma evidência é sobrescrita: nova versão aponta `supersedesEvidenceId`; cada uma mantém fonte, instante e hash.
  - `disclosureLevel` só cresce — `SubmitToPartner` com nível não maior rejeita com `kyb-kyc/verification-case/level-not-escalating`.
  - Fora das fases de coleta, comandos que acrescentam fatos rejeitam com `kyb-kyc/verification-case/collection-closed`.
  - Resultado expirado não conta para a completude **[ABERTA: docs/prd/02-prd-kyb-kyc.md Invariantes — a política de tolerância não é definida]**.

## Comandos (UPRs)

(†) marca pré-condição que depende de outro agregado ou contexto: o caso de uso a verifica por leitura ou consulta gRPC antes de decidir. (††) marca unicidade garantida por índice único e traduzida em rejeição. Ver Relações entre agregados.

### RegisterVerificationSubject

- **Agregado**: `VerificationSubject`; **inicializa se ausente**
- **Comando**: `RegisterVerificationSubject`
- **Entrada**:
  - `organizationId: id`
  - `taxIdType: cnpj | cpf`
  - `taxIdHash: string`
  - `taxIdMasked: string`
  - `jurisdiction: string`
  - `kinds: list<string>`
- **Pré-condições**: nenhuma
- **Efeitos sobre o estado**:
  - quando ausente: campos ← cmd; `registeredAt ← at`
  - quando presente: nenhum — aceita sem alterar e sem emitir
- **Eventos emitidos**: nenhum
- **Resposta**: `organizationId: id`

### OpenVerificationCase

- **Agregado**: `VerificationCase`; **cria**
- **Comando**: `OpenVerificationCase`
- **Entrada**:
  - `organizationId: id`
  - `partnerId: id`
  - `purpose: string`
  - `requiredCheckIds: list<string>`
  - `supersedesId: id` (opcional — revalidação)
- **Pré-condições**:
  - `requiredCheckIds` não vazio → `kyb-kyc/verification-case/required-checks-empty`
  - `purpose` não vazio → `kyb-kyc/verification-case/purpose-empty`
  - organização conhecida, com `VerificationSubject` (†) → `kyb-kyc/verification-case/organization-unknown`
  - nenhum outro caso não final para a mesma organização, parceiro e finalidade (†, ††) → `kyb-kyc/verification-case/case-already-open`
- **Efeitos sobre o estado**:
  - `status ← open`, `completeness ← 0`, `disclosureLevel ← none`
  - `version ← 1`, ou a do caso substituído mais 1 quando há `supersedesId`
  - `openedAt ← at`
- **Eventos emitidos**: `VerificationOpened`
- **Resposta**: `caseId: id`

### AttachEvidence

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `AttachEvidence`
- **Entrada**:
  - `evidenceId: id` (gerado pelo caso de uso)
  - `field: string`, `source: string`, `observedValue: string`
  - `contentHash: string`, `evidenceRef: id`, `contentType: string`
  - `collectedAt: instant`
  - `reusedFromCaseId: id` (opcional — reutilização de evidência ainda válida)
  - `fulfilsRequestId: id` (opcional)
  - `conflictBehavior: use_highest_precedence | flag_discrepancy` e `discrepancyId: id`, `discrepancyClass: blocking | material | informational` — resolvidos pelo caso de uso a partir da `PrecedenceRule`
- **Pré-condições**:
  - `status` em `open`, `collecting`, `validating` ou `pending_information` → `kyb-kyc/verification-case/collection-closed`
  - `contentHash` não vazio → `kyb-kyc/verification-case/content-hash-missing`
  - `source` não vazia → `kyb-kyc/verification-case/source-missing`
  - `fulfilsRequestId`, quando informado, refere uma pendência ainda não cumprida → `kyb-kyc/verification-case/request-not-pending`
- **Efeitos sobre o estado**:
  - `evidences ←` acrescida do item; `evidenceId` já presente com o mesmo `contentHash`: aceita sem alterar e sem emitir
  - `fulfilsRequestId`: `fulfilledAt ← at`
  - `open` → `collecting`; `pending_information` → `validating` quando nenhuma pendência resta
  - existindo evidência de outra `source` para o mesmo `field` com `observedValue` diferente e `conflictBehavior = flag_discrepancy`: `discrepancies ←` acrescida (`status ← open`, `detectedAt ← at`); nenhuma fonte sobrescreve outra
  - `completeness ←` recalculada
- **Eventos emitidos**: `EvidenceCollected`; `DiscrepancyDetected` (só quando abre divergência)
- **Resposta**: `evidenceId: id`

### RunCheck

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `RunCheck`
- **Entrada**:
  - `checkId: string` (`CHK-REG-*`, `CHK-OWN-*`, `CHK-REP-*`, `CHK-LST-*`, `CHK-BNK-*`)
  - `result: passed | failed | inconclusive`
  - `validUntil: instant` (da validade do catálogo, 02 §3) **[ABERTA: 02:Y12]**
  - `source: string`, `evidenceIds: list<id>`
  - `failureClass: blocking | material | informational` e `discrepancyId: id` (obrigatórios quando `failed`)
- **Pré-condições**:
  - fase de coleta → `kyb-kyc/verification-case/collection-closed`
  - `validUntil` posterior a `at` → `kyb-kyc/verification-case/validity-invalid`
  - `evidenceIds` todos presentes em `evidences` → `kyb-kyc/verification-case/evidence-unknown`
  - `failed` exige `failureClass` → `kyb-kyc/verification-case/failure-class-missing`
- **Efeitos sobre o estado**:
  - `checks ←` item de `checkId` substituído ou acrescido
  - `failed`: `discrepancies ←` acrescida (`field ← checkId`, `class ← failureClass`, `status ← open`)
  - `collecting` → `validating`
  - `validating` → `ready_for_partner` e `readyAt ← at` quando `completeness = 1` e sem `blocking` aberta
  - `completeness ←` recalculada
- **Eventos emitidos**: `DiscrepancyDetected` (quando `failed`); `VerificationReady` (quando transita)
- **Resposta**: `caseId: id`, `completeness: decimal`

### RequestInformation

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `RequestInformation`
- **Entrada**:
  - `requestId: id`
  - `description: string`
- **Pré-condições**:
  - fase de coleta → `kyb-kyc/verification-case/collection-closed`
  - `description` não vazia → `kyb-kyc/verification-case/description-empty`
- **Efeitos sobre o estado**:
  - `pendingRequests ←` acrescida (`requestedAt ← at`)
  - `status ← pending_information`
- **Eventos emitidos**: nenhum
- **Resposta**: `requestId: id`

### ResolveDiscrepancy

**[ABERTA: docs/prd/02-prd-kyb-kyc.md Comandos — o PRD não lista comando para resolver ou justificar divergência; docs/design/02 §5 exige "resolução ou justificativa" para `material`]**

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `ResolveDiscrepancy`
- **Entrada**:
  - `discrepancyId: id`
  - `resolution: resolved | justified`
  - `justification: string`
- **Pré-condições**:
  - fase de coleta → `kyb-kyc/verification-case/collection-closed`
  - divergência existe e está `open` → `kyb-kyc/verification-case/discrepancy-not-open`
  - `justified` não se aplica a `blocking` → `kyb-kyc/verification-case/blocking-cannot-be-justified`
  - `justified` exige `justification` não vazia → `kyb-kyc/verification-case/justification-empty`
- **Efeitos sobre o estado**:
  - `status` da divergência `← resolution`; `justification ← cmd.justification`
  - `completeness ←` recalculada; `validating` → `ready_for_partner` quando as condições se cumprem
- **Eventos emitidos**: `VerificationReady` (quando transita)
- **Resposta**: `caseId: id`

### SubmitToPartner

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `SubmitToPartner`
- **Entrada**:
  - `disclosureLevel: screening | analysis | contracting`
  - `consentGrantId: id`
- **Pré-condições**:
  - `consentGrantId` informado → `kyb-kyc/verification-case/consent-missing`; a vigência do consentimento que inclui o parceiro, a solicitação registrada do parceiro (`analysis`) e a oferta selecionada (`contracting`) são verificadas pelo caso de uso por consulta gRPC (†) **[ABERTA: docs/design/92-schemas-eventos.md §4 não declara KYB consumidor de eventos de Consent nem de Marketplace]**
  - `status` em `ready_for_partner`, `partner_review` ou `approved` → `kyb-kyc/verification-case/not-ready-for-partner`
  - `disclosureLevel` maior que o atual → `kyb-kyc/verification-case/level-not-escalating`
- **Efeitos sobre o estado**:
  - `disclosureLevel ← cmd.disclosureLevel`; `disclosures ←` acrescida (`at`)
  - `ready_for_partner` → `partner_review`
- **Eventos emitidos**: nenhum **[ABERTA: docs/design/02-design-kyb-kyc.md §7 manda o escalonamento gerar "evento auditado", mas o catálogo (docs/design/92 §4) não traz evento; cobre a trilha de auditoria do kernel]**
- **Resposta**: `caseId: id`, `disclosureLevel`

### RecordPartnerDecision

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `RecordPartnerDecision`
- **Entrada**:
  - `decision: approved | rejected`
  - `reasonCategory: cadastral | appetite` (obrigatório quando `rejected`)
  - `reasonDisclosureAuthorized: bool`
- **Pré-condições**:
  - `status = partner_review` → `kyb-kyc/verification-case/not-in-partner-review`
  - `rejected` exige `reasonCategory` → `kyb-kyc/verification-case/reason-category-missing`
- **Efeitos sobre o estado**:
  - `partnerDecision ←` {decision, reasonCategory, reasonDisclosureAuthorized, `decidedAt ← at`}
  - `status ← approved` ou `rejected`
- **Eventos emitidos**: `PartnerApproved` ou `PartnerRejected`
- **Resposta**: `caseId: id`, `status`

### ExpireVerification

- **Agregado**: `VerificationCase`; exige existente
- **Comando**: `ExpireVerification`
- **Entrada**:
  - `trigger: check_expiry | ownership_change | representative_change | restrictive_list_hit | bank_account_change | partner_requirement | periodic_review` — os gatilhos de `docs/design/02` §6
  - `affectedBlocks: list<registry | ownership | representatives | restrictive_lists | bank_account | full>`
- **Pré-condições**:
  - `status` fora de `rejected` e `expired` → `kyb-kyc/verification-case/already-closed`
  - `trigger = check_expiry` exige ao menos uma verificação com `validUntil` menor ou igual a `at` → `kyb-kyc/verification-case/nothing-expired`
- **Efeitos sobre o estado**:
  - `status ← expired`
- **Eventos emitidos**: `ReverificationRequired`
- **Resposta**: `caseId: id`

## Eventos de domínio

`EventName()` é `kyb-kyc.verification-case.<evento>`.

### VerificationOpened

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `organizationId: id ← VerificationCase.organizationId`
  - `partnerId: id ← VerificationCase.partnerId`
  - `purpose: string ← VerificationCase.purpose`
  - `version: int ← VerificationCase.version`
  - `at: instant ← at`

### EvidenceCollected

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `evidenceId: id ← cmd.evidenceId`
  - `field: string ← cmd.field`
  - `source: string ← cmd.source`
  - `contentHash: string ← cmd.contentHash`
  - `evidenceRef: id ← cmd.evidenceRef`
  - `collectedAt: instant ← cmd.collectedAt`
  - `at: instant ← at`

### DiscrepancyDetected

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `organizationId: id ← VerificationCase.organizationId`
  - `partnerId: id ← VerificationCase.partnerId`
  - `discrepancyId: id ← Discrepancy.discrepancyId`
  - `field: string ← Discrepancy.field`
  - `class: blocking | material | informational ← Discrepancy.class`
  - `at: instant ← at`

### VerificationReady

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `organizationId: id ← VerificationCase.organizationId`
  - `partnerId: id ← VerificationCase.partnerId`
  - `purpose: string ← VerificationCase.purpose`
  - `completeness: decimal ← VerificationCase.completeness`
  - `at: instant ← at`

### PartnerApproved

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `organizationId: id ← VerificationCase.organizationId`
  - `partnerId: id ← VerificationCase.partnerId`
  - `at: instant ← at`

### PartnerRejected

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `organizationId: id ← VerificationCase.organizationId`
  - `partnerId: id ← VerificationCase.partnerId`
  - `reasonCategory: cadastral | appetite ← VerificationCase.partnerDecision.reasonCategory`
  - `at: instant ← at`
  - o motivo textual do financiador não viaja no evento (02 §8).

### ReverificationRequired

- **Campos**:
  - `caseId: id ← VerificationCase.id`
  - `organizationId: id ← VerificationCase.organizationId`
  - `partnerId: id ← VerificationCase.partnerId`
  - `trigger: string ← cmd.trigger`
  - `affectedBlocks: list<string> ← cmd.affectedBlocks`
  - `at: instant ← at`

## Consultas

- `FindVerificationCase` — filtros: `caseId`; cardinalidade: `one`
- `FindVerificationCasesByOrganization` — filtros: `organizationId`, `partnerId` e `status` (opcionais); cardinalidade: `many`
- `FindBusinessProfile` — filtros: `caseId`; cardinalidade: `one` — visão derivada das evidências pela `PrecedenceRule`
- `FindOwnershipStructure` — filtros: `caseId`; cardinalidade: `one` — visão derivada das evidências `CHK-OWN`
- `FindValidEvidenceByOrganization` — filtros: `organizationId`, `field`, `asOf`; cardinalidade: `many` — reutilização de evidência ainda válida
- `FindDisclosuresByCase` — filtros: `caseId`; cardinalidade: `many` — o cedente vê o que foi divulgado e quando
- `FindVerificationCasesExpiring` — filtros: `expiresBefore`; cardinalidade: `many` — agenda a revalidação
- `FindVerificationSubject` — filtros: `organizationId`; cardinalidade: `one`

## Relações entre agregados

- `VerificationCase.organizationId` → `VerificationSubject`
- `VerificationCase.partnerId` → `Partner` (contexto `partner-management`)
- `VerificationCase.supersedesId` → `VerificationCase`
- `VerificationSubject.organizationId` → `Organization` (contexto `identity-organization`)
- `SubmitToPartner.consentGrantId` → `ConsentGrant` (contexto `consent-mandates`)

As relações são só por identidade. A existência do agregado referenciado (†) é verificada pelo caso de uso por leitura ou consulta gRPC **[ABERTA: FND-04 §3.2 — o golden `bookings` não lê outro agregado dentro da decisão; validar o padrão na fundação]**. As unicidades (††) são índices únicos cuja violação vira rejeição **[ABERTA: tradução de violação de unicidade em rejeição — sem exemplo no golden]**.

## Integração

- **Publica** (integration events), em `apps/backend/kyb/contract/proto/company/kyb/event/v1/`, package `company.kyb.event.v1`:
  - `VerificationReady` → `verification_ready.proto` — consumidores (doc 92 §4): Eligibility, Marketplace, Ledger
  - `DiscrepancyDetected` → `discrepancy_detected.proto` — Risco, Ledger
  - `PartnerApproved` → `partner_approved.proto` — Eligibility, Ledger
  - `PartnerRejected` → `partner_rejected.proto` — Eligibility, Ledger
  - `ReverificationRequired` → `reverification_required.proto` — Eligibility, Ledger
  - `VerificationOpened` → `verification_opened.proto` e `EvidenceCollected` → `evidence_collected.proto` — **[ABERTA: docs/design/92-schemas-eventos.md §4 omite os dois eventos declarados em docs/prd/02-prd-kyb-kyc.md; sem consumidor declarado além do Ledger]**
  - Consumidores declarados fora do doc 92: Backoffice consome `DiscrepancyDetected` (`docs/design/17` §4) e Notificações consome `VerificationReady` (`docs/design/16` §4) **[ABERTA: docs/design/92 §4 não os lista]**
- **Consome**:
  - `company.identity.event.v1.OrganizationRegistered` → `RegisterVerificationSubject`; mapa: `organizationId`, `taxIdType`, `taxIdHash`, `taxIdMasked`, `jurisdiction`, `kinds` ← mesmos campos do evento. Disposições de consumo por referência a FND-04 §6.4; `ErrSchemaMismatch` e `ErrMalformed` do envelope viram `Failure(Validation)` sem abrir UoW.
  - Gatilhos de revalidação por "evento interno" (`docs/design/02` §6: mudança de representante, alteração de conta) — **[ABERTA: docs/design/92 §4 não declara KYB consumidor de `RepresentationRevoked` nem de `BankAccountChange*`]**. Nesta versão `ExpireVerification` é acionado por agendador ou operador, com o `trigger` informado.
  - Consultas gRPC de saída, feitas no provider antes da transação: Identity `FindOrganization` (resolve o documento fiscal em claro, `PII`, para consultar as fontes); Partner Management (conjunto obrigatório do `PartnerAgreement`); Consent (vigência do consentimento).
  - Fontes de dados (cadastro oficial, bureaus): **[ABERTA: porta de integração com bureaus e fontes oficiais — docs/design/02 §1 (Y1); o Receivables Gateway cobre registradoras, não bureaus]** **[A CONFIRMAR: C3, 02:Y11]**.

## Borda e persistência

- **Borda**: só gRPC, `company.kyb.service.v1.KybService`, um RPC por comando (`RegisterVerificationSubject`, `OpenVerificationCase`, `AttachEvidence`, `RunCheck`, `RequestInformation`, `ResolveDiscrepancy`, `SubmitToPartner`, `RecordPartnerDecision`, `ExpireVerification`) e por consulta, atrás da cadeia de interceptors do kernel. Client API e Partner API são REST do `bff` (ver Escopo fora).
- **Banco**: database e role `kyb`, schema `public`, tabelas `verification_subjects` e `verification_cases`, com `tenant_id` à frente da chave. Persistência híbrida: `organization_id`, `partner_id` e `status` são colunas tipadas em `verification_cases` (filtros de `FindVerificationCasesByOrganization`); `next_expiry_at` também (filtro de `FindVerificationCasesExpiring`); o resto vai em `snapshot` `jsonb`. `FindValidEvidenceByOrganization` filtra dentro de lista do snapshot **[ABERTA: exige projeção de leitura de evidências; padrão não verificado no kernel]**.
- Diverge do design: o isolamento por tenant é `tenant_id` injetado pelo `ExecutionContext` no `postgres.Table` (ADR-051 DMPF), não RLS como mecanismo primário (`docs/design/00` §4.1); `tenantId` não é campo do agregado.
- Diverge do design: eventos são Protobuf com CloudEvents montado pelo relay; o envelope JSON e o `schemaUri` do doc 92 §1 não se aplicam.
- Diverge do design: `instant` é inteiro de nanossegundos no domínio (`docs/design/00` §8 fala em milissegundos).
- O acesso do financiador ao dossiê (projeção no tenant do financiador, `docs/design/00` §4.2) **[ABERTA: requer extensão do kernel — ADR-051 DMPF]**.
- Campos `PII` e `PII_SENSITIVE` (`observedValue`, documentos) pedem envelope adicional e chave segregada **[ABERTA: sem provider verificado no kernel — docs/design/90 §1.4]**.

## Políticas transversais

- **Idempotência**: `OpenVerificationCase` entra pelo `bff` por `POST` com chave de idempotência declarada; os demais são idempotentes por identidade do agregado ou da evidência (`evidenceId`) (RST-02; FND-04; ADR-056).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Papéis conforme a matriz de `docs/design/95` (registrar decisão cadastral: `PARTNER_COMPLIANCE`; ver dossiê próprio: papéis do cedente). Um parceiro não acessa evidências destinadas a outro.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); eventos vão ao Ledger. Cada acesso do parceiro a documento é registrado **[ABERTA: docs/design/02-design-kyb-kyc.md §7 — onde vive o log de acesso visível ao cedente]**.
- **Limite de responsabilidade**: o contexto coleta, verifica e organiza; não decide conformidade nem comunica a autoridades (`docs/design/02` §2).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `kyb` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários de aceite por comando:

```text
DADO um evento OrganizationRegistered de uma organização ainda desconhecida
QUANDO RegisterVerificationSubject chega
ENTÃO o sujeito é inicializado com registeredAt = at e nada é emitido

DADO um sujeito já registrado
QUANDO RegisterVerificationSubject chega de novo
ENTÃO aceita sem alterar o sujeito

DADO uma organização conhecida e nenhum caso aberto para o parceiro e a finalidade
QUANDO OpenVerificationCase chega com requiredCheckIds não vazio
ENTÃO o caso nasce open, com completeness 0 e version 1, e VerificationOpened vai à outbox

DADO um caso open
QUANDO AttachEvidence chega com contentHash e source
ENTÃO a evidência é acrescida, o caso vai a collecting e EvidenceCollected é emitido

DADO um caso collecting com uma evidência de outra fonte para o mesmo campo e valor diferente
QUANDO AttachEvidence chega com conflictBehavior flag_discrepancy
ENTÃO abre uma divergência open sem sobrescrever nenhuma fonte e emite DiscrepancyDetected

DADO um caso validating cujo único check obrigatório falta
QUANDO RunCheck chega com result passed e validUntil futuro
ENTÃO completeness vai a 1, o caso vai a ready_for_partner e VerificationReady é emitido

DADO um caso validating
QUANDO RunCheck chega com result failed e failureClass blocking
ENTÃO abre divergência blocking, completeness vai a 0 e DiscrepancyDetected é emitido

DADO um caso collecting
QUANDO RequestInformation chega com description
ENTÃO a pendência é acrescida e o caso vai a pending_information

DADO um caso pending_information com uma pendência
QUANDO AttachEvidence chega com fulfilsRequestId dessa pendência
ENTÃO a pendência recebe fulfilledAt = at e o caso volta a validating

DADO uma divergência material open
QUANDO ResolveDiscrepancy chega com resolution justified e justification
ENTÃO a divergência vai a justified e a completude é recalculada

DADO um caso ready_for_partner
QUANDO SubmitToPartner chega com disclosureLevel screening e consentGrantId
ENTÃO o caso vai a partner_review, o nível fica screening e o histórico de divulgação ganha um item

DADO um caso partner_review
QUANDO RecordPartnerDecision chega com decision approved
ENTÃO o caso vai a approved e PartnerApproved é emitido

DADO um caso partner_review
QUANDO RecordPartnerDecision chega com decision rejected e reasonCategory appetite
ENTÃO o caso vai a rejected e PartnerRejected é emitido com reasonCategory e sem o motivo textual

DADO um caso approved com um check vencido
QUANDO ExpireVerification chega com trigger check_expiry
ENTÃO o caso vai a expired e ReverificationRequired é emitido com os blocos afetados

DADO casos de duas organizações
QUANDO FindVerificationCasesByOrganization consulta uma delas
ENTÃO devolve só os casos dela, fora de qualquer UoW
```

- [ ] Um cenário por código de rejeição (nada é escrito e o estado não muda):

```text
DADO qualquer organização QUANDO OpenVerificationCase chega com requiredCheckIds vazio ENTÃO rejeita com kyb-kyc/verification-case/required-checks-empty
DADO qualquer organização QUANDO OpenVerificationCase chega com purpose vazio ENTÃO rejeita com kyb-kyc/verification-case/purpose-empty
DADO uma organização sem VerificationSubject QUANDO OpenVerificationCase chega ENTÃO rejeita com kyb-kyc/verification-case/organization-unknown
DADO um caso não final para a mesma organização, parceiro e finalidade QUANDO OpenVerificationCase chega ENTÃO rejeita com kyb-kyc/verification-case/case-already-open
DADO um caso ready_for_partner QUANDO AttachEvidence, RunCheck, RequestInformation ou ResolveDiscrepancy chega ENTÃO rejeita com kyb-kyc/verification-case/collection-closed
DADO um caso collecting QUANDO AttachEvidence chega com contentHash vazio ENTÃO rejeita com kyb-kyc/verification-case/content-hash-missing
DADO um caso collecting QUANDO AttachEvidence chega com source vazia ENTÃO rejeita com kyb-kyc/verification-case/source-missing
DADO um caso sem pendência aberta QUANDO AttachEvidence chega com fulfilsRequestId ENTÃO rejeita com kyb-kyc/verification-case/request-not-pending
DADO um caso collecting QUANDO RunCheck chega com validUntil igual ou anterior a at ENTÃO rejeita com kyb-kyc/verification-case/validity-invalid
DADO um caso collecting QUANDO RunCheck chega com evidenceIds ausentes do caso ENTÃO rejeita com kyb-kyc/verification-case/evidence-unknown
DADO um caso collecting QUANDO RunCheck chega com result failed e sem failureClass ENTÃO rejeita com kyb-kyc/verification-case/failure-class-missing
DADO um caso collecting QUANDO RequestInformation chega com description vazia ENTÃO rejeita com kyb-kyc/verification-case/description-empty
DADO uma divergência já resolved QUANDO ResolveDiscrepancy chega ENTÃO rejeita com kyb-kyc/verification-case/discrepancy-not-open
DADO uma divergência blocking open QUANDO ResolveDiscrepancy chega com resolution justified ENTÃO rejeita com kyb-kyc/verification-case/blocking-cannot-be-justified
DADO uma divergência material open QUANDO ResolveDiscrepancy chega com justified e justification vazia ENTÃO rejeita com kyb-kyc/verification-case/justification-empty
DADO um caso ready_for_partner QUANDO SubmitToPartner chega sem consentGrantId ENTÃO rejeita com kyb-kyc/verification-case/consent-missing
DADO um caso collecting QUANDO SubmitToPartner chega ENTÃO rejeita com kyb-kyc/verification-case/not-ready-for-partner
DADO um caso partner_review com nível analysis QUANDO SubmitToPartner chega com screening ENTÃO rejeita com kyb-kyc/verification-case/level-not-escalating
DADO um caso ready_for_partner QUANDO RecordPartnerDecision chega ENTÃO rejeita com kyb-kyc/verification-case/not-in-partner-review
DADO um caso partner_review QUANDO RecordPartnerDecision chega com rejected e sem reasonCategory ENTÃO rejeita com kyb-kyc/verification-case/reason-category-missing
DADO um caso rejected QUANDO ExpireVerification chega ENTÃO rejeita com kyb-kyc/verification-case/already-closed
DADO um caso cujos checks estão todos dentro da validade QUANDO ExpireVerification chega com trigger check_expiry ENTÃO rejeita com kyb-kyc/verification-case/nothing-expired
```

## Escopo fora

- **REST do `bff`**: Client API e Partner API (`docs/design/91-contratos-api.md` §3.2 com o erro `KYB_INCOMPLETE`, §4.1 com o `kybSummary` do convite) são do `bff`; o contexto serve só gRPC.
- **Decisão regulatória e de crédito**: aprovação final de PLD/FT, rating, decisão de financiamento e comunicação a autoridades pelo financiador (`docs/prd/02` Fora do escopo). Um fluxo de comunicação de operação suspeita só existe se A2 (02:Y8) confirmar obrigação própria; o ponto de extensão é o `RiskSignal`.
- **Sinais de risco e fraude**: contexto Fraude e Risco (`SPEC-H4DR929G`); o dono de `RiskSignal` está **[ABERTA]** em Agregados.
- **Identidade e poderes dos representantes**: pertencem a Identity; quem comprova e emite `RepresentationVerified` está **[ABERTA]** em Agregados.
- **Consentimento**: Consent & Mandates concede e revoga; este contexto só referencia o `consentGrantId` ao divulgar.
- **Catálogo de verificações e `PrecedenceRule` como dado versionado**: carregados como configuração, sem agregado nesta versão.
- **Fontes de dados e `CHK-BNK-001`**: adaptadores só depois de C3 e C1; listas obrigatórias (02:Y10) e critério de beneficiário final (02:Y9) idem.
- **Monitoramento e gatilhos automáticos de revalidação** (consulta periódica, comparação de estrutura): agendador fora do contexto, que invoca `ExpireVerification`.
- **Acesso do financiador ao dossiê e log de acessos para o cedente**: dependem da extensão de kernel (ADR-051) e da decisão do log.
- **Notificação ao cedente e ao parceiro**: contexto Notificações.
- **Armazenamento de evidência bruta**: **[ABERTA: provider de object storage]**.
- **Composition root**: o esqueleto de `app/` e o `cmd/` com `--role` nascem do generator; o harness escreve o conteúdo de domínio dentro deles.
