# SuspendUser

Comando que suspende uma conta ativa. Rota OpenAPI destinada ao futuro BFF: `POST /identity/users/{userId}/suspend`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Repo as UserAccountRepository
  participant Domain as UserAccount.Suspend
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Client->>RPC: SuspendUser(user_id, reason, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: SuspendUser(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>Repo: Carrega UserAccount existente
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Snapshot ou não encontrado
    UOW->>Domain: Suspend(reason)
    alt Conta não ativa ou motivo vazio
      Domain-->>UOW: Rejection, sem estado/evento
    else Aceita
      Domain-->>UOW: Estado Suspended + UserSuspended
      UOW->>Repo: Save(snapshot)
      UOW->>Outbox: Enqueue(UserSuspended)
      Outbox->>PG: Persiste evento na transação
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** suspensão é terminal no conjunto de operações atual: não existe reativação. `FindEffectivePermissions` falha fechado para conta suspensa.
