# ADR-014: Persistir em PostgreSQL pelo `postgres.Table`, com persistência híbrida

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D3 vem de `docs/design/00-arquitetura-tecnica.md` §5.1, que escolhe
PostgreSQL pela RLS nativa, pela transação forte para a outbox, pelo JSONB para
payload externo preservado e pelo particionamento declarativo.
`docs/design/90-modelo-de-dados.md` desenha um modelo físico de tabelas largas,
com colunas por atributo; o doc 00 §5.3 e o RNF-V4 de
`docs/design/93-requisitos-nao-funcionais.md` pedem partição mensal das unidades
de recebível por data de liquidação.

O kernel DMPF já decide o banco e a forma de acessá-lo, o que torna D3 uma
decisão **imposta**:

- **PostgreSQL por app** (ADR-053 §1), com a outbox realizada em PostgreSQL
  (ADR-035) e o DDL do kernel publicado como capacidade (`postgres.Outbox`,
  `postgres.Inbox`), que `postgres.Migrate` aplica antes do schema do próprio
  contexto, em `provider/schema.sql` (ADR-053 §3).
- **Persistência híbrida** (ADR-053 §4): coluna tipada só para o que uma
  consulta filtra; o resto do estado vai num `snapshot` `jsonb`, serializado por
  um struct de estado privado do provider, com tags JSON estáveis.
- **Todo SQL de agregado passa pelo `postgres.Table`** (ADR-051), o choke point
  do escopo de tenant. A superfície é `Load` e `Save` por id com versão otimista,
  mais `Relation(filterColumn).Query`, que devolve as linhas do tenant cuja
  coluna de filtro é igual a um valor (`libs/backend/go/postgres/table.go:55-101`).
  Não há join, agregação nem consulta por intervalo.
- **Tempo em `bigint` de nanossegundos**, nunca `timestamptz` (ADR-035, ADR-053
  §8).

## Decisão

PostgreSQL é o banco de todo bounded context, um database por app (ADR-012),
acessado só pelo `postgres.Table` e pelas capacidades do kernel.

Cada agregado persiste no padrão híbrido: chave `(tenant_id, <agregado>_id)`,
`version`, as colunas tipadas que alguma consulta filtra e o `snapshot` `jsonb`
com o restante do estado. O DDL do contexto vive em `provider/schema.sql`.

O modelo de `docs/design/90-modelo-de-dados.md` vale como modelo **lógico**:
entidades, atributos, invariantes e cardinalidades. Ele não é o modelo físico.
As tabelas largas e a partição mensal (doc 00 §5.3, RNF-V4) não são adotadas,
porque o `Table` não faz join nem agregação, e consulta que precise disso vira
projeção de leitura materializada pelo próprio contexto, alimentada por evento.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Modelo físico do doc 90 (tabelas largas, colunas por atributo) | Exige SQL escrito à mão fora do `Table`, o que o ADR-051 fecha (pool só como `ReadPool`, driver negado nos providers de contexto) |
| Partições mensais declarativas nas tabelas de volume (doc 00 §5.3) | O `Table` não conhece partição; o volume se trata por retenção e por projeção, e a necessidade será medida (SPEC-6E7BKFNS, ADR-017) |
| Outro banco relacional ou documental | O kernel realiza outbox, inbox e escopo de tenant só em PostgreSQL |

## Consequências

**Positivas:**

- O escopo de tenant e a versão otimista valem para todo agregado sem código
  próprio do contexto.
- O estado evolui pelo `snapshot` sem migração de coluna; só o que é filtrado
  vira coluna.
- O payload externo preservado sem perda (doc 00 §5.1) cabe no `snapshot` ou em
  object storage, conforme o tamanho (ADR-018).

**Negativas:**

- Relatório, conciliação e visão de portfólio exigem projeções de leitura
  mantidas por evento, com o custo de duplicação e de reconstrução.
- O `snapshot` precisa de tags JSON estáveis e de evolução compatível; renomear
  campo é mudança de contrato do provider.
- Consultas por intervalo (agenda por data de liquidação) não saem do `Table`
  e precisam de coluna de filtro discreta ou de projeção.

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-012](./012-um-app-por-bounded-context.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - ADR-035 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/035-realizacao-postgres-da-outbox.md`
  - ADR-051 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md`
  - ADR-053 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md`
  - `postgres.Table` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/postgres/table.go`
