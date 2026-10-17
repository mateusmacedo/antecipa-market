# ADR-016: Adotar Keycloak 26.x autogerenciado como provedor de identidade OIDC

## Status

Aceito — 2026-10-12, com as condições da Decisão.
Implementa SPEC-GXGASP24 (DEVS-76).

> **Addendum (2026-10-17)** — o kernel `1.0.0-rc.5` entregou a SPEC-SBW7HT6Y
> (ADR-064 do kernel) e a SPEC-STEJG347 (ADR-065 do kernel), e a condição 1
> mudou de forma. Não há mapper que achate a organização em `tenant_id`: o
> realm usa o mapper nativo de organização, e o `authn` lê o tenant por
> `OIDC_TENANT_CLAIM=organization.*.tenant_id`, com o client `dmpf-bff` como
> audiência. O curinga casa exatamente uma organização; com nenhuma ou com mais
> de uma, o caminho fica ausente (IDN-20). Por isso, o usuário vinculado a
> várias organizações escolhe a ativa no pedido do token, pelo scope de
> organização, e não por um mapper. A condição 2 vale como está, com a
> deny-list no Redis alimentada pelo papel `session-events` do `bff`.

## Contexto

A decisão D5 vem de `docs/design/00-arquitetura-tecnica.md` §9, que deixa em
aberto o provedor OIDC externo. `docs/design/01-design-identity-organization.md`
pede código de autorização com PKCE, MFA obrigatório para `APPROVER`,
`ORG_ADMIN` e papéis de plataforma, sessão de 30 min para aprovação e 8 h para
consulta, step-up em ato crítico, federação SSO por organização e revogação
efetiva em menos de 60 s (mecanismo não decidido, lacuna N4). O doc 00 §4.2
separa tenant de organização, e um usuário (o contador) pode ter vínculo com
várias organizações. Parceiros autenticam por mTLS, não por OIDC.

O kernel DMPF condiciona a escolha (classe **condicionada**):

- O verificador `libs/backend/go/authn` exige issuer único, audiência e claim de
  tenant, lida como string; os `Defaults()` assumem um access token de Keycloak.
- O tenant e o sujeito vêm só do token verificado (IDN-01, CTX-06); as
  permissões chegam ao `application service` já resolvidas (IDN-10); decisão
  derivada do contexto, como permissões efetivas, não sobrevive ao contexto
  (CTX-16).
- A revogação de credencial é delegada à plataforma
  (`docs/dmpf/contexto-erros-seguranca.md`, modelo de ameaça).

A nuvem-alvo é a AWS, com o floci como emulador (ADR-019). O Cognito do floci
não tem endpoint de autorização nem PKCE, e o issuer emulado diverge do da AWS.

## Decisão

Keycloak 26.x autogerenciado, com **realm único** (issuer único do `authn`),
Organizations para o vínculo B2B, identity provider por organização para SSO,
código de autorização com PKCE, MFA por OTP ou WebAuthn e step-up por nível de
autenticação. Na AWS, roda como container no orquestrador da plataforma; em dev
e hmg, como Keycloak de referência na infra do kernel.

A decisão vale com três condições:

1. **Claim de tenant**: o Keycloak emite `organization` como objeto. Um mapper
   achata a organização ativa em `tenant_id` string, com escolha do tenant ativo
   para usuário em várias organizações. O mapper e o Keycloak de referência na
   infra do DMPF são a SPEC-SBW7HT6Y (DEVS-97), no repositório do kernel.
2. **Revogação em menos de 60 s**: não vem do TTL de token, em nenhum IdP. Uma
   deny-list por `sub` e `sid`, alimentada por eventos de sessão do IdP e
   consultada no provider do `authn`, é a SPEC-STEJG347 (DEVS-101), no
   repositório do kernel; a fonte dos eventos sai da SPEC-SBW7HT6Y. A consulta
   vale para toda requisição, sem cache de ausência (CTX-16), o que supera a
   lista com cache de 30 s consultada só em operações sensíveis do
   `docs/design/01-design-identity-organization.md` §2.2 e da SPEC-7KW0HHF6.
3. **Origem das permissões efetivas**: IdP (claims) ou enriquecimento no `bff`,
   respeitando IDN-10 e CTX-16. É decisão do contexto Identity & Organization,
   registrada na SPEC-7KW0HHF6.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Amazon Cognito | Sem modelo de organização; tenant só por Lambda de pré-token; no floci, sem código de autorização nem PKCE. O TTL mínimo de 5 min só aumentaria a deny-list |
| Microsoft Entra External ID | Sem organização; fora da nuvem-alvo. O TTL mínimo de 10 min só aumentaria a deny-list |
| Auth0/Okta | Lock-in alto, teto de conexões SSO fora do plano Enterprise e dados no Brasil só em Private Cloud |
| Zitadel Cloud | Sem região no Brasil e vínculo de usuário a várias organizações não verificado |
| WorkOS, Ory | Custo por conexão (WorkOS) e Organizations só na oferta gerenciada (Ory) |

Preços e limites conferidos nas páginas oficiais em 12/10/2026.

## Consequências

**Positivas:**

- Segue os defaults do `authn`; a saída é barata, porque o kernel só exige OIDC
  padrão.
- Cobre B2B, SSO por organização e usuário em várias organizações sem construir
  essa camada.
- Sem licença e sem restrição de residência dos dados.

**Negativas:**

- Operar um IdP crítico: alta disponibilidade, atualização de segurança em dias,
  backup do realm.
- A revogação em 60 s depende de componente novo no kernel (deny-list) e da
  fonte de eventos do Keycloak.
- A região de hospedagem do Keycloak, e com ela a eventual transferência
  internacional pela LGPD, entra no escopo da spec de produção (ADR-019).

## Pendências

- Condição 1 — SPEC-SBW7HT6Y (DEVS-97).
- Condição 2 — SPEC-STEJG347 (DEVS-101), que depende da SPEC-SBW7HT6Y.
- Condição 3 — SPEC-7KW0HHF6 (Identity & Organization).
- Ajuste na SPEC-7KW0HHF6: trocar a lista de revogação com cache de 30 s pela
  deny-list da SPEC-STEJG347, consultada em toda requisição.

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- SPEC-7KW0HHF6 — `docs/specs/SPEC-7KW0HHF6-identity-organization.md`
- [ADR-013](./013-multi-tenancy-por-execution-context.md),
  [ADR-019](./019-go-sobre-o-kernel-dmpf.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - `authn` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/authn/README.md`
  - IDN-01, IDN-10, CTX-06, CTX-16 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/contexto-erros-seguranca.md`
- Keycloak (Organizations) — `https://www.keycloak.org/docs/latest/server_admin/index.html`
- Cognito (revogação e TTL) — `https://docs.aws.amazon.com/cognito/latest/developerguide/token-revocation.html`
