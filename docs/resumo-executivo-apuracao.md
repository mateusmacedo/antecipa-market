# Resumo Executivo — O que falta apurar

> **Data**: 18 de setembro de 2026
> **Base**: consolidação das 111 pendências registradas nos 21 documentos de
> `docs/design/` e `docs/reference/`, deduplicadas e agrupadas por quem responde.
> **Para quem**: Parte 1 para decisão; Parte 2 para execução.

---

# PARTE 1 — Para decisão

## Situação

A documentação construtiva está pronta: 30 documentos definem arquitetura, modelo
de dados, contratos de API, schemas de eventos, requisitos não funcionais,
segurança e o desenho de cada um dos 18 contextos — incluindo quatro que não
existiam nos PRDs originais (fraude, notificações, backoffice, monetização).

**A construção não pode começar.** Não por falta de definição interna, mas porque
decisões estruturais dependem de informação que só existe fora da empresa.

## O que trava

Três frentes, nenhuma resolvível internamente:

| Frente | Natureza | Quem destrava |
| --- | --- | --- |
| **Regulatória** | 16 perguntas sobre normas, obrigações e responsabilidades | Assessoria jurídica especializada |
| **Infraestrutura** | 17 perguntas sobre protocolos, códigos e capacidades | Registradoras e escrituradores |
| **Comercial** | 3 decisões sobre modelo de receita | Sócios |

## O caminho crítico

**O contato com as registradoras destrava mais do que qualquer outra ação
isolada.** Seis das doze pendências bloqueantes dependem dele, e o material
necessário não é público — manuais de integração e a convenção entre
registradoras exigem acordo comercial prévio.

As três mais estruturais:

1. **Códigos de efeito registráveis.** Sem eles, nenhum comando de constituição
   de cessão ou garantia é enviável. O Effects Orchestrator inteiro fica parado.
2. **Identidade da unidade de recebível.** Sem o esquema, a comparação entre
   posições e a conciliação não podem ser garantidas como corretas.
3. **Existe consulta de estado por operação?** Determina se uma falha de
   comunicação é recuperável automaticamente ou vira trabalho manual permanente.
   Nenhuma documentação comercial responde isso.

## O risco de não decidir

| Pendência | Se não for resolvida |
| --- | --- |
| **Autorização do Banco Central** | Risco existencial. Todo o desenho parte da premissa de que a plataforma não é instituição autorizada — intermediação tecnológica sem crédito, custódia ou trânsito de recursos. Se a premissa estiver errada, o modelo muda, não o software. |
| **Obrigações de PLD/FT** | Se a plataforma for obrigada e não houver fluxo de comunicação de operação suspeita, há descumprimento — não lacuna de produto. |
| **Modelo de monetização** | Nada de faturamento se constrói. Pior: se o financiador paga comissão variável, há conflito de interesse na ordenação de ofertas que exige controle desde o desenho, não depois. |
| **Prazos de retenção** | Sem o fundamento jurídico de cada categoria, não há resposta defensável a um pedido de eliminação de dados. |

## O que depende de vocês

Quatro decisões que não dependem de terceiro algum e destravam trabalho imediato:

1. **Modelo de receita** — quem paga, sobre que base, em que momento. Bloqueia o
   contexto de monetização por completo.
2. **Contratar assessoria regulatória** — 16 perguntas aguardam, várias com
   impacto existencial.
3. **Abrir contato com as registradoras** — caminho crítico, e o de maior tempo
   de maturação.
4. **Escopo do MVP** — o sacado da duplicata entra? Recebíveis futuros entram?
   Ambos ampliam consideravelmente o escopo.

## Sequência recomendada

Não são prazos, são dependências.

```text
AGORA, em paralelo:
  ├─ Abrir contato comercial com registradoras        (maior tempo de maturação)
  ├─ Contratar assessoria regulatória                 (16 perguntas aguardando)
  └─ Decidir modelo de receita                        (interno, sem dependência)

DEPOIS do retorno das registradoras:
  ├─ Fechar o desenho do Gateway e dos adaptadores
  ├─ Fechar o modelo canônico dos ativos
  └─ Revisar a ordem das fases de entrega

DEPOIS do retorno jurídico:
  ├─ Confirmar ou revisar a premissa de não autorização
  ├─ Definir política de retenção com fundamento
  └─ Fechar nível de assinatura e textos contratuais

SÓ DEPOIS:
  └─ Início da construção
```

## O que não está bloqueado

Vale registrar o que pode avançar em paralelo, sem esperar nada:

- as dez decisões técnicas de arquitetura (monolito modular ou serviços,
  multi-tenancy, banco, linguagem) — registradas em
  `design/00-arquitetura-tecnica.md`, seção 12;
- prototipagem de interface e validação das jornadas com usuários reais;
- construção do backoffice e das notificações, que dependem pouco de
  infraestrutura externa;
- levantamento de volumetria esperada junto à área comercial.

---

# PARTE 2 — Para execução

## A — Para a assessoria jurídica

16 perguntas. Detalhamento e formulação verificável em
[`reference/roteiro-apuracao-regulatoria.md`](./reference/roteiro-apuracao-regulatoria.md).

### A.1 Enquadramento (prioridade máxima)

| # | Pergunta | Destrava |
| --- | --- | --- |
| A1 | A atividade exige autorização do Banco Central? Que funcionalidades futuras cruzariam essa linha? | Viabilidade do modelo |
| A2 | A plataforma tem obrigações próprias de PLD/FT, incluindo comunicação de operação suspeita? | Contexto de fraude; KYB |
| A3 | Que responsabilidade a plataforma assume ao gerar o instrumento contratual a partir de modelo do financiador? | Modelo de contratação |

### A.2 Ativos e efeitos

| # | Pergunta | Destrava |
| --- | --- | --- |
| A4 | Natureza jurídica do opt-in; prazo, revogação e efeito da revogação | Onboarding; consentimento |
| A5 | Regime vigente da duplicata escritural e situação de operação | Ordem das fases de entrega |
| A6 | Há obrigação de notificar a cessão ao sacado? De quem é? | Escopo do MVP; notificações |
| A7 | Há obrigação de verificação de lastro da operação mercantil? De quem é? | Escopo do contexto de fraude |

### A.3 Contratos e prova

| # | Pergunta | Destrava |
| --- | --- | --- |
| A8 | Nível de assinatura eletrônica exigido para cessão de crédito e cessão fiduciária | Provedor e fluxo de assinatura |
| A9 | Elementos de evidência necessários para oponibilidade em juízo | Composição do dossiê |
| A10 | Limite de alçada em procuração — como tratar assinatura acima dos poderes conferidos | Risco de contrato questionável |
| A11 | Cláusula de confirmação parcial: ajuste proporcional ou revisão contratual? | Tratamento de efeito parcial |

### A.4 Dados pessoais

| # | Pergunta | Destrava |
| --- | --- | --- |
| A12 | Base legal para consulta de agenda e compartilhamento com financiadores | Textos; comportamento da revogação |
| A13 | Prazos de retenção por categoria e o fundamento que os sustenta após o fim da relação | Política de retenção |
| A14 | Papéis de controlador e operador nas três relações (cedente, financiador, provedor) | Contratos com parceiros |

### A.5 Mercado e tributos

| # | Pergunta | Destrava |
| --- | --- | --- |
| A15 | Há exigência de divulgação de custo efetivo total para tomador pessoa jurídica? | Comparabilidade de ofertas |
| A16 | Tributação da operação e do serviço de intermediação; retenções; documento fiscal | Valor líquido; faturamento |

**Produto esperado**: além das respostas, a redação dos instrumentos de
consentimento e dos modelos contratuais.

---

## B — Para as registradoras e escrituradores

17 perguntas. As quatro primeiras são as que mais afetam o desenho e as que menos
aparecem em material comercial.

### B.1 As quatro decisivas

| # | Pergunta | Destrava |
| --- | --- | --- |
| B1 | **Existe operação de consulta de estado para cada operação de escrita?** | Se não, toda falha de comunicação vira trabalho manual irrecuperável |
| B2 | **A consulta de agenda retorna gravames de forma completa, parcial ou não retorna?** | Se não for completa, não é possível operar com segurança sobre aqueles ativos |
| B3 | **Quais os códigos de efeito registráveis, e como se distingue trava de domicílio de cessão?** | Effects Orchestrator inteiro |
| B4 | **Como se forma a identidade da unidade de recebível?** | Modelo de ativos; conciliação |

### B.2 Dados e prazos

| # | Pergunta | Destrava |
| --- | --- | --- |
| B5 | Prazo de validade da posição informada | Regra de frescor; elegibilidade |
| B6 | Prazo típico e máximo de confirmação de um efeito | Limiar de resultado desconhecido |
| B7 | Campos canônicos da unidade de recebível e da duplicata escritural | Modelo de ingestão |
| B8 | Formato dos arquivos de liquidação | Conciliação |
| B9 | Janela de contestação por arranjo de pagamento | Condição de liberação do efeito |

### B.3 Capacidades

| # | Pergunta | Destrava |
| --- | --- | --- |
| B10 | Há suporte a confirmação parcial por alvo? | Tratamento de efeito parcial |
| B11 | Há suporte a efeito sobre recebíveis futuros? | Escopo de produto |
| B12 | Revogação parcial de opt-in é suportada? | Consentimento |
| B13 | Quem informa o rateio quando há efeitos concorrentes sobre o mesmo ativo? | Conciliação |
| B14 | Existe ponto único de consulta entre registradoras, ou é integração N-para-N? | Dimensionamento do Gateway |

### B.4 Operação e contrato

| # | Pergunta | Destrava |
| --- | --- | --- |
| B15 | Procedimento e prazo de revogação de credencial em emergência | Resposta a incidente |
| B16 | Contato para incidente e prazo de resposta; procedimento de disputa | Operação |
| B17 | Ambiente de homologação; critérios acordáveis; modelo de cobrança | Homologação; custo por operação |

---

## C — Para outros fornecedores

| # | Pergunta | Fornecedor | Destrava |
| --- | --- | --- | --- |
| C1 | **Existe serviço de validação de titularidade de conta bancária?** | A identificar | O controle antifraude de maior valor. Sem ele, o vetor de maior perda esperada — desvio do destino do desembolso — fica sem a verificação central |
| C2 | É possível verificar a autorização de documento fiscal por terceiro? | Autoridade fiscal ou especializado | Sinais documentais de fraude |
| C3 | Cobertura e custo por consulta dos provedores de dados cadastrais | Bureaus | Custo por operação; KYB |
| C4 | Provedor de assinatura eletrônica no nível que o jurídico definir | A escolher | Contratação |
| C5 | Provedor de identidade e de verificação documental | A escolher | Onboarding |

**C1 é a mais urgente deste grupo** e aparece em quatro documentos diferentes
como pendência de impacto alto.

---

## D — Decisões internas

Não dependem de terceiro. Podem ser tomadas agora.

### D.1 Comerciais

| # | Decisão | Consequência |
| --- | --- | --- |
| D1 | **Quem paga**: cedente, financiador, ambos, assinatura ou misto | Se o financiador paga comissão variável, há conflito de interesse na ordenação que exige controle desde o desenho |
| D2 | **Base de cálculo**: valor de face, líquido, custo da operação ou fixo | Modelo de cobrança |
| D3 | **Fato gerador**: recomendação é confirmação do desembolso — alinhado ao valor entregue, elimina incentivo a favorecer operações que não se concretizam | Ciclo de receita |
| D4 | Parâmetros comerciais de elegibilidade: faixas de valor, prazo, concentração | Regras do produto |

### D.2 Produto

| # | Decisão | Consequência |
| --- | --- | --- |
| D5 | O sacado da duplicata entra no escopo? | Se sim: notificação de cessão, portal do sacado, tratamento de recusa de aceite |
| D6 | Recebíveis futuros entram no MVP? | Afeta três contextos; hoje previsto para a fase 4 |
| D7 | Patrocínio pode inverter mérito na ordenação de ofertas? | Recomendação é não — contraria o objetivo de competição. Decisão consciente, de qualquer forma |
| D8 | Volumetria esperada por fase | Todo o dimensionamento; hoje é hipótese |

### D.3 Técnicas

Dez decisões registradas em `design/00-arquitetura-tecnica.md`, seção 12. As de
maior impacto se adiadas:

| # | Decisão |
| --- | --- |
| D9 | Monolito modular ou serviços desde o início |
| D10 | Estratégia de multi-tenancy — retrofit em base existente é caro |
| D11 | Banco primário |
| D12 | Linguagem e plataforma de execução — não há indicação nos PRDs |
| D13 | Definição de "alteração material" em portfólio — determina o custo de armazenamento |

---

## E — O que só a operação produz

Não são pendências de apuração. São dados que não existem antes de operar, e o
desenho precisa acomodar a ausência.

| # | Item | Mitigação prevista |
| --- | --- | --- |
| E1 | Calibragem dos sinais de fraude | Iniciar com sinais em modo observação, sem ação automática; bloqueio automático apenas para severidade inequívoca |
| E2 | Limiares de alçada e prazos do backoffice | Começar conservador e ajustar |
| E3 | Validação das jornadas com usuários reais | Prototipagem antes da construção |
| E4 | Volumetria real | Revisar dimensionamento após as primeiras semanas |

---

# ANEXO — Rastreabilidade

Cada pendência e o que ela bloqueia.

| Pendência | Documento bloqueado | Registro de origem |
| --- | --- | --- |
| A1 Autorização BCB | Todos — viabilidade | `roteiro`, 3.1 |
| A2 PLD/FT | `02-design-kyb-kyc`, `15-design-fraude-risco` | Y8, X2 |
| A3 Instrumento | `08-design-contracting` | T10 |
| A4 Opt-in | `03-design-consent-mandates` | K8 |
| A5 Duplicata escritural | `00-prd` estratégia de entrega | `roteiro`, 2.1 |
| A6 Notificar sacado | `16-design-notificacoes`, `95-personas` | C5, A10 |
| A7 Lastro | `15-design-fraude-risco` | X3 |
| A8 Assinatura | `08-design-contracting` | T9 |
| A9 Prova | `12-design-compliance-ledger` | `roteiro`, 4.2 |
| A10 Alçada | `90-modelo-de-dados`, `01-design-identity` | L3, I10 |
| A11 Efeito parcial | `09-design-effects`, `10-design-funding` | L4, F9, D10 |
| A12 Base legal | `03-design-consent-mandates` | `roteiro`, 5.1 |
| A13 Retenção | `93-requisitos`, `12-design-ledger` | N3, L8, S6 |
| A14 Papéis LGPD | `91-contratos-api` | A2 |
| A15 CET | `07-design-marketplace`, `90-modelo-de-dados` | M11, L6 |
| A16 Tributos | `18-design-billing` | G4 |
| B1 Consulta de estado | `04-design-gateway`, `09-design-effects` | G9, F11 |
| B2 Cobertura de gravame | `05-design-portfolio`, `06-design-eligibility` | P10, E10 |
| B3 Códigos de efeito | `09-design-effects` | F8 |
| B4 Identidade da UR | `05-design-portfolio`, `90-modelo-de-dados` | P8, L1 |
| B5 Validade da posição | `05-design-portfolio`, `93-requisitos` | P10, N2 |
| B6 Prazo de confirmação | `09-design-effects` | F11 |
| B7 Campos canônicos | `90-modelo-de-dados` | L1, L2 |
| B8 Arquivos de liquidação | `11-design-settlement` | R10 |
| B9 Janela de contestação | `11-design-settlement` | R9 |
| B10 Confirmação parcial | `09-design-effects` | F10 |
| B11 Recebíveis futuros | `09-design-effects`, `08-design-contracting` | F10, T13 |
| B12 Revogação parcial | `03-design-consent` | K12 |
| B13 Rateio | `11-design-settlement` | R13 |
| B14 Interoperabilidade | `04-design-gateway` | G8 |
| B15 Revogação de credencial | `94-seguranca` | S4 |
| B16 Incidente e disputa | `94-seguranca`, `09-design-effects` | S5, F12 |
| B17 Homologação e custo | `13-design-partner`, `18-design-billing` | N8, G5 |
| C1 Titularidade de conta | `10-design-funding`, `15`, `94`, `02` | D7, X5, S1, Y13 |
| C2 Documento fiscal | `15-design-fraude-risco` | X4 |
| C3 Bureaus | `02-design-kyb-kyc` | Y11 |
| C4 Assinatura | `08-design-contracting` | T12 |
| C5 Identidade | `01-design-identity` | I8, I9 |
| D1–D3 Monetização | `18-design-billing`, `13-design-partner` | G1–G3, N9 |
| D4 Parâmetros | `06-design-eligibility` | E8 |
| D5 Sacado | `95-personas`, `16-design-notificacoes` | A10, C5 |
| D6 Recebíveis futuros | `08`, `09` | T13, F10 |
| D7 Patrocínio | `07-design-marketplace` | M10 |
| D8 Volumetria | `93-requisitos` | N1 |
| D9–D13 Arquitetura | `00-arquitetura-tecnica` | Seção 12 |
| E1 Calibragem | `15-design-fraude-risco` | X1 |
| E2 Alçadas | `17-design-backoffice`, `11-design-settlement` | B2, R11 |
| E3 Jornadas | `93-requisitos`, `07-design-marketplace` | N6, M9 |

---

## Nota sobre este documento

A apuração regulatória e o mapeamento de provedores **não puderam ser feitos**
durante a produção da documentação: o orçamento de buscas da sessão esgotou, os
portais de legislação recusaram conexão direta, as páginas do Banco Central são
renderizadas por JavaScript sem conteúdo legível, e a documentação técnica das
registradoras não é pública.

Nenhum número de norma, prazo ou código foi preenchido por inferência. Num
documento que fundamenta decisão de arquitetura e decisão jurídica, um valor
plausível e errado é pior que uma lacuna declarada — ele produz confiança
indevida e sobrevive à revisão.

O que existe, portanto, são as perguntas certas, formuladas de modo verificável e
mapeadas ao que cada resposta destrava. É esse o produto desta fase.
