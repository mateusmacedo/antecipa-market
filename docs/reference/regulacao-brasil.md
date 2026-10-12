# Regulação Aplicável — Brasil

> **Camada**: referência
> **Situação**: **parcialmente apurado**. Registro de recebíveis de cartão e
> duplicata escritural têm base confirmada; os demais blocos continuam abertos.
> **Atualizado em**: 18 de setembro de 2026

## Como ler este documento

| Marca | Significado |
| --- | --- |
| **[CONFIRMADO]** | Texto normativo lido diretamente |
| **[SECUNDÁRIA]** | Texto lido em fonte que reproduz a norma, não no repositório oficial — **conferir contra o original antes de decisão jurídica** |
| **[DECLARADO]** | Afirmação da própria empresa em seu site |
| **[ABERTO]** | Não apurado |

**Ressalva de método**: os portais oficiais (`planalto.gov.br`,
`bcb.gov.br`) recusaram acesso direto nesta apuração — o primeiro com
`ECONNRESET`, o segundo por servir páginas renderizadas em JavaScript. O que
está marcado **[SECUNDÁRIA]** foi lido em repositórios que reproduzem o texto
normativo. A estrutura (artigos, incisos, parágrafos) é consistente e o conteúdo
é coerente entre si, mas **nenhuma decisão jurídica deve se apoiar nesta versão
sem conferência no original**.

---

## 1. Registro de recebíveis de arranjo de pagamento

### 1.1 Norma central

**Resolução BCB nº 264, de 25 de novembro de 2022** — **[SECUNDÁRIA]**
(reproduzida em `legismap.com.br/leis-e-normas/resolucao-bcb-n-264-de-25-11-2022`,
consultada em 18/09/2026).

Norma correlata citada no texto: **Resolução nº 4.734, de 2019** — art. 4º, § 2º
e art. 6º, inciso II (desconstituição de gravames e ônus de limite de crédito).
**[ABERTO]** — texto não apurado.

### 1.2 Composição e identidade da unidade de recebível

**Art. 3º, § 2º** — **[SECUNDÁRIA]**. Uma unidade de recebível é identificada
por:

| # | Componente |
| --- | --- |
| I | Número de inscrição no CNPJ ou no CPF do usuário final recebedor |
| II | Identificação da instituição de pagamento credenciadora |
| III | Identificação do arranjo de pagamento |
| IV | Data de liquidação |
| V | Data da transação comercial |
| VI | Identificação da transação comercial |

**Esta é a resposta à pergunta B4 do resumo executivo** e fecha a lacuna
bloqueante P8 de `design/05-design-portfolio.md`: a identidade **é composta**,
não um identificador atômico atribuído pela registradora.

**Consequência construtiva direta**: a suspeita registrada em
`design/05-design-portfolio.md`, seção 5 — "a unidade de recebível pode não ter
identificador atômico estável" — está confirmada. A correspondência entre versões
de snapshot e na conciliação precisa usar a chave composta, e a data da transação
comercial e a identificação da transação entram nessa chave.

### 1.3 Valor da unidade de recebível

**Art. 3º, § 3º** — **[SECUNDÁRIA]**. O valor corresponde ao valor bruto total das
transações que compõem a unidade, deduzido **exclusivamente** de:

| # | Dedução |
| --- | --- |
| I | Taxas administrativas ou de remuneração da credenciadora ou do subcredenciador incidentes sobre as transações da unidade |
| II | Estornos por cancelamento, contestação ou fraude, no âmbito do arranjo |
| III | Liquidações de recebíveis constituídos, inclusive por antecipações pós-contratadas |
| IV | Bloqueio de valores de que trata o art. 8º |

**Art. 3º, § 4º** — recebíveis objeto de **antecipação pré-contratada** são
registrados na unidade correspondente à data de liquidação acordada na
antecipação.

**Consequência construtiva**: o modelo de `CardReceivableUnit` em
`design/90-modelo-de-dados.md`, 3.2 previu `grossAmount`, `adjustments` e
`netAmount` — a estrutura está correta, mas o enum de `adjustments` deve refletir
exatamente estas quatro categorias, e o campo de bloqueio precisa ser distinto de
ajuste (ver 1.4).

### 1.4 Bloqueio e reserva financeira

**Art. 8º** — **[SECUNDÁRIA]**:

- valores bloqueados **não compõem** o valor de recebíveis constituídos;
- o bloqueio **não pode reduzir** valores já alocados a contratos de negociação;
- é **vedada** a negociação de valores bloqueados pela credenciadora, incluindo
  antecipação pós-contratada;
- a reserva deve ser compatível com os riscos, com termos definidos no contrato
  entre credenciadora e usuário final recebedor.

**Consequência construtiva**: bloqueio é categoria própria, não ajuste. Um ativo
com valor bloqueado tem disponibilidade reduzida por razão distinta de gravame, e
a política de cálculo de `availableAmount` em `design/05-design-portfolio.md`,
seção 3 precisa considerá-lo como termo separado.

### 1.5 Campos obrigatórios da agenda

**Art. 13, parágrafo único** — **[SECUNDÁRIA]**. As informações que a
credenciadora deve disponibilizar ao usuário final recebedor incluem:

| # | Campo |
| --- | --- |
| I | Valor bruto total das transações que compõem cada unidade |
| II | Valor das deduções de cada unidade, discriminado conforme os incisos I a IV do art. 3º, § 3º |
| III | Valor de recebíveis constituídos de cada unidade, discriminado em: **(a) valor livre** e **(b) valores alocados para cada efeito de contrato** |
| IV | Especificações dos contratos com efeitos sobre a agenda: **(a) identificação da contraparte**, **(b) unidades alcançadas**, **(c) natureza do contrato**, **(d) tipo e valor do efeito (percentual ou valor fixo)** |

**Este é o achado de maior impacto desta apuração.** O inciso III distingue
explicitamente **valor livre** de **valores alocados por efeito** — exatamente a
distinção que `design/05-design-portfolio.md` propôs como
`availableAmount`/`knownEncumbrances`, agora com respaldo normativo.

O inciso IV confirma que a natureza do contrato e o tipo do efeito são
informados por unidade, e que o efeito pode ser **percentual ou valor fixo** —
detalhe que o modelo de `EffectTarget` em `design/90-modelo-de-dados.md`, 3.4
precisa acomodar (hoje prevê apenas `requestedAmount` monetário).

**Ressalva**: o art. 13 trata da obrigação da credenciadora perante o usuário
final recebedor. **[ABERTO]** — resta confirmar se o mesmo conjunto de campos é
o que trafega na consulta de agenda feita por terceiro autorizado.

### 1.6 Efeitos e negociação

**Art. 4º** — **[SECUNDÁRIA]**: a negociação de uma unidade de recebíveis deve
acarretar alteração, no sistema de registro, em favor do beneficiário da
operação. O trecho recuperado está truncado no inciso I.

**Art. 9º** — **[SECUNDÁRIA]**: credenciadoras devem encaminhar ao sistema de
registro informações sobre contratos de negociação realizados pelos usuários
finais recebedores **com instituições não financeiras**, e devem fazer constar em
contrato a obrigação de o usuário repassar essas informações e autorizar seu
envio.

**Art. 15, inciso X** — **[SECUNDÁRIA]**: os sistemas de registro recepcionam e
enviam às instituições financeiras os comandos dos participantes, **com
autorização do usuário final recebedor**, para **desconstituição de gravames e
ônus de limite de crédito** (art. 6º, II da Resolução 4.734/2019).

**[ABERTO] — a pergunta B3 permanece sem resposta completa.** O catálogo exaustivo
de tipos de efeito e seus códigos não está na Resolução 264; está nos **manuais
técnicos da convenção** (ver 1.8). A distinção entre trava de domicílio e cessão
continua a apurar.

### 1.7 Prazos confirmados

| Prazo | Artigo | Confiança |
| --- | --- | --- |
| Correção de inconsistências de conciliação: **2 dias úteis** | Art. 11, § 2º | [SECUNDÁRIA] |
| Resposta a contestação pela credenciadora: **3 dias úteis** | Art. 12 | [SECUNDÁRIA] |
| Portabilidade entre sistemas de registro: **30 dias**, prorrogáveis por **15** | Art. 16, § 1º | [SECUNDÁRIA] |
| Comunicação de alteração da convenção ao BCB: **15 dias** de antecedência | Art. 23, § 4º | [SECUNDÁRIA] |

Estes prazos alimentam `design/93-requisitos-nao-funcionais.md`, seção 3.2, que
até agora só tinha estimativas.

### 1.8 Interoperabilidade e a convenção

**Art. 16** — **[SECUNDÁRIA]**. Sistemas de registro devem adotar mecanismos de
interoperabilidade que permitam:

| # | Mecanismo |
| --- | --- |
| I | Verificação da **unicidade** do registro |
| II | Troca de informações sobre agendas |
| III | Troca de informações sobre contratos de negociação |
| IV | **Portabilidade** do registro de agendas e de contratos |
| V | Conciliação |
| VI | Troca de informações sobre contestações |
| VII | Demais informações, conforme a convenção |

**Art. 18** — a convenção entre as registradoras deve dispor sobre:

| # | Conteúdo |
| --- | --- |
| I | Troca de informações entre sistemas de registro e: (a) instituições financeiras e outros agentes financiadores; (b) credenciadoras e subcredenciadores; (c) serviços de interoperabilidade |
| II | **Padronização** de: (a) **leiaute dos arquivos** de registro; (b) **procedimento de autorização** para disponibilização de informações sobre agendas; (c) **parâmetros dos contratos de negociação** quanto à especificação dos recebíveis; (d) relatórios |
| III | Critérios para escolha do sistema de registro |
| IV | **Horários** para troca de informações |
| V | Estrutura de **tarifas** de interoperabilidade |
| VI | Termos e procedimentos de **adesão** e denúncia |
| VII | Direitos e obrigações dos participantes |
| VIII | **Manuais técnicos operacionais** |
| IX | Governança |

**Art. 18, § 9º e § 10** — os manuais técnicos, aprovados pelas signatárias e
encaminhados ao BCB, **integram a convenção**; havendo conflito, prevalece o
texto principal.

**O item II concentra as respostas que faltam**: leiaute dos arquivos,
procedimento de autorização (o opt-in) e parâmetros dos contratos — ou seja,
os tipos de efeito.

### 1.9 A convenção é pública — e isso muda o caminho

**Art. 19** — **[SECUNDÁRIA]**. As entidades registradoras **devem publicar, em
sítio único na internet**:

| # | Conteúdo |
| --- | --- |
| I | Versões **vigentes e históricas** da convenção e dos manuais técnicos |
| II | Informações estatísticas do ambiente de interoperabilidade |
| III | Documentos estabelecidos na convenção |

E o § 1º exige que estejam **permanentemente atualizados e disponíveis a todos os
participantes do ecossistema**, com controle de versões e detalhamento das
alterações.

**Isto corrige uma premissa dos documentos anteriores.** `design/04-design-receivables-gateway.md`,
seção 8 e `reference/provedores-integracao.md` registravam que a convenção e os
manuais técnicos "não são públicos; exigem acordo comercial". **A norma determina
o contrário**: devem estar publicados em sítio único e acessíveis.

**[ABERTO] — o endereço do sítio único não foi localizado** nesta apuração
(tentativas por adivinhação de domínio falharam, e a busca web estava
indisponível). Obtê-lo é ação de baixo custo e alto retorno: pode responder
sozinho as perguntas B1, B3, B5 a B9 e B14 do resumo executivo.

**Como obter**: perguntar a qualquer registradora, ou consultar o Banco Central.

### 1.10 Homologação

**Art. 24** — **[SECUNDÁRIA]**. Entidades registradoras e instituições
credenciadoras **devem participar de testes homologatórios de integração** entre
seus sistemas:

- ambas indicam **diretor responsável** pelos testes;
- credenciadoras asseguram que seus subcredenciadores participem;
- registradoras apresentam **plano de testes para aprovação do BCB**.

**Art. 25** — instrução normativa estabelecerá disposições complementares
(trecho truncado). **[ABERTO]** — identificar e ler essa instrução normativa.

**Consequência**: existe processo homologatório formal e regulado, com diretor
responsável designado. Isso confirma que a homologação prevista em
`design/13-design-partner-management.md`, seção 2 não é convenção interna — tem
contraparte regulatória.

**Ressalva importante**: o art. 24 trata de testes entre **registradoras e
credenciadoras**. **[ABERTO]** — o processo de integração de um **financiador ou
plataforma tecnológica** ao sistema de registro é regido pela convenção e pelos
manuais técnicos, não por este artigo.

---

## 2. Duplicata escritural

### 2.1 Lei nº 13.775, de 20 de dezembro de 2018 — **[SECUNDÁRIA]**

Lida em `www2.camara.leg.br/legin/fed/lei/2018/lei-13775-20-dezembro-2018-787500-publicacaooriginal-157047-pl.html`
e `jusbrasil.com.br/legislacao/661652291/lei-13775-18`, consultadas em 18/09/2026.
Registro na base oficial de legislação: `lexml.gov.br`, URN
`urn:lex:br:federal:lei:2018-12-20;13775`.

Ementa: "Dispõe sobre a emissão de duplicata sob a forma escritural; altera a Lei
nº 9.492, de 10 de setembro de 1997; e dá outras providências."

| Artigo | Conteúdo |
| --- | --- |
| Art. 1º | Dispõe sobre a emissão de duplicata sob forma escritural |
| Art. 2º | A duplicata da **Lei nº 5.474, de 18 de julho de 1968**, pode ser emitida sob forma escritural, para circulação como efeito comercial |
| Art. 3º | A emissão far-se-á mediante **lançamento em sistema eletrônico de escrituração** gerido por entidades que exerçam a atividade de escrituração de duplicatas escriturais |
| Art. 3º, § 1º | As entidades **devem ser autorizadas por órgão ou entidade da administração federal** direta ou indireta |
| Art. 3º, § 2º | No caso de escrituração por Central Nacional de Registro de Títulos e Documentos, cabe ao **oficial de registro do domicílio do emissor** |
| Art. 3º, § 3º | Se o oficial não estiver integrado ao sistema central, a competência transfere-se para a Capital da entidade federativa |

**[ABERTO]** — artigos 4º em diante não recuperados: aceite, endosso,
apresentação ao sacado, protesto.

**Consequência para o glossário**: o termo "escriturador" tem base legal
confirmada, e a autorização vem de órgão da administração federal — não
necessariamente do Banco Central, como o glossário sugeria.

### 2.2 A duplicata escritural ainda não está em produção

**[SECUNDÁRIA — análise de escritório de advocacia]**. Alerta publicado por
Lefosse em 25/06/2026, consultado em 18/09/2026
(`lefosse.com/noticias/alerta/duplicatas-escriturais-o-que-muda-na-emissao-no-registro-e-na-negociacao-de-recebiveis/`).

> "Atualmente, escrituradores e registradores encontram-se aguardando autorização
> do Banco Central para iniciar a etapa de **produção assistida**. Embora ainda
> não haja data definida para o início da produção assistida…"

**Este é o achado de maior impacto sobre o planejamento do produto.** A
estratégia de entrega de `00-prd-mapa-de-produto.md` coloca a duplicata escritural
na fase 2. Se os escrituradores ainda aguardam autorização para produção
assistida, a fase 2 não tem data possível — e depende de ato do Banco Central que
ainda não ocorreu.

**Recomendação**: tratar a fase 2 como dependente de gatilho externo, não de
prazo próprio, e concentrar o MVP em recebíveis de cartão, onde a infraestrutura
opera.

### 2.3 Cronograma de obrigatoriedade

**[SECUNDÁRIA]** — mesma fonte. **Resolução CMN nº 4.815**, alterada pela
**Resolução CMN nº 5.094/2023**: instituições financeiras deverão utilizar
**exclusivamente duplicatas escriturais** na negociação de recebíveis mercantis.

A obrigatoriedade é escalonada pelo porte do **sacador**, contada a partir da
implementação da última etapa das funcionalidades de interoperabilidade entre os
sistemas:

| Porte do sacador | Prazo |
| --- | --- |
| Grande porte (receita bruta anual acima de R$ 300 milhões) | 180 dias |
| Portes intermediários | **[ABERTO]** — não recuperado |
| Pequeno porte (receita bruta anual acima de R$ 360 mil e até R$ 4,8 milhões, nos termos da Lei Complementar nº 123/2006) | 540 dias |

**O encadeamento do gatilho** **[SECUNDÁRIA]**:

```text
Início das operações de no mínimo DOIS sistemas de escrituração
   -> prazos específicos para implementar as funcionalidades de interoperabilidade
      -> BCB publica ato informando a data de início
         -> a partir dessa data, contam os prazos de 180 a 540 dias por porte
            -> nova instrução normativa detalhará procedimentos e cronograma
```

**Consequência estratégica**: quando a obrigatoriedade entrar em vigor,
instituições financeiras **não poderão** negociar duplicata que não seja
escritural. Isso torna a fase 2 inevitável no médio prazo — mas o gatilho não
está sob controle do projeto, e a data é hoje indeterminada.

**[ABERTO]** — textos das Resoluções CMN 4.815 e 5.094/2023 não lidos; prazos dos
portes intermediários; se o ato do BCB já foi publicado após junho de 2026.

### 2.4 Fluxo de emissão e negociação

**[SECUNDÁRIA]** — mesma fonte.

| Etapa | Prazo |
| --- | --- |
| Escriturador emite a duplicata a pedido do **sacador** e a apresenta ao **sacado** | Apresentação em até **2 dias úteis** da emissão |
| Sacado manifesta **recusa**, pelo próprio sistema | Até **10 dias** |
| Sacado manifesta **aceite**, pelo próprio sistema | Até **15 dias** |
| Duplicata é levada a registro perante **entidade registradora** | — |
| Sacador **autoriza** o agente financiador a acessar sua "agenda de duplicatas" na registradora | — |
| Concretizada a operação, o agente **registra o ato**: transferência de titularidade, constituição de ônus e gravames | — |

Agentes financiadores citados: instituição financeira, FIDC, securitizadora ou
sociedade de fomento mercantil, entre outros.

**Afirmação central da fonte**: "sem a escrituração, não haverá como negociar o
recebível".

**Consequência para o ator A10 (sacado)**: `design/95-personas-jornadas.md`
registrou o sacado como fora do MVP, com a ressalva de que a decisão precisava ser
declarada. O fluxo acima mostra que **o sacado manifesta aceite ou recusa pelo
próprio sistema do escriturador** — ou seja, a interação existe, mas ocorre na
infraestrutura, não na plataforma. Isso **valida** a decisão de mantê-lo fora do
escopo, e a plataforma precisa apenas **consumir** o status de aceite, não
mediá-lo.

**Requisito derivado**: o modelo de `BookEntryInvoice` em
`design/90-modelo-de-dados.md` precisa carregar o estado de aceite (pendente,
aceito, recusado, aceite presumido) e a data-limite de manifestação — hoje o campo
"aceite" aparece em `05-prd-portfolio.md` sem estados definidos.

### 2.5 Aceite presumido

**[SECUNDÁRIA — análise setorial]**, `quicksoft.com.br/contestacao-na-duplicata-escritural/`:
a ausência de recusa no prazo aplicável, **combinada com evidências da entrega da
mercadoria ou da prestação do serviço**, pode caracterizar aceite presumido.

**Consequência para elegibilidade**: um ativo com aceite presumido tem base
probatória distinta de um com aceite expresso. Se o produto do financiador
distinguir os dois, isso é regra de elegibilidade — e exige que o campo de estado
de aceite (2.4) diferencie "aceito" de "aceite presumido".

### 2.6 Contestação e interoperabilidade

**[SECUNDÁRIA — análise setorial]**, mesma fonte: a contestação circula entre
participantes — sacadores, sacados, escrituradores, registradoras, depositários,
instituições financeiras e fundos — por interoperabilidade, de modo que uma
divergência conhecida por um participante não permaneça invisível para outro. Uma
vez registrada, verifica-se se a duplicata está vinculada a operação financeira,
cessão, garantia ou negociação em andamento.

**Consequência**: contestação é evento externo que pode atingir ativo já
antecipado. `design/09-design-effects-orchestrator.md`, seção 8 prevê disputa,
mas não a contestação originada pelo sacado. É cenário a acrescentar quando a
fase 2 for planejada.

---

## 3. Blocos ainda abertos

| Bloco | Situação |
| --- | --- |
| Enquadramento da plataforma (autorização do BCB) | **[ABERTO]** — pergunta A1, de impacto existencial |
| Obrigações de PLD/FT | **[ABERTO]** — pergunta A2 |
| Assinatura eletrônica | **[ABERTO]** — pergunta A8 |
| Proteção de dados: base legal, retenção, papéis | **[ABERTO]** — perguntas A12 a A14 |
| Estruturas de funding (fundos, securitizadoras) | **[ABERTO]** — pergunta A15 |
| Divulgação de custo efetivo total para PJ | **[ABERTO]** — pergunta A15 do roteiro |
| Tributação da operação e do serviço | **[PARCIAL]** — há material sobre IOF indexado (Decretos 12.466/2025 e 12.499/2025, alterando o Decreto 6.306/2007), mas não analisado quanto à incidência em antecipação de recebíveis |
| Resolução nº 4.734/2019 | **[ABERTO]** — citada pela Resolução 264, texto não apurado |
| Instrução normativa do art. 25 da Resolução 264 | **[ABERTO]** — não identificada |

---

## 4. O que esta apuração mudou

| Documento | Impacto |
| --- | --- |
| `design/05-design-portfolio.md` | Lacuna P8 (identidade da UR) **fechada** — é chave composta de 6 componentes |
| `design/90-modelo-de-dados.md` | Lacuna L1 (campos canônicos) **parcialmente fechada**; `EffectTarget` precisa aceitar efeito percentual |
| `design/09-design-effects-orchestrator.md` | Lacuna F8 (códigos de efeito) **continua aberta** — está nos manuais técnicos |
| `design/04-design-receivables-gateway.md` | Premissa de "documentação não pública" **incorreta** — a convenção e os manuais devem estar em sítio único |
| `design/93-requisitos-nao-funcionais.md` | Quatro prazos regulatórios confirmados substituem estimativas |
| `reference/glossario.md` | Termos "unidade de recebível", "escriturador", "gravame" e "portabilidade" ganham base normativa |

---

## 5. Próximo passo de maior retorno

**Localizar o sítio único do art. 19.** Ele deve conter a convenção e os manuais
técnicos — com leiaute de arquivos, procedimento de autorização e parâmetros dos
contratos de negociação. É onde estão as respostas que hoje bloqueiam o Gateway e
o Effects Orchestrator, e o acesso é obrigatório por norma, não negociável
comercialmente.
