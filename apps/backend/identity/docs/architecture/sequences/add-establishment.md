# AddEstablishment

Comando que acrescenta estabelecimento a organização ativa. Rota OpenAPI destinada ao futuro BFF: `POST /identity/organizations/{organizationId}/establishments`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Protector as HMAC identifier protector
  participant Reader as EstablishmentsByTaxIDReader
  participant OrgRepo as OrganizationRepository
  participant Domain as Establishment.Add
  participant Repo as EstablishmentRepository
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Client->>RPC: AddEstablishment(request, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: AddEstablishment(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Retorna resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>Protector: Calcula HMAC do tax_id
    Protector-->>UOW: hash + key version
    UOW->>Reader: Pré-checa unicidade no par org/hash
    Reader->>PG: SELECT pelo hash (pool de leitura)
    PG-->>Reader: Existente ou ausente
    UOW->>OrgRepo: Carrega organização
    OrgRepo->>PG: SELECT tenant-scoped
    PG-->>OrgRepo: Status da organização
    UOW->>Repo: Carrega ID do estabelecimento (deve estar ausente)
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Ausente
    UOW->>Domain: Add(org status, hash, receiver, referências)
    alt Organização inativa ou identificador duplicado
      Domain-->>UOW: Rejection, sem mudança nem evento
    else Aceita
      Domain-->>UOW: Snapshot + EstablishmentAdded
      UOW->>Repo: Save(snapshot)
      UOW->>Outbox: Enqueue(EstablishmentAdded)
      Outbox->>PG: Persiste na transação atual
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite, rejection ou erro
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** persiste hash, máscara e metadados; `EstablishmentAdded` é enfileirado atomicamente com a mudança. O CNPJ pode continuar no snapshot em claro nesta versão.
