# Requisitos Não Funcionais

> **Camada**: construtiva
> **Origem**: lacuna identificada — 13 dos 15 PRDs não têm seção de requisitos
> não funcionais. Os dois que têm (`04-prd-receivables-gateway.md` e
> `12-prd-compliance-ledger.md`) listam mecanismos ("rate limiting", "circuit
> breaker", "WORM") sem números. Sessenta e poucas métricas de produto são
> definidas sem um único alvo.
> **Status**: proposta. Os números são pontos de partida fundamentados, não
> medições — precisam ser confrontados com volumetria real antes de virarem
> compromisso.

## 1. Por que os números importam aqui

Uma métrica sem alvo não orienta decisão de engenharia. `00-prd-mapa-de-produto.md`
mede "tempo entre solicitação e primeira oferta" — mas se o alvo é 30 segundos, a
distribuição aos financiadores precisa ser paralela e assíncrona; se é 4 horas,
uma fila com processamento sequencial basta. São arquiteturas diferentes.

Este documento propõe os alvos. Onde a proposta é arbitrária por falta de dado,
isso está declarado.

## 2. Volumetria

### 2.1 Premissas de dimensionamento

Sem dados históricos, as premissas abaixo são **hipóteses de trabalho** derivadas
da estratégia de entrega de `00-prd-mapa-de-produto.md` (quatro fases, iniciando
com um financiador e uma conexão).

| Parâmetro | Fase 1 (MVP) | Fase 3 (marketplace) | Observação |
| --- | --- | --- | --- |
| Organizações cedentes ativas | 50 | 2.000 | |
| Financiadores conectados | 1 | 10 | |
| Infraestruturas conectadas | 1 | 4 | |
| Solicitações por dia | 20 | 800 | |
| Ativos por snapshot (cartão) | 500–2.000 | 500–5.000 | Agenda de até 365 dias |
| Descobertas de portfólio por dia | 100 | 6.000 | Ver 2.2 |
| Comandos externos por dia | 50 | 3.000 | |
| Eventos publicados por dia | 5.000 | 400.000 | |
| Registros de auditoria por dia | 8.000 | 600.000 | |

**O número que define o custo**: descobertas de portfólio. Se cada organização
ativa atualiza a agenda três vezes ao dia e cada agenda tem 2.000 unidades,
2.000 organizações produzem 12 milhões de linhas de unidade por dia, antes de
qualquer deduplicação.

### 2.2 Consequência: descoberta não pode ser ingênua

A estratégia de versionar apenas em mudança material (ver
`90-modelo-de-dados.md`, 3.2) deixa de ser otimização e passa a ser requisito
estrutural. Sem ela, o armazenamento cresce de forma insustentável a partir da
fase 3.

**Requisitos derivados**:

| # | Requisito |
| --- | --- |
| RNF-V1 | A descoberta de portfólio compara o hash do conteúdo normalizado com o snapshot anterior e não cria versão quando idêntico |
| RNF-V2 | A frequência de descoberta por organização é limitada por política, com sobreposição mínima configurável entre execuções |
| RNF-V3 | Descoberta disparada por simulação não cria snapshot publicado |
| RNF-V4 | Unidades de recebível são particionadas por data prevista de liquidação |
| RNF-V5 | Snapshot não referenciado por oferta, contrato ou efeito tem retenção reduzida, definida em política |

## 3. Latência

Distinção necessária que os PRDs não fazem: **latência da plataforma** e
**latência da operação**. A primeira é responsabilidade de engenharia; a segunda
depende de terceiros e só é gerenciável por transparência.

### 3.1 Latência da plataforma

Medida no percentil 95, excluindo tempo de espera por parceiro externo.

| Operação | Alvo p95 | Limite p99 | Razão |
| --- | --- | --- | --- |
| Autenticação e carga inicial | 1,5 s | 3 s | |
| Listagem de portfólio (paginada) | 800 ms | 2 s | Consulta sobre snapshot já materializado |
| Simulação sem reserva | 2 s | 5 s | Avaliação de elegibilidade sobre lote |
| Criação de solicitação | 1 s | 3 s | Persistência e publicação |
| Comparação de ofertas | 500 ms | 1,5 s | Dados já normalizados na recepção |
| Recepção de oferta (API do financiador) | 300 ms | 1 s | Validação e persistência; afeta o parceiro |
| Recepção de callback de infraestrutura | 200 ms | 500 ms | Aceitar e enfileirar, nunca processar em linha |
| Consulta de linha do tempo de auditoria | 3 s | 10 s | Leitura de volume grande, tolerante |
| Geração de dossiê de evidências | assíncrona | — | Nunca síncrona; retorna identificador de trabalho |

**Regra sobre callback**: a recepção de callback de infraestrutura tem o alvo mais
agressivo de todos e o menor escopo — validar assinatura, persistir o bruto,
enfileirar, responder. Processar em linha significa que uma lentidão interna causa
timeout no parceiro, e timeout no parceiro produz reenvio ou, pior,
`UNKNOWN_OUTCOME` do lado dele.

### 3.2 Latência da operação

Estes alvos dependem de terceiros. A plataforma responde pela transparência do
estado, não pelo prazo.

| Etapa | Alvo | Fator determinante |
| --- | --- | --- |
| Solicitação até primeira oferta | 15 min (p50), 2 h (p95) | Tempo de resposta do financiador |
| Oferta selecionada até contrato assinado | 30 min (p50) | Disponibilidade do signatário |
| Contrato assinado até efeito confirmado | 2 h (p50), 24 h (p95) | Janela de processamento da infraestrutura |
| Efeito confirmado até desembolso | 4 h (p50), 24 h (p95) | Ciclo operacional do financiador |
| Solicitação até crédito | 1 dia útil (p50) | Composição de todos os acima |

**Requisito derivado, ausente dos PRDs**: para cada etapa, a interface exibe o
estado atual, o responsável pela próxima ação e a expectativa de prazo. Quando o
prazo esperado é ultrapassado, a comunicação é ativa, não sob demanda.

## 4. Disponibilidade

### 4.1 Alvos por capacidade

Nem toda parte da plataforma tem a mesma criticidade. Alvo único é desperdício em
um lugar e insuficiente em outro.

| Capacidade | Disponibilidade alvo | Indisponibilidade mensal tolerada | Justificativa |
| --- | --- | --- | --- |
| Recepção de callback de parceiro | 99,9 % | ~43 min | Perder callback gera `UNKNOWN_OUTCOME` e trabalho manual |
| API de recepção de oferta | 99,9 % | ~43 min | Indisponibilidade derruba o marketplace do lado do parceiro |
| Portal do cedente | 99,5 % | ~3,6 h | Uso concentrado em horário comercial |
| Backoffice | 99,0 % | ~7,2 h | Uso interno, tolerante |
| Geração de dossiê | 99,0 % | ~7,2 h | Assíncrona por natureza |
| Escrita no ledger | 99,95 % | ~22 min | Falha aqui impede operação — ver 4.2 |

### 4.2 O ledger é ponto único de falha por desenho

`12-prd-compliance-ledger.md` exige que toda ação relevante seja registrada. Se o
ledger está indisponível, duas opções existem e ambas são ruins:

- **bloquear a operação** — disponibilidade do produto fica limitada à do ledger;
- **prosseguir sem registrar** — viola a premissa de que toda operação tem trilha
  completa, e a falha é silenciosa.

**Proposta**: desacoplar por outbox. A transação de negócio grava estado e outbox
atomicamente no banco do módulo; o ledger consome de forma assíncrona. A
indisponibilidade do ledger atrasa o registro, não impede a operação, e a outbox
garante que nada se perde.

**Consequência a aceitar**: existe janela em que a operação ocorreu e ainda não
está no ledger. A profundidade e a idade da outbox viram métrica de conformidade,
não apenas técnica — e um alarme de outbox acumulando é incidente de compliance.

### 4.3 Degradação

A plataforma depende de terceiros cuja indisponibilidade é esperada
(`00-arquitetura-tecnica.md`, 13: "indisponibilidade de infraestrutura é modo de
operação esperado"). Comportamento exigido:

| Indisponível | Comportamento exigido | Comportamento proibido |
| --- | --- | --- |
| Registradora | Exibir portfólio com `asOf` destacado; bloquear nova contratação; permitir consulta | Exibir posição antiga como atual |
| Financiador | Marcar convite como não entregue; prosseguir com os demais | Falhar a solicitação inteira |
| Provedor de assinatura | Manter contrato em `READY_FOR_SIGNATURE`; comunicar | Tratar como falha de assinatura |
| Provedor de KYB | Manter caso em coleta; não expirar por indisponibilidade | Reprovar por ausência de retorno |
| Ledger | Acumular em outbox; alarmar | Prosseguir descartando registro |

## 5. Durabilidade e recuperação

| Classe | RPO | RTO | Mecanismo |
| --- | --- | --- | --- |
| Banco transacional | 5 min | 1 h | Réplica e recuperação a ponto no tempo |
| Ledger e evidências | 0 | 4 h | Escrita replicada; imutabilidade impede perda por sobrescrita |
| Projeções de leitura | — | 2 h | Reconstruíveis a partir do transacional |
| Payloads brutos de parceiro | 15 min | 24 h | Object storage com versionamento |

**RPO zero para o ledger**: qualquer perda de registro de auditoria é perda de
prova. Se o custo de RPO zero for proibitivo, a alternativa é reduzir a janela ao
mínimo viável e declarar explicitamente o risco residual — não aceitá-lo em
silêncio.

**Teste de recuperação**: exercício de restauração com periodicidade definida,
verificando que o ledger restaurado passa na verificação de integridade da cadeia
de hash (`90-modelo-de-dados.md`, 3.5). Restaurar sem verificar a cadeia não prova
nada.

## 6. Retenção

`12-prd-compliance-ledger.md` invariante: "retenção não é definida genericamente
para todos os eventos". A tabela abaixo é a proposta de categorização; os prazos
dependem de confirmação regulatória e jurídica — ver
`docs/reference/regulacao-brasil.md`.

| Categoria | Exemplo | Prazo proposto | Fundamento a confirmar |
| --- | --- | --- | --- |
| Dossiê contratual | Contrato assinado, evidência de assinatura | Vigência + prazo prescricional | Prescrição civil |
| Evidência de consentimento | Texto, hash, aceite | Vigência + prazo prescricional | LGPD e prova |
| Dossiê cadastral | KYB, beneficiário final | Conforme obrigação de PLD/FT | Norma de PLD/FT |
| Registro de efeito | Comando, confirmação, referência externa | Vigência do efeito + prazo prescricional | |
| Registro de liquidação | Conciliação, ajustes | Prazo fiscal e contábil | |
| Trilha de acesso | Quem acessou qual dossiê | Prazo definido em política | |
| Payload bruto de parceiro | Request e response | Menor prazo viável que preserve capacidade de contestação | |
| Projeção de leitura | Cache de portfólio | Descartável | |
| Snapshot não referenciado | Simulação sem conversão | Curta | |

**Tensão a resolver**: a LGPD confere direito de eliminação; obrigações de
retenção o limitam. A resolução não é técnica — é definir, por categoria, qual
base legal sustenta a retenção após o fim da relação. Sem isso, um pedido de
eliminação não tem resposta defensável.

**Requisito derivado**: o `RetentionPolicy` de cada categoria carrega o fundamento
jurídico como campo obrigatório, não como comentário.

## 7. Segurança (alvos mensuráveis)

Detalhamento em `docs/design/94-seguranca-threat-model.md`. Aqui, apenas o que
tem número.

| Requisito | Alvo |
| --- | --- |
| Revogação de acesso efetiva | Menos de 60 s entre revogação e bloqueio |
| Expiração de sessão inativa | 30 min para papéis de aprovação; 8 h para consulta |
| Rotação de certificado de parceiro | Sobreposição mínima de 30 dias antes do vencimento |
| Alerta de certificado a vencer | 60, 30 e 7 dias antes |
| Rotação de segredo de aplicação | Máximo 90 dias |
| Correção de vulnerabilidade crítica | 7 dias |
| Correção de vulnerabilidade alta | 30 dias |
| Cobertura de MFA em papéis privilegiados | 100 %, sem exceção operacional |

**Sobre "revogação em menos de 60 s"**: `01-prd-identity-organization.md` exige
"bloquear imediatamente acessos revogados". Com token de acesso de vida curta,
"imediato" é o tempo de expiração do token. Um token de 15 minutos significa
revogação efetiva em até 15 minutos — o que contradiz o requisito. Atender aos
60 s exige lista de revogação consultada por requisição ou token de vida muito
curta com renovação frequente. É decisão de arquitetura com custo, e precisa ser
tomada explicitamente.

## 8. Observabilidade (alvos)

| Requisito | Alvo |
| --- | --- |
| Reconstruir uma operação pelo `correlationId` | Menos de 5 min |
| Detecção de comando em `UNKNOWN_OUTCOME` | Alarme em até 5 min após entrada no estado |
| Detecção de outbox acumulando | Alarme quando a idade do item mais antigo exceder 5 min |
| Detecção de divergência de conciliação | Alarme na execução seguinte à detecção |
| Cobertura de rastro nas operações críticas | 100 % das transições de estado externas |

**A métrica mais importante do sistema**: número de comandos em
`UNKNOWN_OUTCOME` por faixa de idade. Ela mede exatamente a quantidade de
operações em que a plataforma não sabe o que aconteceu no mundo real. Crescimento
sustentado dessa métrica é o sinal mais precoce de degradação estrutural, e
precede qualquer métrica de negócio.

## 9. Escalabilidade

| Dimensão | Estratégia |
| --- | --- |
| Requisições de usuário | Escala horizontal sem estado |
| Descoberta de portfólio | Fila com concorrência limitada por parceiro, respeitando o rate limit dele |
| Comandos externos | Concorrência por parceiro, não global — um parceiro lento não pode consumir a capacidade dos demais |
| Ingestão de eventos | Partição por `aggregateId` |
| Escrita no ledger | Partição por `(tenantId, recordType)` |
| Consulta de portfólio | Projeção materializada; leitura não toca a tabela de versões |

**Limite não negociável**: a concorrência contra cada parceiro respeita o limite
contratado (`OperationalPolicy` em `13-prd-partner-management.md`). Escalar a
plataforma sem respeitar isso produz bloqueio pelo parceiro — um modo de falha
pior que a lentidão, porque afeta todos os clientes daquele parceiro
simultaneamente.

## 10. Acessibilidade e compatibilidade

Nenhum PRD trata da interface. Requisitos mínimos propostos:

| Requisito | Alvo |
| --- | --- |
| Acessibilidade | WCAG 2.1 nível AA nas jornadas principais |
| Navegadores | Duas últimas versões maiores dos navegadores majoritários |
| Responsividade | Consulta e acompanhamento em tela pequena; contratação pode exigir tela maior |
| Idioma | Português do Brasil; arquitetura preparada para internacionalização sem compromisso de prazo |

**Nota sobre assinatura em tela pequena**: a persona P2 (aprovador) frequentemente
assina fora do escritório. Exigir tela grande para assinar cria atrito no momento
mais crítico da jornada. Vale avaliar como requisito de produto, não apenas
técnico.

## 11. Custo

Dimensão ausente de todos os PRDs, com impacto direto na viabilidade.

| Vetor de custo | Determinante | Controle |
| --- | --- | --- |
| Consulta a provedor de KYB | Consultas por caso | Reuso de evidência válida (`02-prd`, requisito "reutilização de evidências ainda válidas" já mede isso) |
| Consulta a registradora | Descobertas por organização por dia | RNF-V2 |
| Armazenamento de snapshot | Versões criadas | RNF-V1, RNF-V5 |
| Armazenamento de evidência | Retenção por categoria | Seção 6 |
| Assinatura eletrônica | Envelopes emitidos | Envelope por contrato, não por signatário |

**Requisito derivado**: custo por operação concluída é métrica de produto, não
apenas financeira. Uma operação que consome dez consultas cadastrais e vinte
descobertas para gerar um contrato pode ser deficitária mesmo com receita.

## 12. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| N1 | Volumetria real desconhecida — toda a seção 2 é hipótese | Alto |
| N2 | SLA das infraestruturas não levantado — seção 3.2 é estimativa | Alto |
| N3 | Prazos de retenção pendentes de confirmação jurídica | Alto |
| N4 | Mecanismo de revogação em 60 s não decidido (7) | Médio |
| N5 | Custo unitário dos provedores desconhecido (11) | Médio |
| N6 | Requisitos de interface não validados com usuário (10) | Médio |

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 2 | Todos | Volumetria inexistente |
| 3 | `00-prd-mapa-de-produto.md` | Métricas sem alvo |
| 4 | `12-prd-compliance-ledger.md` | Ledger como dependência crítica não tratada |
| 5 | Nenhum | RPO/RTO ausentes |
| 6 | `12-prd-compliance-ledger.md` | "Retenção por categoria" sem categorias nem prazos |
| 7 | `01-prd-identity-organization.md` | "Bloquear imediatamente" sem definição de imediato |
| 8 | `04-prd-receivables-gateway.md` | Métricas citadas sem alvo |
| 9 | `13-prd-partner-management.md` | Rate limit citado sem estratégia de concorrência |
| 11 | Nenhum | Custo ausente |
