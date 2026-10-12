# Assinatura Eletrônica

> **Camada**: referência
> **Situação**: **parcialmente apurado**. Os níveis legais estão confirmados; o
> nível exigível para cessão de crédito continua aberto.
> **Atualizado em**: 18 de setembro de 2026

## Como ler este documento

| Marca | Significado |
| --- | --- |
| **[LEI]** | Texto legal lido |
| **[DOC]** | Documentação técnica do provedor |
| **[ABERTO]** | Não apurado |

---

## 1. Os três níveis

**Lei nº 14.063, de 23 de setembro de 2020**, art. 4º — **[LEI]**, lida em
`www2.camara.leg.br/legin/fed/lei/2020/lei-14063-23-setembro-2020-790659-normaatualizada-pl.html`,
consultada em 18/09/2026.

| Nível | Definição |
| --- | --- |
| **Simples** | Identifica o signatário e anexa ou associa dados a outros dados em formato eletrônico do signatário |
| **Avançada** | Utiliza certificados não emitidos pela ICP-Brasil ou outro meio de comprovação de autoria e integridade em forma eletrônica, desde que admitido pelas partes como válido ou aceito pela pessoa a quem for oposto o documento, com três requisitos: **(a)** está associada ao signatário de maneira unívoca; **(b)** utiliza dados para criação de assinatura cujo signatário pode, **com elevado nível de confiança, operar sob seu controle exclusivo**; **(c)** está relacionada aos dados a ela associados de tal modo que **qualquer modificação posterior é detectável** |
| **Qualificada** | Utiliza certificado digital, nos termos do § 1º do art. 10 da **Medida Provisória nº 2.200-2, de 24 de agosto de 2001** |

**Art. 4º, § 1º** — **[LEI]**: os três tipos "caracterizam o nível de confiança
sobre a identidade e a manifestação de vontade de seu titular, e a assinatura
eletrônica qualificada é a que possui nível mais elevado de confiabilidade a
partir de suas normas, de seus padrões e de seus procedimentos específicos".

**Art. 4º, § 2º** — **[LEI]**: "devem ser asseguradas formas de revogação ou de
cancelamento definitivo do meio utilizado para as assinaturas previstas nesta
Lei, sobretudo em casos de comprometimento de sua segurança ou de vazamento de
dados".

**Requisito derivado**: o desenho precisa acomodar revogação do meio de
assinatura — não apenas do documento. Isso não está em
`design/08-design-contracting.md`, seção 5.

## 2. Ressalva importante sobre o alcance da Lei 14.063

**A Lei 14.063 disciplina primordialmente as interações com entes públicos.**
Seu Capítulo II trata de: interação interna da administração; interação entre
particulares e entes públicos; e interação entre entes públicos (art. 2º)
**[LEI]**.

O art. 5º determina que cada ente público estabeleça o nível mínimo exigido em
suas interações; o art. 8º impõe que assinaturas qualificadas em atas
deliberativas de pessoas jurídicas de direito privado sejam aceitas pela
administração pública **[LEI]**.

**Para contrato entre particulares — que é o caso da cessão de crédito entre
cedente e financiador — a Lei 14.063 fornece as definições de nível, mas não
determina o nível exigível.** O regime aplicável é o da MP 2.200-2/2001, cujo
art. 10, § 2º admite documento eletrônico com outros meios de comprovação de
autoria e integridade **desde que admitido pelas partes como válido ou aceito
pela pessoa a quem for oposto**.

**[ABERTO]** — o texto da MP 2.200-2 não foi lido nesta apuração. A citação
acima reflete o que a Lei 14.063 refere; conferir o original.

**Consequência prática**: a escolha do nível para a cessão de crédito é
**decisão de risco probatório**, não de conformidade com uma norma que a imponha.
Por isso `design/08-design-contracting.md`, seção 5.1 acertou ao deixar o nível
como atributo configurável por produto — a exigência virá do financiador e da
infraestrutura de registro, não de um único comando legal.

**Ainda [ABERTO]**: se a infraestrutura de registro (registradora ou
escriturador) exigir nível específico para aceitar o comando de constituição de
efeito, essa exigência prevalece na prática. É pergunta a fazer às registradoras.

## 3. ICP-Brasil

**[DOC]** — o Instituto Nacional de Tecnologia da Informação (ITI) mantém as
páginas institucionais da ICP-Brasil e de certificado digital em `gov.br/iti`.
Conteúdo não aprofundado nesta apuração.

**[ABERTO]** — lista de autoridades certificadoras credenciadas; custo e prazo de
emissão de certificado; viabilidade de exigir certificado do representante do
cedente sem inviabilizar o onboarding.

---

## 4. Provedores

### 4.1 Clicksign

Documentação em `developers.clicksign.com`, com índice `llms.txt` e
especificação OpenAPI **[DOC]**.

**API 3.0, orientada a Envelope** (há migração documentada a partir da 1.9):

| Recurso | Operações |
| --- | --- |
| **Envelope** | Criar, listar, ativar/atualizar, detalhar, excluir (apenas em rascunho) |
| **Documentos** | Criar por **upload em base64**, criar por modelo/template, duplicar, editar (cancelar ou finalizar), detalhar, excluir (apenas em rascunho) |
| **Signatários** | Criar, listar, detalhar, excluir (apenas se não iniciou a assinatura) |
| **Requisitos** | Criar, excluir (apenas se não iniciou o processo) |
| **Observadores** | Criar, listar, excluir |
| **Notificações** | Notificar um signatário; notificar todos os do envelope |
| **Eventos** | Listar eventos de documento e de envelope; criar evento customizado; criar evento com imagem |
| **Webhooks** | CRUD completo |

**Regra relevante** **[DOC]**: "para ativar um envelope na plataforma, **cada
signatário deve possuir pelo menos um critério de autenticação configurado**".

**Aderência ao desenho**: o modelo Envelope → Documentos → Signatários →
Requisitos corresponde diretamente ao `SignatureEnvelope` de
`design/90-modelo-de-dados.md`. O conceito de **Requisito** é o que materializa a
exigência de autenticação por signatário — mecanismo para impor o nível de
assinatura por produto (`design/08-design-contracting.md`, 5.1).

**Eventos customizados** são úteis para vincular ao envelope o aceite das
condições materiais (`design/08-design-contracting.md`, seção 3), mantendo os
dois atos no mesmo dossiê sem confundi-los.

### 4.2 ZapSign

Documentação em `docs.zapsign.com.br` **[DOC]**.

| Área | O que a documentação cobre |
| --- | --- |
| **Autenticação** | Token estático ou JWT; documentação sobre tipos de token e onde localizá-los |
| **Operação** | Ambiente de testes; versionamento da API; **políticas de rate limit**; alerta de incidentes; status de erros |
| **Documentos** | Criar por upload; **OneClick (ClickWrap)**; criar por modelo; anexos; detalhar; atualizar; listar; excluir; **histórico de atividades**; posicionar assinaturas; **validação de assinaturas**; cancelar |
| **Signatários** | Detalhar, atualizar, adicionar, excluir; **resetar tentativas de validação**; **grupo de signatários**; **consultar validações**; **assinar em lote via API** |
| **Conta** | Informações do plano; gestão de usuários |

**Pontos de interesse para o desenho**:

- **rate limit e alerta de incidentes documentados** — dois itens que o
  `CapabilityProfile` de `design/04-design-receivables-gateway.md` exige e que
  raramente aparecem em documentação comercial;
- **assinar em lote** — relevante se houver operações com muitos instrumentos;
- **grupo de signatários** — endereça o caso de dupla aprovação;
- **resetar tentativas de validação** — operação de backoffice que o desenho não
  previu (`design/17-design-backoffice.md`).

### 4.3 Autentique

Documentação em `docs.autentique.com.br` **[DOC]**, em plataforma GitBook.
Conteúdo não aprofundado.

### 4.4 DocuSign

Documentação da eSign REST API em `developers.docusign.com` **[DOC]**. Conteúdo
não aprofundado. Provedor internacional — verificar aderência ao regime
brasileiro e à ICP-Brasil.

### 4.5 D4Sign

**[ABERTO]** — `developers.d4sign.com.br` não resolve. Endereço correto da
documentação não identificado.

---

## 5. Critérios de escolha

Derivados de `design/08-design-contracting.md`, seção 5.2 (evidências mínimas) e
`design/94-seguranca-threat-model.md`, T3.

| # | Critério | Por quê |
| --- | --- | --- |
| 1 | Suporta o nível que o financiador e a infraestrutura exigirem | Sem isso, o instrumento não é aceito |
| 2 | Preserva hash do documento apresentado **e** do assinado | Invariante de `08-prd-contracting.md` |
| 3 | Permite critério de autenticação por signatário | Impõe nível diferente por papel |
| 4 | Fornece trilha de auditoria exportável | Compõe o dossiê |
| 5 | Suporta ordem de assinatura configurável | `design/08-design-contracting.md`, 5.3 |
| 6 | Webhook com verificação de origem | Controle T1 |
| 7 | Rate limit e incidentes documentados | `CapabilityProfile` |
| 8 | Registro de tempo confiável | **[ABERTO]** — exigibilidade a confirmar |
| 9 | Expiração de envelope configurável | Envelope expirado não invalida o contrato |

**Critério 2 merece atenção na avaliação**: o desenho exige preservar o documento
**apresentado** e o **assinado**, com hashes distintos. Nem todo provedor expõe o
hash do documento antes da assinatura — é pergunta específica a fazer.

---

## 6. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| 1 | Nível exigível para cessão de crédito e cessão fiduciária | **Alto** — decisão jurídica |
| 2 | Exigência da infraestrutura de registro quanto ao nível | **Alto** — pergunta às registradoras |
| 3 | Texto da MP 2.200-2 não lido | Médio |
| 4 | Preços dos provedores | Médio — afeta custo por operação |
| 5 | Provedor expõe hash do documento pré-assinatura? | Médio |
| 6 | Exigibilidade de registro de tempo confiável | Médio |
| 7 | Autentique, DocuSign e D4Sign não aprofundados | Baixo |

---

## Quem depende deste documento

| Documento | O que espera daqui | Situação |
| --- | --- | --- |
| `design/08-design-contracting.md` | Nível exigido (T9); provedor (T12) | **[PARCIAL]** — níveis definidos, exigibilidade aberta |
| `design/90-modelo-de-dados.md` | Estrutura de `SignatureEnvelope` | **[CONFIRMADO]** — modelo do Clicksign valida o desenho |
| `design/18-design-billing.md` | Custo por envelope (G5) | **[ABERTO]** |
