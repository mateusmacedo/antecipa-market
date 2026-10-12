---
id: SPEC-9TEQ9EV9
slug: partner-management
title: Bounded context — partner-management
stage: backlog
priority: P0
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6]
ticket_url: https://linear.app/mmda/issue/DEVS-78/spec-9teq9ev9-bounded-context-partner-management
subtask_urls: []
created: 2026-10-12
---

# SPEC-9TEQ9EV9: Bounded context — partner-management

## Identidade

- **bounded_context**: `partner-management`
- **name**: `partner`
- **Linguagem ubíqua**:
  - *Parceiro* (`Partner`) — organização externa com um papel: financiador, escriturador, registradora, depositário, provedor cadastral ou instituição de liquidação.
  - *Acordo* (`PartnerAgreement`) — relação contratual do parceiro e sua vigência.
  - *Capacidade* (`Capability`) — operação sobre um tipo de ativo, em um ambiente e uma versão, com restrições declaradas.
  - *Homologação* — verificação dos critérios técnicos H1 a H12 de uma capacidade; os contratuais H13 a H18 ficam na ativação do acordo.
  - *Produto financeiro* (`FinancialProduct`) — produto ofertável, com faixas de valor e prazo, ativos e regiões, versionado.
  - *Perfil de integração* (`IntegrationProfile`) — protocolos, endpoint, certificado e referência de segredo por ambiente.
  - *Política operacional* (`OperationalPolicy`) — SLA, limite de taxa, calendário, janela e contingência.
  - *Regra comercial* (`CommercialRule`) — remuneração e impacto permitido na apresentação; nunca altera o valor da oferta.
  - *Capacidade efetiva* — resolução de contrato vigente, capacidade habilitada e homologada, produto vigente, ausência de suspensão, rollout e parâmetros compatíveis.
  - *Rollout* — exposição gradual de uma capacidade por tenant.
  - *Suspensão* — bloqueia comandos novos e nunca o acompanhamento dos existentes.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/13-prd-partner-management.md`; `docs/design/13-design-partner-management.md` §2 (homologação), §3 (regra comercial), §4 (capacidade efetiva), §5 (suspensão), §6 (rollout), §7 (mudança de contrato de integração), §8 (descontinuidade); `docs/design/04-design-receivables-gateway.md` §2 (`CapabilityProfile`); `docs/design/02-design-kyb-kyc.md` §4; `docs/design/92-schemas-eventos.md` §4; `docs/prd/14-catalogo-contratos-eventos.md`.
- **Situação**: `parcial` — cadastro, estágios, acordo, capacidade, homologação, suspensão, rollout, produto e perfil de integração são construíveis; a regra comercial depende do modelo de monetização e os critérios de homologação dependem de acordo com cada parceiro.
- **Fase de entrega**: 1.
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-7KW0HHF6`, Identity: a organização do parceiro); a cadeia transitiva fica implícita. Brandas: Receivables Gateway (`ExternalSchemaMismatchDetected` leva à suspensão, 13 §7) e Marketplace (`FundingRequestOpened`). A retroalimentação — Gateway, Eligibility e Marketplace consomem os eventos de capacidade deste contexto — nunca entra em `depends_on`.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| #1 / A1 | autorização regulatória da plataforma (transversal) | existencial para o contexto inteiro |
| #7, 13:N9 | modelo de monetização: quem paga, base de cálculo, fato gerador (`docs/design/18` G1–G3) | `CommercialRule` inteira e `DefineCommercialRule` |
| B17, 13:N8 | ambiente de homologação e critérios acordáveis com cada parceiro | `RecordHomologation`; `EnableCapability` por parceiro |
| 13:N10 | situação regulatória a verificar por tipo de parceiro | critério H16 de `ActivateAgreement` |
| 13:N11 | cláusula de acesso residual no contrato de saída | `ChangePartnerStage` para `inactive`; offboarding |

- **Decisões de produto abertas**: `canOverrideMerit` na `CommercialRule` (`docs/design/07-design-marketplace.md` §3.3, R6); fronteira entre `OperationalPolicy` e `IntegrationProfile` deste contexto e o `CapabilityProfile` do Gateway (`docs/design/13` cabeçalho); dono da suspensão automática por quebra de schema (em Integração).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída; `SPEC-7KW0HHF6` concluída (organização do parceiro). O núcleo não espera #7 nem B17: os critérios H1 a H18 vêm do design. `CommercialRule` fica fora da primeira fatia até o #7 ser decidido.

## Agregados

Os agregados versionados (`FinancialProduct`, `IntegrationProfile`, `OperationalPolicy`, `CommercialRule`, `Capability`, `PartnerAgreement`) ganham uma linha por versão, com `version` e `supersedesId`; o comando criador é um só e cobre a primeira versão e as seguintes ("Alterações de parceiro são reproduzíveis historicamente"). `RolloutPolicy` é parte de `Capability`.

### Partner

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `role: funder | bookkeeper | registry | depository | data_provider | settlement_institution`
  - `status: prospect | contracting | integrating | homologating | active | suspended | offboarding | inactive`
  - `registeredAt: instant`, `stageChangedAt: instant`
  - `stageReason: string` (opcional), `offboardingStartedAt: instant` (opcional)
- **Estados**: os oito de `status`.
- **Transições permitidas** (todas por `ChangePartnerStage`):
  - (novo) → `prospect` — por `RegisterPartner`
  - `prospect` → `contracting` → `integrating` → `homologating` → `active`, um estágio por vez
  - `active` → `suspended`; `suspended` → `active` **[ABERTA: docs/prd/13-prd-partner-management.md Estados — o PRD não mostra o retorno de `SUSPENDED` a `ACTIVE`]**
  - `active` | `suspended` → `offboarding`
  - `offboarding` → `inactive`
- **Invariantes de estado**:
  - Um estágio fora da tabela rejeita com `partner-management/partner/invalid-stage-transition`.
  - `active` exige acordo ativo e ao menos uma capacidade habilitada.
  - `inactive` só é alcançável sem efeito vigente pendente de liberação (`docs/design/13` §8).
  - Suspensão não apaga operação nem evidência existente.

### PartnerAgreement

- **Identidade**: `generated`; versionado
- **Campos**: `partnerId: id`, `version: int`, `supersedesId: id` (opcional); `status: draft | active`; `validFrom: instant`, `validUntil: instant` (opcional; ausente = indeterminado); `contractEvidenceId: id` (opcional até a ativação) **[ABERTA: provider de object storage]**; `instrumentTemplateId: id` (opcional) — modelo do instrumento contratual homologado (`docs/design/08` §2) **[ABERTA: docs/design/08-design-contracting.md §2 — o modelo híbrido é recomendação, a decisão de produto está aberta]**; `activatedAt: instant` (opcional)
- **Estados**: `draft`, `active`
- **Transições permitidas**:
  - (novo) → `draft` — por `RegisterAgreement`
  - `draft` → `active` — por `ActivateAgreement`
- **Invariantes de estado**:
  - `validUntil`, quando presente, é posterior a `validFrom` — rejeição `partner-management/partner-agreement/validity-invalid`.
  - A vigência no instante é avaliada pela data, não por estado: acordo `active` fora da vigência não sustenta capacidade efetiva.

### Capability

- **Identidade**: `generated`; versionada
- **Campos**:
  - `partnerId: id`, `operation: string`, `assetType: string`, `environment: string`
  - `version: int`, `supersedesId: id` (opcional)
  - `isWrite: bool`, `stateQuerySupported: bool`, `restrictionNote: string` (opcional)
  - `status: declared | homologated | enabled | suspended`
  - `homologation: list<CriterionResult>` — `criterionId: string` (`H1` a `H12`), `passed: bool`, `evidenceId: id` (opcional), `recordedAt: instant`; coleção indispensável: a transição para `homologated` depende do conjunto
  - `rollout`: `mode: disabled | allowlist | percentage | enabled`, `allowlistTenantIds: list<id>`, `percentage: int`, `excludedTenantIds: list<id>` — listas por definição do rollout
  - `suspension` (opcional): `reason: string`, `trigger: manual | schema_mismatch | offboarding`, `suspendedAt: instant`
  - `enabledAt: instant` (opcional)
- **Estados**: `declared`, `homologated`, `enabled`, `suspended`
- **Transições permitidas**:
  - (novo) → `declared` — por `DeclareCapability`
  - `declared` → `homologated` — por `RecordHomologation`, quando todos os critérios aplicáveis passaram
  - `homologated` | `suspended` → `enabled` — por `EnableCapability`
  - `enabled` → `suspended` — por `SuspendCapability`
- **Invariantes de estado**:
  - Capacidade ativa exige contrato, homologação e configuração válidos — `not-homologated`, `agreement-not-active` e `integration-profile-missing` em `EnableCapability`.
  - Critérios aplicáveis: H1, H2, H4 a H8 e H10 a H12 sempre; H3 (consulta de estado) em operação de escrita com `stateQuerySupported`; H9 (callback) quando aplicável. Operação de escrita sem consulta de estado só é homologada com `restrictionNote` declarada — toda indefinição vira caso humano, por decisão consciente (`docs/design/13` §2.1).
  - O rollout é determinístico: um tenant em `percentage` permanece do mesmo lado entre avaliações; `excludedTenantIds` prevalece sobre qualquer modo; reduzir o rollout impede operações novas e não interrompe as em curso.
  - Suspensão bloqueia comandos novos e nunca o acompanhamento dos existentes; efeito vigente permanece e a liberação continua possível.

### FinancialProduct

- **Identidade**: `generated`; versionado
- **Campos**: `partnerId: id`, `agreementId: id`, `code: string`, `name: string`; `version: int`, `supersedesId: id` (opcional); `status: active | retired`; `validFrom: instant`, `validUntil: instant` (opcional); `assetTypes: list<string>`, `regions: list<string>` — faixas atendidas; `currency: string`, `minAmountMinor: int`, `maxAmountMinor: int`; `minTermDays: int`, `maxTermDays: int` **[ABERTA: docs/design/13-design-partner-management.md — a unidade do prazo não é definida]**; `requiredCheckIds: list<string>` — verificações `CHK-*` que o produto exige no KYB (`docs/design/02` §4)
- **Estados**: `active`, `retired`
- **Transições permitidas**:
  - (novo) → `active` — por `DefineProduct`
  - `active` → `retired` — por `RetireProduct`
- **Invariantes de estado**: produto `retired` ou fora da vigência não recebe novos pedidos; `min` não excede `max` — `partner-management/financial-product/range-invalid`.

### IntegrationProfile

- **Identidade**: `generated`; versionado por ambiente
- **Campos**: `partnerId: id`, `environment: string`, `version: int`, `supersedesId: id` (opcional); `changeKind: initial | announced | unannounced`; `protocols: list<string>`, `endpoint: string`, `schemaVersion: string`; `secretRef: string` — referência ao segredo; o valor nunca entra no domínio; `certificateRef: string`, `certificateExpiresAt: instant`; `effectiveFrom: instant`; `lastAlertedThresholdDays: int` (opcional)
- **Estados**: nenhum
- **Transições permitidas**: nenhuma; mudança de endpoint, certificado ou schema registra versão nova.
- **Invariantes de estado**: segredos são referenciados, nunca expostos; alerta de vencimento nos limiares de 60, 30 e 7 dias, sem repetir o mesmo limiar.

### OperationalPolicy

- **Identidade**: `generated`; versionada
- **Campos**: `partnerId: id`, `version: int`, `supersedesId: id` (opcional), `effectiveFrom: instant`; `rateLimitPerMinute: int`; `businessDays: list<string>`, `cutoffTime: string`, `processingWindow: string`; `slaRef: string`, `fallback: string` **[ABERTA: docs/prd/13-prd-partner-management.md — os valores de contingência não são definidos]**; `supportContact: string`, `incidentContact: string` — critérios H11 e H12
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**: a fronteira com o `CapabilityProfile` do Gateway (limites, calendário) **[ABERTA: docs/design/13-design-partner-management.md cabeçalho e docs/design/04-design-receivables-gateway.md §2.1 — qual contexto é a fonte dos limites e do calendário]**.

### CommercialRule

**[ABERTA: #7 (D1–D3; 13:N9) — o modelo de monetização trava o agregado inteiro; o conteúdo abaixo é o desenho de `docs/design/13` §3, sem valor decidido]**

- **Identidade**: `generated`; versionada
- **Campos**: `partnerId: id`, `version: int`, `supersedesId: id` (opcional); `effectiveFrom: instant`, `effectiveUntil: instant` (opcional); `remunerationModel: none | per_operation | subscription | mixed`; `payer: cedent | funder | both`; `calculationBasis: string`, `rates: list<decimal>`, `triggerEvent: string`; `sponsoredPlacementAllowed: bool`, `maxSponsoredPositions: int`; `canOverrideMerit: bool` **[ABERTA: docs/design/07-design-marketplace.md §3.3 R6 — decisão de produto]**; `disclosureRequired: bool` — sempre verdadeiro, definido pelo domínio e nunca recebido
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**: `disclosureRequired` é sempre verdadeiro; a regra altera posição e divulgação, nunca o valor da oferta (verificável: o `PricingBreakdown` normalizado não a referencia).

## Comandos (UPRs)

(†) marca pré-condição que depende de outro agregado ou contexto: o caso de uso a verifica por leitura ou consulta gRPC antes de decidir. Ver Relações entre agregados.

### RegisterPartner

- **Agregado**: `Partner`; **cria**
- **Entrada**: `organizationId: id`; `role`
- **Pré-condições**:
  - organização existente em Identity (†) → `partner-management/partner/organization-not-found`
  - `kinds` da organização compatíveis com o papel: `funder` para financiador, `partner` para os demais (†) → `partner-management/partner/organization-kind-mismatch`
- **Efeitos sobre o estado**: `status ← prospect`; `registeredAt ← at`; `stageChangedAt ← at`
- **Eventos emitidos**: `PartnerRegistered`
- **Resposta**: `partnerId: id`

### ChangePartnerStage

- **Agregado**: `Partner`; exige existente
- **Entrada**: `target: contracting | integrating | homologating | active | suspended | offboarding | inactive`; `reason: string`
- **Pré-condições**:
  - a transição está na tabela do agregado → `partner-management/partner/invalid-stage-transition`
  - `reason` não vazio ao suspender ou iniciar o offboarding → `partner-management/partner/reason-empty`
  - para `active` a partir de `homologating`: acordo ativo e vigente (†) → `partner-management/partner/agreement-not-active`
  - para `active` a partir de `homologating`: ao menos uma capacidade `enabled` (†) → `partner-management/partner/no-enabled-capability`
  - para `inactive`: nenhum efeito vigente pendente de liberação (†) → `partner-management/partner/effects-pending-release` **[A CONFIRMAR: 13:N11]**
- **Efeitos sobre o estado**: `status ← target`; `stageChangedAt ← at`; `stageReason ← reason`; `offboardingStartedAt ← at` ao entrar em `offboarding`
- **Eventos emitidos**: `PartnerOffboardingStarted` (só ao entrar em `offboarding`)
- **Resposta**: `partnerId: id`, `status`

A suspensão do parceiro inteiro ou o offboarding bloqueiam as capacidades por `SuspendCapability` em cada capacidade `enabled` do parceiro, a cargo do caso de uso **[ABERTA: docs/design/13-design-partner-management.md §5 — os escopos de suspensão por produto, tipo de ativo e parceiro inteiro não têm comando próprio; o fan-out por capacidade é o mínimo coerente]**.

### RegisterAgreement

- **Agregado**: `PartnerAgreement`; **cria**
- **Entrada**: `partnerId: id`; `validFrom: instant`, `validUntil: instant` (opcional); `instrumentTemplateId: id` (opcional); `supersedesId: id` (opcional)
- **Pré-condições**:
  - `validUntil`, quando informado, posterior a `validFrom` → `partner-management/partner-agreement/validity-invalid`
- **Efeitos sobre o estado**: `status ← draft`; `version ← 1`, ou a do acordo substituído mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `agreementId: id`

### ActivateAgreement

- **Agregado**: `PartnerAgreement`; exige existente
- **Entrada**: `contractEvidenceId: id`; `satisfiedCriteria: list<string>` — critérios contratuais `H14` a `H18` atendidos; `regulatoryCheckApplicable: bool`
- **Pré-condições**:
  - `status = draft` → `partner-management/partner-agreement/not-draft`
  - `contractEvidenceId` não vazio (H13, contrato assinado e vigente) → `partner-management/partner-agreement/contract-evidence-missing`
  - `satisfiedCriteria` cobre H14, H15, H17 e H18, e H16 quando `regulatoryCheckApplicable` → `partner-management/partner-agreement/contractual-criteria-incomplete` **[A CONFIRMAR: 13:N10]**
  - `validUntil`, quando presente, posterior a `at` → `partner-management/partner-agreement/validity-invalid`
- **Efeitos sobre o estado**: `status ← active`; `contractEvidenceId ← cmd`; `activatedAt ← at`
- **Eventos emitidos**: `PartnerAgreementActivated`
- **Resposta**: `agreementId: id`

### DeclareCapability

- **Agregado**: `Capability`; **cria**
- **Entrada**: `partnerId: id`, `operation: string`, `assetType: string`, `environment: string`; `isWrite: bool`, `stateQuerySupported: bool`, `restrictionNote: string` (opcional); `supersedesId: id` (opcional)
- **Pré-condições**:
  - `operation` não vazia → `partner-management/capability/operation-empty`
  - `assetType` não vazio → `partner-management/capability/asset-type-empty`
  - operação de escrita sem consulta de estado exige `restrictionNote` → `partner-management/capability/state-query-restriction-undeclared`
- **Efeitos sobre o estado**: `status ← declared`; `rollout.mode ← disabled`; `version ← 1`, ou a da capacidade substituída mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `capabilityId: id`

### RecordHomologation

- **Agregado**: `Capability`; exige existente
- **Comando**: `RecordHomologation` — pode ser repetido até a homologação completar.
- **Entrada**: `results: list<{criterionId, passed, evidenceId}>` — critérios H1 a H12; `callbackApplicable: bool`
- **Pré-condições**:
  - `results` não vazio → `partner-management/capability/criteria-empty`
  - todo `criterionId` entre H1 e H12 → `partner-management/capability/unknown-criterion`
  - `status = declared` → `partner-management/capability/homologation-closed`
- **Efeitos sobre o estado**: `homologation ←` resultados substituídos por `criterionId`, com `recordedAt ← at`; `declared` → `homologated` quando todos os critérios aplicáveis têm `passed`; senão permanece `declared`
- **Eventos emitidos**: nenhum
- **Resposta**: `capabilityId: id`, `status`

### EnableCapability

- **Agregado**: `Capability`; exige existente
- **Comando**: `EnableCapability` — habilita uma capacidade homologada e também a retoma após suspensão.
- **Entrada**: nenhuma além da identidade do agregado
- **Pré-condições**:
  - `status` em `homologated` ou `suspended` → `partner-management/capability/not-homologated`
  - acordo ativo e vigente do parceiro (†) → `partner-management/capability/agreement-not-active`
  - perfil de integração vigente para o ambiente (†) → `partner-management/capability/integration-profile-missing`
- **Efeitos sobre o estado**: `status ← enabled`; `enabledAt ← at`; `suspension ← vazio`
- **Eventos emitidos**: `CapabilityEnabled`
- **Resposta**: `capabilityId: id`

### SuspendCapability

- **Agregado**: `Capability`; exige existente
- **Entrada**: `reason: string`; `trigger: manual | schema_mismatch | offboarding`
- **Pré-condições**:
  - `status = enabled` → `partner-management/capability/not-enabled`
  - `reason` não vazio → `partner-management/capability/reason-empty`
- **Efeitos sobre o estado**: `status ← suspended`; `suspension ←` {`reason`, `trigger`, `suspendedAt ← at`}
- **Eventos emitidos**: `CapabilitySuspended`
- **Resposta**: `capabilityId: id`

### ChangeRollout

- **Agregado**: `Capability`; exige existente
- **Entrada**: `mode: disabled | allowlist | percentage | enabled`; `allowlistTenantIds: list<id>`, `percentage: int`, `excludedTenantIds: list<id>`
- **Pré-condições**:
  - `percentage` entre 0 e 100 em `percentage` → `partner-management/capability/percentage-out-of-range`
  - `allowlistTenantIds` não vazia em `allowlist` → `partner-management/capability/allowlist-empty`
- **Efeitos sobre o estado**: `rollout ← cmd`
- **Eventos emitidos**: nenhum **[ABERTA: docs/design/13-design-partner-management.md §4 e §6 pedem cache invalidado por evento e efeito em menos de 30 s, mas o catálogo (docs/design/92 §4) não traz evento de mudança de rollout]**
- **Resposta**: `capabilityId: id`

### DefineProduct

- **Agregado**: `FinancialProduct`; **cria**
- **Comando**: `DefineProduct` — cria a primeira versão e as seguintes.
- **Entrada**: `partnerId: id`, `agreementId: id`, `code: string`, `name: string`; `assetTypes: list<string>`, `regions: list<string>`; `currency: string`, `minAmountMinor: int`, `maxAmountMinor: int`; `minTermDays: int`, `maxTermDays: int`; `requiredCheckIds: list<string>`; `validFrom: instant`, `validUntil: instant` (opcional); `supersedesId: id` (opcional)
- **Pré-condições**:
  - `assetTypes` não vazio → `partner-management/financial-product/asset-types-empty`
  - `minAmountMinor` não excede `maxAmountMinor` e `minTermDays` não excede `maxTermDays` → `partner-management/financial-product/range-invalid`
  - `validUntil`, quando informado, posterior a `validFrom` → `partner-management/financial-product/validity-invalid`
  - acordo ativo e vigente (†) → `partner-management/financial-product/agreement-not-active`
- **Efeitos sobre o estado**: `status ← active`; `version ← 1`, ou a do produto substituído mais 1
- **Eventos emitidos**: `PartnerProductChanged` (`changeKind` = `defined` na primeira versão, `changed` nas seguintes)
- **Resposta**: `productId: id`

### RetireProduct

- **Agregado**: `FinancialProduct`; exige existente
- **Entrada**: `reason: expired | withdrawn`
- **Pré-condições**:
  - `status = active` → `partner-management/financial-product/not-active`
  - `expired` exige `validUntil` menor ou igual a `at` → `partner-management/financial-product/not-yet-expired`
- **Efeitos sobre o estado**: `status ← retired`
- **Eventos emitidos**: `PartnerProductChanged` (`changeKind` = `expired` ou `retired`)
- **Resposta**: `productId: id`

### RegisterIntegrationProfile

- **Agregado**: `IntegrationProfile`; **cria**
- **Comando**: `RegisterIntegrationProfile` — registra o perfil inicial e toda mudança de endpoint, certificado ou schema como versão nova.
- **Entrada**: `partnerId: id`, `environment: string`; `changeKind: initial | announced | unannounced`; `protocols: list<string>`, `endpoint: string`, `schemaVersion: string`; `secretRef: string`, `certificateRef: string`, `certificateExpiresAt: instant`; `effectiveFrom: instant`, `supersedesId: id` (opcional)
- **Pré-condições**:
  - `endpoint` não vazio → `partner-management/integration-profile/endpoint-empty`
  - `secretRef` não vazia → `partner-management/integration-profile/secret-ref-missing`
  - `certificateExpiresAt` posterior a `at` → `partner-management/integration-profile/certificate-expiry-invalid`
- **Efeitos sobre o estado**: campos ← cmd; `version ← 1`, ou a do perfil substituído mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `profileId: id`

### RaiseCertificateExpiryAlert

- **Agregado**: `IntegrationProfile`; exige existente
- **Entrada**: `thresholdDays: int`
- **Pré-condições**:
  - `thresholdDays` em 60, 30 ou 7 → `partner-management/integration-profile/invalid-alert-threshold`
  - `certificateExpiresAt` menos `thresholdDays` dias menor ou igual a `at` → `partner-management/integration-profile/threshold-not-reached`
  - limiar ainda não alertado, nem um menor → `partner-management/integration-profile/already-alerted`
- **Efeitos sobre o estado**: `lastAlertedThresholdDays ← cmd.thresholdDays`
- **Eventos emitidos**: `PartnerCertificateExpiring`
- **Resposta**: `profileId: id`

### DefineOperationalPolicy

- **Agregado**: `OperationalPolicy`; **cria**
- **Entrada**: `partnerId: id`, `effectiveFrom: instant`, `supersedesId: id` (opcional); `rateLimitPerMinute: int`; `businessDays: list<string>`, `cutoffTime: string`, `processingWindow: string`; `slaRef: string`, `fallback: string`; `supportContact: string`, `incidentContact: string`
- **Pré-condições**:
  - `rateLimitPerMinute` maior que zero → `partner-management/operational-policy/rate-limit-invalid`
  - `supportContact` e `incidentContact` não vazios (H11, H12) → `partner-management/operational-policy/contacts-missing`
- **Efeitos sobre o estado**: campos ← cmd; `version ← 1`, ou a da política substituída mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `policyId: id`

### DefineCommercialRule

**[ABERTA: #7 (D1–D3; 13:N9) — comando travado pelo modelo de monetização]**

- **Agregado**: `CommercialRule`; **cria**
- **Entrada**: `partnerId: id`, `effectiveFrom: instant`, `effectiveUntil: instant` (opcional), `supersedesId: id` (opcional); `remunerationModel`, `payer`, `calculationBasis: string`, `rates: list<decimal>`, `triggerEvent: string`; `sponsoredPlacementAllowed: bool`, `maxSponsoredPositions: int`, `canOverrideMerit: bool`
- **Pré-condições**:
  - `effectiveUntil`, quando informado, posterior a `effectiveFrom` → `partner-management/commercial-rule/validity-invalid`
  - sem `sponsoredPlacementAllowed`, `maxSponsoredPositions` é 0 e `canOverrideMerit` é falso → `partner-management/commercial-rule/sponsored-positions-invalid`
  - `rates` vazio só com `remunerationModel = none` → `partner-management/commercial-rule/rates-invalid`
- **Efeitos sobre o estado**: campos ← cmd; `disclosureRequired ← verdadeiro`; `version ← 1`, ou a da regra substituída mais 1
- **Eventos emitidos**: nenhum **[ABERTA: docs/design/92-schemas-eventos.md §4 não traz evento de `CommercialRule`, embora Billing dependa dela (docs/design/18)]**
- **Resposta**: `ruleId: id`

## Eventos de domínio

`EventName()` é `partner-management.<agregado>.<evento>`.

### PartnerRegistered

- **Campos**: `partnerId: id ← Partner.id`; `organizationId: id ← Partner.organizationId`; `role ← Partner.role`; `at: instant ← at`

### PartnerAgreementActivated

- **Campos**: `agreementId: id ← PartnerAgreement.id`; `partnerId: id ← PartnerAgreement.partnerId`; `version: int ← PartnerAgreement.version`; `validFrom: instant ← PartnerAgreement.validFrom`; `validUntil: instant ← PartnerAgreement.validUntil`; `at: instant ← at`

### CapabilityEnabled

- **Campos**: `capabilityId: id ← Capability.id`; `partnerId: id ← Capability.partnerId`; `operation: string ← Capability.operation`; `assetType: string ← Capability.assetType`; `environment: string ← Capability.environment`; `version: int ← Capability.version`; `rolloutMode ← Capability.rollout.mode`; `at: instant ← at`

### CapabilitySuspended

- **Campos**: `capabilityId: id ← Capability.id`; `partnerId: id ← Capability.partnerId`; `operation: string ← Capability.operation`; `assetType: string ← Capability.assetType`; `reason: string ← cmd.reason`; `trigger ← cmd.trigger`; `at: instant ← at`

### PartnerProductChanged

- **Campos**: `productId: id ← FinancialProduct.id`; `partnerId: id ← FinancialProduct.partnerId`; `version: int ← FinancialProduct.version`; `changeKind: defined | changed | expired | retired ← cmd`; `validFrom: instant ← FinancialProduct.validFrom`; `validUntil: instant ← FinancialProduct.validUntil`; `at: instant ← at`

### PartnerCertificateExpiring

- **Campos**: `profileId: id ← IntegrationProfile.id`; `partnerId: id ← IntegrationProfile.partnerId`; `environment: string ← IntegrationProfile.environment`; `certificateExpiresAt: instant ← IntegrationProfile.certificateExpiresAt`; `thresholdDays: int ← cmd.thresholdDays`; `at: instant ← at`

### PartnerOffboardingStarted

- **Campos**: `partnerId: id ← Partner.id`; `organizationId: id ← Partner.organizationId`; `reason: string ← Partner.stageReason`; `at: instant ← Partner.offboardingStartedAt`

## Consultas

- `FindPartner` — filtros: `partnerId`; cardinalidade: `one`
- `FindPartnersByOrganization` — filtros: `organizationId`; cardinalidade: `many`
- `FindEligiblePartners` — filtros: `assetType`, `operation`, `tenantId`, `amountMinor`, `termDays`, `region`, `asOf`; cardinalidade: `many` — o Marketplace encontra só financiadores elegíveis ao pedido
- `FindEffectiveCapability` — filtros: `partnerId`, `operation`, `assetType`, `tenantId`, `asOf` e, opcionais, `productId`, `amountMinor`, `termDays`; cardinalidade: `one` — devolve permitido ou negado com a condição que falhou
- `FindCapabilitiesByPartner` — filtros: `partnerId`, `status` (opcional); cardinalidade: `many`
- `FindProductsByPartner` — filtros: `partnerId`, `asOf`; cardinalidade: `many`
- `FindAgreementsByPartner` — filtros: `partnerId`; cardinalidade: `many`
- `FindIntegrationProfile` — filtros: `partnerId`, `environment`, `asOf`; cardinalidade: `one`
- `FindOperationalPolicy` — filtros: `partnerId`, `asOf`; cardinalidade: `one`
- `FindCommercialRule` — filtros: `partnerId`, `asOf`; cardinalidade: `one`
- `FindCertificatesExpiring` — filtros: `expiresBefore`; cardinalidade: `many`

A capacidade efetiva é: contrato vigente no instante E capacidade habilitada e homologada E produto vigente E ausência de suspensão da operação E tenant incluído no rollout E parâmetros compatíveis. Se a consulta não puder confirmar, o consumidor nega (`docs/design/13` §4).

## Relações entre agregados

- `Partner.organizationId` → `Organization` (contexto `identity-organization`)
- `PartnerAgreement.partnerId` → `Partner`
- `Capability.partnerId` → `Partner`
- `FinancialProduct.partnerId` → `Partner`; `FinancialProduct.agreementId` → `PartnerAgreement`
- `IntegrationProfile.partnerId`, `OperationalPolicy.partnerId`, `CommercialRule.partnerId` → `Partner`
- `supersedesId` → o próprio agregado, na versão anterior

As relações são só por identidade. A verificação (†) de existência e vigência do agregado referenciado é do caso de uso, por leitura ou consulta gRPC **[ABERTA: FND-04 §3.2 — o golden `bookings` não lê outro agregado dentro da decisão; validar o padrão na fundação]**.

## Integração

- **Publica** (integration events), em `apps/backend/partner/contract/proto/company/partner/event/v1/`, package `company.partner.event.v1`, no tópico do contexto (Kafka, default do DMPF; confirmação na decisão D4 da fundação):
  - `CapabilityEnabled` → `capability_enabled.proto` — consumidores (doc 92 §4): Gateway, Eligibility, Marketplace
  - `CapabilitySuspended` → `capability_suspended.proto` — Gateway, Eligibility, Marketplace, Ops
  - `PartnerProductChanged` → `partner_product_changed.proto` — Eligibility, Marketplace
  - `PartnerCertificateExpiring` → `partner_certificate_expiring.proto` — Ops
  - `PartnerRegistered` → `partner_registered.proto`, `PartnerAgreementActivated` → `partner_agreement_activated.proto` e `PartnerOffboardingStarted` → `partner_offboarding_started.proto` — **[ABERTA: docs/design/92-schemas-eventos.md §4 omite os três eventos declarados em docs/prd/13-prd-partner-management.md; sem consumidor declarado além do Ledger]**
  - A propagação de uma suspensão chega aos consumidores em menos de 30 s (`docs/design/13` §4); o cache dos consumidores é invalidado por estes eventos.
- **Consome**:
  - `company.gateway.event.v1.ExternalSchemaMismatchDetected` → `SuspendCapability` com `trigger = schema_mismatch`; mapa: `partnerId` e `operation` ← mesmos campos do evento, `capabilityId` resolvido pelo caso de uso (†), `reason` ← classificação do evento. Disposições de consumo por referência a FND-04 §6.4; `ErrSchemaMismatch` e `ErrMalformed` do envelope viram `Failure(Validation)` sem abrir UoW. **[ABERTA: docs/design/04-design-receivables-gateway.md §5 manda o Gateway suspender a operação ao detectar a quebra, e docs/design/13 §7 manda Partner Management fazer a suspensão automática; o schema do evento também não está em docs/design/92 §3]**
  - `company.marketplace.event.v1.FundingRequestOpened`: declarado em docs/design/92 §4 (consumidor: Partners), mas `docs/design/13` não define o efeito; sem comando local, não é consumido nesta versão **[ABERTA: docs/design/92-schemas-eventos.md §4 × docs/design/13 — efeito do consumo; a métrica "volume e conversão por produto" do PRD é o único indício]**.
  - Consultas gRPC de saída, feitas no provider antes da transação: Identity `FindOrganization` (em `RegisterPartner`); Effects (efeitos vigentes pendentes de liberação, em `ChangePartnerStage` para `inactive`) **[ABERTA: docs/design/92 §4 não declara Partner consumidor de eventos de Effects]**.

## Borda e persistência

- **Borda**: só gRPC, `company.partner.service.v1.PartnerService`, um RPC por comando (`RegisterPartner`, `ChangePartnerStage`, `RegisterAgreement`, `ActivateAgreement`, `DeclareCapability`, `RecordHomologation`, `EnableCapability`, `SuspendCapability`, `ChangeRollout`, `DefineProduct`, `RetireProduct`, `RegisterIntegrationProfile`, `RaiseCertificateExpiryAlert`, `DefineOperationalPolicy`, `DefineCommercialRule`) e por consulta, atrás da cadeia de interceptors do kernel. A administração por backoffice e a Partner API são REST do `bff`.
- **Banco**: database e role `partner`, schema `public`, tabelas `partners`, `partner_agreements`, `capabilities`, `financial_products`, `integration_profiles`, `operational_policies` e `commercial_rules`, com `tenant_id` à frente da chave. Persistência híbrida, com colunas tipadas só para o que uma consulta filtra:
  - `partners`: `organization_id`, `role`, `status`
  - `partner_agreements`: `partner_id`, `status`
  - `capabilities`: `partner_id`, `operation`, `asset_type`, `environment`, `status` (`FindEffectiveCapability`, `FindEligiblePartners`)
  - `financial_products`: `partner_id`, `status`, `valid_until`
  - `integration_profiles`: `partner_id`, `environment`, `certificate_expires_at` (`FindCertificatesExpiring`)
  - `operational_policies` e `commercial_rules`: `partner_id`
  - o resto de cada agregado vai em `snapshot` `jsonb`. As versões anteriores permanecem como linhas.
- Diverge do design: o isolamento por tenant é `tenant_id` injetado pelo `ExecutionContext` no `postgres.Table` (ADR-051 DMPF), não RLS como mecanismo primário (`docs/design/00` §4.1); `tenantId` não é campo do agregado.
- Diverge do design: eventos são Protobuf com CloudEvents montado pelo relay; o envelope JSON e o `schemaUri` do doc 92 §1 não se aplicam.
- Diverge do design: `instant` é inteiro de nanossegundos no domínio.
- A configuração de parceiros é da plataforma e lida por todos os tenants; o rollout por tenant a avalia pelo `tenantId` do chamador **[ABERTA: requer extensão do kernel — ADR-051 DMPF]**.
- Segredos são referenciados por `secretRef`; nenhum valor entra no domínio, em evento ou em log.

## Políticas transversais

- **Idempotência**: os comandos criadores (`RegisterPartner`, `RegisterAgreement`, `DeclareCapability`, `DefineProduct`, `RegisterIntegrationProfile`, `DefineOperationalPolicy`, `DefineCommercialRule`) entram pelo `bff` por `POST` com chave de idempotência declarada, que segue na metadata gRPC só para o log; os demais são idempotentes por identidade do agregado, e `RaiseCertificateExpiryAlert` por limiar (RST-02; FND-04; ADR-056). O consumo de evento é idempotente pela inbox, por `eventId` (ADR-036).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2) **[ABERTA: docs/design/95-personas-jornadas.md — a matriz de papéis não cobre a administração de parceiros]**.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); toda mudança material de parceiro vai ao Ledger pelos eventos e fica reproduzível pelas versões.
- **Negar em caso de dúvida**: quando a capacidade efetiva não puder ser confirmada, a operação não prossegue, e toda negação registra a condição que falhou.
- **Suspensão e rollout**: reduzir exposição tem efeito em menos de 30 s e nunca interrompe operação em curso nem o acompanhamento de comando já enviado.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `partner` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários de aceite por comando:

```text
DADO uma organização com kind funder em Identity
QUANDO RegisterPartner chega com role funder
ENTÃO o parceiro nasce prospect, com registeredAt = at, e PartnerRegistered vai à outbox

DADO um parceiro homologating com acordo ativo e uma capacidade enabled
QUANDO ChangePartnerStage chega com target active
ENTÃO o parceiro vai a active, com stageChangedAt = at, e nada é emitido

DADO um parceiro active
QUANDO ChangePartnerStage chega com target offboarding e reason
ENTÃO o parceiro vai a offboarding e PartnerOffboardingStarted é emitido

DADO um parceiro registrado
QUANDO RegisterAgreement chega com validFrom e validUntil coerentes
ENTÃO o acordo nasce draft, version 1, e nada é emitido

DADO um acordo draft
QUANDO ActivateAgreement chega com contractEvidenceId e os critérios H14, H15, H17 e H18
ENTÃO o acordo vai a active e PartnerAgreementActivated é emitido

DADO um parceiro
QUANDO DeclareCapability chega com operação de escrita, stateQuerySupported falso e restrictionNote
ENTÃO a capacidade nasce declared, com rollout disabled, e nada é emitido

DADO uma capacidade declared de escrita com consulta de estado
QUANDO RecordHomologation chega com H1 a H8 e H10 a H12 todos passed
ENTÃO a capacidade vai a homologated

DADO uma capacidade homologated, acordo ativo e perfil de integração vigente
QUANDO EnableCapability chega
ENTÃO a capacidade vai a enabled e CapabilityEnabled é emitido

DADO uma capacidade enabled
QUANDO SuspendCapability chega com reason e trigger manual
ENTÃO a capacidade vai a suspended e CapabilitySuspended é emitido

DADO uma capacidade suspended
QUANDO EnableCapability chega
ENTÃO a capacidade volta a enabled sem nova homologação e CapabilityEnabled é emitido

DADO uma capacidade enabled com rollout enabled
QUANDO ChangeRollout chega com mode percentage, percentage 10 e excludedTenantIds
ENTÃO o rollout é substituído, os excluídos prevalecem sobre qualquer modo e nada é emitido

DADO um acordo ativo
QUANDO DefineProduct chega com faixas coerentes
ENTÃO o produto nasce active, version 1, e PartnerProductChanged é emitido com changeKind defined

DADO um produto active com validUntil menor ou igual a at
QUANDO RetireProduct chega com reason expired
ENTÃO o produto vai a retired e PartnerProductChanged é emitido com changeKind expired

DADO um parceiro
QUANDO RegisterIntegrationProfile chega com endpoint, secretRef e certificado futuro
ENTÃO o perfil nasce na version 1, sem o valor do segredo, e nada é emitido

DADO um perfil cujo certificado vence em 25 dias
QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 30
ENTÃO lastAlertedThresholdDays vira 30 e PartnerCertificateExpiring é emitido

DADO um parceiro
QUANDO DefineOperationalPolicy chega com rateLimitPerMinute 100 e os dois contatos
ENTÃO a política nasce na version 1 e nada é emitido

DADO um parceiro e o modelo de monetização decidido
QUANDO DefineCommercialRule chega com sponsoredPlacementAllowed falso e maxSponsoredPositions 0
ENTÃO a regra nasce com disclosureRequired verdadeiro e nada é emitido

DADO um evento ExternalSchemaMismatchDetected de uma operação com capacidade enabled
QUANDO SuspendCapability chega com trigger schema_mismatch
ENTÃO a capacidade vai a suspended e CapabilitySuspended é emitido

DADO uma capacidade suspended e uma enabled do mesmo parceiro
QUANDO FindEffectiveCapability consulta a suspensa
ENTÃO devolve negado com a condição que falhou, fora de qualquer UoW
```

- [ ] Um cenário por código de rejeição (nada é escrito e o estado não muda):

```text
DADO nenhuma organização com o id informado QUANDO RegisterPartner chega ENTÃO rejeita com partner-management/partner/organization-not-found
DADO uma organização kind client QUANDO RegisterPartner chega com role funder ENTÃO rejeita com partner-management/partner/organization-kind-mismatch
DADO um parceiro prospect QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/invalid-stage-transition
DADO um parceiro active QUANDO ChangePartnerStage chega com target suspended e reason vazio ENTÃO rejeita com partner-management/partner/reason-empty
DADO um parceiro homologating sem acordo ativo QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/agreement-not-active
DADO um parceiro homologating sem capacidade enabled QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/no-enabled-capability
DADO um parceiro offboarding com efeito vigente pendente de liberação QUANDO ChangePartnerStage chega com target inactive ENTÃO rejeita com partner-management/partner/effects-pending-release
DADO qualquer parceiro QUANDO RegisterAgreement chega com validUntil anterior a validFrom ENTÃO rejeita com partner-management/partner-agreement/validity-invalid
DADO um acordo active QUANDO ActivateAgreement chega ENTÃO rejeita com partner-management/partner-agreement/not-draft
DADO um acordo draft QUANDO ActivateAgreement chega sem contractEvidenceId ENTÃO rejeita com partner-management/partner-agreement/contract-evidence-missing
DADO um acordo draft QUANDO ActivateAgreement chega sem H17 em satisfiedCriteria ENTÃO rejeita com partner-management/partner-agreement/contractual-criteria-incomplete
DADO qualquer parceiro QUANDO DeclareCapability chega com operation vazia ENTÃO rejeita com partner-management/capability/operation-empty
DADO qualquer parceiro QUANDO DeclareCapability chega com assetType vazio ENTÃO rejeita com partner-management/capability/asset-type-empty
DADO qualquer parceiro QUANDO DeclareCapability de escrita chega com stateQuerySupported falso e sem restrictionNote ENTÃO rejeita com partner-management/capability/state-query-restriction-undeclared
DADO uma capacidade declared QUANDO RecordHomologation chega com results vazio ENTÃO rejeita com partner-management/capability/criteria-empty
DADO uma capacidade declared QUANDO RecordHomologation chega com criterionId H13 ENTÃO rejeita com partner-management/capability/unknown-criterion
DADO uma capacidade homologated QUANDO RecordHomologation chega ENTÃO rejeita com partner-management/capability/homologation-closed
DADO uma capacidade declared QUANDO EnableCapability chega ENTÃO rejeita com partner-management/capability/not-homologated
DADO uma capacidade homologated sem acordo ativo QUANDO EnableCapability chega ENTÃO rejeita com partner-management/capability/agreement-not-active
DADO uma capacidade homologated sem perfil de integração para o ambiente QUANDO EnableCapability chega ENTÃO rejeita com partner-management/capability/integration-profile-missing
DADO uma capacidade declared QUANDO SuspendCapability chega ENTÃO rejeita com partner-management/capability/not-enabled
DADO uma capacidade enabled QUANDO SuspendCapability chega com reason vazio ENTÃO rejeita com partner-management/capability/reason-empty
DADO uma capacidade enabled QUANDO ChangeRollout chega com mode percentage e percentage 101 ENTÃO rejeita com partner-management/capability/percentage-out-of-range
DADO uma capacidade enabled QUANDO ChangeRollout chega com mode allowlist e allowlistTenantIds vazia ENTÃO rejeita com partner-management/capability/allowlist-empty
DADO um acordo ativo QUANDO DefineProduct chega com assetTypes vazio ENTÃO rejeita com partner-management/financial-product/asset-types-empty
DADO um acordo ativo QUANDO DefineProduct chega com minAmountMinor maior que maxAmountMinor ENTÃO rejeita com partner-management/financial-product/range-invalid
DADO um acordo ativo QUANDO DefineProduct chega com validUntil anterior a validFrom ENTÃO rejeita com partner-management/financial-product/validity-invalid
DADO um parceiro sem acordo ativo QUANDO DefineProduct chega ENTÃO rejeita com partner-management/financial-product/agreement-not-active
DADO um produto retired QUANDO RetireProduct chega ENTÃO rejeita com partner-management/financial-product/not-active
DADO um produto active dentro da vigência QUANDO RetireProduct chega com reason expired ENTÃO rejeita com partner-management/financial-product/not-yet-expired
DADO qualquer parceiro QUANDO RegisterIntegrationProfile chega com endpoint vazio ENTÃO rejeita com partner-management/integration-profile/endpoint-empty
DADO qualquer parceiro QUANDO RegisterIntegrationProfile chega sem secretRef ENTÃO rejeita com partner-management/integration-profile/secret-ref-missing
DADO qualquer parceiro QUANDO RegisterIntegrationProfile chega com certificateExpiresAt anterior a at ENTÃO rejeita com partner-management/integration-profile/certificate-expiry-invalid
DADO um perfil QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 45 ENTÃO rejeita com partner-management/integration-profile/invalid-alert-threshold
DADO um perfil cujo certificado vence em 90 dias QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 60 ENTÃO rejeita com partner-management/integration-profile/threshold-not-reached
DADO um perfil já alertado em 30 dias QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 60 ENTÃO rejeita com partner-management/integration-profile/already-alerted
DADO qualquer parceiro QUANDO DefineOperationalPolicy chega com rateLimitPerMinute 0 ENTÃO rejeita com partner-management/operational-policy/rate-limit-invalid
DADO qualquer parceiro QUANDO DefineOperationalPolicy chega sem incidentContact ENTÃO rejeita com partner-management/operational-policy/contacts-missing
DADO qualquer parceiro QUANDO DefineCommercialRule chega com effectiveUntil anterior a effectiveFrom ENTÃO rejeita com partner-management/commercial-rule/validity-invalid
DADO qualquer parceiro QUANDO DefineCommercialRule chega com sponsoredPlacementAllowed falso e maxSponsoredPositions 2 ENTÃO rejeita com partner-management/commercial-rule/sponsored-positions-invalid
DADO qualquer parceiro QUANDO DefineCommercialRule chega com remunerationModel per_operation e rates vazio ENTÃO rejeita com partner-management/commercial-rule/rates-invalid
```

## Escopo fora

- **REST do `bff`**: a administração de parceiros e a Partner API (`docs/design/91-contratos-api.md` §4, que descreve o convite, a oferta e a confirmação de desembolso do financiador) são do `bff`; este contexto serve só gRPC.
- **Adaptadores e `CapabilityProfile`**: o Receivables Gateway aplica as configurações e seleciona adaptador e versão; este contexto só declara a capacidade e os dados de integração.
- **Elegibilidade e ranking**: Eligibility e Marketplace consomem produtos e capacidades; a ordenação, o patrocínio no ranking e o `PricingBreakdown` não são deste contexto.
- **Modelo de monetização, cobrança e tributação**: Billing (`SPEC-J9JWBT9W`); aqui só a `CommercialRule`, travada no #7.
- **Cofre de segredos e emissão de certificado**: o contexto guarda a referência (`secretRef`, `certificateRef`), não o valor nem o ciclo de vida do segredo.
- **Procedimento humano de offboarding**: inventário de operações em curso, tratamento caso a caso e cláusula de acesso residual são do Backoffice e do contrato de saída (13:N11); o contexto garante só que `inactive` não ocorre com efeito vigente pendente.
- **Suspensão por produto, por tipo de ativo e de parceiro inteiro como comandos próprios**: ficam como fan-out por capacidade até a decisão em Agregados.
- **Acesso cross-tenant à configuração**: depende da extensão do kernel (ADR-051).
- **Armazenamento de evidência de homologação e de contrato**: **[ABERTA: provider de object storage]**.
- **Composition root**: o esqueleto de `app/` e o `cmd/` com `--role` nascem do generator; o harness escreve o conteúdo de domínio dentro deles.
