# ProvisionUserAccount

Comando que provisiona a conta correspondente ao `sub` federado. Rota OpenAPI destinada ao futuro BFF: `POST /identity/users`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Repo as UserAccountRepository
  participant Domain as UserAccount.Provision
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Client->>RPC: ProvisionUserAccount(subject, person_id, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: ProvisionUserAccount(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>Repo: Carrega por subject ou inicia agregado New
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Existente ou ausente
    UOW->>Domain: Provision(person_id)
    alt Conta ausente (New)
      Domain-->>UOW: Ativa conta e associa pessoa
      UOW->>Repo: Save(snapshot)
      UOW->>Outbox: Enqueue(lista vazia, sem evento de negócio)
    else Conta já existe (inclusive Suspended)
      Domain-->>UOW: Aceita sem efeito de negócio
      UOW->>Repo: Salva decisão da UoW
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou erro
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** a primeira provisão torna a conta `Active`; uma conta já existente, inclusive suspensa, é aceita sem evento. A existência de conta não é autorização de sessão: a consulta de permissões continua verificando status.
