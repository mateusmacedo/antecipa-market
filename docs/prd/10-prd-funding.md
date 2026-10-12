# PRD — Funding

## Propósito

Controlar a autorização e a confirmação do desembolso realizado pelo financiador diretamente ao cedente, sem trânsito de recursos pela plataforma.

## Escopo

- Verificação de pré-condições para desembolso.
- Emissão de ordem lógica ao financiador.
- Acompanhamento de processamento.
- Registro da confirmação e comprovante.
- Divergência entre valor contratado e valor creditado.
- Cancelamento antes da execução, quando permitido.

## Fora do escopo

- Manutenção de saldo ou conta de pagamento.
- Execução de Pix/TED pela plataforma.
- Custódia e compensação de recursos.
- Decisão de crédito ou cálculo contábil do financiador.

## Modelo de domínio

- `FundingInstruction`: autorização lógica vinculada ao contrato.
- `Disbursement`: execução informada pelo financiador.
- `DisbursementDestination`: conta validada do cedente.
- `FundingProof`: comprovante e referência bancária.
- `FundingMismatch`: divergência de valor, destino ou situação.

## Requisitos funcionais

1. Criar instrução somente para contrato assinado e efeito confirmado.
2. Validar que o destino pertence ao cedente ou é permitido contratualmente.
3. Exibir valor bruto, deduções e líquido esperado.
4. Enviar solicitação idempotente ao financiador.
5. Acompanhar estados assíncronos e consultar resultado desconhecido.
6. Confirmar desembolso apenas mediante retorno do financiador.
7. Comparar valor e destino confirmados com o contrato.
8. Abrir exceção se houver divergência ou estorno.
9. Disponibilizar comprovante ao cedente.
10. Publicar confirmação para Settlement & Reconciliation.

## Estados

`PENDING_PREREQUISITES → READY → REQUESTED → PROCESSING → DISBURSED`

Exceções: `REJECTED`, `CANCELLED`, `UNKNOWN_OUTCOME`, `MISMATCH`, `REVERSED`.

## Invariantes

- A plataforma nunca aparece como conta de destino ou trânsito.
- O valor confirmado não é inferido da ordem enviada.
- Uma chave idempotente não produz dois desembolsos.
- Efeito parcial exige instrução compatível com a parte confirmada ou revisão contratual.
- Mudança de conta após assinatura exige controle reforçado e nova validação.

## Eventos

`FundingInstructionCreated`, `DisbursementRequested`, `DisbursementProcessing`, `DisbursementConfirmed`, `FundingMismatchDetected`, `DisbursementReversed`.

## Critérios de aceitação

- Nenhuma instrução é emitida antes do efeito regulatório aplicável.
- Cedente visualiza valor líquido e situação sem a plataforma custodiar recursos.
- Confirmação contém referência do financiador e comprovante.
- Resultado desconhecido não gera reenvio automático inseguro.

## Métricas

- Tempo entre efeito confirmado e desembolso.
- Divergências de valor ou destino.
- Desembolsos em resultado desconhecido.
- Estornos por mil operações.

## Dependências

Effects Orchestrator libera o desembolso. Partner Management define o protocolo. Compliance Ledger registra as evidências. Settlement & Reconciliation inicia acompanhamento após confirmação.

