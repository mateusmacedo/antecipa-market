# ADR-012: Implantar um app por bounded context sobre o kernel DMPF

## Status

Aceito — 2026-10-12. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D1 vem de `docs/design/00-arquitetura-tecnica.md` §3.1: o design
recomenda começar como monolito modular — um processo de implantação, módulos
1:1 com os bounded contexts, banco lógico segregado por schema — e extrair o
Receivables Gateway e o Compliance Ledger quando um critério de carga, de
segurança ou de time aparecer. O §3.2 condiciona a recomendação a quatro
imposições mecânicas da fronteira.

O workspace consome o kernel DMPF por versão (ADR-011), hoje o
`@mateusmacedo/dmpf-plugin` `1.0.0-rc.4`. As normas do kernel já decidem o
estilo de implantação, o que torna D1 uma decisão **imposta**:

- **Banco e role por app** (ADR-053 §1): database e role têm o nome do app, e
  cada app cria o próprio banco e as próprias tabelas (ADR-054 §5).
- **Um módulo por contexto, em `apps/backend/<ctx>`** (ADR-045, ADR-046), com
  layout canônico e binário único em `cmd/main.go` (ADR-048).
- **Contrato e deploy dentro do app** (ADR-054 §1 e §4): módulo `contract/`
  próprio e `deploy/` com Kustomize (`overlays/{dev,hmg}`), Compose e
  `infra.json`, a partir do qual o `infrasync` gera a infra agregada.
- **Borda REST só no `bff`, contextos em gRPC** (ADR-044, ADR-024): o `bff` é a
  única borda REST/JSON pública e não importa domínio nem aplicação; os
  contextos servem gRPC e drenam a outbox do próprio banco.

Das quatro imposições do §3.2, o kernel cobre três: o verificador de unidades
reprova import cruzado, o banco por app impede leitura de tabela alheia, e o
pacote público do contexto é o contrato. A quarta, validar o evento contra um
registry no momento da publicação, falhando a transação, não tem equivalente em
runtime: o kernel valida o schema com Buf em gate fail-closed de CI e deixa o
registry em runtime como pendência declarada (ADR-023); ver o ADR-022.

**Vários contextos num binário.** A spec pediu verificar se o kernel permite
compor contextos num único processo. O kernel não permite nem proíbe, mas a
composição esbarra em quatro pontos: o contrato e o deploy são do app
(ADR-054 §1 e §4), com um `infra.json` por app; a configuração não tem prefixo
por contexto (`PG_DSN`, `GRPC_ADDR`; ADR-053 §10), então dois contextos no mesmo
processo disputam as mesmas variáveis; o `otelboot` registra providers globais
de trace e métrica (`libs/backend/go/observability/otelboot/start.go:78-79`,
`:135`), o que dá uma identidade de telemetria por processo; e cada contexto
tem o próprio banco, logo o próprio pool.

## Decisão

Cada um dos 17 bounded contexts do catálogo é um app próprio sobre o kernel
DMPF: `apps/backend/<name>`, com módulo Go, binário, database, role, contrato
e `deploy/` próprios, servindo gRPC. O REST público fica no `bff`, que não
importa domínio nem aplicação de nenhum contexto.

O monolito modular do doc 00 §3.1 não é adotado. Os critérios de extração do
§3.1 perdem o objeto, porque o Gateway e o Ledger já nascem isolados.

Compor vários contextos num binário fica fora do escopo. Só vira spike quando
houver necessidade medida, como custo de processos em dev e hmg.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Monolito modular com schema por módulo (doc 00 §3.1) | O kernel dá banco e role a cada app (ADR-053 §1); um processo com vários schemas no mesmo banco contraria a norma e o `infra.json` por app |
| Monolito com extração precoce do Gateway e do Ledger | Mesmo motivo; o critério de extração deixa de ser necessário quando todo contexto já é app |
| Host multicontexto num binário | Configuração sem prefixo, telemetria global por processo e deploy por app; sem necessidade medida, o custo de quebrar essas normas não se paga |

## Consequências

**Positivas:**

- A fronteira entre contextos é imposta por processo, banco e verificador, sem
  depender de disciplina.
- O Gateway e o Ledger ganham isolamento de falha e de segurança desde o início,
  o que o doc 00 só previa na extração.
- Cada contexto é gerado pelo `bounded-context` do plugin e segue a forma
  canônica, o que mantém os 17 apps homogêneos.

**Negativas:**

- Mais processos. Cada contexto roda de dois a três papéis (`serve-api`,
  `serve-relay` e, se consome canal, `serve-consumer`; ADR-044, ADR-056 §8).
  Com 17 contextos mais o `bff`, dev e hmg rodam entre 35 e 52 processos, além
  da infra (Postgres, Redpanda, observabilidade).
- 17 databases e roles por ambiente, criados antes da primeira partida
  (ADR-053).
- Consistência entre contextos só por evento e por gRPC; não há transação
  distribuída (doc 00 §6.4, que o kernel também não oferece).

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-011](./011-template-consome-dmpf-por-versao.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - ADR-023 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/023-autoridade-de-validacao-e-registry.md`
  - ADR-024 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md`
  - ADR-044 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md`
  - ADR-045 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/045-nomes-bare-e-contexto-em-modulo-unico.md`
  - ADR-046 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/046-libs-somente-kernel-de-reuso.md`
  - ADR-048 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/048-layout-canonico-de-bounded-context.md`
  - ADR-053 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md`
  - ADR-054 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/054-apps-autocontidos-e-infras-separadas.md`
  - ADR-056 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/056-idempotencia-de-comando-pela-inbox.md`
  - `otelboot` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/observability/otelboot/start.go`
