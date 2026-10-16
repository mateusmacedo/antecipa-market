# FindEffectivePermissions

Resolve permissões atuais pela união ordenada dos papéis de vínculos ativos, somente se a conta estiver ativa. Rota OpenAPI destinada ao futuro BFF: `GET /identity/users/{userId}/permissions`.

```mermaid
sequenceDiagram
  actor Caller as Chamador (futuro BFF)
  participant RPC as IdentityService
  participant App as application.Service
  participant Auth as Autorização tenant
  participant MembershipReader as MembershipsByUserReader
  participant AccountReader as UserAccountReader
  participant PG as PostgreSQL

  Caller->>RPC: FindEffectivePermissions(user_id)
  RPC->>RPC: Valida forma do ID
  RPC->>App: FindEffectivePermissions(user)
  App->>Auth: Autoriza consulta + tenant
  Auth-->>App: Permitido
  App->>MembershipReader: LoadByUser(user_id)
  MembershipReader->>PG: SELECT vínculos por tenant + user
  PG-->>MembershipReader: Membership snapshots
  MembershipReader-->>App: Vínculos (ou ausência)
  App->>AccountReader: Load(user_id) — verificação final fail-closed
  AccountReader->>PG: SELECT tenant_id + user_account_id
  PG-->>AccountReader: Status da conta ou ausência
  alt Conta ausente ou suspensa
    App->>App: Descarta permissões
    App-->>RPC: Lista vazia
  else Conta ativa
    App->>App: Seleciona memberships Active e faz união ordenada/deduplicada dos papéis
    App-->>RPC: Permissões efetivas
  end
  RPC-->>Caller: Lista gRPC
```

As leituras não compartilham snapshot transacional. Por isso a conta é verificada por último: uma suspensão concorrente esvazia a resposta. A leitura de permissões não é cacheada no desenho desta versão; o BFF é futuro.
