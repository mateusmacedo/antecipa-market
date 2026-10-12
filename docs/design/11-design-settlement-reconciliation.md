# Design — Settlement & Reconciliation

> **PRD de origem**: `docs/prd/11-prd-settlement-reconciliation.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| R1 | "Conciliar por identificadores oficiais e regras do parceiro" sem algoritmo | Req. 3 |
| R2 | "Não presumir unicidade por valor e data" sem alternativa | Req. 2 |
| R3 | Distinção entre pago, baixado, liquidado e repassado não definida | Req. 4 |
| R4 | Alçada de ajuste indefinida | Req. 8 |
| R5 | `SettlementAllocation` sem regra de rateio | Modelo de domínio |
| R6 | Liquidação parcial e múltiplos eventos sem modelo de acumulação | Req. 5 |
| R7 | Condição para solicitar liberação do efeito sem definição | Req. 9 |
| R8 | Chargeback mencionado sem tratamento temporal | Escopo |

## 2. Os quatro estados que o PRD distingue e não define

Req. 4 exige diferenciar "pago, baixado, liquidado e repassado". A diferença é
substantiva e determina quando a obrigação se extingue.

| Estado | Significado | Quem informa | Extingue a obrigação? |
| --- | --- | --- | --- |
| **Pago** | O devedor efetuou o pagamento | Credenciador, sacado, registradora | Não necessariamente |
| **Baixado** | A fonte oficial registrou a quitação do ativo | Escriturador, registradora | Quanto ao ativo, sim |
| **Liquidado** | Os recursos foram efetivamente compensados | Instituição de liquidação | Sim |
| **Repassado** | Os recursos chegaram ao beneficiário do efeito | Beneficiário ou fonte | **É o que importa para liberar** |

**A distinção crítica é entre baixado e repassado.** Um recebível pode estar
baixado na registradora e o financiador não ter recebido — falha de
direcionamento. `11-prd` prevê isso ("detectar pagamento ao beneficiário incorreto
ou ausência de direcionamento", req. 6), e sem estados distintos a detecção é
impossível.

**Consequência para a liberação do efeito (req. 9)**: a condição é o repasse
confirmado, não a baixa. Liberar por baixa liberaria a garantia antes de o credor
receber.

## 3. Correspondência

### 3.1 O problema

Req. 2 proíbe presumir unicidade por valor e data — corretamente. Múltiplos
recebíveis do mesmo valor liquidando no mesmo dia são a regra, não a exceção, em
recebíveis de cartão.

### 3.2 Estratégia em camadas

| Camada | Chave | Confiança | Quando se aplica |
| --- | --- | --- | --- |
| 1 | Identificador oficial do ativo | Determinística | Fonte informa o identificador |
| 2 | Identificador do efeito ou do contrato | Determinística | Fonte informa a referência do efeito |
| 3 | Composição (ativo + data + valor + beneficiário) | Alta | Combinação única no conjunto |
| 4 | Agrupamento (lote reportado contra conjunto esperado) | Média | Fonte reporta agregado |
| 5 | Sem correspondência | — | Divergência |

**Regra**: camadas 1 e 2 são determinísticas e não exigem revisão. A camada 3 só
vale quando a combinação é única — se dois ativos compartilham a combinação, cai
para a camada 5. A camada 4 concilia valor total e abre item de detalhamento
pendente. Nunca se adivinha correspondência.

**Cada `ReconciliationItem` registra a camada usada.** Auditoria precisa saber se
uma correspondência é determinística ou inferida — e uma conciliação com alta
proporção de camada 3 é sinal de que os identificadores não estão sendo
propagados corretamente.

### 3.3 Acumulação

Req. 5 exige suportar liquidação parcial e múltiplos eventos.

```text
SettlementExpectation
  ├─ expectedAmount
  ├─ settledAmount        (soma das ocorrências conciliadas)
  ├─ reversedAmount       (soma dos estornos)
  ├─ netSettledAmount     (settled - reversed)
  └─ occurrences[]        (append-only)
```

| Condição | Estado |
| --- | --- |
| `netSettled = 0` e data futura | `EXPECTED` |
| `netSettled = 0` e data chegada | `DUE` |
| `0 < netSettled < expected` | `PARTIALLY_SETTLED` |
| `netSettled >= expected` | `SETTLED` |
| `SETTLED` e todas as ocorrências conciliadas | `RECONCILED` |
| `netSettled = 0` e data passada além da tolerância | `OVERDUE` |
| `reversed > 0` após `SETTLED` | `REVERSED` |

**Ocorrências são append-only.** Estorno não subtrai da ocorrência original — cria
ocorrência de sinal oposto. Isso preserva o dado bruto (invariante do PRD) e torna
o histórico auditável: subtrair perderia a informação de que houve pagamento e
estorno.

## 4. Rateio

`SettlementAllocation` é definido no PRD como "distribuição entre principal,
encargos e saldo livre conforme fonte competente" — sem regra.

**Posição construtiva**: a plataforma **não calcula rateio**. Ela registra o
rateio informado pela fonte competente e o concilia com o esperado.

**Razão**: quando há múltiplos efeitos sobre o mesmo ativo, a ordem de prioridade
é matéria jurídica e regulatória, definida pela infraestrutura — e
`09-prd-effects-orchestrator.md` já estabelece que "a plataforma não define
prioridade jurídica entre efeitos conflitantes". Calcular rateio seria decidir
prioridade por outro nome.

| Situação | Tratamento |
| --- | --- |
| Fonte informa o rateio | Registrar; conciliar com o esperado |
| Fonte informa apenas o total | Conciliar total; alocação por ativo fica pendente |
| Rateio informado diverge do esperado | Divergência; caso operacional |
| Múltiplos efeitos e fonte não detalha | **Não inferir**; caso operacional |

**Consequência a aceitar**: operações com múltiplos efeitos sobre os mesmos ativos
podem gerar conciliação manual recorrente. A alternativa — inferir — produziria
números plausíveis e possivelmente errados, com aparência de correção.

## 5. Execução da conciliação

```text
1. Selecionar janela (por data de liquidação e parceiro)
2. Carregar expectativas da janela
3. Ingerir ocorrências reportadas (arquivo, API, webhook)
4. Persistir ocorrências brutas          <- antes de qualquer correspondência
5. Aplicar correspondência em camadas
6. Gerar ReconciliationItem por correspondência e por divergência
7. Classificar divergências por tipo e severidade
8. Abrir ou atualizar casos
9. Fechar a execução com sumário
10. Publicar eventos
```

| Aspecto | Definição |
| --- | --- |
| Periodicidade | Diária, após a janela de corte de cada fonte |
| Reprocessamento | Nova execução versionada; a anterior é preservada (req. 10) |
| Idempotência | Ocorrência já ingerida não duplica |
| Escopo | Por parceiro, para isolar atraso de uma fonte |

**Passo 4 antes do 5**: persistir o bruto antes de correlacionar garante que uma
falha na correspondência não perde o dado recebido, e permite reprocessar com
regra corrigida (`04-design-receivables-gateway.md`, 6).

## 6. Divergências

| Tipo | Severidade padrão | Ação |
| --- | --- | --- |
| `AMOUNT_DIVERGENCE` abaixo do limiar | `LOW` | Ajuste com justificativa, um aprovador |
| `AMOUNT_DIVERGENCE` acima do limiar | `HIGH` | Investigar; dois aprovadores |
| `DATE_DIVERGENCE` dentro da tolerância | `LOW` | Registrar |
| `DATE_DIVERGENCE` fora da tolerância | `MEDIUM` | Investigar |
| `BENEFICIARY_DIVERGENCE` | **`CRITICAL`** | Escalar a risco, qualquer valor |
| `UNEXPECTED_SETTLEMENT` | `MEDIUM` | Investigar origem |
| `MISSING_SETTLEMENT` além da tolerância | `HIGH` | Consultar fonte |
| `DUPLICATE_SETTLEMENT` | `HIGH` | Investigar; possível estorno |

**`BENEFICIARY_DIVERGENCE` sempre crítico**: pagamento a quem não deveria receber
é falha na constituição do efeito ou fraude. Valor pequeno pode ser teste.

### 6.1 Alçada de ajuste

Req. 8 exige "justificativa e autorização" sem definir quem autoriza.

| Faixa de impacto | Aprovação |
| --- | --- |
| Abaixo do limiar menor | Operador, com justificativa |
| Entre os limiares | Operador + supervisor |
| Acima do limiar maior | Operador + supervisor + responsável financeiro |
| Qualquer valor com suspeita de fraude | Escala a risco, independentemente da faixa |

**Ajuste nunca altera o dado bruto** (invariante). Cria `ReconciliationAdjustment`
vinculado, com autor, justificativa, aprovadores e instante. A visão conciliada é
derivada de bruto mais ajustes, nunca de bruto alterado.

## 7. Liberação do efeito

Req. 9: "solicitar liberação de efeito após confirmação da obrigação competente".

**Condição composta**:

```text
podeSolicitarLiberacao =
      obrigação contratual cumprida (conforme os termos do contrato)
  AND repasse ao beneficiário confirmado
  AND ausência de estorno dentro da janela de contestação
  AND ausência de disputa aberta sobre o efeito
```

| Componente | Fonte |
| --- | --- |
| Obrigação cumprida | Termos do contrato; para cessão definitiva, a liquidação dos ativos |
| Repasse confirmado | Seção 2 |
| Janela de contestação | Prazo de chargeback por arranjo — **lacuna** |
| Ausência de disputa | Effects Orchestrator |

**A janela de contestação é a lacuna mais relevante.** Em recebíveis de cartão, um
pagamento liquidado pode ser estornado depois. Liberar o efeito imediatamente após
a liquidação expõe o financiador ao estorno sem garantia.

**Postura conservadora**: a liberação aguarda o decurso da janela de contestação
aplicável ao arranjo. Isso mantém o gravame por mais tempo e é preferível a
liberar cedo. O prazo por arranjo precisa ser levantado — ver
`docs/reference/provedores-integracao.md`.

## 8. Chargeback

Mencionado no escopo, sem tratamento.

```text
Liquidação reportada -> SETTLED
  -> chargeback dentro da janela
       -> ocorrência de sinal oposto
       -> estado REVERSED
       -> expectativa reaberta
       -> se o efeito já foi liberado -> caso CRITICAL
       -> se ainda vigente -> permanece; obrigação não cumprida
```

| Situação | Impacto |
| --- | --- |
| Antes do desembolso | Ativo deixa de existir; revisão contratual |
| Após o desembolso, efeito vigente | Obrigação não cumprida; efeito permanece |
| Após a liberação do efeito | **Pior caso** — financiador sem garantia |
| Volume anômalo de chargeback | Sinal de fraude; escala a risco |

**O pior caso justifica a postura da seção 7.** Liberar antes do fim da janela
cria exposição que nenhum controle posterior resolve.

## 9. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| R9 | Janela de contestação por arranjo (7, 8) | **Alto** |
| R10 | Formato dos arquivos de liquidação por fonte | Alto |
| R11 | Limiares de alçada (6.1) | Médio |
| R12 | Tolerância de data e valor (6) | Médio |
| R13 | Fontes informam rateio entre efeitos concorrentes? (4) | Alto |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | 4 | Quatro estados sem definição |
| 3 | 2, 3 | Correspondência sem algoritmo |
| 3.3 | 5 | Acumulação sem modelo |
| 4 | `SettlementAllocation` | Rateio sem regra |
| 6.1 | 8 | Alçada indefinida |
| 7 | 9 | Condição de liberação sem definição |
| 8 | Escopo | Chargeback sem tratamento temporal |
