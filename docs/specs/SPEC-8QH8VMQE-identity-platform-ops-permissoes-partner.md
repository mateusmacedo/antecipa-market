---
id: SPEC-8QH8VMQE
slug: identity-platform-ops-permissoes-partner
title: Identity — papel platform_ops e permissões partner:* da administração de parceiros
stage: done
priority: P1
depends_on: [SPEC-7KW0HHF6, SPEC-9TEQ9EV9]
ticket_url: https://linear.app/mmda/issue/DEVS-126/identity-permissoes-partner-e-linha-de-administracao-de-parceiros-na
subtask_urls: []
created: 2026-10-21
---

# SPEC-8QH8VMQE: Identity — papel platform_ops e permissões partner:* da administração de parceiros

## Resumo

O `identity-organization` passa a ter o primeiro papel de plataforma, `platform_ops`, com as 24 permissões `partner:<operação-kebab>` dos 14 comandos e das 10 consultas do `PartnerService`. A matriz de `docs/design/95-personas-jornadas.md` ganha a linha "Administrar parceiros". Como operador da plataforma, quero que o meu vínculo com a organização de plataforma me dê acesso às rotas de administração de parceiros do `bff`, para configurar parceiros, contratos, capacidades, produtos, integração e política operacional sem acesso direto ao gRPC.

## Contexto

- **Problema**: a SPEC-9TEQ9EV9:483 confere a permissão `partner:<operação-kebab>` no `bff` e deixa as permissões e a linha da matriz 95 no Escopo fora. O catálogo do identity (`apps/backend/identity/domain/permissions.go:9-45`) só tem `operator` e `auditor`, ambos de organização `client` (`domain/role.go:6-7`, `requiredKind`). A configuração de parceiros vive no tenant único da plataforma (SPEC-9TEQ9EV9:415 e :481), então nenhum papel existente pode administrá-la. Sem esta spec, as 24 rotas da DEVS-133 respondem 403 para todo portador: o `ValidateEdge` do kernel (`dmpf/libs/backend/go/http/route.go:113`) confere só se a rota declara `Permission`, não se o identity a concede.
- **Impacto**: destrava a DEVS-133 (rotas de administração do partner no `bff`). O papel `platform_ops`, reservado na SPEC-7KW0HHF6:204, sai da reserva só com as permissões do partner. As demais linhas da matriz que marcam `PLATFORM_OPS` (descoberta, conciliação, reenvio de comando externo, dossiê) ficam para as specs dos contextos donos.
- **Inspiração**: o próprio catálogo de `operator` e `auditor` (SPEC-7KW0HHF6, seção "Role e Permission"): catálogo fechado em código, formato `<contexto>:<acesso>` com `<acesso>` igual à operação em kebab-case, tabela congelada por teste (`domain/permissions_test.go:52`).
- **Links relevantes**:
  - SPEC-7KW0HHF6 — dona do catálogo de papéis e permissões; `platform_ops` e `platform_risk` reservados (:204); nível de verificação dos papéis de plataforma em aberto (:593)
  - SPEC-9TEQ9EV9 — os 24 RPCs do `PartnerService` (`apps/backend/partner/contract/proto/company/partner/service/v1/partner_service.proto:6-29`) e a autorização no `bff` (:483)
  - SPEC-7KH5JJMV — backoffice; usa `PLATFORM_OPS` e `PLATFORM_RISK` e registra a lacuna do papel "supervisor" (:388)
  - `docs/design/95-personas-jornadas.md` — "Matriz de papéis e permissões" e invariante derivada de segregação
  - `docs/design/01-design-identity-organization.md` §3 — nível de verificação por papel, definido só para os papéis de cliente
  - DEVS-133 — consumidora destas permissões

<constraints>
- [P0] O catálogo DEVE continuar fechado em código: `operator` mantém exatamente as 17 permissões e `auditor` as 14 de hoje, na mesma ordem.
- [P0] `platform_ops` DEVE exigir organização de tipo `platform` no `InviteUser`; organização sem `platform` é recusada com `CodeMembershipRoleKindMismatch`, como hoje para os papéis de cliente.
- [P0] As permissões DEVEM seguir `partner:<operação-kebab>`, com a operação igual ao nome do RPC do `PartnerService` em kebab-case, sem `read`/`write` genérico.
- [P0] O novo valor do enum `MembershipRole` DEVE ser aditivo (`MEMBERSHIP_ROLE_PLATFORM_OPS = 3`) nos dois protos; nenhum valor existente muda de número ou nome, e `buf breaking` passa.
- [P1] Nenhum outro papel de plataforma (`platform_risk`) nem de cliente ou financiador (`approver`, `org_admin`, `partner_analyst`, `partner_compliance`) entra no catálogo.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Papel `platform_ops` no domínio**: `RolePlatformOps Role = "platform_ops"` em `domain/role.go`.
  - `requiredKind[RolePlatformOps] = KindPlatform`.
  - `minimumVerification[RolePlatformOps] = VerificationPersonalDataChecked` — decisão provisória [ABERTA: docs/design/01-design-identity-organization.md:§3 — nível dos papéis de plataforma; revisar quando o C5b escolher o provedor de `document_and_liveness`].
- [ ] **[P0] As 24 permissões `partner:*`** em `rolePermissions[RolePlatformOps]`, em ordem lexicográfica:
  - Comandos (14): `partner:activate-agreement`, `partner:change-partner-stage`, `partner:change-rollout`, `partner:declare-capability`, `partner:define-operational-policy`, `partner:define-product`, `partner:enable-capability`, `partner:raise-certificate-expiry-alert`, `partner:record-homologation`, `partner:register-agreement`, `partner:register-integration-profile`, `partner:register-partner`, `partner:retire-product`, `partner:suspend-capability`.
  - Consultas (10): `partner:find-agreements-by-partner`, `partner:find-capabilities-by-partner`, `partner:find-certificates-expiring`, `partner:find-effective-capability`, `partner:find-eligible-partners`, `partner:find-integration-profile`, `partner:find-operational-policy`, `partner:find-partner`, `partner:find-partners-by-organization`, `partner:find-products-by-partner`.
  - `FindEffectivePermissions` de um vínculo `platform_ops` devolve exatamente as 24, fora de qualquer UoW, como para os outros papéis.
- [ ] **[P0] Contrato gRPC e de evento**: `MEMBERSHIP_ROLE_PLATFORM_OPS = 3` em `identity_service.proto` (enum `MembershipRole`, :455) e em `event/v1/types.proto` (:21); código Go regenerado pelo target de geração do contrato.
- [ ] **[P0] Tradução nas bordas do contexto**:
  - `app/rpc/service.go`: `rolesOnTheWire` aceita `MEMBERSHIP_ROLE_PLATFORM_OPS`; a mensagem de erro do `InviteUser` (:518) passa a listar `operator, auditor or platform_ops`.
  - `provider/mapper.go` (`roleOf`, :137): `RolePlatformOps` → `eventv1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS` nos eventos `MembershipGranted` e `MembershipRevoked`, os dois que carregam o papel.
- [ ] **[P0] OpenAPI do identity**: `MembershipRole.enum` (`contract/openapi/v1/openapi.yaml:925`) passa a `[operator, auditor, platform_ops]`.
- [ ] **[P0] `bff`**: `views_identity.go` (:56) traduz `MEMBERSHIP_ROLE_PLATFORM_OPS` para `"platform_ops"` nas respostas de vínculo; o corpo do `InviteUser` aceita `platform_ops` pelo OpenAPI servido.
- [ ] **[P1] Matriz 95**: nova linha "Administrar parceiros (cadastro, contrato, capacidade, produto, integração e política)" com `PLATFORM_OPS = sim`, `PLATFORM_RISK = leitura` e `não` nas demais colunas; uma nota sob a matriz registra que, nesta versão, só `PLATFORM_OPS` está no catálogo do identity.
- [ ] **[P1] SPEC-7KW0HHF6 e README do identity**: a seção "Role e Permission" (:204) passa a listar `platform_ops` entre os papéis desta versão, apontando para esta spec; `apps/backend/identity/README.md:58` cita os três papéis.

### Não-funcionais

- [ ] Compatibilidade: consumidor de `MembershipGranted`/`MembershipRevoked` que ainda não conhece o valor 3 continua decodificando o evento (enum aberto do proto3); nenhum golden existente muda.
- [ ] Segurança: o papel só é concedido com organização `platform` e verificação `personal_data_checked` ou superior; `channel_ownership` é recusado com `CodeMembershipVerificationInsufficient`.
- [ ] Performance: `EffectivePermissions` continua O(n) sobre os papéis do vínculo; o catálogo cresce de 31 para 55 entradas, sem alocação extra além do `Clone`.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Domínio (`identity/domain`) | [x] | papel, tipo de organização exigido, nível mínimo e as 24 permissões |
| Aplicação (`identity/application`) | [ ] | sem mudança de código; testes de `InviteUser` e `FindEffectivePermissions` cobrem o papel novo |
| Contrato (`identity/contract`) | [x] | valor 3 no enum dos dois protos, código gerado e OpenAPI |
| Bordas do contexto (`identity/app/rpc`, `identity/provider`) | [x] | tradução do enum no RPC e no mapper de eventos |
| `bff` (`bff/app/api`) | [x] | tradução do papel nas views de vínculo |
| Documentação (`docs/design/95`, SPEC-7KW0HHF6, README do identity) | [x] | linha da matriz e lista de papéis |

## Localização de código

```text
apps/backend/identity/
  domain/role.go, domain/permissions.go          — papel e catálogo
  domain/permissions_test.go, membership_test.go  — tabela congelada, tipo e nível
  contract/proto/company/identity/service/v1/identity_service.proto
  contract/proto/company/identity/event/v1/types.proto
  contract/gen/go/...                             — regenerado, nunca editado à mão
  contract/openapi/v1/openapi.yaml
  app/rpc/service.go, app/rpc/service_test.go
  provider/mapper.go, provider/mapper_test.go
  application/invite_user_test.go, application/find_effective_permissions_test.go
  README.md
apps/backend/bff/app/api/
  views_identity.go, handlers_identity_commands_memberships_test.go
docs/design/95-personas-jornadas.md
docs/specs/SPEC-7KW0HHF6-identity-organization.md
```

## Design

### Fluxo principal

1. Um portador com vínculo ativo `platform_ops` na organização de plataforma chama uma rota `partner` do `bff`.
2. O `bff` chama `FindEffectivePermissions` do identity, que devolve a união ordenada dos papéis do vínculo, com as 24 `partner:*`.
3. A rota confere `partner:<operação-kebab>` na lista e encaminha ao `PartnerService` (DEVS-133); sem a permissão, 403 sem chamar o RPC.

### Pseudocódigo

```text
requiredKind[platform_ops]        = platform
minimumVerification[platform_ops] = personal_data_checked   // provisório
rolePermissions[platform_ops]     = sort(kebab(rpc) for rpc in PartnerService) prefixados por "partner:"
```

## Decisões técnicas

- **Papel `platform_ops` em vez de papel dedicado**: reusa a coluna `PLATFORM_OPS` da matriz 95 e a reserva da SPEC-7KW0HHF6:204. Alternativa descartada: `partner_admin`, que criaria uma nona coluna sem origem nos PRDs.
- **24 permissões num só papel**: é a fatia mínima que destrava a DEVS-133. Alternativa descartada: `platform_risk` com `suspend-capability`, que traria um segundo papel de plataforma sem consumidor nesta versão.
- **`personal_data_checked` provisório**: não depende de provedor novo. Alternativas descartadas: `channel_ownership`, fraco para quem configura capacidades; `document_and_liveness`, que deixaria o papel impossível de conceder até o C5b.
- **Valor de enum aditivo**: `3` no fim do enum, sem renumerar, mantém a compatibilidade de fio dos eventos já publicados.

## Regras relacionadas

- `docs/design/95-personas-jornadas.md` — invariante derivada: nenhum papel acumula proposição e aprovação do mesmo ato sensível. `platform_ops` registra e ativa contrato (`register-agreement` e `activate-agreement`), e registra homologação e habilita capacidade (`record-homologation` e `enable-capability`); o doc 13 não classifica esses pares como proposição e aprovação [ABERTA: docs/design/13-design-partner-management.md — confirmar com produto se algum par exige segunda pessoa].
- SPEC-7KH5JJMV:388 — o papel "supervisor" do backoffice (doc 17:105, suspender capacidade) continua sem coluna na matriz.

## Verificação e testes

### Critérios de aceite

- [ ] `RolePlatformOps.Permissions()` devolve exatamente as 24 permissões listadas, em ordem lexicográfica.
- [ ] `operator` (17) e `auditor` (14) continuam idênticos; `EffectivePermissions(operator, platform_ops)` devolve 41 permissões sem repetição.
- [ ] `TestARoleOutsideTheCatalogueHasNoPermission` deixa de listar `platform_ops` e passa a listar `platform_risk`.
- [ ] `InviteUser` com `platform_ops` em organização sem `platform` é recusado com `CodeMembershipRoleKindMismatch`; em organização `platform`, aceito.
- [ ] `GrantMembership` de `platform_ops` com `channel_ownership` é recusado com `CodeMembershipVerificationInsufficient`; com `personal_data_checked`, aceito.
- [ ] O RPC `InviteUser` aceita `MEMBERSHIP_ROLE_PLATFORM_OPS`; o mapper publica `MEMBERSHIP_ROLE_PLATFORM_OPS` em `MembershipGranted`.
- [ ] O `bff` responde `"role": "platform_ops"` para um vínculo desse papel; o teste de contrato contra o OpenAPI passa.
- [ ] A matriz 95 tem a linha "Administrar parceiros" com as 8 colunas preenchidas.
- [ ] `buf lint` e `buf breaking` verdes no contrato do identity; `pnpm nx affected -t lint,test,build` (incluindo `test-race`, `vet` e `fmt-check` dos projetos Go) verde; gates do DMPF conformes.

### Cenários de teste

```text
DADO um vínculo ativo platform_ops na organização de plataforma
QUANDO o bff chama FindEffectivePermissions
ENTÃO recebe exatamente as 24 permissões partner:* em ordem lexicográfica

DADO um usuário com vínculos operator e platform_ops
QUANDO EffectivePermissions(operator, platform_ops) é calculado
ENTÃO o resultado tem 41 permissões, ordenadas e sem repetição

DADO uma organização apenas de tipo client
QUANDO InviteUser chega com role platform_ops
ENTÃO o comando é recusado com CodeMembershipRoleKindMismatch e nenhum evento é emitido

DADO um convite platform_ops válido
QUANDO GrantMembership chega com verification_level channel_ownership
ENTÃO o comando é recusado com CodeMembershipVerificationInsufficient
```

<critical_constraints>
- [P0] O catálogo DEVE continuar fechado em código: `operator` mantém exatamente as 17 permissões e `auditor` as 14 de hoje, na mesma ordem.
- [P0] `platform_ops` DEVE exigir organização de tipo `platform` no `InviteUser`; organização sem `platform` é recusada com `CodeMembershipRoleKindMismatch`, como hoje para os papéis de cliente.
- [P0] As permissões DEVEM seguir `partner:<operação-kebab>`, com a operação igual ao nome do RPC do `PartnerService` em kebab-case, sem `read`/`write` genérico.
- [P0] O novo valor do enum `MembershipRole` DEVE ser aditivo (`MEMBERSHIP_ROLE_PLATFORM_OPS = 3`) nos dois protos; nenhum valor existente muda de número ou nome, e `buf breaking` passa.
- [P1] Nenhum outro papel de plataforma (`platform_risk`) nem de cliente ou financiador (`approver`, `org_admin`, `partner_analyst`, `partner_compliance`) entra no catálogo.
</critical_constraints>

## Escopo fora

- **Rotas REST de administração do partner no `bff`**: DEVS-133, que consome estas permissões.
- **`platform_risk` e as demais linhas de `PLATFORM_OPS` na matriz** (descoberta, conciliação, reenvio de comando externo, dossiê): cada contexto dono acrescenta as permissões na própria spec.
- **Primeiro vínculo `platform_ops`**: sem `org_admin`, nenhuma rota do `bff` convida (`identity:invite-user` não está em papel algum); o bootstrap do vínculo de plataforma segue no Escopo fora da SPEC-7KW0HHF6:594 (backoffice e K4).
- **MFA e sessão curta para papéis de plataforma** (C7.1, doc 17): dependem do step-up do kernel, sem spec (SPEC-7KW0HHF6:454).
- **Partner API do financiador** (`partner_analyst`, `partner_compliance`): doc 91 §4, sem ADR no kernel.
- **Leitura entre tenants**: K4 (DEVS-136); nesta versão a configuração de parceiros vive no tenant único da plataforma.
