# Jornada — dupla aprovação bancária

A alteração bancária é representada neste contexto pela referência à evidência do destino. Os dados bancários pertencem a outro contexto e não são recebidos nem persistidos por `identity`.

```mermaid
sequenceDiagram
  actor Requester as Usuário proponente
  actor Approver as Usuário aprovador
  participant Caller as Chamador gRPC
  participant Identity as IdentityService
  participant Org as OrganizationRepository
  participant Change as BankAccountChange aggregate
  participant DB as PostgreSQL identity
  participant Outbox as Outbox

  Requester->>Caller: Inicia troca e apresenta ID da evidência de destino
  Caller->>Identity: RequestBankAccountChange(org, evidence_id)
  Identity->>Org: Confere existência e status Active
  Identity->>Change: Cria estado Requested com requester e horário
  Change->>DB: Persiste snapshot + BankAccountChangeRequested atomicamente
  Identity-->>Caller: Aceite ou rejection
  Note over Requester,Approver: São necessárias duas requisições e dois usuários distintos
  Approver->>Caller: Analisa proposta fora de identity
  Caller->>Identity: ApproveBankAccountChange(change_id, approver_id)
  Identity->>Change: Exige estado Requested e approver != requester
  Change->>DB: Persiste estado Approved + BankAccountChangeApproved atomicamente
  Identity-->>Caller: Aceite ou rejection
  Caller->>Identity: FindBankAccountChange(change_id) ou listagem por organização
  Identity->>DB: Lê estado e referência da evidência, tenant-scoped
  DB-->>Caller: Snapshot sem dados bancários
```

**Não implementado:** rejeitar/cancelar proposta, notificar aprovador ou efetivar os dados no destino. A aprovação não é automática e não altera o armazenamento bancário de outro contexto.
