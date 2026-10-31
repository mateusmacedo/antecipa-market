# ADR-022: Fazer as normas do kernel DMPF prevalecerem sobre os documentos de design

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

> **Addendum (2026-10-12)** — linhas 14 e 15 da tabela, achadas pela
> SPEC-7KW0HHF6 (DEVS-77).
>
> **Addendum (2026-10-17)** — reconferência do bump para o `1.0.0-rc.5`
> (`workflowRef` `b5131ee`), pelo item 4. Mudam as linhas 5 e 8: o kernel
> entregou a SPEC-QPRR24C4 (ADR-063), a SPEC-SBW7HT6Y (ADR-064) e a
> SPEC-STEJG347 (ADR-065). As demais linhas seguem como estavam, porque as
> specs que as tratam continuam em `backlog` no kernel.
>
> **Addendum (2026-10-19)** — a SPEC-QVDGV6J0 (DEVS-100), citada nas linhas 2 e
> 15, foi entregue no kernel `1.0.0-rc.6` (ADR-068 do kernel). A tabela segue
> como estava até a reconferência do bump, pelo item 4, que fica com a adoção
> da `1.0.0-rc.6` (DEVS-136).
>
> **Addendum (2026-10-21)** — a DEVS-136 adotou o kernel `1.0.0-rc.7`
> (`workflowRef` `ddd3d0c`), que inclui a rc.6. A linha 15 deixa de ser
> divergência: a SPEC-7SRQ3RPK (ADR-066 do kernel) entrega a unicidade
> declarada, e identity e partner a adotam. Cada `CONSTRAINT ..._key UNIQUE`
> tem `postgres.Unique` com a rejeição de domínio, e a pré-checagem por
> leitura sai; o partner também troca os índices parciais da v1 de cada série
> por `series_origin`. A leitura das pré-condições (†) do partner passa para a
> UoW sob a guarda da SPEC-HFDSHFR1 (ADR-067 do kernel). A linha 14 segue: a
> porta de cifra em envelope (SPEC-CC5SHV1K, DEVS-102) entrou na rc.7, mas a
> adoção fica para outra issue. As demais linhas não foram reconferidas nesta
> adoção.
>
> **Addendum (2026-10-31)** — a triagem da SPEC-N1X148AW (DEVS-84 e DEVS-122)
> acrescenta a linha 16, achada no Partner já implementado: `partner.events`
> leva 11 tipos de evento e o catálogo do canal declara só um. Acrescenta
> também as linhas 17 (rate limit distribuído) e 18 (tempo de espera do comando
> externo), achadas na adoção do ciclo de comando externo da rc.7. A linha 11 deixa
> de ser divergência: a SPEC-5AQHHMJ0 (ADR-070 do kernel) está na `1.0.0-rc.7`,
> e a SPEC-N1X148AW passa a usar o ciclo de comando externo do kernel. A linha
> 12 segue: a SPEC-CWBA4BPV (DEVS-95) está em `planning` no kernel e não entrou
> na `1.0.0-rc.7`.

## Contexto

Os documentos de `docs/design/` foram escritos antes de o workspace adotar o
kernel DMPF (ADR-011) e recomendam, em vários pontos, soluções que as normas do
kernel decidem de outro modo. As specs do catálogo de bounded contexts citam os
dois lados. Sem uma regra de precedência, cada spec decidiria a divergência por
conta própria.

`docs/README.md` trata `design/` como camada que muda quando a implementação
evolui, mas reescrever 24 documentos apagaria o raciocínio original e a
rastreabilidade das lacunas.

## Decisão

1. Onde `docs/design/` contradiz uma norma do kernel DMPF na versão consumida
   (hoje o `1.0.0-rc.5`, `workflowRef` `b5131ee`), **prevalece o kernel**. O
   documento de design continua valendo como requisito de produto e como
   raciocínio de origem.
2. `docs/prd/`, `docs/design/` e `docs/reference/` não são editados para
   refletir a precedência; a tabela abaixo é o ponto único de reconciliação.
3. Onde o kernel não cobre uma necessidade do design, a extensão nasce como spec
   no repositório do kernel, nunca como código paralelo neste workspace.
4. Divergência nova, achada por uma spec de contexto, entra nesta tabela por
   addendum datado. Cada bump do kernel (ADR-011) reconfere a tabela.

| # | Divergência | Doc de design | Norma DMPF | Resolução | Onde fica tratada |
| --- | --- | --- | --- | --- | --- |
| 1 | Estilo de implantação | 00 §3.1: monolito modular, schema por módulo | ADR-044, ADR-053 §1, ADR-054 | Um app por bounded context, banco e role próprios | [ADR-012](./012-um-app-por-bounded-context.md) |
| 2 | Multi-tenancy | 00 §4.1: RLS com `SET LOCAL` como mecanismo primário | ADR-051, IDN-14 | Choke point no `postgres.Table`; RLS só como segunda camada | [ADR-013](./013-multi-tenancy-por-execution-context.md); SPEC-QVDGV6J0 (DEVS-100) |
| 3 | Broker | 00 §6.3: fila gerenciada FIFO | TRP-37 a TRP-41, ADR-025 | Kafka como alvo, com critério por canal; SNS/SQS nos casos de TRP-40 | [ADR-015](./015-kafka-como-broker.md); SPEC-SWN93XHJ (DEVS-98) |
| 4 | Modelo físico de dados | 90: tabelas largas; 00 §5.3 e 93 RNF-V4: partição mensal | ADR-051, ADR-053 §4 | Persistência híbrida; o doc 90 vale como modelo lógico | [ADR-014](./014-postgresql-persistencia-hibrida.md) |
| 5 | Imutabilidade por permissão de banco | 00 §5.2: sem `UPDATE`/`DELETE` por permissão | ADR-063 (addendum 2026-10-17): role migradora `<app>_migrator` dona do banco e role de runtime `<app>` só com o privilégio declarado por tabela, por opt-in no bloco `database.migrator`; ADR-051 | Role de runtime separada da dona, mais trigger. Os grants por tabela já valem para todo contexto; a role migradora separada ainda não foi adotada por nenhum contexto deste workspace | [ADR-018](./018-imutabilidade-do-ledger.md); SPEC-QPRR24C4 (DEVS-94), entregue no `1.0.0-rc.5` |
| 6 | Envelope de eventos | 92: JSON com `schemaUri`, sem chave de partição | ADR-022 e ADR-023 do kernel; `partitionkey` obrigatório | CloudEvents Protobuf com Buf; `partitionkey` igual ao id do agregado. A validação contra registry na publicação (00 §3.2) não tem equivalente em runtime: o Buf valida em CI, e o registry em runtime é pendência do kernel (ADR-023) | Este ADR; [ADR-015](./015-kafka-como-broker.md); specs de contexto |
| 7 | Borda | 91: Client API, Partner API e webhooks REST por contexto | ADR-024, ADR-044, ADR-053 §6 | REST externo e recepção de webhook só no `bff`; contextos em gRPC | [ADR-012](./012-um-app-por-bounded-context.md); SPEC-N1X148AW |
| 8 | Autenticação | 01: federação por organização e revogação em 60 s | Addendum 2026-10-17: `authn` com issuer único e tenant pelo caminho `organization.*.tenant_id`, que casa exatamente uma organização (ADR-064, IDN-20); deny-list por `sub` e `sid` no Redis, alimentada pelo papel `session-events` do `bff` (ADR-065); IDN-01 | Keycloak de referência do kernel, com o mapper nativo de organização e sem mapper próprio, e deny-list de sessão consultada em toda requisição, que supera a lista com cache de 30 s do doc 01 §2.2 | [ADR-016](./016-provedor-de-identidade-oidc.md) (addendum 2026-10-17); SPEC-SBW7HT6Y (DEVS-97) e SPEC-STEJG347 (DEVS-101), entregues no `1.0.0-rc.5` |
| 9 | Tempo | 00 §8: UTC com milissegundo | ADR-034, ADR-035, ADR-053 §8 | Instante de domínio em `int64` de nanossegundos; data de negócio segue data pura com calendário | Este ADR; specs de contexto |
| 10 | Ambientes | 00 §11: desenvolvimento, integração, homologação e produção | ADR-054 §4 e §7 (overlays dev e hmg, infra de testes) | Dev e hmg pelo baseline; integração coberta por hmg e pela infra de testes; produção pendente | [ADR-019](./019-go-sobre-o-kernel-dmpf.md) |
| 11 | Comando externo de saída | 00 §7.2 e §7.3: `UNKNOWN_OUTCOME`, consulta de estado, chave idempotente de saída | ADR-056 cobre a entrada; addendum 2026-10-31: ADR-070, ciclo de saída com `Dispatch`, `Resolve` e `Abandon`; RES-25 a RES-28 | Ciclo de comando externo de saída do kernel. Deixou de ser divergência: a extensão entrou na `1.0.0-rc.7` | SPEC-5AQHHMJ0 (DEVS-96), entregue na `1.0.0-rc.7`; SPEC-N1X148AW, SPEC-ZFDEPFWC, SPEC-CWYGNY82 |
| 12 | Evidência bruta em object storage | 00 §5.2 | Sem porta nem provider de object storage (a capability `io.storage` existe, ADR-015 do kernel) | Porta de object storage com provider S3 | [ADR-018](./018-imutabilidade-do-ledger.md); SPEC-CWBA4BPV (DEVS-95) |
| 13 | Dados de homologação | 00 §11: anonimizados | Sem norma; o kernel cobre só a infra de teste (ADR-054 §7) | Dados sintéticos | [ADR-021](./021-dados-de-teste-sinteticos.md) |
| 14 | Cifra de campo pessoal (addendum 2026-10-12) | 90 §1.4: envelope adicional para `PII`; envelope com chave segregada para `PII_SENSITIVE` | Sem porta de cifra de campo: SPEC-MSA607GT exclui a cifra por KMS e SPEC-CWBA4BPV cifra só objeto em repouso | Porta de cifra em envelope com chave mestra no KMS. Até a implementação, `PII_SENSITIVE` guardado só como hash e máscara, e `PII` sem envelope | SPEC-CC5SHV1K (DEVS-102); SPEC-7KW0HHF6 |
| 15 | Unicidade de negócio (addendum 2026-10-12) | 90 §3.1: índice único `(taxIdHash, jurisdiction)` sem tenant, para o requisito 1 do PRD 01 | ADR-051: `tenant_id` à frente de toda chave; violação de índice único fora da PK volta como erro técnico, não como rejeição | Unicidade por tenant traduzida em rejeição de domínio. Até a implementação, pré-checagem por leitura com o índice como salvaguarda. Unicidade entre tenants pendente de registro na plataforma | SPEC-7SRQ3RPK (DEVS-103); SPEC-QVDGV6J0 (DEVS-100); SPEC-7KW0HHF6 |
| 16 | Tópico com vários tipos de evento (addendum 2026-10-31) | 92 §4: catálogo de eventos por contexto produtor, sem regra de tópico; o Partner implementado publica todos os seus eventos em `partner.events` | `KFK-03`: um tipo por tópico; `KFK-03b`: vários tipos exigem conjunto fechado declarado no catálogo do canal, variação modelada no contrato e ordenação válida para o conjunto inteiro | Canal `<ctx>.events` com as três condições da `KFK-03b` declaradas no catálogo, e ordem só por agregado (`partitionkey`); o consumidor filtra por `Sink` de conjunto fechado e dá `Ack` sem inbox aos demais tipos. Canal interno de um tipo só segue a `KFK-03`. O catálogo multi-evento no kernel é a SPEC-Z2HM6NAP (DEVS-58, em `backlog`) | SPEC-N1X148AW; SPEC-9TEQ9EV9 e DEVS-171 (fechar o catálogo de `partner.events`) |
| 17 | Rate limit distribuído (addendum 2026-10-31) | 00 §7.4: rate limit distribuído por parceiro e operação | O kernel fixa só a posição do slot `rate_limit` na composição de resiliência (`observability/resilience`), sem limitador | Na fase 1, limitador local por instância no slot, com cota dividida pelo número de réplicas declarado na composition root; o limitador distribuído nasce como spec no kernel, pelo item 3 | SPEC-N1X148AW |
| 18 | Tempo de espera do comando externo (addendum 2026-10-31) | 00 §7.2.5: limite configurado de resultado desconhecido que abre caso operacional | ADR-070 do kernel, item 6: quiescência, intervalo entre consultas e orçamento de reenvio são de quem chama; o kernel não tem timer de espera | Varredura do contexto (`SweepExternalCommands`) acionada por agendador externo: reenvio depois da quiescência, consulta no intervalo do perfil e escalonamento do prazo | SPEC-N1X148AW |

As specs SPEC-QPRR24C4, SPEC-CWBA4BPV, SPEC-MSA607GT, SPEC-QVDGV6J0,
SPEC-5AQHHMJ0, SPEC-SBW7HT6Y, SPEC-STEJG347, SPEC-SWN93XHJ, SPEC-CC5SHV1K,
SPEC-7SRQ3RPK e SPEC-Z2HM6NAP vivem no repositório do kernel (`docs/specs/`), no projeto DMPF
do Linear.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Reescrever os documentos de design | Custo alto e perda do raciocínio original e da rastreabilidade das lacunas |
| Precedência do design sobre o kernel | Exigiria manter um fork do kernel ou código paralelo, contra o consumo por versão (ADR-011) |
| Decidir a divergência em cada spec de contexto | 17 specs decidiriam a mesma divergência de formas diferentes |

## Consequências

**Positivas:**

- Um só lugar para saber o que vale quando design e kernel divergem.
- Cada lacuna do kernel tem dono e issue no repositório que a implementa.

**Negativas:**

- Quem lê o design precisa consultar esta tabela; os documentos não avisam a
  divergência.
- O bump do kernel pode criar ou fechar linhas, o que exige revisão da tabela.

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- SPEC-7KW0HHF6 — `docs/specs/SPEC-7KW0HHF6-identity-organization.md` (addendum 2026-10-12)
- [ADR-011](./011-template-consome-dmpf-por-versao.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`), base
  `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/`:
  - `docs/adr/015-capabilities-externas-por-bloco.md`, `docs/adr/022-codec-cloudevents-proto-data-e-payload-hash.md`, `docs/adr/023-autoridade-de-validacao-e-registry.md`, `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md`, `docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md`
  - `docs/adr/034-fronteira-de-uow-em-go.md`, `docs/adr/035-realizacao-postgres-da-outbox.md`, `docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md`, `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md`
  - `docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md`, `docs/adr/054-apps-autocontidos-e-infras-separadas.md`, `docs/adr/056-idempotencia-de-comando-pela-inbox.md`
  - `docs/dmpf/cloudevents-protobuf-buf.md` (`partitionkey`), `docs/dmpf/politicas-transporte.md` (TRP), `docs/dmpf/contexto-erros-seguranca.md` (IDN), `docs/dmpf/resiliencia-observabilidade.md` (RES)
  - `libs/backend/go/authn/README.md`
- Kernel DMPF, no `workflowRef` do rc.5 (`b5131ee`), base
  `https://github.com/mateusmacedo/dmpf/blob/b5131eea0ce4c593e11e39f339ea9daea1f53ce4/`
  (addendum 2026-10-17):
  - `docs/adr/063-duas-roles-de-banco-por-app.md`, `docs/adr/064-keycloak-de-referencia-sem-extensao.md`, `docs/adr/065-revogacao-de-sessao-no-authn.md`
