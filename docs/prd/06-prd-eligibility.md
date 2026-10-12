# PRD — Eligibility

## Propósito

Aplicar regras técnicas e comerciais determinísticas para indicar quais ativos podem compor uma solicitação de antecipação. O contexto não realiza decisão de crédito.

## Escopo

- Validade e atualidade do snapshot.
- Disponibilidade conhecida do ativo.
- Regras de valor, prazo e tipo de ativo.
- Capacidades dos financiadores.
- Composição de lotes e explicação de exclusões.
- Reserva interna temporária durante a contratação.

## Fora do escopo

- Rating, score ou limite de crédito.
- Precificação financeira.
- Aprovação regulatória do cliente.
- Confirmação de titularidade ou gravame, que permanece externa.

## Modelo de domínio

- `EligibilityPolicy`: conjunto versionado de regras.
- `EligibilityEvaluation`: execução sobre snapshot específico.
- `RuleResult`: aprovação, rejeição ou pendência, com razão.
- `EligibleLot`: ativos selecionáveis para um produto e financiador.
- `Reservation`: bloqueio interno temporário contra concorrência na plataforma.

## Requisitos funcionais

1. Avaliar regras por tipo de ativo, parceiro e produto.
2. Produzir justificativa legível para cada exclusão.
3. Versionar políticas e parâmetros usados.
4. Separar regra bloqueante, alerta e informação.
5. Suportar limites de concentração por sacado, data ou arranjo.
6. Invalidar avaliação quando snapshot, consentimento ou KYB expirar.
7. Reservar ativos durante seleção e contratação, com expiração automática.
8. Reavaliar imediatamente antes da contratação e do registro do efeito.
9. Permitir simulação sem criar reserva.

## Exemplos de regras

- Ativo não vencido e com prazo dentro da faixa do produto.
- Valor mínimo e máximo do financiador.
- Tipo de ativo suportado pelo parceiro.
- Consentimento válido para consulta e compartilhamento.
- Ausência de efeito incompatível conhecido.
- Dossiê cadastral com status mínimo exigido.

## Invariantes

- Elegível não significa crédito aprovado.
- Reserva interna não produz efeito perante terceiros.
- Toda avaliação referencia versões de política e snapshot.
- Regra alterada não modifica avaliação histórica.
- A confirmação externa prevalece sobre qualquer projeção local.

## Eventos

`EligibilityEvaluationRequested`, `AssetEligible`, `AssetIneligible`, `EligibleLotCreated`, `AssetReserved`, `ReservationExpired`, `EligibilityInvalidated`.

## Critérios de aceitação

- O usuário entende por que um ativo não pode ser selecionado.
- Alteração do snapshot invalida reservas e avaliações dependentes quando necessário.
- Ativos reservados não entram em duas contratações simultâneas na plataforma.
- O financiador recebe somente lotes compatíveis com suas capacidades declaradas.

## Métricas

- Taxa de elegibilidade por tipo de ativo.
- Principais motivos de exclusão.
- Reservas expiradas sem contratação.
- Reavaliações que alteraram o lote.

## Dependências

Portfolio fornece posições. KYB/KYC, Consent & Mandates e Partner Management fornecem pré-condições. Marketplace consome lotes elegíveis.

