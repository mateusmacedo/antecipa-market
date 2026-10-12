# ADR-017: Versionar o portfólio por snapshot imutável com hash do conjunto e política de materialidade versionada

## Status

Aceito — 2026-10-12, para o mecanismo; a lista de campos é política v0 (ver
Pendências). Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D6 vem de `docs/design/00-arquitetura-tecnica.md` §5.3: versionar o
portfólio só em mudança material, comparar por hash do conteúdo normalizado e
reter integralmente só o snapshot referenciado por oferta, contrato ou efeito. O
PRD de portfólio pede nova versão "quando houver alteração material" sem
defini-la. `docs/design/05-design-portfolio.md` §2 propõe a lista de campos de decisão
e o hash do snapshot sobre o conjunto ordenado dos hashes dos ativos;
`docs/design/93-requisitos-nao-funcionais.md` fixa RNF-V1 a RNF-V5 (gate por
hash, limite de frequência, simulação não publica, partição por data de
liquidação, retenção reduzida do não referenciado).

O kernel não tem norma sobre o tema (classe **aberta**). A análise encontrou
lacunas no próprio design:

- **Identidade × conteúdo**: o §2 põe titular, data de liquidação, credenciador
  e arranjo no hash, mas esses atributos podem definir a própria unidade de
  recebível (lacunas `#3` e `#4`); mudá-los seria outra unidade, não versão nova.
- **Gate sem escopo**: um único "último hash" compara descobertas de janelas e
  fontes diferentes.
- **Confirmação sobrescrita**: a confirmação de que a posição não mudou atualiza
  o snapshot vigente e perde a trilha.
- **Materialidade misturada com tolerância**: versionar e alertar usam o mesmo
  critério.
- **Custo**: o `postgres.Table` grava um agregado por operação e filtra por uma
  coluna (ADR-014); a volumetria é hipótese (N1, `#12`).

Estimativas da fase 3 (2.000 organizações, 3 descobertas por dia, 2.000 ativos,
linha de ativo de 1,5 KB, não referenciado retido 7 dias e referenciado 5 anos):
cópia completa por sincronização grava 17,2 GB/dia; cópia por snapshot com gate
de hash, 10,3 GB/dia; manifesto com versão por ativo, 1,5 GB/dia. Com retenção
reduzida do não referenciado, o estoque é dominado pelos snapshots
referenciados (4,1 TB em 5 anos com cópia por snapshot, 1,3 TB com manifesto). No
MVP, tudo cabe em menos de 60 GB.

## Decisão

1. **Versão é um snapshot imutável** cujo `contentHash` cobre o conjunto de pares
   `(assetKey, assetHash)` e a `materialityPolicyVersion`. Mudar a lista de
   campos cria nova versão de política, sem migrar snapshots antigos.
2. **O gate compara só com o último snapshot publicado do mesmo escopo** (tipos
   de ativo, horizonte e fontes).
3. **Confirmação é append-only** (`DiscoveryConfirmation`); o frescor é o máximo
   das confirmações mais o SLA, e o snapshot não é alterado.
4. **Versionar é mudança exata** nos campos de decisão; tolerância governa só
   invalidação e alerta.
5. **Campos de identidade compõem o `assetKey`** e ficam fora do hash de
   conteúdo.
6. **Simulação nunca publica** (RNF-V3), e a retenção segue a classe do snapshot
   (RNF-V5).
7. **Representação física reversível**: na fase 1, cópia por snapshot, de
   preferência como documento único por snapshot; a migração para manifesto com
   versão por ativo é pré-aprovada e dispara por gatilho — estoque de
   referenciados acima de 1 a 2 TB em 5 anos ou mais de ~50 `RecordAsset`/s.
   Pelas estimativas acima, o gatilho deve disparar na fase 3; a fase 1 não
   o atinge.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Snapshot completo a cada sincronização | 17,2 GB/dia na fase 3 e invalidação em massa de elegibilidade sem mudança real |
| Log de mudanças com checkpoint | Reprodutibilidade depende de replay e upcasting; o kernel não oferece consulta temporal; sensível a `#3` e `#4` |
| Versão por ativo, com manifesto só se referenciado | "O que o usuário viu" só existe se materializado no instante da oferta |
| Manifesto com versão por ativo desde já | Complexidade média-alta (publicação atômica, coleta de lixo) antes de volumetria medida; fica como evolução por gatilho |

## Consequências

**Positivas:**

- A versão referenciada por oferta é imutável e autocontida.
- O gate corta cerca de 40% da gravação e dá estabilidade semântica à
  referência.
- A política de materialidade evolui sem migração.

**Negativas:**

- Na fase 1, cada snapshot copia todos os ativos.
- Documento único por snapshot com 5.000 ativos pode passar de 4 MB numa
  mensagem gRPC, o limite padrão do gRPC-Go (não verificado no kernel).

## Pendências

- **Lista de campos de decisão (política v0)**: valor líquido esperado, valor
  bruto, ajustes, situação e gravames conhecidos, do
  `docs/design/05-design-portfolio.md` §2. Titular, data de liquidação,
  credenciador e arranjo compõem o `assetKey` provisório da v0: mudar um deles
  muda o par `(assetKey, assetHash)` e, com ele, o `contentHash` do snapshot, então
  nenhuma alteração passa sem versão. A fronteira definitiva entre identidade e
  conteúdo depende de `#3` (esquema de identidade da unidade de recebível) e `#4`
  (campos canônicos dos ativos), e das perguntas `B4` e `B7` às registradoras. O
  enquadramento do art. 3º §2º da Res. BCB 264 em
  `docs/reference/regulacao-brasil.md` precisa ser conferido no texto original
  antes de fechar `#3`.
- **Custo**: volumetria `#12` e N1, que decidem o gatilho do item 7.
- A implementação é da SPEC-6E7BKFNS (Portfolio).

## Referências

- SPEC-GXGASP24 — `docs/specs/SPEC-GXGASP24-fundacao-arquitetural-adrs.md`
- SPEC-6E7BKFNS — `docs/specs/SPEC-6E7BKFNS-portfolio.md`
- [ADR-014](./014-postgresql-persistencia-hibrida.md)
- `docs/README.md`, "Lacunas bloqueantes" — `#3`, `#4` e `#12`
- `docs/resumo-executivo-apuracao.md` — perguntas `B4` e `B7`
