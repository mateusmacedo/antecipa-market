# Modelo de dados — identity

O banco é próprio do contexto. As chaves primárias são compostas por `tenant_id` e o ID do agregado. Colunas tipadas existem para filtros e índices; os demais atributos ficam no `snapshot` JSONB.

```mermaid
erDiagram
  ORGANIZATIONS ||--o{ ESTABLISHMENTS : "organization_id (lógico)"
  ORGANIZATIONS ||--o{ REPRESENTATIONS : "organization_id (lógico)"
  PERSONS ||--o{ REPRESENTATIONS : "person_id (lógico)"
  ORGANIZATIONS ||--o{ MEMBERSHIPS : "organization_id (lógico)"
  USER_ACCOUNTS o|--o{ MEMBERSHIPS : "user_id após concessão (lógico)"
  ORGANIZATIONS ||--o{ BANK_ACCOUNT_CHANGES : "organization_id (lógico)"

  ORGANIZATIONS {
    text tenant_id PK
    text organization_id PK
    bigint version
    text tax_id_hash
    text jurisdiction
    jsonb snapshot
  }
  ESTABLISHMENTS {
    text tenant_id PK
    text establishment_id PK
    bigint version
    text organization_id
    text tax_id_hash
    jsonb snapshot
  }
  PERSONS {
    text tenant_id PK
    text person_id PK
    bigint version
    text document_hash
    jsonb snapshot
  }
  REPRESENTATIONS {
    text tenant_id PK
    text representation_id PK
    bigint version
    text organization_id
    text person_id
    jsonb snapshot
  }
  USER_ACCOUNTS {
    text tenant_id PK
    text user_account_id PK
    bigint version
    jsonb snapshot
  }
  MEMBERSHIPS {
    text tenant_id PK
    text membership_id PK
    bigint version
    text organization_id
    text user_id "nullable until grant"
    jsonb snapshot
  }
  BANK_ACCOUNT_CHANGES {
    text tenant_id PK
    text bank_account_change_id PK
    bigint version
    text organization_id
    jsonb snapshot
  }
```

> As relações do diagrama são referências lógicas por ID no estado do domínio. O DDL não declara foreign keys entre essas tabelas.

## Dados e isolamento

- **Tenant:** inicia a chave primária e participa das unicidades. A consulta de um ID pertencente a outro tenant é tratada como inexistente para o chamador; tentativas de acesso cruzado são registradas pelo kernel.
- **Unicidade:** organizações por `(tenant_id, tax_id_hash, jurisdiction)`; estabelecimentos por `(tenant_id, organization_id, tax_id_hash)`; pessoas por `(tenant_id, document_hash)`.
- **PII:** CNPJ/identificador fiscal de organização e estabelecimento permanece em claro no snapshot nesta versão; pessoa armazena hash HMAC-SHA256, máscara e versão da chave, nunca o documento. A troca bancária guarda somente o ID da evidência de destino, não os dados bancários.
- **Snapshots:** os campos completos podem ser consultados em [schema.sql](../../provider/schema.sql) e nos tipos de snapshot do [domínio](../../domain/).
- **Infra do kernel:** `inbox`, `outbox` e `quarantine` são tabelas geridas pelo kernel DMPF; não estão declaradas no schema local e por isso não são expandidas neste ERD. O fluxo da outbox está em [jornada de publicação](./journeys/outbox-relay.md).
