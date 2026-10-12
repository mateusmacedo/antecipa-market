# Roteiro de Apuração Regulatória

> **Camada**: referência
> **Natureza**: este documento **não contém respostas**. Contém as perguntas que
> precisam ser respondidas por fonte primária ou por assessoria jurídica, com o
> impacto construtivo de cada resposta.
> **Por que existe**: a pesquisa automatizada desta sessão não conseguiu
> confirmar fonte primária (orçamento de busca esgotado; `planalto.gov.br`
> recusando conexão; páginas do Banco Central renderizadas por JavaScript, sem
> conteúdo no HTML). Registrar as perguntas com precisão vale mais do que
> preencher o documento com números não verificados — que, neste domínio,
> produziriam decisão de arquitetura errada com aparência de fundamentada.

## Como usar

Cada pergunta traz:

- **Pergunta** — formulada de modo que a resposta seja verificável.
- **Por que importa** — a decisão de construção que depende dela.
- **Bloqueia** — o que não pode ser construído antes da resposta.
- **Onde buscar** — fonte provável.

A ordem reflete o impacto construtivo, não a hierarquia normativa.

---

## Bloco 1 — Registro de recebíveis de arranjo de pagamento

### 1.1 Tipos de efeito registráveis e seus códigos

**Pergunta**: quais tipos de ônus e gravame podem ser registrados sobre unidades
de recebível, como se distinguem juridicamente, e quais os códigos ou
identificadores usados pelas registradoras para cada um?

**Por que importa**: `09-design-effects-orchestrator.md`, seção 4 define a tabela
`EffectTypeResolution` com o campo `providerEffectCode` vazio. Sem o código
correto, nenhum comando de constituição de efeito é enviável.

**Bloqueia**: Effects Orchestrator inteiro. É a lacuna bloqueante de maior
impacto do projeto.

**Atenção particular**: a distinção entre **trava de domicílio** (direcionamento
de recursos) e **cessão** (transferência de titularidade) tem consequências
jurídicas distintas e é fonte comum de confusão. Ambas aparecem na lista de
tipos de efeito de `09-prd-effects-orchestrator.md` sem distinção operacional.

**Onde buscar**: normas do Conselho Monetário Nacional e do Banco Central sobre
registro de recebíveis; convenção celebrada entre as registradoras; manuais de
integração das registradoras.

### 1.2 Ordem de prioridade entre efeitos concorrentes

**Pergunta**: quando há mais de um efeito sobre a mesma unidade de recebível,
quem define a prioridade, qual o critério, e como o resultado é comunicado aos
participantes?

**Por que importa**: `11-design-settlement-reconciliation.md`, seção 4 assume que
a plataforma **não** calcula rateio, apenas registra o informado pela fonte
competente. Se a fonte não informa o rateio, essa premissa cai e a conciliação
com múltiplos efeitos vira trabalho manual permanente.

**Bloqueia**: conciliação de operações com efeitos concorrentes; regra
`AST-ENC-001` de elegibilidade.

### 1.3 Composição e identidade da unidade de recebível

**Pergunta**: quais campos compõem a unidade de recebível no padrão regulatório,
e como se forma sua identidade — identificador atômico atribuído pela
registradora, ou composição de atributos?

**Por que importa**: `05-design-portfolio.md`, seção 5 registra isso como lacuna
bloqueante. Sem o esquema de identidade, a comparação entre versões de snapshot e
a correspondência na conciliação não podem ser garantidas como corretas.

**Bloqueia**: modelo canônico de `CardReceivableUnit`; comparação de snapshots;
conciliação por identificador oficial.

### 1.4 Mecânica do opt-in

**Pergunta**: qual a natureza jurídica da autorização para um terceiro consultar
a agenda de recebíveis; quem a registra; qual o prazo de vigência; como se
revoga; e em quanto tempo a revogação produz efeito?

**Por que importa**: `03-design-consent-mandates.md`, seções 4 e 5 definem escopo
efetivo como interseção entre consentimento interno e autorização externa, e
estabelecem prazos de propagação de revogação. Ambos são propostas sem base
normativa confirmada.

**Bloqueia**: fluxo de onboarding; prazos de revogação; modelo de escopo.

### 1.5 Interoperabilidade entre registradoras

**Pergunta**: existe ponto único de consulta ou é necessária integração com cada
registradora? Se há padronização, ela cobre quais operações?

**Por que importa**: determina se o Gateway precisa de um adaptador ou de N. Muda
o esforço de integração em ordem de grandeza.

**Bloqueia**: dimensionamento do Gateway; estratégia de entrega.

### 1.6 Regras de liquidação e direcionamento

**Pergunta**: quando há efeito registrado, quem paga a quem, em que ordem, e
quem informa o resultado à plataforma?

**Por que importa**: `11-design-settlement-reconciliation.md`, seção 2 distingue
pago, baixado, liquidado e repassado. A distinção só é implementável se cada
estado tiver fonte identificável.

**Bloqueia**: expectativas de liquidação; detecção de beneficiário incorreto.

### 1.7 Janela de contestação e chargeback

**Pergunta**: qual o prazo máximo, por arranjo de pagamento, em que uma
transação já liquidada pode ser estornada?

**Por que importa**: `11-design-settlement-reconciliation.md`, seção 7 condiciona
a liberação do efeito ao decurso dessa janela. Sem o prazo, ou se libera cedo
(expondo o financiador) ou se retém indefinidamente (prejudicando o cedente).

**Bloqueia**: condição de liberação de efeito.

---

## Bloco 2 — Duplicata escritural

### 2.1 Regime aplicável e situação de operação

**Pergunta**: qual o regime legal e infralegal vigente da duplicata escritural;
está em operação efetiva; quem são os escrituradores autorizados?

**Por que importa**: a estratégia de entrega de `00-prd-mapa-de-produto.md` coloca
duplicata escritural na fase 2. Se o regime não está operacional, a fase precisa
ser reordenada.

**Bloqueia**: planejamento de fases; adaptadores de escrituração.

### 2.2 Aceite e apresentação ao sacado

**Pergunta**: como se dá o aceite na duplicata escritural; o sacado precisa ser
notificado da cessão; em que prazo e por quem?

**Por que importa**: determina se o ator A10 (sacado) entra no escopo.
`95-personas-jornadas.md` o mantém fora do MVP como decisão declarada; se a
notificação for obrigação legal da plataforma ou do cedente, a decisão muda, e
`16-design-notificacoes.md` (lacuna C5) precisa cobri-la.

**Bloqueia**: escopo do MVP para duplicatas; contexto de notificação.

### 2.3 Endosso e endosso-penhor no meio eletrônico

**Pergunta**: como se operacionaliza o endosso na duplicata escritural; o
endosso-penhor tem tratamento próprio?

**Por que importa**: são dois dos tipos de efeito listados em
`09-prd-effects-orchestrator.md`, sem mapeamento operacional.

**Bloqueia**: tipos de efeito para duplicatas.

---

## Bloco 3 — Enquadramento da plataforma

### 3.1 A plataforma precisa de autorização do Banco Central?

**Pergunta**: a atividade descrita — intermediação tecnológica entre cedentes e
financiadores, sem concessão de crédito, sem custódia e sem trânsito de recursos —
exige autorização? Em que hipóteses o modelo passaria a exigir?

**Por que importa**: define a viabilidade do modelo como descrito e os limites que
não podem ser cruzados. Todo o desenho parte da premissa de que a plataforma não é
instituição autorizada.

**Bloqueia**: nada tecnicamente; **tudo** do ponto de vista de viabilidade.

**Atenção**: identificar quais funcionalidades futuras cruzariam a linha — por
exemplo, conta de passagem, garantia própria, ou decisão de crédito.

### 3.2 A plataforma é obrigada de PLD/FT?

**Pergunta**: a plataforma tem obrigações próprias de prevenção à lavagem de
dinheiro, incluindo identificação de clientes e comunicação de operações
suspeitas? Ou essas obrigações recaem exclusivamente sobre os financiadores?

**Por que importa**: `02-design-kyb-kyc.md`, seção 2 assume que a plataforma é
auxiliar e não obrigada. Se estiver errado, falta o fluxo de comunicação de
operação suspeita — e a ausência é descumprimento, não lacuna de produto.

**Bloqueia**: fluxo de comunicação; papel do contexto de fraude.

### 3.3 Responsabilidade sobre o instrumento contratual

**Pergunta**: se a plataforma gera o instrumento a partir de modelo do
financiador, que responsabilidade assume sobre seu conteúdo?

**Por que importa**: `08-design-contracting.md`, seção 2 recomenda o modelo
híbrido por permitir validação estrutural de correspondência entre oferta e
contrato. O custo é assumir papel na produção do documento.

**Bloqueia**: decisão sobre geração do instrumento.

---

## Bloco 4 — Assinatura e prova

### 4.1 Nível de assinatura exigido

**Pergunta**: qual nível de assinatura eletrônica é exigido para contrato de
cessão de crédito e para cessão fiduciária de recebíveis? A exigência varia
conforme o tipo de efeito a registrar? As infraestruturas impõem exigência
própria?

**Por que importa**: `08-design-contracting.md`, seção 5.1 deixa o nível como
atributo configurável justamente por não ter a resposta. A escolha afeta custo,
conversão e validade.

**Bloqueia**: escolha de provedor de assinatura; fluxo de assinatura.

### 4.2 Requisitos de prova

**Pergunta**: que elementos de evidência são necessários para que uma assinatura
eletrônica seja oponível em juízo? Registro de tempo confiável é exigível?

**Por que importa**: define o conteúdo mínimo de `SignatureEnvelope` e do dossiê.

**Bloqueia**: composição do `ContractEvidenceBundle`.

---

## Bloco 5 — Dados pessoais

### 5.1 Base legal para consulta e compartilhamento

**Pergunta**: qual base legal sustenta a consulta à agenda de recebíveis e o
compartilhamento do dossiê cadastral com financiadores? É consentimento, execução
de contrato, ou legítimo interesse?

**Por que importa**: `03-design-consent-mandates.md`, seção 2 distingue
`DataProcessingBasis` de consentimento e afirma que a interface deve separar o que
é revogável do que é apenas informado. A separação só é correta se a base for
conhecida.

**Bloqueia**: textos dos instrumentos; comportamento da revogação.

### 5.2 Prazos de retenção e conflito com eliminação

**Pergunta**: quais prazos de retenção se aplicam a contrato, dossiê cadastral,
evidência de consentimento e registro de efeito? Qual fundamento sustenta a
retenção após o encerramento da relação, diante do direito de eliminação?

**Por que importa**: `93-requisitos-nao-funcionais.md`, seção 6 lista as
categorias com os prazos em branco, e `12-design-compliance-ledger.md` exige que
`RetentionPolicy` carregue o fundamento jurídico como campo obrigatório. Sem
resposta, não há resposta defensável a um pedido de eliminação.

**Bloqueia**: política de retenção; resposta a titular de dados.

### 5.3 Papéis no tratamento

**Pergunta**: nas relações plataforma-cedente, plataforma-financiador e
plataforma-provedor de dados, quem é controlador e quem é operador?

**Por que importa**: define responsabilidades, cláusulas contratuais com parceiros
e quem responde a titular.

**Bloqueia**: contratos com parceiros; fluxo de atendimento a titular.

---

## Bloco 6 — Transparência e comparabilidade

### 6.1 Divulgação de custo efetivo total

**Pergunta**: há exigência de divulgação de CET em operações de crédito ou
antecipação com tomador pessoa jurídica? Se sim, com qual metodologia?

**Por que importa**: `90-modelo-de-dados.md`, 3.3 propõe taxa efetiva mensal como
métrica comparável primária, calculada por fórmula própria. Se houver metodologia
regulada, ela prevalece e o campo passa a ser obrigatório, não proprietário.

**Bloqueia**: `PricingBreakdown`; comparação de ofertas.

### 6.2 Conflito de interesse em marketplace remunerado

**Pergunta**: há disciplina aplicável à remuneração de intermediário que ordena
ofertas de crédito? Existe exigência de divulgação?

**Por que importa**: `07-design-marketplace.md`, 3.3, R6 propõe que patrocínio não
inverta mérito, e registra que é decisão de produto. Se houver norma, deixa de
ser decisão.

**Bloqueia**: política de ranking; `18-design-billing.md`.

---

## Bloco 7 — Tributação

### 7.1 Incidência sobre a operação de antecipação

**Pergunta**: que tributos incidem sobre a operação, sobre quem, e quem é
responsável pelo recolhimento? A incidência varia conforme a estrutura jurídica
(cessão, desconto, operação com garantia)?

**Por que importa**: `90-modelo-de-dados.md`, 3.3 prevê `taxItems` em
`PricingBreakdown`. Sem saber o que incide, a comparabilidade entre ofertas com
estruturas distintas pode estar comparando bases diferentes.

**Bloqueia**: cálculo de valor líquido; comparação entre estruturas.

### 7.2 Tributação do serviço da plataforma

**Pergunta**: qual o enquadramento do serviço de intermediação; qual o município
de incidência; há retenções aplicáveis; qual a obrigação de emissão de documento
fiscal?

**Por que importa**: `18-design-billing.md`, seção 5.

**Bloqueia**: faturamento.

---

## Bloco 8 — Estruturas de funding

### 8.1 Requisitos para fundos e securitizadoras

**Pergunta**: que requisitos um fundo de investimento em direitos creditórios ou
uma securitizadora precisa observar para adquirir recebíveis originados por uma
plataforma? Há exigência quanto à formalização, ao registro ou à verificação de
lastro?

**Por que importa**: determina os requisitos de `PartnerAgreement` e os dados que
o financiador precisa receber.

**Bloqueia**: onboarding de financiadores não bancários.

### 8.2 Verificação de lastro

**Pergunta**: há exigência de verificação de lastro da operação mercantil
subjacente à duplicata? De quem é a obrigação?

**Por que importa**: se a obrigação alcança a plataforma, o contexto de fraude
precisa incluir verificação documental de nota fiscal, o que altera escopo,
custo e integrações.

**Bloqueia**: escopo do contexto de fraude.

---

## Resumo por impacto

| Prioridade | Pergunta | Bloqueia |
| --- | --- | --- |
| 1 | 3.1 — Autorização do Banco Central | Viabilidade do modelo |
| 2 | 1.1 — Tipos e códigos de efeito | Effects Orchestrator |
| 3 | 1.3 — Identidade da unidade de recebível | Portfolio, conciliação |
| 4 | 1.4 — Mecânica do opt-in | Onboarding, consentimento |
| 5 | 1.5 — Interoperabilidade | Dimensionamento do Gateway |
| 6 | 5.2 — Retenção e eliminação | Política de retenção |
| 7 | 4.1 — Nível de assinatura | Contratação |
| 8 | 1.7 — Janela de contestação | Liberação de efeito |
| 9 | 3.2 — Obrigações de PLD/FT | KYB, fraude |
| 10 | 1.2 — Prioridade entre efeitos | Conciliação |
| 11 | 2.1 — Situação da duplicata escritural | Ordem das fases |
| 12 | 6.1 — Divulgação de CET | Comparabilidade |
| 13 | 8.2 — Verificação de lastro | Escopo de fraude |
| 14 | 7.1 — Tributação da operação | Valor líquido |
| 15 | 5.1 — Base legal | Textos e revogação |

---

## Onde buscar

| Assunto | Fonte provável |
| --- | --- |
| Registro de recebíveis, registradoras autorizadas | Banco Central do Brasil |
| Normas do Conselho Monetário Nacional | Banco Central do Brasil |
| Leis federais | Portal oficial da legislação |
| Convenção entre registradoras | Próprias registradoras |
| Manuais de integração | Registradoras e escrituradores, sob acordo |
| Fundos de investimento em direitos creditórios | Comissão de Valores Mobiliários |
| Proteção de dados | Autoridade Nacional de Proteção de Dados |
| Tributação municipal | Legislação do município da sede |
| Interpretação aplicada ao caso | Assessoria jurídica especializada |

**Nota sobre a apuração**: as respostas dos blocos 1, 2 e 8 dependem de material
que frequentemente não é público — manuais de integração e a convenção entre
registradoras costumam exigir acordo comercial. O contato com as registradoras
deveria ocorrer cedo, porque parte das respostas que bloqueiam o desenho só vem
por esse caminho.
