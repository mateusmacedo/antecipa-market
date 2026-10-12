# ADR-021: Testar com dados sintéticos e simulador de parceiro, sem dado real fora de produção

## Status

Aceito — 2026-10-12, para as fases 0 e 1 e as regras R1 a R7; a fase 2 fica em
Pendências. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D10 vem de `docs/design/00-arquitetura-tecnica.md` §11: dados
sintéticos em desenvolvimento e integração, **anonimizados** em homologação, e
credencial de produção nunca fora de produção. O próprio §11 registra que
anonimizar agenda de recebíveis preservando datas e valores "não é trivial", e
`docs/design/94-seguranca-threat-model.md` aceita "sintéticos ou anonimizados;
jamais cópia de produção". Hoje não há produção de onde anonimizar.

O que os testes precisam cobrir:

- O simulador de parceiro (`docs/design/04-design-receivables-gateway.md`,
  lacuna G12) precisa da taxonomia de erro inteira (`RETRYABLE_*`, `FINAL_*`,
  `UNKNOWN_SENT`, `UNKNOWN_AMBIGUOUS`, `SCHEMA_MISMATCH`), de parceiro sem
  consulta de estado (`stateQuerySupported=false`) e de webhook repetido, que o
  §7 deduplica pelo identificador do parceiro.
- A conciliação (`docs/design/11-design-settlement-reconciliation.md`) tem cinco
  camadas e recebíveis de mesmo valor e dia como regra.
- Documentos fiscais incluem o CNPJ alfanumérico, em vigor desde julho de 2026,
  com dígito verificador pelo método do Serpro.

O kernel DMPF só cobre a infraestrutura de teste (classe **aberta**): infra de
testes separada, só em tmpfs (ADR-054 §7); banco por teste, apagado ao fim, num
servidor com `cluster_name=test` (ADR-054 §7, que emenda o ADR-053); e o `testkit`,
com dublês de serviço com falha injetável, relógio e IDs determinísticos e, desde
o rc.4, os kits de processo e telemetria (ADR-062). O kernel não tem gerador de
dados brasileiros, property-based nem simulador de parceiro.

## Decisão

**Fase 0 (agora):**

- Builders Go de entidades com geradores brasileiros (CPF e CNPJ válidos,
  inclusive o alfanumérico, e conta bancária), validados contra o cálculo do
  Serpro.
- Geração dirigida por contrato a partir dos `.proto` e das regras de
  `protovalidate`.
- Testes property-based com `rapid` para invariantes (idempotência, não
  adivinhar correspondência na conciliação, máquinas de estado), com semente
  fixa.
- Simulador de parceiro in-process, roteirizado, com toda a taxonomia de erro do
  design 04.
- Goldens de conciliação no modelo canônico.

**Fase 1 (após o acordo com a primeira registradora):** homologação da
registradora como validação de contrato; respostas sanitizadas viram fixtures de
parser; simulador de protocolo (mTLS, arquivo, fila) com injeção de falha de rede
por Toxiproxy; os goldens ganham a ingestão real.

**Homologação usa dados sintéticos**, não anonimizados, o que diverge do doc 00
§11 (registrado no ADR-022).

**Regras:**

- **R1**: dado real é proibido em dev, integração e hmg.
- **R2**: todo gerador é determinístico, com semente registrada; a CI roda com
  semente fixa, e a exploração com semente nova roda em job separado.
- **R3**: testes com documento fiscal cobrem o CNPJ alfanumérico, e documento
  sintético nunca é enviado a parceiro real.
- **R4**: payload de homologação de parceiro é confidencial e fica fora do git
  até ser sanitizado.
- **R5**: o simulador cobre toda categoria de erro do design 04 e o parceiro sem
  consulta de estado.
- **R6**: goldens são versionados e mudam só com revisão de diff.
- **R7**: o guard `cluster_name=test` do kernel permanece ligado.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Homologação com dados anonimizados (doc 00 §11) | Não há produção para originar, e anonimizar agenda preservando datas e valores tem risco de reidentificação (LGPD art. 12) |
| Cópia de produção mascarada | Proibida pelo doc 94; pseudonimizado continua dado pessoal (LGPD art. 13 §4º) |
| Só a homologação das registradoras | Acesso, custo e representatividade desconhecidos (`B17`); não cobre a taxonomia de erro |
| Geradores aleatórios sem semente | Quebram a regra de determinismo do `testkit` e tornam falha irreproduzível |

## Consequências

**Positivas:**

- Nenhum dado pessoal real em ambiente não produtivo.
- A taxonomia de erro e o desfecho incerto ficam testáveis sem parceiro.
- Falhas de property-based são reproduzíveis pela semente.

**Negativas:**

- Realismo estatístico limitado até a fase 2; teste de performance usa
  volumetria hipotética (`#12`).
- O simulador de protocolo depende dos formatos reais das registradoras.

## Pendências

- **Fase 2** (cópia anonimizada de produção): só com gatilho e parecer jurídico,
  preferindo estatística agregada e regeneração pelo domínio ao mascaramento.
- **Homologação das registradoras**: acesso e custo (`B17`).
- **Formatos**: `#2` a `#5`, as perguntas `B4`, `B7`, `B8` e a lacuna R10 do
  design 11 (formato dos arquivos de liquidação), que decidem o
  simulador de protocolo e a ingestão nos goldens.

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- [ADR-022](./022-precedencia-dmpf-sobre-design.md)
- `docs/README.md`, "Lacunas bloqueantes"; `docs/resumo-executivo-apuracao.md`
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`):
  - ADR-054 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/054-apps-autocontidos-e-infras-separadas.md`
  - ADR-062 — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/docs/adr/062-promover-ao-testkit-os-kits-de-processo-telemetria-e-execucao.md`
  - `testkit` — `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/libs/backend/go/testkit/README.md`
- Cálculo do DV do CNPJ alfanumérico (Serpro) — `https://serpro.gov.br/menu/noticias/videos/calculodvcnpjalfanaumerico.pdf`
- LGPD — `https://www.planalto.gov.br/ccivil_03/_ato2015-2018/2018/lei/l13709.htm`
