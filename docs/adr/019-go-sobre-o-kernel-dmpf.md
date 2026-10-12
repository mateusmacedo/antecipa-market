# ADR-019: Construir em Go sobre o kernel DMPF, com AWS como nuvem-alvo e floci como emulador

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D8 vem de `docs/design/00-arquitetura-tecnica.md` §12, que marca a
linguagem e a plataforma de execução como impacto alto, sem indicação nos PRDs.

Parte da decisão é **imposta** pelo kernel DMPF que o workspace consome por
versão (ADR-011):

- O backend é Go: os bounded contexts são módulos Go gerados pelo
  `bounded-context` do plugin, sobre as libs de `libs/backend/go` do kernel.
- Cada app é autocontido, com `deploy/` contendo Kustomize
  (`k8s/{base,overlays/{dev,hmg}}`), `compose.yml`, `.env.example` e
  `infra.json`, e a infra agregada é gerada pelo `infrasync` (ADR-054 §4).
- O runtime local roda no host sobre a infra de Compose (ADR-054 §5), e os testes
  usam uma infra separada, só em tmpfs (ADR-054 §7).
- A infra de referência do kernel tem Postgres, Redpanda, Redis, PKI,
  observabilidade (Collector, Prometheus, Grafana, Loki, Tempo) e o floci
  2.0.1, emulador AWS escolhido sobre o ElasticMQ, hoje só com SQS e SNS
  (ADR-039; `infra/local/compose/floci.yml`), também na infra de testes. As
  bases Kustomize cobrem Postgres e Redpanda; o overlay `hmg` sobe só os apps e
  a observabilidade, com Postgres e Kafka externos ao cluster
  (`infra/k8s/overlays/hmg/kustomization.yaml`).

A outra parte é **restrição do usuário** (12/10/2026): a infraestrutura segue a
abstração do DMPF, a nuvem-alvo é a AWS, o floci é o emulador e os recursos de
infra vêm do repositório do kernel.

## Decisão

- **Linguagem do backend**: Go, sobre o kernel DMPF, na versão fixada no
  `catalog:` (ADR-011).
- **Plataforma de execução**: um container por app (ADR-012), com Docker,
  Compose e Kustomize por app, conforme o ADR-054; dev e testes usam a infra
  de referência do kernel, e o hmg usa Postgres e Kafka externos.
- **Nuvem-alvo**: AWS. Serviços gerenciados entram pela abstração do kernel —
  porta no kernel, provider AWS e emulação pelo floci — e nunca por SDK chamado
  direto de um contexto.
- **Emulação em dev e nos testes**: floci para os serviços AWS (SQS, SNS, S3, KMS),
  Redpanda para Kafka (ADR-015), Keycloak de referência para identidade
  (ADR-016).
- **Teste de contrato contra a AWS real** para toda capacidade em que o
  floci divergir do serviço real, a começar por S3 Object Lock e KMS
  (SPEC-CWBA4BPV, DEVS-95; SPEC-MSA607GT, DEVS-99, no repositório do kernel).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Outra linguagem de backend (TypeScript/NestJS, JVM) | O kernel DMPF, com outbox, inbox, escopo de tenant e verificador de unidades, é Go; trocar a linguagem significa abandonar o kernel |
| Nuvem neutra, sem alvo declarado | Deixa sem escolha concreta o WORM do ledger (ADR-018), o broker gerenciado (ADR-015) e o KMS; o usuário fixou a AWS |
| LocalStack como emulador | Vetado no kernel (ADR-039); o floci já está na infra de referência |
| SDK da AWS direto nos contextos | Quebra a abstração do DMPF e o teste com fake e kit de conformidade |

## Consequências

**Positivas:**

- Um só toolchain e um só conjunto de gates para os 17 contextos.
- Dev e testes sobem com a infra que o kernel já mantém e testa.
- A nuvem-alvo fixa permite decidir o WORM, o KMS e o broker gerenciado.

**Negativas:**

- O floci é um projeto novo, criado em fevereiro de 2026, e a 2.0.1 diverge da
  AWS em S3 Object Lock e KMS; a infra precisa subir para a 2.2.0 ou superior
  (SPEC-CWBA4BPV) e manter teste de contrato contra a AWS real.
- O que o kernel ainda não tem (object storage, assinatura, Keycloak) depende de
  specs do kernel antes de o contexto que precisa delas ser construído.

## Pendências

- **Stack de front-end**: o kernel não cobre UI. O workspace traz os plugins
  Nx de Next.js e Angular, mas a escolha fica para a spec do primeiro
  front-end, sem spec dona ainda.
- **Infraestrutura AWS de hmg e de produção** (sem spec dona ainda): contas,
  IaC, orquestrador (EKS ou ECS), Postgres e Kafka gerenciados (ADR-015),
  Keycloak (ADR-016) e a conta de contrato onde rodam os testes contra S3 e KMS
  reais. Até ela existir, esses testes de contrato não têm onde rodar e a
  garantia de WORM e de assinatura fica provada só no floci.
- **floci 2.2.0 ou superior com S3 e KMS** na infra do kernel: SPEC-CWBA4BPV
  (DEVS-95).

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-011](./011-template-consome-dmpf-por-versao.md),
  [ADR-012](./012-um-app-por-bounded-context.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - ADR-039 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/039-providers-de-transporte-sink-e-gesto-de-release.md`
  - ADR-054 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/054-apps-autocontidos-e-infras-separadas.md`
  - floci na infra local — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/infra/local/compose/floci.yml`
  - overlay `hmg` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/infra/k8s/overlays/hmg/kustomization.yaml`
- floci — `https://github.com/floci-io/floci`
