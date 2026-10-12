# PRD — Receivables Gateway

## Propósito

Isolar protocolos e semânticas de escrituradores, registradoras, depositários, credenciadores e financiadores, expondo portas canônicas ao restante da plataforma.

## Escopo

- Descoberta e consulta de agendas.
- Ativação e revogação de opt-ins.
- Consulta de titularidade, disponibilidade, ônus e efeitos.
- Envio e acompanhamento de comandos regulatórios.
- Recepção de arquivos, respostas assíncronas e webhooks.
- Tradução de códigos externos para o modelo canônico.

O contexto não mantém a verdade de negócio do portfólio e não decide elegibilidade.

## Portas canônicas

```text
ReceivablesQueryPort
AuthorizationPort
RegulatoryEffectsPort
SettlementQueryPort
ExternalEvidencePort
```

Cada adaptador declara capacidades em vez de depender de condicionais espalhadas pelo domínio.

## Requisitos funcionais

1. Integrar múltiplos parceiros sem expor seus formatos aos contextos consumidores.
2. Preservar identificadores externos, versão, instante e proveniência.
3. Aplicar idempotência por parceiro, comando e versão do ativo.
4. Correlacionar resposta síncrona, callback e consulta posterior.
5. Suportar paginação, arquivos em lote e mensagens assíncronas.
6. Normalizar erros em categorias recuperáveis, definitivas ou desconhecidas.
7. Impedir retry automático de comando não idempotente sem consulta de estado.
8. Detectar quebra de schema e suspender processamento incompatível.
9. Permitir reprocessamento controlado mantendo o payload original.

## Modelo operacional

- `Connector`: implementação para um provedor.
- `CapabilityProfile`: operações e versões suportadas.
- `ExternalCommand`: intenção enviada.
- `ExternalInteraction`: request, response e evidências.
- `ExternalReference`: vínculo entre IDs internos e externos.
- `DeliveryAttempt`: tentativa e resultado.

## Estados de comando

`CREATED → SENT → ACKNOWLEDGED → CONFIRMED`

Exceções: `RETRYABLE_FAILURE`, `FINAL_FAILURE`, `UNKNOWN_OUTCOME`, `CANCELLED`.

## Invariantes

- `ACKNOWLEDGED` não equivale a efeito regulatório confirmado.
- Resultado desconhecido exige consulta antes de novo envio.
- Payload normalizado nunca elimina o payload original.
- Credenciais são segregadas por parceiro e ambiente.

## Eventos

`ExternalCommandSubmitted`, `ExternalCommandAcknowledged`, `ExternalCommandConfirmed`, `ExternalCommandFailed`, `ExternalOutcomeUnknown`, `ExternalSchemaMismatchDetected`.

## Requisitos não funcionais

- mTLS e rotação de certificados.
- Rate limiting por integração.
- Circuit breaker por operação, sem mascarar falha definitiva.
- Métricas por parceiro, endpoint, código e percentil.
- DLQ operacional com dados sensíveis protegidos.
- Relógio UTC combinado com data de negócio informada pelo parceiro.

## Critérios de aceitação

- Um novo parceiro pode ser adicionado sem alterar o domínio de Marketplace ou Contracting.
- Reenvio com a mesma chave não duplica efeito externo.
- Toda resposta pode ser ligada ao comando e à operação originadora.
- Estado desconhecido gera reconciliação e bloqueia avanço indevido.

## Dependências

Partner Management fornece configuração e capacidades. Consent & Mandates autoriza consultas. Effects Orchestrator e Settlement utilizam as portas. Compliance Ledger recebe evidências.

