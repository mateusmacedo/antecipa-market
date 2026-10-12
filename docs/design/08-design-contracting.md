# Design — Contracting

> **PRD de origem**: `docs/prd/08-prd-contracting.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| T1 | "Condições materiais" sem definição | Req. 8 |
| T2 | Nível de assinatura eletrônica não especificado | `SignatureEnvelope` |
| T3 | Geração vs. recepção do instrumento sem critério | Escopo |
| T4 | "Divergências" sem tolerância | Req. 2 |
| T5 | Verificação de poderes sem incluir alçada | Req. 4 |
| T6 | Cláusula de confirmação parcial ausente | — |
| T7 | Contratos sobre recebíveis futuros sem estrutura | Req. 6 |
| T8 | Ordem de assinatura sem regra | `SignatureEnvelope` |

## 2. Quem produz o instrumento

O escopo admite "geração ou recepção do instrumento contratual do financiador" sem
critério.

| Modelo | Vantagem | Custo |
| --- | --- | --- |
| **Financiador fornece** | Cada instituição usa seu jurídico; sem responsabilidade da plataforma sobre o texto | Instrumentos heterogêneos; validação de correspondência difícil; comparabilidade prejudicada |
| **Plataforma gera a partir de modelo do financiador** | Padronização; correspondência verificável campo a campo | A plataforma assume papel na produção do documento |
| **Híbrido** | Modelo do financiador com preenchimento pela plataforma, aprovado previamente | Exige homologação por produto |

**Recomendação**: híbrido. O financiador registra o modelo, a plataforma preenche
a partir de dados versionados, e o modelo é homologado em `PartnerAgreement`.

**Razão**: o requisito 2 exige "validar divergências de taxa, valor, prazo, partes
e ativos" entre oferta e instrumento. Se o instrumento chega como documento
fechado, a validação é leitura de texto — frágil e não automatizável. Com
preenchimento a partir de dados, a correspondência é estrutural e a divergência
deixa de ser possível por construção.

**Contrapartida**: a plataforma passa a ter papel na produção do documento, com
implicação de responsabilidade que precisa ser tratada contratualmente com os
financiadores. Decisão jurídica, não técnica.

## 3. Condições materiais

Req. 8 exige aceite explícito sem definir o que é material. Definição proposta
(alinhada a `95-personas-jornadas.md`, P2):

| # | Condição |
| --- | --- |
| 1 | Valor bruto cedido |
| 2 | Valor líquido a receber |
| 3 | Custo total em reais |
| 4 | Taxa efetiva mensal |
| 5 | Prazo médio ponderado |
| 6 | Data prevista de crédito |
| 7 | Identificação do financiador |
| 8 | Natureza da operação (cessão definitiva, com coobrigação, garantia) |
| 9 | Existência de direito de regresso |
| 10 | Quantidade e valor dos ativos alcançados |
| 11 | Tipo de efeito a ser constituído |
| 12 | Tratamento em caso de confirmação parcial |
| 13 | Condições de liberação do efeito |

**Aceite das condições materiais é registrado separadamente da assinatura do
instrumento**, com hash próprio. São atos distintos: compreender as condições e
firmar o documento.

**Item 12 é novo** e decorre de `09-design-effects-orchestrator.md`, 5.2. Sem
previsão contratual, toda confirmação parcial vira caso manual.

## 4. Validação de correspondência

Req. 2 exige validar divergências sem definir tolerância.

| Campo | Tolerância | Divergência produz |
| --- | --- | --- |
| Valor bruto | Zero | `MISMATCH` bloqueante |
| Valor líquido | Zero | `MISMATCH` bloqueante |
| Taxa | Zero | `MISMATCH` bloqueante |
| Conjunto de ativos | Zero | `MISMATCH` bloqueante |
| Partes | Zero | `MISMATCH` bloqueante |
| Data prevista de crédito | Tolerância em dias úteis | Alerta |
| Prazo médio | Recalculado, não comparado | — |

**Tolerância zero para valores**: qualquer divergência de centavo entre oferta e
instrumento indica erro de cálculo ou adulteração. Tolerar produziria a pergunta
"qual é o limite aceitável de divergência num contrato?" — que não tem resposta
defensável.

**`MISMATCH` bloqueante exige nova apresentação e novo aceite** (critério de
aceitação do PRD). Não se corrige silenciosamente, nem se pede ao usuário que
"confirme mesmo assim".

## 5. Assinatura

### 5.1 Nível exigido

**Lacuna dependente de apuração jurídica** — ver `docs/reference/regulacao-brasil.md`.
A escolha entre assinatura eletrônica simples, avançada e qualificada depende do
que a legislação exige para cessão de crédito e cessão fiduciária, e do que os
financiadores aceitam.

Estrutura de decisão:

| Fator | Impacto |
| --- | --- |
| Exigência legal para o tipo de operação | Determinante |
| Exigência da infraestrutura de registro | Pode ser mais restritiva que a lei |
| Exigência do financiador | Pode ser mais restritiva ainda |
| Fricção para o signatário | Certificado qualificado reduz conversão |

**Postura recomendada até a apuração**: modelar `SignatureEnvelope` com o nível
como atributo configurável por produto, não fixo. Assim a decisão pode ser tomada
depois sem refazer o modelo.

### 5.2 Evidências mínimas

Independentemente do nível:

| Evidência | Razão |
| --- | --- |
| Documento apresentado, com hash | Prova do que foi exibido |
| Documento assinado, com hash | Prova do que foi firmado |
| Identificação do signatário e método de autenticação | Prova de quem |
| `representationId` vigente no instante | Prova do poder (`01-prd`, invariante) |
| Instante da assinatura | |
| Endereço de origem e identificação do dispositivo | Corrobora |
| Registro de tempo confiável | Prova temporal independente |
| Trilha do provedor de assinatura | Evidência do terceiro |

### 5.3 Ordem

| Regra | Definição |
| --- | --- |
| Ordem | Configurável por produto; padrão cedente antes do financiador |
| Imutabilidade | Após a primeira assinatura o instrumento não muda (req. 7); alteração cria nova versão e invalida assinaturas coletadas |
| Expiração | Envelope expira; expiração não invalida o contrato — invalida o envelope |
| Recusa | Encerra o caso com motivo |
| Verificação de poder | No instante de cada assinatura, não na criação do envelope |

**Verificação por assinatura e não por envelope**: entre a criação do envelope e a
última assinatura pode haver dias. Uma representação revogada nesse intervalo não
pode produzir assinatura válida.

### 5.4 Alçada

Req. 4 exige confirmar poderes; `90-modelo-de-dados.md`, 3.1 mostra que poder tem
limite de valor.

```text
podeAssinar =
      representação vigente no instante
  AND powerType compatível com o ato
  AND (scope.maxAmount ausente OU valor da operação <= scope.maxAmount)
  AND (scope.actTypes ausente OU tipo do ato em scope.actTypes)
```

**Falha de alçada bloqueia e informa**: "o representante X tem poderes limitados a
R$ Y; esta operação é de R$ Z". Mensagem genérica de permissão negada faria o
usuário procurar problema técnico onde há limitação jurídica.

## 6. Recebíveis futuros

Req. 6 exige suportar contratos sobre recebíveis futuros sem estrutura.

| Elemento | Definição |
| --- | --- |
| Objeto | Regra de alcance, não lista de ativos (ver `09-design-effects-orchestrator.md`, 6) |
| Valor | Teto ou estimativa, com mecanismo de apuração |
| Vigência | Período de alcance |
| Desembolso | Único antecipado, ou sucessivo conforme apuração |
| Liberação | Encerramento da regra, não de alvos |

**Impacto no `AssetSchedule`**: deixa de ser lista fechada. O modelo precisa de
`scheduleMode` (`ENUMERATED` ou `RULE_BASED`), com a regra quando aplicável.

**Complexidade real**: contrato sobre recebíveis futuros com desembolso antecipado
é operação estruturalmente distinta — o risco é de originação, não de crédito
cedido. Isso pode exigir produto, efeito e conciliação diferentes. A estratégia de
entrega de `00-prd` coloca isso na fase 4, o que é adequado.

## 7. Dossiê contratual

`ContractEvidenceBundle` deve conter, para atender ao critério "o dossiê contém
oferta, revalidação, contrato, assinaturas e hashes":

| Item | Origem |
| --- | --- |
| Pedido e versão | Marketplace |
| Apresentação de ofertas | `OffersPresented` |
| Oferta selecionada e documento original | Marketplace |
| Aceite de condições materiais | Contracting |
| Resultado da revalidação | Eligibility, Portfolio, Consent, KYB |
| Snapshot de portfólio referenciado | Portfolio |
| Representações vigentes | Identity |
| Instrumento apresentado | Contracting |
| Instrumento assinado | Provedor de assinatura |
| Evidências de assinatura | Provedor de assinatura |
| Comando de efeito e confirmação | Effects |
| Hashes de tudo acima | Ledger |

**Critério de completude**: o dossiê permite reconstruir a operação sem consultar
nenhum sistema. Referência sem conteúdo recuperável não constitui dossiê completo
(`12-prd`, invariante).

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| T9 | Nível de assinatura exigido (5.1) | **Alto** |
| T10 | Modelo de produção do instrumento (2) | Alto — exige jurídico |
| T11 | Textos contratuais | Alto |
| T12 | Provedor de assinatura não escolhido | Médio |
| T13 | Estrutura de contrato sobre recebíveis futuros (6) | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | Escopo | Produção do instrumento sem critério |
| 3 | 8 | "Condições materiais" sem definição |
| 4 | 2 | Divergências sem tolerância |
| 5.3 | 7 | Ordem e imutabilidade |
| 5.4 | 4 | Poderes sem alçada |
| 6 | 6 | Recebíveis futuros sem estrutura |
| 7 | Critério de aceitação | Dossiê sem inventário |
