# Sequências dos 22 casos de uso

Cada arquivo é uma sequência individual do contrato `IdentityService`. A sequência segue o código de handler e application; não presume a futura borda REST nem integrações externas não chamadas por `identity`.

## Comandos

1. [RegisterOrganization](./register-organization.md)
2. [AddEstablishment](./add-establishment.md)
3. [RegisterPerson](./register-person.md)
4. [RegisterRepresentation](./register-representation.md)
5. [ProvisionUserAccount](./provision-user-account.md)
6. [SuspendUser](./suspend-user.md)
7. [InviteUser](./invite-user.md)
8. [GrantMembership](./grant-membership.md)
9. [RevokeMembership](./revoke-membership.md)
10. [RequestBankAccountChange](./request-bank-account-change.md)
11. [ApproveBankAccountChange](./approve-bank-account-change.md)

## Consultas

12. [FindOrganization](./find-organization.md)
13. [FindOrganizationByTaxId](./find-organization-by-tax-id.md)
14. [FindEstablishmentsByOrganization](./find-establishments-by-organization.md)
15. [FindPerson](./find-person.md)
16. [FindRepresentation](./find-representation.md)
17. [FindRepresentationsByOrganization](./find-representations-by-organization.md)
18. [FindMembershipsByUser](./find-memberships-by-user.md)
19. [FindMembershipsByOrganization](./find-memberships-by-organization.md)
20. [FindEffectivePermissions](./find-effective-permissions.md)
21. [FindBankAccountChange](./find-bank-account-change.md)
22. [FindBankAccountChangesByOrganization](./find-bank-account-changes-by-organization.md)

### Convenções de leitura

- Todas as rotas HTTP são as publicadas no OpenAPI para o BFF; a interface executada pelo app é gRPC.
- Os 11 comandos exigem `idempotency-key`; fingerprint igual reproduz o resultado anterior e fingerprint diferente não executa novamente o domínio. O detalhe normativo está no [README do app](../../../README.md#idempotência-dos-comandos).
- Consultas passam por autorização e reader tenant-scoped, sem UoW. Os comandos autorizam antes de abrir a UoW; persistência e outbox são transacionais.
- Recusa de domínio é resultado de negócio na resposta, não exceção; falhas de infraestrutura/concorrência tornam-se status gRPC.
