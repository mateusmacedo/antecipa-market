# PRD — Effects Orchestrator

## Propósito

Orquestrar a constituição, confirmação, alteração e extinção de efeitos jurídicos e regulatórios sobre recebíveis, tratando a infraestrutura autorizada como autoridade final.

## Tipos de efeito iniciais

- Desconto ou aquisição definitiva.
- Aquisição com coobrigação, quando juridicamente aplicável.
- Cessão e cessão fiduciária.
- Garantia sobre recebíveis.
- Mudança de titularidade ou beneficiário.
- Endosso e endosso-penhor para duplicatas.
- Cancelamento, liberação e portabilidade.

## Modelo de domínio

- `RegulatoryEffect`: intenção de efeito sobre um conjunto de ativos.
- `EffectTarget`: duplicata, unidade de duplicatas ou UR, total ou parcial.
- `EffectTerms`: tipo, prioridade, alcance, valor e vigência.
- `ExternalEffectReference`: identificador oficial.
- `EffectConfirmation`: retorno autoritativo.
- `EffectDispute`: contestação ou divergência.

## Requisitos funcionais

1. Receber apenas contratos assinados e tecnicamente válidos.
2. Mapear o contrato ao tipo de efeito aceito pela infraestrutura.
3. Reconsultar posição imediatamente antes do comando.
4. Enviar comando idempotente pelo Receivables Gateway.
5. Distinguir aceite técnico de eficácia confirmada.
6. Bloquear Funding enquanto o efeito não estiver confirmado.
7. Tratar confirmação parcial por ativo ou valor.
8. Acompanhar extensões automáticas a recebíveis futuros.
9. Suportar liberação, cancelamento e portabilidade com trilha integral.
10. Abrir disputa quando contrato e posição oficial divergirem.

## Estados

`PLANNED → PREVALIDATING → SUBMITTED → ACKNOWLEDGED → EFFECTIVE → RELEASE_PENDING → RELEASED`

Exceções: `PARTIALLY_EFFECTIVE`, `REJECTED`, `UNKNOWN_OUTCOME`, `DISPUTED`, `CANCELLED`.

## Invariantes

- Somente `EFFECTIVE` autoriza desembolso padrão.
- Identificador externo e confirmação são obrigatórios para eficácia.
- `UNKNOWN_OUTCOME` nunca é reenviado sem consulta de estado.
- Confirmação parcial divide o resultado por alvo; não promove o conjunto inteiro.
- A plataforma não define prioridade jurídica entre efeitos conflitantes.

## Eventos

`RegulatoryEffectPlanned`, `EffectSubmitted`, `EffectAcknowledged`, `EffectConfirmed`, `EffectPartiallyConfirmed`, `EffectRejected`, `EffectOutcomeUnknown`, `EffectReleased`, `EffectDisputed`.

## Critérios de aceitação

- Nenhum desembolso padrão ocorre com efeito apenas enviado.
- É possível demonstrar contrato, payload, retorno e posição final.
- Falha parcial identifica precisamente ativos afetados.
- Cancelamento interno não remove efeito externo sem confirmação de liberação.

## Métricas

- Tempo entre contrato e efeito confirmado.
- Rejeições por motivo e infraestrutura.
- Resultados desconhecidos.
- Efeitos parciais e disputados.
- Tempo de liberação após quitação.

## Dependências

Contracting origina o efeito. Receivables Gateway executa comandos. Portfolio recebe projeções. Funding aguarda confirmação. Settlement informa quitação. Compliance Ledger preserva evidências.

