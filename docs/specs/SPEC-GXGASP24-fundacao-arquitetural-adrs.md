---
id: SPEC-GXGASP24
slug: fundacao-arquitetural-adrs
title: Fundação arquitetural — ADRs das decisões D1–D10 sobre o kernel DMPF
stage: done
priority: P0
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-76/spec-gxgasp24-fundacao-arquitetural-adrs-das-decisoes-d1-d10-sobre-o
subtask_urls: []
created: 2026-10-12
---

# SPEC-GXGASP24: Fundação arquitetural — ADRs das decisões D1–D10 sobre o kernel DMPF

## Resumo

Registrar em ADRs as dez decisões técnicas abertas da plataforma de antecipação
de recebíveis (D1 a D10 de `docs/design/00-arquitetura-tecnica.md` §12) e um ADR
de precedência que reconcilia os documentos de design com as normas do kernel
DMPF que este workspace consome. As extensões que essas decisões exigem do
kernel nascem como specs no repositório do kernel, para que cada ADR aponte uma
capacidade rastreável em vez de uma lacuna solta. Como arquiteto que vai puxar o
primeiro bounded context, quero cada decisão estrutural registrada e rastreável,
para que as 17 specs do catálogo de contextos sejam construídas sobre premissas
fixas.

## Contexto

- **Problema**: o doc 00 §12 exige que cada decisão aberta "vire um ADR antes do
  início da construção". Nenhuma virou. Ao mesmo tempo, o workspace nasceu do
  template DMPF (ADR-011), cujas normas já decidem parte dessas questões, e em
  pelo menos três casos de forma contrária à recomendação do doc 00 (D1, D2 e
  D4). Sem ADRs, cada spec de contexto herdaria premissas conflitantes.
- **Impacto**: as 17 specs de bounded context dependem desta spec
  (`depends_on`). Ela é o único item do catálogo que não depende de apuração
  externa para começar.
- **Versão do kernel**: o workspace consome o `@mateusmacedo/dmpf-plugin`
  `1.0.0-rc.4`, cujo `workflowRef` é o commit
  `487e9de17099c0eb8d44e3e9c2dfbd58b577b958`. Do rc.3 para o rc.4, `docs/dmpf/`
  não mudou; entraram o ADR-061 (cópias de produção dos contextos promovidas ao
  kernel) e o ADR-062 (kits de processo, telemetria e execução promovidos ao
  `testkit`).
- **Restrição de infraestrutura** (decisão do usuário, 12/10/2026): a
  infraestrutura segue a abstração do DMPF, com AWS como nuvem-alvo, floci como
  emulador e os recursos de infra do repositório do kernel (Compose, Kustomize,
  `infrasync`).
- **Classificação das decisões** (pesquisa contra as normas do kernel, por
  evidência em arquivo, e escolhas do usuário em 12/10/2026):

| Dn | Decisão (doc 00 §12) | Classe | Base no kernel DMPF | Status do ADR |
| --- | --- | --- | --- | --- |
| D1 | Monolito modular vs. microsserviços | Imposta | Módulo, binário, banco e role por contexto (ADR-044, ADR-045, ADR-048, ADR-053) | Aceito |
| D2 | Multi-tenancy | Condicionada | Escopo por `ExecutionContext` no `postgres.Table`; RLS rejeitado como mecanismo primário (ADR-050, ADR-051) | Aceito, com pendências |
| D3 | Banco primário | Imposta | PostgreSQL via `postgres.Table`, persistência híbrida (ADR-035, ADR-051, ADR-053) | Aceito |
| D4 | Broker | Condicionada | Kafka é o transporte-alvo, com decisão registrada por canal; SNS/SQS nos dois casos de TRP-40; Kafka é norma de desenho até a revisão de TRP-41 (TRP-37 a TRP-41) | Aceito, com pendências |
| D5 | Provedor de identidade | Condicionada | Verificador OIDC com issuer único, audiência e claim de tenant (`libs/backend/go/authn`, IDN-01) | Aceito, com condições |
| D6 | "Alteração material" em portfólio | Aberta | Sem norma | Aceito para o mecanismo, com pendências |
| D7 | Imutabilidade do ledger | Aberta | Auditoria com integridade própria (DAT-25), retenção com purga registrada (DAT-14 a DAT-18) | Aceito para o mecanismo base, com pendências |
| D8 | Linguagem e plataforma de execução | Imposta | Go no backend; Docker, Compose e Kustomize por app (ADR-054); floci como emulador AWS (ADR-039) | Aceito, com pendências |
| D9 | Expurgo do inbox | Condicionada | `retenção_inbox ≥ janela_redelivery` (INB-14), janela declarada por canal (TRP-23, TRP-24, TRP-24b), zonas de replay (INB-15), purga com evidência (INB-16) | Aceito |
| D10 | Dados de teste | Aberta | Banco por teste e infra de testes separada (ADR-053, ADR-054); kits do `testkit` (ADR-062) | Aceito para as fases 0 e 1, com pendências |

- **Links relevantes**:
  - `docs/design/00-arquitetura-tecnica.md` §3 a §12 — origem das decisões.
  - `docs/adr/README.md` — template e índice dos ADRs.
  - `docs/adr/011-template-consome-dmpf-por-versao.md` — consumo do DMPF por
    versão e bump por `nx migrate`.
  - `.claude/rules/dmpf-bounded-context.md` — normas do kernel aplicadas a
    contextos, com as URLs fixadas no `workflowRef` do rc.4.
  - `docs/README.md`, "Lacunas bloqueantes" (#1 a #12), e
    `docs/resumo-executivo-apuracao.md` (perguntas A1 a E4) — lacunas externas
    que condicionam D4, D6, D7 e D10.
  - `docs/specs/README.md` do repositório do kernel (`github.com/mateusmacedo/dmpf`) — template das specs do kernel.
- **Referências externas**: normas do kernel DMPF em
  `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/`
  (ADRs e `docs/dmpf/`), a mesma revisão citada pela rule de bounded context.
  Linear do kernel: projeto DMPF (P-DEVS-2), time DevTeam.

<constraints>
- [P0] Um ADR por decisão, D1 a D10, numerados de `012` a `021` na ordem Dn, mais o ADR `022` de precedência; nenhum outro arquivo de ADR neste workspace.
- [P0] Seguir o template local de `docs/adr/README.md`: sem frontmatter, título `# ADR-0NN: <título imperativo em PT-BR>`, seções Status, Contexto, Decisão, Alternativas descartadas e Consequências, e `## Pendências` quando houver.
- [P0] Toda afirmação sobre o kernel cita a norma por número (ADR-051, TRP-38, INB-14) e por URL fixada no SHA `487e9de17099c0eb8d44e3e9c2dfbd58b577b958`; caminho local de outra máquina nunca é fonte.
- [P0] Decisão imposta pelo kernel é ratificada, não rediscutida: o ADR registra a recomendação divergente do doc 00 como alternativa descartada.
- [P0] Fato externo não apurado nunca vira premissa aceita: a parte de uma decisão que depende de lacuna externa (`#n`, pergunta `A1` a `E4`) ou de capacidade futura do kernel vai para `## Pendências`, que nomeia a lacuna ou a spec do kernel (ID e issue); a pendência fecha por addendum datado.
- [P0] Extensão do kernel nasce como spec no repositório do kernel, no template de `docs/specs/README.md` do repositório do kernel (`github.com/mateusmacedo/dmpf`), com stage `backlog` e issue no projeto DMPF (P-DEVS-2); este workspace não implementa nenhuma delas.
- [P1] ADR cita specs, nunca planos (`.claude/rules/ephemeral-refs.md`).
- [P1] `docs/prd/`, `docs/design/` e `docs/reference/` permanecem inalterados; a reconciliação vive no ADR `022`.
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] ADR-012 — D1, estilo de implantação**: ratificar um app por bounded context sobre o kernel DMPF (`apps/backend/<name>`, módulo, binário, database e role próprios), com REST público no `bff` e contextos servindo gRPC.
  - Alternativa descartada: monolito modular com schema por módulo (doc 00 §3.1), porque o kernel proíbe banco compartilhado entre apps.
  - Consequência obrigatória: estimar o número de processos em dev e hmg, que passa de 17 contextos mais o `bff`.
  - Registrar que o kernel nem permite nem proíbe vários contextos num binário, com os obstáculos verificados (ADR-054 §1, ADR-044, ADR-053 §10, `otelboot` único por processo, um `infra.json` por app); host multicontexto fica como spike futuro, só com necessidade medida.
- [x] **[P0] ADR-013 — D2, multi-tenancy**: adotar `tenant_id` à frente da chave, com o escopo injetado pelo `ExecutionContext` no `postgres.Table` e fechado por gate (`depguard`), conforme ADR-051 do kernel.
  - Alternativa descartada: RLS com `SET LOCAL app.tenant_id` como mecanismo primário (doc 00 §4). RLS só como segunda camada, viável quando a role de runtime não for superuser nem dona das tabelas (ADR-051; spec do kernel de duas roles, K1).
  - `## Pendências` com o acesso entre tenants (backoffice, doc 00 §4.1) e a projeção no tenant do financiador (doc 00 §4.3), apontando a spec do kernel K4.
- [x] **[P0] ADR-014 — D3, banco primário**: ratificar PostgreSQL acessado só pelo `postgres.Table`, persistência híbrida (coluna tipada só para o que uma consulta filtra, o resto em `snapshot` `jsonb`) e DDL em `provider/schema.sql` por app.
  - Registrar que as tabelas largas de `docs/design/90-modelo-de-dados.md` e a partição mensal do doc 00 §5.3 e do RNF-V4 não são o modelo físico, porque o `Table` não faz join nem agregação.
- [x] **[P0] ADR-015 — D4, broker**: adotar Kafka como transporte-alvo de eventos de domínio (TRP-39), com o critério de TRP-37 registrado por canal; SNS/SQS nos dois casos de TRP-40 (fan-out com filtro por atributo e fila de trabalho sem ordem), emulados pelo floci.
  - Alternativa descartada: fila gerenciada FIFO como transporte universal (doc 00 §6.3), porque a ordem por chave é atendida por partição e o critério de replay não discrimina.
  - `## Pendências`: Kafka é norma de desenho até a revisão de infraestrutura de mensageria (TRP-41), com alvo AWS MSK e Redpanda local, apontando a spec do kernel K8; a volumetria (#12) dimensiona partições e retenção sem mudar a escolha.
- [x] **[P0] ADR-016 — D5, provedor de identidade**: adotar Keycloak 26.x autogerenciado, realm único, como IdP OIDC (código de autorização com PKCE, MFA, Organizations), em conformidade com o verificador `authn` do kernel (issuer único, audiência e claim de tenant obrigatórias).
  - Alternativas descartadas: Cognito e Entra External ID (TTL mínimo de token incompatível com revogação em 60 s; o Cognito do floci não tem código de autorização), Auth0, Zitadel e WorkOS (custo, residência ou lock-in).
  - Condições: (1) mapper que achata `organization` em `tenant_id` string, com tenant ativo para usuário em várias organizações, e Keycloak de referência na infra do DMPF — K6; (2) revogação em 60 s por deny-list de `sub`/`sid` alimentada por eventos do IdP — K7; (3) origem das permissões efetivas (IdP ou enriquecimento no `bff`) — decisão da SPEC-7KW0HHF6.
- [x] **[P0] ADR-017 — D6, alteração material**: adotar o mecanismo de snapshot imutável com `contentHash` do conjunto de pares `(assetKey, assetHash)` e `materialityPolicyVersion` na pré-imagem; gate comparando só com o último snapshot publicado do mesmo escopo; confirmação append-only; versionar por mudança exata nos campos de decisão, com tolerância só para invalidação e alerta; campos de identidade no `assetKey`, fora do hash; simulação nunca publica.
  - Representação física reversível: fase 1 com cópia por snapshot (preferindo documento único por snapshot); migração para manifesto com versão por ativo por gatilho de volumetria (estoque de referenciados acima de 1 a 2 TB em 5 anos ou acima de ~50 `RecordAsset`/s).
  - `## Pendências`: a lista de campos de decisão é política v0, que depende de `#3`, `#4`, `B4` e `B7`; o custo depende de `#12` e N1.
- [x] **[P0] ADR-018 — D7, imutabilidade do ledger**: adotar o mecanismo base em camadas: (1) role de escrita com INSERT e SELECT em `audit_records`, trigger contra UPDATE, DELETE e TRUNCATE, e role dona e migradora fora do runtime; (2) hash encadeado por partição com algoritmo prefixado e canonicalização definida (RFC 8785, inteiros de 64 bits como string decimal, dado pessoal fora da pré-imagem); (3) selo periódico (`SealManifest`) com raiz Merkle das cabeças de todas as partições e hash do selo anterior, assinado por chave em AWS KMS e gravado em S3 Object Lock no escopo de plataforma (COMPLIANCE para selos, com prazo inicial ≤ teto e prorrogação; GOVERNANCE para evidência bruta até `#8`), atrás das portas de object storage e de assinatura do kernel, neutras de nuvem; (4) purga por remoção do início da cadeia pela role migradora, com âncora do último hash e evidência (DAT-18).
  - Registrar as restrições do kernel: a role do app é dona do banco (ADR-051, ADR-053); a camada 1 depende de K1, o storage de K2, a assinatura de K3 e a leitura das cabeças entre tenants de K4.
  - Alternativas descartadas: Amazon QLDB (fim de suporte em 31/07/2025), Azure SQL Ledger, immudb e MinIO (arquivado).
  - `## Pendências`: âncora RFC 3161 por ACT ICP-Brasil (fase 2, decisão de produto e jurídica), prazos de retenção (`#8`, A13), as specs K1 a K4 e os ajustes na SPEC-59BCT7T1 (selo, período, verificador e campos mutáveis).
- [x] **[P0] ADR-019 — D8, linguagem e plataforma**: ratificar Go no backend, sobre o kernel DMPF, com Docker, Compose e Kustomize por app; fixar AWS como nuvem-alvo, floci como emulador e a infra do repositório do kernel em dev e nos testes (o hmg do kernel usa Postgres e Kafka externos).
  - `## Pendências` com o que o kernel não cobre: stack de front-end e infraestrutura AWS de hmg e de produção (contas, IaC, operação, conta de contrato para os testes contra S3 e KMS reais).
- [x] **[P0] ADR-020 — D9, expurgo do inbox**: cada canal declara a `janela_redelivery` com fórmula e parâmetros (TRP-23, TRP-24, TRP-24b), e a retenção do inbox satisfaz `retenção_inbox ≥ janela_redelivery` (INB-14), com margem opcional declarada; a comparação é feita no `Validate` de cada contexto consumidor, com teste, porque o kernel só exige a janela declarada.
  - Replay declara a zona em que opera (INB-15); a purga é registrada como evidência (INB-16) pelos laços de purga do kernel (ADR-056).
  - Alternativa descartada: retenção do inbox como múltiplo fixo da retenção do tópico, porque INB-14 compara com a janela de redelivery, não com a retenção.
- [x] **[P0] ADR-021 — D10, dados de teste**: adotar a fase 0 (builders sintéticos com CPF e CNPJ válidos, inclusive o alfanumérico; geração dirigida por contrato; property-based com semente fixa; simulador de parceiro in-process com toda a taxonomia de erro; goldens de conciliação) e a fase 1 (homologação das registradoras como validação de contrato, fixtures sanitizadas, simulador de protocolo), com as regras R1 a R7.
  - Homologação usa dados sintéticos, não anonimizados (diverge do doc 00 §11).
  - `## Pendências`: fase 2 (cópia anonimizada de produção) só com gatilho e parecer jurídico; acesso e custo da homologação (`B17`); formatos das registradoras (`#2` a `#5`, `B4`, `B7`, `B8`, `R10`).
- [x] **[P0] ADR-022 — Precedência das normas DMPF sobre os documentos de design**: declarar que, onde `docs/design/` contradiz o kernel, prevalece o kernel; tabela `divergência | doc de design | norma DMPF | resolução | onde fica tratada` com as treze divergências abaixo.
  - Estilo de implantação (D1), apontando para ADR-012.
  - Multi-tenancy (D2), apontando para ADR-013 e K4.
  - Broker (D4), apontando para ADR-015 e K8.
  - Modelo físico de dados (doc 90 vs. persistência híbrida), apontando para ADR-014.
  - Imutabilidade por permissão de banco (doc 00 §5.2) vs. role dona do banco, apontando para ADR-018 e K1.
  - Envelope de eventos: JSON com `schemaUri` (doc 92) vs. CloudEvents Protobuf com Buf (ADR-022 e ADR-023 do kernel), incluindo o `partitionkey` obrigatório que o doc 92 não lista e a validação contra registry na publicação (doc 00 §3.2), sem equivalente em runtime no kernel.
  - Borda: Client API, Partner API e webhooks REST (doc 91) vs. REST só no `bff` e contextos gRPC (ADR-024 e ADR-053 do kernel).
  - Autenticação: federação por organização e revogação em 60 s (doc 01) vs. `authn` de issuer único, apontando para ADR-016, K6 e K7.
  - Tempo: milissegundos (doc 00 §8) vs. inteiro de nanossegundos no domínio.
  - Ambientes: quatro (doc 00 §11) vs. baseline dev e hmg.
  - Comando externo de saída com `UNKNOWN_OUTCOME` e idempotência de saída (doc 00 §7) sem equivalente no kernel (ADR-056 cobre só entrada), apontando para K5.
  - Armazenamento de evidência bruta em object storage (doc 00 §5.2) sem provider no kernel, apontando para ADR-018 e K2.
  - Dados de homologação: anonimizados (doc 00 §11) vs. sintéticos, apontando para ADR-021.
- [x] **[P0] Specs de extensão no kernel**: criar em `docs/specs/` do repositório do kernel (`github.com/mateusmacedo/dmpf`) as oito specs abaixo, cada uma com issue no P-DEVS-2 (título `[SPEC-<id>] <título>`, relações "bloqueada por" conforme o `depends_on`), numa branch de trabalho do kernel criada a partir de `develop`.

| # | Título | Prioridade | depends_on | Atende |
| --- | --- | --- | --- | --- |
| K1 | DMPF — Duas roles de banco por app: migrador fora do runtime e runtime com privilégio por tabela e capability | P1 | SPEC-F308KDSF, SPEC-JJKWG4JP, SPEC-VJMM2DE5 | ADR-018 camada 1; RLS de segunda camada do ADR-013 |
| K2 | DMPF — Porta de object storage com versionId, retenção WORM e legal hold; provider S3; floci ≥ 2.2.0 com S3 e KMS | P1 | SPEC-XMNBMY50, SPEC-VJMM2DE5 | ADR-018 camada 3 |
| K3 | DMPF — Porta de assinatura com provider AWS KMS emulado no floci | P1 | K2 | ADR-018 selo |
| K4 | DMPF — Acesso entre tenants declarado: leitura de plataforma sobre `postgres.Table` e projeção entre tenants | P1 | SPEC-9B6SHEH8, K1 | ADR-013 pendência; ADR-018 selo e verificador |
| K5 | DMPF — Ciclo de comando externo de saída: chave idempotente, desfecho incerto, consulta de estado e reconciliação | P1 | SPEC-E15TBHCD, SPEC-JJKWG4JP, SPEC-NYD18TGD | ADR-022 (Gateway, Effects, Funding) |
| K6 | DMPF — Keycloak de referência na infra do DMPF: realm com Organizations, claim de tenant e eventos de sessão | P1 | SPEC-VJMM2DE5 | ADR-016 condição 1 e fonte da 2 |
| K7 | DMPF — Revogação de sessão no authn: deny-list por sub/sid alimentada por eventos do IdP | P1 | SPEC-9B6SHEH8, K6 | ADR-016 condição 2 |
| K8 | DMPF — Revisão de infraestrutura de mensageria Kafka para autorizar operação (TRP-41) com alvo AWS MSK | P2 | SPEC-EAGAXQN1 | ADR-015 pendência |

- [x] **[P1] Índice de ADRs**: em `docs/adr/README.md`, atualizar o último ADR registrado para `022` e acrescentar as linhas ADR-012 a ADR-022 à tabela "Decisões registradas".
- [x] **[P1] Rastreabilidade**: cada ADR declara, no Status, `Implementa SPEC-GXGASP24 (DEVS-76)` e, no Contexto, a seção do doc 00 de onde a decisão vem; cada spec do kernel citada aparece com ID e issue.

### Não-funcionais

- [x] Rastreabilidade: 100% das afirmações sobre o kernel com número de norma e URL fixada no SHA, e cada URL conferida com `git cat-file -e 487e9de:<path>`.
- [x] Legibilidade: cada ADR com no máximo 120 linhas; cada spec do kernel com alvo de até 400 linhas; detalhamento extenso vai para o design de origem, citado.
- [x] Idioma: prosa em PT-BR com acentuação correta (revisão `pt-reviewer` sem erros).

## Camadas afetadas

| Camada | Afetada? | Descrição |
| --- | --- | --- |
| Documentação de decisão (`docs/adr/`) | [x] | Onze ADRs novos e o README atualizado |
| Specs deste workspace (`docs/specs/`) | [x] | Esta spec; as specs de contexto já a citam e ficam inalteradas |
| Specs do kernel (`docs/specs/` em `mateusmacedo/dmpf`) | [x] | Oito specs de extensão em `backlog`, com issues no P-DEVS-2 |
| Ativos do `init` do DMPF | [x] | Bump para o `1.0.0-rc.4` por `nx migrate` (ADR-011), pré-requisito das citações no SHA `487e9de` |
| Design e PRD | [ ] | Inalterados por constraint; a reconciliação vive no ADR-022 |
| Código, contratos, infraestrutura | [ ] | Nenhum; a construção começa nas specs de contexto e nas specs do kernel |

## Localização de código

```text
docs/adr/
  012-um-app-por-bounded-context.md
  013-multi-tenancy-por-execution-context.md
  014-postgresql-persistencia-hibrida.md
  015-kafka-como-broker.md
  016-provedor-de-identidade-oidc.md
  017-alteracao-material-no-portfolio.md
  018-imutabilidade-do-ledger.md
  019-go-sobre-o-kernel-dmpf.md
  020-expurgo-do-inbox.md
  021-dados-de-teste-sinteticos.md
  022-precedencia-dmpf-sobre-design.md
  README.md
dmpf (repositório do kernel): docs/specs/
  SPEC-<id>-<slug>.md   (oito, K1 a K8)
```

**Arquivos a modificar**:

- `docs/adr/README.md` — número do último ADR e linhas novas da tabela
  "Decisões registradas".
- `pnpm-workspace.yaml`, `AGENTS.md` e os ativos do `init` (`.claude/`,
  `.agents/`, `.github/workflows/dmpf-ci.yml`, `.golangci.yml`,
  `dmpf.rendered.json`) — bump para o rc.4.

## Design

### Arquitetura

As specs do kernel vêm antes dos ADRs, porque os ADRs citam os IDs reais. Os
ADRs formam três grupos, escritos nesta ordem porque os posteriores citam os
anteriores:

1. **Ratificações** (D1, D3, D8): fixam o terreno imposto pelo kernel e pela
   restrição de infraestrutura.
2. **Decisões condicionadas** (D2, D4, D5, D9): escolhem dentro do espaço que o
   kernel deixa; D9 cita D4.
3. **Decisões com pendências** (D6, D7, D10) e o ADR-022: aceitam o mecanismo,
   isolam o que depende de fato externo e reconciliam o design com tudo o que
   veio antes.

### Fluxo principal

1. Subir o workspace para o rc.4 e fixar as citações no `workflowRef` `487e9de`.
2. Gerar os IDs, as issues e a branch do kernel; escrever e validar as specs K1
   a K8.
3. Para cada Dn, ler a seção de origem no doc 00 e o design do contexto afetado,
   e conferir a evidência no kernel pela URL fixada no SHA; se a evidência não se
   confirmar, rebaixar a classe e mover a parte não confirmada para
   `## Pendências`, com o motivo no Contexto do ADR.
4. Escrever o ADR-022 com a tabela das treze divergências, apontando cada linha
   para o ADR ou a spec que a trata.
5. Atualizar `docs/adr/README.md` e rodar os gates e a revisão de português.
6. Na entrega, o PR do kernel é mergeado em `develop` antes do PR deste
   workspace.

## Decisões técnicas

- **Um ADR por decisão, não um ADR consolidado**: cada decisão fecha pendências
  em ritmo próprio (D6 espera as registradoras, D7 espera a retenção), e um
  addendum datado por ADR preserva a história. Alternativa descartada: ADR único
  com D1 a D10, porque o addendum de uma decisão reabriria o arquivo das outras.
- **Pendências em seção própria, sem status `Proposto`**: o usuário decidiu D5,
  D6, D7 e D10; o que ainda depende de fato externo ou de capacidade futura do
  kernel fica em `## Pendências`, com a lacuna ou a spec do kernel nomeada.
  Alternativa descartada: manter `Proposto`, que deixaria sem referência estável
  um mecanismo já escolhido.
- **Extensões como specs no kernel, uma capacidade por spec**: cada extensão tem
  dono, prioridade e dependência próprios. Alternativas descartadas: registrar as
  extensões só no ADR-022, que não dá rastreabilidade no repositório que as
  implementa; uma spec guarda-chuva com ondas, que mistura capacidades sem
  relação e estoura o limite de requisitos.
- **Precedência num ADR próprio, sem editar o design**: o `docs/README.md` trata
  `design/` como camada que muda quando a implementação evolui, mas reescrever 24
  documentos agora apagaria o raciocínio original. Alternativa descartada: editar
  cada documento de design, pelo custo e pelo risco de perder a rastreabilidade
  das lacunas.

## Regras relacionadas

- `docs/adr/README.md` — template dos ADRs.
- `.claude/rules/ephemeral-refs.md` — ADR cita specs, não planos.
- `.claude/rules/dmpf-bounded-context.md` — normas do kernel aplicadas a
  contextos.
- `docs/specs/README.md` do repositório do kernel (`github.com/mateusmacedo/dmpf`) — template das specs do kernel.
- Specs do catálogo de bounded contexts (`docs/specs/SPEC-*`, título
  `Bounded context — <ctx>`) — dependem desta spec.

## Verificação e testes

### Critérios de aceite

- [x] O workspace consome o `@mateusmacedo/dmpf-plugin` `1.0.0-rc.4`, e os ativos do `init` citam o SHA `487e9de`.
- [x] Existem exatamente onze arquivos novos em `docs/adr/`, de `012` a `022`, com os nomes da Localização de código.
- [x] Cada ADR tem, nesta ordem, as seções Status, Contexto, Decisão, Alternativas descartadas, Consequências e, quando houver, Pendências, e começa por `# ADR-0NN:`.
- [x] O status de cada ADR é `Aceito`, com `Implementa SPEC-GXGASP24 (DEVS-76)`, e corresponde à tabela do Contexto.
- [x] Cada `## Pendências` nomeia a lacuna externa ou a spec do kernel (ID e issue) que a fecha, ou declara que ainda não tem spec dona.
- [x] O ADR-022 tem uma linha para cada uma das treze divergências listadas nos requisitos, sem repetição.
- [x] Existem oito specs K1 a K8 em `docs/specs/` do repositório do kernel, aprovadas pelo validador estrutural (frontmatter de 9 campos, ID único, `depends_on` existente, DAG acíclico, `ticket_url` válido, `<constraints>` e `<critical_constraints>`, seções obrigatórias), cada uma com issue no P-DEVS-2.
- [x] `docs/adr/README.md` aponta `022` como último ADR e lista ADR-012 a ADR-022.
- [x] `rg -n 'plans/|CHECKPOINT|PLAN\.md' docs/adr/0{12..22}-*.md` não retorna nada.
- [x] Nenhum arquivo de `docs/prd`, `docs/design` e `docs/reference` alterado por esta spec (conferido por data de modificação enquanto não versionados, e por `git diff` contra a versão de origem depois).
- [x] Revisão `pt-reviewer` sem erros nos onze ADRs, no README e nas oito specs do kernel.

### Cenários de teste (mínimo 3)

```text
DADO o ADR-012 escrito como ratificação de D1
QUANDO se procura no arquivo a recomendação do doc 00 §3.1 (monolito modular)
ENTÃO ela aparece em "Alternativas descartadas", com o motivo citando a norma do kernel por número e URL fixada no SHA

DADO uma decisão com parte dependente de lacuna externa (D6, alteração material)
QUANDO o ADR-017 é escrito
ENTÃO o status é Aceito para o mecanismo, a lista de campos aparece como política v0 em "Pendências" com as lacunas #3, #4, B4 e B7, e nenhuma lista de campos aparece como definitiva

DADO um canal Kafka declarado por um contexto
QUANDO o ADR-020 é aplicado a ele
ENTÃO o canal declara a janela_redelivery com fórmula e parâmetros, e a retenção do inbox é maior ou igual a essa janela

DADO que a conferência no kernel não confirma uma evidência da tabela do Contexto
QUANDO o ADR da decisão é escrito
ENTÃO a classe é rebaixada, o motivo fica no Contexto do ADR e a parte não confirmada vai para "Pendências"

DADO o ADR-022 concluído
QUANDO uma spec de contexto encontra divergência entre o design e o kernel
ENTÃO a divergência está na tabela do ADR-022 com a resolução e o ADR ou a spec responsável, ou é acrescentada a ela por addendum datado
```

<critical_constraints>
- [P0] Um ADR por decisão, D1 a D10, numerados de `012` a `021` na ordem Dn, mais o ADR `022` de precedência; nenhum outro arquivo de ADR neste workspace.
- [P0] Seguir o template local de `docs/adr/README.md`: sem frontmatter, título `# ADR-0NN: <título imperativo em PT-BR>`, seções Status, Contexto, Decisão, Alternativas descartadas e Consequências, e `## Pendências` quando houver.
- [P0] Toda afirmação sobre o kernel cita a norma por número (ADR-051, TRP-38, INB-14) e por URL fixada no SHA `487e9de17099c0eb8d44e3e9c2dfbd58b577b958`; caminho local de outra máquina nunca é fonte.
- [P0] Decisão imposta pelo kernel é ratificada, não rediscutida: o ADR registra a recomendação divergente do doc 00 como alternativa descartada.
- [P0] Fato externo não apurado nunca vira premissa aceita: a parte de uma decisão que depende de lacuna externa (`#n`, pergunta `A1` a `E4`) ou de capacidade futura do kernel vai para `## Pendências`, que nomeia a lacuna ou a spec do kernel (ID e issue); a pendência fecha por addendum datado.
- [P0] Extensão do kernel nasce como spec no repositório do kernel, no template de `docs/specs/README.md` do repositório do kernel (`github.com/mateusmacedo/dmpf`), com stage `backlog` e issue no projeto DMPF (P-DEVS-2); este workspace não implementa nenhuma delas.
- [P1] ADR cita specs, nunca planos (`.claude/rules/ephemeral-refs.md`).
- [P1] `docs/prd/`, `docs/design/` e `docs/reference/` permanecem inalterados; a reconciliação vive no ADR `022`.
</critical_constraints>

## Escopo fora

- **Implementação das extensões do kernel** (K1 a K8): as specs nascem em
  `backlog` no repositório do kernel; implementá-las é trabalho de lá.
- **Host multicontexto num binário**: spike futuro, só com necessidade medida.
- **Decisões de produto** (patrocínio no ranking, sacado no escopo, confirmação
  parcial, autor do instrumento contratual, aprovação única para organizações
  pequenas, origem das permissões efetivas): pertencem às specs dos contextos que
  as consomem.
- **Reescrita de `docs/design/` e `docs/prd/`**: a precedência fica no ADR-022;
  atualizar os documentos é trabalho de outra spec, se for pedido.
- **Lacunas técnicas fora de D1 a D10** (rotação do salt do hash fiscal, lacuna L8;
  validação de titularidade de conta, S1): pertencem às specs de Identity,
  Funding e KYB, que já as citam.
- **Front-end e ambiente de produção**: ficam como pendências do ADR-019, sem
  decisão nesta spec.
- **Implementação de qualquer bounded context**: começa nas specs do catálogo,
  depois desta.
