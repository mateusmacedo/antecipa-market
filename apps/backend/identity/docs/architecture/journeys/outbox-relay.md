# Jornada — commit transacional e publicação pela outbox

Somente os comandos que produzem um evento de domínio escrevem uma entrada de outbox. O relay publica após o commit; não há consumer identity nem saga.

```mermaid
sequenceDiagram
  actor Caller as Chamador gRPC
  participant API as identity --role api
  participant UOW as PostgreSQL UnitOfWork
  participant Domain as Aggregate/domain decision
  participant Outbox as Outbox DMPF
  participant Relay as identity --role relay
  participant Kafka as Kafka identity.events

  Caller->>API: Comando + idempotency-key
  API->>UOW: Inicia UoW e executa idempotência
  UOW->>Domain: Carrega e decide
  alt Recusa de domínio
    Domain-->>UOW: Rejection sem evento
    UOW->>UOW: Registra outcome na inbox
    UOW-->>Caller: Commit da resposta idempotente, sem alteração de agregado
  else Aceite sem evento
    Domain-->>UOW: Aceite e zero eventos
    UOW->>UOW: Persiste snapshot/inbox
    UOW-->>Caller: Commit e resposta
  else Aceite com evento
    Domain-->>UOW: Snapshot + evento
    UOW->>UOW: Salva snapshot, inbox e outbox na mesma transação
    UOW-->>Caller: Commit e resposta antes da publicação
  end
  loop Varredura do relay
    Relay->>Outbox: Claim lote vencido e elegível
    Outbox-->>Relay: Registros e metadados de contexto
    Relay->>Relay: Monta envelope CloudEvents e preserva payload
    Relay->>Kafka: Publish(destination, envelope)
    alt Publicado
      Kafka-->>Relay: Ack
      Relay->>Outbox: Marca registro como publicado
    else Erro transitório de publicação
      Kafka-->>Relay: Erro
      Relay->>Outbox: Agenda tentativa com backoff
    else Esgotadas tentativas
      Relay->>Outbox: Marca estado terminal failed
      Note over Relay,Outbox: Retomada exige intervenção manual
    end
  end
```

## Política operacional

- O commit do comando e a gravação na outbox são atômicos; uma recusa de domínio não grava agregado nem evento.
- A tentativa de publicação acontece em outro processo, selecionado por `--role relay`.
- Configuração atual: até 24 tentativas; backoff exponencial de 200 ms até 5 min; retenção de 168 h para eventos publicados; purga em lotes de 1000 a cada 15 min.
- Falha do broker também conta como tentativa. Após o estado terminal `failed`, a operação exige retomada manual.
- O relay serializa o envelope CloudEvents e publica no tópico configurado `identity.events`; o contexto não consome o tópico nem dispara saga.
