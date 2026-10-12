# ADR-020: Reter o inbox pelo menos pela janela de redelivery declarada por canal

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D9 vem de `docs/design/00-arquitetura-tecnica.md` §6.2: o consumidor
mantém o `inbox` com `eventId` como chave, e o design pede política de expurgo —
reter indefinidamente é inviável e expurgar cedo reabre a janela de duplicata. A
proposta do design é reter pelo dobro do tempo máximo de retenção do broker.

O kernel DMPF condiciona a escolha (classe **condicionada**):

- **INB-14** (`docs/dmpf/uow-inbox-outbox.md`):
  `retenção_inbox ≥ janela_redelivery`. Purgar uma chave antes de o transporte
  esgotar a própria janela de redelivery é erro de operação.
- **TRP-23**: a `janela_redelivery` é o horizonte em que o transporte ainda pode
  reentregar a mesma mensagem, não o intervalo entre tentativas, e é declarada
  por canal com a fórmula de cálculo.
- **TRP-24**: a fórmula tem variáveis nomeadas e unidade de tempo e produz um
  limite superior fechado. Em Kafka, é a retenção efetiva do registro (o menor
  entre tempo e tamanho, com compactação e armazenamento remoto) combinada com a
  política de posição inicial do grupo. Em SQS, `maxReceiveCount` × visibilidade
  por tentativa, limitado pela retenção da fila. Em SNS → SQS, a soma da janela
  da assinatura com a da fila.
- **TRP-24b**: o canal declara os parâmetros que instanciam a fórmula; sem eles,
  a janela não é verificável.
- **TRP-25**: replay operacional (DLQ, quarentena, log além da retenção) não é
  redelivery e não entra na janela.
- **INB-15**: todo replay declara a zona em que opera — dentro da retenção do
  inbox, com deduplicação e idempotência de efeito, ou além dela, só com
  idempotência de efeito.
- **INB-16**: a purga é operação com evidência — o que foi purgado, de qual
  consumidor e até qual instante.
- **ADR-056 §8**: `app.RunPurge` hospeda os laços de purga (`PurgeInbox` no
  `serve-consumer`, `PurgeExpiredInbox` no `serve-api`), com padrões de 192 h
  para o inbox, 168 h para a outbox e 24 h para a idempotência de comando.
- **Quem confere INB-14**: o kernel recusa canal sem janela declarada
  (`libs/backend/go/transport/channel/channel.go:193-195`, com limite superior
  em `window.go`), mas a comparação entre a retenção do inbox e a janela fica no
  `Validate` de cada contexto (`libs/backend/go/app/README.md:84`), como o
  `reservations` faz em `apps/backend/reservations/app/config.go:200-203`; o
  ADR-056 §8 descreve essa recusa como se fosse do `serve-consumer`.

## Decisão

- **Cada canal declara a `janela_redelivery`** na spec do contexto que o
  consome, com a fórmula de TRP-24 e os parâmetros de TRP-24b do transporte
  escolhido (ADR-015).
- **A retenção do inbox de cada consumidor satisfaz
  `retenção_inbox ≥ janela_redelivery`** (INB-14) para o canal de maior janela
  que ele consome. Uma margem é opcional e, quando usada, é declarada junto da
  retenção.
- **A purga usa os laços do kernel** (ADR-056 §8) e registra a evidência de
  INB-16; nenhum contexto purga inbox por SQL próprio.
- **Todo replay declara a zona** (INB-15). Replay além da retenção só é
  permitido sobre efeito idempotente por chave natural.
- **Todo contexto consumidor compara, no `Validate` da sua configuração, a
  retenção do inbox com o `Redelivery.UpperBound` de cada canal que consome**, e
  recusa a partida se a desigualdade falhar, com teste — no molde do
  `reservations`. O default de 192 h do kernel só vale se cobrir a janela.
- O produtor registra o transporte e o critério de TRP-37 (ADR-015); a janela é
  do consumidor, porque depende também da posição inicial do grupo dele.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Retenção igual ao dobro da retenção do tópico (doc 00 §6.2) | INB-14 compara com a `janela_redelivery`, não com a retenção do tópico; a regra não vale para SNS → SQS, em que a janela soma duas políticas (TRP-24), e o fator 2 não tem origem declarada |
| Dimensionar o inbox para cobrir qualquer replay | TRP-25 separa replay de redelivery; o replay não tem limite superior, e INB-15 trata o replay além da retenção pela idempotência de efeito |
| Reter o inbox indefinidamente | Crescimento sem limite e conflito com o teto de retenção do dado (DAT-14 a DAT-18) |

## Consequências

**Positivas:**

- A invariante é conferível por desigualdade entre dois prazos declarados, e a
  configuração do contexto recusa a partida que a viola.
- A purga deixa trilha: "nunca chegou" e "chegou e a chave foi removida" ficam
  distinguíveis.

**Negativas:**

- Cada canal paga a declaração da fórmula e dos parâmetros; mudar a retenção de
  um tópico Kafka muda a janela e pode exigir aumentar o inbox.
- Efeitos sem idempotência por chave natural ficam restritos a replay dentro da
  retenção.

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-015](./015-kafka-como-broker.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - INB-14 a INB-16 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/uow-inbox-outbox.md`
  - TRP-23 a TRP-25 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/politicas-transporte.md`
  - ADR-056 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/056-idempotencia-de-comando-pela-inbox.md`
  - DAT-14 a DAT-18 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/contexto-erros-seguranca.md`
  - Checagem de INB-14 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/apps/backend/reservations/app/config.go`, `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/app/README.md`, `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/transport/channel/channel.go`, `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/transport/channel/window.go`
