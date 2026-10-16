# GrantMembership

Comando que transforma um convite válido em vínculo ativo. Rota OpenAPI destinada ao futuro BFF: `POST /identity/memberships/{membershipId}/grant`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant UserRepo as UserAccountRepository
  participant Repo as MembershipRepository
  participant Domain as Membership.Grant
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Client->>RPC: GrantMembership(user, person, confirmação, verificação, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: GrantMembership(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>UserRepo: Lê status da conta do usuário
    UserRepo->>PG: SELECT tenant-scoped
    PG-->>UserRepo: Ativa, inativa/ausente ou erro
    UOW->>Repo: Carrega convite existente
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Membership snapshot
    UOW->>Domain: Grant(user, person, channel_confirmed, verification_level)
    alt Não convidado/expirado, canal não confirmado, verificação insuficiente ou conta inativa
      Domain-->>UOW: Rejection, sem mudança/evento
    else Aceita
      Domain-->>UOW: Membership Active + MembershipGranted
      UOW->>Repo: Save(snapshot)
      UOW->>Outbox: Enqueue(MembershipGranted)
      Outbox->>PG: Persiste evento na transação
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** canal confirmado e nível de verificação são dados do comando; o app não confirma e-mail nem consulta verificador documental. A concessão publica `MembershipGranted`.
