# PRD — Portfolio

## Propósito

Disponibilizar uma visão normalizada, versionada e temporal dos recebíveis consultados, sem substituir a posição oficial mantida pelas infraestruturas reguladas.

## Escopo

- Ingestão de agendas e ativos vindos do Receivables Gateway.
- Modelo canônico para duplicatas e unidades de recebíveis.
- Snapshots imutáveis de portfólio.
- Projeção de saldo disponível, comprometido e liquidado.
- Proveniência e comparação entre versões.
- Consulta otimizada para seleção e simulação.

## Tipos de ativo

`BookEntryInvoice`: sacador, sacado, documento fiscal, emissão, vencimento, valor, aceite, escriturador, titular e situação.

`CardReceivableUnit`: titular original, credenciador/subcredenciador, arranjo, data de liquidação, valor bruto, ajustes, valor líquido e registradora.

## Modelo de domínio

- `Portfolio`: conjunto lógico pertencente a uma organização.
- `PortfolioSnapshot`: fotografia imutável com origem e instante.
- `FinancialAsset`: contrato comum dos ativos.
- `BookEntryInvoice` e `CardReceivableUnit`: especializações.
- `ExternalPosition`: posição informada pela fonte oficial.
- `CommitmentProjection`: efeito conhecido pela plataforma.
- `AvailabilityProjection`: disponibilidade derivada e não autoritativa.

## Requisitos funcionais

1. Importar posições sem perder campos ou identificadores de origem.
2. Criar nova versão quando houver alteração material.
3. Manter histórico consultável e comparação entre snapshots.
4. Distinguir valor nominal, líquido esperado, comprometido e disponível.
5. Marcar dados vencidos conforme SLA da fonte.
6. Impedir que uma projeção antiga fundamente nova contratação.
7. Suportar agrupamentos por vencimento, sacado, arranjo, credenciador e risco.
8. Vincular ajustes, cancelamentos, chargebacks e pagamentos à versão correspondente.
9. Expor divergências entre projeção interna e posição externa.

## Invariantes

- Snapshot publicado é imutável.
- Disponibilidade sempre contém `asOf`, fonte e política de cálculo.
- Ausência de informação não significa ausência de gravame.
- Identidade de duplicata e identidade de UR seguem esquemas distintos.
- Valores agregados são derivados dos itens da mesma versão.

## Eventos

`PortfolioDiscoveryRequested`, `PortfolioSnapshotCreated`, `AssetPositionChanged`, `AssetBecameStale`, `ExternalPositionDiverged`, `AssetSettled`.

## APIs principais

- `POST /portfolios/{id}/discoveries`
- `GET /portfolios/{id}/snapshots/latest`
- `GET /portfolio-snapshots/{id}/assets`
- `GET /portfolio-snapshots/{id}/changes`

## Critérios de aceitação

- Toda oferta informa o snapshot que a originou.
- É possível reproduzir a posição apresentada ao usuário.
- Recebível vencido ou desatualizado não aparece como livre sem ressalva.
- Atualização externa não modifica retroativamente snapshots anteriores.

## Métricas

- Idade média das posições exibidas.
- Divergências por origem.
- Tempo de ingestão de agenda.
- Ativos descartados por inconsistência.

## Dependências

Receivables Gateway fornece os dados. Eligibility consome snapshots. Effects Orchestrator e Settlement enviam mudanças conhecidas. Compliance Ledger preserva versões relevantes.

