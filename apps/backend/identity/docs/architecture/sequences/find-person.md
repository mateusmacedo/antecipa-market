# FindPerson

Consulta de pessoa por ID. Rota OpenAPI destinada ao futuro BFF: `GET /identity/persons/{personId}`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as PersonReader
  participant PG as PostgreSQL

  Client->>RPC: FindPerson(person_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindPerson(id)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: Load(id)
  Reader->>PG: SELECT por tenant_id + person_id
  alt Ausente neste tenant
    PG-->>Reader: Não encontrado
    Reader-->>App: ErrNotFound
    App-->>RPC: Erro de consulta
  else Encontrada
    PG-->>Reader: Snapshot
    Reader-->>App: Nome, hash, máscara e versão da chave
    App-->>RPC: PersonSnapshot sem documento original
  end
  RPC-->>Client: Resposta gRPC ou NOT_FOUND
```

Read-only: sem UoW, metadata de idempotência ou evento. O documento original não é armazenado.
