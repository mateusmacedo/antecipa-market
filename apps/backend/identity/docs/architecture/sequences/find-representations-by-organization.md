# FindRepresentationsByOrganization

Consulta as representações de uma organização, opcionalmente filtradas pela vigência em um instante. Rota OpenAPI destinada ao futuro BFF: `GET /identity/organizations/{organizationId}/representations`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Reader as RepresentationsByOrganizationReader
  participant PG as PostgreSQL

  Client->>RPC: FindRepresentationsByOrganization(org_id, valid_at opcional)
  RPC->>RPC: Valida forma
  RPC->>App: FindRepresentationsByOrganization(query)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Reader: LoadByOrganization(org_id)
  Reader->>PG: SELECT tenant_id + organization_id
  PG-->>Reader: Snapshots da organização
  Reader-->>App: Lista de representações
  alt valid_at ausente (zero)
    App-->>RPC: Lista sem filtro temporal
  else valid_at informado
    App->>App: Mantém valid_from <= valid_at e valid_until = 0 ou valid_at < valid_until
    App-->>RPC: Lista vigente no instante
  end
  RPC-->>Client: Resposta gRPC
```

Read-only: a validade é refinada em memória sobre o resultado do reader; não altera estado ou gera evento.
