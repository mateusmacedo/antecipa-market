# FindOrganization

Consulta de organização por ID. Rota OpenAPI destinada ao futuro BFF: `GET /identity/organizations/{organizationId}`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as OrganizationReader
  participant PG as PostgreSQL

  Client->>RPC: FindOrganization(organization_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindOrganization(id)
  App->>Auth: Autoriza FindOrganization + tenant
  Auth-->>App: Permitido
  App->>Reader: Load(id)
  Reader->>PG: SELECT por tenant_id + organization_id
  alt ID ausente ou pertence a outro tenant
    PG-->>Reader: Não encontrado
    Reader-->>App: ErrNotFound
    App-->>RPC: Erro de consulta
  else Encontrada
    PG-->>Reader: Snapshot e versão
    Reader-->>App: OrganizationSnapshot
    App-->>RPC: Snapshot
  end
  RPC-->>Client: Resposta gRPC ou NOT_FOUND
```

Consulta read-only: sem `idempotency-key`, UoW ou evento. A resposta é tenant-scoped.
