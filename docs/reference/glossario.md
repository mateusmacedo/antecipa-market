# Glossário

> **Camada**: referência
> **Origem**: lacuna identificada — os PRDs usam termos de domínio sem
> definição. Um documento que diz "unidade de recebível", "escriturador",
> "gravame" e "endosso-penhor" sem defini-los só é legível por quem já conhece
> o setor.
> **Status**: proposta

## Como ler este documento

Cada termo traz a definição usada **nesta documentação**. Onde a definição
depende de norma ou de padrão de mercado que ainda não foi apurado, há a marca
**[A CONFIRMAR]** e a remissão a `docs/reference/regulacao-brasil.md` ou
`provedores-integracao.md`.

Definição operacional e definição jurídica podem divergir. Onde isso importa, está
assinalado.

---

## A

**Agenda de recebíveis**
Conjunto de valores que um estabelecimento tem a receber de credenciadores em
datas futuras, decorrente de transações com cartão já realizadas. É a base sobre a
qual se constituem as unidades de recebível. Na plataforma, é obtida por consulta
autorizada e materializada em `PortfolioSnapshot`.

**Ajuste**
Alteração de valor de um recebível por evento posterior à transação — cancelamento
parcial, estorno, taxa retida. Reduz o valor líquido esperado. Modelado como item
em `CardReceivableUnit.adjustments`.

**Antecipação**
Operação pela qual o titular de um recebível futuro obtém recursos antes do
vencimento, mediante deságio. Pode assumir formas jurídicas distintas — cessão de
crédito, desconto, operação com garantia — com consequências diferentes.
**[A CONFIRMAR]** quais formas a plataforma suportará.

**Aquisição definitiva**
Compra do recebível pelo financiador, sem direito de regresso contra o cedente. O
risco de inadimplência passa integralmente ao adquirente. Contrasta com operação
com coobrigação.

**Arranjo de pagamento**
Conjunto de regras que disciplina um serviço de pagamento com cartão, definindo
participantes e responsabilidades. Na plataforma, é dimensão de elegibilidade
(`AST-ARR-001`) e de concentração (`LOT-CNC-004`).

**Aceite**
Ato pelo qual o sacado reconhece a obrigação representada pela duplicata. Duplicata
aceita tem força distinta da não aceita. **[A CONFIRMAR]** o tratamento na
duplicata escritural.

---

## B

**Baixa**
Registro, pela fonte oficial, de que o ativo foi quitado. Distinto de "pago" e de
"repassado" — ver `docs/design/11-design-settlement-reconciliation.md`, 2.

**Beneficiário final**
Pessoa natural que, em última instância, controla ou se beneficia de uma pessoa
jurídica. Identificá-lo exige desdobrar a cadeia societária.
**[A CONFIRMAR]** o critério aplicável.

**Bounded context** (contexto delimitado)
Fronteira dentro da qual um modelo de domínio é coerente e os termos têm
significado único. Os 13 contextos dos PRDs, mais os 5 propostos na camada de
design, são as fronteiras da plataforma.

---

## C

**Cedente**
Quem transfere o direito creditório. Na plataforma, a empresa que antecipa seus
recebíveis. É o cliente principal.

**Cessão de crédito**
Transferência da titularidade de um direito creditório do cedente ao cessionário.

**Cessão fiduciária**
Transferência da titularidade em garantia: o credor recebe a titularidade com a
finalidade de garantir uma obrigação, devendo restituí-la quando cumprida.
Distinta da cessão definitiva quanto ao efeito.

**Chargeback**
Contestação de uma transação com cartão pelo portador, que pode resultar em
estorno após a liquidação. Cria janela de exposição posterior à liquidação — ver
`docs/design/11-design-settlement-reconciliation.md`, 8.

**Coobrigação**
Responsabilidade do cedente pelo pagamento, caso o devedor não pague. Sua presença
muda a natureza econômica da operação e a comparabilidade entre ofertas.

**Credenciador**
Instituição que habilita estabelecimentos a aceitar cartões e é responsável pela
liquidação dos valores a eles devidos. Na plataforma, dimensão de elegibilidade e
de concentração.

**CET (custo efetivo total)**
Medida que consolida todos os custos de uma operação de crédito em taxa única.
**[A CONFIRMAR]** se há exigência de divulgação para tomador pessoa jurídica — a
resposta determina se o campo é obrigatório em `PricingBreakdown`.

---

## D

**Deságio**
Diferença entre o valor de face do recebível e o valor pago pelo financiador. É o
principal componente do custo da antecipação.

**Depositário central**
Entidade que mantém o registro centralizado de ativos e sua titularidade.

**Direito creditório**
Direito de receber um valor de um devedor. Duplicatas e unidades de recebível são
espécies.

**Dupla negociação**
Antecipação do mesmo recebível a mais de um financiador. É a fraude estrutural do
setor, e o registro obrigatório existe em grande medida para impedi-la. Na
plataforma, o controle real é a revalidação junto à fonte imediatamente antes do
registro do efeito — não a reserva interna, que só vale dentro da plataforma.

**Duplicata**
Título de crédito emitido pelo vendedor contra o comprador, representando uma
obrigação decorrente de compra e venda mercantil ou de prestação de serviços.

**Duplicata escritural**
Duplicata emitida e mantida sob forma eletrônica em sistema de escrituração
autorizado, em vez de cartular. **[A CONFIRMAR]** o regime aplicável e a situação
de operação.

**Duplicata fria**
Duplicata emitida sem lastro em operação mercantil real. Fraude clássica do
desconto de duplicatas.

---

## E

**Endosso**
Ato pelo qual o titular de um título de crédito transfere seus direitos a outrem.

**Endosso-penhor**
Endosso que transfere o título em garantia, não em propriedade. O endossatário
pode cobrar, mas detém o título como garantia de outra obrigação.

**Escriturador**
Entidade autorizada a manter o sistema de escrituração de duplicatas.
**[A CONFIRMAR]** quem opera no Brasil.

**Estabelecimento recebedor**
Unidade que recebe os valores de transações com cartão, identificada por
identificador fiscal próprio e credenciada em arranjos de pagamento. É o titular
dos recebíveis — distinção que importa em grupos com matriz e filiais (ver
`docs/design/01-design-identity-organization.md`, 6).

**Efeito (regulatório)**
Nesta documentação, qualquer consequência jurídica constituída sobre um recebível
e registrada na infraestrutura competente: cessão, cessão fiduciária, penhor,
garantia, alteração de titularidade. Modelado como `RegulatoryEffect`.

**Eficácia**
Momento em que o efeito passa a produzir consequências perante terceiros. Na
plataforma, ocorre com a confirmação da infraestrutura — nunca com o aceite
técnico do comando. A distinção é a invariante mais importante de
`09-prd-effects-orchestrator.md`.

---

## F

**FIDC**
Fundo de investimento que aplica em direitos creditórios. Pode ser financiador na
plataforma. **[A CONFIRMAR]** requisitos para aquisição via plataforma.

**Financiador**
Quem fornece os recursos e assume o risco: banco, FIDC, securitizadora, fintech de
crédito, factoring. Na plataforma, é quem decide o crédito — a plataforma não.

---

## G

**Gravame**
Ônus que recai sobre um ativo, restringindo sua livre disposição. Um recebível
gravado não está integralmente disponível. Na plataforma, "ausência de informação
sobre gravame não significa ausência de gravame" é invariante — ver
`docs/design/05-design-portfolio.md`, 3.

---

## I

**Idempotência**
Propriedade de uma operação que, repetida com a mesma chave, produz o mesmo
resultado sem efeito adicional. Fundamental aqui porque o efeito externo é
irreversível — ver `docs/design/00-arquitetura-tecnica.md`, 7.3.

**Infraestrutura (do mercado financeiro)**
Nesta documentação, o conjunto de entidades autorizadas que mantêm o registro
oficial: registradoras, escrituradores, depositários centrais. São a fonte de
verdade dos ativos — a plataforma nunca é.

---

## L

**Liquidação**
Extinção de uma obrigação pelo pagamento efetivo e sua compensação. Distinta de
"pagamento reportado" e de "baixa".

---

## O

**Opt-in**
Autorização, pelo titular, para que um terceiro acesse sua agenda de recebíveis.
Na plataforma, é `ReceivablesOptIn`, distinto do consentimento de tratamento de
dados. **[A CONFIRMAR]** natureza jurídica e mecânica de ativação.

---

## P

**Portabilidade (de recebíveis)**
Transferência de um efeito de um credor para outro sem extinguir a operação
subjacente. **[A CONFIRMAR]** se as infraestruturas suportam.

**Prazo médio ponderado**
Média dos prazos dos ativos de um lote, ponderada pelo valor de face. Base do
cálculo de taxa efetiva mensal — ver `docs/design/90-modelo-de-dados.md`, 3.3.

---

## R

**Registradora**
Entidade autorizada a manter o registro de recebíveis de arranjos de pagamento e
dos ônus sobre eles. **[A CONFIRMAR]** quais operam no Brasil.

**Repasse**
Entrega efetiva dos recursos ao beneficiário do efeito. É a condição para liberar
o efeito — ver `docs/design/11-design-settlement-reconciliation.md`, 2 e 7.

---

## S

**Sacado**
Devedor da duplicata — quem deve pagar. Tem papel jurídico ativo (aceite,
notificação da cessão, pagamento) e está fora do escopo do MVP, com implicações
registradas em `docs/design/95-personas-jornadas.md`, A10.

**Sacador**
Emitente da duplicata — o credor original, normalmente o cedente na plataforma.

**Securitizadora**
Companhia que adquire direitos creditórios e emite títulos lastreados neles. Pode
ser financiadora.

**Snapshot (de portfólio)**
Fotografia imutável da posição dos recebíveis em determinado instante, com origem
e `asOf`. Toda oferta referencia o snapshot que a originou.

**Subcredenciador**
Participante que habilita estabelecimentos sem participar diretamente da
liquidação no arranjo.

---

## T

**Titularidade**
Qualidade de titular de um direito creditório. Sua alteração é um dos efeitos
registráveis.

**Trava de domicílio**
Mecanismo pelo qual os recursos de um recebível são direcionados obrigatoriamente
a uma conta determinada, como garantia. **[A CONFIRMAR]** distinção precisa em
relação à cessão e ordem de prioridade — é uma das perguntas centrais de
`docs/reference/regulacao-brasil.md`.

---

## U

**UR (unidade de recebível)**
Unidade de agregação dos recebíveis de arranjo de pagamento, usada para registro e
constituição de efeitos. Sua composição segue padrão regulatório.
**[A CONFIRMAR]** composição exata e esquema de identidade — lacuna bloqueante
registrada em `docs/design/05-design-portfolio.md`, 5.

---

## Termos da arquitetura

| Termo | Definição |
| --- | --- |
| **Agregado** | Conjunto de entidades tratado como unidade de consistência |
| **Comando** | Intenção de alterar estado, dirigida a um proprietário |
| **Evento** | Fato ocorrido, imutável, publicado a consumidores |
| **Envelope** | Metadados padronizados que acompanham todo evento |
| **Inbox** | Tabela que registra eventos consumidos, garantindo idempotência |
| **Outbox** | Tabela que registra eventos a publicar na mesma transação do estado |
| **Porta canônica** | Interface estável que isola o domínio do protocolo do parceiro |
| **Saga** | Sequência de passos com compensação de negócio, sem rollback técnico |
| **Snapshot** | Ver acima |
| **Tenant** | Domínio de isolamento de dados |
| **`asOf`** | Instante a que uma informação se refere |
| **`correlationId`** | Identificador que liga todos os fatos de uma mesma operação |
| **`causationId`** | Identificador do fato que causou outro |
| **`UNKNOWN_OUTCOME`** | Estado em que não se sabe se um comando externo surtiu efeito |

---

## Termos deliberadamente evitados

| Termo | Por que evitar | Usar |
| --- | --- | --- |
| "Aprovado" para efeito | Confunde aceite técnico com eficácia | "Confirmado" / "Eficaz" |
| "Cliente" sem qualificar | Ambíguo entre cedente e financiador | "Cedente" ou "financiador" |
| "Registro" sem qualificar | Confunde registro na infraestrutura com persistência local | "Registro na infraestrutura" ou "persistência" |
| "Aprovação de crédito" pela plataforma | A plataforma não aprova crédito | "Elegibilidade" (técnica) |
| "Saldo" | Sugere custódia | "Valor disponível" / "valor líquido esperado" |
| "Transferência pela plataforma" | A plataforma não transita recursos | "Desembolso pelo financiador" |
