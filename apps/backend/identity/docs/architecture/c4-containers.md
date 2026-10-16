# Containers e processos — identity

`cmd/identity` é um único binário Go executado em papéis separados com `--role`. `api` e `relay` não compartilham processo; o PostgreSQL é o ponto de integração durável entre eles.

```mermaid
C4Container
  title Containers — bounded context identity

  System_Boundary(identity, "Bounded context identity") {
    Container(api, "identity --role api", "Go, gRPC, DMPF", "22 operações; valida request, autoriza tenant e executa comandos/consultas")
    Container(relay, "identity --role relay", "Go, DMPF relay", "Drena eventos confirmados da outbox e os publica")
    ContainerDb(db, "Banco identity", "PostgreSQL", "Agregados, snapshots, inbox, outbox e quarantine")
  }
  ContainerQueue(topic, "identity.events", "Kafka", "Eventos de integração em envelope CloudEvents")
  System_Ext(otel, "Backend de telemetria", "Recebe traces, métricas e logs via OTLP")
  System_Ext(client, "Chamador gRPC interno", "Passa tenant e identidade autenticada no contexto")

  Rel(client, api, "Invoca IdentityService", "gRPC; mTLS conforme configuração")
  Rel(api, db, "Lê em pool separado; grava comandos na UoW", "pgx/PostgreSQL")
  Rel(relay, db, "Leases, publica e purga entradas da outbox", "pgx/PostgreSQL")
  Rel(relay, topic, "Publica eventos", "Kafka + CloudEvents")
  Rel(api, otel, "Exporta telemetria", "OTLP")
  Rel(relay, otel, "Exporta telemetria", "OTLP")
```

## Papéis e armazenamento

- `api`: serve gRPC, aplica migrações se `MIGRATE=true` e purga entradas vencidas da inbox. Usa pools distintos para transações e consultas consultivas/reads.
- `relay`: lê a outbox no mesmo banco, publica `identity.events` e purga eventos publicados após a retenção. Não consome canal.
- O schema é híbrido: colunas tipadas para filtros/índices e `snapshot` JSONB para o restante do agregado. O [ERD](./data-model.md) mostra as tabelas do contexto.
- `inbox`, `outbox` e `quarantine` são estruturas do kernel DMPF. A gravação do estado e a entrada na outbox pertencem à mesma transação.
- A chave HMAC é exigida pelo `api`; credenciais do Kafka são exigidas pelo `relay`. A chave de identificador não é compartilhada com o relay.
