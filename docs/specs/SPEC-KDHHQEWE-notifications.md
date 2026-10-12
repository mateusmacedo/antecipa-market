---
id: SPEC-KDHHQEWE
slug: notifications
title: Bounded context — notifications
stage: backlog
priority: P2
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6]
ticket_url: https://linear.app/mmda/issue/DEVS-82/spec-kdhhqewe-bounded-context-notifications
subtask_urls: []
created: 2026-10-12
---

# SPEC-KDHHQEWE: Bounded context — notifications

## Identidade

- **bounded_context**: `notifications`
- **name**: `notify`
- **Linguagem ubíqua**:
  - *Modelo* (`NotificationTemplate`) — texto versionado por tipo, canal e idioma.
  - *Regra* (`NotificationRule`) — liga um evento de domínio a destinatários, canais, urgência e condições; pode ser silenciosa por decisão registrada.
  - *Pedido de notificação* (`NotificationRequest`) — intenção de notificar por um evento, resolvida em entregas.
  - *Entrega* (`NotificationDelivery`) — envio por canal e destinatário, com estado.
  - *Preferência* (`RecipientPreference`) — escolha do usuário por categoria; não vale para notificação de segurança nem para obrigação contratual.
  - *Ponto de contato* (`ContactPoint`) — endereço de um canal, com verificação, quarentena e histórico.
  - *Notificação de segurança* — alerta por canal independente do possivelmente comprometido (alteração de conta, novo dispositivo, revogação).
  - *Quarentena de contato* — contato recém-adicionado não recebe alerta de segurança por um prazo configurado (16 §3.1).
  - *Silêncio* — evento sem notificação declarado na regra, nunca esquecido (16 §2, princípio 5).
  - Canais: `EMAIL`, `SMS`, `PUSH`, `WEBHOOK`, `IN_APP`; os valores enumerados do design são preservados literalmente.

## Origem e bloqueios externos

- **Fontes**: `docs/design/16-design-notificacoes.md` §1 a §8 (contexto sem PRD); `docs/design/94-seguranca-threat-model.md` C2.4 e C8.3; `docs/design/92-schemas-eventos.md` §3 e §4; `docs/design/95-personas-jornadas.md` (J3, A10 e matriz); `docs/design/91-contratos-api.md` §5.2 (webhooks ao cedente); `docs/resumo-executivo-apuracao.md` (A6, D5).
- **Situação**: `desbloqueado` — nenhuma lacuna `#1` a `#12` trava o núcleo (regras, pedidos, entregas, contatos, preferências); o sacado (A6/D5) e a redação jurídica (16:C3) travam partes do conteúdo.
- **Fase de entrega**: 1 (inferida).
- **Dependências**: upstream duro `identity` (`SPEC-7KW0HHF6`: destinatários, papéis e usuários). Upstream brando: `marketplace`, `contracting`, `effects`, `funding`, `consent` e `kyb`, cujos eventos disparam notificação (fonte: 16 §4). `depends_on` declara só a fundação e a aresta dura reduzida (`identity`); as arestas de retroalimentação, em que os demais contextos alimentam as notificações, não entram.
- **Bloqueios externos**:

| ID | O que trava | Parte do contexto afetada |
| --- | --- | --- |
| A6, D5, 16:C5 | Obrigação legal de notificar a cessão ao sacado e entrada do sacado no escopo | Destinatários da matriz (`ContractSigned`, "todas as partes"); ator A10 de 95 |
| 16:C3 | Redação editorial e jurídica dos textos | Conteúdo de `NotificationTemplate` |
| 16:C1 | Limiares de escalonamento sem base empírica | Escalonamento por tempo (16 §5.1) |
| 16:C2 | Provedor de e-mail e SMS não escolhido | Entrega real nos canais |
| 16:C4 | Prazo de quarentena de contato não definido | `ContactPoint` utilizável para alerta de segurança |
| `#1`, A1 | Autorização do BCB (existencial, transversal) | Contexto inteiro |

- **Decisões de produto abertas**: sacado no escopo (95 recomenda fora do MVP, A10/D5); valores de limite por destinatário e de agrupamento em resumo (16 §6.2 não os dá).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída. O núcleo não depende de bloqueio externo; 16:C3 trava os textos finais e 16:C2 o provedor, não a construção do modelo.

## Agregados

Nota de modelagem: `tenantId` não é campo dos agregados — o escopo vem do `ExecutionContext` (ADR-051). O doc 90 não detalha este contexto; os campos são o mínimo derivado de 16 §3, §5 e §6.

### NotificationTemplate

- **Identidade**: `generated`
- **Campos**:
  - `templateType: string`
  - `channel: EMAIL | SMS | PUSH | WEBHOOK | IN_APP`
  - `language: string`
  - `version: int`, `supersedesId: id` (opcional) — versionamento explícito (90 §1.5)
  - `subject: string` (opcional), `body: string`
  - `contentFields: list<string>` — coleção indispensável: os campos de dado que o texto usa
  - `effectiveFrom: instant`
- **Estados**: nenhum — nova redação é nova versão.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - O conteúdo só usa os campos permitidos de 16 §6.3: valor da operação, prazo, estado, nome do financiador, identificador da operação e ligação para o app. Dados bancários completos, documento de identificação, anexo com dossiê e token de acesso em texto são proibidos — `notifications/notification-template/forbidden-content`.
  - A redação sem código técnico, com prazo declarado, é revisão editorial e jurídica, que o domínio não verifica **[A CONFIRMAR: 16:C3]** **[ABERTA: 16:C3 — fluxo de revisão e aprovação do texto]**.

### NotificationRule

- **Identidade**: `natural` por `eventType`
- **Campos**:
  - `eventType: string` — nome do evento (92 §4) ou do gatilho da matriz de 16 §4
  - `silent: bool`
  - `recipientRoles: list<string>` — coleção indispensável: papéis de 95 (`OPERATOR`, `APPROVER`, `ORG_ADMIN`) ou `CONTRACT_PARTIES` ("todas as partes"; o sacado só entra se a obrigação de notificar a cessão existir **[A CONFIRMAR: A6, 16:C5]** e se D5 o puser no escopo **[ABERTA: D5]**)
  - `recipientSelection: current-contacts | previous-contacts`
  - `channels: list<EMAIL | SMS | PUSH | WEBHOOK | IN_APP>`
  - `urgency: low | medium | high | critical` — Baixa, Média, Alta, Crítica de 16 §4
  - `security: bool`
  - `escalation: list<EscalationStep>` (opcional) — cada passo: `afterDuration: int` (nanossegundos no estado) e `action: explain-by-email | update-by-email | active-contact` (16 §5.1) **[ABERTA: 16:C1 — os limiares de 2 h, 8 h e 24 h são exemplos sem base empírica]**
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Regra silenciosa não tem destinatários nem canais; regra não silenciosa tem ao menos um de cada; `previous-contacts` exige `security` — `notifications/notification-rule/inconsistent-rule`.
  - Silêncio é decisão consciente: todo evento que a matriz de 16 §4 cobre tem regra, silenciosa ou não.

### NotificationRequest

- **Identidade**: `natural` por `requestKey` (`sourceEventId` mais o passo de escalonamento)
- **Campos**:
  - `sourceEventId: string`, `escalationStep: int` (0 = entrada no estado)
  - `eventType: string`, `urgency: low | medium | high | critical`
  - `recipientOrganizationId: id`
  - `correlationId: id`, `occurredAt: instant`, `requestedAt: instant`
- **Estados**: nenhum — o pedido se resolve em entregas (`NotificationDelivery`), por identidade.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Uma notificação por evento e destinatário: a identidade natural faz a repetição do evento não reenviar (16 §6.2).
  - Evento sem regra — `notifications/notification-request/rule-missing`; evento de regra silenciosa — `notifications/notification-request/silent-rule`.

### NotificationDelivery

- **Identidade**: `natural` por `deliveryKey` (`requestKey`, destinatário e canal)
- **Campos**:
  - `requestKey: string`, `recipientId: id`, `contactPointId: id` (opcional), `templateId: id`
  - `channel: EMAIL | SMS | PUSH | WEBHOOK | IN_APP`
  - `security: bool`
  - `status: REQUESTED | RENDERED | QUEUED | SENT | DELIVERED | READ | SUPPRESSED | FAILED | BOUNCED`
  - `suppressionReason: preference | rule` (opcional)
  - `attemptCount: int`
  - `requestedAt: instant`, `sentAt: instant` (opcional), `updatedAt: instant`
- **Estados**: os de `status` (16 §6.1).
- **Transições permitidas**:
  - (novo) → `REQUESTED` — por `CreateNotificationDelivery`
  - `REQUESTED` → `RENDERED` → `QUEUED` → `SENT` → `DELIVERED` → `READ` — por `AdvanceNotificationDelivery`, um passo por vez
  - `REQUESTED` | `RENDERED` → `SUPPRESSED` — preferência ou regra
  - `QUEUED` | `SENT` → `FAILED`; `SENT` → `BOUNCED`
  - `FAILED` → `QUEUED` — retentativa até 24 h, com espera crescente (16 §6.2)
- **Invariantes de estado**:
  - Transição fora da tabela — `notifications/notification-delivery/invalid-transition`.
  - Entrega de notificação de segurança nunca vai a `SUPPRESSED` por preferência (16 §6.2) — `notifications/notification-delivery/security-not-suppressible`.
  - Retentativa só dentro de 24 h desde `requestedAt` — `notifications/notification-delivery/retry-window-elapsed`. A espera crescente e o teto de tentativas não têm valor **[ABERTA: 16:§6.2 — curva da espera]**.
  - Ordem entre canais não é garantida; cada mensagem é autocontida (16 §6.2).

### RecipientPreference

- **Identidade**: `natural` por `preferenceKey` (`userId` e `category`)
- **Campos**:
  - `userId: id`, `category: string`
  - `enabledChannels: list<EMAIL | SMS | PUSH | WEBHOOK | IN_APP>`
  - `digest: bool` — eventos de baixa urgência agrupados em resumo periódico
  - `updatedAt: instant`
- **Estados**: nenhum.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**: categoria de segurança ou de obrigação contratual não é desativável — `notifications/recipient-preference/mandatory-category-locked`.

### ContactPoint

- **Identidade**: `generated`
- **Campos**:
  - `ownerType: user | organization`, `ownerId: id`
  - `channel: EMAIL | SMS | PUSH | WEBHOOK | IN_APP`
  - `value: string` — classe `PII`
  - `verifiedAt: instant` (opcional), `verificationMethod: string` (opcional)
  - `addedAt: instant`
  - `status: active | deactivated`
  - `deactivatedAt: instant` (opcional), `deactivationReason: string` (opcional)
  - `subscribedEventTypes: list<string>` (só `WEBHOOK`) — subconjunto configurável por tipo (16 §7)
- **Estados**: `active`, `deactivated`.
- **Transições permitidas**:
  - (novo) → `active` — por `RegisterContactPoint`
  - `active` → `deactivated` — por `DeactivateContactPoint`; o histórico é preservado
- **Invariantes de estado**:
  - `value` não vazio — `notifications/contact-point/value-empty`.
  - Verificar ou desativar exige `active` — `notifications/contact-point/not-active`.
  - `usableForSecurityAlerts` (16 §3.1) é derivado: `verifiedAt` presente e `at ≥ addedAt + quarentena`; não é persistido, porque depende do tempo. Diverge do design: 16 §3.1 o lista como campo **[ABERTA: 16:C4 — prazo da quarentena]**.
  - Segredo de assinatura do webhook, por organização (16 §7): fora do agregado **[ABERTA: 16:§7 — dono e rotação do segredo]**.

## Comandos (UPRs)

### DefineNotificationTemplate

- **Agregado**: `NotificationTemplate`; **cria**
- **Entrada**: `templateType: string`; `channel`; `language: string`; `version: int`; `supersedesId: id`; `subject: string`; `body: string`; `contentFields: list<string>`; `effectiveFrom: instant`
- **Pré-condições**:
  - `contentFields` contido nos campos permitidos de 16 §6.3 → `notifications/notification-template/forbidden-content`
- **Efeitos sobre o estado**: campos ← entrada
- **Eventos emitidos**: nenhum
- **Resposta**: `templateId: id`

### ConfigureNotificationRule

- **Agregado**: `NotificationRule`; **inicializa se ausente**
- **Entrada**: `eventType: string`; `silent: bool`; `recipientRoles: list<string>`; `recipientSelection`; `channels: list<EMAIL | SMS | PUSH | WEBHOOK | IN_APP>`; `urgency`; `security: bool`; `escalation: list<EscalationStep>`
- **Pré-condições**:
  - silenciosa sem destinatários e canais; não silenciosa com ao menos um de cada; `previous-contacts` só com `security` → `notifications/notification-rule/inconsistent-rule`
- **Efeitos sobre o estado**: campos ← entrada; quando presente, a regra é substituída
- **Eventos emitidos**: nenhum
- **Resposta**: `eventType: string`

### RequestNotification

- **Agregado**: `NotificationRequest`; **inicializa se ausente**
- **Entrada**: `sourceEventId: string`; `escalationStep: int`; `eventType: string`; `recipientOrganizationId: id`; `correlationId: id`; `occurredAt: instant`; `rule` (instantâneo da `NotificationRule` do `eventType`, resolvido pelo caso de uso)
- **Pré-condições**:
  - existe regra para o `eventType` → `notifications/notification-request/rule-missing`
  - a regra não é silenciosa → `notifications/notification-request/silent-rule`
- **Efeitos sobre o estado**: quando ausente: campos ← entrada; `urgency ← rule.urgency`; `requestedAt ← at`. Quando presente: nenhum — repetição do evento não reenvia.
- **Eventos emitidos**: nenhum
- **Resposta**: `requestKey: string`

### CreateNotificationDelivery

- **Agregado**: `NotificationDelivery`; **inicializa se ausente**
- **Entrada**: `requestKey: string`; `recipientId: id`; `contactPointId: id`; `channel`; `templateId: id`; `security: bool`
- **Pré-condições**: nenhuma além da forma da entrada.
- **Efeitos sobre o estado**: quando ausente: `status ← REQUESTED`; demais campos ← entrada; `attemptCount ← 0`; `requestedAt ← at`; `updatedAt ← at`. Quando presente: nenhum.
- **Eventos emitidos**: nenhum
- **Resposta**: `deliveryKey: string`

### AdvanceNotificationDelivery

- **Agregado**: `NotificationDelivery`; exige existente
- **Entrada**: `target: RENDERED | QUEUED | SENT | DELIVERED | READ | SUPPRESSED | FAILED | BOUNCED`; `suppressionReason: preference | rule`
- **Pré-condições**:
  - `target` alcançável do `status` atual → `notifications/notification-delivery/invalid-transition`
  - `target = SUPPRESSED` por `preference` exige `security = false` → `notifications/notification-delivery/security-not-suppressible`
  - `FAILED` → `QUEUED` só com `at − requestedAt ≤ 24 h` → `notifications/notification-delivery/retry-window-elapsed`
- **Efeitos sobre o estado**: `status ← cmd.target`; `updatedAt ← at`; ao entrar em `SENT`: `sentAt ← at`, `attemptCount ← attemptCount + 1`; ao suprimir: `suppressionReason ← cmd.suppressionReason`
- **Eventos emitidos**: `NotificationDispatched` (só ao entrar em `SENT`)
- **Resposta**: `deliveryKey: string`, `status`

### RegisterContactPoint

- **Agregado**: `ContactPoint`; **cria**
- **Entrada**: `ownerType: user | organization`; `ownerId: id`; `channel`; `value: string`; `subscribedEventTypes: list<string>`
- **Pré-condições**:
  - `value` não vazio → `notifications/contact-point/value-empty`
- **Efeitos sobre o estado**: `status ← active`; demais campos ← entrada; `addedAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `contactPointId: id`

### VerifyContactPoint

- **Agregado**: `ContactPoint`; exige existente
- **Entrada**: `verificationMethod: string`
- **Pré-condições**:
  - `status = active` → `notifications/contact-point/not-active`
- **Efeitos sobre o estado**: `verifiedAt ← at`; `verificationMethod ← cmd.verificationMethod`
- **Eventos emitidos**: nenhum
- **Resposta**: `contactPointId: id`

### DeactivateContactPoint

- **Agregado**: `ContactPoint`; exige existente
- **Entrada**: `reason: string`
- **Pré-condições**:
  - `status = active` → `notifications/contact-point/not-active`
- **Efeitos sobre o estado**: `status ← deactivated`; `deactivatedAt ← at`; `deactivationReason ← cmd.reason`
- **Eventos emitidos**: nenhum **[ABERTA: 16:§7 — a desativação de webhook por falha persistente exige notificar por outro canal, e não há evento para isso em 92 §4]**
- **Resposta**: `contactPointId: id`

### SetRecipientPreference

- **Agregado**: `RecipientPreference`; **inicializa se ausente**
- **Entrada**: `userId: id`; `category: string`; `enabledChannels: list<EMAIL | SMS | PUSH | WEBHOOK | IN_APP>`; `digest: bool`; `categoryMandatory: bool` (resolvido pelo caso de uso a partir das regras de segurança e das obrigações contratuais)
- **Pré-condições**:
  - `categoryMandatory` falso → `notifications/recipient-preference/mandatory-category-locked`
- **Efeitos sobre o estado**: campos ← entrada; `updatedAt ← at`; quando presente, a preferência é substituída
- **Eventos emitidos**: nenhum
- **Resposta**: `preferenceKey: string`

## Eventos de domínio

### NotificationDispatched

Proposto em `92` §4 ("Nenhum contexto trata comunicação"); o schema não está em `92` §3 **[ABERTA: 92:§3 — schema de `NotificationDispatched`; os campos são o mínimo derivado de 16 §6.2]**.

- **Campos**:
  - `deliveryKey: string ← NotificationDelivery.deliveryKey`
  - `requestKey: string ← NotificationDelivery.requestKey`
  - `recipientId: id ← NotificationDelivery.recipientId`
  - `channel ← NotificationDelivery.channel`
  - `templateId: id ← NotificationDelivery.templateId`
  - `security: bool ← NotificationDelivery.security`
  - `attemptCount: int ← NotificationDelivery.attemptCount`
  - `at: instant ← at`
- O evento não carrega o texto nem dado sensível (16 §6.3); a prova do que foi enviado é o par `templateId` e versão.

## Consultas

Todas correm fora da UoW.

- `FindNotificationTemplate` — filtros: `templateType`, `channel`, `language`, `asOf`; cardinalidade: `one` (a de maior `version` com `effectiveFrom` não posterior)
- `FindNotificationRuleByEventType` — filtros: `eventType`; cardinalidade: `one`
- `FindNotificationRequest` — filtros: `requestKey`; cardinalidade: `one`
- `FindNotificationDeliveryByRequest` — filtros: `requestKey`; cardinalidade: `many`
- `FindContactPointByOwner` — filtros: `ownerType`, `ownerId`, `channel` (opcional); cardinalidade: `many`
- `FindSecurityContactPoint` — filtros: `ownerId`, `channel`, `beforeInstant`; cardinalidade: `many` (contatos `active`, verificados, fora da quarentena e adicionados antes de `beforeInstant`, para notificar os "contatos anteriores" de `BankAccountChange*`)
- `FindRecipientPreference` — filtros: `userId`, `category`; cardinalidade: `one`

## Relações entre agregados

- `NotificationRule.eventType` → evento de outro contexto (pelo nome do tipo)
- `NotificationRequest.eventType` → `NotificationRule`
- `NotificationRequest.recipientOrganizationId` → organização (`identity`, só identidade)
- `NotificationDelivery.requestKey` → `NotificationRequest`
- `NotificationDelivery.contactPointId` → `ContactPoint`
- `NotificationDelivery.templateId` → `NotificationTemplate`
- `NotificationDelivery.recipientId` → usuário (`identity`, só identidade)
- `RecipientPreference.userId` → usuário (`identity`, só identidade)
- `ContactPoint.ownerId` → usuário ou organização (`identity`, só identidade)
- A cadeia pedido → entregas → avanço é um comando por UoW, encadeada pelo caso de uso; cada comando é idempotente pela identidade natural.

## Integração

- **Publica** (integration events), em `apps/backend/notify/contract/proto/company/notify/event/v1/`, package `company.notify.event.v1`: `NotificationDispatched` (`notification_dispatched.proto`). Consumidor: o ledger (16 §6.2); `92` §4 não declara consumidores para os eventos propostos **[ABERTA: 92:§4 — consumidor de `NotificationDispatched`]**.
- **Consome** (inbox e Kafka, D4 da fundação; disposições de consumo de FND-04 §6.4). Todo evento abaixo dispara `RequestNotification` com o mesmo mapa: `id` → `sourceEventId`; `escalationStep ← 0`; `type` → `eventType` (sem a versão); `correlationid` → `correlationId`; `time` → `occurredAt`; `recipientOrganizationId` vem do payload quando ele traz `organizationId` e, nos demais, de consulta ao contexto dono do contrato ou do efeito **[ABERTA: 92:§3 — `EffectConfirmed` e `DisbursementConfirmed` trazem `contractId` e a organização do financiador, não a do cedente]**. Evento sem regra termina como recepção rejeitada, e evento de regra silenciosa também.
  - **kyb**: `company.kyb.event.v1.VerificationReady`
  - **consent**: `company.consent.event.v1.ConsentGranted`, `company.consent.event.v1.ConsentRevoked`
  - **marketplace**: `company.marketplace.event.v1.OffersPresented`
  - **contracting**: `company.contracting.event.v1.ContractSigned`
  - **effects**: `company.effects.event.v1.EffectConfirmed`, `company.effects.event.v1.EffectRejected`, `company.effects.event.v1.EffectPartiallyConfirmed`, `company.effects.event.v1.EffectOutcomeUnknown`
  - **funding**: `company.funding.event.v1.DisbursementConfirmed`, `company.funding.event.v1.FundingMismatchDetected`
  - **identity**: `company.identity.event.v1.RepresentationRevoked`, `company.identity.event.v1.BankAccountChangeRequested`, `company.identity.event.v1.BankAccountChangeApproved` (a regra usa `previous-contacts`, nunca o contato novo, 16 §4)
- **Inconsistência registrada**: `92` §4 não lista Notificações como consumidor de nenhum evento, mas 16 §4 lista os acima; este contexto segue 16 §4 **[ABERTA: 92:§4 × 16:§4 — consumidores de Notificações]**.
- **Gatilhos de 16 §4 sem evento correspondente em `92` §4**: `FundingRequestExpired` (existe no PRD 07, mas falta no catálogo), "Novo dispositivo" e "Pendência de KYB" **[ABERTA: 16:§4 × 92:§4 — evento de origem desses três gatilhos]**.
- **Eventos silenciosos** (16 §4): `ReconciliationMismatchDetected`, `AssetSettled`, `PartnerInvited` e `AssetBecameStale` têm regra `silent` e nenhuma assinatura de tópico.
- **Escalonamento** (16 §5.1): para `EffectOutcomeUnknown`, uma rotina de tempo chama `RequestNotification` com `escalationStep` crescente enquanto o estado persiste; o desfecho chega por `EffectConfirmed` ou `EffectRejected` e notifica de imediato. Limiares: **[ABERTA: 16:C1]**.
- **Consultas gRPC a outros contextos**: `identity` resolve usuários e papéis da organização (`FindMembershipsByOrganization`); a cópia do instrumento assinado segue por ligação autenticada e com expiração, nunca anexo (16 §6.3), emitida fora deste contexto **[ABERTA: 16:§6.3 — dono da ligação autenticada (`contracting` ou `bff`)]**.

## Borda e persistência

- **Borda**: só gRPC, `company.notify.service.v1.NotifyService`, um RPC por comando (9) e por consulta (7), atrás da cadeia de interceptors do kernel. O REST (preferências, contatos, configuração de webhook de 91 §5.2 e centro de notificações no app) é do `bff` e fica no Escopo fora.
- **Banco**: database e role `notify`, schema `public`, tabelas `notification_templates`, `notification_rules`, `notification_requests`, `notification_deliveries`, `recipient_preferences`, `contact_points`, `inbox` e `outbox` (kernel), com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra:
  - `notification_templates(template_type, channel, language)` — `FindNotificationTemplate`
  - `notification_deliveries(request_key)` — `FindNotificationDeliveryByRequest`; o `deliveryKey` é a chave natural
  - `contact_points(owner_id, channel, added_at)` — `FindContactPointByOwner` e `FindSecurityContactPoint`
  - `notification_rules`, `notification_requests` e `recipient_preferences` usam a chave natural; o resto vai em `snapshot` `jsonb`.
- **Entrega externa**: provedores de e-mail, SMS e push não têm provider verificado no kernel **[ABERTA: provider de envio de e-mail, SMS e push (16:C2)]**.
- **Webhook ao cedente** (16 §7; 91 §5.2): entrega ao menos uma vez, assinatura sobre o corpo bruto com segredo por organização, retentativa até 24 h, desativação após falha persistente e conteúdo sem `PII_SENSITIVE`; modelado como `ContactPoint` de canal `WEBHOOK`.
- **Divergências do design**: sem RLS como mecanismo primário (`tenant_id` e `ExecutionContext`, ADR-051); o envelope JSON e o `schemaUri` de `92` §1 são substituídos por Protobuf em CloudEvents; `instant` é inteiro de nanossegundos (o doc 00 §8 fala em milissegundos).

## Políticas transversais

- **Idempotência**: os comandos correm em `usecase.RunIdempotent` (ADR-056; FND-04 §7.6); o consumo é idempotente por `id` do CloudEvent (inbox, ADR-036; FND-04 §6.4) e a identidade natural de pedido e entrega impede o reenvio (16 §6.2).
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2). Preferência e contato são do próprio usuário ou do `ORG_ADMIN`; regras e modelos são de plataforma.
- **Auditoria**: todo envio gera `NotificationDispatched` para o ledger (16 §6.2); o restante, pela trilha de auditoria do kernel (FND-08; `observability/audit`).
- **Segurança da notificação**: alerta de segurança vai por canal independente aos contatos anteriores, nunca ao contato novo (C2.4); preferência não o desliga; contato novo fica em quarentena (16 §3.1).
- **Conteúdo**: a notificação informa e direciona; o dado sensível fica atrás de autenticação (16 §6.3).
- **Limite e agrupamento**: máximo por destinatário e janela, exceto urgência crítica; baixa urgência agrupada em resumo (16 §6.2) **[ABERTA: 16:§6.2 — valores do limite e do período do resumo]**.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `notify` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Cenários:

```text
DADO um modelo de e-mail com campos de valor, prazo e estado
QUANDO DefineNotificationTemplate chega
ENTÃO o modelo nasce com a versão informada

DADO qualquer entrada
QUANDO DefineNotificationTemplate chega com contentFields que inclui dados bancários completos
ENTÃO rejeita com notifications/notification-template/forbidden-content e nada é escrito

DADO um evento ainda sem regra
QUANDO ConfigureNotificationRule chega com destinatários, canais e urgência
ENTÃO a regra é inicializada para o eventType

DADO uma regra existente
QUANDO ConfigureNotificationRule chega com novos canais
ENTÃO a regra é substituída

DADO qualquer entrada
QUANDO ConfigureNotificationRule chega com silent verdadeiro e canais preenchidos
ENTÃO rejeita com notifications/notification-rule/inconsistent-rule

DADO uma regra não silenciosa para EffectOutcomeUnknown
QUANDO RequestNotification chega com um sourceEventId novo
ENTÃO o pedido nasce com requestedAt = at e urgency da regra

DADO um pedido já existente para o mesmo sourceEventId e passo
QUANDO RequestNotification chega de novo
ENTÃO aceita sem alterar o pedido, e nenhuma entrega nova é criada

DADO um eventType sem regra
QUANDO RequestNotification chega
ENTÃO rejeita com notifications/notification-request/rule-missing

DADO uma regra silenciosa para AssetSettled
QUANDO RequestNotification chega
ENTÃO rejeita com notifications/notification-request/silent-rule

DADO um pedido e um destinatário com contato verificado
QUANDO CreateNotificationDelivery chega
ENTÃO a entrega nasce REQUESTED com attemptCount 0

DADO uma entrega QUEUED
QUANDO AdvanceNotificationDelivery chega com target SENT
ENTÃO a entrega vai a SENT, sentAt = at, attemptCount incrementa e NotificationDispatched vai à outbox

DADO uma entrega REQUESTED
QUANDO AdvanceNotificationDelivery chega com target READ
ENTÃO rejeita com notifications/notification-delivery/invalid-transition

DADO uma entrega de segurança REQUESTED
QUANDO AdvanceNotificationDelivery chega com target SUPPRESSED por preference
ENTÃO rejeita com notifications/notification-delivery/security-not-suppressible

DADO uma entrega FAILED criada há mais de 24 h
QUANDO AdvanceNotificationDelivery chega com target QUEUED
ENTÃO rejeita com notifications/notification-delivery/retry-window-elapsed

DADO um canal e um valor
QUANDO RegisterContactPoint chega
ENTÃO o contato nasce active com addedAt = at e sem verifiedAt

DADO qualquer entrada
QUANDO RegisterContactPoint chega com value vazio
ENTÃO rejeita com notifications/contact-point/value-empty

DADO um contato active
QUANDO VerifyContactPoint chega com verificationMethod
ENTÃO verifiedAt = at

DADO um contato deactivated
QUANDO VerifyContactPoint ou DeactivateContactPoint chega
ENTÃO rejeita com notifications/contact-point/not-active

DADO um contato active
QUANDO DeactivateContactPoint chega com reason
ENTÃO o contato vai a deactivated com deactivatedAt = at e o registro permanece

DADO um usuário sem preferência na categoria
QUANDO SetRecipientPreference chega com categoryMandatory falso
ENTÃO a preferência é inicializada

DADO uma categoria de segurança
QUANDO SetRecipientPreference chega para desativar canais
ENTÃO rejeita com notifications/recipient-preference/mandatory-category-locked

DADO BankAccountChangeRequested consumido
QUANDO a regra usa previous-contacts e a consulta FindSecurityContactPoint roda com beforeInstant = occurredAt
ENTÃO só contatos verificados, fora da quarentena e adicionados antes do evento recebem entrega

DADO um evento repetido na inbox
QUANDO a segunda entrega chega
ENTÃO nenhum novo pedido nem nova entrega nasce e a recepção termina como repetida
```

## Escopo fora

- **REST do `bff`**: centro de notificações no app, preferências, contatos e configuração do webhook de saída (91 §5.2) ficam no `bff`; este contexto serve só gRPC.
- **Notificar o sacado**: depende de A6/D5 e da lacuna 16:C5; se a cessão exigir notificação ao sacado, o ator A10 volta ao escopo e este contexto ganha destinatário e texto novos.
- **Redação final dos textos**: depende da revisão editorial e jurídica (16:C3); o contexto guarda o modelo versionado, não define a redação.
- **Provedores reais de e-mail, SMS e push**: dependem de 16:C2 e de provider no kernel.
- **Contato ativo da operação** (terceiro limiar do escalonamento): é ação humana do backoffice (`ops`), que este contexto só dispara como passo.
- **Ligação autenticada para a cópia do instrumento**: emitida por quem detém o documento (`contracting` ou `bff`).
- **Matriz completa de limites e agrupamento**: sem valores no design; entram com a operação.
