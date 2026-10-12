# Design — Receivables Gateway

> **PRD de origem**: `docs/prd/04-prd-receivables-gateway.md`
> **Camada**: construtiva
> **Status**: proposta
> **Complementa**: `00-arquitetura-tecnica.md` seção 7 (ciclo de comando,
> idempotência, resiliência) — não repetido aqui.

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| G1 | `CapabilityProfile` sem estrutura | "Cada adaptador declara capacidades" — sem dizer quais nem como |
| G2 | Portas canônicas sem assinatura | Cinco nomes, nenhuma operação |
| G3 | "Normalizar erros em categorias" sem taxonomia | Req. 6 |
| G4 | "Detectar quebra de schema" sem mecanismo | Req. 8 |
| G5 | "Reprocessamento controlado" sem procedimento | Req. 9 |
| G6 | Tradução de códigos externos sem estratégia | Escopo |
| G7 | Nenhum provedor real mapeado | Todo o documento é abstrato |

## 2. Capacidades

O PRD estabelece que capacidade declarada substitui condicional espalhada. Para
isso, a declaração precisa ser específica o bastante para que o domínio decida
sem conhecer o parceiro.

### 2.1 Estrutura

```text
CapabilityProfile
  ├─ partnerId, environment, version, effectiveFrom
  └─ operations[]
       ├─ operation          (identificador canônico)
       ├─ supported          (booleano)
       ├─ protocol           (SYNC_HTTP | ASYNC_CALLBACK | FILE_BATCH | POLLING)
       ├─ stateQuerySupported(booleano — decisivo)
       ├─ idempotencySupport (NATIVE | KEY_HEADER | NONE)
       ├─ partialResultSupported
       ├─ assetTypes[]
       ├─ limits             { maxItemsPerRequest, maxRequestsPerMinute, maxPayloadBytes }
       ├─ schedule           { businessDays, cutoffTime, processingWindow }
       └─ dataQuality        { encumbranceCoverage, positionFreshnessSla }
```

### 2.2 Os três campos que mudam o comportamento do domínio

**`stateQuerySupported`** — se falso, nenhum comando de escrita àquele parceiro é
automaticamente recuperável de `UNKNOWN_OUTCOME`. Todo timeout vira caso humano.
Isso deveria ser critério de homologação em `13-prd-partner-management.md`, e não
é.

**`dataQuality.encumbranceCoverage`** — `COMPLETE`, `PARTIAL` ou `UNAVAILABLE`.
Alimenta a regra `AST-ENC-002` de `06-design-eligibility.md`. Um parceiro que não
informa gravames de forma completa inviabiliza operar com seus ativos; declarar
isso é preferível a descobrir na primeira dupla negociação.

**`partialResultSupported`** — se o parceiro pode confirmar parte dos alvos. Se
falso, `EffectPartiallyConfirmed` nunca deveria ocorrer e sua ocorrência é
inconsistência que exige investigação, não tratamento normal.

### 2.3 Uso

O domínio consulta a capacidade **antes** de invocar:

```text
if (!capability.supports(operation, assetType)) {
    -> CAPABILITY_NOT_ENABLED, sem chamada externa
}
```

Nunca tentar e interpretar o erro. Tentativa contra capacidade inexistente gera
erro do parceiro, consome cota e polui métrica.

## 3. Portas canônicas

Assinaturas conceituais; a forma concreta depende da linguagem escolhida (decisão
D8 de `00-arquitetura-tecnica.md`).

### 3.1 `ReceivablesQueryPort`

| Operação | Entrada | Saída |
| --- | --- | --- |
| `discoverSchedule` | organização, tipos de ativo, janela, referências externas | posições + metadados de origem e cobertura |
| `queryAsset` | referência do ativo | posição de um ativo |
| `queryOwnership` | referência do ativo | titular atual conforme a fonte |
| `queryEncumbrances` | referência do ativo | efeitos registrados |

### 3.2 `AuthorizationPort`

| Operação | Observação |
| --- | --- |
| `activateOptIn` | Cria ou reflete autorização na infraestrutura |
| `queryOptInStatus` | Situação da autorização |
| `revokeOptIn` | Revogação |
| `listAuthorizations` | Autorizações vigentes para a organização |

### 3.3 `RegulatoryEffectsPort`

| Operação | Observação |
| --- | --- |
| `submitEffect` | Comando idempotente; retorna aceite, não eficácia |
| `queryEffect` | **Obrigatória para recuperação de `UNKNOWN_OUTCOME`** |
| `requestRelease` | Liberação |
| `queryEffectsByAsset` | Efeitos incidentes sobre um ativo |

### 3.4 `SettlementQueryPort`

| Operação | Observação |
| --- | --- |
| `querySettlements` | Liquidações em uma janela |
| `querySettlementByAsset` | Por ativo |

### 3.5 `ExternalEvidencePort`

| Operação | Observação |
| --- | --- |
| `fetchEvidence` | Documento ou comprovante da fonte |
| `storeInteraction` | Persiste request e response brutos |

**Regra transversal**: toda saída carrega `asOf`, `providerId` e a referência
externa. Porta que retorna valor sem proveniência quebra a invariante "a
infraestrutura externa é a fonte de verdade" — o consumidor perde a capacidade de
saber o quão antiga é a informação.

## 4. Taxonomia de erros

O PRD exige três categorias (req. 6). Insuficiente na prática — `UNKNOWN` precisa
de subdivisão, porque o tratamento difere.

| Categoria | Significado | Tratamento |
| --- | --- | --- |
| `RETRYABLE_TRANSPORT` | Falha de rede antes de chegar ao parceiro | Retentar com a mesma chave |
| `RETRYABLE_UPSTREAM` | Parceiro indisponível ou limitando | Retentar com espera crescente |
| `FINAL_VALIDATION` | Payload rejeitado por regra do parceiro | Não retentar; corrigir e reenviar com nova chave |
| `FINAL_BUSINESS` | Rejeitado por regra de negócio (ativo indisponível) | Não retentar; caso de negócio |
| `FINAL_AUTHORIZATION` | Credencial inválida ou sem permissão | Não retentar; incidente |
| `UNKNOWN_SENT` | Enviado; resposta não recebida | **Consultar estado** |
| `UNKNOWN_AMBIGUOUS` | Resposta recebida e não interpretável | **Consultar estado** |
| `SCHEMA_MISMATCH` | Resposta fora do contrato | Suspender operação; caso técnico |

**Distinção entre `UNKNOWN_SENT` e `UNKNOWN_AMBIGUOUS`**: no primeiro caso não se
sabe se chegou; no segundo, chegou e não se entendeu. Ambos exigem consulta, mas o
segundo indica problema de contrato com o parceiro e alimenta a detecção de G4.

**Mapeamento**: cada adaptador traduz os códigos do parceiro para esta taxonomia,
em tabela declarada e versionada. Código desconhecido mapeia para
`UNKNOWN_AMBIGUOUS` — o padrão conservador. Mapear desconhecido para
`RETRYABLE` produziria retentativa de comando possivelmente efetivado.

## 5. Detecção de quebra de contrato

O PRD exige detecção (req. 8) sem mecanismo.

| Verificação | Momento | Ação |
| --- | --- | --- |
| Resposta valida contra o schema esperado | Toda resposta | Divergência estrutural gera `SCHEMA_MISMATCH` |
| Campo obrigatório ausente | Toda resposta | Idem |
| Valor de enumeração desconhecido | Toda resposta | **Não é quebra** — registra e segue, se o campo não for decisivo |
| Tipo divergente | Toda resposta | Quebra |
| Versão declarada pelo parceiro difere da homologada | Toda resposta, se declarada | Alerta; quebra se incompatível |

**Valor de enumeração desconhecido não é quebra**: parceiros acrescentam códigos
sem aviso. Falhar por isso derruba a integração por motivo trivial. A exceção:
se o campo determina decisão (o código de situação do ativo, por exemplo), valor
desconhecido é bloqueante para aquele item, não para a integração inteira.

**Ao detectar quebra**:

1. persistir a resposta bruta;
2. suspender a operação específica daquele parceiro (não o parceiro inteiro);
3. emitir `ExternalSchemaMismatchDetected`;
4. abrir caso `SCHEMA_MISMATCH` (`17-design-backoffice.md`);
5. notificar Partner Management.

**Suspender a operação e não o parceiro**: se a resposta de consulta de liquidação
quebrou, a consulta de agenda pode continuar funcionando. Granularidade grossa
transforma um problema pequeno em indisponibilidade total.

## 6. Reprocessamento

O PRD exige reprocessamento controlado preservando o payload original (req. 9).

| Aspecto | Definição |
| --- | --- |
| Fonte | Payload bruto persistido, nunca o normalizado |
| Escopo | Uma interação, ou uma janela por parceiro e operação |
| Efeito | Reexecuta apenas a normalização e a publicação; **nunca** reenvia comando externo |
| Idempotência | Produz os mesmos eventos, deduplicados pelo inbox dos consumidores |
| Autorização | Operação de backoffice com justificativa |
| Registro | Nova `ReconciliationRun` ou execução versionada, sem apagar a anterior |

**Reprocessamento nunca reenvia comando**: reprocessar é reinterpretar o que já se
recebeu. Reenviar é ação externa com consequência jurídica, e é operação distinta
com controles próprios (`17-design-backoffice.md`, seção 5).

**Caso de uso principal**: correção de erro na tradução. Se o adaptador
interpretou errado um campo durante duas semanas, o payload bruto permite
reconstruir a interpretação correta sem consultar o parceiro de novo — e sem
perder o histórico da interpretação errada, que pode ter fundamentado decisões.

## 7. Recepção de webhook

Complementa `94-seguranca-threat-model.md`, T1 e `91-contratos-api.md`, 5.1.

Sequência obrigatória:

```text
1. Verificar origem (mTLS ou assinatura)   -> falha: 401, sem persistir
2. Persistir payload bruto                  -> sempre, antes de interpretar
3. Deduplicar por identificador do parceiro -> repetido: 200, sem reprocessar
4. Enfileirar                                -> sempre
5. Responder 200                             -> em menos de 200 ms (p95)
--- assíncrono ---
6. Correlacionar com comando enviado         -> sem correlação: caso operacional
7. Interpretar e normalizar                  -> falha: SCHEMA_MISMATCH
8. Se confirmação crítica: consultar estado  -> controle C1.3
9. Publicar evento de domínio
```

**Passo 2 antes do 3**: persistir antes de deduplicar garante que se tem o
material mesmo quando a deduplicação está errada.

**Passo 8 apenas para confirmação crítica** — a que libera desembolso. Aplicar a
toda notificação dobraria o tráfego sem ganho proporcional.

## 8. Mapeamento de provedores

**Esta seção depende de `docs/reference/provedores-integracao.md`.** Estrutura a
preencher por provedor:

| Dimensão | O que registrar |
| --- | --- |
| Identificação | Nome, papel, situação de autorização |
| Protocolo | Por operação |
| Autenticação | Mecanismo, gestão de certificado |
| Operações | Mapeadas às portas canônicas |
| Limites | Tamanho, frequência, janela |
| Calendário | Dias úteis, horário de corte |
| Assíncrono | Callback, consulta, arquivo |
| Consulta de estado | **Disponível por operação?** |
| Qualidade de dado | Cobertura de gravame, validade da posição |
| Códigos de erro | Tabela de tradução |
| Homologação | Processo e prazo |

**Até que esta seção esteja preenchida, os adaptadores não podem ser
dimensionados.** É a lacuna bloqueante mais concreta de todo o conjunto: as portas
canônicas são desenho razoável, mas só se confirmam como suficientes quando
confrontadas com dois ou três protocolos reais.

## 9. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| G8 | Nenhum provedor mapeado (8) | **Bloqueante** |
| G9 | Consulta de estado não é critério de homologação | Alto |
| G10 | SLA de validade da posição por fonte desconhecido | Alto |
| G11 | Retenção do payload bruto não definida | Médio |
| G12 | Estratégia de simulação de parceiro para teste | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | Portas canônicas | `CapabilityProfile` sem estrutura |
| 3 | Portas canônicas | Portas sem operações |
| 4 | 6 | Categorias de erro insuficientes |
| 5 | 8 | Detecção de quebra sem mecanismo |
| 6 | 9 | Reprocessamento sem procedimento |
| 7 | 4, 5 | Recepção sem sequência definida |
| 8 | 1 | Nenhum provedor real |
