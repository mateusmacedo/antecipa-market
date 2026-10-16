# Jornada — organização e cadastros relacionados

O tenant é provisionado pelo backoffice no provedor de identidade; as operações seguintes são chamadas explícitas, não uma saga disparada automaticamente pelo registro da organização.

```mermaid
sequenceDiagram
  actor Backoffice as Backoffice
  actor OrgAdmin as Operador autorizado
  participant IdP as Keycloak (fora do app)
  participant Identity as IdentityService gRPC
  participant Org as Organization aggregate
  participant Establishment as Establishment aggregate
  participant Person as Person aggregate
  participant Representation as Representation aggregate
  participant DB as PostgreSQL identity
  participant Relay as identity (role relay)
  participant Events as Kafka identity.events

  Backoffice->>IdP: Provisiona organização e atributo tenant_id
  OrgAdmin->>Identity: RegisterOrganization
  Identity->>Org: Protege tax_id, valida, registra Active
  Org->>DB: Persiste snapshot + hash
  Org->>DB: Enfileira OrganizationRegistered na mesma transação
  Identity-->>OrgAdmin: Aceite/rejection
  opt Estabelecimento operacional associado
    OrgAdmin->>Identity: AddEstablishment
    Identity->>Org: Confere que a organização está Active
    Identity->>Establishment: Valida e cria estabelecimento
    Establishment->>DB: Persiste snapshot + evento EstablishmentAdded
    Identity-->>OrgAdmin: Aceite/rejection
  end
  opt Pessoa que representará a organização
    OrgAdmin->>Identity: RegisterPerson
    Identity->>Person: HMAC de documento, persiste hash/máscara, não documento
    Person->>DB: Persiste snapshot sem evento
    Identity-->>OrgAdmin: Aceite/rejection
    OrgAdmin->>Identity: RegisterRepresentation(person_id, organization_id, poderes)
    Identity->>Org: Confere organização ativa
    Identity->>Person: Confere pessoa existente no tenant
    Identity->>Representation: Valida evidência, alçada e vigência, declara
    Representation->>DB: Persiste estado Declared sem evento
    Identity-->>OrgAdmin: Aceite/rejection
  end
  Relay->>DB: Claim de eventos já confirmados na outbox
  DB-->>Relay: Eventos de integração elegíveis
  Relay->>Events: Publica CloudEvents depois do commit
```

## Limites da jornada

- `RegisterOrganization` não cria automaticamente estabelecimentos, pessoas, representações, usuários ou vínculos.
- `AddEstablishment` e `RegisterRepresentation` fazem precondições explícitas sobre organização ativa; representação também exige pessoa existente.
- `RegisterPerson` e `RegisterRepresentation` não publicam eventos de integração nesta versão.
- O documento de pessoa nunca é persistido. O hash do identificador fiscal e do documento é tenant-scoped.
- O cadastro/gestão do Keycloak e o BFF não são componentes implementados pelo app.
