# InviteUser

Comando que cria um vínculo em estado `Invited`. Rota OpenAPI destinada ao futuro BFF: `POST /identity/organizations/{organizationId}/memberships`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant OrgRepo as OrganizationRepository
  participant Repo as MembershipRepository
  participant Domain as Membership.Invite
  participant PG as PostgreSQL

  Client->>RPC: InviteUser(organization, email, role, expiry, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: InviteUser(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>OrgRepo: Carrega organização e kinds
    OrgRepo->>PG: SELECT tenant-scoped
    PG-->>OrgRepo: Status e tipos da organização
    UOW->>Repo: Carrega MembershipID (deve estar ausente)
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Ausente
    UOW->>Domain: Invite(email, role, expiração, organização)
    alt Email/expiração inválidos, organização inativa ou role incompatível
      Domain-->>UOW: Rejection, sem persistir
    else Aceita
      Domain-->>UOW: Membership Invited
      UOW->>Repo: Save(snapshot)
      Note over UOW,Repo: Sem evento e sem envio de e-mail neste contexto
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** cria o convite persistido, mas não envia comunicação. Confirmação do canal e nível de verificação são pré-condições fornecidas depois a `GrantMembership`, não chamadas a um provedor externo por este caso de uso.
