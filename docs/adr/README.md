# Decisões Arquiteturais (ADRs)

Registro das decisões técnicas relevantes do projeto. Cada decisão vive em um
arquivo próprio neste diretório (`docs/adr/`), legível sem nenhuma ferramenta
externa. Este README é o índice e o guia de escrita.

## Quando criar um ADR

Abra um ADR quando a decisão tiver alcance e trade-offs que valha a pena
preservar:

- Escolha de tecnologia, framework ou biblioteca significativa.
- Padrão arquitetural que afeta múltiplos módulos ou apps.
- Mudança de abordagem de algo já estabelecido no projeto.
- Decisão com alternativas descartadas que precisam ficar rastreáveis.

Reserve o registro para o que sobrevive à sprint. Não abra um ADR para:

- Decisão trivial ou óbvia (por exemplo, adotar UTF-8) — siga o default.
- Escolha de implementação local que não cruza fronteiras de módulo — decida
  no próprio código.
- Correção de bug — documente no commit e, se couber, no teste de regressão.
- Mudança sem alternativas em jogo — não há trade-off a registrar.

## Nomenclatura

Um arquivo por decisão, numeração sequencial e slug em kebab-case:

```text
00N-slug-em-kebab-case.md
```

O número é o próximo livre na sequência (o último registrado abaixo é o
`022`). O título dentro do arquivo repete o número: `# ADR-00N: <título>`.

## Template

Copie o esqueleto abaixo. O formato é local e **sem frontmatter** — o arquivo
começa direto no `# ADR-00N:`.

```markdown
# ADR-00N: Título imperativo em PT-BR

## Status

Aceito — AAAA-MM-DD. Implementa SPEC-XXXXXXXX.

## Contexto

Qual problema ou necessidade levou à decisão. Restrições, requisitos e
contexto relevante.

## Decisão

O que foi decidido e por quê.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Opção A     | Motivo                |
| Opção B     | Motivo                |

## Consequências

**Positivas:**

- Benefício 1

**Negativas:**

- Trade-off 1
```

### `## Status` é opcional

O bloco `## Status` pode ser omitido: ADR-001 e ADR-002 não o trazem, enquanto
ADR-003 a ADR-009 o incluem. Quando presente, o padrão adotado é
`Aceito — AAAA-MM-DD`, seguido, quando aplicável, da referência à spec que a
decisão implementa (por exemplo, `Implementa SPEC-4NR9KKS8`). Uma decisão pode
substituir outra: registre isso na própria `## Decisão`, como ADR-005 fez ao
substituir itens do ADR-004.

### Addendum para revisões datadas

Para atualizar um ADR já aceito sem reescrever a decisão histórica, acrescente
ao final um bloco `## Addendum — AAAA-MM-DD`. Ele preserva o status e a
narrativa originais e apenas registra o estado novo — ver o addendum do
ADR-001.

## Decisões registradas

| Nº | Título | O que decide |
| --- | --- | --- |
| ADR-001 | Baseline do Monorepo Multistack | Parte do preset `--preset=ts` neutro e adiciona os plugins de forma controlada, em vez de herdar um preset focado em framework. |
| ADR-002 | Configuração de Tasks, Cache e Pipelines no NX | Define três camadas de configuração (plugin inferido, `targetDefaults`, `project.json`) e proíbe redeclarar targets para não quebrar o cache. |
| ADR-003 | Hardening do template (tooling, docs, CI/CD, infra) | Centraliza versões no `catalog:` do pnpm, fixa Node 24 / pnpm 11, migra o Biome para o formato 2.x e cria as docs de comunidade. |
| ADR-004 | Split version/publish e alvo Verdaccio | **Parcialmente supersedido** pela ADR-010 (2026-10-09): alvo de publicação e runner. Separa o versionamento (`release.yml`) da publicação de libs (`publish-libs.yml`) e adota o runner `gitea-runner`. |
| ADR-005 | A plataforma de hospedagem é Gitea | **Supersedido** pela ADR-010 (2026-10-09). Registro histórico: toda automação que fala com a plataforma usa a API do Gitea (`/api/v1`); o binário `gh` deixa de ser usado nos workflows. |
| ADR-006 | Fechamento do port de melhorias do monorepo derivado | Fixa a regra de desempate — a origem prevalece por padrão, o template é preservado onde está mais completo — ao portar melhorias. |
| ADR-007 | Fronteira entre o template e o plugin mmda-flow | **Supersedido** (2026-08-13). Registro histórico: bake vs consume. O template é autônomo e não depende de plugin externo. |
| ADR-008 | Destravar o release — proteções do repositório e ajuste do Nx Release ao Gitea | **Parcialmente supersedido** pela ADR-010 (2026-10-09): identidade do bot e push. Remove as proteções de branch e tag, remove o provider de release incompatível e alinha o `release.yml` ao workflow de referência da organização. |
| ADR-009 | O template não carrega libs nem generator de exemplo | Remove `shared-types`, `shared-utils` e o generator `shared-lib` obsoleto, deixando `libs/shared/` e `tools/generators/` como destinos vazios; a infraestrutura de release fica armada com um guard para conjunto vazio. |
| ADR-010 | Migrar a manutenção para o GitHub e o Linear | Muda a plataforma para o GitHub (`gh`, runner `ubuntu-26.04`, reutilizáveis locais), publica as libs no GitHub Packages com escopo `@mateusmacedo` num job encadeado ao release e adota o Linear como tracker; supersede a ADR-005 e partes das ADRs 004 e 008. |
| ADR-011 | O template consome o DMPF por versão | Versiona a saída do `init` do `@mateusmacedo/dmpf-plugin` fixado no `catalog:`, autentica o GitHub Packages por npmrc de usuário, prova o template com o `template-smoke`, deixa o release sem grupos até o primeiro membro e mantém o `dmpf-ci.yml` fora dos checks obrigatórios. |
| ADR-012 | Implantar um app por bounded context sobre o kernel DMPF | Ratifica o app por contexto do kernel (banco, role, contrato e deploy próprios; REST só no `bff`, contextos em gRPC) e descarta o monolito modular do design. |
| ADR-013 | Isolar tenants pelo `ExecutionContext` no `postgres.Table`, com RLS só como segunda camada | Adota o choke point do kernel para o escopo de tenant; acesso e projeção entre tenants, entregues no kernel `1.0.0-rc.6` (SPEC-QVDGV6J0, ADR-068 do kernel), dependem da adoção da rc.6 (DEVS-136). |
| ADR-014 | Persistir em PostgreSQL pelo `postgres.Table`, com persistência híbrida | Ratifica PostgreSQL por app com colunas tipadas só para filtro e `snapshot` `jsonb`; o doc 90 vale como modelo lógico. |
| ADR-015 | Adotar Kafka como transporte-alvo dos eventos de domínio, com decisão registrada por canal | Kafka (MSK na AWS, Redpanda em dev e hmg) com critério de TRP-37 por canal e SNS/SQS nos casos de TRP-40; operação pendente da revisão de TRP-41. |
| ADR-016 | Adotar Keycloak 26.x autogerenciado como provedor de identidade OIDC | Realm único com Organizations; condições: mapper de tenant, deny-list de sessão para revogação em 60 s e origem das permissões efetivas. |
| ADR-017 | Versionar o portfólio por snapshot imutável com hash do conjunto e política de materialidade versionada | Fixa o mecanismo de versão do portfólio; a lista de campos fica como política v0 até as lacunas de identidade e campos canônicos. |
| ADR-018 | Garantir a imutabilidade do Compliance Ledger em camadas — permissão, hash encadeado e selo em WORM | Role de escrita separada, hash encadeado com RFC 8785 e selo Merkle assinado em AWS KMS e gravado em S3 Object Lock. |
| ADR-019 | Construir em Go sobre o kernel DMPF, com AWS como nuvem-alvo e floci como emulador | Ratifica Go e o deploy por app do kernel e fixa AWS e floci; front-end e produção ficam pendentes. |
| ADR-020 | Reter o inbox pelo menos pela janela de redelivery declarada por canal | `retenção_inbox ≥ janela_redelivery` (INB-14) com fórmula por canal (TRP-23/24/24b), zonas de replay e purga com evidência. |
| ADR-021 | Testar com dados sintéticos e simulador de parceiro, sem dado real fora de produção | Fases 0 e 1 com sintéticos, property-based, simulador e goldens, regras R1 a R7; homologação sintética; fase 2 pendente. |
| ADR-022 | Fazer as normas do kernel DMPF prevalecerem sobre os documentos de design | Regra de precedência e tabela das 18 divergências entre `docs/design/` e o kernel, com o ADR ou a spec que trata cada uma. |
