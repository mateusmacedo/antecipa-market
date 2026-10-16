# Jornada — provisionamento, vínculo e permissões

A conta autenticável, o convite para uma organização e o vínculo ativo são operações distintas. `identity` não envia e-mail, confirma canal nem consulta serviço de verificação; as confirmações chegam como dados de `GrantMembership`.

```mermaid
sequenceDiagram
  actor Admin as Administrador da organização
  actor User as Usuário convidado
  participant Caller as Chamador gRPC / futuro BFF
  participant Identity as IdentityService
  participant Account as UserAccount
  participant Membership as Membership
  participant DB as PostgreSQL identity

  Caller->>Identity: ProvisionUserAccount(subject, person_id)
  Identity->>Account: Se ausente, ativa e associa person_id
  Account->>DB: Persiste snapshot, sem evento
  Identity-->>Caller: Resultado idempotente
  Admin->>Identity: InviteUser(org, email, role, expires_at)
  Identity->>Membership: Confere organização ativa e compatibilidade do papel
  Membership->>DB: Persiste estado Invited, sem envio de convite
  Identity-->>Admin: membership_id
  User->>Caller: Confirma posse do canal e conclui verificações externas
  Caller->>Identity: GrantMembership(channel_confirmed, verification_level, user, person)
  Identity->>Account: Confere conta ativa
  Identity->>Membership: Confere convite vigente e nível mínimo do papel
  Membership->>DB: Persiste estado Active + MembershipGranted na outbox
  Identity-->>Caller: Aceite/rejection
  Caller->>Identity: FindEffectivePermissions(user_id), na requisição seguinte
  Identity->>Membership: Lê vínculos do tenant
  Identity->>Account: Confere conta por último
  alt Conta ativa
    Identity-->>Caller: União ordenada dos papéis dos vínculos Active
  else Conta ausente/suspensa
    Identity-->>Caller: Lista de permissões vazia
  end
  opt Revogação do vínculo
    Admin->>Identity: RevokeMembership(membership_id)
    Identity->>Membership: Active -> Revoked, publica MembershipRevoked
  end
  opt Suspensão da conta
    Admin->>Identity: SuspendUser(user_id, reason)
    Identity->>Account: Active -> Suspended, publica UserSuspended
  end
```

## Etapas e fronteiras

1. `ProvisionUserAccount` inicializa por `subject` do provedor federado; conta existente é aceita sem novo efeito de domínio.
2. `InviteUser` persiste o estado `Invited`; não dispara e-mail nem espera callback dentro da mesma chamada.
3. Confirmação de canal e verificação documental/liveness são passos externos. O grant valida apenas os valores recebidos e a conta ativa.
4. `FindEffectivePermissions` agrega papéis somente de memberships `Active` e retorna vazio se a conta não existe ou está suspensa.
5. Revogação e suspensão são comandos independentes. As permissões são consultadas novamente; não há saga ou cache de permissão em `identity`.

O BFF que faria a consulta a cada requisição é futuro/não está presente neste workspace. Ver [as sequências individuais](../sequences/README.md).
