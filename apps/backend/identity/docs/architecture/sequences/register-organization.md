# RegisterOrganization

Comando para criar uma organização no tenant da requisição. Rota OpenAPI destinada ao futuro BFF: `POST /identity/organizations`.

```mermaid
sequenceDiagram
  actor Client as Chamador gRPC
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant UOW as DMPF Execute/UoW
  participant Inbox as Inbox identity.commands
  participant Protector as HMAC identifier protector
  participant Reader as TaxIDReader
  participant Domain as Organization.Register
  participant Repo as OrganizationRepository
  participant Outbox as Outbox
  participant PG as PostgreSQL

  Client->>RPC: RegisterOrganization(request, idempotency-key)
  RPC->>RPC: Valida forma e metadata
  RPC->>App: RegisterOrganization(command)
  App->>Auth: Autoriza operation + tenant
  Auth-->>App: Permitido
  App->>UOW: Execute(command, fingerprint)
  UOW->>PG: Abre transação
  UOW->>Inbox: Verifica chave e digest do fingerprint
  alt Mesmo comando já concluído
    Inbox-->>UOW: Resultado previamente registrado
    UOW-->>RPC: Resultado replayed
  else Chave reutilizada/in-flight
    Inbox-->>UOW: FAILED_PRECONDITION ou ABORTED
    UOW-->>RPC: Erro de idempotência
  else Comando novo
    UOW->>Protector: Normaliza e calcula HMAC do tax_id
    Protector-->>UOW: hash + versão da chave
    UOW->>Reader: Busca por hash e jurisdição (pool de leitura)
    Reader->>PG: Consulta tenant-scoped
    PG-->>Reader: Existente ou ausente
    Reader-->>UOW: Resultado da unicidade consultiva
    UOW->>Repo: Carrega ID (deve estar ausente)
    Repo->>PG: SELECT tenant_id + organization_id
    PG-->>Repo: Ausente
    UOW->>Domain: Register(kinds, tax_id, jurisdição, nomes, pré-checagem)
    alt Invariante recusada
      Domain-->>UOW: Rejection, sem salvar estado ou evento
    else Aceita
      Domain-->>UOW: Snapshot ativo + OrganizationRegistered
      UOW->>Repo: Save(snapshot, expected version)
      Repo->>PG: Persiste snapshot e hash tipado
      UOW->>Outbox: Enqueue(OrganizationRegistered)
      Outbox->>PG: Grava evento na mesma transação
    end
    UOW->>Inbox: Registra outcome para replay
    UOW->>PG: Commit
    UOW-->>RPC: Aceite ou rejection de domínio
  end
  RPC-->>Client: Resposta gRPC
```

**Efeito/evento:** cria estado `Active`; publica `OrganizationRegistered` via outbox. O evento leva hash e máscara, não o identificador em claro. A consulta consultiva não substitui a unicidade do índice PostgreSQL em corridas.
