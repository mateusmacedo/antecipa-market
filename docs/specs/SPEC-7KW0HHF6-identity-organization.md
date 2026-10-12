---
id: SPEC-7KW0HHF6
slug: identity-organization
title: Bounded context — identity-organization
stage: backlog
priority: P0
depends_on: [SPEC-GXGASP24]
ticket_url: https://linear.app/mmda/issue/DEVS-77/spec-7kw0hhf6-bounded-context-identity-organization
subtask_urls: []
created: 2026-10-12
---

# SPEC-7KW0HHF6: Bounded context — identity-organization

## Identidade

- **bounded_context**: `identity-organization`
- **name**: `identity`
- **Linguagem ubíqua**:
  - *Organização* (`Organization`) - pessoa jurídica titular de direitos e parte em contratos; cliente, financiador, parceiro ou plataforma.
  - *Estabelecimento* (`Establishment`) - unidade operacional com identificador fiscal próprio; o *estabelecimento recebedor* é o credenciado em arranjo de pagamento e titular dos recebíveis.
  - *Pessoa* (`Person`) - pessoa natural identificada de forma protegida (hash e máscara; documento em claro só neste agregado).
  - *Representação* (`Representation`) - vínculo entre pessoa e organização, com tipo de poder, fonte, alçada e vigência; nunca é alterada, só revogada e substituída.
  - *Alçada* (`scope`) - limite de valor ou de tipo de ato imposto pelo documento de poderes.
  - *Conta de usuário* (`UserAccount`) - identidade autenticável, federada a um provedor de identidade externo.
  - *Vínculo* (`Membership`) - acesso de um usuário a uma organização com um papel.
  - *Papel* e *permissão* - autorização funcional; papéis de cliente e de plataforma permanecem separados.
  - *Dupla aprovação* - proposição e aprovação do ato sensível por usuários distintos.
  - *Permissão efetiva* - permissões dos papéis ativos, limitadas pela representação vigente, em um instante.

## Origem e bloqueios externos

- **Fontes**: `docs/prd/01-prd-identity-organization.md`; `docs/design/01-design-identity-organization.md` §2 a §6; `docs/design/90-modelo-de-dados.md` §3.1; `docs/design/92-schemas-eventos.md` §4; matriz de papéis em `docs/design/95-personas-jornadas.md`; `docs/prd/14-catalogo-contratos-eventos.md`.
- **Situação**: `desbloqueado` - nenhuma lacuna #1 a #12 trava o núcleo; só a fatia de autenticação e verificação documental depende de C5.
- **Fase de entrega**: 1 (inferida).
- **Dependências**: `depends_on` declara só a fundação `SPEC-GXGASP24`. Upstream duro: nenhum. Upstream brando: Compliance Ledger (histórico de permissões, `docs/design/01` §4). Este contexto é a raiz da cadeia dura (identity, partner, gateway, consent, portfolio, eligibility, marketplace, contracting, effects, funding, settlement).
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
| --- | --- | --- |
| C5 (`01:I8`, `01:I9`) | provedor de identidade (OIDC) e de verificação documental não escolhidos | `UserAccount`, `Membership` (nível de verificação por papel), reautenticação em ato crítico |
| A10 do resumo (`90:L3`) | limite de alçada em procuração e tratamento de assinatura acima dos poderes | campo `scope` de `Representation` |
| `01:I10` | regime de aprovação única assistida exige validação jurídica | configuração da dupla aprovação por organização (fora desta versão) |
| `01:I11` | tratamento de grupo econômico | relação matriz-filial entre organizações distintas |
| `90:L8` | rotação do `salt` do hash de identificador fiscal | `taxIdHash` |
| A1 (#1), transversal | autorização regulatória da plataforma | existencial; não bloqueia a construção |
| #12, transversal | volumetria | dimensionamento, não a construção |

- **Decisões de produto abertas**: protocolo de autenticação OIDC com provedor externo [ABERTA: docs/design/01-design-identity-organization.md:§2.1]; regime de aprovação única assistida [ABERTA: docs/design/01-design-identity-organization.md:§5]; sacado no escopo (`docs/design/95-personas-jornadas.md`, ator A10 daquele documento, distinto de A10 do resumo executivo).
- **Condição para sair do backlog**: `SPEC-GXGASP24` concluída. O núcleo (`Organization`, `Establishment`, `Person`, `Representation`, `BankAccountChange`) não depende de bloqueio externo; `UserAccount` e `Membership` exigem a decisão de C5 antes da sua fatia.

## Agregados

Nota (†): pré-condição que depende de outro agregado ou contexto é verificada pelo caso de uso, por leitura ou consulta gRPC antes de decidir [ABERTA: mecanismo de leitura de outro agregado na UPR - o golden bookings não o exercita; confirmar em FND-04 §3.2 na fundação]. Nota (††): unicidade é garantida por índice único e traduzida em rejeição [ABERTA: tradução de violação de unicidade em Rejection - sem exemplo no golden bookings].

### Organization

- **Identidade**: `generated`
- **Campos**:
  - `kinds: list<client | funder | partner | platform>` - coleção indispensável: a mesma pessoa jurídica acumula papéis (`90` §3.1)
  - `jurisdiction: string` (ISO 3166-1 alfa-2)
  - `taxIdType: cnpj | cpf`
  - `taxId: string` (PII, em claro só aqui)
  - `taxIdHash: string` (hash com `salt` secreto por ambiente)
  - `legalName: string`
  - `tradeName: string` (opcional)
  - `status: active | suspended | offboarded`
  - `createdAt: instant`
- **Estados**: `active`, `suspended`, `offboarded`
- **Transições permitidas**:
  - (novo) → `active` - por `RegisterOrganization`
  - `active` → `suspended` e → `offboarded` - sem comando definido [ABERTA: docs/prd/01-prd-identity-organization.md:Comandos e eventos - o PRD não lista comando de suspensão ou descontinuação, embora `90` §3.1 declare os estados]
- **Invariantes de estado**:
  - `(taxIdHash, jurisdiction)` é único - violação rejeita com `identity-organization/organization/duplicate-tax-id`.
  - O identificador fiscal nunca é chave técnica; a chave é gerada.
  - `kinds`, `jurisdiction`, `taxId` e `legalName` nunca são vazios (rejeições em `RegisterOrganization`).

### Establishment

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `taxIdType: cnpj | cpf`, `taxId: string` (PII), `taxIdHash: string`
  - `tradeName: string` (opcional)
  - `receiver: bool` - estabelecimento recebedor credenciado em arranjo [ABERTA: docs/design/01-design-identity-organization.md:§6 - como se comprova o credenciamento]
  - `externalReferences: list<ExternalReference>` - referências em sistemas externos (`90` §2.1), cardinalidade N
  - `createdAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - `(organizationId, taxIdHash)` é único - rejeita com `identity-organization/establishment/duplicate-tax-id` (os recebíveis pertencem ao estabelecimento identificado pelo seu identificador fiscal, `01` §6 regra 2).

### Person

- **Identidade**: `generated`
- **Campos**:
  - `fullName: string` (PII)
  - `document: string` (`PII_SENSITIVE`: nunca em log nem em evento; envelope adicional e chave segregada [ABERTA: sem provider verificado no kernel para criptografia em envelope - docs/design/90-modelo-de-dados.md:§1.4])
  - `documentHash: string`, `documentMasked: string`
  - `createdAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**: `documentHash` é único - rejeita com `identity-organization/person/duplicate-document`.

### Representation

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`, `personId: id`
  - `powerType: legal_representative | attorney_in_fact | operator`
  - `sourceType: articles_of_association | power_of_attorney | board_resolution | internal_delegation`
  - `sourceEvidenceId: id` (aponta para evidência; bytes fora do agregado [ABERTA: provider de object storage])
  - `scopeCurrency: string`, `scopeMaxAmountMinor: int`, `scopeActTypes: list<string>` - alçada, todos opcionais [ABERTA: docs/design/90-modelo-de-dados.md:§5 L3 - formato do limite e tratamento acima dos poderes]
  - `validFrom: instant`, `validUntil: instant` (opcional; ausente = indeterminado)
  - `verificationStatus: declared | verified | rejected`
  - `proposedByUserId: id`, `approvedByUserId: id` (opcional)
  - `revokedAt: instant` (opcional), `revocationReason: string` (opcional)
  - `revocationRequestedByUserId: id`, `revocationApprovedByUserId: id` (opcionais)
- **Estados**: `declared`, `verified`, `rejected`; revogação marcada por `revokedAt`
- **Transições permitidas**:
  - (novo) → `declared` - por `RegisterRepresentation`
  - `declared` → `verified` | `rejected` - por `VerifyRepresentation`
  - `verified` → revogada (`revokedAt` preenchido) - por `RevokeRepresentation`
- **Invariantes de estado**:
  - Uma `Representation` não é alterada: mudança de poderes revoga a existente e registra nova.
  - Proposição e aprovação são de usuários distintos (dupla aprovação obrigatória, sem exceção - `01` §5).
  - Disputa de dono [ABERTA: emissor de RepresentationVerified e dono da prova de poderes - docs/prd/01-prd-identity-organization.md req. 4 e docs/design/92-schemas-eventos.md §4 atribuem o evento ao contexto Identity; docs/design/02-design-kyb-kyc.md §3.3 (CHK-REP-002, CHK-REP-003) atribui a comprovação documental dos poderes ao KYB/KYC; nenhum documento decide quem emite o evento nem quem aprova a transição declared para verified].

### UserAccount

- **Identidade**: `natural` por `subject` (identificador estável no provedor de identidade externo [ABERTA: docs/design/01-design-identity-organization.md:§7 I8])
- **Campos**:
  - `subject: string`
  - `personId: id` (opcional)
  - `status: active | suspended`
  - `provisionedAt: instant`, `suspendedAt: instant` (opcional), `suspensionReason: string` (opcional)
- **Estados**: `active`, `suspended`
- **Transições permitidas**:
  - (novo) → `active` - por `ProvisionUserAccount`
  - `active` → `suspended` - por `SuspendUser`
- **Invariantes de estado**: remoção ou suspensão de usuário nunca apaga evidências nem histórico anteriores; `subject` nunca é vazio (`identity-organization/user-account/subject-empty`).

### Membership

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `inviteeEmail: string` (PII)
  - `role: operator | approver | org_admin | auditor | partner_analyst | partner_compliance | platform_ops | platform_risk`
  - `status: invited | active | revoked`
  - `invitedByUserId: id`, `invitedAt: instant`, `invitationExpiresAt: instant`
  - `userId: id`, `personId: id` (opcionais até a concessão)
  - `verificationLevel: channel_ownership | personal_data_checked | document_and_liveness` (opcional até a concessão)
  - `approvedByUserId: id` (opcional)
  - `grantedAt: instant`, `revokedAt: instant`, `revocationReason: string` (opcionais)
- **Estados**: `invited`, `active`, `revoked`
- **Transições permitidas**:
  - (novo) → `invited` - por `InviteUser`
  - `invited` → `active` - por `GrantMembership`
  - `active` → `revoked` - por `RevokeMembership` [ABERTA: docs/prd/01-prd-identity-organization.md req. 7 exige revogar acesso, mas o PRD só lista `SuspendUser`, que atinge o usuário inteiro]
- **Invariantes de estado**:
  - O papel não muda: troca de papel revoga o vínculo e cria outro.
  - Papéis de plataforma só em organização com `kind` `platform`; papéis de cliente nunca se misturam com eles (`identity-organization/membership/role-kind-mismatch`).
  - Nível mínimo de verificação por papel: `auditor` = `channel_ownership`; `operator` = `personal_data_checked`; `approver` e `org_admin` = `document_and_liveness` (`01` §3). Papéis de parceiro e de plataforma [ABERTA: docs/design/01-design-identity-organization.md:§3 - nível não definido].

### BankAccountChange

[ABERTA: docs/prd/01-prd-identity-organization.md:Modelo de domínio - nenhum agregado ou comando do PRD cobre a mudança de conta; `docs/design/92` §4 declara os eventos e `docs/design/01` §5 declara a dupla aprovação obrigatória. Este desenho é o mínimo coerente. O dado bancário em si (`PII_SENSITIVE`) não vive aqui; os controles de quarentena e reconfirmação são de Funding, `docs/design/10` §4.]

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `destinationEvidenceId: id`
  - `requestedByUserId: id`, `approvedByUserId: id` (opcional)
  - `status: requested | approved`
  - `requestedAt: instant`, `approvedAt: instant` (opcional)
- **Estados**: `requested`, `approved`
- **Transições permitidas**:
  - (novo) → `requested` - por `RequestBankAccountChange`
  - `requested` → `approved` - por `ApproveBankAccountChange`
- **Invariantes de estado**: aprovador distinto do proponente.

### Role e Permission

Não são agregados: catálogo fechado em código, derivado da matriz de `docs/design/95-personas-jornadas.md` ("Matriz de papéis e permissões"), referenciada e não reescrita. Papéis de cliente: `operator`, `approver`, `org_admin`, `auditor`. Papéis de financiador: `partner_analyst`, `partner_compliance`. Papéis de plataforma: `platform_ops`, `platform_risk`. Invariante: nenhum papel acumula proposição e aprovação do mesmo ato sensível.

## Comandos (UPRs)

### RegisterOrganization

- **Agregado**: `Organization`; **cria**
- **Entrada**: `kinds: list<...>`, `jurisdiction`, `taxIdType`, `taxId`, `taxIdHash`, `legalName`, `tradeName` (opcional)
- **Pré-condições**:
  - `kinds` não vazio → `identity-organization/organization/kinds-empty`
  - `jurisdiction` com duas letras → `identity-organization/organization/jurisdiction-invalid`
  - `taxId` não vazio → `identity-organization/organization/tax-id-empty`
  - `legalName` não vazio → `identity-organization/organization/legal-name-empty`
  - `(taxIdHash, jurisdiction)` ainda não registrado (††) → `identity-organization/organization/duplicate-tax-id`
- **Efeitos sobre o estado**: `status ← active`; demais campos `← cmd`; `createdAt ← at`
- **Eventos emitidos**: `OrganizationRegistered`
- **Resposta**: `organizationId: id`

### AddEstablishment

- **Agregado**: `Establishment`; **cria**
- **Entrada**: `organizationId`, `taxIdType`, `taxId`, `taxIdHash`, `tradeName` (opcional), `receiver`, `externalReferences`
- **Pré-condições**:
  - organização existente e `active` (†) → `identity-organization/establishment/organization-not-active`
  - `taxId` não vazio → `identity-organization/establishment/tax-id-empty`
  - `(organizationId, taxIdHash)` ainda não registrado (††) → `identity-organization/establishment/duplicate-tax-id`
- **Efeitos sobre o estado**: campos `← cmd`; `createdAt ← at`
- **Eventos emitidos**: `EstablishmentAdded`
- **Resposta**: `establishmentId: id`

### RegisterPerson

[ABERTA: docs/prd/01-prd-identity-organization.md:Comandos e eventos - nenhum comando cria `Person`.]

- **Agregado**: `Person`; **cria**
- **Entrada**: `fullName`, `document`, `documentHash`, `documentMasked`
- **Pré-condições**:
  - `fullName` não vazio → `identity-organization/person/full-name-empty`
  - `document` não vazio → `identity-organization/person/document-empty`
  - `documentHash` ainda não registrado (††) → `identity-organization/person/duplicate-document`
- **Efeitos sobre o estado**: campos `← cmd`; `createdAt ← at`
- **Eventos emitidos**: nenhum
- **Resposta**: `personId: id`

### RegisterRepresentation

- **Agregado**: `Representation`; **cria**
- **Entrada**: `organizationId`, `personId`, `powerType`, `sourceType`, `sourceEvidenceId`, `scopeCurrency`, `scopeMaxAmountMinor`, `scopeActTypes` (alçada opcional), `validFrom`, `validUntil` (opcional), `proposedByUserId`
- **Pré-condições**:
  - `sourceEvidenceId` não vazio (origem e evidência dos poderes, req. 4) → `identity-organization/representation/source-evidence-missing`
  - `validUntil`, quando informado, maior que `validFrom` → `identity-organization/representation/validity-invalid`
  - alçada com `scopeMaxAmountMinor` positivo e `scopeCurrency` informada, ou ausente por inteiro → `identity-organization/representation/scope-invalid`
  - organização existente e `active` (†) → `identity-organization/representation/organization-not-active`
  - pessoa existente (†) → `identity-organization/representation/person-not-found`
- **Efeitos sobre o estado**: `verificationStatus ← declared`; campos `← cmd`
- **Eventos emitidos**: nenhum (o fato verificado nasce em `VerifyRepresentation`)
- **Resposta**: `representationId: id`

### VerifyRepresentation

[ABERTA: docs/prd/01-prd-identity-organization.md:Comandos e eventos - o PRD não lista o comando que emite `RepresentationVerified`; ver a disputa de emissor no agregado `Representation`.]

- **Agregado**: `Representation`; exige existente
- **Entrada**: `decision: verified | rejected`, `verifiedByUserId`
- **Pré-condições**:
  - `verificationStatus = declared` → `identity-organization/representation/not-declared`
  - `verifiedByUserId` distinto de `proposedByUserId` → `identity-organization/representation/same-actor-approval`
- **Efeitos sobre o estado**: `verificationStatus ← cmd.decision`; `approvedByUserId ← cmd.verifiedByUserId`
- **Eventos emitidos**: `RepresentationVerified` (só quando `decision = verified`)
- **Resposta**: `representationId: id`

### RevokeRepresentation

- **Agregado**: `Representation`; exige existente
- **Entrada**: `reason`, `requestedByUserId`, `approvedByUserId` [ABERTA: docs/design/01-design-identity-organization.md:§5 - o mecanismo de captura da segunda aprovação da revogação (dois atores no comando ou dois passos) não está definido]
- **Pré-condições**:
  - `verificationStatus = verified` e `revokedAt` ausente → `identity-organization/representation/not-active`
  - `approvedByUserId` distinto de `requestedByUserId` → `identity-organization/representation/same-actor-approval`
  - `reason` não vazio → `identity-organization/representation/reason-empty`
- **Efeitos sobre o estado**: `revokedAt ← at`; `revocationReason ← cmd.reason`; usuários da revogação `← cmd`
- **Eventos emitidos**: `RepresentationRevoked`
- **Resposta**: `representationId: id`

### ProvisionUserAccount

[ABERTA: docs/design/01-design-identity-organization.md:§2.1 - a criação de `UserAccount` na federação OIDC não tem comando no PRD.]

- **Agregado**: `UserAccount`; **inicializa se ausente**
- **Entrada**: `subject`, `personId` (opcional)
- **Pré-condições**: `subject` não vazio → `identity-organization/user-account/subject-empty`
- **Efeitos sobre o estado**: quando ausente: `status ← active`; `provisionedAt ← at`; quando presente: nenhum - aceita sem alterar e sem emitir
- **Eventos emitidos**: nenhum
- **Resposta**: `userId: id` (o `subject`)

### InviteUser

- **Agregado**: `Membership`; **cria**
- **Entrada**: `organizationId`, `inviteeEmail`, `role`, `invitedByUserId`, `invitationExpiresAt`
- **Pré-condições**:
  - `inviteeEmail` não vazio → `identity-organization/membership/invitee-email-empty`
  - `invitationExpiresAt` maior que `at` → `identity-organization/membership/invitation-expiry-invalid`
  - organização existente e `active` (†) → `identity-organization/membership/organization-not-active`
  - papel compatível com os `kinds` da organização (†) → `identity-organization/membership/role-kind-mismatch`
- **Efeitos sobre o estado**: `status ← invited`; `invitedAt ← at`; campos `← cmd`
- **Eventos emitidos**: nenhum
- **Resposta**: `membershipId: id`

### GrantMembership

- **Agregado**: `Membership`; exige existente
- **Entrada**: `userId`, `personId`, `channelConfirmed: bool`, `verificationLevel`, `approvedByUserId` (opcional)
- **Pré-condições**:
  - `status = invited` → `identity-organization/membership/not-invited`
  - `invitationExpiresAt` maior que `at` → `identity-organization/membership/invitation-expired`
  - `channelConfirmed = true` (posse do canal) → `identity-organization/membership/channel-not-confirmed`
  - `verificationLevel` igual ou acima do mínimo do papel → `identity-organization/membership/verification-insufficient`
  - papel `approver` com `approvedByUserId` presente [ABERTA: docs/design/01-design-identity-organization.md:§5 - os papéis de aprovação não são enumerados; a dupla aprovação é obrigatória por padrão e configurável com aceitação de risco, fora desta versão] → `identity-organization/membership/second-approval-missing`
  - `approvedByUserId` distinto de `invitedByUserId` → `identity-organization/membership/same-actor-approval`
  - usuário existente e `active` (†) → `identity-organization/membership/user-not-active`
- **Efeitos sobre o estado**: `status ← active`; `grantedAt ← at`; `userId`, `personId`, `verificationLevel`, `approvedByUserId ← cmd`
- **Eventos emitidos**: `MembershipGranted`
- **Resposta**: `membershipId: id`

### RevokeMembership

- **Agregado**: `Membership`; exige existente
- **Entrada**: `reason`
- **Pré-condições**: `status = active` → `identity-organization/membership/not-active`
- **Efeitos sobre o estado**: `status ← revoked`; `revokedAt ← at`; `revocationReason ← cmd.reason`
- **Eventos emitidos**: nenhum [ABERTA: docs/design/92-schemas-eventos.md:§4 - sem evento de revogação de vínculo; o bloqueio em menos de 60 s vem da lista de revogação local]
- **Resposta**: `membershipId: id`

### SuspendUser

- **Agregado**: `UserAccount`; exige existente
- **Entrada**: `reason`
- **Pré-condições**:
  - `status = active` → `identity-organization/user-account/not-active`
  - `reason` não vazio → `identity-organization/user-account/reason-empty`
- **Efeitos sobre o estado**: `status ← suspended`; `suspendedAt ← at`; `suspensionReason ← cmd.reason`
- **Eventos emitidos**: `UserSuspended`
- **Resposta**: `userId: id`

### RequestBankAccountChange

- **Agregado**: `BankAccountChange`; **cria**
- **Entrada**: `organizationId`, `destinationEvidenceId`, `requestedByUserId`
- **Pré-condições**:
  - `destinationEvidenceId` não vazio → `identity-organization/bank-account-change/destination-evidence-missing`
  - organização existente e `active` (†) → `identity-organization/bank-account-change/organization-not-active`
- **Efeitos sobre o estado**: `status ← requested`; `requestedAt ← at`; campos `← cmd`
- **Eventos emitidos**: `BankAccountChangeRequested`
- **Resposta**: `bankAccountChangeId: id`

### ApproveBankAccountChange

- **Agregado**: `BankAccountChange`; exige existente
- **Entrada**: `approvedByUserId`
- **Pré-condições**:
  - `status = requested` → `identity-organization/bank-account-change/not-requested`
  - `approvedByUserId` distinto de `requestedByUserId` → `identity-organization/bank-account-change/same-actor-approval`
- **Efeitos sobre o estado**: `status ← approved`; `approvedAt ← at`; `approvedByUserId ← cmd`
- **Eventos emitidos**: `BankAccountChangeApproved`
- **Resposta**: `bankAccountChangeId: id`

## Eventos de domínio

Convenção: `EventName()` é `identity-organization.<agregado>.<evento>`. Campos `PII` entram como hash ou máscara, nunca o valor (`90` §1.4).

### OrganizationRegistered

- **Campos**: `organizationId: id ← Organization.id`; `kinds: list ← Organization.kinds`; `jurisdiction: string`; `taxIdType`; `taxIdHash: string`; `taxIdMasked: string ← máscara de taxId`; `at: instant ← at`

### EstablishmentAdded

- **Campos**: `establishmentId: id`; `organizationId: id`; `taxIdType`; `taxIdHash: string`; `receiver: bool`; `at: instant ← at`

### MembershipGranted

- **Campos**: `membershipId: id`; `organizationId: id`; `userId: id`; `role`; `verificationLevel`; `at: instant ← at`

### RepresentationVerified

[ABERTA: docs/design/92-schemas-eventos.md:§3 - o schema não está definido; campos abaixo são o mínimo derivado de `90` §3.1.]

- **Campos**: `representationId: id`; `organizationId: id`; `personId: id`; `powerType`; `scopeCurrency`, `scopeMaxAmountMinor`, `scopeActTypes` (alçada); `validFrom: instant`; `validUntil: instant` (opcional); `at: instant ← at`

### RepresentationRevoked

- **Campos**: `representationId: id`; `organizationId: id`; `revokedAt: instant ← Representation.revokedAt`; `reason: string ← Representation.revocationReason`

### UserSuspended

- **Campos**: `userId: id`; `reason: string`; `at: instant ← at`

### BankAccountChangeRequested

- **Campos**: `bankAccountChangeId: id`; `organizationId: id`; `destinationEvidenceId: id`; `requestedByUserId: id`; `at: instant ← at`

### BankAccountChangeApproved

- **Campos**: `bankAccountChangeId: id`; `organizationId: id`; `approvedByUserId: id`; `at: instant ← at`

## Consultas

- `FindOrganization` - filtros: `organizationId`; cardinalidade: `one`
- `FindOrganizationByTaxId` - filtros: `taxIdHash`, `jurisdiction`; cardinalidade: `one`
- `FindEstablishmentsByOrganization` - filtros: `organizationId`; cardinalidade: `many`
- `FindPerson` - filtros: `personId`; cardinalidade: `one`
- `FindRepresentation` - filtros: `representationId`; cardinalidade: `one`
- `FindRepresentationsByOrganization` - filtros: `organizationId`, `validAt` (opcional); cardinalidade: `many` (consulta de vigência usada por Eligibility, ORG-REP-001)
- `FindMembershipsByUser` - filtros: `userId`; cardinalidade: `many`
- `FindMembershipsByOrganization` - filtros: `organizationId`; cardinalidade: `many`
- `FindEffectivePermissions` - filtros: `userId`, `organizationId`, `asOf`; cardinalidade: `one`. Estado atual servido localmente: união das permissões dos papéis ativos, limitada pela alçada da representação vigente da `Person` vinculada, descontadas as revogações anteriores a `asOf`. Histórico reconstruído pelo Compliance Ledger, por `AuditSubject` do usuário [ABERTA: docs/design/01-design-identity-organization.md:§4 - a consulta histórica mora no ledger, fora deste contexto]
- `FindUserAccessStatus` - filtros: `userId`; cardinalidade: `one` (lista de revogação com cache de 30 s, `01` §2.2)
- `FindBankAccountChange` - filtros: `bankAccountChangeId`; cardinalidade: `one`
- `FindBankAccountChangesByOrganization` - filtros: `organizationId`; cardinalidade: `many`

## Relações entre agregados

Sempre por identidade. A existência e o estado do agregado referenciado são verificados pelo caso de uso (†).

- `Establishment.organizationId` → `Organization`
- `Representation.organizationId` → `Organization`
- `Representation.personId` → `Person`
- `Representation.sourceEvidenceId` → evidência (fora do contexto)
- `UserAccount.personId` → `Person`
- `Membership.organizationId` → `Organization`
- `Membership.userId` → `UserAccount`
- `Membership.personId` → `Person`
- `BankAccountChange.organizationId` → `Organization`

## Integração

- **Publica** (integration events), em `apps/backend/identity/contract/proto/company/identity/event/v1/`, package `company.identity.event.v1`:
  - `OrganizationRegistered` (`organization_registered.proto`) - consumidores no doc 92 §4: KYB, Ledger
  - `RepresentationVerified` - Consent, Contracting, Ledger
  - `RepresentationRevoked` - Consent, Contracting, Ledger
  - `UserSuspended` - Ledger
  - `BankAccountChangeRequested` e `BankAccountChangeApproved` - evento proposto no doc 92 §4; consumidores no desenho de Funding e Notificações
  - `EstablishmentAdded` e `MembershipGranted` - declarados no PRD, ausentes do catálogo [ABERTA: docs/design/92-schemas-eventos.md:§4 - sem consumidor declarado]
- **Consome**: nenhum.
- **Consultas gRPC servidas a outros contextos**: `FindOrganization` (KYB resolve o identificador fiscal em claro), `FindRepresentationsByOrganization` e `FindOrganizationByTaxId` (Eligibility, Partner, Contracting).

## Borda e persistência

- **Borda**: só gRPC, `company.identity.service.v1.IdentityService`, um RPC por comando e por consulta, atrás da cadeia de interceptors do kernel. O REST do PRD (`POST /organizations`, `POST /organizations/{id}/establishments`, `POST /organizations/{id}/memberships`, `POST /organizations/{id}/representations`, `GET /users/{id}/effective-permissions`) é do `bff` e fica no Escopo fora.
- **Banco**: database e role `identity`, schema `public`, tabelas `organizations`, `establishments`, `persons`, `representations`, `user_accounts`, `memberships` e `bank_account_changes`, com `tenant_id` à frente da chave. Persistência híbrida; colunas tipadas só onde uma consulta filtra: `organizations(tax_id_hash, jurisdiction)` para `FindOrganizationByTaxId` e para a unicidade; `establishments(organization_id)`; `persons(document_hash)` para a unicidade; `representations(organization_id, person_id)`; `memberships(organization_id, user_id)`; `bank_account_changes(organization_id)`. O resto vai em `snapshot` `jsonb`.
- Diverge do design: isolamento por `tenant_id` injetado pelo `ExecutionContext` no `postgres.Table` (ADR-051 do DMPF), sem RLS como mecanismo primário (`docs/design/00` §4.1); `tenantId` não é campo dos agregados.
- Diverge do design: eventos de integração em Protobuf montados em CloudEvents pelo relay; o envelope JSON e `schemaUri` do doc 92 §1 são ignorados.
- Diverge do design: `instant` é inteiro de nanossegundos no domínio (o doc 00 §8 fala em milissegundos).
- Acesso cross-tenant dos papéis de plataforma e proibição de acesso cruzado entre financiadores [ABERTA: requer extensão do kernel - ADR-051 DMPF].

## Políticas transversais

- **Idempotência**: os comandos de criação (`RegisterOrganization`, `AddEstablishment`, `RegisterPerson`, `RegisterRepresentation`, `InviteUser`, `RequestBankAccountChange`) entram pelo `bff` por `POST` com chave de idempotência declarada, que segue na metadata gRPC; `ProvisionUserAccount` é idempotente por identidade natural; os demais, por identidade do agregado (RST-02; FND-04; ADR-056).
- **Autorização**: pelo gancho do bloco `application`, antes da transação (FND-04 §3.2), combinando tenant, recurso, papel e representação. MFA e reautenticação nos atos críticos (`01` §2.1) dependem do provedor de identidade [ABERTA: docs/design/01-design-identity-organization.md:§2.1]. Revogação efetiva em menos de 60 s: token curto de 5 min mais lista de revogação com cache de 30 s nas operações sensíveis, recomendação de `01` §2.2.
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); o Compliance Ledger consome os eventos. Dados pessoais mascarados em log conforme a classificação do doc 90 §1.4.

## Critérios de aceite

- [ ] Os gates do workspace verdes para `identity` (`fmt-check`, `vet`, `build`, `lint`, `test-race`, `test-distributed` com Postgres, `dmpf-context-check.sh`, verificador com `--base`).
- [ ] Cenários de aceite por comando:

```text
DADO nenhuma organização com aquele identificador fiscal e jurisdição
QUANDO RegisterOrganization chega com kinds [client], taxId, legalName e jurisdiction BR
ENTÃO a organização nasce active com createdAt = at e OrganizationRegistered vai à outbox com hash e máscara, sem o taxId

DADO uma organização active
QUANDO AddEstablishment chega com receiver true e um taxId novo
ENTÃO o estabelecimento é criado e EstablishmentAdded é emitido

DADO uma pessoa ainda desconhecida
QUANDO RegisterPerson chega com fullName, document e documentHash novos
ENTÃO a pessoa é criada sem emitir evento

DADO uma pessoa e uma organização active
QUANDO RegisterRepresentation chega com fonte, evidência e vigência válidas
ENTÃO a representação nasce declared e nenhum evento é emitido

DADO uma representação declared proposta pelo usuário U1
QUANDO VerifyRepresentation chega com decision verified pelo usuário U2
ENTÃO verificationStatus vai a verified e RepresentationVerified é emitido

DADO uma representação declared
QUANDO VerifyRepresentation chega com decision rejected
ENTÃO verificationStatus vai a rejected e nenhum evento é emitido

DADO uma representação verified
QUANDO RevokeRepresentation chega com reason e dois usuários distintos
ENTÃO revokedAt = at e RepresentationRevoked é emitido

DADO um subject ainda desconhecido
QUANDO ProvisionUserAccount chega
ENTÃO a conta nasce active; repetida com o mesmo subject, aceita sem alterar

DADO uma organização active
QUANDO InviteUser chega com e-mail, papel operator e expiração futura
ENTÃO o vínculo nasce invited

DADO um vínculo invited, canal confirmado e verificação suficiente para o papel
QUANDO GrantMembership chega
ENTÃO o vínculo vai a active com grantedAt = at e MembershipGranted é emitido

DADO um vínculo active
QUANDO RevokeMembership chega
ENTÃO o vínculo vai a revoked e FindUserAccessStatus passa a refletir o bloqueio

DADO uma conta active
QUANDO SuspendUser chega com reason
ENTÃO a conta vai a suspended e UserSuspended é emitido

DADO uma organização active
QUANDO RequestBankAccountChange chega
ENTÃO a mudança nasce requested e BankAccountChangeRequested é emitido

DADO uma mudança requested pelo usuário U1
QUANDO ApproveBankAccountChange chega pelo usuário U2
ENTÃO a mudança vai a approved e BankAccountChangeApproved é emitido

DADO um usuário com vínculo active e representação vigente com alçada
QUANDO FindEffectivePermissions consulta no instante atual
ENTÃO devolve as permissões do papel limitadas pela alçada, fora de qualquer UoW
```

- [ ] Cenários de rejeição, um por código (nada é escrito em nenhum deles):

```text
DADO qualquer entrada QUANDO RegisterOrganization chega com kinds vazio ENTÃO rejeita com identity-organization/organization/kinds-empty
DADO qualquer entrada QUANDO RegisterOrganization chega com jurisdiction de tamanho diferente de dois ENTÃO rejeita com identity-organization/organization/jurisdiction-invalid
DADO qualquer entrada QUANDO RegisterOrganization chega com taxId vazio ENTÃO rejeita com identity-organization/organization/tax-id-empty
DADO qualquer entrada QUANDO RegisterOrganization chega com legalName vazio ENTÃO rejeita com identity-organization/organization/legal-name-empty
DADO uma organização com o mesmo taxIdHash e jurisdiction QUANDO RegisterOrganization chega ENTÃO rejeita com identity-organization/organization/duplicate-tax-id
DADO uma organização suspended QUANDO AddEstablishment chega ENTÃO rejeita com identity-organization/establishment/organization-not-active
DADO qualquer entrada QUANDO AddEstablishment chega com taxId vazio ENTÃO rejeita com identity-organization/establishment/tax-id-empty
DADO um estabelecimento com o mesmo taxIdHash na organização QUANDO AddEstablishment chega ENTÃO rejeita com identity-organization/establishment/duplicate-tax-id
DADO qualquer entrada QUANDO RegisterPerson chega com fullName vazio ENTÃO rejeita com identity-organization/person/full-name-empty
DADO qualquer entrada QUANDO RegisterPerson chega com document vazio ENTÃO rejeita com identity-organization/person/document-empty
DADO uma pessoa com o mesmo documentHash QUANDO RegisterPerson chega ENTÃO rejeita com identity-organization/person/duplicate-document
DADO qualquer entrada QUANDO RegisterRepresentation chega sem sourceEvidenceId ENTÃO rejeita com identity-organization/representation/source-evidence-missing
DADO qualquer entrada QUANDO RegisterRepresentation chega com validUntil menor ou igual a validFrom ENTÃO rejeita com identity-organization/representation/validity-invalid
DADO qualquer entrada QUANDO RegisterRepresentation chega com scopeMaxAmountMinor sem scopeCurrency ENTÃO rejeita com identity-organization/representation/scope-invalid
DADO uma organização offboarded QUANDO RegisterRepresentation chega ENTÃO rejeita com identity-organization/representation/organization-not-active
DADO um personId inexistente QUANDO RegisterRepresentation chega ENTÃO rejeita com identity-organization/representation/person-not-found
DADO uma representação verified QUANDO VerifyRepresentation chega ENTÃO rejeita com identity-organization/representation/not-declared
DADO uma representação declared proposta por U1 QUANDO VerifyRepresentation chega por U1 ENTÃO rejeita com identity-organization/representation/same-actor-approval
DADO uma representação declared QUANDO RevokeRepresentation chega ENTÃO rejeita com identity-organization/representation/not-active
DADO uma representação verified QUANDO RevokeRepresentation chega com requestedByUserId igual a approvedByUserId ENTÃO rejeita com identity-organization/representation/same-actor-approval
DADO uma representação verified QUANDO RevokeRepresentation chega com reason vazio ENTÃO rejeita com identity-organization/representation/reason-empty
DADO qualquer entrada QUANDO ProvisionUserAccount chega com subject vazio ENTÃO rejeita com identity-organization/user-account/subject-empty
DADO uma conta suspended QUANDO SuspendUser chega ENTÃO rejeita com identity-organization/user-account/not-active
DADO uma conta active QUANDO SuspendUser chega com reason vazio ENTÃO rejeita com identity-organization/user-account/reason-empty
DADO qualquer entrada QUANDO InviteUser chega com e-mail vazio ENTÃO rejeita com identity-organization/membership/invitee-email-empty
DADO qualquer entrada QUANDO InviteUser chega com expiração anterior a at ENTÃO rejeita com identity-organization/membership/invitation-expiry-invalid
DADO uma organização suspended QUANDO InviteUser chega ENTÃO rejeita com identity-organization/membership/organization-not-active
DADO uma organização client QUANDO InviteUser chega com papel platform_ops ENTÃO rejeita com identity-organization/membership/role-kind-mismatch
DADO um vínculo active QUANDO GrantMembership chega ENTÃO rejeita com identity-organization/membership/not-invited
DADO um vínculo invited expirado QUANDO GrantMembership chega ENTÃO rejeita com identity-organization/membership/invitation-expired
DADO um vínculo invited QUANDO GrantMembership chega com channelConfirmed false ENTÃO rejeita com identity-organization/membership/channel-not-confirmed
DADO um vínculo approver QUANDO GrantMembership chega com verificationLevel personal_data_checked ENTÃO rejeita com identity-organization/membership/verification-insufficient
DADO um vínculo approver QUANDO GrantMembership chega sem approvedByUserId ENTÃO rejeita com identity-organization/membership/second-approval-missing
DADO um vínculo approver convidado por U1 QUANDO GrantMembership chega com approvedByUserId U1 ENTÃO rejeita com identity-organization/membership/same-actor-approval
DADO uma conta suspended QUANDO GrantMembership chega ENTÃO rejeita com identity-organization/membership/user-not-active
DADO um vínculo invited QUANDO RevokeMembership chega ENTÃO rejeita com identity-organization/membership/not-active
DADO qualquer entrada QUANDO RequestBankAccountChange chega sem destinationEvidenceId ENTÃO rejeita com identity-organization/bank-account-change/destination-evidence-missing
DADO uma organização suspended QUANDO RequestBankAccountChange chega ENTÃO rejeita com identity-organization/bank-account-change/organization-not-active
DADO uma mudança approved QUANDO ApproveBankAccountChange chega ENTÃO rejeita com identity-organization/bank-account-change/not-requested
DADO uma mudança requested por U1 QUANDO ApproveBankAccountChange chega por U1 ENTÃO rejeita com identity-organization/bank-account-change/same-actor-approval
```

## Escopo fora

- **REST do `bff`**: Client API e rotas do PRD 01 ficam no `bff` (referência de superfícies em `docs/design/91-contratos-api.md` §1); este contexto serve só gRPC.
- **Provedor de identidade, MFA, sessões e verificação documental ou de vivacidade**: dependem de C5; o contexto guarda o `subject`, o nível de verificação atingido e as listas de revogação, não executa a autenticação.
- **Configuração por organização da dupla aprovação** dos atos configuráveis (seleção de oferta, assinatura, revogação de consentimento, concessão de papel de aprovação) e o regime de aprovação única assistida: aguardam `01:I10` (validação jurídica); só as duas aprovações obrigatórias estão modeladas.
- **Grupo econômico** (`01:I11`): cada filial com identificador fiscal próprio é uma `Organization` distinta, sem hierarquia entre elas.
- **Comandos de suspensão e descontinuação de organização** e de alteração de papel: não definidos no PRD.
- **Verificação cadastral aprofundada**: KYB/KYC; **mandatos transacionais**: Consent & Mandates.
- **Histórico de permissões e reconstrução temporal**: Compliance Ledger.
- **Dado bancário, validação de titularidade, quarentena e reconfirmação de conta**: Funding (`docs/design/10` §4).
- **Fases posteriores**: o sacado como ator (`docs/design/95`), pagador de Billing e acesso cross-tenant dependem de decisões ou de extensão do kernel.
