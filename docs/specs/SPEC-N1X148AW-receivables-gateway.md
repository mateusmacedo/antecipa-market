---
id: SPEC-N1X148AW
slug: receivables-gateway
title: Bounded context — receivables-gateway
stage: backlog
priority: P0
depends_on: [SPEC-GXGASP24, SPEC-9TEQ9EV9]
ticket_url: https://linear.app/mmda/issue/DEVS-84/spec-n1x148aw-bounded-context-receivables-gateway
subtask_urls: []
created: 2026-10-12
---

# SPEC-N1X148AW: Bounded context — receivables-gateway

## Identidade

- **bounded_context**: `receivables-gateway`
- **name**: `gateway`
- **Convenções de tipo**: valor monetário é `int` em unidade menor mais `currency: string`; data pura (`…Date`) é `string` ISO 8601; `instant` é inteiro de nanossegundos. Os valores de enumeração usam `snake_case` minúsculo; o desenho (`04`) os escreve em `UPPER_SNAKE` e o mapeamento é 1 para 1.
- **Linguagem ubíqua**:
  - *Conector* (`Connector`) - implementação de integração para um parceiro em um ambiente.
  - *Perfil de capacidades* (`CapabilityProfile`) - operações, protocolos e qualidade de dado que um conector declara, por versão.
  - *Comando externo* (`ExternalCommand`) - intenção de escrita enviada a um parceiro, durável antes de ser transmitida.
  - *Interação externa* (`ExternalInteraction`) - troca bruta (request, response, evidência) com um parceiro, preservada sem normalização.
  - *Referência externa* (`ExternalReference`) - vínculo entre um objeto interno e a identidade dele no parceiro, preservada exatamente como recebida.
  - *Tentativa de entrega* (`DeliveryAttempt`) - registro de uma transição do comando: instante, resultado e interação.
  - *Resultado desconhecido* (`UNKNOWN_OUTCOME`) - não se sabe se o comando surtiu efeito; resolve-se só por consulta de estado, nunca por reenvio.
  - *Consulta de estado* - operação do parceiro que informa o desfecho de um comando enviado.
  - *Porta canônica* - interface estável que isola os demais contextos do protocolo do parceiro.
  - *Proveniência* - `asOf`, `providerId` e referência externa que toda saída de porta carrega.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/04-prd-receivables-gateway.md`; `docs/design/04-design-receivables-gateway.md` §2 a §7 (capacidades, portas, erros, quebra de contrato, reprocessamento, webhook); `docs/design/00-arquitetura-tecnica.md` §7 (ciclo do comando externo, idempotência, resiliência); `docs/design/90-modelo-de-dados.md` §2.1 (`ExternalReference`); `docs/design/92-schemas-eventos.md` §4; `docs/prd/14-catalogo-contratos-eventos.md`.
- **Situação**: `parcial` - portas, taxonomia de erros, webhook, replay e simulador de parceiro (04:G12) são construíveis; os adaptadores reais dependem do mapeamento de provedores (#5).
- **Fase de entrega**: 1 (cartão); 2 (escriturador); 4 (múltiplas infraestruturas).
- **Dependências**: duro, Partner Management (`SPEC-9TEQ9EV9`: `CapabilityEnabled`, `CapabilitySuspended`); brando, Consent & Mandates (`ConsentGranted`, `ConsentRevoked`, que travam a consulta em modo fail-closed). `depends_on` declara só a fundação e a aresta dura reduzida; Consent depende do Gateway (`AuthorizationPort`), então o consumo de eventos de Consent é aresta de retroalimentação e não entra em `depends_on`.
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
|---|---|---|
| #5 (04:G8; B1, B14 a B17) | Nenhum provedor mapeado: sem protocolo, calendário, limites, tabela de códigos de erro e homologação reais | Adaptadores reais e dimensionamento; portas e simulador seguem construíveis |
| B1 (04:G9) | Existência de consulta de estado por operação de escrita | `stateQuerySupported` dos perfis reais; sem ela todo `unknown_outcome` vai a tratamento humano |
| B5 (04:G10) | Validade da posição por fonte | `positionFreshnessSla` do perfil |
| B14 | Ponto único de consulta entre registradoras ou integração N para N | Dimensionamento do conector |
| B15, B16, B17 | Revogação de credencial em emergência, contato de incidente, homologação e custo | Operação e homologação dos conectores |
| 04:G11 | Retenção do payload bruto não definida | Retenção de `ExternalInteraction` |
| #1 (A1) | Autorização do Banco Central (transversal) | Existencial para o produto inteiro |
| #12 (E4) | Volumetria real (transversal) | Dimensionamento, não bloqueia construção |

- **Decisões de produto abertas**: nenhuma própria. Técnicas: retenção do bruto (04:G11) e política de suspensão por operação (04 §5).
- **Condição para sair do backlog**: fundação `SPEC-GXGASP24` concluída e `SPEC-9TEQ9EV9` (Partner Management) concluída; núcleo construível com o simulador de parceiro (04:G12). Adaptadores reais só depois de #5.

## Agregados

Seis agregados. `DeliveryAttempt` é entidade interna de `ExternalCommand` (não agregado): cada transição e a tentativa que a registra precisam ser atômicas, e uma UPR altera um agregado. Diverge do design: nenhum, o PRD só lista `DeliveryAttempt` no modelo operacional.

### Connector

- **Identidade**: `natural` por `connectorKey` (`<partnerId>:<environment>`)
- **Campos**:
  - `partnerId: id`
  - `environment: development | integration | homologation | production`
  - `credentialRef: string` - referência ao cofre; credencial é segregada por parceiro e ambiente e nunca entra no estado
  - `suspensions: list<OperationSuspension>` - coleção indispensável: suspensão é por operação, não por parceiro (04 §5). Elemento: `operation: string` (vazio = conector inteiro), `assetType: string` (vazio = todos), `cause: partner | schema_mismatch`, `since: instant`
  - `registeredAt: instant`
- **Estados**: nenhum campo de estado; "suspenso" deriva de `suspensions`.
- **Transições permitidas**: nenhuma de estado; `suspensions` cresce e encolhe por `SuspendOperation` e `ResumeOperation`.
- **Invariantes de estado**:
  - `credentialRef` nunca é vazio - `RegisterConnector` rejeita com `receivables-gateway/connector/credential-ref-empty`.
  - Suspensão bloqueia novos comandos e leituras; o acompanhamento de comandos já enviados continua (13 §5).
- **Multi-tenancy**: conector e perfil são da plataforma e compartilhados entre tenants; leitura por qualquer tenant é **[ABERTA: requer extensão do kernel - ADR-051 DMPF]**.

### CapabilityProfile

- **Identidade**: `natural` por `profileKey` (`<connectorKey>:<version>`)
- **Campos**:
  - `connectorId: id`, `version: int`, `effectiveFrom: instant`
  - `operations: list<OperationCapability>` - coleção indispensável (04 §2.1). Elemento: `operation: string`; `supported: bool`; `protocol: sync_http | async_callback | file_batch | polling`; `stateQuerySupported: bool`; `idempotencySupport: native | key_header | none`; `partialResultSupported: bool`; `assetTypes: list<string>`; `maxItemsPerRequest`, `maxRequestsPerMinute`, `maxPayloadBytes: int`; `businessCalendar`, `cutoffTime`, `processingWindow: string`; `encumbranceCoverage: complete | partial | unavailable`; `positionFreshnessSla: int` (segundos, opcional: ausente quando a fonte não declara, B5)
- **Estados**: nenhum. Cada versão é imutável; mudança de capacidade é nova versão.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Um perfil declara ao menos uma operação - `receivables-gateway/capability-profile/operations-empty`.
  - Uma versão não é redeclarada - `receivables-gateway/capability-profile/already-declared`.
  - O perfil vigente em um instante é o de maior `version` com `effectiveFrom` menor ou igual a ele.

### QueryAuthorization

Projeção local do consentimento de consulta; existe para a trava fail-closed da `ReceivablesQueryPort`.

- **Identidade**: `natural` por `consentId`
- **Campos**: `organizationId: id`, `consentVersion: int`, `status: granted | revoked`, `assetTypes: list<string>` (escopo; coleção indispensável), `periodFromDate: string`, `periodUntilDate: string`, `grantedAt: instant`, `revokedAt: instant` (opcional)
- **Estados**: `granted`, `revoked`
- **Transições permitidas**: (novo) → `granted` por `GrantQueryAuthorization`; `granted` → `revoked` por `RevokeQueryAuthorization`; `granted` → `granted` (renovação, `consentVersion` maior)
- **Invariantes de estado**: `revoked` é final. Autorização ausente equivale a negada.
- Só `consentType = RECEIVABLES_OPT_IN` é projetado (92 §3.1); os demais instrumentos de Consent são descartados no consumo.

### ExternalCommand

- **Identidade**: `natural` por `idempotencyKey`, derivada de dados de negócio estáveis: `hash(tenantId, partnerId, operationType, businessKey, payloadVersion)` (00 §7.3). A identidade natural é a idempotência de saída no domínio: reenvio com a mesma chave não cria segundo comando. **[ABERTA: ciclo de comando externo sem suporte no kernel]**
- **Campos**:
  - `connectorId: id`, `operation: string`, `assetType: string`, `businessKey: string`, `payloadVersion: int`
  - `payload: string` (JSON canônico, opaco ao domínio, classe `CONFIDENTIAL`, vai no `snapshot`), `payloadHash: string`
  - `profileVersion: int`, `stateQuerySupported: bool`, `idempotencySupport: native | key_header | none`, `partialResultSupported: bool` - os quatro copiados do perfil vigente na criação
  - `status`, `createdAt: instant`, `unknownSince: instant` (opcional), `confirmedAt: instant` (opcional), `externalReferenceId: id` (opcional)
  - `attempts: list<DeliveryAttempt>` - coleção indispensável, uma entrada por transição (00 §7.2). Elemento: `sequence: int`, `at: instant`, `result: sent | acknowledged | confirmed | failed | unknown`, `category: string` (categoria da taxonomia, vazia nos sucessos), `interactionId: id`
- **Estados**: `created`, `sent`, `acknowledged`, `confirmed`, `retryable_failure`, `final_failure`, `unknown_outcome`, `cancelled`
- **Transições permitidas**:
  - `created` → `sent` - `RecordCommandSent`; `created` → `cancelled` - `CancelExternalCommand`
  - `sent` → `acknowledged` - `RecordCommandAcknowledged`
  - `sent` ou `acknowledged` → `confirmed` - `RecordCommandConfirmed`
  - `sent` ou `acknowledged` → `retryable_failure` ou `final_failure` - `RecordCommandFailure`
  - `sent` ou `acknowledged` → `unknown_outcome` - `RecordUnknownOutcome`
  - `unknown_outcome` → `confirmed`, `final_failure` ou `retryable_failure` - `ResolveUnknownOutcome`
  - `retryable_failure` → `sent` - `RecordCommandSent` (reenvio com a mesma chave)
- **Invariantes de estado**:
  - `confirmed`, `final_failure` e `cancelled` são terminais.
  - `acknowledged` não equivale a efeito confirmado: só `confirmed` emite `ExternalCommandConfirmed`.
  - `unknown_outcome` não volta a `sent`: reenvio direto rejeita com `receivables-gateway/external-command/not-sendable`.
  - `payload`, `payloadHash` e `payloadVersion` não mudam após a criação.
  - Com `unknown_outcome` e `stateQuerySupported = false` o comando é não reenviável sem intervenção humana (00 §7.2.4).

### ExternalInteraction

- **Identidade**: `generated`
- **Campos**:
  - `connectorId: id`, `operation: string`, `direction: outbound | inbound`, `commandId: id` (opcional)
  - `partnerEventId: string` (opcional; identificador do parceiro, chave de deduplicação), `businessDate: string` (opcional; data de negócio informada pelo parceiro)
  - `requestRef: string`, `requestHash: string`, `responseRef: string`, `responseHash: string` - o bruto vive em armazenamento de objetos e o domínio guarda referência e hash. **[ABERTA: provider de object storage]**
  - `status`, `normalizationRevision: int`, `mismatchKind: structure | missing_field | type | version` (opcional), `mismatchDetail: string` (opcional), `receivedAt: instant`
- **Estados**: `stored`, `duplicate`, `normalized`, `uncorrelated`, `schema_mismatch`
- **Transições permitidas**:
  - (novo) → `stored` ou `duplicate` - `RecordExternalInteraction`
  - `stored` → `normalized`, `uncorrelated` ou `schema_mismatch` - `ConcludeInteraction`
  - `normalized`, `uncorrelated` ou `schema_mismatch` → `stored` - `ReprocessInteraction` (incrementa `normalizationRevision`)
- **Invariantes de estado**:
  - O bruto nunca é eliminado nem substituído pelo normalizado - `RecordExternalInteraction` rejeita sem referência com `receivables-gateway/external-interaction/raw-payload-missing`.
  - Valor de enumeração desconhecido não é quebra de contrato (04 §5); só estrutura, campo obrigatório ausente, tipo divergente ou versão incompatível produzem `schema_mismatch`.

### ExternalReference

- **Identidade**: `natural` por `referenceKey` (`<providerId>:<system>:<referenceType>:<reference>`)
- **Campos**: `providerId: id`, `system: registry | bookkeeper | acquirer | funder | depository`, `referenceType: asset | effect | authorization | command | settlement`, `reference: string` (exatamente como recebido), `normalizedReference: string` (derivado, nunca substitui o original), `subjectType: string`, `subjectId: id`, `asOf: instant`, `observedVersion: string` (opcional)
- **Estados**: nenhum
- **Transições permitidas**: nenhuma; o vínculo é imutável, exceto a renovação de `asOf` e `observedVersion`.
- **Invariantes de estado**: `reference` nunca é vazio - `receivables-gateway/external-reference/reference-empty`; uma referência externa aponta para um único objeto interno - `receivables-gateway/external-reference/already-linked`. Um objeto interno tem N referências.

## Comandos (UPRs)

Pré-condições que dependem de outro agregado do contexto (perfil, conector, interação) são resolvidas pelo caso de uso, por consulta local, antes da UoW, e entram como fato de entrada. **[ABERTA: ciclo de comando externo sem suporte no kernel]**

### RegisterConnector

- **Agregado**: `Connector`; **inicializa se ausente**
- **Entrada**: `partnerId: id`, `environment`, `credentialRef: string`
- **Pré-condições**: `credentialRef` não vazio → `receivables-gateway/connector/credential-ref-empty`
- **Efeitos sobre o estado**: quando ausente: `registeredAt ← at`, `suspensions ← []`; quando presente: nenhum
- **Eventos emitidos**: nenhum
- **Resposta**: `connectorKey: string`
- **Origem do cadastro**: **[ABERTA: 13 §7 - o IntegrationProfile do Partner não publica evento de cadastro de conector]**

### SuspendOperation

- **Agregado**: `Connector`; exige existente
- **Entrada**: `operation: string` (vazio = conector inteiro), `assetType: string`, `cause: partner | schema_mismatch`
- **Pré-condições**: nenhuma; suspensão repetida da mesma operação e causa é aceita sem alterar
- **Efeitos sobre o estado**: `suspensions ← suspensions + {operation, assetType, cause, since = at}`
- **Eventos emitidos**: nenhum
- **Resposta**: `connectorKey: string`

### ResumeOperation

- **Agregado**: `Connector`; exige existente
- **Entrada**: `operation: string`, `assetType: string`
- **Pré-condições**: nenhuma; retomar o que não está suspenso é aceito sem alterar
- **Efeitos sobre o estado**: remove de `suspensions` as entradas de `operation` e `assetType` informados
- **Eventos emitidos**: nenhum
- **Resposta**: `connectorKey: string`

### DeclareCapabilityProfile

- **Agregado**: `CapabilityProfile`; **cria**
- **Entrada**: `connectorId: id`, `version: int`, `effectiveFrom: instant`, `operations: list<OperationCapability>`
- **Pré-condições**:
  - `operations` não vazio → `receivables-gateway/capability-profile/operations-empty`
  - versão ainda não declarada → `receivables-gateway/capability-profile/already-declared`
- **Efeitos sobre o estado**: todos os campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `profileKey: string`

### GrantQueryAuthorization

- **Agregado**: `QueryAuthorization`; **inicializa se ausente**
- **Entrada**: `consentId: id`, `organizationId: id`, `consentVersion: int`, `assetTypes: list<string>`, `periodFromDate: string`, `periodUntilDate: string`
- **Pré-condições**: nenhuma; versão igual ou menor que a presente é aceita sem alterar
- **Efeitos sobre o estado**: quando ausente: `status ← granted`, demais campos ← entrada, `grantedAt ← at`; quando presente e `consentVersion` maior e `status = granted`: escopo e período ← entrada; quando `revoked`: nenhum
- **Eventos emitidos**: nenhum
- **Resposta**: `consentId: id`

### RevokeQueryAuthorization

- **Agregado**: `QueryAuthorization`; exige existente
- **Entrada**: `consentId: id`, `effectiveAt: instant`
- **Pré-condições**: nenhuma; revogar o já revogado é aceito sem alterar
- **Efeitos sobre o estado**: `status ← revoked`, `revokedAt ← effectiveAt`
- **Eventos emitidos**: nenhum
- **Resposta**: `consentId: id`

### CreateExternalCommand

- **Agregado**: `ExternalCommand`; **inicializa se ausente**
- **Entrada**: `idempotencyKey: string`, `connectorId: id`, `operation: string`, `assetType: string`, `businessKey: string`, `payloadVersion: int`, `payload: string`, `payloadHash: string`, `profileVersion: int`, `stateQuerySupported: bool`, `idempotencySupport`, `partialResultSupported: bool`, `capabilityGate: allowed | not_enabled | suspended`
- **Pré-condições**:
  - `capabilityGate = allowed`: o caso de uso verifica `supports(operation, assetType)` no perfil vigente e a ausência de suspensão no conector, sem chamada externa → `receivables-gateway/external-command/capability-not-enabled` (`not_enabled`) ou `receivables-gateway/external-command/operation-suspended` (`suspended`)
  - `businessKey` não vazio → `receivables-gateway/external-command/business-key-empty`
  - `payload` não vazio → `receivables-gateway/external-command/payload-empty`
  - chave já existente com `payloadHash` diferente → `receivables-gateway/external-command/idempotency-key-conflict`
- **Efeitos sobre o estado**: quando ausente: campos ← entrada, `status ← created`, `createdAt ← at`; quando presente com o mesmo `payloadHash`: nenhum, devolve o comando existente
- **Eventos emitidos**: nenhum
- **Resposta**: `idempotencyKey: string`, `status`
- **Alteração de conteúdo após o envio** (mesmo `businessKey` com `payloadVersion` novo e comando anterior em `sent` ou além) exige revisão e não reenvio (00 §7.3): **[ABERTA: 00:§7.3 - a checagem cruza comandos de chaves diferentes]**

### RecordCommandSent

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `interactionId: id`
- **Pré-condições**:
  - `status` é `created` ou `retryable_failure` → `receivables-gateway/external-command/not-sendable`
  - reenvio a partir de `retryable_failure` exige `idempotencySupport ≠ none` ou `stateQuerySupported` → `receivables-gateway/external-command/retry-not-safe` (req. 7 do PRD)
- **Efeitos sobre o estado**: `status ← sent`; `attempts ← attempts + {sequence + 1, at, sent, interactionId}`
- **Eventos emitidos**: `ExternalCommandSubmitted`
- **Resposta**: `idempotencyKey: string`

### RecordCommandAcknowledged

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `interactionId: id`, `externalReferenceId: id` (opcional)
- **Pré-condições**: `status = sent` → `receivables-gateway/external-command/not-sent`
- **Efeitos sobre o estado**: `status ← acknowledged`; `externalReferenceId ← entrada` quando informado; `attempts ← attempts + {…, acknowledged}`
- **Eventos emitidos**: `ExternalCommandAcknowledged` **[ABERTA: 92:§4 não lista o evento, o PRD 04 sim]**
- **Resposta**: `idempotencyKey: string`

### RecordCommandConfirmed

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `interactionId: id`, `externalReferenceId: id`, `confirmationSource: callback | polled_query | file`, `partial: bool`
- **Pré-condições**:
  - `status` é `sent` ou `acknowledged` → `receivables-gateway/external-command/not-in-flight`
  - `partial = true` só se `partialResultSupported` → `receivables-gateway/external-command/partial-result-not-supported` (ocorrência é inconsistência a investigar, 04 §2.2)
- **Efeitos sobre o estado**: `status ← confirmed`, `confirmedAt ← at`, `externalReferenceId ← entrada`; `attempts ← attempts + {…, confirmed}`
- **Eventos emitidos**: `ExternalCommandConfirmed`
- **Resposta**: `idempotencyKey: string`

### RecordCommandFailure

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `interactionId: id`, `category: retryable_transport | retryable_upstream | final_validation | final_business | final_authorization`, `partnerCode: string`, `partnerMessage: string`
- **Pré-condições**: `status` é `sent` ou `acknowledged` → `receivables-gateway/external-command/not-in-flight`
- **Efeitos sobre o estado**: `status ← retryable_failure` para `retryable_*` e `final_failure` para `final_*`; `attempts ← attempts + {…, failed, category}`
- **Eventos emitidos**: `ExternalCommandFailed` só quando `final_failure`; falha recuperável fica no histórico de tentativas, sem evento **[ABERTA: 04:§4 não diz se falha recuperável é publicada]**
- **Resposta**: `idempotencyKey: string`, `status`

### RecordUnknownOutcome

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `interactionId: id`, `category: unknown_sent | unknown_ambiguous | schema_mismatch`
- **Pré-condições**: `status` é `sent` ou `acknowledged` → `receivables-gateway/external-command/not-in-flight`
- **Efeitos sobre o estado**: `status ← unknown_outcome`, `unknownSince ← at`; `attempts ← attempts + {…, unknown, category}`. Resposta fora do contrato a um comando enviado conta como `unknown_ambiguous` (00 §7.2.1).
- **Eventos emitidos**: `ExternalOutcomeUnknown`
- **Resposta**: `idempotencyKey: string`
- **Prazo de ambiguidade**: o limite configurado que abre caso operacional (00 §7.2.5) e o gatilho temporal **[ABERTA: ciclo de comando externo sem suporte no kernel]**; o prazo típico de confirmação depende de B6 (**[A CONFIRMAR: B6]**)

### ResolveUnknownOutcome

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `resolution: confirmed | final_failure | not_received`, `resolvedBy: state_query | human`, `queryInteractionId: id` (opcional), `externalReferenceId: id` (opcional), `partnerCode: string`, `partnerMessage: string`, `justification: string` (opcional), `partial: bool`
- **Pré-condições**:
  - `status = unknown_outcome` → `receivables-gateway/external-command/not-unknown`
  - `resolvedBy = state_query` só se `stateQuerySupported` → `receivables-gateway/external-command/state-query-not-supported`
  - `resolvedBy = state_query` exige `queryInteractionId` → `receivables-gateway/external-command/query-evidence-missing`
  - `resolvedBy = human` exige `justification` não vazia → `receivables-gateway/external-command/justification-missing`
  - `partial = true` só se `partialResultSupported` → `receivables-gateway/external-command/partial-result-not-supported`
- **Efeitos sobre o estado**: `confirmed` → `status ← confirmed`, `confirmedAt ← at`, `externalReferenceId ← entrada`; `final_failure` → `status ← final_failure`; `not_received` → `status ← retryable_failure` **[ABERTA: 04:§4 - saída de `unknown_outcome` quando a consulta prova que o parceiro não recebeu]**
- **Eventos emitidos**: `ExternalCommandConfirmed` ou `ExternalCommandFailed` conforme a resolução; `not_received` não emite
- **Resposta**: `idempotencyKey: string`, `status`

### CancelExternalCommand

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**: `status = created` (nada foi enviado) → `receivables-gateway/external-command/already-sent`. Desfazer um efeito já enviado é outro comando externo (`requestRelease`), não uma transição **[ABERTA: 04:§4 - quando `cancelled` é permitido]**
- **Efeitos sobre o estado**: `status ← cancelled`
- **Eventos emitidos**: nenhum
- **Resposta**: `idempotencyKey: string`

### RecordExternalInteraction

- **Agregado**: `ExternalInteraction`; **cria**
- **Entrada**: `connectorId: id`, `operation: string`, `direction`, `commandId: id` (opcional), `partnerEventId: string` (opcional), `businessDate: string` (opcional), `requestRef`, `requestHash`, `responseRef`, `responseHash` (opcionais), `duplicateOf: id` (opcional, resolvido pelo caso de uso por `FindExternalInteractionByPartnerEvent`)
- **Pré-condições**: ao menos uma referência de payload bruto → `receivables-gateway/external-interaction/raw-payload-missing`
- **Efeitos sobre o estado**: campos ← entrada; `status ← duplicate` quando `duplicateOf` informado, senão `stored`; `normalizationRevision ← 1`; `receivedAt ← at`. O bruto é persistido sempre, antes de deduplicar e de interpretar (04 §7, passos 2 e 3).
- **Eventos emitidos**: nenhum
- **Resposta**: `interactionId: id`, `status`

### ConcludeInteraction

- **Agregado**: `ExternalInteraction`; exige existente
- **Entrada**: `outcome: normalized | uncorrelated | schema_mismatch`, `commandId: id` (opcional), `mismatchKind` e `mismatchDetail: string` (obrigatórios em `schema_mismatch`), `partnerDeclaredVersion: string` (opcional)
- **Pré-condições**:
  - `status = stored` → `receivables-gateway/external-interaction/already-concluded`
  - `schema_mismatch` exige `mismatchKind` e `mismatchDetail` não vazio → `receivables-gateway/external-interaction/mismatch-detail-missing`
- **Efeitos sobre o estado**: `status ← outcome`; `commandId ← entrada` quando informado; campos de quebra ← entrada
- **Eventos emitidos**: `ExternalSchemaMismatchDetected` quando `schema_mismatch`; `uncorrelated` abre caso operacional **[ABERTA: 92:§4 sem evento para interação não correlacionada]**
- **Resposta**: `interactionId: id`, `status`
- **Suspensão da operação afetada**: após `schema_mismatch` o caso de uso executa `SuspendOperation` com `cause = schema_mismatch`, em segunda UoW **[ABERTA: ciclo de comando externo sem suporte no kernel - orquestração entre dois agregados]**

### ReprocessInteraction

- **Agregado**: `ExternalInteraction`; exige existente
- **Entrada**: `justification: string`
- **Pré-condições**:
  - `status` é `normalized`, `uncorrelated` ou `schema_mismatch` → `receivables-gateway/external-interaction/not-concluded`
  - `justification` não vazia (operação de backoffice, 04 §6) → `receivables-gateway/external-interaction/justification-missing`
  - referência do bruto presente → `receivables-gateway/external-interaction/raw-payload-missing`
- **Efeitos sobre o estado**: `status ← stored`, `normalizationRevision ← normalizationRevision + 1`. Reexecuta só a normalização e a publicação; nunca reenvia comando externo.
- **Eventos emitidos**: nenhum
- **Resposta**: `interactionId: id`, `normalizationRevision: int`
- **Lacunas**: 04 §6 diz que o reprocessamento "produz os mesmos eventos, deduplicados pelo inbox", e 92 §5.3 exige novo `eventId` na correção **[ABERTA: 04:§6 × 92:§5.3]**; reprocessamento que muda a leitura de uma resposta já aplicada a comando terminal **[ABERTA: 04:§6]**; o registro da execução não tem evento no catálogo, o Ledger não é avisado **[ABERTA: 92:§4]**

### LinkExternalReference

- **Agregado**: `ExternalReference`; **inicializa se ausente**
- **Entrada**: `providerId: id`, `system`, `referenceType`, `reference: string`, `normalizedReference: string`, `subjectType: string`, `subjectId: id`, `asOf: instant`, `observedVersion: string` (opcional)
- **Pré-condições**:
  - `reference` não vazio → `receivables-gateway/external-reference/reference-empty`
  - referência já ligada a outro objeto → `receivables-gateway/external-reference/already-linked`
- **Efeitos sobre o estado**: quando ausente: campos ← entrada; quando presente para o mesmo objeto: `asOf` e `observedVersion` ← entrada se `asOf` for mais recente
- **Eventos emitidos**: nenhum
- **Resposta**: `referenceKey: string`

## Eventos de domínio

Os seis são publicados. O catálogo `92 §3` não define o `data` de nenhum evento do Gateway: os campos abaixo são derivados do agregado e das convenções de 92 §2 (motivo de falha preserva o código e o texto originais do parceiro).

### ExternalCommandSubmitted

- **Campos**: `idempotencyKey: string ← ExternalCommand.idempotencyKey`; `connectorId: id`; `operation: string`; `businessKey: string`; `attemptSequence: int ← attempts.sequence`; `at: instant ← at`

### ExternalCommandAcknowledged

- **Campos**: `idempotencyKey`, `connectorId`, `operation`, `businessKey` (como acima); `externalReferenceId: id` (opcional); `at: instant ← at`

### ExternalCommandConfirmed

- **Campos**: `idempotencyKey`, `connectorId`, `operation`, `businessKey`; `externalReferenceId: id`; `confirmationSource: callback | polled_query | file`; `partial: bool`; `interactionId: id`; `confirmedAt: instant ← confirmedAt`

### ExternalCommandFailed

- **Campos**: `idempotencyKey`, `connectorId`, `operation`, `businessKey`; `category: final_validation | final_business | final_authorization`; `partnerCode: string`; `partnerMessage: string`; `interactionId: id`; `at: instant ← at`

### ExternalOutcomeUnknown

- **Campos**: `idempotencyKey`, `connectorId`, `operation`, `businessKey`; `failureCategory: unknown_sent | unknown_ambiguous`; `attemptCount: int ← size(attempts)`; `lastAttemptAt: instant`; `stateQuerySupported: bool ← ExternalCommand.stateQuerySupported`; `requiresHumanResolution: bool ← não stateQuerySupported`; `unknownSince: instant`

### ExternalSchemaMismatchDetected

- **Campos**: `interactionId: id`; `connectorId: id`; `operation: string`; `mismatchKind`; `mismatchDetail: string`; `partnerDeclaredVersion: string`; `at: instant ← at`

## Consultas

Locais, fora da UoW:

- `FindConnector` - filtros: `connectorKey`; cardinalidade: `one`
- `FindEffectiveCapabilityProfile` - filtros: `connectorId`, `effectiveAt`; cardinalidade: `one`
- `FindCapabilityProfile` - filtros: `connectorId`, `version`; cardinalidade: `one`
- `FindQueryAuthorizationByOrganization` - filtros: `organizationId`; cardinalidade: `many`
- `FindExternalCommand` - filtros: `idempotencyKey`; cardinalidade: `one`
- `FindExternalCommandsByBusinessKey` - filtros: `connectorId`, `businessKey`; cardinalidade: `many`
- `FindExternalCommandsByStatus` - filtros: `connectorId`, `status`; cardinalidade: `many` (acompanhamento de `unknown_outcome`)
- `FindExternalInteraction` - filtros: `interactionId`; cardinalidade: `one`
- `FindExternalInteractionByPartnerEvent` - filtros: `connectorId`, `partnerEventId`; cardinalidade: `one` (deduplicação)
- `FindExternalInteractionsByCommand` - filtros: `commandId`; cardinalidade: `many`
- `FindExternalReference` - filtros: `referenceKey`; cardinalidade: `one`
- `FindExternalReferencesBySubject` - filtros: `subjectType`, `subjectId`; cardinalidade: `many`

Leituras externas pelas portas canônicas (04 §3). Cada uma corre em ordem: guarda local, chamada ao adaptador (bloco `provider`), `RecordExternalInteraction`. Toda saída carrega `asOf`, `providerId` e a referência externa. Leitura que expira por tempo é erro recuperável (00 §7.4). **[ABERTA: ciclo de comando externo sem suporte no kernel - a leitura registra interação, mas consulta do kernel é somente leitura]**

- `ReceivablesQueryPort`: `DiscoverSchedule`, `QueryAsset`, `QueryOwnership`, `QueryEncumbrances` - guarda: `QueryAuthorization` ativa para a organização e o tipo de ativo → `receivables-gateway/query-authorization/consent-not-active`; perfil suporta a operação e a operação não está suspensa → `receivables-gateway/connector/capability-not-enabled`
- `AuthorizationPort` (leituras): `QueryOptInStatus`, `ListAuthorizations` - guarda de capacidade; sem guarda de consentimento, porque o opt-in externo antecede a autorização de consulta
- `RegulatoryEffectsPort` (leituras): `QueryEffect` (obrigatória para resolver `unknown_outcome`), `QueryEffectsByAsset` - guarda de capacidade
- `SettlementQueryPort`: `QuerySettlements`, `QuerySettlementByAsset` - guarda de capacidade
- `ExternalEvidencePort`: `FetchEvidence` - guarda de capacidade; `storeInteraction` é o comando `RecordExternalInteraction`
- As escritas das portas (`activateOptIn`, `revokeOptIn`, `submitEffect`, `requestRelease`) são operações de `ExternalCommand`.

## Relações entre agregados

- `Connector.partnerId` → `Partner` (Partner Management, outro contexto)
- `CapabilityProfile.connectorId` → `Connector`
- `QueryAuthorization.organizationId` → `Organization` (Identity, outro contexto)
- `ExternalCommand.connectorId` → `Connector`
- `ExternalCommand.externalReferenceId` → `ExternalReference`
- `ExternalCommand.attempts.interactionId` → `ExternalInteraction`
- `ExternalInteraction.connectorId` → `Connector`
- `ExternalInteraction.commandId` → `ExternalCommand`
- `ExternalReference.subjectId` → objeto interno de outro contexto (ativo, efeito, autorização, liquidação)

## Integração

- **Publica** (integration events, em `apps/backend/gateway/contract/proto/company/gateway/event/v1/`, package `company.gateway.event.v1`): `ExternalCommandSubmitted` (`external_command_submitted.proto`), `ExternalCommandAcknowledged` (`external_command_acknowledged.proto`), `ExternalCommandConfirmed`, `ExternalCommandFailed`, `ExternalOutcomeUnknown`, `ExternalSchemaMismatchDetected`. Consumidores segundo 92 §4: `Submitted` → Ledger; `Confirmed` → Effects, Funding, Ledger; `Failed` → Effects, Ops, Ledger; `OutcomeUnknown` → Effects, Funding, Ops, Ledger; `SchemaMismatchDetected` → Partners, Ops, Ledger. Consent não consta como consumidor de `Confirmed`, `Failed` e `OutcomeUnknown`, mas 03 §5.1 depende do desfecho de `activateOptIn` **[ABERTA: 92:§4 × 03:§5.1]**.
- **Consome** (disposições de consumo: FND-04 §6.4; efeito de broker só depois do retorno da transação):
  - `company.consent.event.v1.ConsentGranted` → `GrantQueryAuthorization`. Mapa: `consentId`, `organizationId`, `version` → `consentVersion`, `scope.assetTypes` → `assetTypes`, `scope.periodFrom` → `periodFromDate`, `scope.periodUntil` → `periodUntilDate`. Só `consentType = RECEIVABLES_OPT_IN`; os demais são descartados.
  - `company.consent.event.v1.ConsentRevoked` → `RevokeQueryAuthorization`. Mapa: `consentId`, `effectiveAt`. Consentimento ainda desconhecido: disposição de espera ou falha de FND-04 §6.4.
  - `company.partner.event.v1.CapabilitySuspended` → `SuspendOperation` com `cause = partner`. Mapa assumido: `partnerId` e `environment` → `connectorId`, `operation`, `assetType`. O escopo "produto" não existe no Gateway e é ignorado. **[ABERTA: 92:§3 não define o `data` de `CapabilitySuspended`]**
  - `company.partner.event.v1.CapabilityEnabled` → `ResumeOperation`. Mesmo mapa. **[ABERTA: 92:§3]**
- **Chamadas síncronas de entrada** (gRPC): Consent (`AuthorizationPort`), Portfolio (`ReceivablesQueryPort`), Effects (`RegulatoryEffectsPort`), Settlement (`SettlementQueryPort`), Contracting e Funding (`ExternalEvidencePort`).

## Borda e persistência

- **Borda**: só gRPC, `company.gateway.service.v1.GatewayService`, um RPC por comando e por consulta (os 17 comandos acima, as 12 consultas locais e as leituras externas das portas), atrás da cadeia de interceptors do kernel. Webhook de entrada, mTLS de origem e a resposta em menos de 200 ms (p95) são do `bff` (91 §5.1); o `bff` entrega o bruto ao contexto por `RecordExternalInteraction`.
- **Banco**: database e role `gateway`, schema `public`, tabelas `connectors`, `capability_profiles`, `query_authorizations`, `external_commands`, `external_interactions` e `external_references`, com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra: `capability_profiles.connector_id` e `version` (`FindEffectiveCapabilityProfile`); `query_authorizations.organization_id`; `external_commands.connector_id`, `business_key` e `status`; `external_interactions.connector_id`, `partner_event_id` e `command_id`; `external_references.subject_type` e `subject_id`. O resto do estado vai em `snapshot` `jsonb`. O payload bruto fica fora do banco. **[ABERTA: provider de object storage]**
- **Instante**: inteiro de nanossegundos no domínio. Diverge do design: `00 §8` fala em milissegundo; segue o DMPF. A data de negócio do parceiro é `businessDate`, separada de `receivedAt`.
- **Diverge do design**: o doc 92 §1 (envelope JSON, `schemaUri`) cede ao contrato Protobuf e ao CloudEvents montado pelo relay; o broker é Kafka, a confirmar na decisão D4 da fundação (`SPEC-GXGASP24`).

## Políticas transversais

- **Idempotência**: os comandos entram pela inbox de comando do kernel (ADR-056; FND-04 §7.6), com chave declarada na borda; os eventos consumidos têm inbox por `eventId` (ADR-036). A idempotência de saída para o parceiro é a identidade natural de `ExternalCommand`, sem equivalente no kernel **[ABERTA: ciclo de comando externo sem suporte no kernel]**. Resiliência por operação (timeout, backoff só em `retryable_*`, circuit breaker por operação e parceiro, bulkhead, rate limit distribuído - 00 §7.4) é do bloco `provider`; abrir o circuito não tem evento no catálogo **[ABERTA: 00:§7.4]**.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2); os chamadores são serviços internos. Negar em dúvida (fail-closed) para consentimento, capacidade e suspensão.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`) e pelos eventos para o Ledger. Reprocessamento é operação de backoffice com justificativa. Métricas por parceiro, endpoint, código e percentil (PRD 04, RNF).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `gateway` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Simulador de parceiro (04:G12) cobrindo sucesso, falha recuperável, falha definitiva, `unknown_outcome` com e sem consulta de estado e quebra de contrato.
- [ ] Cenários:

```text
DADO um conector sem registro
QUANDO RegisterConnector chega com credentialRef preenchido
ENTÃO o conector nasce sem suspensões e registeredAt = at

DADO qualquer entrada
QUANDO RegisterConnector chega com credentialRef vazio
ENTÃO rejeita com receivables-gateway/connector/credential-ref-empty

DADO um conector ativo
QUANDO SuspendOperation chega para submitEffect
ENTÃO a suspensão entra em suspensions com since = at e as demais operações seguem

DADO um conector com submitEffect suspenso
QUANDO ResumeOperation chega para submitEffect
ENTÃO a suspensão sai de suspensions

DADO uma versão de perfil ainda não declarada
QUANDO DeclareCapabilityProfile chega com uma operação
ENTÃO o perfil é criado imutável com a versão e a effectiveFrom informadas

DADO qualquer conector
QUANDO DeclareCapabilityProfile chega sem operações
ENTÃO rejeita com receivables-gateway/capability-profile/operations-empty

DADO um perfil já declarado na versão 2
QUANDO DeclareCapabilityProfile chega novamente com a versão 2
ENTÃO rejeita com receivables-gateway/capability-profile/already-declared

DADO um consentimento RECEIVABLES_OPT_IN concedido
QUANDO GrantQueryAuthorization chega
ENTÃO a autorização nasce granted com o escopo e o período do consentimento

DADO uma autorização granted
QUANDO RevokeQueryAuthorization chega
ENTÃO a autorização vai a revoked e a consulta passa a falhar fechada

DADO uma autorização revoked
QUANDO DiscoverSchedule é chamado para a organização
ENTÃO rejeita com receivables-gateway/query-authorization/consent-not-active e nenhuma chamada externa sai

DADO uma operação suspensa ou não suportada no perfil vigente
QUANDO QueryAsset é chamado
ENTÃO rejeita com receivables-gateway/connector/capability-not-enabled e nenhuma chamada externa sai

DADO uma chave de idempotência nova e capabilityGate allowed
QUANDO CreateExternalCommand chega
ENTÃO o comando nasce created, durável antes de qualquer envio, com stateQuerySupported e idempotencySupport copiados do perfil

DADO um comando já criado com a mesma chave e o mesmo payloadHash
QUANDO CreateExternalCommand chega de novo
ENTÃO aceita sem alterar e devolve o comando existente, sem segundo efeito externo

DADO uma chave existente
QUANDO CreateExternalCommand chega com payloadHash diferente
ENTÃO rejeita com receivables-gateway/external-command/idempotency-key-conflict

DADO capabilityGate not_enabled
QUANDO CreateExternalCommand chega
ENTÃO rejeita com receivables-gateway/external-command/capability-not-enabled sem chamada externa

DADO capabilityGate suspended
QUANDO CreateExternalCommand chega
ENTÃO rejeita com receivables-gateway/external-command/operation-suspended

DADO qualquer capacidade habilitada
QUANDO CreateExternalCommand chega com businessKey vazio
ENTÃO rejeita com receivables-gateway/external-command/business-key-empty

DADO qualquer capacidade habilitada
QUANDO CreateExternalCommand chega com payload vazio
ENTÃO rejeita com receivables-gateway/external-command/payload-empty

DADO um comando created
QUANDO RecordCommandSent chega
ENTÃO o comando vai a sent, uma tentativa é registrada e ExternalCommandSubmitted vai à outbox

DADO um comando confirmed ou unknown_outcome
QUANDO RecordCommandSent chega
ENTÃO rejeita com receivables-gateway/external-command/not-sendable

DADO um comando retryable_failure com idempotencySupport none e sem consulta de estado
QUANDO RecordCommandSent chega
ENTÃO rejeita com receivables-gateway/external-command/retry-not-safe

DADO um comando sent
QUANDO RecordCommandAcknowledged chega
ENTÃO o comando vai a acknowledged e ExternalCommandAcknowledged é emitido, sem tratar o aceite como efeito confirmado

DADO um comando created
QUANDO RecordCommandAcknowledged chega
ENTÃO rejeita com receivables-gateway/external-command/not-sent

DADO um comando acknowledged
QUANDO RecordCommandConfirmed chega
ENTÃO o comando vai a confirmed com confirmedAt = at e ExternalCommandConfirmed é emitido com a referência externa

DADO um comando created ou já confirmed
QUANDO RecordCommandConfirmed, RecordCommandFailure ou RecordUnknownOutcome chega
ENTÃO rejeita com receivables-gateway/external-command/not-in-flight

DADO um parceiro com partialResultSupported falso
QUANDO RecordCommandConfirmed chega com partial verdadeiro
ENTÃO rejeita com receivables-gateway/external-command/partial-result-not-supported

DADO um comando sent
QUANDO RecordCommandFailure chega com category retryable_transport
ENTÃO o comando vai a retryable_failure, a tentativa fica no histórico e nenhum evento é emitido

DADO um comando sent
QUANDO RecordCommandFailure chega com category final_business
ENTÃO o comando vai a final_failure e ExternalCommandFailed é emitido com código e texto originais do parceiro

DADO um comando sent
QUANDO RecordUnknownOutcome chega por timeout após o envio
ENTÃO o comando vai a unknown_outcome, unknownSince = at e ExternalOutcomeUnknown é emitido, nunca final_failure

DADO um comando unknown_outcome com stateQuerySupported verdadeiro
QUANDO ResolveUnknownOutcome chega com resolvedBy state_query, queryInteractionId e resolution confirmed
ENTÃO o comando vai a confirmed e ExternalCommandConfirmed é emitido, sem reenvio

DADO um comando unknown_outcome
QUANDO ResolveUnknownOutcome chega com resolution not_received por consulta de estado
ENTÃO o comando vai a retryable_failure e nenhum evento é emitido

DADO um comando sent
QUANDO ResolveUnknownOutcome chega
ENTÃO rejeita com receivables-gateway/external-command/not-unknown

DADO um comando unknown_outcome com stateQuerySupported falso
QUANDO ResolveUnknownOutcome chega com resolvedBy state_query
ENTÃO rejeita com receivables-gateway/external-command/state-query-not-supported

DADO um comando unknown_outcome
QUANDO ResolveUnknownOutcome chega com resolvedBy state_query sem queryInteractionId
ENTÃO rejeita com receivables-gateway/external-command/query-evidence-missing

DADO um comando unknown_outcome
QUANDO ResolveUnknownOutcome chega com resolvedBy human e justification vazia
ENTÃO rejeita com receivables-gateway/external-command/justification-missing

DADO um comando created
QUANDO CancelExternalCommand chega
ENTÃO o comando vai a cancelled e nada é enviado ao parceiro

DADO um comando já enviado
QUANDO CancelExternalCommand chega
ENTÃO rejeita com receivables-gateway/external-command/already-sent

DADO um webhook com payload bruto
QUANDO RecordExternalInteraction chega com requestRef e partnerEventId novo
ENTÃO a interação nasce stored com normalizationRevision 1, antes de qualquer interpretação

DADO um partnerEventId já visto
QUANDO RecordExternalInteraction chega com duplicateOf
ENTÃO a interação nasce duplicate, o bruto é preservado e nada é reprocessado

DADO qualquer entrada
QUANDO RecordExternalInteraction chega sem requestRef nem responseRef
ENTÃO rejeita com receivables-gateway/external-interaction/raw-payload-missing

DADO uma interação stored
QUANDO ConcludeInteraction chega com outcome normalized e commandId
ENTÃO a interação vai a normalized ligada ao comando e o bruto permanece intacto

DADO uma interação stored com resposta fora do contrato
QUANDO ConcludeInteraction chega com outcome schema_mismatch, mismatchKind e mismatchDetail
ENTÃO a interação vai a schema_mismatch e ExternalSchemaMismatchDetected é emitido

DADO uma interação já concluída
QUANDO ConcludeInteraction chega
ENTÃO rejeita com receivables-gateway/external-interaction/already-concluded

DADO uma interação stored
QUANDO ConcludeInteraction chega com schema_mismatch sem mismatchDetail
ENTÃO rejeita com receivables-gateway/external-interaction/mismatch-detail-missing

DADO uma interação normalized
QUANDO ReprocessInteraction chega com justificativa
ENTÃO a interação volta a stored com normalizationRevision + 1 e nenhum comando externo é reenviado

DADO uma interação ainda stored
QUANDO ReprocessInteraction chega
ENTÃO rejeita com receivables-gateway/external-interaction/not-concluded

DADO uma interação normalized
QUANDO ReprocessInteraction chega sem justificativa
ENTÃO rejeita com receivables-gateway/external-interaction/justification-missing

DADO uma referência externa ainda não ligada
QUANDO LinkExternalReference chega
ENTÃO o vínculo nasce com reference exatamente como recebida e normalizedReference derivada

DADO uma referência já ligada a outro objeto
QUANDO LinkExternalReference chega para um objeto diferente
ENTÃO rejeita com receivables-gateway/external-reference/already-linked

DADO qualquer entrada
QUANDO LinkExternalReference chega com reference vazia
ENTÃO rejeita com receivables-gateway/external-reference/reference-empty
```

## Escopo fora

- **REST do bff**: recepção de webhook (verificação de origem, resposta em menos de 200 ms, absorver e enfileirar) e qualquer rota pública são do `bff` (91 §5.1 e §6); o contexto serve só gRPC.
- **Adaptadores reais de registradora e escriturador**: dependem de #5 (mapeamento de provedores, códigos de erro, homologação); esta versão entrega portas, taxonomia, suspensão, webhook, replay e o simulador de parceiro.
- **Verdade de negócio do portfólio e decisão de elegibilidade**: pertencem a Portfolio e Eligibility; o Gateway só devolve posições com proveniência.
- **Decisão de elegibilidade do parceiro e homologação**: Partner Management; o Gateway apenas aplica `CapabilityEnabled` e `CapabilitySuspended`.
- **Caso operacional por `schema_mismatch` ou interação não correlacionada**: Backoffice (17); o Gateway só publica o fato.
- **Fases posteriores**: escriturador (fase 2) e múltiplas infraestruturas simultâneas (fase 4, B14); recebíveis futuros dependem de B11.
- **Retenção do payload bruto**: sem política até 04:G11 e #8 (retenção); o armazenamento de objetos depende de provider ainda não verificado.
- **Composition root**: o esqueleto de `app/`, o `cmd/` com `--role` e o `project.json` nascem do generator; o harness escreve o conteúdo de domínio dentro deles.
