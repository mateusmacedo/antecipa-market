# PRD — Partner Management

## Propósito

Gerenciar financiadores, escrituradores, registradoras, depositários, provedores cadastrais e instituições de liquidação como parceiros configuráveis, com contratos, capacidades e limites explícitos.

## Escopo

- Cadastro e classificação de parceiros.
- Produtos, ativos e operações suportados.
- Ambientes, endpoints, credenciais e certificados referenciados.
- SLA, calendário, janelas e limites.
- Requisitos documentais e de integração.
- Modelo comercial e regras de apresentação.
- Ativação, suspensão e descontinuidade.

## Modelo de domínio

- `Partner`: organização externa e papel.
- `PartnerAgreement`: relação contratual e vigência.
- `Capability`: operação, ativo, versão e restrições.
- `FinancialProduct`: produto ofertável e parâmetros.
- `IntegrationProfile`: protocolos e referências de segredo.
- `OperationalPolicy`: SLA, rate limit, janela e fallback.
- `CommercialRule`: remuneração e impacto permitido na apresentação.

## Requisitos funcionais

1. Impedir uso de parceiro sem contrato ativo e capacidade homologada.
2. Versionar produtos, requisitos e capacidades.
3. Configurar faixas de valor, prazo, ativos e regiões atendidas.
4. Vincular credenciais por ambiente sem armazenar segredo em configuração comum.
5. Controlar rollout gradual por tenant e produto.
6. Suspender operação específica sem desativar todo o parceiro.
7. Registrar mudança de schema, certificado e endpoint.
8. Disponibilizar capacidade efetiva aos demais contextos.
9. Registrar remuneração e regras de transparência do marketplace.
10. Manter plano de saída e tratamento de operações em curso.

## Estados

`PROSPECT → CONTRACTING → INTEGRATING → HOMOLOGATING → ACTIVE → SUSPENDED → OFFBOARDING → INACTIVE`

## Invariantes

- Capacidade ativa exige contrato, homologação e configuração válidos.
- Segredos são referenciados, nunca expostos no domínio.
- Suspensão não apaga operações e evidências existentes.
- Produto expirado não recebe novos pedidos.
- Regra comercial não pode alterar oferta do financiador.

## Eventos

`PartnerRegistered`, `PartnerAgreementActivated`, `CapabilityEnabled`, `CapabilitySuspended`, `PartnerProductChanged`, `PartnerCertificateExpiring`, `PartnerOffboardingStarted`.

## Critérios de aceitação

- Marketplace encontra apenas financiadores elegíveis ao pedido.
- Gateway seleciona adaptador e versão compatíveis.
- Uma suspensão bloqueia novos comandos sem perder acompanhamento dos existentes.
- Alterações de parceiro são reproduzíveis historicamente.

## Métricas

- Disponibilidade e erro por parceiro.
- Capacidades homologadas versus contratadas.
- Incidentes por mudança não compatível.
- Certificados próximos do vencimento.
- Volume e conversão por produto.

## Dependências

Identity & Organization representa a organização parceira. Receivables Gateway aplica configurações. Marketplace e Eligibility consomem produtos. Compliance Ledger registra mudanças materiais.

