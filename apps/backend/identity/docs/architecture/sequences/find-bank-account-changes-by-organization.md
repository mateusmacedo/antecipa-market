# FindBankAccountChangesByOrganization

Consulta as solicitações de troca bancária de uma organização. Rota OpenAPI destinada ao futuro BFF: `GET /identity/organizations/{organizationId}/bank-account-changes`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as BankAccountChangesByOrganizationReader
  participant PG as PostgreSQL

  Client->>RPC: FindBankAccountChangesByOrganization(organization_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindBankAccountChangesByOrganization(id)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: LoadByOrganization(id)
  Reader->>PG: SELECT tenant_id + organization_id
  PG-->>Reader: Snapshots correspondentes
  Reader-->>App: Lista tenant-scoped
  App-->>RPC: Lista de BankAccountChangeSnapshot
  RPC-->>Client: Resposta gRPC
```

Read-only: sem UoW, metadata de idempotência ou evento.
