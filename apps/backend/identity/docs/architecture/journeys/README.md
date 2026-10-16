# Jornadas ponta a ponta

As jornadas reúnem sequências já catalogadas. `identity` não executa sagas/process managers, não consome eventos e não envia convites: verificações/canais externos são explicitamente fronteiras do chamador.

| Jornada | Diagrama | Operações cobertas |
|---|---|---|
| Organização e seus registros | [Cadastro organizacional](./organization-onboarding.md) | [RegisterOrganization](../sequences/register-organization.md), [AddEstablishment](../sequences/add-establishment.md), [RegisterPerson](../sequences/register-person.md), [RegisterRepresentation](../sequences/register-representation.md) |
| Acesso de usuário e autorização efetiva | [Vínculo e permissões](./membership-access.md) | [ProvisionUserAccount](../sequences/provision-user-account.md), [InviteUser](../sequences/invite-user.md), [GrantMembership](../sequences/grant-membership.md), [FindEffectivePermissions](../sequences/find-effective-permissions.md), [RevokeMembership](../sequences/revoke-membership.md), [SuspendUser](../sequences/suspend-user.md) |
| Dupla aprovação para troca bancária | [Troca de conta bancária](./bank-account-change.md) | [RequestBankAccountChange](../sequences/request-bank-account-change.md), [ApproveBankAccountChange](../sequences/approve-bank-account-change.md), [FindBankAccountChange](../sequences/find-bank-account-change.md), [FindBankAccountChangesByOrganization](../sequences/find-bank-account-changes-by-organization.md) |
| Integração assíncrona | [Outbox e relay](./outbox-relay.md) | [RegisterOrganization](../sequences/register-organization.md), [AddEstablishment](../sequences/add-establishment.md), [SuspendUser](../sequences/suspend-user.md), [GrantMembership](../sequences/grant-membership.md), [RevokeMembership](../sequences/revoke-membership.md), [RequestBankAccountChange](../sequences/request-bank-account-change.md), [ApproveBankAccountChange](../sequences/approve-bank-account-change.md) e papel `relay` |

Estas jornadas não substituem as [22 sequências individuais](../sequences/README.md). As operações de consulta restantes também têm suas interações próprias, mesmo quando não compõem um fluxo de escrita.
