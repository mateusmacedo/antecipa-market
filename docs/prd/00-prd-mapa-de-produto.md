# PRD — Plataforma Multiativo de Antecipação de Recebíveis

## Controle do documento

- Versão: 1.0
- Status: proposta para refinamento
- Abrangência: Brasil
- Ativos iniciais: duplicatas escriturais e unidades de recebíveis de cartões
- Papel da empresa: plataforma tecnológica intermediadora

## Visão do produto

Construir uma plataforma B2B que permita a empresas consultar recebíveis elegíveis, solicitar antecipação, comparar ofertas de financiadores, contratar e acompanhar registro, desembolso, liquidação e conciliação. A plataforma não concede crédito, não assume titularidade dos ativos, não escritura ou registra ativos em nome próprio, não mantém contas de pagamento e não transita recursos.

## Objetivos

1. Reduzir tempo e esforço para obtenção de capital de giro.
2. Permitir competição entre financiadores com ofertas comparáveis.
3. Impedir dupla negociação e uso de ativos indisponíveis.
4. Preservar a infraestrutura regulada como fonte oficial de titularidade, gravames e liquidação.
5. Produzir evidências reproduzíveis de cada consentimento, decisão, contrato e comando externo.

## Não objetivos

- Decidir ou conceder crédito.
- Captar recursos de investidores.
- Executar empréstimos entre pessoas.
- Custodiar valores ou direitos creditórios.
- Substituir escrituradores, registradoras ou depositários centrais.
- Criar um registro privado concorrente com as infraestruturas autorizadas.

## Bounded contexts

| Contexto | Responsabilidade | PRD |
| --- | --- | --- |
| Identity & Organization | Identidade, empresas, representantes e tenants | `01-prd-identity-organization.md` |
| KYB/KYC | Verificação cadastral e beneficiário final | `02-prd-kyb-kyc.md` |
| Consent & Mandates | Opt-ins, mandatos, escopo e revogação | `03-prd-consent-mandates.md` |
| Receivables Gateway | Integração com escrituradores e registradoras | `04-prd-receivables-gateway.md` |
| Portfolio | Visão normalizada e versionada dos recebíveis | `05-prd-portfolio.md` |
| Eligibility | Regras técnicas para composição do lote | `06-prd-eligibility.md` |
| Marketplace | Solicitação, recepção e comparação de ofertas | `07-prd-marketplace.md` |
| Contracting | Formalização, assinatura e dossiê contratual | `08-prd-contracting.md` |
| Effects Orchestrator | Constituição e acompanhamento de efeitos | `09-prd-effects-orchestrator.md` |
| Funding | Ordem e confirmação de desembolso | `10-prd-funding.md` |
| Settlement & Reconciliation | Liquidação, baixa e divergências | `11-prd-settlement-reconciliation.md` |
| Compliance Ledger | Evidências imutáveis e reprodução temporal | `12-prd-compliance-ledger.md` |
| Partner Management | Capacidades e configuração de parceiros | `13-prd-partner-management.md` |

## Jornada principal

`ONBOARDING → DISCOVERY → ELIGIBILITY → QUOTATION → SELECTION → CONTRACTING → EFFECT REGISTRATION → FUNDING → SETTLEMENT → RECONCILIATION`

Estados de exceção: `REJECTED`, `EXPIRED`, `CANCELLED`, `DISPUTED`, `REGISTRATION_FAILED` e `RECONCILIATION_REQUIRED`.

`REGISTRATION_FAILED` e `RECONCILIATION_REQUIRED` nomeiam desfechos da jornada, não estados de agregado. Nos contextos que os produzem, o primeiro corresponde a `REJECTED` em Effects Orchestrator quando a recusa é definitiva, e a `UNKNOWN_OUTCOME` quando o resultado do comando é ambíguo; o segundo corresponde a `MISMATCH` em Settlement & Reconciliation, com abertura de caso operacional.

## Regras transversais

1. A infraestrutura externa competente é a fonte de verdade do ativo.
2. Um efeito somente é considerado eficaz após confirmação externa.
3. Uma oferta referencia uma versão imutável do portfólio.
4. Antes da assinatura e antes do registro deve ocorrer revalidação de disponibilidade.
5. Todo comando externo deve ser idempotente e correlacionável.
6. Desembolso e liquidação não passam por contas da plataforma.
7. Consentimento regulatório e base legal de tratamento de dados são controles distintos.
8. Falhas externas são tratadas por saga e conciliação, nunca por transação distribuída presumida.

## Indicadores de produto

- Tempo entre solicitação e primeira oferta.
- Percentual de solicitações com oferta válida.
- Conversão de oferta selecionada para efeito confirmado.
- Tempo entre efeito confirmado e desembolso.
- Taxa de comandos externos reprocessados com sucesso.
- Divergências de liquidação por mil operações.
- Operações com dossiê integralmente reproduzível.

## Estratégia de entrega

1. Recebíveis de cartão, um financiador e uma conexão de infraestrutura.
2. Duplicata escritural, um escriturador e um financiador.
3. Marketplace com múltiplos financiadores.
4. Múltiplas infraestruturas, FIDCs e contratos sobre recebíveis futuros.

## Critério de conclusão do produto inicial

Uma empresa elegível consegue autorizar consulta, visualizar recebíveis, solicitar ofertas, selecionar uma condição, assinar o contrato, acompanhar a confirmação do efeito e verificar desembolso e liquidação, com trilha de auditoria completa e sem trânsito financeiro pela plataforma.

