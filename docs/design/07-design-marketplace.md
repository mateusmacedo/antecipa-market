# Design — Marketplace

> **PRD de origem**: `docs/prd/07-prd-marketplace.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| M1 | "Métricas comparáveis" sem definição de quais | Req. 4 |
| M2 | "Ordenação auditável" sem algoritmo | Req. 6 e invariante sobre patrocínio |
| M3 | Comparação entre ofertas de escopos diferentes não tratada | Ausente |
| M4 | Distribuição aos financiadores sem política | Req. 2 |
| M5 | "Silêncio" do parceiro sem definição | Req. 9 |
| M6 | Contraproposta mencionada e não modelada | Req. 9 |
| M7 | Encerramento de convites remanescentes sem regra | Req. 10 |
| M8 | Conflito de interesse tratado como invariante, sem controle | Invariante sobre remuneração |

## 2. O problema central: comparabilidade

O PRD estabelece que a plataforma calcula "métricas comparáveis sem reprecificar a
operação". A tensão é real: ofertas chegam com estruturas diferentes — uma cobra
deságio puro, outra deságio menor com tarifa fixa, uma terceira cobre menos ativos
com prazo médio distinto. Somar e ordenar sem cuidado produz ranking enganoso.

### 2.1 O que é comparável e o que não é

| Situação | Comparável por | Observação |
| --- | --- | --- |
| Mesmo conjunto de ativos, estruturas de preço diferentes | Valor líquido e taxa efetiva | Caso simples |
| Conjuntos diferentes de ativos | **Somente taxa efetiva** | Valor líquido absoluto é enganoso |
| Prazos médios diferentes | **Somente taxa efetiva** | Deságio maior pode custar menos ao ano |
| Com e sem coobrigação | **Não diretamente comparáveis** | Produtos distintos; risco distinto |
| Com e sem garantia adicional | **Não diretamente comparáveis** | Idem |

**Consequência de desenho**: a interface não apresenta uma lista única ordenada
quando as ofertas não são comparáveis entre si. Apresenta grupos comparáveis, com
a diferença estrutural explicitada. Ordenar por valor líquido ofertas que cobrem
conjuntos diferentes de ativos é induzir a erro — a maior pode custar mais por
real antecipado.

**Isso não está no PRD e é a lacuna de produto mais relevante do contexto.**

### 2.2 Métrica primária

Definida em `90-modelo-de-dados.md`, 3.3:

```text
effectiveMonthlyRate = (grossAmount / netAmount)^(30 / weightedAverageTermDays) - 1
```

| Componente | Definição |
| --- | --- |
| `grossAmount` | Soma do valor de face dos ativos cobertos pela oferta |
| `netAmount` | Valor que o cedente efetivamente recebe, após deságio, tarifas e tributos |
| `weightedAverageTermDays` | Média dos prazos por ativo, ponderada pelo valor de face |
| Prazo por ativo | Dias entre a data prevista de crédito e a data de liquidação do ativo |

**Métricas secundárias apresentadas junto**:

- valor líquido em reais;
- custo total em reais (`grossAmount - netAmount`);
- prazo médio ponderado em dias;
- quantidade e valor dos ativos cobertos;
- custo por real antecipado (`totalCost / netAmount`) — útil quando as coberturas
  diferem.

**A plataforma não calcula probabilidade de aprovação, não estima preço e não
recomenda.** Qualquer uma dessas atividades a colocaria em papel de assessoria,
contrariando o não objetivo "decidir ou conceder crédito".

## 3. Ordenação

### 3.1 Estrutura

```text
RankingPolicy
  ├─ policyId, version
  ├─ criteria[]: { field, direction, weight }
  ├─ grouping: como agrupar ofertas não comparáveis
  ├─ sponsoredPlacement: { enabled, maxPositions, disclosureRequired }
  └─ tieBreaker[]
```

### 3.2 Política padrão proposta

| Ordem | Critério | Direção |
| --- | --- | --- |
| 1 | Grupo de comparabilidade | Grupo de cobertura integral primeiro |
| 2 | `effectiveMonthlyRate` | Crescente |
| 3 | `netAmount` | Decrescente |
| 4 | `validUntil` | Decrescente |
| 5 | `receivedAt` | Crescente |

**Por que taxa efetiva antes de valor líquido**: valor líquido maior sobre
cobertura maior não significa melhor negócio. A taxa normaliza. O valor líquido
entra como desempate e é exibido com destaque igual — a decisão é do cliente, e
frequentemente ele prefere mais dinheiro a taxa menor.

**`receivedAt` como último desempate**: garante determinismo. Ordenação não
determinística impede reprodução exata, exigida pelo critério de aceitação.

### 3.3 Patrocínio

O PRD admite patrocínio ("inclusive quando houver patrocínio") e proíbe ocultá-lo.
Controles construtivos:

| # | Controle |
| --- | --- |
| R1 | Posição patrocinada é marcada visualmente e no payload (`sponsored: true`) |
| R2 | Remuneração da plataforma é exibida em valor, não apenas sinalizada |
| R3 | Patrocínio não altera o cálculo das métricas — só a posição |
| R4 | Número máximo de posições patrocinadas declarado na política |
| R5 | A ordenação sem patrocínio é registrada em `OffersPresented`, junto com a exibida |
| R6 | Oferta patrocinada não ocupa a primeira posição quando houver oferta com taxa melhor — ver abaixo |

**R5 é o controle que torna a invariante verificável.** Registrar as duas
ordenações permite a qualquer auditoria medir exatamente quanto o patrocínio
deslocou cada oferta. Sem isso, "auditável" significa apenas "registrado".

**R6 é decisão de produto, não de engenharia.** Permitir que patrocínio supere a
melhor oferta cria conflito direto com o objetivo "permitir competição entre
financiadores com ofertas comparáveis" (`00-prd`, objetivo 2). A recomendação é
restringir patrocínio a destaque visual e desempate, nunca a inversão de mérito.
Precisa de decisão explícita — e a decisão contrária deve ser consciente.

### 3.4 Reprodutibilidade

`OffersPresented.v1` (ver `92-schemas-eventos.md`, 3.3) carrega `rankingPolicyId`,
`rankingPolicyHash`, `calculationMethodId`, a posição de cada oferta, o
`rankingScore` e as ofertas excluídas com motivo.

**Teste de reprodutibilidade**: dado o evento, recalcular a ordenação com a
política referenciada produz a mesma sequência. Automatizável e executado em CI a
cada mudança de política.

## 4. Distribuição

O PRD diz "distribuição controlada aos financiadores autorizados" (escopo) e
"enviar a cada financiador somente dados autorizados" (req. 2), sem política.

### 4.1 Modos

| Modo | Comportamento | Quando usar |
| --- | --- | --- |
| `ALL_AUTHORIZED` | Todos os financiadores com capacidade compatível e consentimento | Padrão |
| `SELECTED` | Apenas os escolhidos pelo cedente | Cliente com relação preexistente |
| `WAVES` | Grupos sucessivos, com intervalo | Preservar relacionamento ou limitar exposição |

**Sobre `WAVES`**: tem apelo comercial e um efeito colateral sério — o cedente que
aceita a oferta da primeira onda nunca sabe se a segunda seria melhor. Isso
contraria o objetivo de competição. Se adotado, a política precisa ser
transparente ao cedente e a decisão de aceitar antes do fim da janela precisa ser
informada.

### 4.2 Elegibilidade do financiador ao convite

Um financiador só é convidado se, cumulativamente:

1. `PartnerAgreement` ativo e produto vigente;
2. capacidade habilitada para o tipo de ativo e operação;
3. parâmetros do produto compatíveis com o lote (faixa de valor, prazo, arranjo);
4. consentimento de compartilhamento do cedente inclui aquele financiador;
5. sem suspensão vigente;
6. não reprovou o mesmo cedente em janela recente — ver abaixo.

**Item 6, ausente do PRD**: convidar repetidamente um financiador que reprovou o
cedente há dois dias gera ruído para ele e expectativa falsa para o cliente. A
janela de supressão é configurável, e a reprovação por motivo cadastral tem janela
maior que a por motivo de apetite.

**Cuidado**: a supressão não pode se transformar em decisão de crédito da
plataforma. É controle operacional de ruído, e o financiador pode desativá-la.

### 4.3 Estados do convite

```text
CREATED -> SENT -> DELIVERED -> [OFFERED | DECLINED | EXPIRED]
```

| Estado | Significado |
| --- | --- |
| `SENT` | Transmitido |
| `DELIVERED` | Confirmado pelo parceiro (leitura da fila ou confirmação de webhook) |
| `OFFERED` | Oferta recebida |
| `DECLINED` | Recusa explícita, com motivo |
| `EXPIRED` | Janela encerrada sem resposta |

**"Silêncio" (req. 9) é `EXPIRED`, distinto de `DECLINED`.** A diferença importa:
silêncio recorrente indica problema de integração ou desinteresse estrutural;
recusa explícita traz motivo. Métricas separadas, tratamentos separados.

**Convite não entregue não falha a solicitação** — os demais prosseguem
(`93-requisitos-nao-funcionais.md`, 4.3). Um financiador indisponível não pode
derrubar o marketplace.

## 5. Contraproposta

O PRD menciona registro de contraproposta (req. 9) sem modelá-la.

**Definição proposta**: contraproposta é oferta que cobre subconjunto dos ativos
ou altera condição estrutural do pedido. Não é negociação iterativa de preço.

| Aspecto | Definição |
| --- | --- |
| Representação | `FinancingOffer` com `coveredAssetIds` subconjunto e `isCounterOffer: true` |
| Comparabilidade | Grupo separado (ver 2.1) |
| Aceitação | Cria contrato apenas sobre os ativos cobertos; os demais permanecem disponíveis |
| Reserva | Libera os ativos não cobertos ao selecionar |

**O que não se faz**: rodada de negociação com mensagens entre cedente e
financiador. Isso transformaria a plataforma em intermediária de negociação, com
implicações de papel que `00-prd` evita deliberadamente. Se o financiador quer
ofertar diferente, envia outra oferta.

## 6. Ciclo de vida do pedido

```text
DRAFT -> OPEN -> COLLECTING_OFFERS -> OFFERS_AVAILABLE -> OFFER_SELECTED -> CONTRACTING -> CLOSED
```

### 6.1 Transições e gatilhos

| De | Para | Gatilho | Efeito colateral |
| --- | --- | --- | --- |
| `DRAFT` | `OPEN` | Criação confirmada | Cria reservas |
| `OPEN` | `COLLECTING_OFFERS` | Primeiro convite entregue | — |
| `COLLECTING_OFFERS` | `OFFERS_AVAILABLE` | Primeira oferta válida | Notifica o cedente |
| `OFFERS_AVAILABLE` | `OFFER_SELECTED` | Seleção com revalidação aprovada | Congela demais ofertas |
| `OFFER_SELECTED` | `CONTRACTING` | Caso de contrato criado | — |
| `CONTRACTING` | `CLOSED` | Contrato assinado ou cancelado | Libera reservas não usadas |
| qualquer | `ELIGIBILITY_INVALIDATED` | `EligibilityInvalidated` | Encerra convites; comunica |
| `COLLECTING_OFFERS` | `NO_OFFER` | Janela encerrada sem oferta válida | Libera reservas; comunica |
| qualquer | `CANCELLED` | Cancelamento pelo cedente | Libera reservas; notifica convidados |

### 6.2 Encerramento de convites remanescentes

O PRD diz "encerrar convites remanescentes após contratação, quando aplicável"
(req. 10) sem definir quando se aplica.

**Proposta**:

| Situação | Convites remanescentes |
| --- | --- |
| Oferta selecionada cobre todos os ativos | Encerrados imediatamente na seleção |
| Oferta selecionada cobre subconjunto | Mantidos para os ativos não cobertos, se a janela permitir |
| Contratação cancelada antes da assinatura | Reabertos, se a janela permitir e as ofertas seguirem válidas |
| Contrato assinado | Todos encerrados |

**Encerrar na seleção e não na assinatura**: manter convites abertos após a
seleção significa receber ofertas para ativos já comprometidos. O custo é que, se
a contratação falhar, é preciso reabrir — tratado pela terceira linha.

**Notificação ao financiador não selecionado**: informa que o pedido foi encerrado
e se houve contratação, sem revelar condições da oferta vencedora. Revelar preço
de concorrente é vazamento (ver `94-seguranca-threat-model.md`, T4).

## 7. Validação da oferta na recepção

A plataforma valida sem reprecificar:

| Verificação | Rejeita se |
| --- | --- |
| Consistência aritmética | `netAmount` não corresponde aos componentes |
| Cobertura | Ativo fora do lote do pedido |
| Versão | Responde a versão desatualizada do pedido |
| Validade | `validUntil` no passado, ou além do máximo permitido |
| Capacidade | Produto expirado ou capacidade suspensa |
| Duplicidade | Mesma chave idempotente com corpo diferente |
| Completude | Ausência de campo obrigatório de precificação |

**Não se valida**: se o preço é razoável, se está acima ou abaixo do mercado, se o
financiador tem capacidade financeira. Nada disso é papel da plataforma.

**Oferta rejeitada gera `OfferRejectedAsInvalid`** com o motivo, visível ao
financiador. Rejeitar em silêncio produz suporte e desconfiança.

## 8. Lacunas remanescentes

| # | Lacuna | Impacto |
| --- | --- | --- |
| M9 | Apresentação de grupos não comparáveis precisa de validação com usuário | Alto |
| M10 | Política de patrocínio exige decisão de produto (3.3, R6) | Alto |
| M11 | Obrigatoriedade de CET para tomador PJ | Médio — ver `docs/reference/regulacao-brasil.md` |
| M12 | Janelas de supressão por reprovação (4.2) sem parâmetro | Baixo |
| M13 | Formato do documento de oferta (PDF, estruturado, ambos) | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | 4 | Métricas comparáveis sem definição |
| 2.1 | — | Comparabilidade entre escopos diferentes |
| 3 | 6 | Ordenação sem algoritmo |
| 3.3 | Invariante de remuneração | Conflito de interesse sem controle |
| 4 | 2 | Distribuição sem política |
| 4.3 | 9 | Silêncio não distinguido de recusa |
| 5 | 9 | Contraproposta não modelada |
| 6.2 | 10 | "Quando aplicável" sem critério |
| 7 | 3 | Validação sem escopo definido |
