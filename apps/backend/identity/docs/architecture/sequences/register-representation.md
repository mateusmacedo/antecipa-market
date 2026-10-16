# RegisterRepresentation

Comando que declara poderes de uma pessoa para uma organização. Rota OpenAPI destinada ao futuro BFF: `POST /identity/organizations/{organizationId}/representations`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant OrgRepo as OrganizationRepository
  participant PersonRepo as PersonRepository
  participant Domain as Representation.Register
  participant Repo as RepresentationRepository
  participant PG as PostgreSQL

  Client->>RPC: RegisterRepresentation(request, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: RegisterRepresentation(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>OrgRepo: Lê status da organização na UoW
    OrgRepo->>PG: SELECT tenant-scoped
    PG-->>OrgRepo: Status ou ausente
    UOW->>PersonRepo: Confirma existência da pessoa na UoW
    PersonRepo->>PG: SELECT tenant-scoped
    PG-->>PersonRepo: Encontrada ou ausente
    UOW->>Repo: Carrega RepresentationID (deve estar ausente)
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Ausente
    UOW->>Domain: Register(poder, evidência, alçada, vigência, precondições)
    alt Organização/pessoa inválida, evidência ou escopo inválidos
      Domain-->>UOW: Rejection, sem persistir
    else Aceita
      Domain-->>UOW: Snapshot em estado Declared
      UOW->>Repo: Save(snapshot)
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** não publica evento. A representação inicia `Declared`; não há neste contexto operações de verificação/revogação nem transição para outro estado.
