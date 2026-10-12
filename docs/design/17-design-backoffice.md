# Design — Backoffice e Operações

> **Contexto novo**: não existe PRD correspondente.
> **Camada**: construtiva
> **Status**: proposta

## 1. Por que este contexto precisa existir

Os PRDs produzem trabalho operacional sem definir onde ele é executado:

| Origem | O que é produzido | Destino hoje |
| --- | --- | --- |
| `04-prd-receivables-gateway.md` | `UNKNOWN_OUTCOME`, `FINAL_FAILURE`, "DLQ operacional" | Indefinido |
| `04-prd-receivables-gateway.md` | `ExternalSchemaMismatchDetected` — "suspender operação incompatível e alertar" | Alerta para quem? |
| `09-prd-effects-orchestrator.md` | `EffectDisputed`, `PARTIALLY_EFFECTIVE` | Indefinido |
| `10-prd-funding.md` | `FundingMismatchDetected`, `DisbursementReversed` | Indefinido |
| `11-prd-settlement-reconciliation.md` | `ReconciliationCase` com "responsável e SLA" | Responsável indefinido |
| `03-prd-consent-mandates.md` | `AuthorizationSyncFailed` | Indefinido |
| `02-prd-kyb-kyc.md` | `DiscrepancyDetected` bloqueante | Indefinido |
| `13-prd-partner-management.md` | `PartnerCertificateExpiring` | Indefinido |

Um sistema que depende de infraestrutura externa assíncrona e parcialmente
confiável **gera exceção como modo normal de operação**. Sem ferramenta, a
exceção é tratada por acesso direto ao banco — o que destrói simultaneamente a
trilha de auditoria, o controle de acesso e a invariante de que ajuste não altera
dado bruto.

## 2. Princípios

1. **Nada se resolve fora da ferramenta.** Acesso direto ao banco de produção para
   corrigir dado de negócio é incidente, não procedimento.
2. **Toda ação é comando de domínio.** O backoffice invoca os mesmos casos de uso
   do fluxo normal, com autorização diferente. Não escreve estado diretamente.
3. **Ajuste nunca sobrescreve.** Cria registro de ajuste vinculado ao original
   (`11-prd`, invariante).
4. **Acesso a dado de cliente exige motivo.** Vinculado a caso aberto
   (`94-seguranca-threat-model.md`, C7.5).
5. **A ferramenta não inventa capacidade.** Se o domínio não permite uma
   transição, o backoffice também não.

## 3. Modelo de domínio

| Agregado | Papel |
| --- | --- |
| `OperationalCase` | Unidade de trabalho: origem, tipo, severidade, responsável, prazo, estado |
| `CaseAction` | Ação executada no caso: tipo, autor, justificativa, resultado |
| `CaseEvidence` | Material anexado: consulta, resposta, comunicação |
| `EscalationRule` | Quando e para quem escalar |
| `WorkQueue` | Agrupamento por tipo e severidade, com política de atribuição |

### 3.1 `OperationalCase`

| Campo | Tipo | Observação |
| --- | --- | --- |
| `id` | ID | |
| `tenantId` | ID | Tenant afetado |
| `caseType` | enum | Ver 4 |
| `severity` | enum | `LOW`, `MEDIUM`, `HIGH`, `CRITICAL` |
| `sourceEventId` | ID | Evento que originou |
| `subjects` | objeto[] | Entidades relacionadas: contrato, efeito, ativo, organização |
| `financialImpact` | `MonetaryAmount` | Nulo quando não aplicável |
| `status` | enum | `OPEN`, `ASSIGNED`, `IN_PROGRESS`, `PENDING_EXTERNAL`, `PENDING_APPROVAL`, `RESOLVED`, `CLOSED` |
| `assigneeUserId` | ID | |
| `dueAt` | instante | Derivado da severidade |
| `resolution` | objeto | Código, justificativa, aprovadores |

**`PENDING_EXTERNAL` como estado próprio**: grande parte dos casos aguarda
terceiro — resposta da registradora, retorno do financiador. Sem estado próprio,
esses casos poluem a fila ativa e estouram prazo por motivo alheio ao operador. O
relógio de prazo pausa em `PENDING_EXTERNAL`.

## 4. Catálogo de casos

| Tipo | Origem | Severidade | Prazo alvo | Ação típica |
| --- | --- | --- | --- | --- |
| `EXTERNAL_OUTCOME_UNKNOWN` | `ExternalOutcomeUnknown` | `HIGH` | 4 h | Consultar estado; resolver ou escalar |
| `EFFECT_REJECTED` | `EffectRejected` | `HIGH` | 4 h | Analisar motivo; cancelar contrato ou corrigir e reenviar |
| `EFFECT_PARTIAL` | `EffectPartiallyConfirmed` | `HIGH` | 4 h | Decidir entre instrução proporcional e revisão contratual |
| `EFFECT_DISPUTED` | `EffectDisputed` | `CRITICAL` | 2 h | Acionar parceiro e jurídico |
| `FUNDING_MISMATCH` | `FundingMismatchDetected` | `CRITICAL` | 2 h | Confirmar valor e destino; acionar financiador |
| `DISBURSEMENT_REVERSED` | `DisbursementReversed` | `CRITICAL` | 2 h | Avaliar impacto sobre o efeito |
| `RECONCILIATION_MISMATCH` | `ReconciliationMismatchDetected` | Por valor | 1 dia útil | Investigar; ajustar com justificativa |
| `BENEFICIARY_DIVERGENCE` | `ReconciliationMismatchDetected` | `CRITICAL` | 2 h | Suspeita de fraude; escalar a risco |
| `CONSENT_SYNC_FAILED` | `AuthorizationSyncFailed` | `MEDIUM` | 1 dia útil | Reprocessar; contatar parceiro |
| `KYB_DISCREPANCY` | `DiscrepancyDetected` | `MEDIUM` | 2 dias úteis | Solicitar informação ao cliente |
| `SCHEMA_MISMATCH` | `ExternalSchemaMismatchDetected` | `HIGH` | 4 h | Suspender capacidade; acionar parceiro |
| `CERTIFICATE_EXPIRING` | `PartnerCertificateExpiring` | `MEDIUM` | Antes do vencimento | Coordenar rotação |
| `DLQ_MESSAGE` | Fila morta | `MEDIUM` | 1 dia útil | Analisar; reprocessar ou descartar com justificativa |
| `OUTBOX_STALLED` | Alarme técnico | `HIGH` | 1 h | Investigar; desbloquear |

**Severidade de `RECONCILIATION_MISMATCH` por valor**: divergência de centavos e
divergência de milhares não merecem o mesmo tratamento. Limiar configurável.

**`BENEFICIARY_DIVERGENCE` sempre crítico, independentemente do valor**: pagamento
a beneficiário errado é sinal de fraude ou de falha na constituição do efeito.
Valor baixo pode ser teste de um ataque maior.

## 5. Ações e alçadas

| Ação | Alçada | Exige justificativa | Exige segundo aprovador |
| --- | --- | --- | --- |
| Consultar estado externo | Operador | não | não |
| Reprocessar mensagem de fila morta | Operador | sim | não |
| Reenviar comando externo | Operador | sim | **sim, e exige identificador da consulta prévia** |
| Suspender capacidade de parceiro | Supervisor | sim | não |
| Registrar ajuste de conciliação abaixo do limiar | Operador | sim | não |
| Registrar ajuste acima do limiar | Supervisor | sim | sim |
| Cancelar contrato antes da eficácia | Supervisor | sim | sim |
| Solicitar liberação de efeito | Supervisor | sim | sim |
| Bloquear organização por risco | Analista de risco | sim | não |
| Emitir dossiê de auditoria | Supervisor | sim | sim |
| Acessar dado de cliente sem caso aberto | Operador | sim, com motivo | não, mas gera registro destacado |

**Reenvio de comando exige identificador da consulta prévia** — imposição mecânica
do controle C9.3 (`94-seguranca-threat-model.md`). A operação não aceita reenvio
sem o parâmetro; o procedimento deixa de depender de disciplina.

## 6. Interface

### 6.1 Fila de trabalho

Ordenação padrão: severidade decrescente, depois prazo crescente. Casos vencidos
no topo, destacados.

Filtros: tipo, severidade, responsável, parceiro, tenant, faixa de impacto
financeiro, idade.

**Requisito**: a fila mostra casos de todos os tenants. É o único lugar da
plataforma com essa característica, e por isso concentra o maior risco de
vazamento — daí os controles da seção 5 e o registro de todo acesso.

### 6.2 Detalhe do caso

Deve reunir, sem exigir navegação:

- linha do tempo da operação afetada (do ledger, por `correlationId`);
- estado atual de cada contexto envolvido;
- payloads bruto e normalizado da interação externa;
- ações já executadas, com autor e justificativa;
- ações disponíveis, com a alçada de cada uma;
- casos relacionados (mesmo cliente, mesmo parceiro, mesma janela).

**"Casos relacionados" é o que transforma tratamento pontual em detecção.** Cinco
divergências do mesmo parceiro na mesma hora não são cinco casos independentes —
é um incidente. Sem a correlação, cada operador resolve o seu e ninguém vê o
padrão.

### 6.3 Painel operacional

| Indicador | Por quê |
| --- | --- |
| Casos abertos por tipo e severidade | Carga |
| Casos vencidos | Cumprimento de prazo |
| Idade do caso mais antigo por tipo | Detecta caso esquecido |
| Comandos em `UNKNOWN_OUTCOME` por faixa de idade | Métrica mais importante (`93-requisitos-nao-funcionais.md`, 8) |
| Taxa de erro por parceiro | Degradação |
| Profundidade e idade da outbox | Conformidade (`93-requisitos-nao-funcionais.md`, 4.2) |
| Casos por mil operações | Qualidade estrutural |

**"Casos por mil operações" é o indicador de saúde de produto**, não apenas
operacional. Crescimento significa que o produto está gerando trabalho manual mais
rápido do que gera receita — insustentável, e frequentemente invisível nas
métricas de negócio.

## 7. Integração com o ledger

Toda ação do backoffice produz `AuditRecord` com: ator, papel, caso, ação,
justificativa, estado anterior e posterior, aprovadores.

**Registro obrigatório de acesso a dado de cliente**, mesmo somente leitura
(`12-prd`, requisito 6 — "registrar acesso e exportação das próprias evidências").

## 8. O que o backoffice não faz

| Proibido | Razão |
| --- | --- |
| Alterar dado de negócio diretamente | Viola princípio 2 |
| Executar transição que o domínio não permite | Viola princípio 5 |
| Aprovar crédito ou alterar condição de contrato | Não é papel da plataforma |
| Alterar dado bruto recebido de parceiro | Viola invariante de `11-prd` |
| Remover registro de auditoria | Viola invariante de `12-prd` |
| Operar em nome do cliente sem mandato | Viola `03-prd` |

**O último merece atenção**: é tentador permitir que o operador "ajude" o cliente
executando ação por ele. Sem mandato registrado, isso é ação sem poder — e
contamina toda a cadeia de evidência que a plataforma existe para produzir.

## 9. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| B1 | Dimensionamento da equipe depende de volume de casos, desconhecido | Alto |
| B2 | Limiares de alçada por valor precisam de definição | Médio |
| B3 | Prazos alvo são propostas sem base histórica | Médio |
| B4 | Integração com ferramenta externa de atendimento não decidida | Baixo |

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 1 | `03`, `04`, `09`, `10`, `11`, `13` | Trabalho operacional sem destino |
| 4 | Idem | Casos sem catálogo, severidade ou prazo |
| 5 | `11-prd-settlement-reconciliation.md` | "Justificativa e autorização" sem alçada |
| 5 | `04`, `09` | Procedimento de reenvio sem imposição mecânica |
| 7 | `12-prd-compliance-ledger.md` | Ação operacional sem trilha |
