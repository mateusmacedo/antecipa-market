# ADR-015: Adotar Kafka como transporte-alvo dos eventos de domínio, com decisão registrada por canal

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D4 vem de `docs/design/00-arquitetura-tecnica.md` §6.3. O broker
precisa de ordenação por chave, retenção configurável, consumo por grupo e
mensagem morta. O design recomenda, de forma condicional, uma fila gerenciada
com ordenação por grupo (SQS FIFO), porque o Compliance Ledger já provê
reprodução histórica e o broker não precisaria ser fonte de replay.

O kernel DMPF condiciona a escolha (classe **condicionada**), em
`docs/dmpf/politicas-transporte.md` §8 e no ADR-025:

- **TRP-37**: o transporte é decidido por critérios declarados, nesta ordem:
  quem consome, acoplamento temporal, cardinalidade do destino, ordenação
  exigida, retenção e reprocessamento. O primeiro que discrimina decide.
- **TRP-38**: a matriz põe evento de domínio com consumidor interno, em caso
  novo, em Kafka (CloudEvents em Protobuf, at-least-once, ordem por partição pela
  chave declarada); fan-out com filtro por atributo em SNS → SQS; trabalho
  enfileirado sem ordem em SQS standard.
- **TRP-39**: Kafka é o transporte-alvo; canal novo usa Kafka, salvo quando um
  critério de TRP-37 discrimina a favor de SNS/SQS, e o caso é registrado no
  canal com o critério que o decidiu.
- **TRP-40**: SNS/SQS é normatizado, não tolerado, e é escolha técnica superior
  em dois casos: fan-out com filtro por atributo e fila de trabalho sem ordem.
- **TRP-41**: enquanto a política Kafka (§11) não passar por revisão de
  infraestrutura de mensageria, ela vale como norma de desenho e não como
  autorização de operação. O documento registra a ausência de base as-is de
  Kafka.

O kernel já tem providers de Kafka e de SNS/SQS (ADR-039), com Redpanda e floci
como emuladores. A nuvem-alvo é a AWS (ADR-019).

## Decisão

- **Eventos de domínio entre contextos vão para Kafka**, com `partitionkey` igual
  ao identificador do agregado, o que dá a ordem por agregado que o doc 00 §6.1
  exige.
- **SNS → SQS** só nos dois casos de TRP-40: fan-out para N consumidores com
  filtro por atributo, e fila de trabalho sem ordem (SQS standard).
- **Cada canal registra, na spec do contexto produtor, o transporte e o critério
  de TRP-37** que o decidiu, conforme TRP-39; a `janela_redelivery` é
  declarada na spec do consumidor (ADR-020).
- **Na AWS**, Kafka é o Amazon MSK e SNS/SQS são os serviços nativos. **Em dev e
  na infra de testes**, Redpanda e floci, da infra de referência do kernel; o hmg
  do kernel usa Kafka externo ao cluster (`infra/k8s/overlays/hmg`), sem
  provisionamento no kernel.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| SQS FIFO como transporte universal (doc 00 §6.3) | Contraria TRP-39 para evento de domínio novo. A ordem por chave já é atendida por partição, e o critério de replay não discrimina, porque o ledger provê a reprodução histórica |
| SNS/SQS como alvo, por familiaridade operacional | TRP-39 fixa Kafka como alvo; SNS/SQS fica nos casos de TRP-40 |
| Publicar o mesmo canal nos dois transportes | Recusado pelo kernel (`ADR-DMPF-P` em `politicas-transporte.md`) |

## Consequências

**Positivas:**

- O desenho segue a norma do kernel e os providers já testados.
- O replay por offset fica disponível para reprocessar projeções sem depender só
  do ledger.
- A escolha por canal fica auditável: o critério está escrito na spec.

**Negativas:**

- Kafka é o transporte com menos base operacional na organização: o kernel
  declara zero clientes Kafka no acervo.
- Operar MSK custa mais e exige dimensionar partições e retenção, que dependem
  da volumetria (`#12`), ainda hipótese.
- Cada canal paga o registro do critério e da `janela_redelivery`.

## Pendências

- **Autorização de operação do Kafka (TRP-41)**: SPEC-SWN93XHJ (DEVS-98), no
  repositório do kernel — revisão de infraestrutura de mensageria com alvo MSK
  em produção. Até lá, os contextos são desenhados e
  testados sobre Kafka, e a operação em produção depende da revisão.
- **Volumetria (`#12`)**: dimensiona partições e retenção, sem mudar a escolha.

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-019](./019-go-sobre-o-kernel-dmpf.md),
  [ADR-020](./020-expurgo-do-inbox.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - TRP-37 a TRP-41 (§8) — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/politicas-transporte.md`
  - `partitionkey` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/cloudevents-protobuf-buf.md`
  - ADR-025 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md`
  - ADR-039 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/039-providers-de-transporte-sink-e-gesto-de-release.md`
  - overlay `hmg` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/infra/k8s/overlays/hmg/kustomization.yaml`
