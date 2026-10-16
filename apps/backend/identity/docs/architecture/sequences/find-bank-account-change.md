# FindBankAccountChange

Consulta de uma solicitação de troca bancária por ID. Rota OpenAPI destinada ao futuro BFF: `GET /identity/bank-account-changes/{bankAccountChangeId}`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as BankAccountChangeReader
  participant PG as PostgreSQL

  Client->>RPC: FindBankAccountChange(change_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindBankAccountChange(id)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: Load(id)
  Reader->>PG: SELECT por tenant_id + bank_account_change_id
  alt Ausente neste tenant
    PG-->>Reader: Não encontrado
    Reader-->>App: ErrNotFound
    App-->>RPC: Erro de consulta
  else Encontrada
    PG-->>Reader: Snapshot
    Reader-->>App: Estado, atores e ID de evidência
    App-->>RPC: BankAccountChangeSnapshot
  end
  RPC-->>Client: Resposta gRPC ou NOT_FOUND
```

O snapshot não contém os dados bancários do destino.
