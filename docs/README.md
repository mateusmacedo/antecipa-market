# Documentação — Plataforma Multiativo de Antecipação de Recebíveis

## Como esta documentação está organizada

Três camadas, com propósitos distintos e ciclos de vida distintos.

```text
docs/
├── resumo-executivo-apuracao.md    O QUE FALTA para começar a construir
├── prd/          O QUE construir e POR QUÊ  — camada conceitual (preservada)
├── design/       COMO construir              — camada construtiva
└── reference/    O QUE PRECISA SER APURADO   — camada de referência
```

**Começando por aqui**: [`resumo-executivo-apuracao.md`](./resumo-executivo-apuracao.md)
consolida as 111 pendências dos 21 documentos, deduplicadas e agrupadas por quem
responde — jurídico, registradoras, fornecedores e decisão interna. Abre com uma
página para decisão e segue com o detalhamento por interlocutor.

| Camada | Pergunta que responde | Muda quando |
| --- | --- | --- |
| `prd/` | Que contextos existem, quais suas invariantes | A intenção do produto muda |
| `design/` | Que estruturas, contratos e procedimentos concretizam isso | A implementação evolui |
| `reference/` | Que fatos externos condicionam as decisões | A apuração avança |

**Os PRDs não foram alterados.** Cada documento de design referencia o PRD de
origem e declara, numa tabela inicial, exatamente qual lacuna daquele PRD ele
fecha. A tabela final de cada documento faz o caminho inverso — requisito por
requisito.

---

## Camada conceitual — `prd/`

| Documento | Contexto |
| --- | --- |
| `00-prd-mapa-de-produto.md` | Visão, objetivos, jornada, regras transversais |
| `01-prd-identity-organization.md` | Identidade, organizações, representação |
| `02-prd-kyb-kyc.md` | Verificação cadastral |
| `03-prd-consent-mandates.md` | Consentimentos e mandatos |
| `04-prd-receivables-gateway.md` | Integração com infraestruturas |
| `05-prd-portfolio.md` | Posição dos recebíveis |
| `06-prd-eligibility.md` | Regras de composição de lote |
| `07-prd-marketplace.md` | Pedidos e ofertas |
| `08-prd-contracting.md` | Formalização |
| `09-prd-effects-orchestrator.md` | Efeitos regulatórios |
| `10-prd-funding.md` | Desembolso |
| `11-prd-settlement-reconciliation.md` | Liquidação e conciliação |
| `12-prd-compliance-ledger.md` | Evidências |
| `13-prd-partner-management.md` | Parceiros |
| `14-catalogo-contratos-eventos.md` | Integração entre contextos |

---

## Camada construtiva — `design/`

### Transversais

| Documento | Conteúdo |
| --- | --- |
| `00-arquitetura-tecnica.md` | Estilo, multi-tenancy, persistência, mensageria, integração externa, tempo, observabilidade, decisões abertas |
| `90-modelo-de-dados.md` | Convenções, tipos compartilhados, agregados com atributos, classificação de dados |
| `91-contratos-api.md` | Quatro superfícies, versionamento, idempotência, erros, rotas detalhadas |
| `92-schemas-eventos.md` | Envelope estendido, schemas por contexto, catálogo completo, governança |
| `93-requisitos-nao-funcionais.md` | Volumetria, latência, disponibilidade, recuperação, retenção, custo |
| `94-seguranca-threat-model.md` | Fronteiras de confiança, dez ameaças com controles, resposta a incidente |
| `95-personas-jornadas.md` | Atores, personas, jornadas, matriz de permissões |

### Por contexto — correspondem a PRDs existentes

| Documento | Lacunas principais que fecha |
| --- | --- |
| `01-design-identity-organization.md` | Revogação em 60 s, validação de identidade, permissões temporais, dupla aprovação, matriz e filial |
| `02-design-kyb-kyc.md` | Catálogo de verificações, cálculo de completude, divergências, revalidação, níveis de compartilhamento |
| `03-design-consent-mandates.md` | Aceite forte, interseção de escopos, propagação de revogação, renovação, estrutura do instrumento |
| `04-design-receivables-gateway.md` | `CapabilityProfile`, portas com operações, taxonomia de erros, detecção de quebra, reprocessamento |
| `05-design-portfolio.md` | Alteração material, política de disponibilidade, frescor, comparação de snapshots, divergências |
| `06-design-eligibility.md` | Regras em código versionado, catálogo normativo, justificativa legível, reserva, invalidação |
| `07-design-marketplace.md` | Comparabilidade, ordenação, patrocínio, distribuição, contraproposta |
| `08-design-contracting.md` | Condições materiais, tolerância de divergência, assinatura, alçada, dossiê |
| `09-design-effects-orchestrator.md` | Sequência de submissão, mapeamento de efeito, confirmação parcial, recebíveis futuros, disputas |
| `10-design-funding.md` | Validação de destino, mudança de conta, efeito parcial, comprovante, estorno |
| `11-design-settlement-reconciliation.md` | Quatro estados de liquidação, correspondência em camadas, rateio, alçada, liberação, chargeback |
| `12-design-compliance-ledger.md` | Imutabilidade em camadas, detecção de lacuna, reconstrução, dossiê, retenção |
| `13-design-partner-management.md` | Critérios de homologação, regra comercial, capacidade efetiva, suspensão, rollout, saída |

### Contextos novos — sem PRD correspondente

| Documento | Por que existe |
| --- | --- |
| `15-design-fraude-risco.md` | Nenhum PRD trata de fraude; a plataforma constitui efeitos irreversíveis e coordena desembolsos |
| `16-design-notificacoes.md` | Nenhum PRD trata de comunicação; estados de exceção deixam o cliente sem informação |
| `17-design-backoffice.md` | Sete PRDs produzem trabalho operacional sem destino definido |
| `18-design-billing.md` | `CommercialRule` existe sem modelo de receita, fato gerador ou tributação |

---

## Camada de referência — `reference/`

| Documento | Conteúdo | Situação |
| --- | --- | --- |
| `glossario.md` | Termos de domínio e de arquitetura | Completo, com itens marcados **[A CONFIRMAR]** |
| `roteiro-apuracao-regulatoria.md` | 20 perguntas com impacto construtivo, priorizadas | Completo — são perguntas, não respostas |
| `regulacao-brasil.md` | Apuração normativa | **Parcial** — registro de recebíveis de cartão e duplicata escritural com base confirmada |
| `provedores-integracao.md` | Registradoras e escrituradores | **Parcial** — caminho de integração mapeado em nível normativo |
| `bureaus-dados.md` | Bureaus de dados cadastrais e de risco | **Parcial** — um provedor mapeado em profundidade, com preços |
| `assinatura-eletronica.md` | Níveis legais e provedores de assinatura | **Parcial** — níveis confirmados, exigibilidade aberta |
| `fraude-tipologias-controles.md` | Parte empírica da análise de fraude | **Vazio** — a parte analítica está em `design/15` |

**Limitação declarada desta versão**: a apuração regulatória e o mapeamento de
provedores não puderam ser concluídos. O orçamento de busca da sessão esgotou;
`planalto.gov.br` e `legislacao.presidencia.gov.br` recusaram conexão direta; as
páginas do Banco Central são renderizadas por JavaScript, sem conteúdo legível, e
o acesso ao dado estruturado é bloqueado; a documentação técnica das registradoras
não é pública.

Em vez de preencher os documentos com números não verificados — que produziriam
decisão de arquitetura errada com aparência de fundamentada — a apuração foi
convertida em `roteiro-apuracao-regulatoria.md`: 20 perguntas formuladas de modo
verificável, priorizadas por impacto, cada uma indicando o que bloqueia e onde
buscar. É o documento que se leva ao jurídico e às registradoras.

---

## Lacunas bloqueantes

Consolidado do que impede o início da construção, em ordem de impacto.

| # | Lacuna | Onde está registrada | Depende de |
| --- | --- | --- | --- |
| 1 | Autorização regulatória da plataforma | Roteiro, 3.1 | Jurídico |
| 2 | Tipos e códigos de efeito por infraestrutura | `09-design`, 4 | Apuração + registradoras |
| 3 | Esquema de identidade da unidade de recebível | `05-design`, 5 | Apuração + registradoras |
| 4 | Campos canônicos dos ativos | `90-modelo-de-dados`, 5 (L1, L2) | Apuração |
| 5 | Nenhum provedor mapeado | `04-design`, 8 | Contato comercial |
| 6 | Mecânica e prazos do opt-in | Roteiro, 1.4 | Apuração |
| 7 | Modelo de monetização | `18-design`, 2 (G1-G3) | Decisão comercial |
| 8 | Prazos de retenção e fundamento | `93-requisitos`, 6 | Jurídico |
| 9 | Nível de assinatura exigido | `08-design`, 5.1 | Jurídico |
| 10 | Janela de contestação por arranjo | `11-design`, 7 | Apuração |
| 11 | Validação de titularidade de conta | `94-seguranca`, S1 | Pesquisa de provedor |
| 12 | Volumetria real | `93-requisitos`, 2 | Dado de negócio |

**As lacunas 2 a 6 e 10 têm um caminho comum**: contato direto com registradoras
e escrituradores. Parte do material que bloqueia o desenho não é público — manuais
de integração e a convenção entre registradoras costumam exigir acordo comercial.
Esse contato deveria ocorrer cedo.

---

## Decisões abertas

Registradas em `00-arquitetura-tecnica.md`, seção 12. Cada uma merece um registro
de decisão antes do início da construção.

| # | Decisão | Impacto se adiada |
| --- | --- | --- |
| D1 | Monolito modular ou microsserviços | Alto |
| D2 | Estratégia de multi-tenancy | Alto — retrofit é caro |
| D3 | Banco primário | Alto |
| D6 | Definição de alteração material | Alto — define custo de armazenamento |
| D8 | Linguagem e plataforma | Alto — sem indicação nos PRDs |

Além dessas, decisões de produto registradas nos documentos de design:

| Decisão | Onde |
| --- | --- |
| Patrocínio pode inverter mérito no ranking? | `07-design-marketplace`, 3.3, R6 |
| Sacado entra no escopo? | `95-personas-jornadas`, A10 |
| Caminho padrão para confirmação parcial | `09-design-effects-orchestrator`, 5.2 |
| Quem produz o instrumento contratual | `08-design-contracting`, 2 |
| Regime de aprovação única para organizações pequenas | `01-design-identity-organization`, 5 |

---

## Convenções desta documentação

| Marca | Significado |
| --- | --- |
| **[ABERTA]** | Decisão técnica não tomada |
| **[A CONFIRMAR]** | Fato externo não verificado |
| **Bloqueante** | Impede construção do que depende |
| Tabela "Lacunas do PRD que este documento fecha" | Abre cada documento de design |
| Tabela "Rastreabilidade" | Encerra cada documento; mapeia seção a requisito |

**Sobre o tom**: os documentos declaram limites em vez de os omitir. Onde uma
proposta tem custo, o custo está escrito. Onde um controle não protege contra
determinado cenário, isso está dito — um mecanismo apresentado como garantia
absoluta gera confiança indevida, que é pior que a ausência do mecanismo.
