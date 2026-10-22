# RegisterPerson

Comando que registra uma pessoa natural sem persistir o documento original. Rota OpenAPI destinada ao futuro BFF: `POST /identity/persons`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Protector as HMAC identifier protector
  participant Domain as Person.Register
  participant Repo as PersonRepository
  participant PG as PostgreSQL

  Client->>RPC: RegisterPerson(request, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: RegisterPerson(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>Protector: Normaliza documento e calcula HMAC
    Protector-->>UOW: document_hash + versão + máscara
    UOW->>Repo: Carrega PersonID (deve estar ausente)
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Ausente
    UOW->>Domain: Register(nome, hash, máscara)
    alt Entrada recusada
      Domain-->>UOW: Rejection, não persiste nem emite evento
    else Aceita
      Domain-->>UOW: Snapshot sem documento em claro
      UOW->>Repo: Save(snapshot)
      Repo->>PG: Persiste hash, máscara, versão e nome
      alt Documento já registrado no tenant
        PG-->>Repo: persons_tenant_id_document_hash_key
        Repo-->>UOW: Rejection declarada, sem estado nem evento
      end
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** nenhum evento de integração é publicado por `RegisterPerson`. O documento aparece no fingerprint apenas transitoriamente; a inbox guarda o digest HMAC do fingerprint, não o fingerprint em claro.
