# Arquitetura Técnica

> **Camada**: construtiva
> **Origem**: lacuna identificada — nenhum dos 15 PRDs define estratégia de
> implantação, multi-tenancy, persistência, mensageria ou stack. O
> `14-catalogo-contratos-eventos.md` pressupõe outbox transacional, inbox e
> schema registry sem definir como nada disso é provido.
> **Status**: proposta. As decisões marcadas como **[ABERTA]** exigem
> confirmação antes de virarem premissa de construção.

## 1. O que a arquitetura precisa resolver

Os PRDs não são neutros do ponto de vista arquitetural. Cinco invariantes,
espalhadas por documentos diferentes, determinam quase tudo:

| Invariante | Origem | Consequência arquitetural |
| --- | --- | --- |
| "A infraestrutura externa competente é a fonte de verdade do ativo" | `00-prd`, regra 1 | O estado local é sempre projeção. Nenhuma decisão irreversível pode se basear em dado local sem revalidação. |
| "Não existe rollback técnico de um efeito já constituído" | `14-catalogo`, sagas | Sem transação distribuída. Toda compensação é de negócio e assíncrona. |
| "`UNKNOWN_OUTCOME` nunca é reenviado sem consulta de estado" | `04-prd`, `09-prd` | Retry cego é proibido. Todo comando externo exige operação de consulta pareada. |
| "Snapshot publicado é imutável" | `05-prd` | Escrita append-only para posições; versionamento como conceito de primeira classe. |
| "Ledger não aceita atualização destrutiva" | `12-prd` | Armazenamento com garantia de imutabilidade, separado do transacional. |

Some-se a isso a característica dominante do domínio: **a plataforma é
majoritariamente um orquestrador de sistemas externos lentos, assíncronos e
parcialmente confiáveis**. O volume de computação própria é baixo; o volume de
coordenação de falha é alto. Isso empurra a arquitetura para durabilidade de
processo, não para throughput.

## 2. Princípios

1. **Projeção local nunca é autoridade.** Todo dado externo carrega `asOf`,
   `source` e `providerReference`. Consulta local serve para exibir e filtrar;
   decidir efeito exige revalidação.
2. **Comando externo é durável antes de ser enviado.** A intenção é persistida,
   depois transmitida. Nunca o inverso.
3. **Idempotência é contrato, não otimização.** Toda operação externa tem chave
   idempotente derivada de dados de negócio estáveis.
4. **Ambiguidade é estado, não exceção.** `UNKNOWN_OUTCOME` é um estado
   modelado, com tratamento próprio, e não um erro a ser engolido.
5. **Fronteira de contexto é fronteira de dado.** Contextos não compartilham
   tabelas. Integração é por evento ou por porta explícita.
6. **Evidência é produto.** O ledger não é log. É entregável contratual.

## 3. Estilo arquitetural

### 3.1 Decisão: monolito modular com fronteiras rígidas **[ABERTA]**

**Recomendação**: iniciar como monolito modular — um processo de implantação,
módulos internos correspondendo 1:1 aos bounded contexts, comunicação interna
exclusivamente por eventos e portas, banco lógico segregado por schema.

**Razão**: os 13 contextos dos PRDs (mais os 5 propostos nesta camada) são
fronteiras *de domínio*, não necessariamente *de implantação*. Distribuí-los
desde o início traz custo de operação — rastreamento distribuído, consistência
eventual entre serviços, versionamento de contrato interno — antes de existir
volume ou time que justifique.

**Critério de extração**: um módulo vira serviço quando satisfizer pelo menos um
destes, não antes:

- perfil de carga incompatível com o resto (o Gateway é o candidato natural:
  concorrência alta de I/O, janela de retry longa, isolamento de falha);
- requisito de isolamento de segurança (o Compliance Ledger é candidato:
  superfície de escrita mínima, leitura auditada, chaves segregadas);
- time dedicado com cadência de release própria.

**Candidatos a extração precoce** (nesta ordem): `Receivables Gateway`,
`Compliance Ledger`.

**Risco da recomendação**: monolito modular só funciona se a fronteira for
mecanicamente imposta. Sem isso, degrada para monolito comum em poucos meses.
Ver 3.2.

### 3.2 Imposição mecânica da fronteira

A fronteira precisa ser verificável em build, não confiada à disciplina.

| Regra | Como impor |
| --- | --- |
| Módulo não importa tipo interno de outro módulo | Teste de arquitetura em CI que falha o build ao detectar import cruzado fora do pacote público do módulo |
| Módulo não lê tabela de outro módulo | Usuário de banco por módulo, com `GRANT` restrito ao próprio schema |
| Comunicação só por evento ou porta | O pacote público de cada módulo expõe apenas portas e tipos do catálogo compartilhado |
| Evento publicado corresponde ao schema registrado | Validação contra o registry no momento da publicação, falhando a transação |

**Sem essas quatro, a recomendação 3.1 deve ser revista para microsserviços
desde o início** — porque a fronteira de processo passa a ser a única imposição
disponível.

## 4. Multi-tenancy

`01-prd-identity-organization.md` estabelece: "todo recurso de negócio pertence a
exatamente um tenant". Os PRDs não dizem como.

### 4.1 Decisão: banco compartilhado com isolamento por linha **[ABERTA]**

| Estratégia | Isolamento | Custo operacional | Adequação |
| --- | --- | --- | --- |
| Banco por tenant | Máximo | Proibitivo acima de dezenas de tenants | Não |
| Schema por tenant | Alto | Alto (migração multiplicada) | Não |
| **Linha com `tenant_id` + RLS** | Médio-alto | Baixo | **Recomendado** |
| Linha com `tenant_id` sem RLS | Baixo | Mínimo | Não — depende de o código nunca errar |

**Recomendação**: `tenant_id` obrigatório em toda tabela de negócio, com
*row-level security* ativa no banco e contexto de sessão definido pela camada de
aplicação a cada requisição.

**Razão para RLS e não só filtro em código**: o filtro em código falha em silêncio
— uma consulta esquecida vaza dados entre tenants sem erro. Com RLS, a consulta
esquecida retorna vazio. Falha visível é preferível a vazamento silencioso, e
este domínio tem financiadores concorrentes na mesma base.

**Implicações**:

- toda conexão define `SET LOCAL app.tenant_id` no início da transação;
- processos assíncronos (consumo de evento, job) propagam `tenantId` do envelope
  e definem o contexto antes de qualquer consulta;
- operações de plataforma (backoffice) usam papel específico com política de RLS
  distinta, e todo acesso cross-tenant é registrado no ledger.

### 4.2 Tenant não é organização

Distinção necessária que os PRDs não fazem explicitamente: uma organização
financiadora e uma organização cedente não devem compartilhar tenant, mas o
financiador precisa ver dados do pedido do cedente.

**Modelo proposto**: o `tenant_id` identifica o *domínio de isolamento*. O acesso
de um financiador a dados de um cedente não ocorre por compartilhamento de tenant,
mas por **concessão explícita e versionada** (`PartnerInvitation` com escopo de
dados), materializada como projeção no tenant do financiador. O dado é copiado
sob autorização, não lido através da fronteira.

Isso tem custo (duplicação) e benefício (revogação e escopo se tornam
verificáveis, e o vazamento por consulta errada deixa de ser possível).

## 5. Persistência

### 5.1 Decisão: PostgreSQL como banco primário **[ABERTA]**

**Razão**: RLS nativa (4.1), transacional forte para o padrão outbox (6.1),
suporte a JSONB para payloads externos preservados sem perda (`04-prd`, requisito
"payload normalizado nunca elimina o payload original"), particionamento
declarativo para as tabelas de alto volume, e extensões de índice adequadas a
consulta temporal.

### 5.2 Classes de dado e tratamento

| Classe | Exemplo | Padrão | Retenção |
| --- | --- | --- | --- |
| Transacional mutável | `ContractCase`, `FundingRequest` | Tabela normal, atualização in-place | Ciclo de vida + retenção legal |
| Versionado imutável | `PortfolioSnapshot`, `EligibilityPolicy`, `ConsentGrant` | Append-only, nova linha por versão, sem `UPDATE` | Retenção legal |
| Evidência bruta externa | payload de request/response de parceiro | Object storage, referência no banco | Definida por obrigação |
| Registro de auditoria | `AuditRecord` | Append-only com garantia de imutabilidade | Definida por categoria |
| Projeção de leitura | visão de portfólio para a interface | Materializada, reconstruível | Descartável |

**Regra**: nenhuma tabela das classes "versionado imutável", "evidência" e
"auditoria" recebe `UPDATE` ou `DELETE` pela aplicação. Imposição por permissão
de banco, não por convenção.

### 5.3 Volume e particionamento

O volume concentra-se em dois lugares: unidades de recebível e registros de
auditoria.

`CardReceivableUnit` cresce com o número de cedentes multiplicado pelo horizonte
da agenda multiplicado pela frequência de atualização. Uma agenda de cartões
típica cobre até 365 dias à frente e muda diariamente. Se cada atualização gera
nova versão, o crescimento é rápido.

**Estratégia proposta**:

- particionar `card_receivable_unit` por data de liquidação prevista (mensal);
- versionar apenas em mudança material — definir o que é material (alteração de
  valor líquido, de data, de titularidade, de gravame), não gerar versão para
  reingestão idêntica; a comparação é por hash do conteúdo normalizado;
- reter snapshots integralmente apenas quando referenciados por oferta, contrato
  ou efeito; snapshots de simulação têm retenção curta.

**Lacuna a resolver**: `05-prd-portfolio.md` requisito 2 diz "criar nova versão
quando houver alteração material" sem definir material. A definição acima é
proposta e precisa de validação — ela determina diretamente o custo de
armazenamento.

## 6. Mensageria e consistência

### 6.1 Outbox transacional

`14-catalogo-contratos-eventos.md` exige "produtores usam transactional outbox"
sem especificar. Definição construtiva:

1. A transação de negócio grava o estado e insere a linha na tabela `outbox` do
   próprio módulo, atomicamente.
2. Um processo relay lê a outbox em ordem de inserção por agregado e publica no
   broker.
3. A publicação é confirmada marcando a linha; falha mantém a linha para nova
   tentativa.
4. Entrega é *pelo menos uma vez*. Duplicata é tratada pelo consumidor.

**Ordenação**: garantida por agregado, conforme regra 6 do catálogo. A chave de
partição do broker é `aggregateId`. Ordem global não existe e não deve ser
presumida por nenhum consumidor.

### 6.2 Inbox e idempotência de consumo

Consumidor mantém tabela `inbox` com `eventId` como chave primária. O
processamento e a inserção no inbox ocorrem na mesma transação. `eventId`
repetido é descartado silenciosamente, com contador.

**Requisito derivado não presente nos PRDs**: o inbox precisa de política de
expurgo. Reter `eventId` indefinidamente é inviável; expurgar cedo demais
reabre a janela de duplicata. Proposta: reter pelo dobro do tempo máximo de
retenção do broker, e registrar a política explicitamente.

### 6.3 Broker **[ABERTA]**

Requisitos: ordenação por chave, retenção configurável, consumo por grupo,
mensagem morta.

Duas famílias atendem — log particionado (Kafka e equivalentes) ou fila com
ordenação por grupo (SQS FIFO e equivalentes). A escolha depende de: necessidade
de reprocessar histórico a partir de offset (favorece log), e volume esperado
(favorece fila gerenciada se baixo).

**Recomendação condicional**: dado que `12-prd-compliance-ledger.md` já provê
reprodução histórica pelo ledger, o broker não precisa ser a fonte de replay.
Isso torna uma fila gerenciada suficiente e mais barata de operar no início.

### 6.4 O que NÃO usar

Sem transação distribuída, sem *two-phase commit*, sem saga com rollback técnico.
`14-catalogo` já estabelece que compensação é de negócio. Concretamente:

| Falha | Compensação correta | Compensação proibida |
| --- | --- | --- |
| Contrato assinado, efeito rejeitado | Cancelar contrato com registro do motivo, comunicar partes | Apagar o contrato |
| Efeito confirmado, desembolso falhou | Manter efeito, abrir caso, reprocessar desembolso ou solicitar liberação do efeito | Desfazer o efeito localmente |
| Efeito parcialmente confirmado | Revisar instrução de desembolso para a parte confirmada, ou revisar contrato | Tratar como confirmado integral |

## 7. Integração externa

### 7.1 Estrutura do adaptador

`04-prd-receivables-gateway.md` define cinco portas canônicas. Construtivamente,
cada adaptador de parceiro implementa o subconjunto declarado em seu
`CapabilityProfile`, e o domínio consulta a capacidade antes de invocar — nunca
tenta e trata o erro.

```text
Domínio
  -> Porta canônica (interface estável)
      -> Resolução de adaptador por Partner + Capability + versão
          -> Adaptador do parceiro
              -> Cliente de protocolo (HTTP/mTLS, SFTP, fila)
                  -> Parceiro externo
```

**Regra**: o domínio nunca conhece o nome do parceiro em condicional. Uma
mudança de comportamento por parceiro vira capacidade declarada ou política
configurada, nunca `if (partner == "X")`.

### 7.2 Ciclo de vida do comando externo

```text
CREATED       comando persistido, ainda não enviado
  -> SENT            transmitido, sem confirmação de recebimento
  -> ACKNOWLEDGED    parceiro confirmou recebimento (não é eficácia)
  -> CONFIRMED       parceiro confirmou o efeito
```

Cada transição registra `DeliveryAttempt` com instante, resultado e payload.

**Tratamento de `UNKNOWN_OUTCOME`** — o caso mais delicado de todo o sistema:

1. Timeout, erro de rede ou resposta não interpretável após envio produz
   `UNKNOWN_OUTCOME`, nunca `FINAL_FAILURE`.
2. O estado bloqueia todo avanço dependente (Funding, notadamente).
3. Resolução exclusivamente por **consulta de estado** ao parceiro, usando a
   referência do comando. Nunca por reenvio.
4. Se o parceiro não oferecer operação de consulta para aquela operação, o
   `CapabilityProfile` deve declará-lo, e o comando correspondente é classificado
   como **não reenviável sem intervenção humana** — vai para fila de P6.
5. Persistindo a ambiguidade além do limite configurado, abre-se caso operacional
   com prazo.

**Implicação para contratação de parceiro**: a existência de operação de consulta
de estado deveria ser requisito de homologação em `13-prd-partner-management.md`.
Hoje não é. Sem ela, todo comando àquele parceiro é potencialmente irrecuperável
automaticamente.

### 7.3 Chave de idempotência

Derivada de dados de negócio estáveis, nunca de identificador gerado por
tentativa:

```text
idempotencyKey = hash(tenantId, partnerId, operationType, businessKey, payloadVersion)
```

Onde `businessKey` é o identificador do objeto de negócio (contrato, efeito,
instrução de desembolso) e `payloadVersion` muda apenas quando o conteúdo
materialmente muda. Reenvio idêntico produz a mesma chave; alteração de conteúdo
produz chave diferente — e alteração de conteúdo após envio é, por si, um evento
que exige revisão, não reenvio.

### 7.4 Resiliência

| Mecanismo | Escopo | Cuidado |
| --- | --- | --- |
| Timeout | Por operação, não por parceiro | Timeout curto demais gera `UNKNOWN_OUTCOME` desnecessário |
| Retry com backoff | Somente operações classificadas `RETRYABLE` | Nunca aplicar a comando sem consulta de estado pareada |
| Circuit breaker | Por parceiro e operação | Não pode mascarar falha definitiva — abrir o circuito gera evento, não silêncio |
| Rate limit | Por parceiro, conforme `OperationalPolicy` | Precisa ser distribuído se houver múltiplas instâncias |
| Bulkhead | Pool de conexão isolado por parceiro | Impede que um parceiro lento consuma a capacidade dos demais |

**Regra sobre timeout**: em operação de leitura, timeout produz erro recuperável.
Em operação de escrita sem consulta de estado disponível, timeout produz
`UNKNOWN_OUTCOME` e não pode ser retentado. Essa distinção precisa estar no
adaptador, por operação.

## 8. Tempo

`04-prd-receivables-gateway.md` menciona "relógio UTC combinado com data de
negócio informada pelo parceiro" sem desenvolver. Construtivamente, três noções
de tempo coexistem e não devem ser confundidas:

| Noção | Origem | Uso |
| --- | --- | --- |
| Instante de ocorrência (`occurredAt`) | Relógio do produtor do fato | Ordenação causal dentro do agregado |
| Instante de registro (`recordedAt`) | Relógio da plataforma na ingestão | Auditoria, detecção de atraso |
| Data de negócio (`businessDate`) | Calendário do parceiro / dia útil | Vencimento, liquidação, janela de corte |

**Regras**:

- todo timestamp persistido é UTC com precisão de milissegundo;
- toda data de negócio é data pura, sem fuso, acompanhada do calendário que a
  qualifica (o dia útil de um credenciador pode não coincidir com o de outro);
- diferença relevante entre `occurredAt` e `recordedAt` é sinal operacional e
  deve ser observada, não normalizada em silêncio;
- nenhuma regra de negócio compara timestamp de produtores diferentes assumindo
  relógios sincronizados.

## 9. Segurança

Detalhamento em `docs/design/94-seguranca-threat-model.md`. Decisões estruturais
aqui:

| Aspecto | Decisão |
| --- | --- |
| Autenticação de usuário | OIDC com provedor de identidade externo **[ABERTA]**; MFA obrigatório para `APPROVER`, `ORG_ADMIN` e todos os papéis de plataforma |
| Autenticação de parceiro | mTLS com certificado por parceiro e ambiente; rotação com sobreposição |
| Autorização | Combinação de tenant, papel, recurso e representação vigente, avaliada por requisição |
| Segredos | Cofre externo; o domínio referencia por identificador, nunca armazena valor (`13-prd`, invariante) |
| Criptografia em repouso | Padrão para todo armazenamento; envelope adicional para documento pessoal e dossiê |
| Criptografia em trânsito | TLS 1.3 mínimo; mTLS para parceiro |
| PII em log | Proibida. Mascaramento aplicado na serialização, não na gravação — o dado não chega ao log |

## 10. Observabilidade

Três sinais, com propósitos distintos:

**Rastro distribuído**: `correlationId` do envelope de evento propaga para o
rastro. Uma operação completa — da solicitação ao desembolso — deve ser
reconstruível pelo `correlationId` em minutos.

**Métricas**: os PRDs listam 60+ métricas de produto. Elas não substituem
métricas técnicas. O conjunto mínimo técnico:

- latência e taxa de erro por parceiro, operação e código de retorno (percentis,
  não média — média esconde a cauda que gera caso operacional);
- profundidade de outbox e atraso de publicação;
- profundidade de inbox e taxa de duplicata descartada;
- contagem de comandos em `UNKNOWN_OUTCOME` por idade — esta é a métrica de
  saúde mais importante do sistema;
- casos operacionais abertos por tipo e idade.

**Log estruturado**: sem PII, com `tenantId`, `correlationId` e `causationId`
em todo registro.

**Distinção necessária**: o Compliance Ledger não é observabilidade. Ele tem
requisito de integridade, retenção e valor probatório que nenhuma stack de
observabilidade oferece. `12-prd` já diz "uma operação pode ser reproduzida sem
consultar logs voláteis" — os dois sistemas coexistem e não se substituem.

## 11. Ambientes

| Ambiente | Parceiros | Dados | Propósito |
| --- | --- | --- | --- |
| Desenvolvimento | Simulados | Sintéticos | Desenvolvimento local |
| Integração | Homologação do parceiro | Sintéticos | Teste de contrato |
| Homologação | Homologação do parceiro | Anonimizados | Validação de negócio, homologação de parceiro |
| Produção | Produção | Reais | Operação |

**Regra**: credencial de produção nunca existe fora de produção. `13-prd`
requisito 4 exige vínculo de credencial por ambiente — construtivamente, isso
significa cofres separados por ambiente, sem caminho de leitura entre eles.

**Lacuna**: nenhum PRD trata de dados de teste. Testar conciliação e efeitos
exige cenários com dados de recebível realistas. Anonimizar agenda de recebíveis
preservando a estrutura de datas e valores é trabalho não trivial e precisa de
decisão.

## 12. Decisões abertas

Cada uma precisa virar um ADR antes do início da construção.

| # | Decisão | Impacto se adiada |
| --- | --- | --- |
| D1 | Monolito modular vs. microsserviços (3.1) | Alto — define estrutura de código e implantação |
| D2 | Estratégia de multi-tenancy (4.1) | Alto — retrofit de RLS em base existente é caro |
| D3 | Banco primário (5.1) | Alto |
| D4 | Broker (6.3) | Médio — abstraível por porta |
| D5 | Provedor de identidade (9) | Médio |
| D6 | Definição de "alteração material" em portfólio (5.3) | Alto — define custo de armazenamento |
| D7 | Tecnologia de imutabilidade do ledger (5.2) | Médio |
| D8 | Linguagem e plataforma de execução | Alto — não há indicação nos PRDs |
| D9 | Política de expurgo do inbox (6.2) | Baixo |
| D10 | Estratégia de dados de teste (11) | Médio |

## 13. O que esta arquitetura deliberadamente não faz

- **Não busca alta escala horizontal no início.** O gargalo é o parceiro externo,
  não a plataforma.
- **Não implementa motor de regras genérico.** `06-prd-eligibility.md` pede
  regras versionadas e explicáveis — isso se resolve com políticas versionadas em
  código e dados, não necessariamente com um motor configurável em runtime. Ver
  `docs/design/06-design-eligibility.md`.
- **Não assume disponibilidade dos parceiros.** Indisponibilidade de
  infraestrutura é modo de operação esperado, não incidente.
- **Não trata a plataforma como sistema de registro.** Ela é sistema de
  orquestração e evidência. Essa distinção deve permanecer explícita no código.

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 3 | Todos | Estratégia de implantação inexistente |
| 4 | `01-prd-identity-organization.md` | "Todo recurso pertence a um tenant" sem mecanismo |
| 5 | `05-prd-portfolio.md`, `12-prd-compliance-ledger.md` | Persistência e volume |
| 6 | `14-catalogo-contratos-eventos.md` | Outbox e inbox pressupostos sem definição |
| 7 | `04-prd-receivables-gateway.md` | Ciclo de comando e `UNKNOWN_OUTCOME` sem procedimento |
| 8 | `04-prd-receivables-gateway.md` | "Relógio UTC + data de negócio" sem desenvolvimento |
| 10 | Todos | Métricas de produto sem métricas técnicas |
| 11 | `13-prd-partner-management.md` | Ambientes citados sem definição |
