# RevokeMembership

Comando que revoga um vínculo ativo. Rota OpenAPI destinada ao futuro BFF: `POST /identity/memberships/{membershipId}/revoke`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Repo as MembershipRepository
  participant Domain as Membership.Revoke
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Client->>RPC: RevokeMembership(membership_id, reason, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: RevokeMembership(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>Repo: Carrega Membership existente
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Snapshot ou não encontrado
    UOW->>Domain: Revoke(reason)
    alt Membership não está Active
      Domain-->>UOW: Rejection, sem mudança/evento
    else Aceita
      Domain-->>UOW: Estado Revoked + MembershipRevoked
      UOW->>Repo: Save(snapshot)
      UOW->>Outbox: Enqueue(MembershipRevoked)
      Outbox->>PG: Persiste evento na transação
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** vínculo passa de `Active` a `Revoked`; permissões efetivas são recalculadas nas próximas consultas, sem cache no desenho da spec.
