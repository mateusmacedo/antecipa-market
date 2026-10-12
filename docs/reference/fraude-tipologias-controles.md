# Tipologias de Fraude e Controles

> **Camada**: referência
> **Situação**: **parcialmente coberto em outro documento**; a parte empírica
> não foi apurada.

## O que existe e onde

A análise de tipologias, sinais e controles está em
[`../design/15-design-fraude-risco.md`](../design/15-design-fraude-risco.md),
que cobre:

| Seção | Conteúdo |
| --- | --- |
| 2 | Tipologias por origem: ativo, sujeito, desembolso |
| 2.4 | Janelas de exposição que persistem mesmo com registro obrigatório |
| 3 | Catálogo de sinais: cadastrais, comportamentais, relacionais, de desembolso, documentais |
| 4 | Modelo de domínio e decisões de risco |
| 5 | Integração no fluxo operacional |
| 6 | Grafo de relacionamentos e o problema do falso positivo |
| 7 | Governança e limite de papel |

Os controles de segurança correspondentes estão em
[`../design/94-seguranca-threat-model.md`](../design/94-seguranca-threat-model.md),
ameaças T1 a T10.

## O que falta — e por quê

O documento de design foi construído a partir da **mecânica das operações
modeladas nos PRDs**, não de dado empírico. A pesquisa de mercado desta sessão
não pôde ser concluída (orçamento de busca esgotado), então **não há**:

| Ausente | Consequência |
| --- | --- |
| Incidência relativa de cada tipologia | Não se sabe onde concentrar esforço |
| Ordem de grandeza de perda por tipo | Não se sabe o que justifica quanto controle |
| Casos concretos documentados | Falta validação de que o catálogo cobre o que ocorre |
| Benchmark de falso positivo aceitável | Limiares não calibráveis |
| Controles adotados pelo mercado | Possível lacuna no catálogo de controles |

**A consequência prática** está registrada como lacuna X1 em
`design/15-design-fraude-risco.md`: um sistema de detecção sem calibragem produz
falso positivo demais — e a equipe passa a ignorar os sinais — ou de menos, e não
detecta. Nenhum dos dois é aceitável.

**Mitigação proposta para o início da operação** (também em X1): começar com
sinais em modo observação, sem ação automática, e calibrar com os primeiros
meses de dado próprio. Bloqueio automático apenas para sinais de severidade
inequívoca — destino de desembolso divergente, representação revogada, gravame de
terceiro detectado sobre ativo em operação.

## O que apurar

| # | Pergunta | Onde buscar |
| --- | --- | --- |
| 1 | Quais tipologias predominam em antecipação de recebíveis no Brasil, e em que proporção? | Associações setoriais; bureaus de crédito; publicações de prevenção a fraude |
| 2 | Qual a perda média por tipologia? | Idem |
| 3 | Que controles o mercado adota, e quais se mostraram efetivos? | Idem; conversas com financiadores |
| 4 | Qual taxa de falso positivo é operacionalmente sustentável? | Benchmark; experiência de financiadores parceiros |
| 5 | É praticável confirmar a operação com o sacado sem inviabilizar a jornada? | Prática de mercado |
| 6 | É possível verificar a autorização de documento fiscal por terceiro? Como? | Autoridade fiscal; provedores especializados |
| 7 | Há tipologia específica de recebíveis de cartão que o catálogo não cobre? | Credenciadores; registradoras |

**A pergunta 6 é a de maior impacto construtivo**: se a verificação de documento
fiscal for viável e obrigatória, os sinais `RSK-DOC-*` deixam de ser desejáveis e
passam a ser necessários, com integração e custo correspondentes. Está também
como pergunta 8.2 do
[`roteiro-apuracao-regulatoria.md`](./roteiro-apuracao-regulatoria.md).

## Quem depende deste documento

| Documento | O que espera daqui |
| --- | --- |
| `design/15-design-fraude-risco.md` | Calibragem dos limiares (lacuna X1); validação do catálogo (X7) |
| `design/06-design-eligibility.md` | Regras de bloqueio por risco |
| `design/17-design-backoffice.md` | Dimensionamento da fila de casos de risco |
