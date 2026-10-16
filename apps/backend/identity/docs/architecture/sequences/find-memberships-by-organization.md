# FindMembershipsByOrganization

Consulta os vínculos de uma organização. Rota OpenAPI destinada ao futuro BFF: `GET /identity/organizations/{organizationId}/memberships`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as MembershipsByOrganizationReader
  participant PG as PostgreSQL

  Client->>RPC: FindMembershipsByOrganization(organization_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindMembershipsByOrganization(id)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: LoadByOrganization(id)
  Reader->>PG: SELECT tenant_id + organization_id
  PG-->>Reader: Membership snapshots
  Reader-->>App: Lista tenant-scoped
  App-->>RPC: Lista de MembershipSnapshot
  RPC-->>Client: Resposta gRPC
```

Read-only: sem UoW, metadata de idempotência ou evento.
