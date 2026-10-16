# FindRepresentation

Consulta de representação por ID. Rota OpenAPI destinada ao futuro BFF: `GET /identity/representations/{representationId}`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as RepresentationReader
  participant PG as PostgreSQL

  Client->>RPC: FindRepresentation(representation_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindRepresentation(id)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: Load(id)
  Reader->>PG: SELECT por tenant_id + representation_id
  alt Ausente neste tenant
    PG-->>Reader: Não encontrado
    Reader-->>App: ErrNotFound
    App-->>RPC: Erro de consulta
  else Encontrada
    PG-->>Reader: Snapshot, versão e campos de poder/vigência
    Reader-->>App: RepresentationSnapshot
    App-->>RPC: Snapshot
  end
  RPC-->>Client: Resposta gRPC ou NOT_FOUND
```

Read-only: sem UoW, metadata de idempotência ou evento.
