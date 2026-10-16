# FindEstablishmentsByOrganization

Consulta a lista de estabelecimentos de uma organização. Rota OpenAPI destinada ao futuro BFF: `GET /identity/organizations/{organizationId}/establishments`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as EstablishmentsByOrganizationReader
  participant PG as PostgreSQL

  Client->>RPC: FindEstablishmentsByOrganization(organization_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindEstablishmentsByOrganization(id)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: LoadByOrganization(id)
  Reader->>PG: SELECT tenant_id + organization_id
  PG-->>Reader: Snapshots correspondentes
  Reader-->>App: Lista tenant-scoped (vazia se não houver resultados)
  App-->>RPC: Lista de EstablishmentSnapshot
  RPC-->>Client: Resposta gRPC
```

Read-only: sem UoW, metadata de idempotência ou evento.
