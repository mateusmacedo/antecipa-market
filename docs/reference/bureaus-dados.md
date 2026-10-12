# Bureaus e Fontes de Dados

> **Camada**: referência
> **Situação**: **parcialmente apurado**. Um provedor está mapeado em
> profundidade; os demais, em nível de catálogo.
> **Atualizado em**: 18 de setembro de 2026

## Como ler este documento

| Marca | Significado |
| --- | --- |
| **[DOC]** | Lido na documentação técnica do provedor |
| **[SITE]** | Lido em material comercial do provedor |
| **[ABERTO]** | Não apurado |

**Ressalva sobre preços**: os valores citados foram lidos na documentação
pública do provedor em 18/09/2026. Preço publicado não é preço contratado —
serve para dimensionar ordem de grandeza e comparar datasets entre si, não para
orçamento.

---

## 1. O que este contexto precisa consultar

Derivado do catálogo de verificações de
[`../design/02-design-kyb-kyc.md`](../design/02-design-kyb-kyc.md), seção 3, e
dos sinais de risco de
[`../design/15-design-fraude-risco.md`](../design/15-design-fraude-risco.md),
seção 3.

| Necessidade | Verificação | Criticidade |
| --- | --- | --- |
| Situação cadastral e atividade da empresa | `CHK-REG-001` a `CHK-REG-005` | Alta |
| Quadro societário e beneficiário final | `CHK-OWN-001` a `CHK-OWN-004` | Alta |
| Identidade e poderes do representante | `CHK-REP-001` a `CHK-REP-003` | Alta |
| Listas restritivas e exposição política | `CHK-LST-001` a `CHK-LST-004` | Alta |
| **Titularidade de conta bancária** | `CHK-BNK-001` | **Crítica** — controle antifraude de maior valor |
| Autorização de documento fiscal | `RSK-DOC-001`, `RSK-DOC-002` | Alta, se a verificação de lastro for obrigação |
| Sinais de relacionamento entre entidades | `RSK-REL-001` a `RSK-REL-005` | Média |
| Monitoramento de alteração cadastral | Revalidação por evento | Alta |

---

## 2. BigDataCorp

Provedor com a documentação pública mais completa encontrada — inclui tabela de
preços por dataset e especificação de API. Documentação em
`docs.bigdatacorp.com.br`, com índice em formato `llms.txt` **[DOC]**.

### 2.1 Organização da oferta

| API | Conteúdo |
| --- | --- |
| **Companies** | Dados básicos, econômicos, envolvimento político, ESG, compliance regulatório, risco |
| **People** | Dados básicos, risco, compliance regulatório, sociodemográficos, comportamento, contatos, veículos |
| **Validations** | Validações (escopo não detalhado nesta apuração) |
| **On-Demand** | Consultas de empresas, de pessoas, de veículos, **de notas fiscais**; certidões de empresas, de pessoas e de endereços |
| **Marketplace** | Crédito de empresas, endereços, telefones, IP — datasets de parceiros |
| **Lawsuits** | Processos judiciais |

### 2.2 Datasets relevantes ao contexto

Preços por consulta, conforme faixa de volume mensal **[DOC]**:

| Dataset | Nome técnico | 1–10 mil | 500 mil–1 mi | Atende |
| --- | --- | --- | --- | --- |
| **KYC e Compliance do Grupo Econômico** | `economic_group_kyc` | R$ 0,200 | R$ 0,163 | `CHK-OWN-002`, `CHK-LST-001` a `CHK-LST-003` |
| Representante Legal (Receita Federal) | `ondemand_legal_representative_company` | R$ 0,040 | R$ 0,033 | `CHK-REP-002` |
| Status do CAEPF | `ondemand_rf_status_caepf` | R$ 0,060 | R$ 0,049 | Cadastro de PF com atividade econômica |
| Processos judiciais de familiares | `family_lawsuits_distribution_data` | R$ 0,100 | — | `RSK-REL-001` |
| Score de Crédito — Quod | `partner_quod_credit_score_company` | R$ 2,410 | R$ 1,963 | Informação ao financiador, não decisão da plataforma |
| Flags Negativos — Quod | `partner_quod_credit_risk_company` | R$ 1,210 | R$ 0,986 | Idem |

### 2.3 O achado de maior impacto: `economic_group_kyc`

**[DOC]** — o dataset "consolida informações de KYC e compliance regulatório
aplicáveis ao **grupo econômico completo** associado ao CNPJ consultado,
abrangendo empresas e pessoas vinculadas por **relações societárias diretas e
indiretas**. O retorno reúne indicadores e registros históricos relacionados a
**pessoas politicamente expostas (PEP), sanções e restrições nacionais e
internacionais**, permitindo análises de conformidade em escala, **sem a
necessidade de consultas individuais por entidade**".

**Por que muda a estratégia de consulta**: o desenho em
`design/02-design-kyb-kyc.md` previa verificações separadas para beneficiário
final (`CHK-OWN-002`) e para cada lista restritiva (`CHK-LST-001` a
`CHK-LST-003`), o que sugeria N consultas por cadeia societária. Um dataset que
resolve o grupo econômico inteiro por R$ 0,20 reduz tanto o custo quanto a
complexidade de orquestração.

**A verificar antes de adotar**: qual o critério de corte da cadeia societária;
com que frequência as listas são atualizadas; qual a cobertura das listas
internacionais. Estão nas páginas de "Origem da informação" e "Fontes detalhadas
e frequência de atualização" de cada grupo — não lidas nesta apuração.

### 2.4 Nota fiscal — resposta parcial a uma lacuna

A API On-Demand tem grupo **"Consultas de Notas Fiscais"** **[DOC]**. Isso
endereça a pergunta C2 do resumo executivo e a lacuna X4 de
`design/15-design-fraude-risco.md` — a viabilidade de verificar documento fiscal
por terceiro.

**[ABERTO]** — o que exatamente o grupo retorna (autorização na SEFAZ? XML
completo? apenas status?) e sob que condições de acesso. É consulta de alta
prioridade.

### 2.5 Mecanismos de integração

| Mecanismo | Definição **[DOC]** |
| --- | --- |
| **Autenticação** | Token JWT no campo `AccessToken` ou no cabeçalho da requisição. Há suporte a configuração OAuth (`OAuthBasicToken`, `OAuthPublicKey`/`OAuthPrivateKey`, `OAuthURLTarget`) e a chave privada SFTP |
| **Consulta síncrona** | Chamada direta às APIs por dataset |
| **Chamadas assíncronas (hooks)** | Retorna `QueryId` imediatamente; o resultado é enviado por POST à URL informada no parâmetro `hooks`. Exige os campos `api` e `hooks`. O modelo de resposta é idêntico ao síncrono |
| **Batch** | Envio de arquivo com lote de registros; retorna `JobId`. O arquivo entra por link público HTTP/HTTPS ou por bucket S3 com credenciais que permitam ler e gravar |
| **Monitoramento** | Assinatura por dataset e campo: `basic_data.listen(TaxIdStatus)`, `kyc.listen(IsCurrentlySanctioned)`. Alertas por e-mail, webhook ou SFTP. Identificado por `SubscriptionId` |
| **API de Preços** | Retorna a tabela completa do cliente ou o preço estimado de uma consulta, informando API e datasets. **Sem custo** |

### 2.6 Como isso se encaixa no desenho

| Mecanismo | Onde se encaixa |
| --- | --- |
| Chamadas assíncronas com `QueryId` | Mesmo padrão de correlação de `design/04-design-receivables-gateway.md`, 7.2 — `QueryId` é a referência externa, e o callback exige verificação de origem (`design/94-seguranca-threat-model.md`, T1) |
| Monitoramento por campo | Implementa diretamente os gatilhos de revalidação de `design/02-design-kyb-kyc.md`, seção 6 — `kyc.listen(IsCurrentlySanctioned)` é o gatilho "inclusão em lista restritiva" |
| API de Preços gratuita | Permite calcular o custo por operação **antes** de consultar, alimentando a métrica de margem de `design/18-design-billing.md`, seção 7 |
| Batch | Útil para revalidação periódica da carteira, não para o fluxo de onboarding |

**A API de Monitoramento merece destaque**: `design/02-design-kyb-kyc.md`
propunha revalidação por reconsulta periódica — caro e com atraso. Assinatura por
evento é mais barata e mais rápida. O endpoint de teste é gratuito, com limite de
500 requisições por mês.

---

## 3. Serpro

Fonte governamental. Documentação em `apicenter.estaleiro.serpro.gov.br` **[DOC]**.

| Item | Definição |
| --- | --- |
| **API Consulta CNPJ** | Serviço HTTP REST com operação de consulta às informações cadastrais de pessoas jurídicas no Brasil |
| **Autenticação** | Bearer token com **1 hora** de validade. Recomenda-se gerar um token por hora e usá-lo até o gateway retornar HTTP 401 |
| **Contratação** | Aberta a empresas de qualquer porte, entidades de classe, grupos econômicos e instituições públicas |

**Posição no desenho**: fonte primária de dado cadastral, com a autoridade que um
bureau agregador não tem. Serve como fonte de maior precedência na regra de
`design/02-design-kyb-kyc.md`, seção 5 (`PrecedenceRule`) — divergência entre
Serpro e agregador é divergência material, não ruído.

**[ABERTO]** — outras APIs do catálogo Serpro relevantes (validação de dados de
pessoa, por exemplo) não foram levantadas. O catálogo lista uma API chamada
"AntecipaGov", cuja página retornou erro 500; o nome sugere relação com
antecipação de recebíveis de contratos públicos e merece verificação.

---

## 4. Serasa Experian

**[SITE]** — `serasaexperian.com.br/solucoes/serasa-lista-pep/`.

| Item | Definição |
| --- | --- |
| **Modalidades** | Consulta individual e download de lista |
| **Canais** | API, batch e lote, para todas as funcionalidades |
| **Cobrança** | Por consumo, sem pacote obrigatório |
| **Funcionalidades** | Verificação de Dados, Retorno de Dados, PEP, PEM, Restrições e Antecedentes |
| **Download de lista PEP** | 4 períodos disponíveis; em cada um, 3 arquivos (Titular, Parentes e Relacionados); 2 downloads por período |
| **Nota** | Declara unificação de dados com a ClearSale |

**A opção de download da lista completa é arquiteturalmente distinta da consulta
por CPF/CNPJ**: permite verificação local, sem chamada externa por cadastro — mais
rápida e mais barata em volume, ao custo de manter a lista atualizada e de assumir
a responsabilidade por essa atualização. A decisão entre os dois modelos afeta
latência, custo e exposição a indisponibilidade de terceiro.

---

## 5. Equifax BoaVista

**[SITE]** — `boavistaservicos.com.br/planos/consulta-cadastral/`.

| Item | Definição |
| --- | --- |
| **Consulta de CNPJ retorna** | Dados cadastrais e restritivos; participações e quadro societário; score e recomendação de crédito; indicadores positivos de crédito obtido, pontualidade e comprometimento futuro |
| **Acesso** | Portal com login e senha, mediante plano contratado; consulta por CPF ou CNPJ |

**[ABERTO]** — a página descreve acesso por portal. Existência e condições de
API não foram apuradas. Para o volume previsto, acesso por portal é inviável.

---

## 6. idwall e CAF

**[DOC]** — documentação pública identificada, conteúdo não aprofundado.

| Provedor | O que se sabe |
| --- | --- |
| **idwall** | `docs.idwall.co`, com índice `llms.txt`. Estrutura organizada em **Relatórios** e **Matrizes**, com API documentada. O conceito de matriz sugere motor de regras configurável sobre os relatórios |
| **CAF** | `docs.caf.io`, portal de desenvolvedor. Estrutura não aprofundada |

Ambos atuam em verificação de identidade e onboarding — cobrem `CHK-REP-001`
(identidade do representante), que os bureaus de dados cadastrais não resolvem
sozinhos.

---

## 7. A lacuna que permanece aberta

**Validação de titularidade de conta bancária** — `CHK-BNK-001`.

Nenhum dos provedores levantados oferece explicitamente este serviço na
documentação consultada. É o controle antifraude de maior valor do sistema
(`design/94-seguranca-threat-model.md`, C2.3, e `design/15-design-fraude-risco.md`,
X5): sem ele, o vetor de maior perda esperada — desvio do destino do desembolso —
fica sem verificação central.

Caminhos a investigar:

| Caminho | Observação |
| --- | --- |
| API de Validações da BigDataCorp | Escopo não detalhado nesta apuração; verificar primeiro |
| Open Finance | Compartilhamento de dados com consentimento; verificar se cobre titularidade de conta de pessoa jurídica |
| Confirmação por micro-depósito | Independe de terceiro; lento, mas sob controle próprio |
| Provedores especializados em antifraude | Não levantados |

**Recomendação**: tratar como item de investigação dedicada, não como subitem da
escolha de bureau. O desenho já prevê a alternativa do micro-depósito
(`design/10-design-funding.md`, seção 3, camada 3) — mais lenta, porém viável sem
depender de provedor.

---

## 8. Arquitetura de consulta sugerida

Proposta, à luz do que foi apurado:

```text
ONBOARDING (uma vez por cedente)
  ├─ Serpro Consulta CNPJ         -> dado cadastral autoritativo
  ├─ economic_group_kyc (BDC)     -> grupo econômico, PEP, sanções, restrições
  ├─ ondemand_legal_representative -> representantes legais
  └─ verificação de identidade     -> idwall ou CAF, para o representante

CONTÍNUO (sem custo por operação)
  └─ Monitoramento BDC por campo   -> TaxIdStatus, IsCurrentlySanctioned

POR OPERAÇÃO (quando aplicável)
  ├─ Consultas de Notas Fiscais    -> se a verificação de lastro for obrigação
  └─ validação de conta bancária   -> [PROVEDOR NÃO IDENTIFICADO]

SOB DEMANDA DO FINANCIADOR
  └─ Score e flags de crédito      -> repassado, nunca usado em decisão própria
```

**Princípio a preservar**: score de crédito é informação repassada ao
financiador, nunca fundamento de decisão da plataforma — o limite de papel de
`design/02-design-kyb-kyc.md`, seção 2. O custo de R$ 2,41 por consulta de score
reforça que ele só deve ser consultado quando o financiador o exigir, e faturado
a quem o pediu.

---

## 9. Impacto no custo por operação

`design/18-design-billing.md`, seção 7 registra o custo de consultas cadastrais
como vetor desconhecido. Com os preços públicos, é possível estimar a ordem de
grandeza do onboarding:

| Consulta | Custo na menor faixa |
| --- | --- |
| Consulta CNPJ (Serpro) | **[ABERTO]** |
| `economic_group_kyc` | R$ 0,200 |
| `ondemand_legal_representative_company` | R$ 0,040 |
| Verificação de identidade do representante | **[ABERTO]** |

**Observação de dimensionamento**: os itens conhecidos somam centavos por
cedente. Se a verificação de identidade e a validação de conta bancária ficarem
na mesma ordem, o custo de onboarding não é o vetor dominante — o custo recorrente
de consulta às registradoras provavelmente é. Isso inverte a prioridade de
controle de custo assumida em `design/93-requisitos-nao-funcionais.md`, seção 11,
e precisa ser confirmado quando os preços das registradoras forem conhecidos.

---

## 10. Próximos passos

| # | Ação | Por quê |
| --- | --- | --- |
| 1 | Ler a documentação da **API de Validações** da BigDataCorp | Pode conter validação de conta bancária — a lacuna crítica |
| 2 | Ler **Consultas de Notas Fiscais** (On-Demand) | Define a viabilidade da verificação de lastro |
| 3 | Ler "Origem da informação" e "Fontes detalhadas" de `economic_group_kyc` | Critério de corte da cadeia e frequência de atualização das listas |
| 4 | Obter preço da API Consulta CNPJ do Serpro | Completa a estimativa de onboarding |
| 5 | Verificar a API "AntecipaGov" do Serpro | O nome sugere relação direta com o domínio |
| 6 | Avaliar consulta versus download de lista PEP | Decisão de arquitetura com impacto em latência e custo |
| 7 | Aprofundar idwall e CAF | Verificação de identidade do representante |

---

## Quem depende deste documento

| Documento | O que espera daqui | Situação |
| --- | --- | --- |
| `design/02-design-kyb-kyc.md` | Fontes para o catálogo de verificações (Y11) | **[PARCIAL]** — maior parte mapeada |
| `design/15-design-fraude-risco.md` | Verificação de documento fiscal (X4) | **[PARCIAL]** — grupo existe, escopo aberto |
| `design/10-design-funding.md` | Validação de titularidade de conta (D7) | **[ABERTO]** — lacuna crítica |
| `design/94-seguranca-threat-model.md` | Mesmo controle (S1) | **[ABERTO]** |
| `design/18-design-billing.md` | Custos unitários (G5) | **[PARCIAL]** — preços de referência obtidos |
