# Design — Identity & Organization

> **PRD de origem**: `docs/prd/01-prd-identity-organization.md`
> **Camada**: construtiva
> **Status**: proposta
> **Complementa**: `90-modelo-de-dados.md` 3.1 (atributos),
> `95-personas-jornadas.md` (papéis e matriz de permissões),
> `00-arquitetura-tecnica.md` 4 (multi-tenancy). Não repetidos aqui.

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| I1 | Protocolo de autenticação não especificado | "Autenticação federada" |
| I2 | "Validação de identidade" no convite sem definição | Req. 3 |
| I3 | Alçada de representação ausente | Req. 4 |
| I4 | "Visão efetiva das permissões em determinado instante" sem mecanismo | Req. 8 |
| I5 | Dupla aprovação sem padrão nem configuração | Req. 6 |
| I6 | "Bloquear imediatamente" sem definição de imediato | Req. 7 |
| I7 | Relação matriz-filial-estabelecimento recebedor sem semântica | Escopo |

## 2. Autenticação

### 2.1 Decisão: OIDC com provedor de identidade externo **[ABERTA]**

| Requisito | Definição |
| --- | --- |
| Protocolo | OIDC, com código de autorização e PKCE |
| MFA | Obrigatório para `APPROVER`, `ORG_ADMIN` e todos os papéis de plataforma |
| Fatores aceitos | Aplicativo de senha temporária ou chave de segurança; SMS apenas como recuperação |
| Sessão | 30 min de inatividade para papéis de aprovação; 8 h para consulta |
| Reautenticação | Exigida em ato crítico, mesmo com sessão válida |
| Federação | Suportada para organizações que a exijam, por organização |

**Atos críticos que exigem reautenticação**: conceder ou revogar consentimento,
assinar contrato, selecionar oferta, alterar dados bancários, registrar ou revogar
representação, convidar usuário com papel de aprovação.

### 2.2 Revogação em menos de 60 segundos

Req. 7 exige bloqueio imediato; `93-requisitos-nao-funcionais.md`, 7 estabelece
60 s. Token de acesso com validade de 15 minutos não atende.

| Opção | Custo | Adequação |
| --- | --- | --- |
| Token de vida muito curta com renovação frequente | Carga no provedor de identidade | Boa |
| Lista de revogação consultada por requisição | Latência e dependência adicional | Boa, com cache curto |
| Introspecção por requisição | Latência alta | Ruim |

**Recomendação**: token curto (5 min) com renovação, mais lista de revogação com
cache de 30 s consultada nas operações sensíveis. Consulta em toda requisição é
desperdício; em nenhuma é vulnerabilidade.

## 3. Validação de identidade no convite

Req. 3 exige vincular usuário "somente após convite e validação de identidade",
sem definir a validação.

| Etapa | Requisito |
| --- | --- |
| Convite | Emitido por `ORG_ADMIN`, para endereço específico, com expiração |
| Posse do canal | Confirmação do endereço |
| Identidade da pessoa | Verificação proporcional ao papel — ver abaixo |
| Vínculo | `Membership` criada apenas após ambas |

| Papel | Nível de verificação |
| --- | --- |
| `AUDITOR` | Posse do canal |
| `OPERATOR` | Posse do canal + dados pessoais conferidos com fonte |
| `APPROVER` | Verificação documental e de vivacidade |
| `ORG_ADMIN` | Verificação documental e de vivacidade |

**Posse de e-mail não é identidade.** Quem controla a caixa de e-mail de um
convite não é necessariamente a pessoa convidada — e para papéis que assinam
contratos, essa diferença é a fronteira entre operação válida e inválida.

## 4. Permissões efetivas

Req. 8 exige "visão efetiva das permissões de um usuário em determinado
instante", sem mecanismo. O "em determinado instante" é a parte difícil: exige
consulta temporal, não apenas estado atual.

```text
permissõesEfetivas(usuário, organização, instante) =
    união das permissões dos papéis ativos no instante
  ∩ limites da representação vigente no instante
  - revogações com efeito anterior ao instante
```

| Requisito | Implementação |
| --- | --- |
| Consulta do estado atual | Projeção materializada, atualizada por evento |
| Consulta histórica | Reconstruída do ledger, por `AuditSubject` do usuário |
| Desempenho | Estado atual em memória; histórico assíncrono |

**Por que a consulta histórica importa**: quando se questiona um contrato, a
pergunta é "esta pessoa tinha poder naquele dia?", não "tem hoje". Sem consulta
temporal, a resposta exige reconstrução manual — exatamente o que
`12-prd-compliance-ledger.md` existe para evitar.

## 5. Dupla aprovação

Req. 6 exige "quando configurado", sem padrão.

| Ação | Padrão | Configurável |
| --- | --- | --- |
| Alteração de dados bancários | **Obrigatória, sem exceção** | Não |
| Registro ou revogação de representante | **Obrigatória, sem exceção** | Não |
| Concessão de papel de aprovação | Obrigatória | Sim, com aceitação de risco |
| Seleção de oferta | Desativada | Sim |
| Assinatura de contrato | Desativada | Sim |
| Revogação de consentimento | Desativada | Sim |

**As duas primeiras não são configuráveis** porque correspondem aos vetores de
fraude de maior perda esperada (`94-seguranca-threat-model.md`, T2 e T3).

**Organização com um único usuário**: não consegue cumprir dupla aprovação. Duas
saídas, ambas com custo:

1. exigir segundo usuário para operar — atrito no onboarding;
2. aceitar aprovação por canal alternativo verificado (confirmação por telefone
   registrado, por exemplo) como segundo fator de aprovação.

**Recomendação**: a segunda, com registro explícito de que a organização opera em
regime de aprovação única assistida, e com notificação reforçada.

## 6. Matriz, filial e estabelecimento recebedor

O escopo menciona "relação entre matriz, filiais e estabelecimentos recebedores"
sem semântica. A distinção importa porque determina de quem são os recebíveis.

| Conceito | Definição |
| --- | --- |
| `Organization` | Pessoa jurídica; titular de direitos e parte em contratos |
| `Establishment` | Unidade operacional, com identificador fiscal próprio |
| Estabelecimento recebedor | `Establishment` credenciado em arranjo de pagamento |

**Regras**:

1. O contrato é firmado pela `Organization`, não pelo `Establishment`.
2. Os recebíveis pertencem ao estabelecimento recebedor, identificado pelo seu
   próprio identificador fiscal.
3. Filial com identificador fiscal próprio pode ser `Organization` distinta, com
   representação própria — depende da estrutura jurídica.
4. O consentimento e o opt-in são por estabelecimento recebedor, não por grupo.
5. A representação que assina precisa ter poderes sobre a `Organization` titular
   dos recebíveis.

**Caso que quebra o modelo simples**: grupo econômico em que a matriz quer
antecipar recebíveis das filiais. Se as filiais são pessoas jurídicas distintas,
cada uma precisa de consentimento e representação próprios — a matriz não pode
ceder o que não é dela. Isso é limitação jurídica, não do sistema, e a interface
precisa torná-la compreensível em vez de deixá-la aparecer como erro.

## 7. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| I8 | Provedor de identidade não escolhido | Médio |
| I9 | Provedor de verificação documental não escolhido | Médio |
| I10 | Regime de aprovação única assistida (5) exige validação jurídica | Alto |
| I11 | Tratamento de grupo econômico (6) | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2.2 | 7 | "Imediatamente" sem definição |
| 3 | 3 | Validação de identidade sem definição |
| 4 | 8 | Permissões temporais sem mecanismo |
| 5 | 6 | Dupla aprovação sem padrão |
| 6 | Escopo | Matriz e filial sem semântica |
