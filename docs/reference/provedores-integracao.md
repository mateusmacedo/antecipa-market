# Provedores e Integrações

> **Camada**: referência
> **Situação**: **parcialmente apurado**. O caminho de integração está mapeado em
> nível normativo; os detalhes técnicos por provedor continuam abertos.
> **Atualizado em**: 18 de setembro de 2026

## Como ler este documento

| Marca | Significado |
| --- | --- |
| **[NORMA]** | Deriva de texto normativo — ver `regulacao-brasil.md` |
| **[DECLARADO]** | Afirmação da própria empresa em seu site |
| **[ABERTO]** | Não apurado |

---

## 1. Correção de premissa

As versões anteriores deste documento e de
`design/04-design-receivables-gateway.md` registravam que a documentação de
integração das registradoras "não é pública; requer contato comercial".

**Isso está incorreto.** O art. 19 da Resolução BCB nº 264/2022 **[NORMA]**
obriga as entidades registradoras a publicar, **em sítio único na internet**, as
versões vigentes e históricas da convenção e dos manuais técnicos, permanentemente
atualizadas e **disponíveis a todos os participantes do ecossistema**, com
controle de versões.

E o art. 18 define o que esses documentos contêm — inclusive os três itens que
mais bloqueiam o desenho:

| Item da convenção | Responde |
| --- | --- |
| Padronização do **leiaute dos arquivos** de registro | Formato de ingestão |
| Padronização do **procedimento de autorização** para disponibilização de agendas | Mecânica do opt-in |
| Padronização dos **parâmetros dos contratos de negociação** quanto à especificação dos recebíveis | Tipos e códigos de efeito |

**Consequência prática**: o acesso à especificação técnica do registro de
recebíveis de cartão **não depende de negociação comercial**. Depende de localizar
o sítio único.

**[ABERTO] — o endereço não foi localizado nesta apuração.** As tentativas de
adivinhação de domínio falharam e a busca web estava indisponível. Obter o
endereço é a ação de maior retorno e menor custo de todo o levantamento:
perguntar a qualquer registradora, ou consultar o Banco Central.

---

## 2. Registradoras de recebíveis de arranjo de pagamento

### 2.1 O que foi possível confirmar

| Entidade | Situação declarada | Fonte | Confiança |
| --- | --- | --- | --- |
| **Núclea** | "A Núclea é autorizada pelo Banco Central do Brasil a atuar como registradora de recebíveis de cartões". Declara ser "a maior registradora de recebíveis de cartões do Brasil", com "quase R$ 3 trilhões processados todos os anos" | `nuclea.com.br/registradora-de-recebiveis/`, 18/09/2026 | **[DECLARADO]** |
| **CERC** | Apresenta-se como registradora especializada em recebíveis. Declara: "registra para constituição de ônus e gravames sobre ativos"; "registra ativos e transações para controle formal de propriedade"; "atua na publicidade formal de transações" | `cerc.inf.br`, 18/09/2026 | **[DECLARADO]** |
| **B3** | Mantém área de registro; a página de recebíveis retornou erro 500 na consulta | `b3.com.br`, 18/09/2026 | **[ABERTO]** |
| **TAG** | Domínio `tagbrasil.com` está parqueado; `tag.com.br` não resolve. Domínio correto não identificado | — | **[ABERTO]** |

**Nenhuma dessas entradas substitui a lista oficial.** A relação de entidades
autorizadas deve ser obtida junto ao Banco Central — a página correspondente é
renderizada por JavaScript e o acesso ao dado estruturado foi bloqueado nesta
apuração.

### 2.2 Segmentação declarada pela CERC

O formulário de contato da CERC oferece as opções de interesse: "Sou
financiador", "Sou estabelecimento comercial", "Sou credenciadora/subcredenciadora"
e "Outros", com segmentos incluindo "SupplyChain" e
"Fintech/Pagamentos/Bancos Digitais/Marketplace" **[DECLARADO]**.

Útil para o primeiro contato: a plataforma se apresenta provavelmente como
fintech, ou como intermediária a serviço de financiadores — e a escolha muda o
interlocutor.

---

## 3. Como funciona a integração — o que a norma determina

### 3.1 Papéis no ecossistema

**[NORMA]** — Resolução BCB nº 264/2022:

```text
Usuário final recebedor (o cedente)
        │ autoriza consulta e negociação
        ▼
Instituição credenciadora / subcredenciador
        │ envia agendas e contratos ao registro
        ▼
Sistema de registro (entidade registradora)
        │ interopera com os demais sistemas
        ▼
Instituições financeiras e outros agentes financiadores
```

A plataforma não aparece nesse desenho como papel próprio. **[ABERTO]** — em que
qualidade ela se conecta: como participante do sistema de registro, como
prestador de serviço a um financiador participante, ou por outra via. É a
pergunta mais importante do primeiro contato.

### 3.2 Obrigações dos sistemas de registro que afetam quem integra

**[NORMA]** — art. 15, incisos recuperados:

| Inciso | Obrigação | Por que importa |
| --- | --- | --- |
| VI | Comunicar incidentes operacionais aos demais sistemas e ao BCB | Existe canal formal de incidente |
| VIII | Conciliar parâmetros de contratos, valores de agendas e **autorizações de consulta** recebidos por interoperabilidade | As autorizações são conciliadas entre sistemas |
| IX | Acatar e tratar **contestações** de participantes sobre registro, consulta, negociação e liquidação | Existe processo formal de contestação |
| X | Recepcionar e enviar às instituições financeiras os comandos dos participantes, **com autorização do usuário final recebedor**, para **desconstituição de gravames e ônus** | Confirma o caminho de liberação de efeito |
| XI | Recepcionar e enviar comandos às credenciadoras e subcredenciadores | — |

### 3.3 Interoperabilidade — a resposta à pergunta B14

**[NORMA]** — art. 16. Os sistemas de registro devem adotar mecanismos que
permitam unicidade do registro, troca de informações sobre agendas e contratos,
**portabilidade** entre sistemas, conciliação e troca de informações sobre
contestações.

**Consequência para o Gateway**: existe camada de interoperabilidade obrigatória
entre registradoras. Isso **reduz** o risco de integração N-para-N total, mas
**[ABERTO]** — resta confirmar se um consumidor externo consulta uma registradora
e obtém a posição consolidada, ou se precisa consultar cada uma.

A norma trata da interoperabilidade **entre sistemas de registro**; não
necessariamente expõe ponto único a terceiros.

**Portabilidade**: 30 dias, prorrogáveis por 15, contados do recebimento do
pedido pelo sistema de destino **[NORMA]**, art. 16, § 1º.

### 3.4 Homologação

**[NORMA]** — art. 24. Registradoras e credenciadoras **devem participar de
testes homologatórios de integração**, com diretor responsável designado por cada
parte e plano de testes aprovado pelo Banco Central.

**Ressalva**: o artigo trata de testes entre **registradoras e credenciadoras**.
O processo de homologação de um financiador ou de uma plataforma é regido pela
convenção e pelos manuais técnicos. **[ABERTO]**.

### 3.5 Prazos que a integração precisa respeitar

**[NORMA]**:

| Prazo | Origem |
| --- | --- |
| Correção de inconsistências de conciliação: 2 dias úteis | Art. 11, § 2º |
| Resposta a contestação pela credenciadora: 3 dias úteis | Art. 12 |
| Portabilidade: 30 dias + 15 | Art. 16, § 1º |
| Comunicação de alteração da convenção: 15 dias de antecedência | Art. 23, § 4º |

O último é relevante para o desenho: mudanças na convenção — que inclui os
manuais técnicos — têm aviso prévio mínimo de 15 dias. Isso dimensiona a janela
de adaptação prevista em `design/13-design-partner-management.md`, seção 7.

### 3.6 Grade de horários e tarifas

**[NORMA]** — art. 18, incisos IV e V: a convenção define os horários para troca
de informações e a estrutura de tarifas de interoperabilidade.

**[ABERTO]** — valores e janelas concretos estão nos manuais técnicos.

---

## 4. Estrutura a preencher, por provedor

Mantida da versão anterior. As três primeiras linhas mudaram de status.

| Dimensão | Situação |
| --- | --- |
| Identificação e autorização | **[PARCIAL]** — Núclea e CERC declaram atuação; lista oficial pendente |
| Protocolo por operação | **[ABERTO]** — nos manuais técnicos |
| Leiaute dos arquivos | **[ABERTO]** — padronizado pela convenção, art. 18, II, (a) |
| Autenticação | **[ABERTO]** |
| Operações mapeadas às portas canônicas | **[ABERTO]** |
| Limites | **[ABERTO]** |
| Calendário e horário de corte | **[ABERTO]** — convenção, art. 18, IV |
| Callback, consulta, arquivo | **[ABERTO]** |
| **Consulta de estado por operação** | **[ABERTO]** — continua sendo a pergunta técnica mais importante |
| Cobertura de gravame e validade da posição | **[PARCIAL]** — o art. 13 define os campos devidos ao usuário final; falta confirmar o que trafega na consulta por terceiro |
| Códigos de erro | **[ABERTO]** |
| Homologação | **[PARCIAL]** — existe processo regulado entre registradora e credenciadora; para plataforma, aberto |
| Custo | **[ABERTO]** — tarifas de interoperabilidade na convenção |

---

## 5. Outras categorias de provedor

Sem apuração nesta rodada.

| Categoria | Situação |
| --- | --- |
| **Escrituradores de duplicata escritural** | **Nenhum em produção** — ver abaixo |
| Provedores de dados cadastrais (KYB/KYC) | **[PARCIAL]** — ver [`bureaus-dados.md`](./bureaus-dados.md) |
| Provedores de assinatura eletrônica | **[PARCIAL]** — ver [`assinatura-eletronica.md`](./assinatura-eletronica.md) |
| Validação de titularidade de conta bancária | **[ABERTO]** — controle antifraude de maior valor; nenhum provedor identificado, ver [`bureaus-dados.md`](./bureaus-dados.md), seção 7 |

### 5.1 Escrituradores de duplicata escritural — situação

**[SECUNDÁRIA]** — conforme [`regulacao-brasil.md`](./regulacao-brasil.md),
seção 2.2: escrituradores e registradores **aguardam autorização do Banco Central
para iniciar a produção assistida**, sem data definida (posição de junho de 2026).

**Consequência**: não há escriturador em operação com o qual integrar. Mapear
provedores nesta categoria é prematuro — o que importa agora é acompanhar o ato
do BCB que dá início à contagem dos prazos.

**O que acompanhar**:

| # | Item |
| --- | --- |
| 1 | Ato do BCB informando a data de início das operações |
| 2 | Instrução normativa que detalhará procedimentos e cronograma |
| 3 | Quais entidades obtêm autorização, e quando |
| 4 | Se a autorização para escriturar e para registrar recai sobre as mesmas entidades |

**Nota**: a Lei 13.775/2018, art. 3º, § 1º exige que os escrituradores sejam
autorizados por **órgão ou entidade da administração federal** — não
necessariamente pelo Banco Central. A fonte setorial indica que, na prática, a
autorização para produção está com o BCB. **[ABERTO]** — confirmar a competência.

---

## 6. Perguntas para o primeiro contato

Revisadas à luz do que a norma já respondeu. As quatro primeiras são as que a
convenção não responde e só o provedor esclarece.

| # | Pergunta |
| --- | --- |
| 1 | **Onde está o sítio único do art. 19?** (convenção e manuais técnicos) |
| 2 | **Em que qualidade uma plataforma tecnológica se conecta** — participante do sistema de registro, prestador de serviço a financiador participante, ou outra? |
| 3 | **Existe operação de consulta de estado para cada operação de escrita?** |
| 4 | Qual o **processo e o prazo de homologação** para esse perfil de integrador? |
| 5 | A consulta de agenda por terceiro autorizado retorna os mesmos campos do art. 13 — incluindo **valor livre e valores alocados por efeito**? |
| 6 | Consultando uma registradora, obtém-se a posição consolidada de todas, ou é preciso integrar com cada uma? |
| 7 | Qual o prazo típico e o máximo de confirmação de um efeito? |
| 8 | Qual o modelo de tarifação para este perfil de uso? |
| 9 | Existe ambiente de homologação com dados representativos? |
| 10 | Procedimento e prazo de revogação de credencial em emergência; contato para incidente |

---

## 7. Quem depende deste documento

| Documento | O que espera daqui | Situação |
| --- | --- | --- |
| `design/04-design-receivables-gateway.md` | Mapeamento por provedor | **[ABERTO]** — depende do sítio único |
| `design/05-design-portfolio.md` | Validade da posição por fonte | **[ABERTO]** |
| `design/06-design-eligibility.md` | Parâmetro de `AST-FRS-001` | **[ABERTO]** |
| `design/09-design-effects-orchestrator.md` | Prazos e suporte a parcial | **[ABERTO]** |
| `design/11-design-settlement-reconciliation.md` | Formato dos arquivos; janela de contestação | **[PARCIAL]** — prazo de contestação da credenciadora confirmado em 3 dias úteis |
| `design/13-design-partner-management.md` | Critérios de homologação | **[PARCIAL]** |
| `design/10-design-funding.md` | Validação de titularidade de conta | **[ABERTO]** |
