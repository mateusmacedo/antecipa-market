# ApproveBankAccountChange

Comando que aprova uma proposta ainda pendente por usuário diferente do solicitante. Rota OpenAPI destinada ao futuro BFF: `POST /identity/bank-account-changes/{bankAccountChangeId}/approve`.

```mermaid
sequenceDiagram
  actor Approver as Usuário aprovador
  participant RPC as IdentityService
  participant App as application.Service
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Repo as BankAccountChangeRepository
  participant Domain as BankAccountChange.Approve
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Approver->>RPC: ApproveBankAccountChange(change_id, approver_id, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: ApproveBankAccountChange(command)
  App->>App: Autoriza operation + tenant
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave/digest
  alt Replay ou falha de chave
    Inbox-->>UOW: Resultado salvo ou erro de idempotência
  else Comando novo
    UOW->>Repo: Carrega proposta existente
    Repo->>PG: SELECT tenant-scoped
    PG-->>Repo: Snapshot ou não encontrado
    UOW->>Domain: Approve(approver_id)
    alt Proposta não Requested
      Domain-->>UOW: Rejection
    else Aprovador é o solicitante
      Domain-->>UOW: Rejection de dupla aprovação
    else Aceita
      Domain-->>UOW: Estado Approved + BankAccountChangeApproved
      UOW->>Repo: Save(snapshot)
      UOW->>Outbox: Enqueue(BankAccountChangeApproved)
      Outbox->>PG: Persiste evento na transação
    end
    UOW->>Inbox: Registra outcome
    UOW->>PG: Commit
  end
  UOW-->>RPC: Aceite ou rejection
  RPC-->>Approver: Resposta gRPC
```

**Efeito/evento:** aprovação não move os dados bancários; consolida a decisão e publica `BankAccountChangeApproved`. Não há operação de rejeição/cancelamento implementada neste contexto.
