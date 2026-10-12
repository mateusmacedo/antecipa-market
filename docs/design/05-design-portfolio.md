# Design — Portfolio

> **PRD de origem**: `docs/prd/05-prd-portfolio.md`
> **Camada**: construtiva
> **Status**: proposta
> **Complementa**: `90-modelo-de-dados.md` 3.2 (atributos e `contentHash`),
> `93-requisitos-nao-funcionais.md` 2 (volumetria e particionamento). Não
> repetidos aqui.

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| P1 | "Alteração material" sem definição | Req. 2 |
| P2 | Política de cálculo de disponibilidade sem fórmula | Invariante `asOf`/fonte/política |
| P3 | "Marcar dados vencidos conforme SLA da fonte" sem SLA | Req. 5 |
| P4 | Campos canônicos dos ativos incompletos | Seção "Tipos de ativo" |
| P5 | Identidade de duplicata e de UR "seguem esquemas distintos" sem especificar | Invariante |
| P6 | Divergência entre projeção e posição externa sem procedimento | Req. 9 |
| P7 | Comparação entre snapshots sem semântica de diferença | Req. 3 |

## 2. Alteração material

Define quando se cria versão — e, com isso, o custo de armazenamento do sistema
inteiro (`93-requisitos-nao-funcionais.md`, 2.2).

**Definição proposta**: entram no `contentHash` do ativo apenas os campos que
podem alterar decisão de antecipação.

| Campo | Entra no hash | Razão |
| --- | --- | --- |
| Identificador do ativo | sim | Identidade |
| Valor líquido esperado | sim | Muda a operação |
| Data de liquidação | sim | Muda prazo e preço |
| Titular | sim | Muda quem pode ceder |
| Situação | sim | Muda disponibilidade |
| Gravames conhecidos | sim | Muda disponibilidade |
| Valor bruto | sim | Base de cálculo |
| Ajustes | sim | Alteram o líquido |
| Credenciador, arranjo | sim | Afetam elegibilidade |
| `asOf` da consulta | **não** | Mudaria a cada consulta |
| Metadados de paginação, ordem | **não** | Irrelevantes |
| Campos informativos sem efeito em decisão | **não** | — |

**Hash do snapshot**: sobre o conjunto ordenado dos hashes dos ativos. Conjunto
idêntico produz hash idêntico, e nenhuma versão é criada.

**Consequência**: consulta que retorna exatamente a mesma posição não gera
snapshot novo — apenas atualiza o `asOf` do snapshot vigente e registra a consulta.
A distinção entre "quando consultamos" e "o que a posição era" fica explícita.

## 3. Disponibilidade

O PRD exige que a disponibilidade sempre tenha `asOf`, fonte e política de
cálculo, sem fornecer política.

```text
availableAmount =
  se encumbranceCoverage != COMPLETE          -> nulo
  se situação != disponível                    -> zero
  senão                                        -> netAmount
                                                - soma(gravames conhecidos)
                                                - soma(reservas internas ativas)
```

| Situação | `availableAmount` | Razão |
| --- | --- | --- |
| Cobertura de gravame incompleta | **nulo** | Não se sabe; nulo força tratamento |
| Ativo liquidado ou baixado | zero | — |
| Ativo com gravame total | zero | — |
| Ativo com gravame parcial | Líquido menos gravame | — |
| Ativo reservado internamente | Reduzido pela reserva | Reserva é interna, mas real para a plataforma |

**Nulo e não valor otimista** é a decisão central, e implementa a invariante
"ausência de informação não significa ausência de gravame". Um campo nulo obriga o
consumidor a decidir; um campo preenchido com o líquido integral leva silenciosamente
a operar sobre ativo comprometido.

**A disponibilidade é derivada e não autoritativa** (`AvailabilityProjection`, no
PRD). Serve para exibir e filtrar; nunca para decidir registro de efeito — que
exige revalidação junto à fonte (`09-design-effects-orchestrator.md`, 2).

## 4. Frescor

Req. 5 exige marcar dados vencidos conforme SLA da fonte, sem SLA.

```text
staleAfter = sourceAsOf + capability.dataQuality.positionFreshnessSla
```

O prazo vem do `CapabilityProfile` do parceiro
(`04-design-receivables-gateway.md`, 2.1) — é propriedade da fonte, não da
plataforma.

| Estado | Efeito |
| --- | --- |
| Fresco | Uso normal |
| Vencido | `AssetBecameStale`; elegibilidade reprova (`AST-FRS-001`); exibição com ressalva |
| Sem SLA declarado | Prazo conservador padrão; alerta de configuração |

**Recebível vencido ou desatualizado nunca aparece como livre sem ressalva**
(critério de aceitação). Na interface, isso é indicação visível de que a posição é
de determinado instante e pode ter mudado — não uma nota de rodapé.

## 5. Identidade dos ativos

A invariante diz que duplicata e unidade de recebível "seguem esquemas distintos"
sem especificar. Ambos dependem de apuração normativa — ver
`docs/reference/regulacao-brasil.md`. Estrutura:

| Tipo | Componentes prováveis de identidade | Fonte da identidade |
| --- | --- | --- |
| `BookEntryInvoice` | Identificador atribuído pelo sistema de escrituração | Escriturador |
| `CardReceivableUnit` | Composição de titular, arranjo, credenciador e data de liquidação, conforme padrão regulatório | Registradora |

**A unidade de recebível pode não ter identificador atômico estável**: se a
identidade é composta e a agenda se reorganiza, a correspondência entre versões
exige regra própria. Isso afeta diretamente a comparação entre snapshots (seção 6)
e a conciliação (`11-design-settlement-reconciliation.md`, 3).

**Lacuna bloqueante**: sem o esquema real de identidade, não é possível garantir
que a comparação entre versões de snapshot esteja correta.

## 6. Comparação entre snapshots

Req. 3 exige comparação, sem semântica.

```text
diff(snapshotA, snapshotB) =
  ├─ added[]      presentes em B, ausentes em A
  ├─ removed[]    presentes em A, ausentes em B
  ├─ changed[]    presentes em ambos, com hash diferente
  │    └─ por ativo: campos alterados, com valor anterior e novo
  └─ unchanged    contagem
```

| Categoria | Significado provável |
| --- | --- |
| `added` | Nova transação ou novo recebível |
| `removed` | Liquidado, cancelado, ou **saiu do escopo da consulta** |
| `changed` com valor menor | Ajuste, estorno ou chargeback |
| `changed` com novo gravame | Efeito de terceiro — **sinal de dupla negociação** |
| `changed` de titularidade | Cessão ocorrida fora da plataforma |

**`removed` é ambíguo e perigoso.** Ativo ausente pode ter liquidado ou ter saído
da janela consultada. A distinção exige que a consulta declare sua janela e que a
comparação considere apenas a interseção das janelas. Comparar consultas de
janelas diferentes produz falsos `removed`.

**`changed` com novo gravame de terceiro é sinal de risco**, não apenas atualização:
o cedente comprometeu o ativo em outro lugar. Deve gerar `ExternalPositionDiverged`
e alimentar o contexto de risco.

## 7. Divergência com a posição externa

Req. 9 exige expor divergências, sem procedimento.

| Divergência | Severidade | Ação |
| --- | --- | --- |
| Projeção de gravame local não existe na fonte | Alta | Efeito pode não ter se constituído; verificar |
| Gravame na fonte desconhecido localmente | **Crítica** | Efeito de terceiro; possível dupla negociação |
| Titular diverge | **Crítica** | Cessão fora da plataforma; disputa |
| Valor diverge além da tolerância | Média | Ajuste não capturado |
| Situação diverge | Média | Atualização não propagada |

**A fonte prevalece sempre.** A projeção local é corrigida, nunca o contrário. A
correção gera evento e, nas severidades crítica e alta, caso operacional.

**Divergência crítica sobre ativo com efeito vigente da plataforma escala para
disputa** (`09-design-effects-orchestrator.md`, 8), não apenas para operações.

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| P8 | Esquema de identidade dos ativos (5) | **Bloqueante** |
| P9 | Campos canônicos conforme padrão regulatório | **Bloqueante** |
| P10 | SLA de frescor por fonte (4) | Alto |
| P11 | Tolerância de valor na comparação (7) | Médio |
| P12 | Retenção de snapshot não referenciado | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | 2 | "Alteração material" sem definição |
| 3 | Invariante | Política de disponibilidade sem fórmula |
| 4 | 5 | SLA sem origem |
| 5 | Invariante | Esquemas de identidade não especificados |
| 6 | 3 | Comparação sem semântica |
| 7 | 9 | Divergência sem procedimento |
