---
id: SPEC-7KW0HHF6
slug: identity-organization
title: Bounded context — identity-organization
stage: done
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
  - *Pessoa* (`Person`) - pessoa natural identificada de forma protegida, por hash e máscara do documento; o documento em claro só entra com a cifra em envelope do kernel (SPEC-CC5SHV1K).
  - *Representação* (`Representation`) - vínculo entre pessoa e organização, com tipo de poder, fonte, alçada e vigência; nunca é alterada, só revogada e substituída — a revogação chega com `RevokeRepresentation`, no Escopo fora.
  - *Alçada* (`scope`) - limite de valor ou de tipo de ato imposto pelo documento de poderes.
  - *Conta de usuário* (`UserAccount`) - identidade autenticável, federada ao Keycloak (ADR-016) pelo `sub`.
  - *Vínculo* (`Membership`) - acesso de um usuário a uma organização com um papel.
  - *Papel* e *permissão* - autorização funcional; papéis de cliente e de plataforma permanecem separados.
  - *Tenant* - domínio de isolamento de dados (ADR-013); nesta versão, um por organização, atribuído pelo backoffice ao criar a organização no Keycloak.
  - *Dupla aprovação* - proposição e aprovação do ato sensível por usuários distintos, em requisições distintas.
  - *Permissão efetiva* - permissões dos papéis dos vínculos ativos de um usuário, no instante da requisição; resolvidas pelo `bff` a cada requisição (ADR-016, condição 3).

## Origem e bloqueios externos

- **Fontes**: `docs/prd/01-prd-identity-organization.md`; `docs/design/01-design-identity-organization.md` §2 a §6; `docs/design/90-modelo-de-dados.md` §1.4 e §3.1; `docs/design/92-schemas-eventos.md` §4; matriz de papéis em `docs/design/95-personas-jornadas.md`; `docs/prd/14-catalogo-contratos-eventos.md`; ADR-013, ADR-015, ADR-016, ADR-021 e ADR-022 (linhas 2, 6, 8, 9, 14 e 15).
- **Situação**: `desbloqueado` - a fundação está concluída e o núcleo desta versão roda no kernel `1.0.0-rc.4`, com dois contornos registrados no ADR-022 (linhas 14 e 15).
- **Fase de entrega**: 1.
- **Dependências**: `depends_on` declara só a fundação `SPEC-GXGASP24`. Upstream duro de contexto: nenhum. Upstream brando: Compliance Ledger (histórico de permissões, `docs/design/01` §4). Este contexto é a raiz da cadeia dura (identity, partner, gateway, consent, portfolio, eligibility, marketplace, contracting, effects, funding, settlement). Extensões do kernel que tocam este contexto, nenhuma bloqueando a construção desta versão:
  - SPEC-SBW7HT6Y (DEVS-97) - Keycloak de referência e claim `tenant_id`;
  - SPEC-STEJG347 (DEVS-101) - revogação de sessão no `authn`;
  - SPEC-QVDGV6J0 (DEVS-100) - leitura de plataforma e projeção entre tenants, entregue no `1.0.0-rc.6` (ADR-068 do DMPF); o uso aqui depende da adoção da rc.6 (DEVS-136);
  - SPEC-CWBA4BPV (DEVS-95) - object storage das evidências;
  - SPEC-CC5SHV1K (DEVS-102) - cifra em envelope de `PII` e `PII_SENSITIVE`;
  - SPEC-7SRQ3RPK (DEVS-103) - violação de unicidade traduzida em rejeição.
- **Bloqueios externos**:

| ID | o que trava | parte do contexto afetada |
| --- | --- | --- |
| C5a (`01:I8`) | resolvido: Keycloak 26.x com realm único (ADR-016); claim de tenant e eventos de sessão dependem de SPEC-SBW7HT6Y e SPEC-STEJG347 | identidade de `UserAccount` por `sub`; operação real da borda |
| C5b (`01:I9`) | provedor de verificação documental e de vivacidade não escolhido [ABERTA: docs/design/01-design-identity-organization.md:§7 I9] | vínculos `approver` e `org_admin` (nível `document_and_liveness`), fora desta versão |
| A10 do resumo (`90:L3`) | limite de alçada em procuração e tratamento de assinatura acima dos poderes | campo `scope` de `Representation` |
| `01:I10` | regime de aprovação única assistida exige validação jurídica | configuração da dupla aprovação por organização (fora desta versão) |
| `01:I11` | tratamento de grupo econômico | relação matriz-filial entre organizações distintas |
| `90:L8` | rotação da chave secreta do HMAC (o `salt` do doc 90) que protege identificador fiscal e documento; a custódia é deste contexto: a chave vive só na configuração do `identity`, e a rotação é proibida nesta versão (‡) [ABERTA: docs/design/90-modelo-de-dados.md:L8 - política de rotação] | `taxIdHash`, `documentHash` |
| A1 (#1), transversal | autorização regulatória da plataforma | existencial; não bloqueia a construção |
| #12, transversal | volumetria | dimensionamento, inclusive partições Kafka (ADR-015); não a construção |
| K4 (SPEC-QVDGV6J0, entregue na `1.0.0-rc.6`; adoção na DEVS-136) | leitura e projeção entre tenants | consultas servidas a contextos de outro tenant, papéis de plataforma, autocadastro e unicidade global (fora desta versão) |

- **Decisões registradas nesta spec (2026-10-12)**:
  1. *Origem das permissões efetivas* (ADR-016, condição 3): enriquecimento no `bff`. O Keycloak autentica e entrega o tenant; o `bff` resolve as permissões a cada requisição pela consulta `FindEffectivePermissions` deste contexto, sem cache (CTX-16). A alternativa de permissões como claims do Keycloak foi descartada por criar duas fontes de verdade e por só revogar vínculo na expiração do token, já que a SPEC-STEJG347 exclui revogação por pertencimento ou permissão. Autorizar por papel no contexto consumidor foi descartado por CTX-12 e IDN-10.
  2. *Tenant da organização*: um tenant por organização, já provisionado nesta versão - o backoffice cria a organização no Keycloak com o atributo `tenant_id` (SPEC-SBW7HT6Y), e `RegisterOrganization` roda nesse tenant pelo caminho normal do kernel. Autocadastro e unicidade global ficam para fatia posterior, com K4.
  3. *Lacunas do kernel*: cifra em envelope e unicidade como rejeição nascem como specs no kernel (ADR-022, regra 3). Até a implementação, este contexto usa os contornos das notas (††) e (‡‡).
  4. *Escopo desta versão*: o núcleo implementável sem decisão de produto aberta; o restante está em Escopo fora.
- **Decisões de produto abertas**: regime de aprovação única assistida [ABERTA: docs/design/01-design-identity-organization.md:§5]; sacado no escopo (`docs/design/95-personas-jornadas.md`, ator A10 daquele documento, distinto de A10 do resumo executivo); emissor de `RepresentationVerified` (ver `Representation`); passo de aprovação da revogação de representação (ver Escopo fora); primeiro vínculo de cada organização (ver Escopo fora).
- **Condição para sair do backlog** (cumprida): `SPEC-GXGASP24` concluída e as decisões acima registradas. Nada externo bloqueou o núcleo desta versão.

## Agregados

Nota (†): pré-condição que depende de outro agregado é verificada no `Run` do caso de uso por `Load` de outro repositório da mesma UoW, antes de `usecase.Decide` (FRT-01 do DMPF); `ErrNotFound` vira a rejeição declarada. O agregado lido não entra no lock otimista; o risco de *write skew* é aceito porque as pré-condições só leem estado de mudança rara (`active`).

Nota (††): unicidade é por tenant, com `tenant_id` à frente do índice único (ADR-013). Até a SPEC-7SRQ3RPK (ADR-022, linha 15), vale um contorno explícito à UOW-11 do DMPF: o `Run` do caso de uso, já depois do registro idempotente, faz uma leitura consultiva por porta de leitura, que roda em autocommit separado da UoW do comando, e devolve a rejeição declarada quando acha o valor. Nessa leitura, `CrossTenantAccess` conta como ausente no tenant; a chave composta é refinada em memória sobre o resultado da coluna filtrada, porque o `Relation` do kernel filtra por uma coluna. O índice único é a autoridade: numa corrida, exatamente uma requisição grava e a outra sai como erro técnico (`Unexpected`), sem linha nem evento. Unicidade entre tenants está no Escopo fora.

Nota (‡): `taxIdHash` e `documentHash` vêm de uma porta de proteção de identificador, declarada no bloco `ports` e realizada no `provider` - o `depguard` proíbe `crypto/*` em `domain` e `application`; o caso de uso a chama antes da UPR. A porta calcula HMAC-SHA256 com a chave secreta da configuração do `identity` (o "salt" do doc 90, que aqui é chave de HMAC, não salt) sobre a entrada `v1 | tenant | tipo | valor normalizado`, com cada campo prefixado pelo próprio tamanho, e devolve `{hash, keyVersion}`, com o hash em hexadecimal. Normalização por tipo: CNPJ em maiúsculas e sem pontuação, aceitando o formato alfanumérico; CPF e documento de pessoa, só dígitos. O tenant na entrada separa os hashes entre tenants. A `keyVersion` é persistida ao lado de cada hash. A chave tem no mínimo 32 bytes, é validada na partida e nunca aparece em log. Rotação proibida nesta versão: sem o documento em claro, os hashes antigos não podem ser recalculados. A máscara é função pura do bloco `domain`. A entrada gRPC traz o valor em claro; ele nunca sai do contexto em evento nem em log.

Nota (‡‡): campos `PII` são guardados sem envelope adicional até a SPEC-CC5SHV1K; campos `PII_SENSITIVE` não são persistidos nesta versão (ADR-022, linha 14).

Nota (§): campos de ator (`*ByUserId`) contêm o `sub` do usuário autenticado, preenchido pelo `bff` a partir do token verificado, nunca do corpo da requisição externa (CTX-06). No contexto, são proveniência e dado da invariante de segregação de funções [ABERTA: confirmar que a invariante de segregação não conta como "insumo de autorização no destino" da CTX-12].

### Organization

- **Identidade**: `generated`
- **Campos**:
  - `kinds: list<client | funder | partner | platform>` - coleção indispensável: a mesma pessoa jurídica acumula papéis (`90` §3.1)
  - `jurisdiction: string` (ISO 3166-1 alfa-2)
  - `taxIdType: cnpj | cpf`
  - `taxId: string` (`PII`, ‡‡)
  - `taxIdHash: string`, `taxIdHashKeyVersion: int`, `taxIdMasked: string` (‡)
  - `legalName: string` (`PII`, ‡‡)
  - `tradeName: string` (`PII`, ‡‡, opcional)
  - `status: active | suspended | offboarded`
  - `createdAt: instant`
- **Estados**: `active`, `suspended`, `offboarded`
- **Transições permitidas**:
  - (novo) → `active` - por `RegisterOrganization`
  - `active` → `suspended` e → `offboarded` - sem comando definido [ABERTA: docs/prd/01-prd-identity-organization.md:Comandos e eventos - o PRD não lista comando de suspensão ou descontinuação, embora `90` §3.1 declare os estados]
- **Invariantes de estado**:
  - `(taxIdHash, jurisdiction)` é único no tenant (††) - violação rejeita com `identity-organization/organization/duplicate-tax-id`.
  - O identificador fiscal nunca é chave técnica; a chave é gerada.
  - `taxId` do tipo `cnpj` aceita o formato alfanumérico (letras e dígitos nas doze primeiras posições), em vigor desde julho de 2026 (ADR-021); hash e máscara não presumem só dígitos.
  - `kinds`, `jurisdiction`, `taxId` e `legalName` nunca são vazios (rejeições em `RegisterOrganization`).
  - A organização nasce no tenant da requisição; `tenantId` não é campo do agregado.

### Establishment

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `taxIdType: cnpj | cpf`, `taxId: string` (`PII`, ‡‡), `taxIdHash: string`, `taxIdHashKeyVersion: int`, `taxIdMasked: string` (‡)
  - `tradeName: string` (opcional)
  - `receiver: bool` - estabelecimento recebedor credenciado em arranjo [ABERTA: docs/design/01-design-identity-organization.md:§6 - como se comprova o credenciamento]
  - `externalReferences: list<ExternalReference>` - referências em sistemas externos (`90` §2.1), cardinalidade N
  - `createdAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - `(organizationId, taxIdHash)` é único no tenant (††) - rejeita com `identity-organization/establishment/duplicate-tax-id` (os recebíveis pertencem ao estabelecimento identificado pelo seu identificador fiscal, `01` §6 regra 2).

### Person

- **Identidade**: `generated`
- **Campos**:
  - `fullName: string` (`PII`, ‡‡)
  - `documentHash: string`, `documentHashKeyVersion: int`, `documentMasked: string` (‡)
  - `createdAt: instant`
- **Estados**: nenhum
- **Transições permitidas**: nenhuma
- **Invariantes de estado**:
  - `documentHash` é único no tenant (††) - rejeita com `identity-organization/person/duplicate-document`.
  - O `document` (`PII_SENSITIVE`) é usado só para derivar hash e máscara e não é persistido nesta versão; o documento em claro, com envelope e chave segregada, entra com a SPEC-CC5SHV1K (‡‡).

### Representation

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`, `personId: id`
  - `powerType: legal_representative | attorney_in_fact | operator`
  - `sourceType: articles_of_association | power_of_attorney | board_resolution | internal_delegation`
  - `sourceEvidenceId: id` - id opaco da evidência; os bytes vão para a porta de object storage WORM da SPEC-CWBA4BPV, fora deste agregado
  - `scopeCurrency: string`, `scopeMaxAmountMinor: int`, `scopeActTypes: list<string>` - alçada, todos opcionais [ABERTA: docs/design/90-modelo-de-dados.md:§5 L3 - formato do limite e tratamento acima dos poderes]
  - `validFrom: instant`, `validUntil: instant` (opcional; ausente = indeterminado)
  - `verificationStatus: declared` - nesta versão; `verified` e `rejected` entram com `VerifyRepresentation` (Escopo fora)
  - `proposedByUserId: id` (§)
- **Estados**: `declared` nesta versão; a verificação e a revogação estão no Escopo fora.
- **Transições permitidas**:
  - (novo) → `declared` - por `RegisterRepresentation`
- **Invariantes de estado**:
  - Uma `Representation` não é alterada: mudança de poderes revoga a existente e registra nova — a revogação chega com `RevokeRepresentation`, no Escopo fora.
  - Disputa de dono [ABERTA: emissor de RepresentationVerified e dono da prova de poderes - docs/prd/01-prd-identity-organization.md req. 4 e docs/design/92-schemas-eventos.md §4 atribuem o evento ao contexto Identity; docs/design/02-design-kyb-kyc.md §3.3 (CHK-REP-002, CHK-REP-003) atribui a comprovação documental dos poderes ao KYB/KYC; nenhum documento decide quem emite o evento nem quem aprova a transição declared para verified].

### UserAccount

- **Identidade**: `natural` por `subject`, igual à claim `sub` do realm único do Keycloak (ADR-016), lida pelo `authn` do kernel. A identidade natural do kernel é por tenant: o mesmo `sub` tem uma `UserAccount` em cada tenant em que atua, e `SuspendUser` atinge só a conta do tenant da requisição.
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
  - `inviteeEmail: string` (`PII`, ‡‡)
  - `role: operator | auditor` - papéis desta versão; os demais estão no Escopo fora
  - `status: invited | active | revoked`
  - `invitedByUserId: id` (§), `invitedAt: instant`, `invitationExpiresAt: instant`
  - `userId: id`, `personId: id` (opcionais até a concessão)
  - `verificationLevel: channel_ownership | personal_data_checked` (opcional até a concessão)
  - `grantedAt: instant`, `revokedAt: instant`, `revocationReason: string` (opcionais)
- **Estados**: `invited`, `active`, `revoked`
- **Transições permitidas**:
  - (novo) → `invited` - por `InviteUser`
  - `invited` → `active` - por `GrantMembership`
  - `active` → `revoked` - por `RevokeMembership` [ABERTA: docs/prd/01-prd-identity-organization.md req. 7 exige revogar acesso, mas o PRD só lista `SuspendUser`, que atinge o usuário inteiro]
- **Invariantes de estado**:
  - O papel não muda: troca de papel revoga o vínculo e cria outro.
  - Papel de cliente (`operator`, `auditor`) exige `client` entre os `kinds` da organização (`identity-organization/membership/role-kind-mismatch`).
  - Nível mínimo de verificação por papel: `auditor` = `channel_ownership`; `operator` = `personal_data_checked` (`01` §3). O nível é informado pela borda, que conduz a verificação.

### BankAccountChange

[ABERTA: docs/prd/01-prd-identity-organization.md:Modelo de domínio - nenhum agregado ou comando do PRD cobre a mudança de conta; `docs/design/92` §4 declara os eventos e `docs/design/01` §5 declara a dupla aprovação obrigatória. Este desenho é o mínimo coerente. O dado bancário em si (`PII_SENSITIVE`) não vive aqui; os controles de quarentena e reconfirmação são de Funding, `docs/design/10` §4.]

- **Identidade**: `generated`
- **Campos**:
  - `organizationId: id`
  - `destinationEvidenceId: id`
  - `requestedByUserId: id` (§), `approvedByUserId: id` (§, opcional)
  - `status: requested | approved`
  - `requestedAt: instant`, `approvedAt: instant` (opcional)
- **Estados**: `requested`, `approved`
- **Transições permitidas**:
  - (novo) → `requested` - por `RequestBankAccountChange`
  - `requested` → `approved` - por `ApproveBankAccountChange`
- **Invariantes de estado**: aprovador distinto do proponente; cada um age na própria requisição.

### Role e Permission

Não são agregados: catálogo fechado em código, derivado da matriz de `docs/design/95-personas-jornadas.md` ("Matriz de papéis e permissões"), referenciada e não reescrita. Cada papel mapeia para a lista de permissões que o `bff` exige por rota, no formato `<contexto>:<acesso>` do `bff` de referência do kernel (`apps/backend/bff/app/api/routes.go`), com `<acesso>` igual à operação em kebab-case: o `read`/`write` do kernel não separa "propõe" de "aprova" nem `RequestBankAccountChange` de `InviteUser`. Papéis desta versão: `operator`, `auditor` e `platform_ops`, este com as 24 permissões `partner:*`, em organização de plataforma (SPEC-8QH8VMQE). Papéis reservados para fatias seguintes: `approver` e `org_admin` (cliente), `partner_analyst` e `partner_compliance` (financiador), `platform_risk` (plataforma). Invariante: nenhum papel acumula proposição e aprovação do mesmo ato sensível.

Tabela congelada (aprovada em 2026-10-13), em ordem lexicográfica; `operator` tem 17 permissões e `auditor`, 14:

| Permissão | Ação da matriz | `operator` | `auditor` |
| --- | --- | --- | --- |
| `eligibility:request-eligibility-evaluation` | Simular | sim | não |
| `identity:find-bank-account-change` | Alterar dados bancários (acompanhar) | sim | não |
| `identity:find-bank-account-changes-by-organization` | Alterar dados bancários (acompanhar) | sim | não |
| `identity:request-bank-account-change` | Alterar dados bancários (propõe) | sim | não |
| `kyb:find-business-profile` | Ver dossiê KYB próprio | sim | sim |
| `kyb:find-disclosures-by-case` | Ver dossiê KYB próprio | sim | sim |
| `kyb:find-ownership-structure` | Ver dossiê KYB próprio | sim | sim |
| `kyb:find-verification-case` | Ver dossiê KYB próprio | sim | sim |
| `kyb:find-verification-cases-by-organization` | Ver dossiê KYB próprio | sim | sim |
| `ledger:access-evidence-bundle` | Emitir dossiê de auditoria | não | sim |
| `ledger:find-evidence-bundle` | Emitir dossiê de auditoria | não | sim |
| `ledger:request-evidence-bundle` | Emitir dossiê de auditoria (solicita) | não | sim |
| `marketplace:open-funding-request` | Criar solicitação | sim | não |
| `portfolio:find-availability-by-snapshot` | Visualizar portfólio | sim | sim |
| `portfolio:find-latest-published-snapshot` | Visualizar portfólio | sim | sim |
| `portfolio:find-portfolio` | Visualizar portfólio | sim | sim |
| `portfolio:find-snapshots-by-portfolio` | Visualizar portfólio | sim | sim |
| `portfolio:group-assets-by-snapshot` | Visualizar portfólio | sim | sim |
| `portfolio:list-assets-by-snapshot` | Visualizar portfólio | sim | sim |
| `portfolio:request-discovery` | Disparar descoberta | sim | não |

Regras: as permissões efetivas são a união dos papéis dos vínculos `active` do usuário no tenant, sem repetição e em ordem lexicográfica crescente; conta ausente ou `suspended` resulta em lista vazia. Fora da tabela, por decisão: `eligibility:create-eligible-lot` e `eligibility:reserve-asset`, cujo disparo na criação da solicitação está em aberto na spec do marketplace; e as leituras do próprio `identity` (`find-organization`, `find-memberships-by-organization` e demais), que a matriz não concede a nenhum dos dois papéis.

## Comandos (UPRs)

### RegisterOrganization

- **Agregado**: `Organization`; **cria**
- **Entrada**: `kinds: list<...>`, `jurisdiction`, `taxIdType`, `taxId`, `legalName`, `tradeName` (opcional); derivados (‡): `taxIdHash`, `taxIdMasked`
- **Pré-condições**:
  - `kinds` não vazio → `identity-organization/organization/kinds-empty`
  - `jurisdiction` com duas letras → `identity-organization/organization/jurisdiction-invalid`
  - `taxId` não vazio → `identity-organization/organization/tax-id-empty`
  - `legalName` não vazio → `identity-organization/organization/legal-name-empty`
  - `(taxIdHash, jurisdiction)` ainda não registrado no tenant (††) → `identity-organization/organization/duplicate-tax-id`
- **Efeitos sobre o estado**: `status ← active`; demais campos `← cmd`; `createdAt ← at`
- **Eventos emitidos**: `OrganizationRegistered`
- **Resposta**: `organizationId: id`

### AddEstablishment

- **Agregado**: `Establishment`; **cria**
- **Entrada**: `organizationId`, `taxIdType`, `taxId`, `tradeName` (opcional), `receiver`, `externalReferences`; derivados (‡): `taxIdHash`, `taxIdMasked`
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
- **Entrada**: `fullName`, `document`; derivados (‡): `documentHash`, `documentMasked`
- **Pré-condições**:
  - `fullName` não vazio → `identity-organization/person/full-name-empty`
  - `document` não vazio → `identity-organization/person/document-empty`
  - `documentHash` ainda não registrado no tenant (††) → `identity-organization/person/duplicate-document`
- **Efeitos sobre o estado**: `fullName`, `documentHash`, `documentMasked ← cmd`; `createdAt ← at`; o `document` não é guardado (‡‡)
- **Eventos emitidos**: nenhum
- **Resposta**: `personId: id`

### RegisterRepresentation

- **Agregado**: `Representation`; **cria**
- **Entrada**: `organizationId`, `personId`, `powerType`, `sourceType`, `sourceEvidenceId`, `scopeCurrency`, `scopeMaxAmountMinor`, `scopeActTypes` (alçada opcional), `validFrom`, `validUntil` (opcional), `proposedByUserId` (§)
- **Pré-condições**:
  - `sourceEvidenceId` não vazio (origem e evidência dos poderes, req. 4) → `identity-organization/representation/source-evidence-missing`
  - `validUntil`, quando informado, maior que `validFrom` → `identity-organization/representation/validity-invalid`
  - parte monetária da alçada com `scopeMaxAmountMinor` positivo e `scopeCurrency` informada, ou ausente por inteiro; `scopeActTypes` pode vir sozinho → `identity-organization/representation/scope-invalid`
  - organização existente e `active` (†) → `identity-organization/representation/organization-not-active`
  - pessoa existente (†) → `identity-organization/representation/person-not-found`
- **Efeitos sobre o estado**: `verificationStatus ← declared`; campos `← cmd`
- **Eventos emitidos**: nenhum (o fato verificado nasce em `VerifyRepresentation`, no Escopo fora)
- **Resposta**: `representationId: id`

### ProvisionUserAccount

[ABERTA: gatilho do provisionamento - JIT na borda no primeiro acesso ou evento do Keycloak; o IdP está decidido (ADR-016) e a SPEC-SBW7HT6Y só define o canal de eventos de sessão.]

- **Agregado**: `UserAccount`; **inicializa se ausente**
- **Entrada**: `subject`, `personId` (opcional)
- **Pré-condições**: `subject` não vazio → `identity-organization/user-account/subject-empty`
- **Efeitos sobre o estado**: quando ausente: `status ← active`; `provisionedAt ← at`; quando presente: nenhum efeito de negócio - aceita sem emitir, mas a versão técnica do agregado sobe, porque o kernel salva todo agregado decidido. Exige chave de idempotência como todo comando (ver Políticas transversais).
- **Eventos emitidos**: nenhum
- **Resposta**: `userId: id` (o `subject`)

### InviteUser

- **Agregado**: `Membership`; **cria**
- **Entrada**: `organizationId`, `inviteeEmail`, `role`, `invitedByUserId` (§), `invitationExpiresAt`
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
- **Entrada**: `userId`, `personId`, `channelConfirmed: bool`, `verificationLevel`
- **Pré-condições**:
  - `status = invited` → `identity-organization/membership/not-invited`
  - `invitationExpiresAt` maior que `at` → `identity-organization/membership/invitation-expired`
  - `channelConfirmed = true` (posse do canal) → `identity-organization/membership/channel-not-confirmed`
  - `verificationLevel` igual ou acima do mínimo do papel → `identity-organization/membership/verification-insufficient`
  - usuário existente e `active` (†) → `identity-organization/membership/user-not-active`
- **Efeitos sobre o estado**: `status ← active`; `grantedAt ← at`; `userId`, `personId`, `verificationLevel ← cmd`
- **Eventos emitidos**: `MembershipGranted`
- **Resposta**: `membershipId: id`

### RevokeMembership

- **Agregado**: `Membership`; exige existente
- **Entrada**: `reason`
- **Pré-condições**: `status = active` → `identity-organization/membership/not-active`
- **Efeitos sobre o estado**: `status ← revoked`; `revokedAt ← at`; `revocationReason ← cmd.reason`
- **Eventos emitidos**: `MembershipRevoked` [ABERTA: docs/design/92-schemas-eventos.md:§4 - ausente do catálogo; necessário ao histórico de permissões do Compliance Ledger, PRD 01 Dependências]
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
- **Entrada**: `organizationId`, `destinationEvidenceId`, `requestedByUserId` (§)
- **Pré-condições**:
  - `destinationEvidenceId` não vazio → `identity-organization/bank-account-change/destination-evidence-missing`
  - organização existente e `active` (†) → `identity-organization/bank-account-change/organization-not-active`
- **Efeitos sobre o estado**: `status ← requested`; `requestedAt ← at`; campos `← cmd`
- **Eventos emitidos**: `BankAccountChangeRequested`
- **Resposta**: `bankAccountChangeId: id`

### ApproveBankAccountChange

- **Agregado**: `BankAccountChange`; exige existente
- **Entrada**: `approvedByUserId` (§)
- **Pré-condições**:
  - `status = requested` → `identity-organization/bank-account-change/not-requested`
  - `approvedByUserId` distinto de `requestedByUserId` → `identity-organization/bank-account-change/same-actor-approval`
- **Efeitos sobre o estado**: `status ← approved`; `approvedAt ← at`; `approvedByUserId ← cmd`
- **Eventos emitidos**: `BankAccountChangeApproved`
- **Resposta**: `bankAccountChangeId: id`

## Eventos de domínio

Convenção: `EventName()` é `identity-organization.<agregado>.<evento>`. Campos `PII` entram como hash ou máscara, nunca o valor; `PII_SENSITIVE` nunca entra (`90` §1.4).

### OrganizationRegistered

- **Campos**: `organizationId: id ← Organization.id`; `kinds: list ← Organization.kinds`; `jurisdiction: string`; `taxIdType`; `taxIdHash: string`; `taxIdMasked: string`; `at: instant ← at`

### EstablishmentAdded

- **Campos**: `establishmentId: id`; `organizationId: id`; `taxIdType`; `taxIdHash: string`; `receiver: bool`; `at: instant ← at`

### MembershipGranted

- **Campos**: `membershipId: id`; `organizationId: id`; `userId: id`; `role`; `verificationLevel`; `at: instant ← at`

### MembershipRevoked

- **Campos**: `membershipId: id`; `organizationId: id`; `userId: id`; `role`; `reason: string ← Membership.revocationReason`; `at: instant ← at`

### UserSuspended

- **Campos**: `userId: id`; `reason: string`; `at: instant ← at`

### BankAccountChangeRequested

- **Campos**: `bankAccountChangeId: id`; `organizationId: id`; `destinationEvidenceId: id`; `requestedByUserId: id`; `at: instant ← at`

### BankAccountChangeApproved

- **Campos**: `bankAccountChangeId: id`; `organizationId: id`; `approvedByUserId: id`; `at: instant ← at`

## Consultas

Todas no tenant da requisição; nenhuma atravessa tenant nesta versão.

- `FindOrganization` - filtros: `organizationId`; cardinalidade: `one`
- `FindOrganizationByTaxId` - filtros: `taxIdType`, `taxId`, `jurisdiction` (o caso de uso deriva o `taxIdHash`, que depende do tipo, ‡); cardinalidade: `one`
- `FindEstablishmentsByOrganization` - filtros: `organizationId`; cardinalidade: `many`
- `FindPerson` - filtros: `personId`; cardinalidade: `one`
- `FindRepresentation` - filtros: `representationId`; cardinalidade: `one`
- `FindRepresentationsByOrganization` - filtros: `organizationId`, `validAt` (opcional); cardinalidade: `many` (consulta de vigência usada por Eligibility, ORG-REP-001)
- `FindMembershipsByUser` - filtros: `userId`; cardinalidade: `many`
- `FindMembershipsByOrganization` - filtros: `organizationId`; cardinalidade: `many`
- `FindEffectivePermissions` - filtros: `userId` (o `sub`); cardinalidade: `one`. Chamada pelo `bff` a cada requisição (ADR-016, condição 3). Devolve a união das permissões dos papéis dos vínculos `active` do usuário no tenant da requisição, ou lista vazia quando a conta não existe ou está `suspended`. Sem cache em nenhum ponto (CTX-16): revogação e suspensão valem na requisição seguinte ao commit. São duas leituras, porque o `Relation` do kernel filtra por uma coluna: primeiro `memberships` por `user_id`, depois `user_accounts`, deixando a suspensão como última checagem (fail-closed); a lista sai deduplicada e ordenada. As duas leituras não compartilham snapshot: a requisição iniciada depois do commit de revogação ou suspensão vê o efeito, e a requisição em voo durante o commit pode ver o estado anterior. Alçada e vigência da representação não entram: são regra de domínio das UPRs consumidoras. O histórico é do Compliance Ledger [ABERTA: SPEC-59BCT7T1 - reconstrução a partir de MembershipGranted, MembershipRevoked e UserSuspended].
- `FindBankAccountChange` - filtros: `bankAccountChangeId`; cardinalidade: `one`
- `FindBankAccountChangesByOrganization` - filtros: `organizationId`; cardinalidade: `many`

## Relações entre agregados

Sempre por identidade. A existência e o estado do agregado referenciado são verificados pelo caso de uso (†), exceto `UserAccount.personId` e `Membership.personId`: nesta versão são referências informadas pela borda e não verificadas, porque nenhum código de rejeição cobre pessoa ausente em `ProvisionUserAccount` e `GrantMembership`.

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

- **Publica** (integration events), em `apps/backend/identity/contract/proto/company/identity/event/v1/`, package `company.identity.event.v1`. Transporte Kafka pela outbox e pelo relay do kernel (ADR-015), em Protobuf dentro de CloudEvents, com `partitionkey` igual ao id do agregado (ADR-022, linha 6); o critério de canal de TRP-37 é declarado no `deploy/infra.json`.
  - `OrganizationRegistered` (`organization_registered.proto`) - consumidores no doc 92 §4: KYB, Ledger
  - `EstablishmentAdded` - Ledger (PRD 01, Dependências) [ABERTA: docs/design/92-schemas-eventos.md:§4 - ausente do catálogo]
  - `MembershipGranted` e `MembershipRevoked` - Ledger (PRD 01, Dependências) [ABERTA: docs/design/92-schemas-eventos.md:§4 - ausentes do catálogo]
  - `UserSuspended` - Ledger
  - `BankAccountChangeRequested` e `BankAccountChangeApproved` - eventos propostos no doc 92 §4; consumidores no desenho de Funding e Notificações
- **Consome**: nenhum.
- **Consultas gRPC servidas a outros contextos**: `FindEffectivePermissions` (`bff`, a cada requisição); `FindOrganization` (KYB resolve o identificador fiscal em claro, no tenant da organização); `FindRepresentationsByOrganization` e `FindOrganizationByTaxId` (Eligibility, Partner, Contracting). Quando o chamador opera em outro tenant - o financiador lendo o cedente -, a consulta depende de K4 [ABERTA: SPEC-QVDGV6J0 entregue no kernel `1.0.0-rc.6` (DEVS-100, ADR-068 do DMPF); o uso aqui depende da adoção da rc.6 (DEVS-136); ADR-013].

## Borda e persistência

- **Borda**: só gRPC, `company.identity.service.v1.IdentityService`, um RPC por comando e por consulta, atrás da cadeia de interceptors do kernel. O REST do PRD (`POST /organizations`, `POST /organizations/{id}/establishments`, `POST /organizations/{id}/memberships`, `POST /organizations/{id}/representations`) e o enriquecimento de permissões são do `bff` e ficam no Escopo fora.
- **Banco**: database e role `identity`, schema `public`, tabelas `organizations`, `establishments`, `persons`, `representations`, `user_accounts`, `memberships` e `bank_account_changes`, com `tenant_id` à frente da chave. Persistência híbrida (ADR-014); colunas tipadas só onde uma consulta filtra: `organizations(tax_id_hash, jurisdiction)`, com índice único `(tenant_id, tax_id_hash, jurisdiction)` de salvaguarda; `establishments(organization_id)` e `establishments(tax_id_hash)`, com índice único `(tenant_id, organization_id, tax_id_hash)`; `persons(document_hash)`, com índice único `(tenant_id, document_hash)`; `representations(organization_id, person_id)`; `memberships(organization_id)` e `memberships(user_id)`; `bank_account_changes(organization_id)`. O resto vai em `snapshot` `jsonb`.
- Diverge do design (ADR-022): isolamento por `tenant_id` injetado pelo `ExecutionContext` no `postgres.Table`, sem RLS como mecanismo primário (linha 2; ADR-013), e `tenantId` não é campo dos agregados; eventos em Protobuf dentro de CloudEvents, sem o envelope JSON e o `schemaUri` do doc 92 §1 (linha 6); `instant` é inteiro de nanossegundos no domínio (linha 9); cifra de campo pessoal pendente (linha 14); unicidade por tenant com pré-checagem (linha 15).

## Políticas transversais

- **Tenant**: vem só do token verificado (IDN-01, CTX-06); nesta versão, um tenant por organização, atribuído pelo backoffice ao criar a organização no Keycloak (ADR-013, ADR-016, SPEC-SBW7HT6Y). O gancho de autorização do kernel nega a operação sem tenant.
- **Idempotência**: todo comando exige chave de idempotência na metadata gRPC - o registro idempotente do kernel recusa a chamada sem ela (FND-04; ADR-056 do DMPF). Os comandos de criação (`RegisterOrganization`, `AddEstablishment`, `RegisterPerson`, `RegisterRepresentation`, `InviteUser`, `RequestBankAccountChange`) entram pelo `bff` por `POST` com a chave declarada (RST-02). Em `ProvisionUserAccount`, a identidade natural dá só a semântica de "inicializa se ausente": o replay com a mesma chave devolve a resposta registrada; a repetição com chave nova aceita sem evento e sobe a versão técnica; duas primeiras provisões concorrentes com chaves distintas podem terminar uma delas em conflito de versão retentável. O fingerprint inclui campos em claro, como o `document` de `RegisterPerson`, e o kernel guarda na inbox só o digest dele; por isso a composition root troca o digest padrão (SHA-256 sem chave, quebrável por força bruta) por HMAC-SHA256 com a chave secreta do `identity` e separação de domínio (‡‡).
- **Autorização**: por papel, na borda. O `bff` verifica o token, resolve as permissões efetivas por `FindEffectivePermissions` a cada requisição e confere a permissão da rota (ADR-016, condição 3). No contexto, o interceptor gRPC do kernel monta o `ExecutionContext` sem sujeito e sem permissões, e o gancho de autorização do bloco `application` exige só o tenant: sujeito e permissões não atravessam o salto (CTX-12) e o caso de uso não consulta a autoridade de identidade (IDN-10). Alçada e vigência da representação são regra de domínio das UPRs consumidoras (Consent, Contracting), alimentadas por eventos.
- **Revogação em menos de 60 s**: vínculo revogado e conta suspensa valem na requisição seguinte ao commit, pela resolução por requisição sem cache (CTX-16); sessão encerrada no IdP e usuário desabilitado, pela deny-list de `sub` e `sid` do `authn` (SPEC-STEJG347), consultada em toda requisição (ADR-016, condição 2). A lista com cache de 30 s do doc 01 §2.2 não se aplica (ADR-022, linha 8).
- **MFA**: OTP ou WebAuthn no Keycloak (ADR-016; SPEC-SBW7HT6Y). Reautenticação em ato crítico [ABERTA: o `authn` do kernel não lê `acr` nem `auth_time` - `ports.Identity` tem só sujeito, tenant e permissões -; o step-up exige extensão do kernel ainda sem spec].
- **Dados pessoais**: classificação do doc 90 §1.4. `PII` é mascarada em log e entra em evento só como hash ou máscara; `PII_SENSITIVE` não é persistido nesta versão (‡‡).
- **Auditoria**: pela trilha de auditoria do kernel (FND-08; `observability/audit`); o Compliance Ledger consome os eventos.

## Critérios de aceite

- [x] Os gates do workspace verdes para `identity` (`fmt-check`, `vet`, `build`, `lint`, `test-race`, `test-distributed` com Postgres, `dmpf-context-check.sh`, `conformance --root .`).
- [x] Builders de teste sintéticos (ADR-021): CPF e CNPJ válidos, inclusive o CNPJ alfanumérico (R3), sem dado real.
- [x] Cenários de aceite por comando e consulta:

```text
DADO nenhuma organização com aquele identificador fiscal e jurisdição no tenant
QUANDO RegisterOrganization chega com kinds [client], um CNPJ alfanumérico, legalName e jurisdiction BR
ENTÃO a organização nasce active com createdAt = at e OrganizationRegistered vai à outbox com hash e máscara, sem o taxId

DADO uma organização active
QUANDO AddEstablishment chega com receiver true e um taxId novo
ENTÃO o estabelecimento é criado e EstablishmentAdded é emitido

DADO uma pessoa ainda desconhecida no tenant
QUANDO RegisterPerson chega com fullName e document
ENTÃO a pessoa é criada com documentHash e documentMasked, o document não aparece no snapshot e nenhum evento é emitido

DADO uma pessoa e uma organização active
QUANDO RegisterRepresentation chega com fonte, evidência e vigência válidas
ENTÃO a representação nasce declared e nenhum evento é emitido

DADO um subject ainda desconhecido no tenant
QUANDO ProvisionUserAccount chega com uma chave de idempotência
ENTÃO a conta nasce active; o replay com a mesma chave devolve a mesma resposta; a repetição com o mesmo subject e chave nova aceita sem evento e sem alterar o estado de negócio

DADO duas primeiras ProvisionUserAccount concorrentes para o mesmo subject, com chaves de idempotência distintas
QUANDO as duas chegam ao mesmo tempo
ENTÃO existe exatamente uma conta active; cada requisição é aceita ou sai como conflito de versão retentável

DADO uma organização active com kinds [client]
QUANDO InviteUser chega com e-mail, papel operator e expiração futura
ENTÃO o vínculo nasce invited

DADO um vínculo operator invited, canal confirmado e verificação personal_data_checked
QUANDO GrantMembership chega
ENTÃO o vínculo vai a active com grantedAt = at e MembershipGranted é emitido

DADO um usuário com vínculo operator active
QUANDO FindEffectivePermissions consulta pelo seu sub
ENTÃO devolve exatamente as 17 permissões de operator da tabela de Role e Permission, em ordem lexicográfica, fora de qualquer UoW

DADO um usuário com vínculo auditor active
QUANDO FindEffectivePermissions consulta pelo seu sub
ENTÃO devolve exatamente as 14 permissões de auditor da tabela de Role e Permission, em ordem lexicográfica

DADO um usuário com vínculos operator e auditor active no tenant
QUANDO FindEffectivePermissions consulta pelo seu sub
ENTÃO devolve a união das duas listas, 20 permissões sem repetição, em ordem lexicográfica

DADO um sub sem conta no tenant
QUANDO FindEffectivePermissions consulta por ele
ENTÃO devolve lista vazia

DADO um vínculo active
QUANDO RevokeMembership chega
ENTÃO o vínculo vai a revoked, MembershipRevoked é emitido e a consulta seguinte de FindEffectivePermissions não traz as permissões do papel

DADO uma conta active com vínculo active
QUANDO SuspendUser chega com reason
ENTÃO a conta vai a suspended, UserSuspended é emitido e FindEffectivePermissions devolve lista vazia

DADO uma organização active
QUANDO RequestBankAccountChange chega
ENTÃO a mudança nasce requested e BankAccountChangeRequested é emitido

DADO uma mudança requested pelo usuário U1
QUANDO ApproveBankAccountChange chega pelo usuário U2
ENTÃO a mudança vai a approved e BankAccountChangeApproved é emitido

DADO duas requisições RegisterOrganization no tenant com o mesmo taxId e jurisdiction, chaves de idempotência distintas e uma barreira que sobrepõe as transações
QUANDO as duas chegam ao mesmo tempo
ENTÃO exatamente uma é aceita, exatamente uma linha fica em organizations e exatamente um OrganizationRegistered vai à outbox; a outra sai como erro técnico

DADO duas requisições AddEstablishment com o mesmo organizationId e taxId, chaves de idempotência distintas e uma barreira que sobrepõe as transações
QUANDO as duas chegam ao mesmo tempo
ENTÃO exatamente uma é aceita, exatamente uma linha fica em establishments e exatamente um EstablishmentAdded vai à outbox; a outra sai como erro técnico

DADO duas requisições RegisterPerson no tenant com o mesmo document, chaves de idempotência distintas e uma barreira que sobrepõe as transações
QUANDO as duas chegam ao mesmo tempo
ENTÃO exatamente uma é aceita e exatamente uma linha fica em persons; a outra sai como erro técnico

DADO duas requisições RegisterOrganization concorrentes com o mesmo taxId e jurisdiction, uma no tenant A e outra no tenant B
QUANDO as duas chegam ao mesmo tempo
ENTÃO as duas são aceitas, e cada tenant fica com exatamente uma linha em organizations e um OrganizationRegistered

DADO uma organização registrada no tenant A
QUANDO FindOrganizationByTaxId chega do tenant B com o mesmo taxId
ENTÃO responde como inexistente
```

- [x] Cenários de rejeição, um por código (nada é escrito em nenhum deles):

```text
DADO qualquer entrada QUANDO RegisterOrganization chega com kinds vazio ENTÃO rejeita com identity-organization/organization/kinds-empty
DADO qualquer entrada QUANDO RegisterOrganization chega com jurisdiction de tamanho diferente de dois ENTÃO rejeita com identity-organization/organization/jurisdiction-invalid
DADO qualquer entrada QUANDO RegisterOrganization chega com taxId vazio ENTÃO rejeita com identity-organization/organization/tax-id-empty
DADO qualquer entrada QUANDO RegisterOrganization chega com legalName vazio ENTÃO rejeita com identity-organization/organization/legal-name-empty
DADO uma organização com o mesmo taxId e jurisdiction no tenant QUANDO RegisterOrganization chega ENTÃO rejeita com identity-organization/organization/duplicate-tax-id
DADO uma organização suspended QUANDO AddEstablishment chega ENTÃO rejeita com identity-organization/establishment/organization-not-active
DADO qualquer entrada QUANDO AddEstablishment chega com taxId vazio ENTÃO rejeita com identity-organization/establishment/tax-id-empty
DADO um estabelecimento com o mesmo taxId na organização QUANDO AddEstablishment chega ENTÃO rejeita com identity-organization/establishment/duplicate-tax-id
DADO qualquer entrada QUANDO RegisterPerson chega com fullName vazio ENTÃO rejeita com identity-organization/person/full-name-empty
DADO qualquer entrada QUANDO RegisterPerson chega com document vazio ENTÃO rejeita com identity-organization/person/document-empty
DADO uma pessoa com o mesmo document no tenant QUANDO RegisterPerson chega ENTÃO rejeita com identity-organization/person/duplicate-document
DADO qualquer entrada QUANDO RegisterRepresentation chega sem sourceEvidenceId ENTÃO rejeita com identity-organization/representation/source-evidence-missing
DADO qualquer entrada QUANDO RegisterRepresentation chega com validUntil menor ou igual a validFrom ENTÃO rejeita com identity-organization/representation/validity-invalid
DADO qualquer entrada QUANDO RegisterRepresentation chega com scopeMaxAmountMinor sem scopeCurrency ENTÃO rejeita com identity-organization/representation/scope-invalid
DADO uma organização offboarded QUANDO RegisterRepresentation chega ENTÃO rejeita com identity-organization/representation/organization-not-active
DADO um personId inexistente QUANDO RegisterRepresentation chega ENTÃO rejeita com identity-organization/representation/person-not-found
DADO qualquer entrada QUANDO ProvisionUserAccount chega com subject vazio ENTÃO rejeita com identity-organization/user-account/subject-empty
DADO uma conta suspended QUANDO SuspendUser chega ENTÃO rejeita com identity-organization/user-account/not-active
DADO uma conta active QUANDO SuspendUser chega com reason vazio ENTÃO rejeita com identity-organization/user-account/reason-empty
DADO qualquer entrada QUANDO InviteUser chega com e-mail vazio ENTÃO rejeita com identity-organization/membership/invitee-email-empty
DADO qualquer entrada QUANDO InviteUser chega com expiração anterior a at ENTÃO rejeita com identity-organization/membership/invitation-expiry-invalid
DADO uma organização suspended QUANDO InviteUser chega ENTÃO rejeita com identity-organization/membership/organization-not-active
DADO uma organização com kinds [funder] QUANDO InviteUser chega com papel operator ENTÃO rejeita com identity-organization/membership/role-kind-mismatch
DADO um vínculo active QUANDO GrantMembership chega ENTÃO rejeita com identity-organization/membership/not-invited
DADO um vínculo invited expirado QUANDO GrantMembership chega ENTÃO rejeita com identity-organization/membership/invitation-expired
DADO um vínculo invited QUANDO GrantMembership chega com channelConfirmed false ENTÃO rejeita com identity-organization/membership/channel-not-confirmed
DADO um vínculo operator invited QUANDO GrantMembership chega com verificationLevel channel_ownership ENTÃO rejeita com identity-organization/membership/verification-insufficient
DADO uma conta suspended QUANDO GrantMembership chega ENTÃO rejeita com identity-organization/membership/user-not-active
DADO um vínculo invited QUANDO RevokeMembership chega ENTÃO rejeita com identity-organization/membership/not-active
DADO qualquer entrada QUANDO RequestBankAccountChange chega sem destinationEvidenceId ENTÃO rejeita com identity-organization/bank-account-change/destination-evidence-missing
DADO uma organização suspended QUANDO RequestBankAccountChange chega ENTÃO rejeita com identity-organization/bank-account-change/organization-not-active
DADO uma mudança approved QUANDO ApproveBankAccountChange chega ENTÃO rejeita com identity-organization/bank-account-change/not-requested
DADO uma mudança requested por U1 QUANDO ApproveBankAccountChange chega por U1 ENTÃO rejeita com identity-organization/bank-account-change/same-actor-approval
```

## Escopo fora

- **REST do `bff`**: Client API, rotas do PRD 01 e o enriquecimento de permissões - um `Authenticator` que verifica o token e chama `FindEffectivePermissions` - ficam no `bff` (referência de superfícies em `docs/design/91-contratos-api.md` §1); este contexto serve só gRPC.
- **`VerifyRepresentation` e `RepresentationVerified`**: aguardam a decisão do emissor e do dono da prova de poderes (ver `Representation`). Consent e Contracting consomem o evento e dependem desta fatia.
- **`RevokeRepresentation` e `RepresentationRevoked`**: a segunda aprovação vira passo próprio (`ApproveRepresentationRevocation`), porque dois atores na mesma entrada violariam CTX-06 [ABERTA: docs/design/01-design-identity-organization.md:§5 - confirmar o passo com produto].
- **Papéis `approver` e `org_admin`**: exigem o nível `document_and_liveness`, cujo provedor não foi escolhido (C5b), e a concessão com segunda aprovação em passo próprio. **Papéis de financiador e `platform_risk`**: nível de verificação não definido; `platform_ops` usa `personal_data_checked`, provisório (SPEC-8QH8VMQE) [ABERTA: docs/design/01-design-identity-organization.md:§3]; os de plataforma dependem também de K4.
- **Primeiro vínculo de cada organização** [ABERTA: sem `org_admin` nesta versão, nenhum fluxo de cliente cria o primeiro vínculo; o caminho de plataforma (backoffice) depende de K4 e da fatia de papéis de aprovação].
- **Autocadastro e unicidade de identificador fiscal entre tenants**: registro global no tenant de plataforma, por operação de plataforma; dependem de SPEC-QVDGV6J0 (K4).
- **Consultas vindas de outros tenants e acesso de papéis de plataforma**: SPEC-QVDGV6J0 (K4), entregue na `1.0.0-rc.6` (ADR-068 do DMPF); o uso aqui depende da adoção da rc.6 (DEVS-136).
- **Documento em claro e cifra em envelope de `PII`**: SPEC-CC5SHV1K. **Violação de unicidade como rejeição**: SPEC-7SRQ3RPK.
- **Provedor de identidade, MFA, sessões e verificação documental ou de vivacidade**: Keycloak (ADR-016; SPEC-SBW7HT6Y; SPEC-STEJG347) e C5b; o contexto guarda o `subject` e o nível de verificação atingido, não executa a autenticação.
- **Configuração por organização da dupla aprovação** dos atos configuráveis (seleção de oferta, assinatura, revogação de consentimento, concessão de papel de aprovação) e o regime de aprovação única assistida: aguardam `01:I10` (validação jurídica).
- **Grupo econômico** (`01:I11`): cada filial com identificador fiscal próprio é uma `Organization` distinta, sem hierarquia entre elas.
- **Comandos de suspensão e descontinuação de organização** e de alteração de papel: não definidos no PRD.
- **Verificação cadastral aprofundada**: KYB/KYC; **mandatos transacionais**: Consent & Mandates.
- **Histórico de permissões e reconstrução temporal**: Compliance Ledger.
- **Dado bancário, validação de titularidade, quarentena e reconfirmação de conta**: Funding (`docs/design/10` §4).
- **Fases posteriores**: o sacado como ator (`docs/design/95`) e o pagador de Billing dependem de decisões de produto.
