# FindMembershipsByUser

Consulta os vínculos de um usuário no tenant corrente. Rota OpenAPI destinada ao futuro BFF: `GET /identity/users/{userId}/memberships`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as MembershipsByUserReader
  participant PG as PostgreSQL

  Client->>RPC: FindMembershipsByUser(user_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindMembershipsByUser(user)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: LoadByUser(user_id)
  Reader->>PG: SELECT tenant_id + user_id
  PG-->>Reader: Membership snapshots
  Reader-->>App: Lista tenant-scoped
  App-->>RPC: Lista de MembershipSnapshot
  RPC-->>Client: Resposta gRPC
```

Read-only: sem UoW, metadata de idempotência ou evento.
