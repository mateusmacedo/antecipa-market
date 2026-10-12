# Design — Monetização e Faturamento

> **Contexto novo**: não existe PRD correspondente.
> **Camada**: construtiva
> **Status**: proposta. Depende de decisões comerciais ainda não tomadas.

## 1. Por que este contexto precisa existir

`13-prd-partner-management.md` modela `CommercialRule` — "remuneração e impacto
permitido na apresentação" — e estabelece a invariante "regra comercial não pode
alterar oferta do financiador". `07-prd-marketplace.md` exige que a remuneração
seja visível no ranking. Nenhum dos dois diz:

- quem paga;
- quanto;
- em que momento o direito à remuneração nasce;
- como se fatura;
- qual o tratamento tributário.

Sem isso: o campo `platformRemuneration` de `PricingBreakdown`
(`90-modelo-de-dados.md`, 3.3) não tem de onde ser calculado, o evento
`OffersPresented` não tem o que divulgar, e a viabilidade do negócio não é
verificável.

Há também uma consequência de conformidade: se a plataforma é remunerada pelo
financiador e ordena as ofertas, existe conflito de interesse estrutural. A
divulgação exigida pelo PRD só é implementável se a regra comercial for modelada
com precisão.

## 2. Decisões comerciais pendentes

Este documento **não** decide. Explicita as opções e as consequências
construtivas, para que a decisão seja tomada com o custo visível.

### 2.1 Quem paga

| Modelo | Como funciona | Consequência |
| --- | --- | --- |
| **Cedente paga** | Tarifa deduzida do valor líquido, ou cobrada à parte | Conflito de interesse menor — a plataforma otimiza para o cliente. Reduz o líquido recebido, competindo com a taxa do financiador. Exige aparecer no `PricingBreakdown` como item próprio |
| **Financiador paga** | Comissão sobre operação originada | Conflito de interesse direto: a plataforma ordena ofertas de quem a remunera. Exige divulgação e controles reforçados |
| **Ambos** | Combinação | Soma os dois conjuntos de exigências |
| **Assinatura** | Valor fixo periódico por cedente ou financiador | Sem conflito por operação. Receita previsível; adoção mais difícil |
| **Misto** | Assinatura mais sucesso | Comum; exige os dois mecanismos |

**Impacto no ranking**: se o financiador paga e a comissão varia entre parceiros,
a plataforma tem incentivo mensurável para ordenar a favor do que paga mais.
`07-design-marketplace.md`, R6 propõe que patrocínio não inverta mérito — essa
decisão perde força se a remuneração variável por parceiro não for divulgada
operação a operação.

**Controle mínimo, caso a comissão varie por parceiro**: a comissão é exibida por
oferta no momento da apresentação, e a ordenação sem consideração comercial é
registrada em `OffersPresented` (controle R5). Sem ambos, a invariante do PRD é
declarativa.

### 2.2 Base de cálculo

| Base | Observação |
| --- | --- |
| Percentual do valor de face cedido | Simples; desalinha a plataforma do custo real ao cliente |
| Percentual do valor líquido desembolsado | Alinha ao que o cliente recebe |
| Percentual do custo da operação (deságio + tarifas) | Alinha ao valor gerado; mais difícil de explicar |
| Valor fixo por operação | Previsível; regressivo para operação pequena |
| Escalonado por volume | Comum; exige apuração por período |

### 2.3 Momento do fato gerador

| Momento | Risco |
| --- | --- |
| Apresentação de ofertas | Receita sem operação concluída; incentivo perverso |
| Seleção da oferta | Operação pode não se concretizar |
| Assinatura do contrato | Efeito pode ser rejeitado |
| **Confirmação do efeito** | Alinhado ao valor entregue |
| **Confirmação do desembolso** | Mais conservador; alinhado ao resultado do cliente |
| Liquidação | Muito tardio; descasa receita e custo |

**Recomendação**: confirmação do desembolso. É o instante em que o cliente
efetivamente obteve o que buscou, e elimina qualquer incentivo a favorecer
operações que não se concretizam. Custo: ciclo de receita mais longo.

## 3. Modelo de domínio

| Agregado | Papel |
| --- | --- |
| `PricingPlan` | Plano versionado: modelo, base, alíquotas, faixas, vigência |
| `PlanAssignment` | Vínculo de plano a organização, com vigência |
| `BillableEvent` | Fato gerador registrado, com base de cálculo congelada |
| `Charge` | Cobrança calculada, com componentes e tributos |
| `Invoice` | Agrupamento por período e sacado |
| `InvoiceLine` | Linha rastreável à operação de origem |
| `TaxRule` | Regra tributária por natureza e localidade |

### 3.1 `BillableEvent`

| Campo | Observação |
| --- | --- |
| `id` | |
| `sourceEventId` | Evento de domínio que o originou |
| `payerOrganizationId` | Quem paga |
| `planAssignmentId` | Plano vigente no momento do fato |
| `calculationBasis` | Valor congelado da base — não recalculado depois |
| `operationRefs` | Contrato, efeito, desembolso |
| `occurredAt` | |
| `status` | `PENDING`, `CHARGED`, `WAIVED`, `REVERSED` |

**`calculationBasis` congelado**: o valor de referência é capturado no fato
gerador. Se o desembolso é posteriormente estornado, o tratamento é `REVERSED`
com registro — nunca recálculo silencioso. Sem isso, a fatura muda depois de
emitida e a conciliação com o cliente se torna impossível.

## 4. Fluxo

```text
Evento de domínio (ex.: DisbursementConfirmed)
  -> Avaliação de aplicabilidade (plano vigente, isenção, franquia)
  -> BillableEvent com base congelada
  -> Cálculo de Charge (componentes + tributos)
  -> Acúmulo no período
  -> Fechamento e emissão de Invoice
  -> Cobrança
  -> Conciliação do recebimento
```

**Isolamento obrigatório**: este contexto consome eventos e não participa do fluxo
de operação. Falha no faturamento nunca bloqueia contratação ou desembolso —
seria transformar problema de cobrança em problema de crédito para o cliente.

## 5. Tributação

Lacuna que exige apuração externa — ver `docs/reference/regulacao-brasil.md`.

| Questão | Por que importa |
| --- | --- |
| Natureza do serviço da plataforma | Determina o enquadramento no ISS e a alíquota |
| Município de incidência | Local do prestador ou do tomador, conforme a natureza |
| Retenções aplicáveis | Afetam o valor líquido recebido |
| Obrigação acessória | Emissão de documento fiscal por operação ou por período |
| Tratamento quando o financiador paga | Serviço prestado a instituição financeira pode ter regra específica |

**Requisito derivado**: o `Charge` separa valor do serviço, tributos retidos e
tributos incidentes. Um valor único impede emitir documento fiscal correto e
impede o cliente de escriturar.

## 6. Transparência

Decorre da invariante de `07-prd` ("remuneração comercial não pode ser escondida
no ranking").

| Requisito | Definição |
| --- | --- |
| Divulgação na apresentação | Remuneração da plataforma exibida por oferta, em valor |
| Divulgação no contrato | Consta do instrumento ou de anexo |
| Extrato | O cliente consulta o que foi cobrado, por operação |
| Rastreabilidade | Toda linha de fatura aponta para a operação de origem |
| Ausência de cobrança oculta | Nenhuma cobrança fora do plano vigente |

**Divulgação em valor e não em percentual**: "1,5% de comissão" é abstrato;
"R$ 23,40 nesta operação" é verificável. O percentual pode acompanhar, nunca
substituir.

## 7. Custo da operação (o outro lado)

`93-requisitos-nao-funcionais.md`, 11 identifica os vetores de custo. Para
avaliar margem, o contexto precisa apurá-los por operação.

| Custo | Origem |
| --- | --- |
| Consultas cadastrais | Provedor de KYB, por consulta |
| Consultas à infraestrutura | Registradora e escriturador, por consulta ou por lote |
| Registro de efeito | Tarifa da infraestrutura, quando houver |
| Assinatura eletrônica | Por envelope |
| Armazenamento de evidência | Por volume e retenção |

**Métrica derivada**: margem por operação concluída. Uma operação que consome
custo de consulta desproporcional ao ticket é deficitária mesmo com receita — e a
plataforma só descobre se apurar. Isso é informação de produto, não de
contabilidade: se operações pequenas são deficitárias, o ticket mínimo é decisão
de sobrevivência.

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| G1 | Modelo de monetização não decidido (2.1) | **Bloqueante** — nada se constrói sem |
| G2 | Base de cálculo não decidida (2.2) | Bloqueante |
| G3 | Fato gerador não decidido (2.3) | Bloqueante |
| G4 | Enquadramento tributário não apurado (5) | Alto |
| G5 | Custos unitários dos provedores desconhecidos (7) | Alto |
| G6 | Integração com emissão fiscal não definida | Médio |

**Este contexto não deve ser construído antes de G1 a G3.** Implementar um motor
de cobrança genérico para acomodar qualquer modelo é o caminho mais caro e o menos
provável de acertar.

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 1 | `13-prd-partner-management.md` | `CommercialRule` sem conteúdo |
| 2.1 | `07-prd-marketplace.md` | Conflito de interesse sem modelo |
| 3.1 | — | Base de cálculo e estorno |
| 6 | `07-prd-marketplace.md` | Divulgação exigida sem mecanismo |
| 7 | — | Custo por operação ausente |
