# PRD — Compliance Ledger

## Propósito

Preservar evidências técnicas e negociais de forma íntegra, temporal e pesquisável, permitindo reproduzir o que ocorreu sem transformar o ledger em banco transacional dos demais contextos.

## Escopo

- Registro append-only de eventos relevantes.
- Hash e referência de documentos e payloads.
- Linha do tempo por organização, ativo, pedido, contrato e parceiro.
- Pacote de evidências para auditoria e contestação.
- Política de retenção e legal hold.
- Detecção de lacunas e inconsistências na trilha.

## Modelo de domínio

- `AuditRecord`: registro imutável de ação ou fato.
- `EvidenceReference`: localização, hash, tipo e política de acesso.
- `AuditSubject`: entidade à qual o registro se relaciona.
- `EvidenceBundle`: conjunto exportável para uma finalidade.
- `RetentionPolicy`: prazo e fundamento.
- `LegalHold`: suspensão controlada de descarte.

## Requisitos funcionais

1. Receber eventos de todos os bounded contexts com esquema versionado.
2. Registrar ator, tenant, instante, correlação, origem e resultado.
3. Preservar hash do payload original e dos artefatos materiais.
4. Permitir reconstrução da jornada em ordem causal e temporal.
5. Gerar dossiês com controle de acesso e finalidade.
6. Registrar acesso e exportação das próprias evidências.
7. Aplicar retenção por categoria, contrato e obrigação.
8. Impedir alteração ou exclusão ordinária de registros.
9. Detectar sequência ausente, duplicada ou inválida.
10. Permitir comprovar versão de regra, oferta, consentimento e documento.

## Invariantes

- Ledger não aceita atualização destrutiva; correções são novos registros.
- Hash sem referência recuperável não constitui dossiê completo.
- Dados pessoais são minimizados; o ledger referencia conteúdos protegidos quando possível.
- Relógio do produtor e instante de ingestão são mantidos separadamente.
- Retenção não é definida genericamente para todos os eventos.

## Eventos consumidos

Todos os eventos listados no catálogo compartilhado, especialmente consentimentos, verificações, apresentação de ofertas, assinatura, efeitos, desembolso, liquidação e ajustes manuais.

## APIs principais

- `GET /audit/timelines/{correlationId}`
- `POST /audit/evidence-bundles`
- `GET /audit/evidence-bundles/{id}`
- `POST /audit/legal-holds`

## Requisitos não funcionais

- Armazenamento WORM ou controle equivalente para evidências críticas.
- Criptografia e segregação de chaves.
- Índices de pesquisa separados do conteúdo original.
- Verificação periódica de integridade.
- Exportações assinadas e com manifesto.

## Critérios de aceitação

- Uma operação pode ser reproduzida sem consultar logs voláteis.
- É possível demonstrar exatamente qual oferta e contrato foram aceitos.
- Correção operacional não apaga a evidência anterior.
- Acesso a dossiê sensível é autorizado e auditado.

## Métricas

- Operações com trilha completa.
- Eventos rejeitados por schema ou integridade.
- Tempo para gerar dossiê.
- Verificações de integridade com falha.

## Dependências

Todos os contextos publicam eventos. Identity & Organization controla acesso. Partner Management e jurídico fornecem políticas de retenção.

