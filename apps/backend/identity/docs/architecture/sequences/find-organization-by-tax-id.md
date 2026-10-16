# FindOrganizationByTaxId

Consulta por identificador fiscal protegido, com filtro final de jurisdição. Rota OpenAPI destinada ao futuro BFF: `POST /identity/organization-lookups`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant Protector as HMAC identifier protector
  participant Reader as OrganizationsByTaxIDReader
  participant PG as PostgreSQL

  Client->>RPC: FindOrganizationByTaxId(type, tax_id, jurisdiction)
  RPC->>RPC: Valida forma
  RPC->>App: FindOrganizationByTaxID(query)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>Protector: Normaliza e calcula HMAC(type, tenant, valor)
  Protector-->>App: tax_id_hash
  App->>Reader: LoadByTaxIDHash(hash)
  Reader->>PG: SELECT tenant-scoped por hash
  alt Hash sem resultado neste tenant
    PG-->>Reader: Lista vazia
    Reader-->>App: Sem organização
    App-->>RPC: NOT_FOUND
  else Acesso cruzado detectado
    PG-->>Reader: CrossTenantAccess
    Reader-->>App: Sinal de segurança
    App->>App: Registra ocorrência, mascara para ErrNotFound
    App-->>RPC: NOT_FOUND
  else Hash encontrado
    PG-->>Reader: Organizações candidatas
    Reader-->>App: Snapshots
    App->>App: Filtra jurisdição em memória
    alt Jurisdição não encontrada
      App-->>RPC: NOT_FOUND
    else Correspondência
      App-->>RPC: OrganizationSnapshot
    end
  end
  RPC-->>Client: Resposta gRPC ou NOT_FOUND
```

**Privacidade:** o valor fiscal original não é usado como chave da consulta; o hash inclui tenant e tipo do identificador. A resposta não revela que um identificador pertence a outro tenant.
