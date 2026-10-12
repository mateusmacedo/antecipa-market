# PRD — Marketplace

## Propósito

Orquestrar pedidos de financiamento e ofertas comparáveis de múltiplos financiadores, preservando transparência, atribuição da decisão e rastreabilidade da ordenação.

## Escopo

- Criação de pedido a partir de lote elegível.
- Distribuição controlada aos financiadores autorizados.
- Recepção, validação e expiração de ofertas.
- Normalização de preço e condições sem alterar a proposta original.
- Comparação, ordenação explicável e seleção.
- Encaminhamento ao Contracting.

## Modelo de domínio

- `FundingRequest`: intenção do cedente e ativos oferecidos.
- `PartnerInvitation`: convite a um financiador.
- `FinancingOffer`: oferta original e versão normalizada.
- `PricingBreakdown`: valor bruto, desconto, tarifas, tributos e líquido.
- `RankingPolicy`: critério de apresentação.
- `OfferSelection`: escolha autenticada do cliente.

## Requisitos funcionais

1. Criar pedido apenas com avaliação de elegibilidade válida.
2. Enviar a cada financiador somente dados autorizados.
3. Preservar integralmente condições e validade da oferta recebida.
4. Calcular métricas comparáveis sem reprecificar a operação.
5. Informar instituição concedente, custo, valor líquido, prazo e condições.
6. Tornar critérios de ordenação auditáveis, inclusive quando houver patrocínio.
7. Bloquear seleção de oferta expirada ou alterada.
8. Revalidar lote e consentimentos no momento da seleção.
9. Registrar rejeição, silêncio e contraproposta por parceiro.
10. Encerrar convites remanescentes após contratação, quando aplicável.

## Estados do pedido

`DRAFT → OPEN → COLLECTING_OFFERS → OFFERS_AVAILABLE → OFFER_SELECTED → CONTRACTING → CLOSED`

Exceções: `NO_OFFER`, `EXPIRED`, `CANCELLED`, `ELIGIBILITY_INVALIDATED`.

## Invariantes

- A plataforma não aprova crédito nem garante disponibilidade da oferta.
- Uma oferta pertence a um financiador e a uma versão do pedido.
- Ordenação nunca altera os valores originais.
- Seleção referencia versão e hash da oferta exibida.
- Remuneração comercial não pode ser escondida no ranking.

## Eventos

`FundingRequestOpened`, `PartnerInvited`, `OfferReceived`, `OfferRejectedAsInvalid`, `OffersPresented`, `OfferSelected`, `FundingRequestExpired`, `FundingRequestClosed`.

## APIs principais

- `POST /funding-requests`
- `POST /funding-requests/{id}/invitations`
- `POST /funding-requests/{id}/offers`
- `GET /funding-requests/{id}/comparable-offers`
- `POST /funding-requests/{id}/selection`

## Critérios de aceitação

- Ofertas de estruturas diferentes são comparáveis sem perda do documento original.
- O cliente sabe quem concede o crédito e quanto receberá líquido.
- Uma oferta expirada não avança para contrato.
- A apresentação pode ser reconstruída exatamente para auditoria.

## Métricas

- Tempo até primeira oferta.
- Ofertas por pedido.
- Conversão por financiador e produto.
- Diferença entre melhor preço e oferta selecionada.
- Pedidos encerrados sem oferta.

## Dependências

Eligibility fornece lote. Consent & Mandates autoriza compartilhamento. Partner Management define produtos. Contracting recebe seleção. Compliance Ledger preserva a apresentação.

