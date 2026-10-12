# Design — Compliance Ledger

> **PRD de origem**: `docs/prd/12-prd-compliance-ledger.md`
> **Camada**: construtiva
> **Status**: proposta
> **Complementa**: `90-modelo-de-dados.md` 3.5 (`AuditRecord`, encadeamento),
> `93-requisitos-nao-funcionais.md` 4.2 e 6 (disponibilidade e retenção),
> `94-seguranca-threat-model.md` T6 (adulteração). Não repetidos aqui.

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| L1 | "WORM ou controle equivalente" sem definição do equivalente | Requisitos não funcionais |
| L2 | Detecção de lacuna na trilha sem mecanismo | Req. 9 |
| L3 | `EvidenceBundle` sem formato nem fluxo de autorização | Req. 5 |
| L4 | Retenção por categoria sem categorias | Req. 7 |
| L5 | "Reconstrução em ordem causal e temporal" sem algoritmo | Req. 4 |
| L6 | Verificação periódica de integridade sem procedimento | Requisitos não funcionais |
| L7 | Relação com o fluxo operacional não definida (bloqueia ou não?) | — |

## 2. O ledger não bloqueia a operação

Lacuna estrutural resolvida em `93-requisitos-nao-funcionais.md`, 4.2 e
reafirmada aqui pela importância: a escrita no ledger é **assíncrona via outbox**.

| Consequência | Tratamento |
| --- | --- |
| Existe janela entre o fato e seu registro | Aceita e medida |
| Indisponibilidade do ledger não impede operação | Requisito |
| Perda de registro é inaceitável | Outbox transacional garante durabilidade |
| Profundidade da outbox é métrica de conformidade | Alarme em 5 min de atraso |

**Alternativa rejeitada**: escrita síncrona. Tornaria o ledger ponto único de falha
do produto inteiro, para ganhar uma garantia que a outbox já oferece — o registro
não se perde, apenas chega depois.

## 3. Imutabilidade

O PRD pede "WORM ou controle equivalente" sem definir o equivalente. Três camadas
independentes, porque nenhuma isolada é suficiente:

| Camada | Protege contra | Limite |
| --- | --- | --- |
| Permissão de banco sem `UPDATE`/`DELETE` | Erro de aplicação | Não protege contra quem tem acesso administrativo |
| Encadeamento por hash | Alteração posterior não detectada | Detecta; não impede. Não protege contra recálculo completo |
| Retenção imutável no armazenamento de evidências | Alteração por administrador | Depende da capacidade do armazenamento |
| Notarização externa **[ABERTA]** | Recálculo completo da cadeia | Dependência externa |

**Declaração honesta do limite**: sem notarização externa, um administrador com
acesso total pode recalcular a cadeia inteira de uma partição. O encadeamento
protege contra alteração oportunista e detecta corrupção; não constitui prova
contra quem controla a infraestrutura.

Isso precisa estar documentado, porque apresentar o encadeamento como prova
absoluta cria confiança indevida — pior que não ter o mecanismo.

## 4. Detecção de lacuna

Req. 9 exige detectar sequência ausente, duplicada ou inválida, sem mecanismo.

| Verificação | Método | Periodicidade |
| --- | --- | --- |
| Sequência ausente | `sequence` monotônica sem intervalo por partição | Contínua, na ingestão |
| Duplicidade | `eventId` único | Contínua |
| Cadeia inválida | `previousRecordHash` confere com o anterior | Periódica |
| Evento produzido e não registrado | Reconciliação entre outbox dos módulos e ledger | Periódica |
| Evento registrado sem origem | `causationId` sem comando correspondente | Periódica |

**A quarta verificação é a mais importante e a mais esquecida.** Detectar
descontinuidade na sequência do ledger não detecta um evento que nunca chegou.
A reconciliação entre o que os módulos produziram e o que o ledger recebeu é a
única forma de saber se a trilha está completa.

**Lacuna detectada gera incidente de conformidade**, não apenas alarme técnico.

## 5. Reconstrução

Req. 4 exige reconstruir a jornada em ordem causal e temporal, sem algoritmo.

```text
reconstruir(correlationId):
  1. carregar todos os AuditRecord com o correlationId
  2. montar grafo por causationId (causa -> efeito)
  3. ordenar topologicamente
  4. desempatar por occurredAt; se igual, por recordedAt
  5. anexar EvidenceReference resolvidas conforme a autorização do solicitante
```

| Situação | Tratamento |
| --- | --- |
| Registro sem `causationId` | Raiz da cadeia |
| Ciclo no grafo | Anomalia; sinalizar, não ordenar arbitrariamente |
| `occurredAt` posterior a `recordedAt` | Desvio de relógio do produtor; sinalizar |
| Registro de outra correlação referenciado | Incluir como referência, não como nó |

**Ordem causal antes de temporal**: relógios de produtores diferentes não são
comparáveis (`00-arquitetura-tecnica.md`, 8). A causalidade é o que o
`causationId` estabelece de forma confiável; o tempo apenas desempata.

## 6. Dossiê de evidências

Req. 5 exige gerar dossiês com controle de acesso e finalidade, sem formato nem
fluxo.

### 6.1 Fluxo

```text
Solicitação (finalidade declarada, escopo, solicitante)
  -> Autorização (segundo aprovador, conforme 95-personas-jornadas.md)
  -> Geração assíncrona
  -> Coleta: registros + evidências + hashes
  -> Manifesto com inventário e hashes
  -> Assinatura do pacote
  -> Disponibilização com prazo e registro de acesso
  -> Expiração
```

### 6.2 Conteúdo

| Item | Obrigatório |
| --- | --- |
| Manifesto com inventário completo | sim |
| Hash de cada item e do pacote | sim |
| Linha do tempo reconstruída | sim |
| Registros de auditoria do escopo | sim |
| Documentos referenciados | conforme autorização |
| Versões de política, texto e método aplicáveis | sim |
| Identificação do solicitante e da finalidade | sim |
| Instante de geração e validade | sim |
| Assinatura do pacote | sim |

**Sem as versões de política, o dossiê é incompleto**: provar que uma oferta foi
apresentada em determinada ordem exige a política de ordenação vigente naquele
momento. Sem ela, a apresentação é registrada mas não verificável.

**Critério de completude**: o dossiê permite reconstruir sem acesso a nenhum
sistema. Referência cujo conteúdo não é recuperável não constitui dossiê completo
(invariante do PRD).

### 6.3 Acesso

| Requisito | Definição |
| --- | --- |
| Autorização | Solicitação com finalidade; aprovação por segundo responsável |
| Minimização | Escopo limitado à finalidade declarada |
| Registro | A própria emissão e cada acesso geram registro (req. 6) |
| Validade | Pacote expira; expiração não apaga o registro da emissão |
| Rastreabilidade | Marca d'água por destinatário, quando o formato permitir |

## 7. Categorias de retenção

Req. 7 exige aplicar retenção por categoria, sem categorias. Definidas em
`93-requisitos-nao-funcionais.md`, 6; os prazos dependem de apuração jurídica.

**Requisito estrutural**: `RetentionPolicy` carrega o fundamento jurídico como
campo obrigatório. Política sem fundamento não é aplicável — e sem ela não há
resposta defensável a um pedido de eliminação.

### 7.1 Legal hold

| Requisito | Definição |
| --- | --- |
| Efeito | Suspende descarte, independentemente da retenção |
| Escopo | Por sujeito, por período, ou por correlação |
| Autorização | Jurídico; não é operação de rotina |
| Visibilidade | Registro sob hold é identificável na consulta |
| Liberação | Registrada, com autor e fundamento |
| Precedência | Hold prevalece sobre qualquer política de descarte |

**Aplicação automática**: abertura de disputa ou caso crítico deveria aplicar hold
preventivo sobre os registros da operação. Descartar evidência durante uma disputa
por decurso de prazo é falha evitável, e depender de alguém lembrar de aplicar o
hold é frágil.

## 8. Verificação de integridade

| Verificação | Periodicidade | Ação em falha |
| --- | --- | --- |
| Cadeia de hash por partição | Diária, incremental | Incidente crítico |
| Cadeia completa | Periódica | Incidente crítico |
| Existência das evidências referenciadas | Periódica, por amostragem | Incidente |
| Hash das evidências | Periódica, por amostragem | Incidente crítico |
| Reconciliação outbox-ledger | Diária | Incidente de conformidade |
| Restauração testada | Periódica | Incidente |

**Restauração sem verificação de cadeia não prova nada**: restaurar e confirmar que
o banco sobe é insuficiente. O teste inclui verificar a integridade da cadeia no
ambiente restaurado.

## 9. O que o ledger não é

| Não é | Razão |
| --- | --- |
| Observabilidade | Propósitos e requisitos distintos (`00-arquitetura-tecnica.md`, 10) |
| Banco de leitura dos contextos | Cada contexto tem seu próprio estado (invariante do PRD) |
| Sistema de relatórios | Volume e estrutura inadequados; usar projeção derivada |
| Fonte de verdade operacional | É registro do que ocorreu, não estado atual |

**Tentação a evitar**: usar o ledger para responder "qual o estado atual do
contrato X". Ele responde "o que aconteceu com o contrato X". A diferença é
importante — derivar estado atual de um log append-only é caro e produz acoplamento
que a invariante do PRD proíbe.

## 10. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| L8 | Prazos de retenção por categoria | **Alto** |
| L9 | Notarização externa (3) | Médio |
| L10 | Tecnologia de armazenamento imutável | Médio |
| L11 | Formato do pacote de dossiê | Baixo |
| L12 | Fluxo de aprovação de legal hold | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | — | Relação com o fluxo operacional |
| 3 | Requisitos não funcionais | "Controle equivalente" sem definição |
| 4 | 9 | Detecção de lacuna sem mecanismo |
| 5 | 4 | Reconstrução sem algoritmo |
| 6 | 5, 6 | Dossiê sem formato nem fluxo |
| 7 | 7 | Categorias e fundamento |
| 8 | Requisitos não funcionais | Verificação sem procedimento |
