# RequestBankAccountChange

Comando que registra uma proposta de troca da conta bancária. Rota OpenAPI destinada ao futuro BFF: `POST /identity/organizations/{organizationId}/bank-account-changes`.

```mermaid
sequenceDiagram
  actor Requester as Usuário solicitante
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant OrgRepo as OrganizationRepository
  participant Repo as BankAccountChangeRepository
  participant Domain as BankAccountChange.Request
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Requester->>RPC: RequestBankAccountChange(org, evidence_id, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: RequestBankAccountChange(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>OrgRepo: Lê status da organização
    OrgRepo->>PG: SELECT tenant-scoped
    PG-->>OrgRepo: Status ou ausente
    UOW->>Repo: Carrega ChangeID (deve estar ausente)
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Ausente
    UOW->>Domain: Request(evidence_id, requester, status da organização)
    alt Evidência vazia ou organização não ativa
      Domain-->>UOW: Rejection, sem mudança/evento
    else Aceita
      Domain-->>UOW: Estado Requested + BankAccountChangeRequested
      UOW->>Repo: Save(snapshot com ID da evidência)
      UOW->>Outbox: Enqueue(BankAccountChangeRequested)
      Outbox->>PG: Persiste evento na transação
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Requester: Resposta gRPC
```

**Efeito/evento:** só o ID da evidência do destino é armazenado; dados bancários não entram neste contexto. A aprovação é um segundo comando e deve vir de outro usuário.
