# Mapa diagramático — identity

Este mapa percorre o bounded context `identity-organization` da fronteira gRPC ao domínio e à persistência. Os diagramas descrevem o comportamento implementado no app; capacidades que existem apenas em spec/design são identificadas como futuras.

## Como navegar

1. [Contexto do sistema](./c4-context.md) — atores, consumidores e integrações.
2. [Containers](./c4-containers.md) — processos `api` e `relay`, dados e observabilidade.
3. [Componentes](./c4-components.md) — caminho da RPC ao domínio e aos providers.
4. [Modelo de dados](./data-model.md) — agregados, tabelas e limites tenant-scoped.
5. [Jornadas](./journeys/README.md) — fluxos que atravessam operações.
6. [Estados](./states/README.md) — ciclos de vida efetivamente modelados.
7. [Sequências dos casos de uso](./sequences/README.md) — uma interação por método gRPC.

## Legenda de escopo

- **Implementado**: comportamento presente no código, contrato ou configuração atual.
- **Futuro/fora do escopo**: aparece em spec/design, mas não está implementado por `identity`; não representa uma dependência atual de runtime.
- **Saga/process manager**: o app não implementa saga nem consome eventos. Os comandos são síncronos; eventos de integração são publicados assincronamente por outbox/relay.
- **Referência lógica**: associação por IDs, sem implicar chave estrangeira no PostgreSQL.

## Fronteira e contrato

`identity` expõe somente `company.identity.service.v1.IdentityService` por gRPC interno. A borda REST/BFF e as rotas OpenAPI publicadas para ela são futuras/não presentes neste workspace. O app não recebe nem processa pedidos REST. Publica no canal Kafka `identity.events` e não consome canal Kafka.

## Catálogo completo de operações

| Agregado | Operação | Tipo | Rota OpenAPI publicada para BFF | Permissão |
|---|---|---|---|---|
| Organization | [RegisterOrganization](./sequences/register-organization.md) | Comando | `POST /identity/organizations` | `identity:register-organization` |
| Organization | [FindOrganization](./sequences/find-organization.md) | Consulta | `GET /identity/organizations/{organizationId}` | `identity:find-organization` |
| Organization | [FindOrganizationByTaxId](./sequences/find-organization-by-tax-id.md) | Consulta | `POST /identity/organization-lookups` | `identity:find-organization-by-tax-id` |
| Establishment | [AddEstablishment](./sequences/add-establishment.md) | Comando | `POST /identity/organizations/{organizationId}/establishments` | `identity:add-establishment` |
| Establishment | [FindEstablishmentsByOrganization](./sequences/find-establishments-by-organization.md) | Consulta | `GET /identity/organizations/{organizationId}/establishments` | `identity:find-establishments-by-organization` |
| Person | [RegisterPerson](./sequences/register-person.md) | Comando | `POST /identity/persons` | `identity:register-person` |
| Person | [FindPerson](./sequences/find-person.md) | Consulta | `GET /identity/persons/{personId}` | `identity:find-person` |
| Representation | [RegisterRepresentation](./sequences/register-representation.md) | Comando | `POST /identity/organizations/{organizationId}/representations` | `identity:register-representation` |
| Representation | [FindRepresentation](./sequences/find-representation.md) | Consulta | `GET /identity/representations/{representationId}` | `identity:find-representation` |
| Representation | [FindRepresentationsByOrganization](./sequences/find-representations-by-organization.md) | Consulta | `GET /identity/organizations/{organizationId}/representations` | `identity:find-representations-by-organization` |
| UserAccount | [ProvisionUserAccount](./sequences/provision-user-account.md) | Comando | `POST /identity/users` | `identity:provision-user-account` |
| UserAccount | [SuspendUser](./sequences/suspend-user.md) | Comando | `POST /identity/users/{userId}/suspend` | `identity:suspend-user` |
| Membership | [InviteUser](./sequences/invite-user.md) | Comando | `POST /identity/organizations/{organizationId}/memberships` | `identity:invite-user` |
| Membership | [GrantMembership](./sequences/grant-membership.md) | Comando | `POST /identity/memberships/{membershipId}/grant` | `identity:grant-membership` |
| Membership | [RevokeMembership](./sequences/revoke-membership.md) | Comando | `POST /identity/memberships/{membershipId}/revoke` | `identity:revoke-membership` |
| Membership | [FindMembershipsByUser](./sequences/find-memberships-by-user.md) | Consulta | `GET /identity/users/{userId}/memberships` | `identity:find-memberships-by-user` |
| Membership | [FindMembershipsByOrganization](./sequences/find-memberships-by-organization.md) | Consulta | `GET /identity/organizations/{organizationId}/memberships` | `identity:find-memberships-by-organization` |
| Membership | [FindEffectivePermissions](./sequences/find-effective-permissions.md) | Consulta | `GET /identity/users/{userId}/permissions` | `identity:find-effective-permissions` |
| BankAccountChange | [RequestBankAccountChange](./sequences/request-bank-account-change.md) | Comando | `POST /identity/organizations/{organizationId}/bank-account-changes` | `identity:request-bank-account-change` |
| BankAccountChange | [ApproveBankAccountChange](./sequences/approve-bank-account-change.md) | Comando | `POST /identity/bank-account-changes/{bankAccountChangeId}/approve` | `identity:approve-bank-account-change` |
| BankAccountChange | [FindBankAccountChange](./sequences/find-bank-account-change.md) | Consulta | `GET /identity/bank-account-changes/{bankAccountChangeId}` | `identity:find-bank-account-change` |
| BankAccountChange | [FindBankAccountChangesByOrganization](./sequences/find-bank-account-changes-by-organization.md) | Consulta | `GET /identity/organizations/{organizationId}/bank-account-changes` | `identity:find-bank-account-changes-by-organization` |

As rotas REST acima vêm do OpenAPI e são destinadas à futura borda BFF; o endpoint operacional atual é gRPC. Cada comando exige `idempotency-key`; consultas não leem essa metadata. O BFF resolve permissões efetivas consultando `FindEffectivePermissions`; o serviço gRPC autoriza pelo tenant do contexto de execução.

## Fontes de verdade

- [README do app](../../README.md) — configuração, operações, idempotência, eventos, deploy e testes.
- [Contrato Protobuf](../../contract/proto/company/identity/service/v1/identity_service.proto) e [OpenAPI](../../contract/openapi/v1/openapi.yaml).
- [Application](../../application/) e [domain](../../domain/) — execução e invariantes.
- [Schema PostgreSQL](../../provider/schema.sql) e [wiring](../../app/wiring.go).
- [Spec do bounded context](../../../../../docs/specs/SPEC-7KW0HHF6-identity-organization.md) — decisões e escopo; não substitui o comportamento atual do código.
