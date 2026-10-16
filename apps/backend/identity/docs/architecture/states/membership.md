# Estados — Membership

```mermaid
stateDiagram-v2
  [*] --> New
  New --> Invited : InviteUser aceita
  Invited --> Active : GrantMembership aceita
  Active --> Revoked : RevokeMembership aceita
  Revoked --> [*]
```

`GrantMembership` exige convite ainda não expirado, canal confirmado, nível de verificação suficiente para o papel e conta de usuário ativa. Convite expirado não vira estado `Expired`: a tentativa é recusada, sem transição automática. `Revoked` não tem reativação.
