---
id: SPEC-N1X148AW
slug: receivables-gateway
title: Bounded context — receivables-gateway
stage: building
priority: P0
depends_on: [SPEC-GXGASP24, SPEC-9TEQ9EV9]
ticket_url: https://linear.app/mmda/issue/DEVS-84/spec-n1x148aw-bounded-context-receivables-gateway
subtask_urls: [https://linear.app/mmda/issue/DEVS-173, https://linear.app/mmda/issue/DEVS-174]
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
  - *Tentativa de entrega* - uma chamada ao parceiro para um comando externo, registrada pelo kernel no ledger `external_attempt` (ADR-070 do DMPF, item 10), fora do domínio.
  - *Chave de saída* (`OutboundKey`) - chave idempotente enviada ao parceiro, derivada da intenção pelo kernel (`IDM-11`); não é a identidade do agregado.
  - *Intenção de despacho* - mensagem gravada na outbox na UoW que cria o comando; o envio acontece no handler do consumer (`RES-45`).
  - *Resultado desconhecido* (`UNKNOWN_OUTCOME`) - não se sabe se o comando surtiu efeito; resolve-se só por consulta de estado, nunca por reenvio.
  - *Consulta de estado* - operação do parceiro que informa o desfecho de um comando enviado.
  - *Porta canônica* - interface estável que isola os demais contextos do protocolo do parceiro.
  - *Proveniência* - `asOf`, `providerId` e referência externa que toda saída de porta carrega.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/04-prd-receivables-gateway.md`; `docs/design/04-design-receivables-gateway.md` §2 a §7 (capacidades, portas, erros, quebra de contrato, reprocessamento, webhook); `docs/design/00-arquitetura-tecnica.md` §7 (ciclo do comando externo, idempotência, resiliência); `docs/design/90-modelo-de-dados.md` §2.1 (`ExternalReference`); `docs/design/92-schemas-eventos.md` §4; `docs/prd/14-catalogo-contratos-eventos.md`. Normas do kernel DMPF `1.0.0-rc.7`: ADR-070 (ciclo de comando externo de saída, SPEC-5AQHHMJ0), ADR-067 (leitura relacional na UoW com guarda, SPEC-HFDSHFR1), ADR-068 (acesso entre tenants, SPEC-QVDGV6J0) e ADR-056 (idempotência de comando pela inbox). A segunda fatia adota a `1.0.0-rc.8`, que traz a ADR-073 (porta de object storage e provider S3 com Object Lock, SPEC-CWBA4BPV).
- **Situação**: `parcial`, em duas fatias. A primeira constrói já: conector, perfil, autorização de consulta, criação e cancelamento de comando, intenção de despacho e varredura. A segunda depende da porta de object storage do kernel (SPEC-CWBA4BPV, DEVS-95), porque toda interação exige a referência do bruto: envio com captura da resposta, consulta de estado, webhook, replay e leituras externas. A porta saiu na `1.0.0-rc.8` do kernel (SPEC-CWBA4BPV `done` em 2026-11-02), e a segunda fatia (DEVS-174) a adota. Os adaptadores reais dependem ainda do mapeamento de provedores (#5); o simulador de parceiro (04:G12) cobre os testes.
- **Fase de entrega**: 1 (cartão); 2 (escriturador); 4 (múltiplas infraestruturas).
- **Dependências**: duro, Partner Management (`SPEC-9TEQ9EV9`: `CapabilityEnabled`, `CapabilitySuspended`; `IntegrationProfileRegistered` e o consumo de `ExternalSchemaMismatchDetected` chegam pela DEVS-171); duro só para a segunda fatia, a porta de object storage do kernel (`SPEC-CWBA4BPV`, DEVS-95, `done` no kernel e publicada na `1.0.0-rc.8`); brando, Consent & Mandates (`ConsentGranted`, `ConsentRevoked`, que travam a consulta em modo fail-closed). `depends_on` declara a fundação e a aresta dura reduzida. A porta do kernel fica fora dele, na tabela de bloqueios, porque a spec só entra em `building` com todas as dependências `done` e a primeira fatia não usa a porta; a SPEC-9TEQ9EV9 trata a mesma porta assim. Consent depende do Gateway (`AuthorizationPort`), então o consumo de eventos de Consent é aresta de retroalimentação e não entra em `depends_on`. O ciclo de comando externo de saída e a leitura relacional com guarda já estão na `1.0.0-rc.7`, adotada na DEVS-136.
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
| B6 (`docs/resumo-executivo-apuracao.md` §B.2) | Prazo típico e máximo de confirmação de um efeito | Valor de `ambiguityDeadline` dos perfis reais; o simulador declara o próprio |
| SPEC-CWBA4BPV (DEVS-95, kernel) — resolvido na `1.0.0-rc.8` | Porta de object storage com `versionId` e retenção WORM, provider S3 | Gravação do bruto de `ExternalInteraction` |
| DEVS-171 (Partner) | Evento `IntegrationProfileRegistered` e consumo de `ExternalSchemaMismatchDetected` | Cadastro do conector por evento e a camada de suspensão do Partner; o simulador e o RPC `RegisterConnector` cobrem os testes |

- **Decisões de produto abertas**: nenhuma própria. Técnica: retenção do bruto (04:G11).
- **Decisões da triagem** (DEVS-84 e DEVS-122, 2026-10-31):
  1. Tenant único da plataforma na fase 1, como a configuração de parceiros (SPEC-9TEQ9EV9, decisão 2); leitura e escrita de plataforma entre tenants ficam no Escopo fora.
  2. Suspensão por quebra de contrato em camadas: o Gateway suspende a operação na hora; o Partner suspende a capacidade com `schema_mismatch` e exige re-homologação (DEVS-171); o `CapabilityEnabled` posterior levanta a suspensão no Gateway.
  3. O conector nasce do evento `IntegrationProfileRegistered` do Partner (DEVS-171), com `credentialRef ← secretRef`.
  4. O Gateway não consome `CapabilityRolloutChanged`: o rollout é de quem abre a operação (`FindEffectiveCapability`, 13 §4).
  5. Interação não correlacionada emite `ExternalInteractionUncorrelated`, que abre caso no Backoffice.
  6. `stateQuerySupported` efetivo é o mais restritivo entre o perfil do Gateway e a capacidade homologada no Partner (fail-closed).
  7. Reprocessamento nunca altera comando terminal: a interação ganha revisão nova e o caso operacional é aberto por evento.
  8. `ExternalInteractionReprocessed` avisa o Ledger de todo reprocessamento e o Backoffice do que muda a leitura de comando terminal.
  9. Abrir o circuito gera só telemetria, sem evento de domínio.
  10. O tempo do ciclo (reenvio depois da quiescência, consulta de estado e prazo de espera) é da varredura `SweepExternalCommands`, acionada por agendador externo, sem timer no kernel.
  11. O `CapabilityProfile` do Gateway é a fonte do protocolo, dos limites técnicos e do calendário da integração (04 §2.1); o Partner guarda a política comercial e operacional. O perfil é declarado por `DeclareCapabilityProfile`, operado pela plataforma na homologação do conector.
- **Decisões da fatia 2** (DEVS-174, 2026-11-03):
  12. O orçamento de reenvio é `maxSubmissions`, declarado por operação no perfil e copiado ao comando. Sem teto, o ciclo `unknown_outcome` → `FindingAbsent` → `retryable_failure` → `RequestResend` → envio não termina: cada `retryable_failure` encerra o episódio de espera, então o `ambiguityDeadline` não vence, e o `maxQueries` do kernel conta por tentativa. O `retry` que encontra o orçamento esgotado leva a `final_failure` com `resend_budget_exhausted`, que o `ExternalCommandFailed` já leva a Effects, Ops e Ledger. A ausência confirmada reabre a série da `businessKey`, como o cancelamento.
  13. Conflito e saturação da observação ficam visíveis. `ResolveUnknownOutcome` lê o `ports.Observed` do kernel: `ObserveConflict` → `receivables-gateway/external-command/observation-conflict`; `ObserveSaturated` → `receivables-gateway/external-command/observation-saturated`; `ObserveDuplicate` → `Accepted` com o status atual. As rejeições voltam como desfecho, nunca como erro, para a transação fazer commit: o registro do ledger sobrevive e o replay da chave devolve a mesma rejeição (`application.RunIdempotent` grava a rejeição na inbox). No callback, no arquivo e na consulta de estado, o `Resolve` corre em UoW própria depois da conclusão da interação; conflito não muda o comando (decisão 7) e fica visível pelo registro no ledger e pela métrica e pelo log do kernel (`ports.MarkExternalConflict`); saturação é só telemetria (`ports.MarkExternalSaturation`). Para a linha do ledger não saturar antes do teto de consultas, `maxQueries` é menor que `ports.MaxAttemptEntries` (64).
  14. O despacho fica retido enquanto a operação está suspensa. O handler de `gateway.dispatch` lê o comando e o conector antes de `application.Dispatch`: comando fora de `created` e `retryable_failure` (cancelado, terminal ou em espera) recebe `Ack` sem envio; operação suspensa não envia, executa `HoldDispatch` e dá `Ack`. A reentrega não serve, porque a suspensão dura até a re-homologação e esgotaria a reentrega na quarentena. A varredura libera o comando retido cuja operação não está mais suspensa por `RequestResend`, que grava a intenção com o mesmo `IntentID`. A suspensão que chega entre a leitura e o envio não desfaz o envio, como no cancelamento.
  15. `FindExternalCommandsByStatus` aceita só os status não terminais (`created`, `acknowledged`, `retryable_failure`, `unknown_outcome`); status terminal é entrada inválida. Os conjuntos não terminais são limitados pelo volume em voo. A relação do kernel não tem ordem nem limite, e o `snapshot` leva o `payload`, então paginação e projeção sem payload ficam como pendência no kernel.
  16. O bruto vive na porta `ports.ObjectStore` do kernel, pelo provider `s3` (ADR-073 do DMPF). A loja tem escopo `tenant`, porque toda chamada corre no tenant da plataforma (decisão 1), cifra `AES256` por padrão ou KMS pelo ARN da chave, e o contexto recebe só `ObjectWriter` e `ObjectReader`, nunca `RetentionManager` nem `ObjectPurger`. Sem retenção até 04:G11: a loja não declara teto e nenhum `Put` leva retenção, então a proteção até lá é a versão do objeto e o privilégio. O bucket é versionado, com Object Lock e sem retenção padrão, provisionado fora do contexto; o `Ready` do provider recusa outro. A chave é `interactions/<connectorId>/<interactionId>/request` ou `.../response`; `requestRef` e `responseRef` são a chave, `requestVersionId` e `responseVersionId` a versão devolvida pelo `Put` cujo registro fez commit, e `requestHash` e `responseHash` o SHA-256 hexadecimal do conteúdo, conferido pela porta na escrita. A porta não tem escrita condicional: repetir o `Put` cria versão nova, e a anterior fica órfã, o que é aceito. `ErrIntegrity` e chave inválida são defeito; loja indisponível ou não pronta é dependência transiente; `ErrPutOutcomeUnknown` não é repetido pela loja, e o fluxo repete com a versão anterior órfã.
  17. A resposta do envio é capturada por um coletor por mensagem. O `Submit` do kernel devolve só classe, observação e erro, então o handler do despacho cria um coletor que o adaptador do envio preenche e o `Apply` lê. O adaptador cunha o `interactionId` de saída a cada chamada, grava o pedido na porta antes de chamar o parceiro e a resposta depois; sem o pedido gravado, não chama o parceiro e devolve `ClassRetryableTransport`. Falha ao gravar a resposta não muda o desfecho: o passo é aplicado sem `responseCapture`, sem interação de saída, e vira métrica.
  18. A normalização é assíncrona. `RecordExternalInteraction` (fora de `duplicate`) e `ReprocessInteraction` gravam na outbox `ExternalInteractionStored` (interno), no canal `gateway.normalizations`. O consumer lê o bruto pela versão, normaliza pelo adaptador da conexão (o simulador de parceiro nos testes), executa `ConcludeInteraction`, que grava o achado na interação, e, depois do commit, entrega ao kernel (`application.Resolve`) o achado gravado quando a leitura traz o desfecho de um comando não terminal, com `Observation.ID = <interactionId>.r<normalizationRevision>`, para o reprocessamento ser observação nova e não `ObserveDuplicate`. Na reentrega de uma revisão já concluída, o consumer não normaliza de novo: usa o achado gravado, então uma versão nova do normalizador não produz observação contraditória para a mesma revisão. A captura de saída (`ExternalResponseCaptured`, canal `gateway.interactions`) só registra a interação; na revisão 1 o desfecho segue o passo capturado (`suspend` → `schema_mismatch` com os campos de quebra da captura; os demais → `normalized`) e nenhuma observação vai ao kernel, porque o `Dispatch` já aplicou o passo. A consulta de estado da varredura grava o bruto e registra a interação, e o achado chega ao comando pelo mesmo consumer.
  19. O webhook entra pelo RPC `ReceivePartnerInteraction`, que o `bff` chama. O RPC leva `connectorKey`, `operation`, `source: callback | file`, `partnerEventId`, `businessDate`, `contentType` e o corpo bruto até `maxPayloadBytes` da operação. O contexto calcula o hash, grava o bruto e executa `RecordExternalInteraction` com `direction = inbound`, deduplicando por `partnerEventId`. A deduplicação não depende de leitura antes do `Save`: o primeiro recebimento usa `interactionId` derivado de `connectorId` e `partnerEventId`, e a chave primária serializa a corrida; o perdedor recebe conflito, relê e grava `duplicate` com id novo e `duplicateOf`. O `bff` responde ao parceiro só depois do commit; com falha, o parceiro reenvia e a deduplicação absorve. A rota REST do `bff` segue no Escopo fora.
  20. As leituras externas são genéricas até #5. Sem o mapeamento de provedores, as respostas não têm forma tipada, então as onze leituras das portas entram por um RPC `ReadExternal`, com `operation` do vocabulário, a guarda da porta e uma resposta com `interactionId`, `asOf`, `providerId`, `externalReference`, `reading` (JSON canônico da leitura normalizada) e `readingHash`. Os RPCs tipados por porta chegam com os adaptadores reais.
  21. O papel `consumer` roda, além dos consumers de `partner.events` e `gateway.events`, o handler de `gateway.dispatch`, os consumers de `gateway.interactions` e `gateway.normalizations` e a varredura de tentativas do kernel (`app.RunSweep` com `SweepExternalAttempts`), com o mesmo corte do `Dispatch` (`app.ExternalBudget.StaleAfter`). `SweepExternalCommands` segue acionado pelo agendador externo. Sem adaptador de parceiro configurado (até #5), o despacho, a consulta de estado, a leitura externa e a normalização falham fechado como dependência indisponível, sem chamada externa; o simulador de parceiro (04:G12) existe só nos testes. A loja do bruto passa pelo `Ready` do provider antes de a borda e o consumer servirem; sem ele, o papel recusa a partida.
  22. O consumo de `ConsentGranted` e `ConsentRevoked` espera o contrato do Consent (SPEC-Z3YR96YS, em `backlog`, sem contexto publicado) e fica numa terceira fatia. Até lá a autorização de consulta entra pelos RPCs `GrantQueryAuthorization` e `RevokeQueryAuthorization`, que já travam a consulta em modo fail-closed.
- **Condição para entrar em `building`**: a primeira fatia entra com a fundação `SPEC-GXGASP24` e o Partner Management `SPEC-9TEQ9EV9` concluídos, os dois já `done`, e o núcleo construível com o simulador de parceiro (04:G12). A segunda fatia esperava a SPEC-CWBA4BPV no kernel, `done` desde 2026-11-02, e entra com a adoção da `1.0.0-rc.8`. Adaptadores reais só depois de #5.

## Agregados

Seis agregados. As tentativas de entrega não são do domínio: o kernel as registra no ledger `external_attempt` (`ports.AttemptLedger`; ADR-070 do DMPF, item 10), e `ExternalCommand` guarda só o desfecho de negócio. Diverge do design: o PRD lista `DeliveryAttempt` no modelo operacional; o histórico de tentativas fica no ledger do kernel, e as interações se ligam ao comando por `commandId`.

### Connector

- **Identidade**: `natural` por `connectorKey` (`<partnerId>:<environment>`)
- **Campos**:
  - `partnerId: id`
  - `environment: development | integration | homologation | production`
  - `credentialRef: string` - referência ao cofre; credencial é segregada por parceiro e ambiente e nunca entra no estado
  - `integrationVersion: int` - versão do perfil de integração do Partner que gerou o cadastro
  - `capabilities: list<EnabledCapability>` - projeção das capacidades habilitadas pelo Partner (`CapabilityEnabled`); coleção indispensável, porque operação nunca habilitada é negada (fail-closed, 13 §4). Elemento: `capabilityId: id`, `operation: string`, `assetType: string`, `stateQuerySupported: bool` (da homologação), `version: int`, `at: instant`
  - `suspensions: list<OperationSuspension>` - coleção indispensável: suspensão é por operação, não por parceiro (04 §5). Elemento: `operation: string` (vazio = conector inteiro), `assetType: string` (vazio = todos), `cause: partner | schema_mismatch`, `capabilityId: id` (vazio em `schema_mismatch`), `version: int` (zero em `schema_mismatch`), `since: instant`
  - `registeredAt: instant`
- **Estados**: nenhum campo de estado; "habilitado" deriva de `capabilities` e "suspenso", de `suspensions`.
- **Transições permitidas**: nenhuma de estado; `capabilities` e `suspensions` mudam por `EnableOperation` e `SuspendOperation`.
- **Invariantes de estado**:
  - `credentialRef` nunca é vazio - `RegisterConnector` rejeita com `receivables-gateway/connector/credential-ref-empty`.
  - Uma operação só é permitida com entrada em `capabilities` para ela (no mesmo `assetType` ou com `assetType` vazio) e sem suspensão que a cubra (fail-closed, 13 §4).
  - Suspensão bloqueia novos comandos e leituras; o acompanhamento de comandos já enviados continua (13 §5).
  - Para a mesma `operation` e o mesmo `assetType`, evento de capacidade com `version` menor que a já aplicada é descartado sem alterar: a `partitionkey` do Partner é o `capability_id`, e versões da mesma série podem chegar fora de ordem.
- **Multi-tenancy**: conector e perfil são da plataforma. Na fase 1 o contexto inteiro vive no tenant único da plataforma, como a configuração de parceiros (SPEC-9TEQ9EV9, decisão 2), e `tenantId` é dado de negócio no comando externo (decisão 1). Leitura e escrita de plataforma entre tenants estão no Escopo fora: o ADR-068 do DMPF permite ler (`ports.PlatformReader`) ou projetar (`ports.Projector`), mas exclui a escrita de plataforma (item 14), e `SuspendOperation` grava no conector.

### CapabilityProfile

- **Identidade**: `natural` por `profileKey` (`<connectorKey>:<version>`)
- **Campos**:
  - `connectorId: id`, `version: int`, `effectiveFrom: instant`
  - `operations: list<OperationCapability>` - coleção indispensável (04 §2.1). Elemento: `operation: string`; `supported: bool`; `protocol: sync_http | async_callback | file_batch | polling`; `stateQuerySupported: bool`; `idempotencySupport: native | key_header | none`; `partialResultSupported: bool`; `assetTypes: list<string>`; `maxItemsPerRequest`, `maxRequestsPerMinute`, `maxPayloadBytes: int`; `businessCalendar`, `cutoffTime`, `processingWindow: string`; `encumbranceCoverage: complete | partial | unavailable`; `positionFreshnessSla: int` (segundos, opcional: ausente quando a fonte não declara, B5); `efficacy: on_submit | by_observation`, `quiescence: int` (ns), `authoritativeAbsence: bool`, `maxQueries: int`, `keyAlphabet: string`, `keyMaxLength: int` - declaração do parceiro do `RES-44` (`ports.ExternalProfile`), com `idempotencySupport` como `KeySemantics` (`native` → `KeyNative`, `key_header` → `KeyHeader`, `none` → `KeyNone`); `queryInterval: int` (segundos; intervalo entre consultas de estado, que é de quem chama pelo ADR-070 do DMPF, item 6); `ambiguityDeadline: int` (segundos; prazo da espera em `acknowledged` ou `unknown_outcome` até escalar a caso operacional, 00 §7.2.5; valor real depende de B6); `maxSubmissions: int` (envios por comando, contando o primeiro: o orçamento do reenvio do `RES-42`, que é de quem chama pelo ADR-070 do DMPF, item 6; decisão 12; valor real dos perfis de produção **[A CONFIRMAR: #5]**)
- **Estados**: nenhum. Cada versão é imutável; mudança de capacidade é nova versão.
- **Transições permitidas**: nenhuma.
- **Invariantes de estado**:
  - Um perfil declara ao menos uma operação - `receivables-gateway/capability-profile/operations-empty`.
  - Uma versão não é redeclarada - `receivables-gateway/capability-profile/already-declared`.
  - A declaração do parceiro de cada operação passa em `application.ValidateExternalProfile` e declara `maxQueries` menor que `ports.MaxAttemptEntries` (decisão 13) - `receivables-gateway/capability-profile/external-profile-invalid`.
  - Operação de escrita (`activateOptIn`, `revokeOptIn`, `submitEffect`, `requestRelease`) declara `ambiguityDeadline` maior que zero - `receivables-gateway/capability-profile/ambiguity-deadline-missing` -, `maxSubmissions` maior que zero - `receivables-gateway/capability-profile/max-submissions-missing` - e, com `stateQuerySupported`, `queryInterval` maior que zero - `receivables-gateway/capability-profile/query-interval-missing`.
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

- **Identidade**: `natural` por `idempotencyKey`, derivada de dados de negócio estáveis: `hash(tenantId, partnerId, operationType, businessKey, payloadVersion)` (00 §7.3). A identidade natural é a idempotência no domínio: a mesma chave não cria segundo comando. A chave enviada ao parceiro é a chave de saída do kernel (`IDM-11`): `application.DeriveOutboundKey` sobre `OutboundIntent{Tenant, Destination = connectorKey, Operation = operation, IntentID = idempotencyKey, Generation = 1}`, no alfabeto e no comprimento do perfil copiado. `Generation` não avança: corrigir depois de rejeição final é versão nova de payload, portanto comando novo (SPEC-ZFDEPFWC, `ResubmitEffect` com `validation_corrected`).
- **Campos**:
  - `tenantId: id` (tenant de negócio que originou o comando; na fase 1 é dado, não escopo, decisão 1), `connectorId: id`, `operation: string`, `assetType: string`, `businessKey: string`, `payloadVersion: int`, `seriesKey: string` (`<connectorKey>:<operation>:<businessKey>`, chave da série de versões)
  - `payload: string` (JSON canônico, opaco ao domínio, classe `CONFIDENTIAL`, vai no `snapshot`), `payloadHash: string`
  - `profile: CommandProfile` - a declaração da operação no perfil vigente, copiada inteira na criação, para que uma versão nova do perfil não mude a chave de um comando em voo: `profileVersion`, `stateQuerySupported` (efetivo: o mais restritivo entre o perfil e a capacidade homologada no Partner, decisão 6), `idempotencySupport`, `partialResultSupported`, `efficacy`, `quiescence`, `authoritativeAbsence`, `maxQueries`, `keyAlphabet`, `keyMaxLength`, `queryInterval`, `ambiguityDeadline`, `maxSubmissions`
  - `status`, `failureCategory: string` (opcional; categoria do último desfecho que não é sucesso), `createdAt: instant`, `submissions: int` (envios aplicados), `lastSubmittedAt: instant` (opcional), `waitingSince: instant` (opcional; início do episódio de espera atual), `acknowledgedNotified: bool` e `unknownNotified: bool` (eventos de espera já emitidos no episódio), `nextQueryAt: instant` (opcional), `resendAfter: instant` (opcional), `dispatchHeldSince: instant` (opcional; despacho retido pela suspensão da operação, decisão 14), `reconciliationRequired: bool`, `ambiguityExpiredAt: instant` (opcional), `confirmedAt: instant` (opcional), `externalReferenceId: id` (opcional)
- **Estados**: `created`, `acknowledged`, `confirmed`, `retryable_failure`, `final_failure`, `unknown_outcome`, `cancelled`. Diverge do design: `SENT` (04 §4) não persiste. No ciclo do kernel o envio e o passo que ele produz são aplicados na mesma UoW de consumo; o fato do envio fica em `submissions`, em `ExternalCommandSubmitted` e no ledger de tentativas. `retryable_failure` é só o estado depois de achado `FindingAbsent` (o parceiro não recebeu): as classes retentáveis do envio não chegam ao domínio (ver `ApplyExternalStep`).
- **Transições permitidas**: entram por comando `CreateExternalCommand`, `CancelExternalCommand`, `HoldDispatch`, `RequestResend`, `ResolveUnknownOutcome` e `ExpireAmbiguity` (`HoldDispatch` pelo handler do despacho; `RequestResend` e `ExpireAmbiguity` pela varredura `SweepExternalCommands`; `HoldDispatch` não muda o status); as transições do envio e da observação são efeitos de `ApplyExternalStep`, chamado pelo kernel com o passo de `application.NextStep` (ADR-070 do DMPF, item 6):
  - (novo) → `created` - `CreateExternalCommand`; `created` → `cancelled` - `CancelExternalCommand`
  - `created`, `retryable_failure` ou `cancelled` (corrida com o envio) → `confirmed`, `acknowledged`, `final_failure` ou `unknown_outcome` - `ApplyExternalStep` com o passo do envio
  - `acknowledged` ou `unknown_outcome` → `confirmed`, `acknowledged`, `retryable_failure`, `final_failure` ou `unknown_outcome` - `ApplyExternalStep` com o passo de uma observação (consulta de estado, callback, arquivo ou reconciliação humana por `ResolveUnknownOutcome`)
- **Invariantes de estado**:
  - `confirmed`, `final_failure` e `cancelled` são terminais. A única exceção é o envio que já tinha começado quando o cancelamento chegou: o fato externo prevalece (ver `CancelExternalCommand`).
  - `acknowledged` não equivale a efeito confirmado: só `confirmed` emite `ExternalCommandConfirmed`.
  - `unknown_outcome` não volta a ser enviado direto: o reenvio exige achado `FindingAbsent` (consulta ou reconciliação), espera a quiescência (`resendAfter`) e usa a mesma chave de saída (`RES-42`).
  - `payload`, `payloadHash`, `payloadVersion` e `profile` não mudam após a criação.
  - Sem consulta de estado, ou no teto de consultas, o passo é `reconcile`: `reconciliationRequired ← true`, e o comando só sai de `unknown_outcome` por `ResolveUnknownOutcome` (00 §7.2.4; `RES-43`).
  - Episódio de espera é a permanência no conjunto {`acknowledged`, `unknown_outcome`}: começa ao entrar no conjunto vindo de fora dele e termina ao sair do conjunto. Ir de um desses estados ao outro (por `FindingPending` ou `FindingInconclusive`) não abre episódio novo. `waitingSince` marca o início do episódio e é zerado, com `ambiguityExpiredAt`, `nextQueryAt`, `reconciliationRequired`, `acknowledgedNotified` e `unknownNotified`, quando o episódio termina; cada episódio tem um prazo só, e a reconciliação pedida num episódio não passa ao seguinte.
  - Confirmação parcial só vale com `partialResultSupported`. Sem ele, o adaptador classifica a resposta do envio como `ClassUnknownAmbiguous` e a observação como `FindingInconclusive`, nunca como aceite ou `FindingApplied`, porque o ledger do kernel não desfaz um achado terminal (inconsistência a investigar, 04 §2.2).

### ExternalInteraction

- **Identidade**: `generated`, cunhada por quem registra a interação (caso de uso, handler do despacho ou varredura) antes do registro e informada em `RecordExternalInteraction`, porque o mesmo id serve de `Observation.ID` e de base da chave de idempotência do comando
- **Campos**:
  - `connectorId: id`, `operation: string`, `direction: outbound | inbound`, `source: submit_response | state_query | callback | file | external_read`, `commandId: id` (opcional), `contentType: string`, `capturedStep: string` (opcional; só na captura de saída, o passo que o `Dispatch` aplicou)
  - `partnerEventId: string` (opcional; identificador do parceiro, chave de deduplicação), `businessDate: string` (opcional; data de negócio informada pelo parceiro)
  - `requestRef: string`, `requestVersionId: string`, `requestHash: string`, `responseRef: string`, `responseVersionId: string`, `responseHash: string` - o bruto vive na porta de object storage do kernel (provider S3, ADR-022 linha 12; SPEC-CWBA4BPV, DEVS-95), nunca por SDK direto, e o domínio guarda referência, `versionId` e hash do conteúdo (ADR-018). O bruto é gravado pela porta antes do registro da interação: pelo RPC de entrada no callback e no arquivo (`ReceivePartnerInteraction`, decisão 19), pela varredura na consulta de estado, pelo caso de uso na leitura externa e pelo adaptador do envio na resposta de um comando (ver `ApplyExternalStep`; decisões 16 e 17).
  - `status`, `normalizationRevision: int`, `readingHash: string` (opcional; hash da leitura normalizada, inclusive o achado sobre o comando), `finding: applied | rejected | absent | pending | inconclusive` (opcional; o achado sobre o comando que a conclusão gravou, decisão 18), `previousOutcome: normalized | uncorrelated | schema_mismatch` e `previousReadingHash: string` (opcionais; leitura anterior ao último reprocessamento), `mismatchKind: structure | missing_field | type | version` (opcional), `mismatchDetail: string` (opcional), `receivedAt: instant`
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

Pré-condições que dependem de outro agregado do contexto (perfil, conector, série de comandos, interação) são lidas dentro da UoW, por `Load` de outro repositório ou pela leitura relacional com guarda do kernel (ADR-067 do DMPF; campo `postgres.Table.Guard` e `Relation.QueryWithin`), como na nota (†) da SPEC-9TEQ9EV9, e entram na decisão como fato. Guardas, todas com valor `string`: `capability_profiles.connector_id`, `external_commands.series_key` e `external_interactions.command_id`.

As transições do comando externo depois da criação não entram pela borda: são efeitos de `ApplyExternalStep`, chamado por `application.Dispatch` (envio, no handler do consumer da intenção de despacho, fora da UoW do caso de uso; `RES-45`), por `application.Abandon` (tentativa órfã fechada pela varredura de tentativas do kernel, `app.RunSweep`, com `ClassUnknownSent` presumida) e por `application.Resolve` (observação de consulta de estado, callback, arquivo ou reconciliação humana) (ADR-070 do DMPF, itens 6, 8 a 10).

O tempo do ciclo é de quem chama, não do kernel (ADR-070 do DMPF, item 6: quiescência, intervalo entre consultas e orçamento de reenvio): o kernel não tem timer de espera nem entrega adiada. Por isso a varredura `SweepExternalCommands`, método de comando comum no tenant da plataforma, acionado por agendador externo (decisão 10), percorre os comandos em espera por `FindExternalCommandsByStatus` e, um por UoW: pede o reenvio dos `retryable_failure` passados de `resendAfter` (`RequestResend`); consulta o estado dos `acknowledged` e `unknown_outcome` com `nextQueryAt` vencido, sem `reconciliationRequired` e com a operação não suspensa, pelo adaptador de `QueryEffect` (`ports.ExternalState.Query`), registra a interação e entrega a observação a `application.Resolve`, com o `interactionId` da consulta como `Observation.ID`; e escala as esperas vencidas (`ExpireAmbiguity`). A varredura recebe uma chave de idempotência por chamada; cada comando interno usa chave determinística derivada de `idempotencyKey`, do comando e do instante que o habilitou (`resend.<idempotencyKey>.<resendAfter>`, `query.<idempotencyKey>.<nextQueryAt>`, que também cunha o `interactionId` da consulta, e `expire.<idempotencyKey>.<waitingSince>`), como as chaves determinísticas dos comandos agendados da SPEC-9TEQ9EV9 (decisão 12). Repetir a varredura é seguro.

### RegisterConnector

- **Agregado**: `Connector`; **inicializa se ausente**
- **Entrada**: `partnerId: id`, `environment`, `credentialRef: string`, `integrationVersion: int`
- **Pré-condições**: `credentialRef` não vazio → `receivables-gateway/connector/credential-ref-empty`
- **Efeitos sobre o estado**: quando ausente: `credentialRef` e `integrationVersion` ← entrada, `registeredAt ← at`, `capabilities ← []`, `suspensions ← []`; quando presente e `integrationVersion` maior: `credentialRef` e `integrationVersion` ← entrada (troca de credencial ou de certificado, 13 §7); quando presente com versão igual ou menor: nenhum
- **Eventos emitidos**: nenhum
- **Resposta**: `connectorKey: string`
- **Origem do cadastro**: consumo de `company.partner.event.v1.IntegrationProfileRegistered` (DEVS-171), com `credentialRef ← secretRef` (decisão 3). O RPC segue na borda, como todo comando.

### SuspendOperation

- **Agregado**: `Connector`; exige existente
- **Entrada**: `operation: string` (vazio = conector inteiro), `assetType: string`, `cause: partner | schema_mismatch`, `capabilityId: id` (vazio em `schema_mismatch`), `version: int` (zero em `schema_mismatch`)
- **Pré-condições**: nenhuma; suspensão repetida da mesma operação e causa é aceita sem alterar; com `cause = partner`, versão menor que a já aplicada para a mesma `operation` e o mesmo `assetType` é descartada sem alterar
- **Efeitos sobre o estado**: `suspensions ← suspensions + {operation, assetType, cause, capabilityId, version, since = at}`
- **Eventos emitidos**: nenhum
- **Resposta**: `connectorKey: string`
- **Origem**: `CapabilitySuspended` do Partner (`cause = partner`) e o consumo do próprio `ExternalSchemaMismatchDetected` (`cause = schema_mismatch`, com `assetType` vazio; decisão 2).

### EnableOperation

- **Agregado**: `Connector`; exige existente
- **Entrada**: `capabilityId: id`, `operation: string`, `assetType: string`, `stateQuerySupported: bool`, `version: int`
- **Pré-condições**: nenhuma; versão menor que a já aplicada para a mesma `operation` e o mesmo `assetType` é descartada sem alterar
- **Efeitos sobre o estado**: `capabilities` passa a ter a entrada de `operation` e `assetType` com os campos da entrada, substituindo a anterior; saem de `suspensions` as entradas de `operation` com o mesmo `assetType` ou com `assetType` vazio, de qualquer causa. A reabilitação depois da re-homologação levanta também a suspensão por `schema_mismatch` (decisão 2).
- **Eventos emitidos**: nenhum
- **Resposta**: `connectorKey: string`
- **Origem**: `CapabilityEnabled` do Partner.

### DeclareCapabilityProfile

- **Agregado**: `CapabilityProfile`; **cria**
- **Entrada**: `connectorId: id`, `version: int`, `effectiveFrom: instant`, `operations: list<OperationCapability>`
- **Pré-condições**:
  - `operations` não vazio → `receivables-gateway/capability-profile/operations-empty`
  - versão ainda não declarada → `receivables-gateway/capability-profile/already-declared`
  - declaração do parceiro de cada operação válida em `application.ValidateExternalProfile`, com `maxQueries` menor que `ports.MaxAttemptEntries` → `receivables-gateway/capability-profile/external-profile-invalid`
  - operação de escrita com `ambiguityDeadline` maior que zero → `receivables-gateway/capability-profile/ambiguity-deadline-missing`
  - operação de escrita com `maxSubmissions` maior que zero → `receivables-gateway/capability-profile/max-submissions-missing`
  - operação de escrita com `stateQuerySupported` e `queryInterval` maior que zero → `receivables-gateway/capability-profile/query-interval-missing`
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
- **Entrada**: `idempotencyKey: string`, `tenantId: id`, `connectorId: id`, `operation: string`, `assetType: string`, `businessKey: string`, `payloadVersion: int`, `payload: string`, `payloadHash: string`, `profile: CommandProfile`, `capabilityGate: allowed | not_enabled | suspended`, `seriesOpen: bool`
- **Pré-condições**:
  - `capabilityGate = allowed`: o caso de uso lê na UoW o perfil vigente e o conector e verifica `supports(operation, assetType)` no perfil, a capacidade habilitada em `capabilities` e a ausência de suspensão, sem chamada externa → `receivables-gateway/external-command/capability-not-enabled` (`not_enabled`) ou `receivables-gateway/external-command/operation-suspended` (`suspended`)
  - `businessKey` não vazio → `receivables-gateway/external-command/business-key-empty`
  - `payload` não vazio → `receivables-gateway/external-command/payload-empty`
  - chave já existente com `payloadHash` diferente → `receivables-gateway/external-command/idempotency-key-conflict`
  - `seriesOpen` falso → `receivables-gateway/external-command/content-changed-after-send`. O caso de uso lê a série na UoW, sob a guarda `external_commands.series_key`: `seriesOpen` é falso quando um comando da mesma série com `payloadVersion` diferente existe e não terminou em `cancelled` nem em `final_failure` por `final_validation` ou `resend_budget_exhausted` (a ausência confirmada, decisão 12), inclusive em `created` (parado na reentrega, na quarentena ou na fila) e em `retryable_failure`. Duas versões da mesma série nunca seguem para o parceiro: a anterior ainda em `created` é cancelada antes, e a enviada exige revisão, não reenvio (00 §7.3).
- **Efeitos sobre o estado**: quando ausente: campos ← entrada, `seriesKey ← <connectorKey>:<operation>:<businessKey>`, `status ← created`, `createdAt ← at`, `submissions ← 0`, `reconciliationRequired ← false`, e a intenção de despacho (`ExternalCommandDispatchRequested`, com `IntentID = idempotencyKey`) vai à outbox na mesma UoW (`RES-45`); quando presente com o mesmo `payloadHash`: nenhum, devolve o comando existente
- **Eventos emitidos**: nenhum de integração; só a intenção de despacho, interna ao contexto
- **Resposta**: `idempotencyKey: string`, `status`

### ApplyExternalStep

Efeito interno do ciclo de comando externo, na UoW de consumo: é a função `Apply` de `application.ExternalDispatch`, `application.ExternalAbandon` e `application.ExternalResolution` do kernel. Não tem RPC.

As classes retentáveis do envio (`ClassRetryableTransport`, `ClassRetryableUpstream`) não chegam aqui. O `Dispatch` encerra a tentativa e pede a reentrega da mesma mensagem, com a mesma chave (`TransientDependency`). O orçamento delas é o do consumer (decorator do `RES-27`, reentrega e quarentena), e a mensagem que esgota a reentrega vai à quarentena do kernel, tratada pelo Backoffice como `DLQ_MESSAGE` (SPEC-7KH5JJMV). Enquanto isso o comando segue em `created`, até a reentrega da quarentena ou o cancelamento: a classe retentável garante que o pedido não produziu efeito. Nessas classes o handler registra a interação de saída com o bruto capturado (`RecordExternalInteraction`, `direction = outbound`, `commandId`) antes de devolver a falha para reentrega; perder esse registro numa queda é aceitável, porque a tentativa fica no ledger e a reentrega segue.

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**:
  - `step: confirm | await | retry | fail | query | reconcile | suspend` (`application.Step`)
  - `origin: submit | observation` (`submit` vale para `Dispatch` e para `Abandon`)
  - `class` (`ports.ExternalClass` registrada na tentativa), `finding` (`ports.Finding`, quando `origin = observation`)
  - `source: submit_response | state_query | callback | file | human`
  - `interactionId: id` (cunhado pelo handler ou pelo caso de uso; vazio no `Abandon`), `externalReferenceId: id` (opcional), `partnerCode: string`, `partnerMessage: string`, `partial: bool`
  - `responseCapture` (opcional, só com `origin = submit`): `requestRef`, `requestVersionId`, `requestHash`, `responseRef`, `responseVersionId`, `responseHash`, gravados pelo adaptador do envio na porta de object storage e lidos pelo `Apply`, que é uma função do contexto; com `ClassSchemaMismatch`, também `mismatchKind`, `mismatchDetail` e `partnerDeclaredVersion`, que o adaptador apura ao classificar a resposta
- **Pré-condições**:
  - `origin = submit` exige `status` em `created`, `retryable_failure` ou `cancelled`; `origin = observation` exige `status` em `acknowledged` ou `unknown_outcome` → `receivables-gateway/external-command/not-in-flight`
- **Efeitos sobre o estado**:
  - `origin = submit`: `submissions ← submissions + 1`, `lastSubmittedAt ← at`, `dispatchHeldSince` zerado, e então o passo. Um comando `cancelled` só recebe envio na corrida com o handler do despacho; o fato externo prevalece e o comando segue o passo.
  - `confirm` → `status ← confirmed`, `confirmedAt ← at`; `externalReferenceId ← entrada` quando informado, preservando a referência gravada no aceite quando a resolução humana não a traz.
  - `await` → `status ← acknowledged`; `externalReferenceId ← entrada` quando informado; com consulta de estado, `nextQueryAt ← at + queryInterval` (convertido a ns).
  - `retry` (só por achado `FindingAbsent`) → com `submissions` menor que `maxSubmissions`: `status ← retryable_failure`, `failureCategory` vazia, `resendAfter ← at + quiescence`; o reenvio, com a mesma chave de saída, é pedido pela varredura (`RequestResend`, `RES-42`). Com `submissions` maior ou igual a `maxSubmissions` (orçamento de reenvio esgotado, decisão 12): `status ← final_failure`, `failureCategory ← resend_budget_exhausted`, sem `resendAfter`. O achado de ausência garante que o parceiro não tem o comando, então parar não deixa efeito pendente; reenviar depois disso exige comando novo, como na linha `Final*` da tabela do kernel.
  - `fail` → `status ← final_failure`; `failureCategory ← final_validation | final_business | final_authorization` pela classe, ou `final_business` pelo achado `FindingRejected` (recusa pela regra do parceiro, `ERR-28`).
  - `query` → `status ← unknown_outcome`, `failureCategory ←` categoria da classe registrada, `nextQueryAt ← at + queryInterval` (convertido a ns).
  - `reconcile` → `status ← unknown_outcome`, `failureCategory ←` categoria da classe registrada, `reconciliationRequired ← true`, `nextQueryAt` zerado.
  - `suspend` → `status ← unknown_outcome`, `failureCategory ← schema_mismatch`, `reconciliationRequired ← não stateQuerySupported`, `nextQueryAt ← at` quando há consulta. A retomada reentra como `ClassUnknownAmbiguous` (`RES-43`): a varredura só consulta comandos de operação não suspensa, então a consulta volta sozinha quando `EnableOperation` levanta a suspensão; sem consulta, o comando já está na reconciliação.
  - Início de episódio de espera (entrada no conjunto {`acknowledged`, `unknown_outcome`} vinda de fora dele): `waitingSince ← at`. Fim do episódio (saída do conjunto): `waitingSince`, `nextQueryAt`, `ambiguityExpiredAt`, `reconciliationRequired`, `acknowledgedNotified` e `unknownNotified` zerados.
  - Mapa de `ports.ExternalClass` para `failureCategory`, 1 para 1: `ClassFinalValidation` → `final_validation`, `ClassFinalBusiness` → `final_business`, `ClassFinalAuthorization` → `final_authorization`, `ClassUnknownSent` → `unknown_sent`, `ClassUnknownAmbiguous` e classe zero → `unknown_ambiguous`, `ClassSchemaMismatch` → `schema_mismatch`, `ClassAccepted` → vazia. Fora do mapa, o `retry` com o orçamento esgotado grava `resend_budget_exhausted`.
  - Com `responseCapture`, a mesma UoW grava na outbox `ExternalResponseCaptured` (interno), com `interactionId`, `commandId`, as referências do bruto e o passo. O consumer do próprio contexto registra a interação de saída (`RecordExternalInteraction`, `direction = outbound`, `source = submit_response`, com o passo capturado), e o consumer da normalização a conclui (decisão 18): quando o passo foi `suspend`, em `schema_mismatch` (`ConcludeInteraction`), o que dispara a suspensão (decisão 2). Os dois comandos entram pela inbox de comando com chave derivada de `interactionId` (`record.<interactionId>` e `conclude.<interactionId>.<normalizationRevision>`), então a reentrega é segura.
- **Eventos emitidos**: `ExternalCommandSubmitted` quando `origin = submit`; depois, pela chegada: em `confirmed` → `ExternalCommandConfirmed`; primeira entrada em `acknowledged` no episódio → `ExternalCommandAcknowledged` (`acknowledgedNotified ← true`); em `final_failure` → `ExternalCommandFailed`; primeira entrada em `unknown_outcome` no episódio → `ExternalOutcomeUnknown` (`unknownNotified ← true`); em `retryable_failure` → nenhum, porque o achado de ausência é intermediário e o evento é fato concluído (14, regra 1).
- **Resposta**: nenhuma; o kernel encerra a tentativa no ledger.

### HoldDispatch

Chamado pelo handler de `gateway.dispatch` quando a operação do comando está suspensa no momento do envio (decisão 14). Entra pela inbox de comando com chave `hold.<idempotencyKey>.<messageId>`.

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**: `status` em `created` ou `retryable_failure` (nada em voo) → `receivables-gateway/external-command/not-holdable`
- **Efeitos sobre o estado**: `dispatchHeldSince ← at` quando vazio; quando presente, nenhum. O status não muda, e o comando retido segue cancelável em `created`; não abre episódio de espera.
- **Eventos emitidos**: nenhum
- **Resposta**: `idempotencyKey: string`

### RequestResend

Chamado pela varredura `SweepExternalCommands`, um comando por UoW. A quiescência antes do reenvio é de quem chama (ADR-070 do DMPF, item 6). Também libera o despacho retido, quando a operação deixou de estar suspensa (decisão 14).

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status = retryable_failure`, ou `status = created` com `dispatchHeldSince` presente → `receivables-gateway/external-command/not-resendable`
  - sem `dispatchHeldSince`, `resendAfter` presente e menor ou igual a `at` (quiescência cumprida, `RES-42`) → `receivables-gateway/external-command/quiescence-not-elapsed`
- **Efeitos sobre o estado**: `resendAfter` e `dispatchHeldSince` zerados; a intenção de despacho (`ExternalCommandDispatchRequested`, com o mesmo `IntentID` e a mesma geração, logo a mesma chave de saída) vai à outbox na mesma UoW
- **Eventos emitidos**: nenhum de integração
- **Resposta**: `idempotencyKey: string`

### ExpireAmbiguity

Chamado pela varredura `SweepExternalCommands`, um comando por UoW (decisão 10).

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**:
  - `status` em `acknowledged` ou `unknown_outcome` → `receivables-gateway/external-command/not-waiting`
  - `waitingSince` mais `ambiguityDeadline` (convertido a ns) menor ou igual a `at` → `receivables-gateway/external-command/ambiguity-deadline-not-reached`
- **Efeitos sobre o estado**: quando `ambiguityExpiredAt` está vazio: `ambiguityExpiredAt ← at`, `reconciliationRequired ← true`; quando presente: nenhum
- **Eventos emitidos**: `ExternalCommandAmbiguityExpired`, uma vez por episódio de espera. O Backoffice abre o caso `EXTERNAL_AMBIGUITY_EXPIRED`, que escala a espera (00 §7.2.5). Em `unknown_outcome` já existe o caso `EXTERNAL_OUTCOME_UNKNOWN`, aberto na entrada pelo catálogo de 17 §4 (SPEC-7KH5JJMV); os dois casos têm o comando como sujeito. Em `acknowledged` é o primeiro caso.
- **Resposta**: `idempotencyKey: string`
- **Prazo**: `ambiguityDeadline` da operação, copiado do perfil; o valor real dos perfis de produção depende de B6 (**[A CONFIRMAR: B6]**). O dono do prazo é Ops (`RES-43`).

### ResolveUnknownOutcome

Reconciliação humana (`RES-43`): a decisão volta ao kernel como `ports.Observation`, com o id da resolução como `Observation.ID`, por `application.Resolve`, que escolhe o passo e chama `ApplyExternalStep`. A consulta de estado automática não passa por aqui: é feita pela varredura.

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: `resolution: confirmed | final_failure | not_received`, `queryInteractionId: id`, `externalReferenceId: id` (opcional), `partnerCode: string`, `partnerMessage: string`, `justification: string`, `partial: bool`
- **Pré-condições**:
  - `status` em `acknowledged` ou `unknown_outcome` → `receivables-gateway/external-command/not-waiting`. A espera em `acknowledged` também chega aqui, depois do prazo (`ExpireAmbiguity`); o ledger aceita a observação porque a última tentativa terminou com `ClassAccepted`, sem achado terminal.
  - `justification` não vazia → `receivables-gateway/external-command/justification-missing`
  - `queryInteractionId` presente: a decisão humana se apoia numa consulta ou evidência registrada (17 §5; 94 C9.3) → `receivables-gateway/external-command/query-evidence-missing`
  - `partial = true` só se `partialResultSupported` → `receivables-gateway/external-command/partial-result-not-supported`
  - o ledger do kernel grava a observação: achado terminal contrário ao gravado → `receivables-gateway/external-command/observation-conflict`; linha da tentativa cheia → `receivables-gateway/external-command/observation-saturated`. As duas voltam como desfecho do caso de uso, e a transação faz commit com o registro do ledger (decisão 13); a observação repetida (`ObserveDuplicate`) responde com o status atual.
- **Efeitos sobre o estado**: a resolução vira achado (`confirmed` → `FindingApplied`, `final_failure` → `FindingRejected`, `not_received` → `FindingAbsent`), e o passo segue a tabela do kernel. `not_received` leva a `retryable_failure` com a mesma chave (00 §7.2.3): voltar a `created` apagaria o fato do envio, e `final_failure` impediria o reenvio. Com o orçamento de reenvio esgotado, o mesmo `retry` leva a `final_failure` com `resend_budget_exhausted` (decisão 12). Com `idempotencySupport = none` e sem `authoritativeAbsence`, o kernel devolve `reconcile` e o comando segue em `unknown_outcome`. `reconciliationRequired ← false` quando o passo não é `reconcile`.
- **Eventos emitidos**: os de `ApplyExternalStep`
- **Resposta**: `idempotencyKey: string`, `status`

### CancelExternalCommand

- **Agregado**: `ExternalCommand`; exige existente
- **Entrada**: nenhuma além da identidade
- **Pré-condições**: `status = created` (nada foi enviado) → `receivables-gateway/external-command/already-sent`. Depois do envio, cancelar é impossível; desfazer um efeito já enviado é outro comando externo (`requestRelease`), não uma transição (09 §7; 14, Sagas: não há rollback técnico de efeito constituído).
- **Efeitos sobre o estado**: `status ← cancelled`
- **Eventos emitidos**: nenhum
- **Resposta**: `idempotencyKey: string`
- **Corrida com o envio**: o handler do despacho lê o comando antes de `application.Dispatch` e descarta a intenção de comando `cancelled`. O cancelamento que chega entre essa leitura e o envio não desfaz o envio: `ApplyExternalStep` registra o envio e o comando segue o passo, porque o fato externo prevalece.

### RecordExternalInteraction

- **Agregado**: `ExternalInteraction`; **cria**
- **Entrada**: `interactionId: id`, `connectorId: id`, `operation: string`, `direction`, `source`, `contentType: string`, `commandId: id` (opcional), `partnerEventId: string` (opcional), `businessDate: string` (opcional), `requestRef`, `requestVersionId`, `requestHash`, `responseRef`, `responseVersionId`, `responseHash` (opcionais), `capturedStep` e, quando ele é `suspend`, `mismatchKind`, `mismatchDetail` e `partnerDeclaredVersion` (opcionais; só na captura de saída), `duplicateOf: id` (opcional, resolvido pelo caso de uso por `FindExternalInteractionByPartnerEvent`)
- **Pré-condições**: ao menos uma referência de payload bruto → `receivables-gateway/external-interaction/raw-payload-missing`
- **Efeitos sobre o estado**: campos ← entrada; `status ← duplicate` quando `duplicateOf` informado, senão `stored`; `normalizationRevision ← 1`; `receivedAt ← at`. O bruto é persistido sempre, antes de deduplicar e de interpretar (04 §7, passos 2 e 3). Em `stored`, `ExternalInteractionStored` (interno) vai à outbox na mesma UoW, e o consumer de `gateway.normalizations` conclui a interação (decisão 18).
- **Eventos emitidos**: nenhum de integração; só `ExternalInteractionStored`, interno ao contexto
- **Resposta**: `interactionId: id`, `status`

### ConcludeInteraction

- **Agregado**: `ExternalInteraction`; exige existente
- **Entrada**: `outcome: normalized | uncorrelated | schema_mismatch`, `commandId: id` (opcional), `readingHash: string` (hash da leitura normalizada, inclusive o achado sobre o comando; vazio fora de `normalized`), `finding` (opcional; o achado sobre o comando, só em `normalized` com `commandId`), `commandTerminal: bool` (fato: o comando ligado está em estado terminal, lido na UoW por `Load`), `mismatchKind` e `mismatchDetail: string` (obrigatórios em `schema_mismatch`), `partnerDeclaredVersion: string` (opcional)
- **Pré-condições**:
  - `status = stored` → `receivables-gateway/external-interaction/already-concluded`
  - `schema_mismatch` exige `mismatchKind` e `mismatchDetail` não vazio → `receivables-gateway/external-interaction/mismatch-detail-missing`
- **Efeitos sobre o estado**: `status ← outcome`; `readingHash ← entrada`; `finding ← entrada`; `commandId ← entrada` quando informado; campos de quebra ← entrada
- **Eventos emitidos**: na revisão 1, `ExternalSchemaMismatchDetected` quando `schema_mismatch` e `ExternalInteractionUncorrelated` quando `uncorrelated` (abre caso no Backoffice, 04 §7; decisão 5). Na revisão maior que 1, `ExternalInteractionReprocessed` sempre, com `outcomeChanged` verdadeiro quando o desfecho difere de `previousOutcome` ou `readingHash` difere de `previousReadingHash`, e `terminalCommandAffected = outcomeChanged e commandTerminal`; os eventos de `schema_mismatch` e `uncorrelated` só saem com `outcomeChanged` (decisões 7 e 8; ver `ReprocessInteraction`).
- **Resposta**: `interactionId: id`, `status`
- **Suspensão da operação afetada**: por coreografia, sem segunda UoW no caso de uso. `ExternalSchemaMismatchDetected` vai à outbox na UoW de `ConcludeInteraction`, e o próprio contexto o consome pela inbox (ADR-036) e executa `SuspendOperation` com `cause = schema_mismatch` (decisão 2). O Partner consome o mesmo evento (DEVS-171).
- **Desfecho de comando por callback ou arquivo**: quando a interação normalizada traz o desfecho de um comando que não é terminal, o consumer da normalização entrega o achado ao kernel como `ports.Observation` (`application.Resolve`), depois do commit de `ConcludeInteraction`, com `<interactionId>.r<normalizationRevision>` como `Observation.ID` (decisão 18). A captura de saída na revisão 1 não entrega observação, porque o `Dispatch` já aplicou o passo. O ledger grava a observação uma vez só (`ObserveDuplicate` na repetição; `ObserveConflict` é só um achado terminal contrário a outro), então repetir o caso de uso é seguro; o comando que ficar sem desfecho cai na varredura, que consulta o estado ou escala a espera. Comando terminal nunca é alterado (decisão 7).

### ReprocessInteraction

- **Agregado**: `ExternalInteraction`; exige existente
- **Entrada**: `justification: string`
- **Pré-condições**:
  - `status` é `normalized`, `uncorrelated` ou `schema_mismatch` → `receivables-gateway/external-interaction/not-concluded`
  - `justification` não vazia (operação de backoffice, 04 §6) → `receivables-gateway/external-interaction/justification-missing`
  - referência do bruto presente → `receivables-gateway/external-interaction/raw-payload-missing`
- **Efeitos sobre o estado**: `previousOutcome ← status`, `previousReadingHash ← readingHash`, `status ← stored`, `normalizationRevision ← normalizationRevision + 1`, e `ExternalInteractionStored` (interno) vai à outbox na mesma UoW. Reexecuta só a normalização e a publicação; nunca reenvia comando externo.
- **Eventos emitidos**: nenhum de integração; o resultado do reprocessamento é publicado por `ConcludeInteraction` (`ExternalInteractionReprocessed`)
- **Resposta**: `interactionId: id`, `normalizationRevision: int`
- **Reemissão**: reprocessar sem mudar a leitura não emite evento de negócio. Leitura nova emite evento novo, com `eventId` novo, porque a correção nunca reaproveita `eventId` (92 §5.3; 14, regra 8). O PRD e o catálogo prevalecem sobre 04 §6, que fala em "mesmos eventos, deduplicados pelo inbox".
- **Comando terminal**: leitura nova sobre comando terminal não altera o comando (04 §6: o histórico da interpretação errada fica). `ExternalInteractionReprocessed` sai com `terminalCommandAffected = outcomeChanged e commandTerminal` verdadeiro, e o Backoffice abre o caso (decisões 7 e 8). Sobre comando não terminal, a leitura nova entra no kernel como `ports.Observation`, como no callback.
- **Registro**: o Ledger recebe `ExternalInteractionReprocessed` em todo reprocessamento (decisão 8), além da trilha de auditoria do kernel (FND-08).

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

Os nove abaixo são publicados no canal `gateway.events`; a intenção de despacho e a captura da resposta são mensagens internas. O catálogo `92 §3` não define o `data` de nenhum evento do Gateway: os campos são derivados do agregado e das convenções de 92 §2 (evento autocontido; motivo de falha preserva o código e o texto originais do parceiro), e o contrato é o `.proto` desta spec (ADR-022, linha 6). Nenhum evento carrega payload bruto nem dado pessoal. `partnerId`, `environment` e `connectorKey` vêm do `Connector`, lido na UoW por `Load`; os eventos de comando levam `tenantId` do comando, porque na fase 1 o tenant de negócio é dado, não escopo (decisão 1).

### ExternalCommandSubmitted

- **Campos**: `idempotencyKey: string ← ExternalCommand.idempotencyKey`; `tenantId: id`; `connectorId: id`; `operation: string`; `businessKey: string`; `submission: int ← submissions`; `at: instant ← at`

### ExternalCommandAcknowledged

- **Campos**: `idempotencyKey`, `tenantId`, `connectorId`, `operation`, `businessKey` (como acima); `externalReferenceId: id` (opcional); `at: instant ← at`

### ExternalCommandConfirmed

- **Campos**: `idempotencyKey`, `tenantId`, `connectorId`, `operation`, `businessKey`; `externalReferenceId: id`; `confirmationSource: submit_response | state_query | callback | file | human ← source`; `partial: bool`; `interactionId: id`; `confirmedAt: instant ← confirmedAt`

### ExternalCommandFailed

- **Campos**: `idempotencyKey`, `tenantId`, `connectorId`, `operation`, `businessKey`; `category: final_validation | final_business | final_authorization | resend_budget_exhausted`; `partnerCode: string`; `partnerMessage: string`; `interactionId: id`; `at: instant ← at`

### ExternalOutcomeUnknown

- **Campos**: `idempotencyKey`, `tenantId`, `connectorId`, `operation`, `businessKey`; `failureCategory: unknown_sent | unknown_ambiguous | schema_mismatch` (vazia quando a última tentativa terminou em `ClassAccepted`, isto é, quando a espera de um aceite passou a desfecho incerto por `FindingInconclusive` ou pelo teto de consultas); `submissions: int ← submissions`; `lastSubmittedAt: instant`; `stateQuerySupported: bool ← profile.stateQuerySupported`; `requiresHumanResolution: bool ← reconciliationRequired`; `unknownSince: instant ← waitingSince`

### ExternalSchemaMismatchDetected

- **Campos**: `interactionId: id`; `connectorId: id`; `connectorKey: string`; `partnerId: id`; `environment`; `operation: string`; `assetType: string` (opcional; `ExternalCommand.assetType`, lido na UoW, quando a interação tem `commandId`; vazio nos demais casos); `commandId: id` (opcional); `mismatchKind`; `mismatchDetail: string`; `partnerDeclaredVersion: string`; `at: instant ← at`. `partnerId` e `environment` tornam o evento autocontido para o Partner e para o caso do Backoffice (92 §2).

### ExternalInteractionUncorrelated

- **Campos**: `interactionId: id`; `connectorId: id`; `connectorKey: string`; `partnerId: id`; `environment`; `operation: string`; `partnerEventId: string` (opcional); `businessDate: string` (opcional); `receivedAt: instant`; `at: instant ← at`

### ExternalInteractionReprocessed

- **Campos**: `interactionId: id`; `connectorId: id`; `partnerId: id`; `operation: string`; `commandId: id` (opcional); `previousRevision: int`; `normalizationRevision: int`; `previousOutcome`; `outcome`; `outcomeChanged: bool`; `terminalCommandAffected: bool` (`outcomeChanged` e `commandTerminal`); `at: instant ← at`

### ExternalCommandAmbiguityExpired

- **Campos**: `idempotencyKey`, `tenantId`, `connectorId`, `partnerId: id`, `operation`, `businessKey`; `status: acknowledged | unknown_outcome`; `waitingSince: instant`; `ambiguityDeadline: int`; `stateQuerySupported: bool`; `at: instant ← ambiguityExpiredAt`

### ExternalResponseCaptured (interno)

- **Canal**: `gateway.interactions`, um tipo só (`KFK-03`), consumido só pelo próprio contexto.
- **Campos**: `interactionId: id`; `commandId: id ← idempotencyKey`; `connectorId: id`; `operation: string`; `requestRef`, `requestVersionId`, `requestHash`, `responseRef`, `responseVersionId`, `responseHash`; `contentType: string` (da resposta, para o registro da interação); `step`; `mismatchKind`, `mismatchDetail` e `partnerDeclaredVersion` (quando o passo foi `suspend`); `at: instant`

### ExternalInteractionStored (interno)

- **Canal**: `gateway.normalizations`, um tipo só (`KFK-03`), consumido só pelo próprio contexto (decisão 18).
- **Campos**: `interactionId: id`; `connectorId: id`; `operation: string`; `direction`; `source`; `commandId: id` (opcional); `normalizationRevision: int`; `at: instant ← at`. O bruto não viaja: o consumer o lê da porta pela versão gravada na interação.

### ExternalCommandDispatchRequested (interno)

- **Canal**: `gateway.dispatch`, um tipo só (`KFK-03`), consumido só pelo handler do despacho do próprio contexto.
- **Campos**: `idempotencyKey: string` (é o `IntentID` do `IDM-11`); `connectorId: id`; `operation: string`; `at: instant ← at` (da criação ou do `RequestResend` que gravou a intenção). O payload do comando não viaja: o handler lê o comando pela chave.

## Consultas

Locais, fora da UoW:

- `FindConnector` - filtros: `connectorKey`; cardinalidade: `one`
- `FindEffectiveCapabilityProfile` - filtros: `connectorId`, `effectiveAt`; cardinalidade: `one`
- `FindCapabilityProfile` - filtros: `connectorId`, `version`; cardinalidade: `one`
- `FindQueryAuthorizationByOrganization` - filtros: `organizationId`; cardinalidade: `many`
- `FindExternalCommand` - filtros: `idempotencyKey`; cardinalidade: `one`
- `FindExternalCommandsByBusinessKey` - filtros: `connectorId`, `businessKey`; cardinalidade: `many`
- `FindExternalCommandsByStatus` - filtros: `status` (só não terminal: `created`, `acknowledged`, `retryable_failure`, `unknown_outcome`; terminal é entrada inválida, decisão 15), `connectorId` (opcional); cardinalidade: `many` (acompanhamento e varredura `SweepExternalCommands`: reenvio, liberação do despacho retido, consulta e prazo)
- `FindExternalInteraction` - filtros: `interactionId`; cardinalidade: `one`
- `FindExternalInteractionByPartnerEvent` - filtros: `connectorId`, `partnerEventId`; cardinalidade: `one` (deduplicação)
- `FindExternalInteractionsByCommand` - filtros: `commandId`; cardinalidade: `many`
- `FindExternalReference` - filtros: `referenceKey`; cardinalidade: `one`
- `FindExternalReferencesBySubject` - filtros: `subjectType`, `subjectId`; cardinalidade: `many`

Leituras externas pelas portas canônicas (04 §3). Como gravam interação, são casos de uso de escrita, não consultas do kernel (`UOW-11`): entram pela inbox de comando com chave (`kernelgrpc.WithCommands`, `application.Execute`; ADR-056). Cada uma corre em ordem: guarda local, chamada ao adaptador (bloco `provider`), e só então `application.Execute`, que envolve `RecordExternalInteraction`. Toda saída carrega `asOf`, `providerId` e a referência externa. Leitura que expira por tempo é erro recuperável (00 §7.4). No replay da mesma chave de comando o adaptador é chamado de novo e o kernel devolve o desfecho já gravado; o bruto dessa segunda leitura fica gravado como versão órfã, porque a porta não tem escrita condicional (decisão 16), o que é aceitável porque a leitura não muda o estado do parceiro. Até #5 as leituras entram pelo RPC genérico `ReadExternal`, e o desfecho guardado na inbox leva a leitura normalizada, limitada a `maxPayloadBytes` da operação (decisão 20).

- `ReceivablesQueryPort`: `DiscoverSchedule`, `QueryAsset`, `QueryOwnership`, `QueryEncumbrances` - guarda: `QueryAuthorization` ativa para a organização e o tipo de ativo → `receivables-gateway/query-authorization/consent-not-active`; perfil suporta a operação e a operação não está suspensa → `receivables-gateway/connector/capability-not-enabled`
- `AuthorizationPort` (leituras): `QueryOptInStatus`, `ListAuthorizations` - guarda de capacidade; sem guarda de consentimento, porque o opt-in externo antecede a autorização de consulta
- `RegulatoryEffectsPort` (leituras): `QueryEffect` (adaptador de `ports.ExternalState`, que resolve `unknown_outcome` no passo `query`), `QueryEffectsByAsset` - guarda de capacidade
- `SettlementQueryPort`: `QuerySettlements`, `QuerySettlementByAsset` - guarda de capacidade
- `ExternalEvidencePort`: `FetchEvidence` - guarda de capacidade; `storeInteraction` é o comando `RecordExternalInteraction`
- As escritas das portas (`activateOptIn`, `revokeOptIn`, `submitEffect`, `requestRelease`) são operações de `ExternalCommand`.

## Relações entre agregados

- `Connector.partnerId` → `Partner` (Partner Management, outro contexto)
- `Connector.capabilities.capabilityId` → `Capability` (Partner Management, outro contexto)
- `CapabilityProfile.connectorId` → `Connector`
- `QueryAuthorization.organizationId` → `Organization` (Identity, outro contexto)
- `ExternalCommand.connectorId` → `Connector`
- `ExternalCommand.externalReferenceId` → `ExternalReference`
- `ExternalInteraction.connectorId` → `Connector`
- `ExternalInteraction.commandId` → `ExternalCommand`
- `ExternalReference.subjectId` → objeto interno de outro contexto (ativo, efeito, autorização, liquidação)

## Integração

- **Publica** (integration events no canal `gateway.events`, em `apps/backend/gateway/contract/proto/company/gateway/event/v1/`, package `company.gateway.event.v1`): `ExternalCommandSubmitted` (`external_command_submitted.proto`), `ExternalCommandAcknowledged` (`external_command_acknowledged.proto`), `ExternalCommandConfirmed`, `ExternalCommandFailed`, `ExternalOutcomeUnknown`, `ExternalSchemaMismatchDetected`, `ExternalInteractionUncorrelated`, `ExternalInteractionReprocessed` e `ExternalCommandAmbiguityExpired`. O canal leva vários tipos, então o catálogo declara o conjunto fechado dos nove, a variação modelada no contrato e a ordenação válida para o conjunto: ordem só por agregado, pela `partitionkey` (`KFK-03b`; ADR-022, linha 16).
  - Consumidores: `Submitted` → Ledger; `Acknowledged` → Ledger (PRD 12, req. 1); `Confirmed` → Effects, Funding, Ledger; `Failed` → Effects, Ops, Ledger; `OutcomeUnknown` → Effects, Funding, Ops, Ledger; `SchemaMismatchDetected` → Partner (DEVS-171), o próprio Gateway (suspensão), Ops, Ledger; `InteractionUncorrelated` → Ops, Ledger; `InteractionReprocessed` → Ledger, e Ops quando `terminalCommandAffected`; `CommandAmbiguityExpired` → Ops, Ledger.
  - `Acknowledged`, `InteractionUncorrelated`, `InteractionReprocessed` e `CommandAmbiguityExpired` faltam em 92 §4. O contrato é o `.proto` desta spec (ADR-022, linha 6), e o addendum do catálogo 92 e 14 fica para issue de documentação, como no Partner.
  - Consent não consome eventos do Gateway: o provider do Consent chama a `AuthorizationPort` (`activateOptIn`, `revokeOptIn`, `queryOptInStatus`) e resolve o desfecho desconhecido pela consulta, nunca por reenvio (SPEC-Z3YR96YS, Integração). Com protocolo `async_callback`, o Consent consulta até o desfecho.
- **Interno**: `ExternalCommandDispatchRequested` no canal `gateway.dispatch`, consumido pelo handler do despacho, que chama `application.Dispatch` (`RES-45`); `ExternalResponseCaptured` no canal `gateway.interactions`, consumido para registrar a interação de saída; `ExternalInteractionStored` no canal `gateway.normalizations`, consumido para normalizar e concluir toda interação gravada e entregar o achado ao kernel (decisão 18). Cada canal interno leva um tipo só (`KFK-03`), e os três rodam no papel `consumer` (decisão 21).
- **Consome** (disposições de consumo: FND-04 §6.4; efeito de broker só depois do retorno da transação; `janela_redelivery` declarada, ADR-020):
  - Consumo de Consent na terceira fatia (decisão 22), **[A CONFIRMAR: contrato do Consent]**:
  - `company.consent.event.v1.ConsentGranted` → `GrantQueryAuthorization`. Mapa: `consentId`, `organizationId`, `version` → `consentVersion`, `scope.assetTypes` → `assetTypes`, `scope.periodFrom` → `periodFromDate`, `scope.periodUntil` → `periodUntilDate`. Só `consentType = RECEIVABLES_OPT_IN`; os demais são descartados.
  - `company.consent.event.v1.ConsentRevoked` → `RevokeQueryAuthorization`. Mapa: `consentId`, `effectiveAt`. Consentimento ainda desconhecido: disposição de espera ou falha de FND-04 §6.4.
  - Canal `partner.events`, com 11 tipos: `Sink` de conjunto fechado no bloco `app`, no padrão do app de referência `reservations` do kernel (`apps/backend/reservations/app/sink.go`), aceitando `capability-enabled`, `capability-suspended` e `integration-profile-registered`; os demais tipos recebem `Ack` sem inbox. O catálogo do canal do Partner ainda declara só `partner-registered` (DEVS-171).
  - `company.partner.event.v1.CapabilitySuspended` → `SuspendOperation` com `cause = partner`. Mapa (`capability_suspended.proto`): `partner_id` e `environment` → `connectorKey` (`<partnerId>:<environment>`, ambiente em minúsculas; `ENVIRONMENT_UNSPECIFIED` vira `Failure(Validation)`); `capability_id` → `capabilityId`; `operation`; `asset_type` → `assetType`; `version`. `reason` e `trigger` não entram no estado, e o evento não tem escopo de produto.
  - `company.partner.event.v1.CapabilityEnabled` → `EnableOperation`. Mesmo mapa, mais `state_query_supported` → `stateQuerySupported`. `rollout_mode` e `restriction_note` não entram no estado.
  - `company.partner.event.v1.IntegrationProfileRegistered` (DEVS-171) → `RegisterConnector`. Mapa: `partnerId`, `environment`, `secretRef` → `credentialRef`, versão do perfil → `integrationVersion`.
  - `company.gateway.event.v1.ExternalSchemaMismatchDetected`, publicado pelo próprio contexto → `SuspendOperation` com `cause = schema_mismatch`, `operation` do evento e `assetType` vazio (decisão 2). O canal `gateway.events` leva nove tipos, então o consumo usa `Sink` de conjunto fechado que aceita só `external-schema-mismatch-detected`; os demais recebem `Ack` sem inbox.
  - `CapabilityRolloutChanged` não é consumido (decisão 4): o evento traz só `mode` e `percentage`, e o rollout é avaliado por quem abre a operação (`FindEffectiveCapability`, `FindEligiblePartners`; 13 §4). Reduzir o rollout não interrompe operação em curso (13 §6).
  - Vocabulário de `operation`: os nomes das operações das portas (04 §3), os mesmos que o Partner usa ao homologar capacidades.
- **Chamadas síncronas de entrada** (gRPC): Consent (`AuthorizationPort`), Portfolio (`ReceivablesQueryPort`), Effects (`RegulatoryEffectsPort`), Settlement (`SettlementQueryPort`), Contracting e Funding (`ExternalEvidencePort`); agendador externo (`SweepExternalCommands`, método de comando comum no tenant da plataforma).

## Borda e persistência

- **Borda**: só gRPC, `company.gateway.service.v1.GatewayService`, um RPC por comando e por consulta, atrás da cadeia de interceptors do kernel: os comandos acima menos `ApplyExternalStep` (efeito interno do ciclo de comando externo), `HoldDispatch` (do handler do despacho, decisão 14), `RecordExternalInteraction` e `ConcludeInteraction` (internos ao ciclo da interação: entram pelo RPC de entrada, pela leitura externa, pela varredura e pelos consumers internos, decisões 18 e 19), `RequestResend` e `ExpireAmbiguity` (servidos pelo RPC `SweepExternalCommands`, método de comando comum: grava, então não é método de plataforma do `WithPlatformMethods`, que só lê entre tenants), as 12 consultas locais e as leituras externas das portas. Todas as chamadas correm no tenant único da plataforma (decisão 1); o tenant de negócio viaja como dado, no `tenantId` de `CreateExternalCommand`. Webhook de entrada, mTLS de origem e a resposta em menos de 200 ms (p95) são do `bff` (91 §5.1); o `bff` entrega o bruto ao contexto pelo RPC `ReceivePartnerInteraction`, que grava o bruto e executa `RecordExternalInteraction` (decisão 19). Além dos comandos, a borda serve `ReceivePartnerInteraction` e `ReadExternal` (decisão 20), os dois com chave de comando.
- **Banco**: database e role `gateway`, schema `public`, tabelas `connectors`, `capability_profiles`, `query_authorizations`, `external_commands`, `external_interactions` e `external_references`, com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra: `capability_profiles.connector_id` e `version` (`FindEffectiveCapabilityProfile`); `query_authorizations.organization_id`; `external_commands.connector_id`, `business_key`, `series_key` e `status`; `external_interactions.connector_id`, `partner_event_id` e `command_id`; `external_references.subject_type` e `subject_id`. O resto do estado vai em `snapshot` `jsonb`. Guardas da leitura relacional (ADR-067 do DMPF): `capability_profiles.connector_id`, `external_commands.series_key` e `external_interactions.command_id`. O ledger de tentativas `external_attempt` é tabela de plataforma do kernel, com o tenant na chave (ADR-070 do DMPF, item 10). O payload bruto fica fora do banco, na porta de object storage do kernel (SPEC-CWBA4BPV, DEVS-95); o banco guarda referência, `versionId` e hash.
- **Instante**: inteiro de nanossegundos no domínio. Diverge do design: `00 §8` fala em milissegundo; segue o DMPF. A data de negócio do parceiro é `businessDate`, separada de `receivedAt`.
- **Diverge do design**: o doc 92 §1 (envelope JSON, `schemaUri`) cede ao contrato Protobuf e ao CloudEvents montado pelo relay; o broker é Kafka, a confirmar na decisão D4 da fundação (`SPEC-GXGASP24`).

## Políticas transversais

- **Idempotência**: os comandos entram pela inbox de comando do kernel (ADR-056; FND-04 §7.6), com chave declarada na borda; os eventos consumidos têm inbox por `eventId` (ADR-036). A idempotência de saída é do kernel (`IDM-11`): chave de saída derivada da intenção, ledger `external_attempt` (`ports.AttemptLedger`) e `application.Dispatch`; tentativas órfãs por queda de processo são fechadas por `app.RunSweep`, cuja função de varredura do contexto chama `application.Abandon` no tenant da tentativa, e o passo resultante entra em `ApplyExternalStep` (`RES-43`, `RES-45`). A identidade natural de `ExternalCommand` segue como idempotência de negócio.
- **Resiliência** (00 §7.4), no bloco `provider`: timeout, com o orçamento de tempo da chamada em `app.ExternalBudget`; retry da chamada só nas classes retentáveis, com a mesma chave, pelo decorator do `RES-27` e pela reentrega do consumer até a quarentena; circuit breaker por operação e parceiro (`observability/resilience.Breaker`); bulkhead (`Bulkhead`); classificação da resposta HTTP (`http.OutcomeTable.Classify`). Rate limit: o kernel só fixa a posição do slot `rate_limit` na composição, sem limitador; na fase 1, limitador local por instância nesse slot, com cota de `maxRequestsPerMinute` dividida pelo número de réplicas declarado na composition root. O limitador distribuído que 00 §7.4 pede fica no Escopo fora. Abrir o circuito gera só telemetria, sem evento de domínio (decisão 9): métrica por parceiro e operação, log estruturado e alarme.
- **Autorização**: pelo gancho de autorização do bloco `application`, antes da transação (FND-04 §3.2); os chamadores são serviços internos. Negar em dúvida (fail-closed) para consentimento, capacidade e suspensão.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`) e pelos eventos para o Ledger. Reprocessamento é operação de backoffice com justificativa, e o Ledger recebe `ExternalInteractionReprocessed`. Métricas por parceiro, endpoint, código e percentil (PRD 04, RNF).

## Critérios de aceite

- [ ] Os gates do workspace verdes para `gateway` (`fmt-check`, `vet`, `build`, `lint`, `test-race` e `test-distributed` com Postgres, verificador com `--base` e `dmpf-context-check`).
- [ ] Simulador de parceiro (04:G12) sobre o parceiro roteirizado do kernel (`serviceskit.NewPartner`, `KIT-12`), cobrindo sucesso, falha recuperável com reentrega até a quarentena, falha definitiva, `unknown_outcome` com e sem consulta de estado, ausência confirmada com reenvio depois da quiescência e quebra de contrato; adaptador `ExternalCommand` e ledger aprovados nas cláusulas de conformidade de `providerkit` (`KIT-04`).
- [ ] Cenários:

```text
DADO um conector sem registro
QUANDO RegisterConnector chega com credentialRef preenchido
ENTÃO o conector nasce sem suspensões e registeredAt = at

DADO qualquer entrada
QUANDO RegisterConnector chega com credentialRef vazio
ENTÃO rejeita com receivables-gateway/connector/credential-ref-empty

DADO um conector registrado na integrationVersion 1
QUANDO RegisterConnector chega com integrationVersion 2 e outro credentialRef
ENTÃO o credentialRef é trocado e capabilities e suspensions seguem como estavam

DADO um conector com submitEffect habilitado
QUANDO SuspendOperation chega para submitEffect
ENTÃO a suspensão entra em suspensions com since = at e as demais operações seguem

DADO um conector com submitEffect suspenso por schema_mismatch
QUANDO EnableOperation chega para submitEffect depois da re-homologação
ENTÃO a capacidade entra em capabilities e a suspensão sai de suspensions

DADO uma capacidade de submitEffect aplicada na versão 3
QUANDO CapabilitySuspended chega com a versão 2 da mesma operação
ENTÃO o evento é descartado sem alterar o conector

DADO um conector sem capacidade habilitada para submitEffect
QUANDO CreateExternalCommand chega para submitEffect
ENTÃO rejeita com receivables-gateway/external-command/capability-not-enabled e nenhuma chamada externa sai

DADO ExternalSchemaMismatchDetected publicado pelo próprio contexto
QUANDO o consumer do Gateway o recebe
ENTÃO SuspendOperation suspende a operação com cause schema_mismatch, sem segunda UoW no caso de uso que concluiu a interação

DADO uma versão de perfil ainda não declarada
QUANDO DeclareCapabilityProfile chega com uma operação
ENTÃO o perfil é criado imutável com a versão e a effectiveFrom informadas

DADO qualquer conector
QUANDO DeclareCapabilityProfile chega sem operações
ENTÃO rejeita com receivables-gateway/capability-profile/operations-empty

DADO um perfil já declarado na versão 2
QUANDO DeclareCapabilityProfile chega novamente com a versão 2
ENTÃO rejeita com receivables-gateway/capability-profile/already-declared

DADO uma operação com keyMaxLength que não alcança 128 bits no keyAlphabet
QUANDO DeclareCapabilityProfile chega
ENTÃO rejeita com receivables-gateway/capability-profile/external-profile-invalid

DADO uma operação submitEffect sem ambiguityDeadline
QUANDO DeclareCapabilityProfile chega
ENTÃO rejeita com receivables-gateway/capability-profile/ambiguity-deadline-missing

DADO uma operação submitEffect com stateQuerySupported e sem queryInterval
QUANDO DeclareCapabilityProfile chega
ENTÃO rejeita com receivables-gateway/capability-profile/query-interval-missing

DADO uma operação submitEffect sem maxSubmissions
QUANDO DeclareCapabilityProfile chega
ENTÃO rejeita com receivables-gateway/capability-profile/max-submissions-missing

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
ENTÃO o comando nasce created, durável antes de qualquer envio, com a declaração da operação copiada do perfil e ExternalCommandDispatchRequested na outbox

DADO um comando da série enviado com payloadVersion 1 e confirmado
QUANDO CreateExternalCommand chega para a mesma série com payloadVersion 2
ENTÃO rejeita com receivables-gateway/external-command/content-changed-after-send

DADO um comando da série em final_failure por final_validation
QUANDO CreateExternalCommand chega para a mesma série com payloadVersion 2
ENTÃO o comando novo nasce created, com outra chave de saída

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

DADO um comando created e um perfil com eficácia on_submit
QUANDO o envio volta ClassAccepted
ENTÃO ApplyExternalStep leva o comando a confirmed, submissions = 1, e ExternalCommandSubmitted e ExternalCommandConfirmed vão à outbox

DADO um comando created e um perfil com eficácia by_observation
QUANDO o envio volta ClassAccepted
ENTÃO o comando vai a acknowledged e ExternalCommandAcknowledged é emitido, sem tratar o aceite como efeito confirmado

DADO um comando created
QUANDO o envio volta ClassRetryableTransport
ENTÃO ApplyExternalStep não é chamado, o comando segue created, nenhum evento é emitido e a mensagem é reentregue com a mesma chave de saída

DADO um comando created cuja intenção esgotou a reentrega com ClassRetryableUpstream
QUANDO a mensagem vai à quarentena do kernel
ENTÃO o comando segue created e pode ser cancelado, porque nenhum pedido produziu efeito

DADO um comando created
QUANDO o envio volta ClassFinalBusiness
ENTÃO o comando vai a final_failure e ExternalCommandFailed é emitido com código e texto originais do parceiro

DADO um comando created com stateQuerySupported verdadeiro
QUANDO o envio termina por timeout depois de escrito (ClassUnknownSent)
ENTÃO o comando vai a unknown_outcome, waitingSince = at, nextQueryAt = at + queryInterval e ExternalOutcomeUnknown é emitido, nunca final_failure

DADO uma tentativa de envio que ficou begun por queda do processo
QUANDO app.RunSweep a encontra vencida e chama application.Abandon
ENTÃO ApplyExternalStep recebe origin submit com ClassUnknownSent presumida e o comando vai a unknown_outcome

DADO um comando created com stateQuerySupported falso
QUANDO o envio volta ClassUnknownAmbiguous
ENTÃO o comando vai a unknown_outcome com reconciliationRequired verdadeiro e ExternalOutcomeUnknown sai com requiresHumanResolution verdadeiro

DADO um comando confirmed
QUANDO ApplyExternalStep chega
ENTÃO rejeita com receivables-gateway/external-command/not-in-flight

DADO um parceiro com partialResultSupported falso
QUANDO a consulta de estado devolve uma confirmação parcial
ENTÃO o adaptador a entrega como FindingInconclusive, nunca como FindingApplied, e o comando segue em unknown_outcome até o teto de consultas e depois à reconciliação

DADO um comando unknown_outcome com nextQueryAt vencido e a operação habilitada
QUANDO SweepExternalCommands roda e a consulta de estado devolve FindingApplied
ENTÃO a interação da consulta é registrada, o comando vai a confirmed e ExternalCommandConfirmed é emitido, sem reenvio

DADO um comando unknown_outcome com failureCategory schema_mismatch e a operação suspensa
QUANDO SweepExternalCommands roda
ENTÃO o comando não é consultado até EnableOperation levantar a suspensão

DADO um comando unknown_outcome com idempotencySupport key_header
QUANDO a consulta de estado devolve FindingAbsent
ENTÃO o comando vai a retryable_failure com resendAfter = at + quiescence e nenhum evento é emitido

DADO um comando unknown_outcome com idempotencySupport key_header e submissions igual a maxSubmissions
QUANDO a consulta de estado devolve FindingAbsent
ENTÃO o comando vai a final_failure com failureCategory resend_budget_exhausted, sem resendAfter, e ExternalCommandFailed vai à outbox

DADO um comando retryable_failure com resendAfter vencido
QUANDO SweepExternalCommands roda
ENTÃO RequestResend grava a intenção de despacho com o mesmo IntentID e o reenvio usa a mesma chave de saída

DADO um comando retryable_failure dentro da quiescência
QUANDO RequestResend chega
ENTÃO rejeita com receivables-gateway/external-command/quiescence-not-elapsed

DADO um comando unknown_outcome com idempotencySupport key_header e reconciliationRequired
QUANDO ResolveUnknownOutcome chega com resolution not_received, queryInteractionId e justificativa
ENTÃO o comando vai a retryable_failure e nenhum evento é emitido

DADO um comando confirmed
QUANDO ResolveUnknownOutcome chega
ENTÃO rejeita com receivables-gateway/external-command/not-waiting

DADO um comando acknowledged cujo prazo de espera venceu, com reconciliationRequired
QUANDO ResolveUnknownOutcome chega com resolution confirmed, queryInteractionId e justificativa
ENTÃO o comando vai a confirmed e ExternalCommandConfirmed é emitido com confirmationSource human

DADO um comando que vai de unknown_outcome a acknowledged por FindingPending e volta por FindingInconclusive
QUANDO as transições acontecem no mesmo episódio de espera
ENTÃO waitingSince não muda e ExternalOutcomeUnknown e ExternalCommandAcknowledged saem no máximo uma vez cada

DADO um comando da série em created com payloadVersion 1
QUANDO CreateExternalCommand chega para a mesma série com payloadVersion 2
ENTÃO rejeita com receivables-gateway/external-command/content-changed-after-send até a versão 1 ser cancelada

DADO um comando unknown_outcome
QUANDO ResolveUnknownOutcome chega sem queryInteractionId
ENTÃO rejeita com receivables-gateway/external-command/query-evidence-missing

DADO um comando unknown_outcome
QUANDO ResolveUnknownOutcome chega com justification vazia
ENTÃO rejeita com receivables-gateway/external-command/justification-missing

DADO um comando unknown_outcome há mais tempo que o ambiguityDeadline
QUANDO SweepExternalCommands roda
ENTÃO ExpireAmbiguity grava ambiguityExpiredAt, reconciliationRequired fica verdadeiro e ExternalCommandAmbiguityExpired é emitido uma única vez no episódio

DADO um comando acknowledged esperando callback há mais tempo que o ambiguityDeadline
QUANDO SweepExternalCommands roda
ENTÃO ExternalCommandAmbiguityExpired é emitido com status acknowledged

DADO um comando que saiu de unknown_outcome por reenvio e voltou a unknown_outcome
QUANDO o novo episódio de espera passa do ambiguityDeadline
ENTÃO ExternalCommandAmbiguityExpired é emitido de novo, contado a partir do novo waitingSince

DADO um comando unknown_outcome dentro do prazo
QUANDO ExpireAmbiguity chega
ENTÃO rejeita com receivables-gateway/external-command/ambiguity-deadline-not-reached

DADO um comando created
QUANDO CancelExternalCommand chega
ENTÃO o comando vai a cancelled e o handler do despacho descarta a intenção sem enviar ao parceiro

DADO um comando cancelado enquanto o handler do despacho já tinha iniciado o envio
QUANDO o envio volta ClassAccepted com eficácia on_submit
ENTÃO o comando vai a confirmed e ExternalCommandSubmitted e ExternalCommandConfirmed são emitidos, porque o fato externo prevalece

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
ENTÃO a interação vai a schema_mismatch e ExternalSchemaMismatchDetected é emitido com partnerId, environment e connectorKey

DADO uma interação stored sem comando correspondente
QUANDO ConcludeInteraction chega com outcome uncorrelated
ENTÃO a interação vai a uncorrelated e ExternalInteractionUncorrelated é emitido

DADO uma interação reprocessada cuja leitura nova confirma um comando já em final_failure
QUANDO ConcludeInteraction chega com a revisão 2
ENTÃO o comando não muda e ExternalInteractionReprocessed sai com terminalCommandAffected verdadeiro

DADO uma interação reprocessada cuja leitura não mudou
QUANDO ConcludeInteraction chega com a revisão 2
ENTÃO só ExternalInteractionReprocessed é emitido, com outcomeChanged falso

DADO uma interação já concluída
QUANDO ConcludeInteraction chega
ENTÃO rejeita com receivables-gateway/external-interaction/already-concluded

DADO uma interação stored
QUANDO ConcludeInteraction chega com schema_mismatch sem mismatchDetail
ENTÃO rejeita com receivables-gateway/external-interaction/mismatch-detail-missing

DADO uma interação normalized
QUANDO ReprocessInteraction chega com justificativa
ENTÃO a interação volta a stored com normalizationRevision + 1, previousOutcome = normalized e nenhum comando externo é reenviado

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

DADO uma operação com maxQueries igual a ports.MaxAttemptEntries
QUANDO DeclareCapabilityProfile chega
ENTÃO rejeita com receivables-gateway/capability-profile/external-profile-invalid

DADO um comando final_failure por resend_budget_exhausted na série de uma businessKey
QUANDO CreateExternalCommand chega com payloadVersion diferente na mesma série
ENTÃO o comando novo nasce em created e a intenção de despacho vai à outbox

DADO um comando unknown_outcome cuja tentativa já tem FindingApplied gravado no ledger
QUANDO ResolveUnknownOutcome chega com resolution final_failure, queryInteractionId e justificativa
ENTÃO rejeita com receivables-gateway/external-command/observation-conflict, o conflito fica gravado no ledger e a repetição com a mesma chave devolve a mesma rejeição

DADO um comando unknown_outcome cuja tentativa tem a linha do ledger cheia
QUANDO ResolveUnknownOutcome chega com resolution not_received, queryInteractionId e justificativa
ENTÃO rejeita com receivables-gateway/external-command/observation-saturated e o comando não muda

DADO um comando created cuja operação foi suspensa depois da criação
QUANDO o handler de gateway.dispatch recebe a intenção
ENTÃO nenhuma chamada externa sai, HoldDispatch grava dispatchHeldSince e a mensagem recebe Ack

DADO um comando created com dispatchHeldSince e a operação habilitada de novo
QUANDO SweepExternalCommands roda
ENTÃO RequestResend zera dispatchHeldSince e grava a intenção de despacho com o mesmo IntentID

DADO um comando cancelled
QUANDO o handler de gateway.dispatch recebe a intenção
ENTÃO nenhuma chamada externa sai e a mensagem recebe Ack

DADO um status terminal
QUANDO FindExternalCommandsByStatus chega com ele
ENTÃO a consulta é recusada como entrada inválida

DADO um comando created e o simulador de parceiro que aceita o envio
QUANDO o handler de gateway.dispatch o envia
ENTÃO o pedido e a resposta ficam gravados na porta de object storage, ExternalResponseCaptured vai à outbox na UoW do passo, e o consumer registra a interação de saída com requestVersionId e responseVersionId da porta

DADO a porta de object storage indisponível ao gravar o pedido do envio
QUANDO o handler de gateway.dispatch processa a intenção
ENTÃO nenhuma chamada externa sai e a mensagem volta à reentrega com a mesma chave

DADO uma interação de saída registrada com capturedStep suspend
QUANDO o consumer de gateway.normalizations a processa
ENTÃO ConcludeInteraction a conclui em schema_mismatch, ExternalSchemaMismatchDetected vai à outbox e nenhuma observação vai ao kernel

DADO um callback do parceiro com o desfecho confirmado de um comando acknowledged
QUANDO ReceivePartnerInteraction chega e a normalização conclui a interação
ENTÃO o bruto fica gravado na porta, a interação vai a normalized e o comando vai a confirmed pelo Resolve com Observation.ID = <interactionId>.r1

DADO um callback com partnerEventId já recebido
QUANDO ReceivePartnerInteraction chega de novo
ENTÃO a interação nova nasce duplicate, ExternalInteractionStored não é gravado e o comando não muda

DADO um corpo maior que maxPayloadBytes da operação
QUANDO ReceivePartnerInteraction chega
ENTÃO rejeita com receivables-gateway/external-interaction/payload-too-large e nada é gravado na porta

DADO uma interação normalized cuja leitura deixou o comando em acknowledged por erro de normalização
QUANDO ReprocessInteraction chega e a normalização corrigida lê o desfecho confirmado
ENTÃO a observação entra no kernel com Observation.ID = <interactionId>.r2, o comando vai a confirmed e ExternalInteractionReprocessed vai à outbox com outcomeChanged verdadeiro

DADO um comando unknown_outcome com consulta de estado e nextQueryAt vencido
QUANDO SweepExternalCommands roda
ENTÃO o bruto da consulta fica gravado na porta, a interação state_query é registrada, e o achado chega ao comando pelo consumer de gateway.normalizations

DADO uma QueryAuthorization ativa e o simulador de parceiro
QUANDO ReadExternal chega com queryAsset
ENTÃO a resposta traz interactionId, asOf, providerId, externalReference, reading e readingHash, e a interação external_read fica registrada com o bruto na porta
```

## Escopo fora

- **REST do bff**: recepção de webhook (verificação de origem, resposta em menos de 200 ms, absorver e enfileirar) e qualquer rota pública são do `bff` (91 §5.1 e §6), com spec própria; o contexto serve só gRPC e entrega `ReceivePartnerInteraction` para o `bff` chamar (decisão 19).
- **RPCs tipados das leituras externas**: chegam com os adaptadores reais (#5); até lá, `ReadExternal` genérico (decisão 20).
- **Consumo de `ConsentGranted` e `ConsentRevoked`**: terceira fatia, quando o Consent publicar o contrato (decisão 22).
- **Paginação e projeção sem payload na listagem por status**: pendência na relação do kernel (decisão 15).
- **Adaptadores reais de registradora e escriturador**: dependem de #5 (mapeamento de provedores, códigos de erro, homologação); esta versão entrega portas, taxonomia, suspensão, webhook, replay e o simulador de parceiro, com as partes que gravam bruto na segunda fatia (DEVS-174), sobre a porta de object storage da `1.0.0-rc.8` do kernel (ver Situação).
- **Verdade de negócio do portfólio e decisão de elegibilidade**: pertencem a Portfolio e Eligibility; o Gateway só devolve posições com proveniência.
- **Decisão de elegibilidade do parceiro e homologação**: Partner Management; o Gateway apenas aplica `CapabilityEnabled`, `CapabilitySuspended` e `IntegrationProfileRegistered`.
- **Caso operacional** por `schema_mismatch`, interação não correlacionada, prazo de ambiguidade vencido e reprocessamento que muda a leitura de comando terminal: Backoffice (17); o Gateway só publica o fato.
- **Multi-tenancy entre tenants**: leitura e escrita de plataforma entre tenants; o ADR-068 do DMPF exclui a escrita de plataforma (item 14). A fase 1 vive no tenant único da plataforma (decisão 1).
- **Timer interno de espera**: o kernel não tem timer nem entrega adiada; reenvio, consulta e prazo são da varredura acionada por agendador externo (decisão 10; ADR-022, linha 18).
- **Rate limit distribuído entre réplicas** (00 §7.4): extensão do kernel, pelo item 3 do ADR-022; a fase 1 usa limitador local por instância (ADR-022, linha 17).
- **Evento de abertura de circuito**: só telemetria (decisão 9).
- **Rollout por tenant**: `CapabilityRolloutChanged` não é consumido (decisão 4).
- **Fases posteriores**: escriturador (fase 2) e múltiplas infraestruturas simultâneas (fase 4, B14); recebíveis futuros dependem de B11.
- **Retenção do payload bruto**: sem política até 04:G11 e #8 (retenção); o armazenamento usa a porta do kernel da `1.0.0-rc.8` (SPEC-CWBA4BPV), sem retenção nem legal hold até lá (decisão 16).
- **Composition root**: o esqueleto de `app/`, o `cmd/` com `--role` e o `project.json` nascem do generator; o harness escreve o conteúdo de domínio dentro deles.
