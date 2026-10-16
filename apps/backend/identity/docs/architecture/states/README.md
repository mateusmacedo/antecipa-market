# Estados dos agregados

Os diagramas mostram estados persistidos e transições que existem no domínio de `identity`. Um status enum sem comando de transição é anotado como tal; não são inventados cancelamentos, verificações, reativações ou expirações automáticas.

| Agregado | Diagrama | Ciclo de vida observado |
|---|---|---|
| Organization | [organization](./organization.md) | Criação ativa; outros valores do enum não têm transição neste app |
| Establishment | [establishment](./establishment.md) | Registro de uma vez, sem campo de status |
| Person | [person](./person.md) | Registro de uma vez, sem documento persistido ou campo de status |
| Representation | [representation](./representation.md) | `New` → `Declared`; verificação/revogação fora do contexto implementado |
| UserAccount | [user-account](./user-account.md) | `New` → `Active` → `Suspended` |
| Membership | [membership](./membership.md) | `New` → `Invited` → `Active` → `Revoked` |
| BankAccountChange | [bank-account-change](./bank-account-change.md) | `New` → `Requested` → `Approved` |

A expiração do convite bloqueia `GrantMembership`, mas não altera automaticamente o status. A vigência da representação é avaliada por `valid_from`/`valid_until` na consulta, sem transição temporal persistida.
