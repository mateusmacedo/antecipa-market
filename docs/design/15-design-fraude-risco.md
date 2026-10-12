# Design — Fraude e Risco Operacional

> **Contexto novo**: não existe PRD correspondente.
> **Camada**: construtiva
> **Status**: proposta
>
> **Origem do conteúdo**: as tipologias e controles descritos derivam da mecânica
> das operações modeladas nos PRDs e do desenho de segurança em
> `94-seguranca-threat-model.md`. **Não há, nesta versão, estatística de
> mercado, caso concreto ou benchmark de incidência** — a pesquisa de mercado
> desta sessão não pôde ser concluída (orçamento de busca esgotado). Onde a
> calibragem depende de dado que não tenho, isso está declarado em vez de
> estimado.

## 1. Por que este contexto precisa existir

Nenhum dos 15 PRDs trata de fraude. O mais próximo é `RiskSignal` em
`02-prd-kyb-kyc.md`, definido como "sinal informativo, nunca decisão automática
da plataforma" — correto como limite de papel, insuficiente como capacidade.

A ausência tem consequência direta: a plataforma constitui efeitos jurídicos
irreversíveis sobre ativos de terceiros e coordena desembolsos para contas
informadas por usuários. São exatamente os dois pontos onde a fraude ataca.

**Distinção que este contexto preserva**:

| A plataforma | O financiador |
| --- | --- |
| Produz sinal de risco | Decide crédito |
| Bloqueia a própria operação por risco de fraude | Decide aceitar o cedente |
| Impede uso indevido da plataforma | Assume o risco da operação |

**Bloquear por suspeita de fraude não é decisão de crédito.** É recusa de
prestar o próprio serviço — direito de qualquer prestador, e dever quando há
indício de ilícito. O limite: a plataforma não comunica ao financiador que o
cedente "é fraudador"; comunica que não prosseguirá com aquela operação.

## 2. Tipologias

Classificadas por onde a fraude nasce e onde é detectável.

### 2.1 Fraude no ativo

| Tipologia | Mecânica | Onde é detectável | Mitigada pelo registro? |
| --- | --- | --- | --- |
| **Duplicata sem lastro** | Emissão de duplicata sem operação mercantil correspondente | Verificação do documento fiscal; confirmação com o sacado | **Não** — o registro impede dupla negociação, não cria lastro |
| **Duplicata já negociada** | Antecipação do mesmo título a mais de um financiador | Consulta de gravames antes do registro | **Sim**, quando o registro é obrigatório e consultado |
| **Conluio cedente-sacado** | Sacado confirma operação inexistente | Análise de relacionamento entre as partes; histórico | Não |
| **Nota fiscal adulterada** | Documento alterado ou inexistente na origem | Verificação de autorização junto à autoridade fiscal | Não |
| **Transação de cartão simulada** | Transações criadas para gerar agenda antecipável | Padrão de transações; relação com o portador | Não |
| **Chargeback intencional** | Contestação após a antecipação | Padrão de contestação; concentração | Não |

**A conclusão estrutural**: o registro obrigatório resolve bem um problema — a
dupla negociação — e **não resolve** a fraude de lastro. Um ativo sem lastro
registrado corretamente continua sem lastro. Essa é a razão pela qual a
verificação de lastro merece a pergunta 8.2 do
`docs/reference/roteiro-apuracao-regulatoria.md`.

### 2.2 Fraude no sujeito

| Tipologia | Mecânica | Onde é detectável |
| --- | --- | --- |
| **Empresa de fachada** | Pessoa jurídica constituída para a fraude | Idade, atividade, capital, estrutura societária, endereço |
| **Interposta pessoa** | Sócio formal não é o controlador real | Beneficiário final; perfil incompatível |
| **Identidade apropriada** | Uso de dados de terceiro para constituir ou operar | Verificação documental e de vivacidade |
| **Representante sem poderes** | Assinatura por quem não tem ou não tem mais poderes | Vigência e alçada da representação |

### 2.3 Fraude no desembolso

| Tipologia | Mecânica | Onde é detectável | Severidade |
| --- | --- | --- | --- |
| **Desvio de destino** | Alteração da conta de recebimento antes do desembolso | Validação de titularidade; quarentena; notificação independente | **Máxima** |
| **Sequestro de conta** | Acesso indevido à conta do usuário do cedente | Dispositivo, geografia, comportamento | Máxima |
| **Engenharia social interna** | Indução de operador da plataforma a alterar dado | Alçada; justificativa; revisão | Alta |

**O desvio de destino é a tipologia de maior perda esperada** porque o dano é
imediato, o recurso é irrecuperável e o vetor é simples. Os controles estão em
`94-seguranca-threat-model.md`, T2 — e o decisivo é C2.5: alteração de conta
entre a assinatura e o desembolso **bloqueia a operação**, não apenas exige
aprovação.

### 2.4 Janelas de exposição remanescentes

Mesmo com registro obrigatório e todos os controles, persistem janelas
estruturais:

| Janela | Exposição | Mitigação possível |
| --- | --- | --- |
| Entre a consulta de posição e o registro do efeito | O ativo pode ser comprometido nesse intervalo | Revalidação imediatamente antes do comando; minimizar o intervalo |
| Entre o registro do efeito e o desembolso | Alteração de dados do cedente | Reavaliação de pré-condições na emissão da instrução |
| Entre a liquidação e o fim da janela de contestação | Estorno posterior | Não liberar o efeito antes do decurso |
| Ativos fora do registro obrigatório | Sem controle de dupla negociação | Restringir tipos de ativo aceitos |

**A primeira janela não é eliminável**, apenas encurtável. É a razão pela qual
`09-design-effects-orchestrator.md`, seção 3 coloca a revalidação como penúltimo
passo, imediatamente antes do envio.

## 3. Sinais

`RiskSignal` existe em `02-prd-kyb-kyc.md` sem catálogo. Proposta — com a
ressalva de que **os limiares não estão calibrados**, por falta de dado
histórico ou de mercado.

### 3.1 Cadastrais

| Código | Sinal |
| --- | --- |
| `RSK-CAD-001` | Organização constituída recentemente |
| `RSK-CAD-002` | Atividade econômica incompatível com o tipo de recebível |
| `RSK-CAD-003` | Capital social desproporcional ao volume solicitado |
| `RSK-CAD-004` | Endereço compartilhado com número atípico de empresas |
| `RSK-CAD-005` | Sócio participa de número atípico de empresas |
| `RSK-CAD-006` | Alteração societária recente |
| `RSK-CAD-007` | Divergência entre fontes cadastrais |
| `RSK-CAD-008` | Situação cadastral irregular em algum momento recente |

### 3.2 Comportamentais

| Código | Sinal |
| --- | --- |
| `RSK-BHV-001` | Primeira operação com valor atípico para o perfil |
| `RSK-BHV-002` | Aceleração súbita na frequência de solicitações |
| `RSK-BHV-003` | Concentração extrema em um sacado |
| `RSK-BHV-004` | Valores com padrão artificial (repetidos, redondos) |
| `RSK-BHV-005` | Agenda de recebíveis com crescimento incompatível com o histórico |
| `RSK-BHV-006` | Solicitação imediatamente após alteração cadastral relevante |
| `RSK-BHV-007` | Abandono recorrente de operações na etapa de contratação |

### 3.3 Relacionais

| Código | Sinal |
| --- | --- |
| `RSK-REL-001` | Cedente e sacado com sócio comum |
| `RSK-REL-002` | Cedente e sacado com endereço comum |
| `RSK-REL-003` | Múltiplas organizações operando do mesmo dispositivo ou origem de rede |
| `RSK-REL-004` | Mesmo representante em organizações sem vínculo declarado |
| `RSK-REL-005` | Organização vinculada a outra com histórico de fraude confirmada |

**Os sinais relacionais exigem estrutura de grafo**, não consulta tabular. É a
diferença entre detectar uma empresa suspeita e detectar um esquema. Decisão de
arquitetura com custo próprio — ver seção 6.

### 3.4 Do desembolso

| Código | Sinal |
| --- | --- |
| `RSK-BNK-001` | Conta de destino alterada recentemente |
| `RSK-BNK-002` | Titularidade da conta não confirmada |
| `RSK-BNK-003` | Conta de terceiro autorizada contratualmente |
| `RSK-BNK-004` | Alteração de conta após a seleção de oferta |
| `RSK-BNK-005` | Destino confirmado diverge do validado |

**`RSK-BNK-005` não é sinal, é incidente.** Aparece aqui por completude, mas seu
tratamento é o de `FUNDING_MISMATCH` crítico, não o de sinal a ponderar.

### 3.5 Documentais

| Código | Sinal |
| --- | --- |
| `RSK-DOC-001` | Documento fiscal não confirmado na origem |
| `RSK-DOC-002` | Divergência entre documento fiscal e título |
| `RSK-DOC-003` | Documento com indício de adulteração |
| `RSK-DOC-004` | Sequência de numeração atípica |

**Dependem da pergunta 8.2** do roteiro de apuração: se a verificação de lastro é
obrigação da plataforma, esses sinais passam de desejáveis a necessários, com
integração e custo correspondentes.

## 4. Modelo de domínio

| Agregado | Papel |
| --- | --- |
| `RiskAssessment` | Avaliação de uma organização ou operação em um instante |
| `RiskSignal` | Sinal individual: código, severidade, evidência, instante |
| `RiskPolicy` | Política versionada: sinais, pesos, limiares, ações |
| `RiskDecision` | Resultado: prosseguir, prosseguir com sinalização, revisar, bloquear |
| `RiskBlock` | Bloqueio vigente, com escopo, motivo e prazo |
| `RiskCase` | Investigação, com responsável e desfecho |
| `EntityRelation` | Aresta do grafo: tipo, entidades, origem, confiança |

### 4.1 Decisões possíveis

| Decisão | Efeito | Quem pode reverter |
| --- | --- | --- |
| `PROCEED` | Nada | — |
| `PROCEED_WITH_SIGNAL` | Sinais acompanham o pedido ao financiador | — |
| `REVIEW` | Operação retida; caso para análise de risco | Analista de risco |
| `BLOCK_OPERATION` | Operação impedida; demais permitidas | Analista de risco |
| `BLOCK_ORGANIZATION` | Nenhuma nova operação | Supervisão de risco |

**`PROCEED_WITH_SIGNAL` é o modo padrão para sinal não conclusivo.** Preserva o
limite de papel: a plataforma informa, o financiador decide. Bloquear por sinal
fraco seria decidir crédito por outro nome.

**`REVIEW` retém a operação e tem custo de experiência.** Precisa de prazo curto
e comunicação ao cedente — uma operação retida sem explicação é indistinguível de
uma falha.

## 5. Integração no fluxo

```text
Onboarding concluído       -> avaliação cadastral e relacional
Solicitação criada          -> avaliação da operação
                               `- BLOCK  -> não distribui
                               `- REVIEW -> retém
                               `- demais -> prossegue
Antes da assinatura         -> reavaliação (sinais novos desde a solicitação)
Antes do desembolso         -> reavaliação de destino
Após liquidação             -> avaliação retrospectiva (padrão de chargeback)
Contínuo                    -> monitoramento de alteração cadastral e relacional
```

**Reavaliação antes da assinatura**: entre a solicitação e a assinatura podem
surgir sinais — alteração de conta, mudança societária, nova organização
relacionada. Avaliar apenas na entrada deixa a janela mais crítica descoberta.

**Avaliação retrospectiva**: fraude confirmada depois alimenta o grafo e o
histórico. Sem o ciclo de retorno, a detecção não melhora.

## 6. Grafo de relacionamentos

Os sinais de 3.3 exigem consultar relações entre entidades — sócio comum,
endereço comum, dispositivo comum.

| Abordagem | Custo | Adequação |
| --- | --- | --- |
| Consultas relacionais sob demanda | Baixo inicial; degrada com profundidade | Suficiente para relações diretas |
| Estrutura de grafo dedicada | Alto | Necessária para detecção de esquema |
| Tabela materializada de relações | Médio | Meio-termo viável |

**Recomendação**: começar com `EntityRelation` materializada e consulta limitada
a dois graus. Detecção de esquema com profundidade maior é capacidade a
acrescentar quando houver volume que a justifique.

**Cuidado com falso positivo**: escritórios de contabilidade concentram endereço
e representante de dezenas de empresas legítimas. Um sinal de "endereço comum"
sem tratamento de exceção gera ruído que faz a equipe ignorar o sinal — o pior
resultado possível. Toda regra relacional precisa de lista de exceção mantida.

## 7. Governança

| Requisito | Definição |
| --- | --- |
| Limite de papel | A plataforma não emite juízo sobre a idoneidade do cedente ao financiador; informa sinais objetivos |
| Bloqueio motivado | Todo bloqueio tem código, evidência e responsável |
| Contestação | O cedente pode contestar; o processo tem prazo e resposta |
| Revisão de política | Periódica, com medição de falso positivo e falso negativo |
| Segregação | Quem investiga não aprova o próprio bloqueio acima de determinado impacto |
| Registro | Toda avaliação, sinal e decisão vai ao ledger |
| Comunicação a autoridade | **Depende da pergunta 3.2** do roteiro de apuração |

**Sobre comunicar ao cedente o motivo do bloqueio**: há tensão real entre
transparência e não revelar os critérios de detecção a quem tenta burlá-los. A
posição proposta: comunicar que houve bloqueio e oferecer canal de contestação,
sem detalhar os sinais. Decisão que merece validação jurídica.

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| X1 | Limiares não calibrados — não há dado histórico nem benchmark | **Alto** |
| X2 | Obrigação de comunicação a autoridade (7) | Alto — ver roteiro, 3.2 |
| X3 | Obrigação de verificação de lastro | Alto — ver roteiro, 8.2 |
| X4 | Viabilidade de verificar documento fiscal por terceiro (3.5) | Alto |
| X5 | Serviço de validação de titularidade de conta | **Alto** — bloqueia o controle de maior valor |
| X6 | Confirmação com o sacado é praticável sem inviabilizar a operação? | Médio |
| X7 | Tipologias de mercado não levantadas — o catálogo pode ter lacunas | Médio |

**X1 é a limitação estrutural desta versão**: um sistema de detecção sem
calibragem produz falso positivo demais (e a equipe passa a ignorar) ou de menos
(e não detecta). A calibragem exige ou dado histórico próprio — que não existe
antes de operar — ou benchmark de mercado, que a pesquisa não pôde levantar.

**Mitigação para o início da operação**: começar com sinais em modo observação,
sem ação automática, e calibrar com os primeiros meses. Bloqueio automático
apenas para os sinais de severidade inequívoca — destino divergente, representação
revogada, gravame de terceiro detectado.

---

## Rastreabilidade

| Seção | Origem | Lacuna fechada |
| --- | --- | --- |
| 1 | `02-prd-kyb-kyc.md` | `RiskSignal` sem contexto que o produza |
| 2.4 | `09-prd-effects-orchestrator.md` | Janelas de exposição não identificadas |
| 3 | `02-prd-kyb-kyc.md` | Sinais sem catálogo |
| 4.1 | — | Decisões de risco sem modelo |
| 5 | Todos | Avaliação de risco ausente do fluxo |
| 7 | `02-prd-kyb-kyc.md` | Limite de papel sem governança |
