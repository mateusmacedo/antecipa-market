# ADR-013: Isolar tenants pelo `ExecutionContext` no `postgres.Table`, com RLS só como segunda camada

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

> **Addendum (2026-10-19)** — o kernel `1.0.0-rc.6` entregou a SPEC-QVDGV6J0
> (DEVS-100; ADR-068 do kernel), e a pendência de acesso e projeção entre
> tenants deixou de ser lacuna do kernel. A leitura de plataforma passa por
> `PlatformReader` sobre o `postgres.Table`, com alcance declarado
> (`ports.PlatformReach`, de que `http.PlatformReach` é alias), auditoria que
> falha fechada e gate `forbidigo` (`dmpf-gate-check.sh`). A projeção passa por
> `Projector`, com o tenant de destino explícito, tirado do payload, e o
> envelope inalterado (ADR-068, item 7). O ADR-068 emenda o ADR-050 e o ADR-051
> do kernel e, in-place, o `IDN-11` e o `IDN-12`. A decisão deste ADR vale como
> está; o uso aqui depende da adoção da `1.0.0-rc.6` (DEVS-136), e o texto de
> Contexto, Consequências e Pendências sobre esse ponto fica como registro
> histórico.

## Contexto

A decisão D2 vem de `docs/design/00-arquitetura-tecnica.md` §4. O §4.1
recomenda `tenant_id` em toda tabela de negócio com row-level security ativa e
`SET LOCAL app.tenant_id` a cada transação, porque filtro em código "falha em
silêncio". O §4.1 também pede um papel de plataforma com política de RLS própria
para o backoffice, com todo acesso entre tenants registrado no ledger. O §4.2
separa tenant de organização: o financiador vê dados do pedido do cedente por
**projeção no tenant do financiador**, sob concessão explícita, e não por leitura
através da fronteira.

O kernel DMPF condiciona a escolha (classe **condicionada**):

- **Choke point em Go, não RLS** (ADR-051, que implementa IDN-14): todo SQL de
  agregado passa pelo `postgres.Table`, que põe `tenant_id` no predicado e na
  chave a partir do `ports.ExecutionContext`; tenant ausente recusa
  (`ErrTenantUnresolved`, IDN-15). `Tx.Conn()` foi removido, o pool chega ao
  provider só como `ReadPool`, e a regra `depguard` `context-provider` nega os
  drivers em `apps/**/provider/**`.
- **Acesso cruzado vira evento**: a falta de linha que existe em outro tenant
  responde `ErrNotFound` (IDN-13) e sinaliza `ports.CrossTenantAccess` como
  evento de segurança (IDN-12).
- **RLS inerte com a role atual**: o ADR-051 mediu a role da conexão como
  superuser e dona das tabelas, e o Postgres ignora RLS para superuser. No rc.4,
  com banco por app, a role é `LOGIN` e dona do banco
  (`infra/local/compose/provisioning.generated.yml:26-29`):
  `FORCE ROW LEVEL SECURITY` passaria a valer, mas o dono pode desligá-la. O
  ADR-051 deixa RLS como segunda camada quando houver role de aplicação que não
  seja superuser nem dona das tabelas.
- **Tabelas de infraestrutura fora do escopo de tenant** (ADR-050): outbox,
  inbox e quarentena são de plataforma; o tenant atravessa a outbox no
  `metadata`, resolvido do portador.
- **Tenant só da autenticação** (IDN-01, CTX-06): o tenant do contexto vem do
  token verificado, nunca de campo da entrada.
- **Alcance de plataforma declarado** (IDN-19): operação de plataforma declara o
  escopo de dado; alcance irrestrito é declarado e autorizado como tal. O
  `postgres.Table`, porém, sempre filtra pelo tenant do portador, e não há
  caminho de leitura entre tenants nem de projeção com tenant de destino
  diferente do portador.

## Decisão

- Toda tabela de agregado tem `tenant_id` à frente da chave, posto pelo
  `postgres.Table` a partir do `ExecutionContext`; nenhum contexto escreve SQL de
  agregado fora dele, e o gate `context-provider` permanece ligado.
- O tenant é o domínio de isolamento, não a organização (doc 00 §4.2). O token
  traz o tenant ativo; o mapeamento de organização para tenant é do IdP
  (ADR-016).
- Acesso entre tenants por engano responde como inexistente e gera evento de
  segurança, conforme ADR-051.
- RLS fica como segunda camada, a ligar quando a role de runtime não for
  superuser nem dona das tabelas, o que depende da spec de duas roles por app
  no kernel (SPEC-QPRR24C4, DEVS-94), inclusive da investigação de superuser em
  Kubernetes e CI.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| RLS com `SET LOCAL app.tenant_id` como mecanismo primário (doc 00 §4.1) | Inerte com a role dona das tabelas (ADR-051); o choke point já torna impossível a consulta sem tenant no caminho da aplicação |
| Banco ou schema por tenant (doc 00 §4.1) | Proibitivo com dezenas a milhares de organizações e incompatível com o banco por app do kernel (ADR-053) |
| Filtro de tenant escrito por cada contexto | Recusado por IDN-14: o isolamento não pode depender de cada autor lembrar da condição |

## Consequências

**Positivas:**

- A consulta sem tenant não tem como ser escrita no caminho da aplicação.
- A tentativa de leitura entre tenants por erro é observável como evento de
  segurança, sem vazar o estado.

**Negativas:**

- Backoffice e projeção entre tenants não têm caminho no kernel atual; 14 das 17
  specs de contexto citam acesso ou projeção entre tenants e dependem da spec do
  kernel listada em Pendências.
- A sonda do acesso cruzado custa uma consulta extra a cada linha não encontrada
  (ADR-051).

## Pendências

- **Leitura de plataforma e projeção entre tenants**: SPEC-QVDGV6J0 (DEVS-100),
  no repositório do kernel — leitura declarada sobre o `Table` com alcance
  auditado (IDN-19) e projeção com tenant de destino explícito, decidida em ADR
  do kernel por tocar ADR-050 e CTX-24 a CTX-26.
- **RLS como segunda camada**: depende da SPEC-QPRR24C4 (DEVS-94).

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-012](./012-um-app-por-bounded-context.md),
  [ADR-016](./016-provedor-de-identidade-oidc.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - ADR-050 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md`
  - ADR-051 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md`
  - ADR-053 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md`
  - IDN-01, IDN-12 a IDN-15, IDN-19, CTX-06, CTX-24 a CTX-26 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/dmpf/contexto-erros-seguranca.md`
