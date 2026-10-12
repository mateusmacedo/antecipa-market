# PRD — Settlement & Reconciliation

## Propósito

Acompanhar pagamentos dos recebíveis, baixas, direcionamento ao beneficiário e divergências entre contrato, efeito registrado, posição oficial e fluxo financeiro.

## Escopo

- Expectativa de liquidação por ativo e contrato.
- Ingestão de confirmações de pagamento e baixa.
- Conciliação de valores, datas, beneficiários e identificadores.
- Tratamento de liquidação parcial, atraso, chargeback e cancelamento.
- Casos de divergência e resolução operacional.
- Solicitação de liberação do efeito após cumprimento.

## Modelo de domínio

- `SettlementExpectation`: fluxo esperado.
- `SettlementOccurrence`: evento informado por fonte externa.
- `ReconciliationRun`: execução de comparação.
- `ReconciliationItem`: correspondência ou divergência individual.
- `ReconciliationCase`: caso operacional com responsável e SLA.
- `SettlementAllocation`: distribuição entre principal, encargos e saldo livre conforme fonte competente.

## Requisitos funcionais

1. Gerar expectativas a partir dos ativos e efeitos confirmados.
2. Ingerir confirmações sem presumir unicidade por valor e data.
3. Conciliar por identificadores oficiais e regras do parceiro.
4. Diferenciar pago, baixado, liquidado e repassado.
5. Suportar liquidação parcial e múltiplos eventos.
6. Detectar pagamento ao beneficiário incorreto ou ausência de direcionamento.
7. Abrir caso com evidências e classificação de impacto.
8. Permitir ajuste somente com justificativa e autorização.
9. Solicitar liberação de efeito após confirmação da obrigação competente.
10. Reprocessar períodos sem apagar resultados anteriores.

## Estados da expectativa

`EXPECTED → DUE → PARTIALLY_SETTLED → SETTLED → RECONCILED`

Exceções: `OVERDUE`, `MISMATCH`, `REVERSED`, `DISPUTED`, `WRITE_OFF_REPORTED`.

## Invariantes

- Pagamento relatado não equivale a baixa confirmada.
- Ajuste manual não altera o dado bruto recebido.
- Reprocessamento gera nova execução versionada.
- Liberação de efeito depende da confirmação exigida pelo contrato e pela infraestrutura.
- A plataforma não redistribui recursos por conta própria.

## Eventos

`SettlementExpected`, `SettlementReported`, `AssetPartiallySettled`, `AssetSettled`, `ReconciliationMatched`, `ReconciliationMismatchDetected`, `ReconciliationCaseResolved`, `EffectReleaseRequested`.

## Critérios de aceitação

- Cada fluxo esperado termina conciliado ou associado a caso aberto.
- Divergências mantêm dados originais, decisão e responsável.
- Liquidações parciais não encerram o ativo integralmente.
- A situação exibida informa a fonte e o instante da confirmação.

## Métricas

- Percentual conciliado automaticamente.
- Divergências por mil liquidações.
- Tempo médio de resolução.
- Valores não identificados e pagamentos em atraso.
- Liberações pendentes após quitação.

## Dependências

Portfolio fornece ativos. Effects Orchestrator fornece beneficiário e efeitos. Funding fornece desembolso. Receivables Gateway consulta fontes. Compliance Ledger preserva execuções e ajustes.

