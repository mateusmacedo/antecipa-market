---
id: SPEC-9TEQ9EV9
slug: partner-management
title: Bounded context — partner-management
stage: planning
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
  - *Regra comercial* (`CommercialRule`) — remuneração e impacto permitido na apresentação; nunca altera o valor da oferta (fora desta versão; ver Escopo fora).
  - *Capacidade efetiva* — resolução de parceiro `active`, contrato vigente, capacidade habilitada e homologada, produto vigente, ausência de suspensão, rollout e parâmetros compatíveis.
  - *Rollout* — exposição gradual de uma capacidade por tenant.
  - *Suspensão* — bloqueia comandos novos e nunca o acompanhamento dos existentes.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/13-prd-partner-management.md`; `docs/design/13-design-partner-management.md` §2 (homologação), §3 (regra comercial), §4 (capacidade efetiva), §5 (suspensão), §6 (rollout), §7 (mudança de contrato de integração), §8 (descontinuidade); `docs/design/04-design-receivables-gateway.md` §2 (`CapabilityProfile`); `docs/design/02-design-kyb-kyc.md` §4; `docs/design/92-schemas-eventos.md` §4; `docs/prd/14-catalogo-contratos-eventos.md`; `docs/adr/013-multi-tenancy-por-execution-context.md`, `docs/adr/015-kafka-como-broker.md`, `docs/adr/020-expurgo-do-inbox.md` e `docs/adr/022-precedencia-dmpf-sobre-design.md` (linhas 2, 3, 5, 6, 9, 12 e 15); ADR-063 do DMPF (role migradora e role de runtime); `docs/specs/SPEC-N1X148AW-receivables-gateway.md:62` (enum de ambiente).
- **Situação**: `parcial` — o núcleo é construível no kernel `1.0.0-rc.5`: cadastro, estágios, acordo, capacidade, homologação, suspensão, rollout, produto, perfil de integração e política operacional. Ficam no Escopo fora: `CommercialRule`, consumo de eventos, `inactive`, cliente gRPC real do Identity e disparo agendado.
- **Fase de entrega**: 1; primeira fatia em tenant único da plataforma.
- **Dependências**: `depends_on` declara só a fundação (`SPEC-GXGASP24`) e a aresta dura reduzida (`SPEC-7KW0HHF6`, Identity: a organização do parceiro); a cadeia transitiva fica implícita. Brandas: nenhuma nesta versão; o consumo de `ExternalSchemaMismatchDetected` (Receivables Gateway) e de `FundingRequestOpened` (Marketplace) está no Escopo fora. A retroalimentação — Gateway, Eligibility e Marketplace consomem os eventos de capacidade deste contexto — nunca entra em `depends_on`. Extensões do kernel que tocam este contexto, nenhuma bloqueando a construção desta versão:
  - SPEC-QVDGV6J0 (DEVS-100) - leitura de plataforma e projeção entre tenants (K4);
  - SPEC-CWBA4BPV (DEVS-95) - object storage das evidências;
  - SPEC-7SRQ3RPK (DEVS-103) - violação de unicidade traduzida em rejeição;
  - SPEC-QPRR24C4 (DEVS-94) - role migradora separada da role de runtime, entregue no `1.0.0-rc.5` (ADR-063).
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| #1 / A1 | autorização regulatória da plataforma (transversal) | existencial para o contexto inteiro |
| #7, 13:N9 | modelo de monetização: quem paga, base de cálculo, fato gerador (`docs/design/18` G1–G3) | `CommercialRule`, fora desta versão (Escopo fora) |
| B17, 13:N8 | ambiente de homologação e critérios acordáveis com cada parceiro | H1 a H12 aplicados a todo parceiro; critérios por parceiro fora desta versão |
| 13:N10 | situação regulatória a verificar por tipo de parceiro | H16 declarado pelo operador em `ActivateAgreement` |
| 13:N11 | cláusula de acesso residual no contrato de saída | `inactive` fora desta versão |
| K4 (SPEC-QVDGV6J0, DEVS-100) | leitura e projeção entre tenants | consultas servidas a outros tenants; nesta versão, a configuração vive num tenant único da plataforma |
| Effects (SPEC-ZFDEPFWC) | consulta de efeitos vigentes por parceiro, ausente da spec de Effects | pré-condição de `inactive`, fora desta versão |

- **Decisões registradas nesta spec (2026-10-17)**:
  1. *Idempotência*: todo comando exige chave de idempotência na metadata gRPC (FND-04; ADR-056 do DMPF); o replay com a mesma chave devolve a resposta registrada.
  2. *Tenant único*: nesta versão, a configuração de parceiros vive num tenant único da plataforma, e o `tenantId` dos filtros é o tenant avaliado no rollout; leitura entre tenants espera o K4 (SPEC-QVDGV6J0).
  3. *Enum de ambiente*: `environment` é `development | integration | homologation | production`, igual ao Gateway (SPEC-N1X148AW:62), em `Capability`, `IntegrationProfile`, eventos e consultas.
  4. *Eventos de produto por fato*: `PartnerProductChanged` com `changeKind` dá lugar a `PartnerProductDefined`, `PartnerProductChanged`, `PartnerProductExpired` e `PartnerProductRetired` (`docs/design/92-schemas-eventos.md` §5.3).
  5. *Rollout notificado e determinístico*: `ChangeRollout` emite `CapabilityRolloutChanged`; o bucket do tenant vem de sha256 pela porta da Nota (§); a dimensão de produto do rollout está no Escopo fora.
  6. *Re-homologação*: a retomada de capacidade suspensa com `trigger = schema_mismatch` exige nova homologação (`docs/design/13` §7); os demais triggers retomam direto por `EnableCapability`.
  7. *Retorno de `suspended` a `active`*: o parceiro volta a `active` exigindo só acordo ativo e vigente; o retorno não altera as capacidades, e as `suspended` são reabilitadas uma a uma por `EnableCapability`; a capacidade efetiva passa a exigir `Partner.status = active`.
  8. *Unicidade do parceiro*: `(organizationId, role)` é único no tenant para qualquer status; papéis distintos da mesma organização são permitidos.
  9. *H17 declarativo*: H17 é declaração do operador em `satisfiedCriteria`; a verificação contra a regra comercial depende do #7 e está no Escopo fora.
  10. *`requiredCheckIds` no produto*: a lista fica no `FinancialProduct`, que pertence a parceiro e acordo e cumpre o "por produto e parceiro" de `docs/design/02` §4; o KYB a lê pela consulta de produto.
  11. *Sem fan-out*: `ChangePartnerStage` muda só o `Partner`; suspender o parceiro ou iniciar o offboarding não altera as capacidades, e a capacidade efetiva nega com `Partner.status ≠ active`.
  12. *Sem papel de job*: `RaiseCertificateExpiryAlert` e `RetireProduct(expired)` são comandos RPC idempotentes com chaves determinísticas `cert-alert.<profileId>.<thresholdDays>.<yyyymmdd>` e `product-expired.<productId>.<yyyymmdd>`, com o dia UTC da avaliação, para que a repetição no dia seguinte decida de novo sobre o estado; ids de perfil e de produto seguem `^[A-Za-z0-9._-]{1,100}$`, para que a chave caiba no formato da chave de idempotência; o disparo agendado está no Escopo fora.
  13. *Diretório de organizações fail-closed*: a porta `OrganizationDirectory` tem, em produção, provider que falha fechado; o fake vale só em testes (Nota (‡)).
  14. *Role migradora*: o banco do contexto pertence à role migradora `partner_migrator`, e a role de runtime `partner` tem só os privilégios declarados por tabela (ADR-063 do DMPF; ADR-022, linha 5).
  15. *Versão vigente*: no `PartnerAgreement`, é a de maior `version` em `active` com `validFrom` até o instante avaliado; na `Capability`, a de maior `version` entre `enabled` e `suspended`, e a capacidade efetiva nega quando ela está `suspended`; nos demais agregados versionados, a de maior `version` com `validFrom` ou `effectiveFrom` até o instante avaliado.
  16. *Série de versões*: nos cinco criadores com versão, `supersedesId` aponta a última versão da série, e sem ele a série não pode ter v1; a violação rejeita com `<agregado>/supersedes-invalid` (Nota (††)).
  17. *`callbackApplicable` persistido*: a aplicabilidade do callback (H9) é declarada em `DeclareCapability` e guardada na capacidade.
  18. *Escopo desta versão*: o núcleo implementável sem decisão de produto aberta; o restante está em Escopo fora.
- **Decisões de produto abertas**: fronteira entre `OperationalPolicy` e `IntegrationProfile` deste contexto e o `CapabilityProfile` do Gateway **[ABERTA: docs/design/13-design-partner-management.md cabeçalho e docs/design/04-design-receivables-gateway.md §2.1 — qual contexto é a fonte dos limites e do calendário]**; regressão `homologating → integrating` e abandono do processo de entrada (`docs/prd/13-prd-partner-management.md:42` mostra só a sequência de estados); modelo do instrumento contratual referenciado por `instrumentTemplateId` **[ABERTA: docs/design/08-design-contracting.md §2 — o modelo híbrido é recomendação, a decisão de produto está aberta]**; dono da suspensão por quebra de schema (`docs/design/04-design-receivables-gateway.md:166-173` suspende a operação no Gateway; `docs/design/13-design-partner-management.md:178` prevê a suspensão automática neste contexto).
- **Condição para sair do backlog** (cumprida): `SPEC-GXGASP24` e `SPEC-7KW0HHF6` em `done`; contradições normativas resolvidas em 2026-10-17.

## Agregados

Nota (†): pré-condição sobre outro agregado é lida no `Run` do caso de uso por `Load` de outro repositório da mesma UoW, antes de `usecase.Decide`; `ErrNotFound` vira a rejeição declarada. Nesta versão, a pré-condição relacional sem id (acordo ativo e vigente, perfil vigente para o ambiente, capacidade habilitada) é lida pela `Relation` do kernel, que no rc.5 só opera sobre o pool de leitura, fora da UoW; a corrida com um comando concorrente sobre o outro agregado é aceita até o kernel oferecer leitura relacional na transação (DEVS-132).

Nota (††): unicidade e série de versões são verificadas por pré-leitura em autocommit, separada da UoW do comando, com índice único de salvaguarda e `tenant_id` à frente (ADR-013). Numa corrida, o índice decide: exatamente uma requisição grava, e a perdedora sai como erro técnico, sem linha nem evento (ADR-022, linha 15, até a SPEC-7SRQ3RPK).

Nota (‡): `FindOrganization` é chamada pela porta `OrganizationDirectory`, no `Run` do caso de uso, antes de `usecase.Decide`. Em produção, o provider falha fechado com erro técnico ("diretório de organizações não configurado"); o fake existe só em testes. O provider gRPC real está fora desta versão e reavalia a chamada antes da transação.

Nota (§): o bucket de rollout vem da porta `RolloutBucket`, realizada no `provider`, porque o `depguard` proíbe `crypto/*` em `domain` e `application`.

Os agregados versionados (`FinancialProduct`, `IntegrationProfile`, `OperationalPolicy`, `Capability`, `PartnerAgreement`) ganham uma linha por versão, com `version` e `supersedesId`; o comando criador é um só e cobre a primeira versão e as seguintes ("Alterações de parceiro são reproduzíveis historicamente"). `RolloutPolicy` é parte de `Capability`. Cada agregado versionado tem sua série:

- `partnerId`: `PartnerAgreement` e `OperationalPolicy`;
- `partnerId+environment`: `IntegrationProfile`;
- `partnerId+code`: `FinancialProduct`;
- `partnerId+operation+assetType+environment`: `Capability`.

`supersedesId`, quando informado, aponta a última versão da série; sem ele, a série não pode ter v1. A violação rejeita com `<agregado>/supersedes-invalid` (††). A versão vigente é, no `PartnerAgreement`, a de maior `version` em `active` com `validFrom` até o instante avaliado; na `Capability`, a de maior `version` entre `enabled` e `suspended` (se for a `suspended`, a capacidade efetiva nega); nos demais, a de maior `version` com `validFrom` ou `effectiveFrom` até o instante avaliado.

### Partner

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `role: funder | bookkeeper | registry | depository | data_provider | settlement_institution`
  - `status: prospect | contracting | integrating | homologating | active | suspended | offboarding`
  - `registeredAt: instant`, `stageChangedAt: instant`
  - `stageReason: string` (opcional), `offboardingStartedAt: instant` (opcional)
- **Estados**: os sete de `status`.
- **Transições permitidas** (por `ChangePartnerStage`, salvo a criação):
  - (novo) → `prospect` — por `RegisterPartner`
  - `prospect` → `contracting` → `integrating` → `homologating` → `active`, um estágio por vez
  - `active` → `suspended`; `suspended` → `active`
  - `active` | `suspended` → `offboarding`
- **Invariantes de estado**:
  - Um estágio fora da tabela rejeita com `partner-management/partner/invalid-stage-transition`.
  - `(organizationId, role)` é único no tenant para qualquer status (††) → `partner-management/partner/already-registered`; papéis distintos da mesma organização são permitidos.
  - `active` a partir de `homologating` exige acordo ativo e vigente e ao menos uma capacidade `enabled`; a partir de `suspended`, só acordo ativo e vigente; o retorno não altera as capacidades, e as que estiverem `suspended` são reabilitadas uma a uma por `EnableCapability`.
  - Suspender o parceiro ou iniciar o offboarding não altera as capacidades; a capacidade efetiva nega com `Partner.status ≠ active`.
  - Suspensão não apaga operação nem evidência existente.

### PartnerAgreement

- **Identidade**: `generated`; versionado
- **Campos**: `partnerId: id`, `version: int`, `supersedesId: id` (opcional); `status: draft | active`; `validFrom: instant`, `validUntil: instant` (opcional; ausente = indeterminado); `contractEvidenceId: id` (opcional até a ativação) - id opaco da evidência; os bytes vão para a porta de object storage da SPEC-CWBA4BPV (DEVS-95), fora deste agregado; `instrumentTemplateId: id` (opcional) - id opaco do modelo do instrumento contratual (`docs/design/08` §2; o modelo está nas Decisões de produto abertas); `activatedAt: instant` (opcional)
- **Estados**: `draft`, `active`
- **Transições permitidas**:
  - (novo) → `draft` — por `RegisterAgreement`
  - `draft` → `active` — por `ActivateAgreement`
- **Invariantes de estado**:
  - `validUntil`, quando presente, é posterior a `validFrom` — rejeição `partner-management/partner-agreement/validity-invalid`.
  - `supersedesId`, quando informado, aponta a última versão da série do parceiro; sem ele, a série não tem v1 (††) — rejeição `partner-management/partner-agreement/supersedes-invalid`.
  - A vigência no instante é avaliada pela data, não por estado: acordo `active` fora da vigência não sustenta capacidade efetiva.

### Capability

- **Identidade**: `generated`; versionada
- **Campos**:
  - `partnerId: id`, `operation: string`, `assetType: string`, `environment: development | integration | homologation | production`
  - `version: int`, `supersedesId: id` (opcional)
  - `isWrite: bool`, `stateQuerySupported: bool`, `restrictionNote: string` (opcional), `callbackApplicable: bool`
  - `status: declared | homologated | enabled | suspended`
  - `homologation: list<CriterionResult>` — `criterionId: string` (`H1` a `H12`), `passed: bool`, `evidenceId: id` (opcional), `recordedAt: instant`; coleção indispensável: a transição para `homologated` depende do conjunto
  - `rollout`: `mode: disabled | allowlist | percentage | enabled`, `allowlistTenantIds: list<id>`, `percentage: int`, `excludedTenantIds: list<id>` — listas por definição do rollout
  - `suspension` (opcional): `reason: string`, `trigger: manual | schema_mismatch | offboarding`, `suspendedAt: instant`
  - `enabledAt: instant` (opcional)
- **Estados**: `declared`, `homologated`, `enabled`, `suspended`
- **Transições permitidas**:
  - (novo) → `declared` — por `DeclareCapability`
  - `declared` → `homologated` — por `RecordHomologation`, quando todos os critérios aplicáveis passaram; também `suspended` com `trigger = schema_mismatch` → `homologated` por `RecordHomologation`, contando só resultados com `recordedAt > suspension.suspendedAt`
  - `homologated` → `enabled` — por `EnableCapability`; `suspended` → `enabled` só com `trigger` `manual` ou `offboarding`
  - `enabled` → `suspended` — por `SuspendCapability`
- **Invariantes de estado**:
  - Capacidade ativa exige contrato, homologação e configuração válidos — `not-homologated`, `agreement-not-active`, `integration-profile-missing` e `rehomologation-required` em `EnableCapability`.
  - Critérios aplicáveis: H1, H2, H4 a H8 e H10 a H12 sempre; H3 (consulta de estado) em operação de escrita com `stateQuerySupported`; H9 (callback) quando `callbackApplicable`. Operação de escrita sem consulta de estado só é homologada com `restrictionNote` declarada — toda indefinição vira caso humano, por decisão consciente (`docs/design/13` §2.1).
  - O rollout é determinístico: o bucket do tenant são os 8 primeiros bytes de `sha256(capabilityId ":" tenantId)`, lidos como inteiro sem sinal big-endian, mod 100, calculado pela porta (§); em `percentage`, o tenant está incluído quando o bucket é menor que `percentage`, e por isso permanece do mesmo lado entre avaliações; `excludedTenantIds` prevalece sobre qualquer modo; reduzir o rollout impede operações novas e não interrompe as em curso; toda mudança de rollout é notificada por `CapabilityRolloutChanged`.
  - Suspensão bloqueia comandos novos e nunca o acompanhamento dos existentes; efeito vigente permanece e a liberação continua possível.

### FinancialProduct

- **Identidade**: `generated`; versionado
- **Campos**: `partnerId: id`, `agreementId: id`, `code: string`, `name: string`; `version: int`, `supersedesId: id` (opcional); `status: active | retired`; `validFrom: instant`, `validUntil: instant` (opcional); `assetTypes: list<string>`, `regions: list<string>` — faixas atendidas; `currency: string`, `minAmountMinor: int`, `maxAmountMinor: int`; `minTermDays: int`, `maxTermDays: int` — prazo em dias (`docs/design/06-design-eligibility.md:121` e `:187`; `docs/design/07-design-marketplace.md:58`); `requiredCheckIds: list<string>` — verificações `CHK-*` que o produto exige no KYB (`docs/design/02` §4). A lista fica no produto, que pertence a parceiro e acordo; isso diverge de `docs/design/02` §4 e da SPEC-R25V2A8C:86, que a põem no `PartnerAgreement`, e o KYB a lê pela consulta de produto
- **Estados**: `active`, `retired`
- **Transições permitidas**:
  - (novo) → `active` — por `DefineProduct`
  - `active` → `retired` — por `RetireProduct`
- **Invariantes de estado**: produto `retired` ou fora da vigência não recebe novos pedidos; `min` não excede `max` — `partner-management/financial-product/range-invalid`.

### IntegrationProfile

- **Identidade**: `generated`; versionado por ambiente
- **Campos**: `partnerId: id`, `environment: development | integration | homologation | production`, `version: int`, `supersedesId: id` (opcional); `changeKind: initial | announced | unannounced`; `protocols: list<string>`, `endpoint: string`, `schemaVersion: string`; `secretRef: string` — referência ao segredo; o valor nunca entra no domínio; `certificateRef: string`, `certificateExpiresAt: instant`; `effectiveFrom: instant`; `lastAlertedThresholdDays: int` (opcional)
- **Estados**: nenhum
- **Transições permitidas**: nenhuma; mudança de endpoint, certificado ou schema registra versão nova.
- **Invariantes de estado**: segredos são referenciados, nunca expostos; alerta de vencimento nos limiares de 60, 30 e 7 dias, sem repetir o mesmo limiar; `supersedesId`, quando informado, aponta a última versão da série `partnerId+environment`, e sem ele a série não tem v1 (††) — `partner-management/integration-profile/supersedes-invalid`.

### OperationalPolicy

- **Identidade**: `generated`; versionada
- **Campos**: `partnerId: id`, `version: int`, `supersedesId: id` (opcional), `effectiveFrom: instant`; `rateLimitPerMinute: int`; `businessDays: list<string>`, `cutoffTime: string`, `processingWindow: string`; `slaRef: string`, `fallback: string` — texto livre; os valores de contingência estão no Escopo fora; `supportContact: string`, `incidentContact: string` — critérios H11 e H12; contatos são PII
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**: nenhum além da série de versões (††); a fronteira com o `CapabilityProfile` do Gateway está nas Decisões de produto abertas.

## Comandos (UPRs)

(†) marca pré-condição que depende de outro agregado do contexto: o caso de uso a verifica por leitura na mesma UoW antes de decidir, conforme a Nota (†); a organização do parceiro vem pela porta da Nota (‡). Ver Relações entre agregados.

### RegisterPartner

- **Agregado**: `Partner`; **cria**
- **Entrada**: `organizationId: id`; `role`
- **Pré-condições**:
  - organização existente em Identity (‡) → `partner-management/partner/organization-not-found`
  - `kinds` da organização compatíveis com o papel: `funder` para financiador, `partner` para os demais (‡) → `partner-management/partner/organization-kind-mismatch`
  - `(organizationId, role)` ainda não registrado no tenant (††) → `partner-management/partner/already-registered`
- **Efeitos sobre o estado**: `status ← prospect`; `registeredAt ← at`; `stageChangedAt ← at`
- **Eventos emitidos**: `PartnerRegistered`
- **Resposta**: `partnerId: id`

### ChangePartnerStage

- **Agregado**: `Partner`; exige existente
- **Entrada**: `target: contracting | integrating | homologating | active | suspended | offboarding`; `reason: string` (opcional; exigido em `suspended` e `offboarding`)
- **Pré-condições**:
  - a transição está na tabela do agregado → `partner-management/partner/invalid-stage-transition`
  - `reason` não vazio ao suspender ou iniciar o offboarding → `partner-management/partner/reason-empty`
  - para `active` a partir de `homologating` ou `suspended`: acordo ativo e vigente (†) → `partner-management/partner/agreement-not-active`
  - para `active` a partir de `homologating`: ao menos uma capacidade `enabled` (†) → `partner-management/partner/no-enabled-capability`
- **Efeitos sobre o estado**: `status ← target`; `stageChangedAt ← at`; `stageReason ← reason` quando informado; `offboardingStartedAt ← at` ao entrar em `offboarding`
- **Eventos emitidos**: `PartnerOffboardingStarted` (só ao entrar em `offboarding`)
- **Resposta**: `partnerId: id`, `status`

Suspender ou iniciar o offboarding do parceiro não altera as capacidades; a capacidade efetiva nega com `Partner.status ≠ active`. A propagação ao Gateway está no Escopo fora.

### RegisterAgreement

- **Agregado**: `PartnerAgreement`; **cria**
- **Entrada**: `partnerId: id`; `validFrom: instant`, `validUntil: instant` (opcional); `instrumentTemplateId: id` (opcional); `supersedesId: id` (opcional)
- **Pré-condições**:
  - `validUntil`, quando informado, posterior a `validFrom` → `partner-management/partner-agreement/validity-invalid`
  - `supersedesId`, quando informado, aponta a última versão da série do parceiro; sem ele, a série não tem v1 (††) → `partner-management/partner-agreement/supersedes-invalid`
- **Efeitos sobre o estado**: `status ← draft`; `version ← 1`, ou a do acordo substituído mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `agreementId: id`

### ActivateAgreement

- **Agregado**: `PartnerAgreement`; exige existente
- **Entrada**: `contractEvidenceId: id`; `satisfiedCriteria: list<string>` — critérios contratuais `H14` a `H18` atendidos; nesta versão, H17 é declaração do operador; `regulatoryCheckApplicable: bool`
- **Pré-condições**:
  - `status = draft` → `partner-management/partner-agreement/not-draft`
  - `contractEvidenceId` não vazio (H13, contrato assinado e vigente) → `partner-management/partner-agreement/contract-evidence-missing`
  - `satisfiedCriteria` cobre H14, H15, H17 e H18, e H16 declarado quando `regulatoryCheckApplicable` → `partner-management/partner-agreement/contractual-criteria-incomplete`
  - `validUntil`, quando presente, posterior a `at` → `partner-management/partner-agreement/validity-invalid`
- **Efeitos sobre o estado**: `status ← active`; `contractEvidenceId ← cmd`; `activatedAt ← at`
- **Eventos emitidos**: `PartnerAgreementActivated`
- **Resposta**: `agreementId: id`

### DeclareCapability

- **Agregado**: `Capability`; **cria**
- **Entrada**: `partnerId: id`, `operation: string`, `assetType: string`, `environment: development | integration | homologation | production`; `isWrite: bool`, `stateQuerySupported: bool`, `restrictionNote: string` (opcional), `callbackApplicable: bool`; `supersedesId: id` (opcional)
- **Pré-condições**:
  - `operation` não vazia → `partner-management/capability/operation-empty`
  - `assetType` não vazio → `partner-management/capability/asset-type-empty`
  - operação de escrita sem consulta de estado exige `restrictionNote` → `partner-management/capability/state-query-restriction-undeclared`
  - `supersedesId`, quando informado, aponta a última versão da série `partnerId+operation+assetType+environment`; sem ele, a série não tem v1 (††) → `partner-management/capability/supersedes-invalid`
- **Efeitos sobre o estado**: `status ← declared`; `callbackApplicable ← cmd`; `rollout.mode ← disabled`; `version ← 1`, ou a da capacidade substituída mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `capabilityId: id`

### RecordHomologation

- **Agregado**: `Capability`; exige existente
- **Comando**: `RecordHomologation` — pode ser repetido até a homologação completar.
- **Entrada**: `results: list<{criterionId, passed, evidenceId}>` — critérios H1 a H12
- **Pré-condições**:
  - `results` não vazio → `partner-management/capability/criteria-empty`
  - todo `criterionId` entre H1 e H12 → `partner-management/capability/unknown-criterion`
  - `status = declared`, ou `suspended` com `trigger = schema_mismatch` → `partner-management/capability/homologation-closed`
- **Efeitos sobre o estado**: `homologation ←` resultados substituídos por `criterionId`, com `recordedAt ← at`; vindo de `suspended`, contam só os resultados com `recordedAt > suspension.suspendedAt`; `declared` | `suspended` → `homologated` quando todos os critérios aplicáveis têm `passed`; senão permanece no estado de origem
- **Eventos emitidos**: nenhum
- **Resposta**: `capabilityId: id`, `status`

### EnableCapability

- **Agregado**: `Capability`; exige existente
- **Comando**: `EnableCapability` — habilita uma capacidade homologada e também a retoma após suspensão manual ou por offboarding.
- **Entrada**: nenhuma além da identidade do agregado
- **Pré-condições**:
  - `status` em `homologated` ou `suspended` → `partner-management/capability/not-homologated`
  - `suspended` com `trigger = schema_mismatch` → `partner-management/capability/rehomologation-required`
  - acordo ativo e vigente do parceiro (†) → `partner-management/capability/agreement-not-active`
  - perfil de integração vigente para o ambiente (†) → `partner-management/capability/integration-profile-missing`
- **Efeitos sobre o estado**: `status ← enabled`; `enabledAt ← at`; `suspension ← vazio`
- **Eventos emitidos**: `CapabilityEnabled`
- **Resposta**: `capabilityId: id`

### SuspendCapability

- **Agregado**: `Capability`; exige existente
- **Entrada**: `reason: string`; `trigger: manual | schema_mismatch | offboarding` — o RPC `SuspendCapability` só expõe `manual`: a requisição não tem campo `trigger`, e o adaptador fixa `trigger = manual`; `schema_mismatch` e `offboarding` ficam para fluxos internos, no Escopo fora
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
- **Eventos emitidos**: `CapabilityRolloutChanged`
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
  - `supersedesId`, quando informado, aponta a última versão da série `partnerId+code`; sem ele, a série não tem v1 (††) → `partner-management/financial-product/supersedes-invalid`
- **Efeitos sobre o estado**: `status ← active`; `version ← 1`, ou a do produto substituído mais 1
- **Eventos emitidos**: `PartnerProductDefined` na primeira versão; `PartnerProductChanged` nas seguintes
- **Resposta**: `productId: id`

### RetireProduct

- **Agregado**: `FinancialProduct`; exige existente
- **Entrada**: `reason: expired | withdrawn`
- **Pré-condições**:
  - `status = active` → `partner-management/financial-product/not-active`
  - `expired` exige `validUntil` menor ou igual a `at` → `partner-management/financial-product/not-yet-expired`
- **Efeitos sobre o estado**: `status ← retired`
- **Eventos emitidos**: `PartnerProductExpired` (com `reason = expired`) ou `PartnerProductRetired` (com `reason = withdrawn`)
- **Resposta**: `productId: id`

O disparo de `RetireProduct(expired)` vem de chamador externo, com chave `product-expired.<productId>.<yyyymmdd>`; o agendamento está no Escopo fora.

### RegisterIntegrationProfile

- **Agregado**: `IntegrationProfile`; **cria**
- **Comando**: `RegisterIntegrationProfile` — registra o perfil inicial e toda mudança de endpoint, certificado ou schema como versão nova.
- **Entrada**: `partnerId: id`, `environment: development | integration | homologation | production`; `changeKind: initial | announced | unannounced`; `protocols: list<string>`, `endpoint: string`, `schemaVersion: string`; `secretRef: string`, `certificateRef: string`, `certificateExpiresAt: instant`; `effectiveFrom: instant`, `supersedesId: id` (opcional)
- **Pré-condições**:
  - `endpoint` não vazio → `partner-management/integration-profile/endpoint-empty`
  - `secretRef` não vazia → `partner-management/integration-profile/secret-ref-missing`
  - `certificateExpiresAt` posterior a `at` → `partner-management/integration-profile/certificate-expiry-invalid`
  - `supersedesId`, quando informado, aponta a última versão da série `partnerId+environment`; sem ele, a série não tem v1 (††) → `partner-management/integration-profile/supersedes-invalid`
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

A chave de idempotência é determinística, `cert-alert.<profileId>.<thresholdDays>.<yyyymmdd>`; o disparo vem de chamador externo.

### DefineOperationalPolicy

- **Agregado**: `OperationalPolicy`; **cria**
- **Entrada**: `partnerId: id`, `effectiveFrom: instant`, `supersedesId: id` (opcional); `rateLimitPerMinute: int`; `businessDays: list<string>`, `cutoffTime: string`, `processingWindow: string`; `slaRef: string`, `fallback: string`; `supportContact: string`, `incidentContact: string`
- **Pré-condições**:
  - `rateLimitPerMinute` maior que zero → `partner-management/operational-policy/rate-limit-invalid`
  - `supportContact` e `incidentContact` não vazios (H11, H12) → `partner-management/operational-policy/contacts-missing`
  - `supersedesId`, quando informado, aponta a última versão da série `partnerId`; sem ele, a série não tem v1 (††) → `partner-management/operational-policy/supersedes-invalid`
- **Efeitos sobre o estado**: campos ← cmd; `version ← 1`, ou a da política substituída mais 1
- **Eventos emitidos**: nenhum
- **Resposta**: `policyId: id`

## Eventos de domínio

`EventName()` é `partner-management.<agregado>.<evento>`.

### PartnerRegistered

- **Campos**: `partnerId: id ← Partner.id`; `organizationId: id ← Partner.organizationId`; `role ← Partner.role`; `at: instant ← at`

### PartnerAgreementActivated

- **Campos**: `agreementId: id ← PartnerAgreement.id`; `partnerId: id ← PartnerAgreement.partnerId`; `version: int ← PartnerAgreement.version`; `validFrom: instant ← PartnerAgreement.validFrom`; `validUntil: instant ← PartnerAgreement.validUntil`; `at: instant ← at`

### CapabilityEnabled

- **Campos**: `capabilityId: id ← Capability.id`; `partnerId: id ← Capability.partnerId`; `operation: string ← Capability.operation`; `assetType: string ← Capability.assetType`; `environment ← Capability.environment`; `version: int ← Capability.version`; `rolloutMode ← Capability.rollout.mode`; `stateQuerySupported: bool ← Capability.stateQuerySupported`; `restrictionNote: string ← Capability.restrictionNote`; `at: instant ← at`

### CapabilitySuspended

- **Campos**: `capabilityId: id ← Capability.id`; `partnerId: id ← Capability.partnerId`; `operation: string ← Capability.operation`; `assetType: string ← Capability.assetType`; `environment ← Capability.environment`; `version: int ← Capability.version`; `reason: string ← cmd.reason`; `trigger ← cmd.trigger`; `at: instant ← at`

### CapabilityRolloutChanged

- **Campos**: `capabilityId: id ← Capability.id`; `partnerId: id ← Capability.partnerId`; `operation: string ← Capability.operation`; `assetType: string ← Capability.assetType`; `environment ← Capability.environment`; `version: int ← Capability.version`; `mode ← cmd.mode`; `percentage: int ← cmd.percentage`; `at: instant ← at`

### PartnerProductDefined

- **Campos**: `productId: id ← FinancialProduct.id`; `partnerId: id ← FinancialProduct.partnerId`; `agreementId: id ← FinancialProduct.agreementId`; `code: string ← FinancialProduct.code`; `version: int ← FinancialProduct.version`; `validFrom: instant ← FinancialProduct.validFrom`; `validUntil: instant ← FinancialProduct.validUntil`; `assetTypes: list<string> ← FinancialProduct.assetTypes`; `regions: list<string> ← FinancialProduct.regions`; `currency: string ← FinancialProduct.currency`; `minAmountMinor: int ← FinancialProduct.minAmountMinor`; `maxAmountMinor: int ← FinancialProduct.maxAmountMinor`; `minTermDays: int ← FinancialProduct.minTermDays`; `maxTermDays: int ← FinancialProduct.maxTermDays`; `at: instant ← at`

### PartnerProductChanged

- **Campos**: `productId: id ← FinancialProduct.id`; `partnerId: id ← FinancialProduct.partnerId`; `agreementId: id ← FinancialProduct.agreementId`; `code: string ← FinancialProduct.code`; `version: int ← FinancialProduct.version`; `supersedesId: id ← FinancialProduct.supersedesId`; `validFrom: instant ← FinancialProduct.validFrom`; `validUntil: instant ← FinancialProduct.validUntil`; `assetTypes: list<string> ← FinancialProduct.assetTypes`; `regions: list<string> ← FinancialProduct.regions`; `currency: string ← FinancialProduct.currency`; `minAmountMinor: int ← FinancialProduct.minAmountMinor`; `maxAmountMinor: int ← FinancialProduct.maxAmountMinor`; `minTermDays: int ← FinancialProduct.minTermDays`; `maxTermDays: int ← FinancialProduct.maxTermDays`; `at: instant ← at`

### PartnerProductExpired

- **Campos**: `productId: id ← FinancialProduct.id`; `partnerId: id ← FinancialProduct.partnerId`; `version: int ← FinancialProduct.version`; `validUntil: instant ← FinancialProduct.validUntil`; `at: instant ← at`

### PartnerProductRetired

- **Campos**: `productId: id ← FinancialProduct.id`; `partnerId: id ← FinancialProduct.partnerId`; `version: int ← FinancialProduct.version`; `at: instant ← at`

### PartnerCertificateExpiring

- **Campos**: `profileId: id ← IntegrationProfile.id`; `partnerId: id ← IntegrationProfile.partnerId`; `environment ← IntegrationProfile.environment`; `certificateExpiresAt: instant ← IntegrationProfile.certificateExpiresAt`; `thresholdDays: int ← cmd.thresholdDays`; `at: instant ← at`

### PartnerOffboardingStarted

- **Campos**: `partnerId: id ← Partner.id`; `organizationId: id ← Partner.organizationId`; `reason: string ← Partner.stageReason`; `at: instant ← Partner.offboardingStartedAt`

## Consultas

As consultas leem o tenant do portador; nesta versão, a configuração vive num tenant único da plataforma, e o `tenantId` dos filtros é o tenant avaliado no rollout (K4 no Escopo fora).

- `FindPartner` — filtros: `partnerId`; cardinalidade: `one`
- `FindPartnersByOrganization` — filtros: `organizationId`; cardinalidade: `many`
- `FindEligiblePartners` — filtros: `assetType`, `operation`, `environment`, `tenantId`, `amountMinor`, `termDays`, `region`, `asOf`; cardinalidade: `many` — o Marketplace encontra só financiadores elegíveis ao pedido
- `FindEffectiveCapability` — filtros: `partnerId`, `operation`, `assetType`, `environment`, `tenantId`, `asOf` e, opcionais, `productId`, `amountMinor`, `termDays`; cardinalidade: `one` — devolve permitido, ou negado com `failedCondition: capability_not_found | partner_not_active | agreement_not_effective | capability_not_enabled | product_not_effective | not_in_rollout | parameters_incompatible`; a condição de produto só é avaliada com `productId`
- `FindCapabilitiesByPartner` — filtros: `partnerId`, `environment` (opcional), `status` (opcional); cardinalidade: `many`
- `FindProductsByPartner` — filtros: `partnerId`, `asOf`; cardinalidade: `many` — versão vigente de cada produto, pela regra de Agregados
- `FindAgreementsByPartner` — filtros: `partnerId`; cardinalidade: `many`
- `FindIntegrationProfile` — filtros: `partnerId`, `environment`, `asOf`; cardinalidade: `one` — versão vigente, pela regra de Agregados
- `FindOperationalPolicy` — filtros: `partnerId`, `asOf`; cardinalidade: `one`
- `FindCertificatesExpiring` — filtros: `environment`, `expiresBefore`; cardinalidade: `many` — igualdade por `environment`, só a versão vigente de cada série `partnerId+environment`, e refino em memória por `certificateExpiresAt` anterior a `expiresBefore`; o custo linear é aceito em tenant único

A capacidade efetiva é: parceiro `active` E contrato vigente no instante E versão vigente da capacidade em `enabled` E produto vigente (com `productId`) E tenant incluído no rollout E parâmetros compatíveis. A avaliação segue a ordem do enum `failedCondition`. Se a consulta não puder confirmar, o consumidor nega (`docs/design/13` §4).

## Relações entre agregados

- `Partner.organizationId` → `Organization` (contexto `identity-organization`)
- `PartnerAgreement.partnerId` → `Partner`
- `Capability.partnerId` → `Partner`
- `FinancialProduct.partnerId` → `Partner`; `FinancialProduct.agreementId` → `PartnerAgreement`
- `IntegrationProfile.partnerId`, `OperationalPolicy.partnerId` → `Partner`
- `supersedesId` → o próprio agregado, na versão anterior

As relações são só por identidade. A verificação (†) de existência e vigência do agregado referenciado é do caso de uso, por leitura na mesma UoW, conforme a Nota (†); a organização do parceiro vem pela porta da Nota (‡).

## Integração

- **Publica** (integration events), em `apps/backend/partner/contract/proto/company/partner/event/v1/`, package `company.partner.event.v1`, no tópico `partner.events` (DLQ `partner.events.dlq`). Transporte Kafka pela outbox e pelo relay do kernel (ADR-015), em Protobuf dentro de CloudEvents, com `partitionkey` igual ao id do agregado de origem, gravado pelo kernel e afirmado no e2e (ADR-022, linha 6). Critério de TRP-37: o primeiro critério que discrimina é a ordenação exigida (os eventos de um mesmo agregado chegam em ordem aos consumidores), o que leva a Kafka, com ordem por partição (ADR-015:18-20). O critério vive nesta spec; o `deploy/infra.json` declara só os tópicos e as ACLs.
  - `PartnerRegistered` → `partner_registered.proto` — Ledger
  - `PartnerAgreementActivated` → `partner_agreement_activated.proto` — Ledger
  - `CapabilityEnabled` → `capability_enabled.proto` — consumidores (doc 92 §4): Gateway, Eligibility, Marketplace
  - `CapabilitySuspended` → `capability_suspended.proto` — Gateway, Eligibility, Marketplace, Ops
  - `CapabilityRolloutChanged` → `capability_rollout_changed.proto` — consumidores das capacidades (Gateway, Eligibility, Marketplace)
  - `PartnerProductDefined` → `partner_product_defined.proto` — Eligibility, Marketplace
  - `PartnerProductChanged` → `partner_product_changed.proto` — Eligibility, Marketplace
  - `PartnerProductExpired` → `partner_product_expired.proto` — Eligibility, Marketplace
  - `PartnerProductRetired` → `partner_product_retired.proto` — Eligibility, Marketplace
  - `PartnerCertificateExpiring` → `partner_certificate_expiring.proto` — Ops
  - `PartnerOffboardingStarted` → `partner_offboarding_started.proto` — Ledger
  - `PartnerRegistered`, `PartnerAgreementActivated` e `PartnerOffboardingStarted` vêm do PRD 13 (`docs/prd/13-prd-partner-management.md:54`) e faltam no doc 92 §4 e no catálogo (`docs/prd/14-catalogo-contratos-eventos.md:56`), como os eventos novos desta spec; o addendum do catálogo fica para uma issue de documentação ligada à DEVS-78.
  - A propagação de uma suspensão de capacidade (`CapabilitySuspended`) chega aos consumidores em menos de 30 s (`docs/design/13` §4); o cache dos consumidores é invalidado por estes eventos. A suspensão do parceiro não emite evento nesta versão: só aparece na consulta `FindEffectiveCapability` (`partner_not_active`), e a propagação dela está no Escopo fora.
- **Consome**: nenhum nesta versão; por isso a `janela_redelivery` não se aplica (ADR-020). O consumo de `ExternalSchemaMismatchDetected` (Receivables Gateway) e de `FundingRequestOpened` (Marketplace) está no Escopo fora; a dependência de Effects aparece no item do estado `inactive` do Escopo fora.
- **Saída**: `FindOrganization` pela porta `OrganizationDirectory` (‡), chamada no `Run` do caso de uso antes de `usecase.Decide`; a chamada antes da transação fica para o provider gRPC real.

## Borda e persistência

- **Borda**: só gRPC, `company.partner.service.v1.PartnerService`, com 24 RPCs atrás da cadeia de interceptors do kernel: um por comando (14: `RegisterPartner`, `ChangePartnerStage`, `RegisterAgreement`, `ActivateAgreement`, `DeclareCapability`, `RecordHomologation`, `EnableCapability`, `SuspendCapability`, `ChangeRollout`, `DefineProduct`, `RetireProduct`, `RegisterIntegrationProfile`, `RaiseCertificateExpiryAlert`, `DefineOperationalPolicy`), declarados em `rpc.Commands()`, e um por consulta (10). O OpenAPI v1 fica em `contract/openapi/v1`. A administração por backoffice e a Partner API são REST do `bff`.
- **Banco**: database `partner`, schema `public`; a role migradora `partner_migrator` é dona do banco, e a role de runtime `partner` tem só os privilégios declarados por tabela (ADR-063 do DMPF). Seis tabelas, `partners`, `partner_agreements`, `capabilities`, `financial_products`, `integration_profiles` e `operational_policies`, com `tenant_id` à frente da chave. Persistência híbrida, com colunas tipadas só para o que uma consulta filtra:
  - `partners`: `organization_id`, `role`, `status`, com índice único `(tenant_id, organization_id, role)` de salvaguarda (††)
  - `partner_agreements`: `partner_id`, `status`
  - `capabilities`: `partner_id`, `operation`, `asset_type`, `environment`, `status` (`FindEffectiveCapability`, `FindEligiblePartners`)
  - `financial_products`: `partner_id`, `status`, `valid_until` — `valid_until` é tipada, sem filtro SQL; a vigência é avaliada em memória pela regra de Agregados
  - `integration_profiles`: `partner_id`, `environment` (`FindCertificatesExpiring`), `certificate_expires_at` — tipada, sem filtro SQL; o refino por vencimento é em memória
  - `operational_policies`: `partner_id`
  - nas cinco tabelas versionadas (`partner_agreements`, `capabilities`, `financial_products`, `integration_profiles` e `operational_policies`), `supersedes_id` tipada, com `UNIQUE (tenant_id, supersedes_id)` e índice único parcial da v1 por série (††)
  - o resto de cada agregado vai em `snapshot` `jsonb`. As versões anteriores permanecem como linhas.
- Diverge do design (ADR-022, linha 2): o isolamento por tenant é `tenant_id` injetado pelo `ExecutionContext` no `postgres.Table` (ADR-051 DMPF), não RLS como mecanismo primário (`docs/design/00` §4.1); `tenantId` não é campo do agregado.
- Diverge do design (ADR-022, linha 6): eventos são Protobuf com CloudEvents montado pelo relay; o envelope JSON e o `schemaUri` do doc 92 §1 não se aplicam.
- Diverge do design (ADR-022, linha 9): `instant` é inteiro de nanossegundos no domínio.
- Diverge do design (ADR-022, linha 15): a unicidade por tenant é verificada por pré-leitura, com o índice como salvaguarda (††), até a SPEC-7SRQ3RPK.
- Nesta versão, a configuração de parceiros vive num tenant único da plataforma, e o rollout avalia o `tenantId` dos filtros (decisão de tenant único); leitura e projeção entre tenants esperam o K4, no Escopo fora.
- Segredos são referenciados por `secretRef`; nenhum valor entra no domínio, em evento ou em log.

## Políticas transversais

- **Tenant**: tenant único da plataforma nesta versão; vem do token verificado.
- **Idempotência**: todo comando exige chave de idempotência na metadata gRPC (FND-04; ADR-056). Os comandos de criação entram pelo `bff` por `POST` com a chave declarada (RST-02). O replay com a mesma chave devolve a resposta registrada; a repetição com chave nova decide de novo sobre o estado atual. `RaiseCertificateExpiryAlert` e `RetireProduct(expired)` usam chave determinística com o dia UTC da avaliação: `cert-alert.<profileId>.<thresholdDays>.<yyyymmdd>` e `product-expired.<productId>.<yyyymmdd>`; a recusa por prazo ainda não alcançado fica registrada só naquele dia, e o domínio impede segundo alerta pelo mesmo limiar e segunda expiração. O contexto não consome evento nesta versão.
- **Autorização**: a permissão `partner:<operação-kebab>` é conferida no `bff`; no contexto, `usecase.Permitted` exige só o tenant. As permissões `partner:*` e a matriz de papéis (`docs/design/95-personas-jornadas.md`) estão no Escopo fora.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); os eventos publicados vão ao Ledger, e toda mudança fica reproduzível pelas versões.
- **Dados**: `supportContact` e `incidentContact` são PII, fora de log e de evento; segredos só por referência.
- **Negar em caso de dúvida**: quando a capacidade efetiva não puder ser confirmada, a operação não prossegue, e toda negação registra a condição que falhou (enum fechado).
- **Suspensão e rollout**: reduzir a exposição de uma capacidade (suspensão ou rollout) tem efeito em menos de 30 s e nunca interrompe operação em curso nem o acompanhamento de comando já enviado; a suspensão do parceiro só aparece em `FindEffectiveCapability` (`partner_not_active`), e a propagação dela está no Escopo fora.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `partner`: `fmt-check`, `vet`, `build`, `lint` e `test-race` e `test-distributed` com Postgres; `buf-lint`, `buf-pins`, `buf-generate-check` e `buf-breaking` contra `origin/master`; `dmpf-context-check.sh` (`--phase structural` com `DMPF_KERNEL_DDL` e `--context` do contexto); `conformance --root .`; `dmpf-gate-check.sh`; `modsync --check` e `infrasync --check`.
- [ ] Cenários de aceite por comando e para as consultas de capacidade efetiva e de elegibilidade:

```text
DADO uma organização com kind funder em Identity
QUANDO RegisterPartner chega com role funder
ENTÃO o parceiro nasce prospect, com registeredAt = at, e PartnerRegistered vai à outbox

DADO uma organização com kind funder em Identity e nenhum parceiro dessa organização com role funder
QUANDO dois RegisterPartner concorrentes chegam com a mesma organização, role funder e chaves de idempotência distintas
ENTÃO exatamente um grava, com uma linha e um PartnerRegistered, e o outro sai como erro técnico, sem linha nem evento

DADO um parceiro homologating com acordo ativo e vigente e uma capacidade enabled
QUANDO ChangePartnerStage chega com target active
ENTÃO o parceiro vai a active, com stageChangedAt = at, e nada é emitido

DADO um parceiro active com uma capacidade enabled
QUANDO ChangePartnerStage chega com target suspended e reason
ENTÃO o parceiro vai a suspended, a capacidade segue enabled e nada é emitido

DADO um parceiro suspended com acordo ativo e vigente e uma capacidade suspended
QUANDO ChangePartnerStage chega com target active
ENTÃO o parceiro volta a active, a capacidade segue suspended e nada é emitido

DADO um parceiro active
QUANDO ChangePartnerStage chega com target offboarding e reason
ENTÃO o parceiro vai a offboarding, com offboardingStartedAt = at, e PartnerOffboardingStarted é emitido

DADO um parceiro registrado
QUANDO RegisterAgreement chega com validFrom e validUntil coerentes
ENTÃO o acordo nasce draft, version 1, e nada é emitido

DADO um acordo draft
QUANDO ActivateAgreement chega com contractEvidenceId e os critérios H14, H15, H17 e H18
ENTÃO o acordo vai a active e PartnerAgreementActivated é emitido

DADO um parceiro
QUANDO DeclareCapability chega com operação de escrita, stateQuerySupported falso, restrictionNote e callbackApplicable verdadeiro
ENTÃO a capacidade nasce declared, version 1, com rollout disabled e callbackApplicable persistido, e nada é emitido

DADO uma capacidade declared de escrita com consulta de estado e callbackApplicable falso
QUANDO RecordHomologation chega com H1 a H8 e H10 a H12 todos passed
ENTÃO a capacidade vai a homologated

DADO uma capacidade suspended com trigger schema_mismatch, com todos os critérios aplicáveis passed antes de suspendedAt
QUANDO RecordHomologation chega com todos os critérios aplicáveis passed
ENTÃO a capacidade vai a homologated, contando só os resultados com recordedAt posterior a suspendedAt, e nada é emitido

DADO uma capacidade homologated, acordo ativo e vigente e perfil de integração vigente para o ambiente
QUANDO EnableCapability chega
ENTÃO a capacidade vai a enabled e CapabilityEnabled é emitido com environment, stateQuerySupported e restrictionNote

DADO uma capacidade enabled
QUANDO SuspendCapability chega com reason
ENTÃO a capacidade vai a suspended com trigger manual e CapabilitySuspended é emitido com environment e version

DADO uma capacidade suspended com trigger manual, acordo ativo e vigente e perfil de integração vigente
QUANDO EnableCapability chega
ENTÃO a capacidade volta a enabled sem nova homologação e CapabilityEnabled é emitido

DADO uma capacidade enabled com rollout enabled
QUANDO ChangeRollout chega com mode percentage, percentage 10 e excludedTenantIds
ENTÃO o rollout é substituído, os excluídos prevalecem sobre qualquer modo e CapabilityRolloutChanged é emitido

DADO uma capacidade com rollout percentage 10
QUANDO o mesmo tenant é avaliado duas vezes
ENTÃO fica do mesmo lado nas duas, com bucket igual aos 8 primeiros bytes de sha256(capabilityId ":" tenantId), big-endian, mod 100

DADO um acordo ativo e vigente
QUANDO DefineProduct chega com faixas coerentes e sem supersedesId
ENTÃO o produto nasce active, version 1, e PartnerProductDefined é emitido com as faixas

DADO um produto active na version 1
QUANDO DefineProduct chega com o mesmo code e supersedesId igual ao id dessa versão
ENTÃO o produto nasce na version 2 e PartnerProductChanged é emitido com supersedesId

DADO um produto active com validUntil menor ou igual a at
QUANDO RetireProduct chega com reason expired
ENTÃO o produto vai a retired e PartnerProductExpired é emitido

DADO um produto active
QUANDO RetireProduct chega com reason withdrawn
ENTÃO o produto vai a retired e PartnerProductRetired é emitido

DADO um produto já retirado por RetireProduct com reason expired e a chave product-expired.<productId>.<yyyymmdd>
QUANDO a mesma chamada chega de novo com a mesma chave
ENTÃO devolve a resposta registrada e nenhum PartnerProductExpired novo é emitido

DADO um parceiro
QUANDO RegisterIntegrationProfile chega com endpoint, secretRef e certificado futuro
ENTÃO o perfil nasce na version 1, sem o valor do segredo, e nada é emitido

DADO um perfil cujo certificado vence em 25 dias
QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 30
ENTÃO lastAlertedThresholdDays vira 30 e PartnerCertificateExpiring é emitido

DADO um alerta de 30 dias já emitido com a chave cert-alert.<profileId>.30.<yyyymmdd>
QUANDO RaiseCertificateExpiryAlert chega de novo com a mesma chave
ENTÃO devolve a resposta registrada e nenhum PartnerCertificateExpiring novo é emitido

DADO um parceiro
QUANDO DefineOperationalPolicy chega com rateLimitPerMinute 100 e os dois contatos
ENTÃO a política nasce na version 1 e nada é emitido

DADO um RegisterAgreement já aceito com uma chave de idempotência
QUANDO o mesmo RegisterAgreement chega com a mesma chave
ENTÃO devolve a resposta registrada, sem nova linha

DADO uma capacidade suspended e uma enabled do mesmo parceiro, operação e environment, com assetTypes distintos
QUANDO FindEffectiveCapability consulta com o assetType da suspensa
ENTÃO devolve negado com failedCondition capability_not_enabled, fora de qualquer UoW

DADO um parceiro suspended com uma capacidade enabled
QUANDO FindEffectiveCapability consulta essa capacidade
ENTÃO devolve negado com failedCondition partner_not_active

DADO uma capacidade enabled com rollout enabled e o tenant em excludedTenantIds
QUANDO FindEffectiveCapability consulta com esse tenantId
ENTÃO devolve negado com failedCondition not_in_rollout

DADO uma registradora active com acordo vigente e capacidade enabled, com o tenant no rollout
QUANDO FindEffectiveCapability consulta sem productId
ENTÃO devolve permitido, sem avaliar a condição de produto

DADO financiadores com e sem produto compatível com o pedido e um parceiro de outro papel
QUANDO FindEligiblePartners consulta por assetType, operation, environment, tenantId, amountMinor, termDays, region e asOf
ENTÃO devolve só os financiadores elegíveis ao pedido
```

- [ ] Ao menos um cenário por código de rejeição (nada é escrito e o estado não muda):

```text
DADO nenhuma organização com o id informado QUANDO RegisterPartner chega ENTÃO rejeita com partner-management/partner/organization-not-found
DADO uma organização kind client QUANDO RegisterPartner chega com role funder ENTÃO rejeita com partner-management/partner/organization-kind-mismatch
DADO um parceiro registrado com a mesma organização e role QUANDO RegisterPartner chega com essa organização e role ENTÃO rejeita com partner-management/partner/already-registered
DADO um parceiro prospect QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/invalid-stage-transition
DADO um parceiro active QUANDO ChangePartnerStage chega com target suspended e reason vazio ENTÃO rejeita com partner-management/partner/reason-empty
DADO um parceiro homologating sem acordo ativo QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/agreement-not-active
DADO um parceiro suspended sem acordo ativo e vigente QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/agreement-not-active
DADO um parceiro homologating sem capacidade enabled QUANDO ChangePartnerStage chega com target active ENTÃO rejeita com partner-management/partner/no-enabled-capability
DADO qualquer parceiro QUANDO RegisterAgreement chega com validUntil anterior a validFrom ENTÃO rejeita com partner-management/partner-agreement/validity-invalid
DADO um acordo active QUANDO ActivateAgreement chega ENTÃO rejeita com partner-management/partner-agreement/not-draft
DADO um acordo draft QUANDO ActivateAgreement chega sem contractEvidenceId ENTÃO rejeita com partner-management/partner-agreement/contract-evidence-missing
DADO um acordo draft QUANDO ActivateAgreement chega sem H17 em satisfiedCriteria ENTÃO rejeita com partner-management/partner-agreement/contractual-criteria-incomplete
DADO acordos nas versions 1 e 2 do mesmo parceiro QUANDO RegisterAgreement chega com supersedesId igual ao id da version 1 ENTÃO rejeita com partner-management/partner-agreement/supersedes-invalid
DADO qualquer parceiro QUANDO DeclareCapability chega com operation vazia ENTÃO rejeita com partner-management/capability/operation-empty
DADO qualquer parceiro QUANDO DeclareCapability chega com assetType vazio ENTÃO rejeita com partner-management/capability/asset-type-empty
DADO qualquer parceiro QUANDO DeclareCapability de escrita chega com stateQuerySupported falso e sem restrictionNote ENTÃO rejeita com partner-management/capability/state-query-restriction-undeclared
DADO uma capacidade na série partnerId+operation+assetType+environment QUANDO DeclareCapability chega para a mesma série sem supersedesId ENTÃO rejeita com partner-management/capability/supersedes-invalid
DADO uma capacidade declared QUANDO RecordHomologation chega com results vazio ENTÃO rejeita com partner-management/capability/criteria-empty
DADO uma capacidade declared QUANDO RecordHomologation chega com criterionId H13 ENTÃO rejeita com partner-management/capability/unknown-criterion
DADO uma capacidade homologated QUANDO RecordHomologation chega ENTÃO rejeita com partner-management/capability/homologation-closed
DADO uma capacidade suspended com trigger manual QUANDO RecordHomologation chega ENTÃO rejeita com partner-management/capability/homologation-closed
DADO uma capacidade declared QUANDO EnableCapability chega ENTÃO rejeita com partner-management/capability/not-homologated
DADO uma capacidade suspended com trigger schema_mismatch QUANDO EnableCapability chega ENTÃO rejeita com partner-management/capability/rehomologation-required
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
DADO um produto na série partnerId+code QUANDO DefineProduct chega com o mesmo code e sem supersedesId ENTÃO rejeita com partner-management/financial-product/supersedes-invalid
DADO um produto retired QUANDO RetireProduct chega ENTÃO rejeita com partner-management/financial-product/not-active
DADO um produto active dentro da vigência QUANDO RetireProduct chega com reason expired ENTÃO rejeita com partner-management/financial-product/not-yet-expired
DADO qualquer parceiro QUANDO RegisterIntegrationProfile chega com endpoint vazio ENTÃO rejeita com partner-management/integration-profile/endpoint-empty
DADO qualquer parceiro QUANDO RegisterIntegrationProfile chega sem secretRef ENTÃO rejeita com partner-management/integration-profile/secret-ref-missing
DADO qualquer parceiro QUANDO RegisterIntegrationProfile chega com certificateExpiresAt anterior a at ENTÃO rejeita com partner-management/integration-profile/certificate-expiry-invalid
DADO um perfil de outro ambiente do mesmo parceiro QUANDO RegisterIntegrationProfile chega com supersedesId igual ao id desse perfil ENTÃO rejeita com partner-management/integration-profile/supersedes-invalid
DADO um perfil QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 45 ENTÃO rejeita com partner-management/integration-profile/invalid-alert-threshold
DADO um perfil cujo certificado vence em 90 dias QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 60 ENTÃO rejeita com partner-management/integration-profile/threshold-not-reached
DADO um perfil já alertado em 30 dias QUANDO RaiseCertificateExpiryAlert chega com thresholdDays 60 ENTÃO rejeita com partner-management/integration-profile/already-alerted
DADO qualquer parceiro QUANDO DefineOperationalPolicy chega com rateLimitPerMinute 0 ENTÃO rejeita com partner-management/operational-policy/rate-limit-invalid
DADO qualquer parceiro QUANDO DefineOperationalPolicy chega sem incidentContact ENTÃO rejeita com partner-management/operational-policy/contacts-missing
DADO uma política na version 1 de um parceiro QUANDO DefineOperationalPolicy chega para o mesmo parceiro sem supersedesId ENTÃO rejeita com partner-management/operational-policy/supersedes-invalid
```

## Escopo fora

- **REST do `bff`**: a administração de parceiros e a Partner API (`docs/design/91-contratos-api.md` §4, que descreve o convite, a oferta e a confirmação de desembolso do financiador) são do `bff`; este contexto serve só gRPC.
- **Adaptadores e `CapabilityProfile`**: o Receivables Gateway aplica as configurações e seleciona adaptador e versão; este contexto só declara a capacidade e os dados de integração.
- **Elegibilidade e ranking**: Eligibility e Marketplace consomem produtos e capacidades; a ordenação, o patrocínio no ranking e o `PricingBreakdown` não são deste contexto.
- **Modelo de monetização, cobrança e tributação**: Billing (`SPEC-J9JWBT9W`); aqui só a `CommercialRule`, travada no #7.
- **Cofre de segredos e emissão de certificado**: o contexto guarda a referência (`secretRef`, `certificateRef`), não o valor nem o ciclo de vida do segredo.
- **Procedimento humano de offboarding**: inventário de operações em curso, tratamento caso a caso e cláusula de acesso residual são do Backoffice e do contrato de saída (13:N11).
- **Comandos de suspensão por produto, tipo de ativo e parceiro inteiro**: nesta versão, a suspensão é por capacidade (`SuspendCapability`), e a do parceiro muda só o `Partner` (`ChangePartnerStage`).
- **Leitura e projeção entre tenants**: K4 (SPEC-QVDGV6J0, DEVS-100); nesta versão, a configuração vive num tenant único da plataforma.
- **Armazenamento de evidência de homologação e de contrato**: porta de object storage da SPEC-CWBA4BPV (DEVS-95); o contexto guarda só ids opacos.
- **Composition root**: o esqueleto de `app/` e o `cmd/` com `--role` nascem do generator; o harness escreve o conteúdo de domínio dentro deles.
- **`CommercialRule` e `FindCommercialRule`**: agregado, comando de definição, consulta e tabela saem desta versão, com os três códigos de regra comercial (`commercial-rule/validity-invalid`, `commercial-rule/sponsored-positions-invalid` e `commercial-rule/rates-invalid`) **[ABERTA: #7 (D1–D3; 13:N9) — o modelo de monetização trava o agregado e o comando; `canOverrideMerit` depende de docs/design/07-design-marketplace.md §3.3 R6; docs/design/92-schemas-eventos.md §4 não traz evento de `CommercialRule`, embora Billing dependa dela (docs/design/18)]**. A verificação de H17 contra a regra comercial entra com ela.
- **Consumo de `ExternalSchemaMismatchDetected`**: dono da suspensão por quebra de schema, mapa `connectorId → partnerId`, tópico multi-tipo do Gateway e `janela_redelivery`; também a suspensão com `trigger = schema_mismatch` disparada pelo evento, porque o RPC `SuspendCapability` só expõe `manual` **[ABERTA: docs/design/04-design-receivables-gateway.md §5 manda o Gateway suspender a operação ao detectar a quebra, e docs/design/13 §7 manda Partner Management fazer a suspensão automática; o schema do evento também não está em docs/design/92 §3]**.
- **Consumo de `FundingRequestOpened`**: declarado em docs/design/92 §4 (consumidor: Partners), sem efeito definido em `docs/design/13` **[ABERTA: docs/design/92-schemas-eventos.md §4 × docs/design/13 — efeito do consumo; a métrica "volume e conversão por produto" do PRD é o único indício]**.
- **Estado `inactive` e o código `effects-pending-release`**: a saída de `offboarding` exige consultar efeitos vigentes pendentes de liberação, ausente da spec de Effects (SPEC-ZFDEPFWC) **[A CONFIRMAR: 13:N11; docs/design/92 §4 não declara Partner consumidor de eventos de Effects]**.
- **Cliente gRPC real do Identity e deploy**: o provider real da porta `OrganizationDirectory` (‡) e a implantação do contexto.
- **Permissões `partner:*`**: as permissões no Identity e a linha de administração de parceiros na matriz de papéis **[ABERTA: docs/design/95-personas-jornadas.md — a matriz de papéis não cobre a administração de parceiros]**.
- **Propagação da suspensão e do offboarding do parceiro**: fan-out ou saga às capacidades (`SuspendCapability` com `trigger = offboarding`) e propagação ao Gateway **[ABERTA: docs/design/13-design-partner-management.md §5 — os escopos de suspensão por produto, tipo de ativo e parceiro inteiro não têm comando próprio]**.
- **Comandos de expiração**: o disparo agendado de `RaiseCertificateExpiryAlert` e `RetireProduct(expired)`, pelo agendador da plataforma ou pelo backoffice, com as chaves determinísticas.
- **Rollout por produto** (`docs/prd/13-prd-partner-management.md`, requisito 5): a dimensão de produto no rollout; o rollout desta versão é só por tenant.
- **Reabertura de homologação por nova versão de perfil de integração**: a convivência entre a capacidade homologada e o perfil novo.
- **Rotação e revogação de credencial**.
- **Vencimento de acordo sem evento e acordos sobrepostos**: a vigência é avaliada pela data; não há evento de vencimento nem regra de sobreposição.
- **Regressões de estágio**: `homologating → integrating` e abandono do processo de entrada (`docs/prd/13-prd-partner-management.md:42`).
- **Atributos do acordo** além da vigência, da evidência e do modelo do instrumento.
- **Parâmetros de produto lidos pela Eligibility** além dos publicados nos eventos de produto.
- **Papel `ACQUIRER`**: o PRD 13 não o lista; em `docs/design/90` §2.1 ele é `system` de `ExternalReference`.
- **Prefixo de id** (`docs/design/90` §1.1): os ids são `generated`, sem prefixo.
- **Eventos das mudanças materiais sem evento**: `RegisterAgreement`, `DeclareCapability`, `RecordHomologation`, `RegisterIntegrationProfile` e `DefineOperationalPolicy` não emitem evento nesta versão.
- **Valores de contingência** da `OperationalPolicy`: `fallback` é texto livre **[ABERTA: docs/prd/13-prd-partner-management.md — os valores de contingência não são definidos]**.
