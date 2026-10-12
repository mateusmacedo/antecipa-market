# Design — Notificações e Comunicação

> **Contexto novo**: não existe PRD correspondente.
> **Camada**: construtiva
> **Status**: proposta

## 1. Por que este contexto precisa existir

Nenhum dos 15 PRDs menciona comunicação com o cedente. A consequência prática é
severa e concentra-se nos estados de exceção:

> O cliente assinou um contrato de cessão dos seus recebíveis. O efeito ficou em
> `UNKNOWN_OUTCOME`. O desembolso está bloqueado. Do ponto de vista dele:
> assinou e o dinheiro não veio, sem nenhuma informação.

`09-prd-effects-orchestrator.md` trata o estado corretamente do ponto de vista do
sistema. Do ponto de vista do cliente, a ausência de comunicação transforma um
problema técnico gerenciável em ruptura de confiança.

Há também um vetor de segurança: notificação por canal independente é o principal
controle contra alteração fraudulenta de dados bancários
(`94-seguranca-threat-model.md`, C2.4) e contra sequestro de conta (C8.3). Sem
contexto de notificação, esses controles não têm implementação.

## 2. Princípios

1. **Todo estado que bloqueia o cliente é comunicado**, mesmo sem novidade — "ainda
   estamos aguardando" é informação.
2. **Linguagem do cliente, não do sistema.** `EFFECT_OUTCOME_UNKNOWN` não é
   mensagem; é código.
3. **Notificação de segurança é independente do canal comprometido.** Alteração de
   dados bancários notifica os contatos cadastrados anteriormente, não o novo.
4. **Notificação não é o registro.** O ledger é a prova; a notificação é
   conveniência — mas o envio é registrado.
5. **Silêncio é decisão consciente.** Evento sem notificação está declarado na
   matriz, não esquecido.

## 3. Modelo de domínio

| Agregado | Papel |
| --- | --- |
| `NotificationTemplate` | Modelo versionado por tipo, canal e idioma |
| `NotificationRule` | Mapeia evento de domínio para destinatários, canais e condições |
| `NotificationRequest` | Intenção de notificar, resolvida em entregas |
| `NotificationDelivery` | Entrega por canal e destinatário, com estado |
| `RecipientPreference` | Preferências por usuário e categoria |
| `ContactPoint` | Endereço verificado, com histórico |

### 3.1 `ContactPoint` e verificação

| Campo | Observação |
| --- | --- |
| `channel` | `EMAIL`, `SMS`, `PUSH`, `WEBHOOK`, `IN_APP` |
| `value` | Classe `PII` |
| `verifiedAt` | Nulo enquanto não verificado |
| `verificationMethod` | Como foi verificado |
| `usableForSecurityAlerts` | Verdadeiro apenas se verificado e fora de quarentena |
| `addedAt` | Base da quarentena |

**Quarentena de contato**: contato recém-adicionado não recebe alerta de segurança
por prazo configurado. Sem isso, o atacante que compromete a conta adiciona o
próprio e-mail e recebe os alertas sobre as próprias ações — tornando o controle
C2.4 inútil.

## 4. Matriz de notificação

| Evento | Destinatário | Canal | Urgência | Silencioso? |
| --- | --- | --- | --- | --- |
| `OffersPresented` | Operador, aprovador | E-mail, no app, push | Alta | não |
| `FundingRequestExpired` (sem oferta) | Operador | E-mail, no app | Média | não |
| `ContractSigned` | Todas as partes | E-mail com cópia | Média | não |
| `EffectConfirmed` | Operador, aprovador | No app | Baixa | não |
| `EffectRejected` | Operador, aprovador | E-mail, no app | Alta | não |
| `EffectPartiallyConfirmed` | Operador, aprovador | E-mail, no app | Alta | não |
| `EffectOutcomeUnknown` | Operador | No app; e-mail após limiar de tempo | Média | não — ver 5 |
| `DisbursementConfirmed` | Operador, aprovador | E-mail, no app, push | Alta | não |
| `FundingMismatchDetected` | Aprovador | E-mail, telefone | Crítica | não |
| `ReconciliationMismatchDetected` | — | — | — | **sim** — interno |
| `AssetSettled` | — | — | — | **sim** — ruído |
| `ConsentGranted` | Aprovador | E-mail | Média | não |
| `ConsentRevoked` | Todos os administradores | E-mail | Alta | não |
| `RepresentationRevoked` | Todos os administradores | E-mail | Alta | não |
| `BankAccountChangeRequested` | **Contatos anteriores** | E-mail, SMS | Crítica | não |
| `BankAccountChangeApproved` | **Contatos anteriores** | E-mail, SMS | Crítica | não |
| Novo dispositivo | Usuário e administradores | E-mail | Alta | não |
| `VerificationReady` | Operador | E-mail, no app | Média | não |
| Pendência de KYB | Operador | E-mail, no app | Média | não — com lembrete |
| `PartnerInvited` | — | — | — | **sim** — interno |
| `AssetBecameStale` | — | — | — | **sim** — ruído |

**Notificações marcadas como silenciosas são decisão registrada**, não omissão.
`AssetSettled` ocorre milhares de vezes; notificar seria ruído que faz o cliente
desativar tudo — inclusive os alertas de segurança.

**`BankAccountChange*` notifica contatos anteriores**: implementação direta do
controle C2.4. Notificar o contato novo seria notificar o atacante.

## 5. Comunicação em estado de exceção

O caso que motiva este contexto.

### 5.1 Escalonamento por tempo

| Tempo no estado | Ação |
| --- | --- |
| Entrada | Registro no app, com estado e expectativa |
| Após limiar 1 (ex.: 2 h) | E-mail explicando a situação e o prazo esperado |
| Após limiar 2 (ex.: 8 h) | E-mail com atualização e canal de contato |
| Após limiar 3 (ex.: 24 h) | Contato ativo da operação |
| Resolução | Notificação imediata do desfecho |

### 5.2 Redação

O que o cliente precisa saber, nesta ordem: o que está acontecendo, o que isso
significa para ele, o que está sendo feito, quando terá resposta, o que ele pode
fazer.

**Exemplo — `EffectOutcomeUnknown`**:

> **Sua operação está em verificação junto à registradora**
>
> O registro da cessão dos seus recebíveis foi enviado, mas ainda não recebemos a
> confirmação da registradora. Isso acontece ocasionalmente por instabilidade na
> comunicação entre sistemas.
>
> **O que isso significa**: o desembolso está retido até a confirmação. Seu
> contrato permanece válido e seus recebíveis não foram liberados para outra
> operação.
>
> **O que estamos fazendo**: consultando a registradora periodicamente para
> confirmar a situação do registro.
>
> **Previsão**: até o fim do próximo dia útil. Avisaremos assim que houver
> definição.
>
> Dúvidas: [canal].

**Regras de redação**:

- sem código técnico nem nome de estado interno;
- prazo declarado, mesmo aproximado — "em breve" aumenta ansiedade;
- dizer o que **não** aconteceu de ruim, quando verdadeiro (contrato válido,
  ativos não liberados) reduz o impacto percebido;
- sem transferir culpa ao parceiro nominalmente;
- sem prometer o que não se controla.

## 6. Entrega

### 6.1 Estados

```text
REQUESTED -> RENDERED -> QUEUED -> SENT -> DELIVERED -> [READ]
```

Exceções: `SUPPRESSED` (preferência ou regra), `FAILED`, `BOUNCED`.

### 6.2 Requisitos

| Requisito | Definição |
| --- | --- |
| Idempotência | Uma notificação por evento e destinatário; repetição do evento não reenvia |
| Ordem | Não garantida entre canais; cada mensagem é autocontida |
| Agrupamento | Eventos de baixa urgência agrupados em resumo periódico |
| Limite | Máximo por destinatário e janela, exceto urgência crítica |
| Preferência | Respeitada, **exceto** para notificação de segurança e obrigação contratual |
| Retentativa | Até 24 h, com espera crescente |
| Registro | Todo envio gera `NotificationDispatched` no ledger |

**Preferência não se aplica a notificação de segurança**: permitir desativar o
alerta de alteração de conta bancária é oferecer ao atacante um interruptor para o
controle que o detectaria.

### 6.3 Dados no conteúdo

| Permitido | Proibido |
| --- | --- |
| Valor da operação, prazo, estado | Dados bancários completos |
| Nome do financiador | Documento de identificação |
| Identificador da operação | Anexo com dossiê |
| Ligação para o app | Token de acesso em texto |

**Regra**: a notificação informa e direciona; o dado sensível está atrás de
autenticação. Anexar contrato ao e-mail é conveniente e cria cópia não controlada
de documento com dados pessoais e comerciais.

**Exceção**: a cópia do instrumento assinado às partes (`08-prd`, requisito 9) é
obrigação contratual. Solução: ligação autenticada e com expiração, não anexo.

## 7. Webhooks para o cedente

Cliente com ERP integrado quer receber eventos, não ler e-mail. Contrato em
`91-contratos-api.md`, 5.2.

| Requisito | Definição |
| --- | --- |
| Assinatura | Sobre o corpo bruto, com segredo por organização |
| Eventos | Subconjunto configurável, sempre por tipo |
| Entrega | Pelo menos uma vez; consumidor deve deduplicar |
| Retentativa | Até 24 h |
| Desativação | Após falha persistente, com notificação por outro canal |
| Conteúdo | Sem `PII_SENSITIVE`; referências resolvidas por API |

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| C1 | Limiares de escalonamento (5.1) sem base empírica | Médio |
| C2 | Provedor de e-mail e SMS não escolhido | Baixo |
| C3 | Textos exigem revisão de redação e jurídica | Alto |
| C4 | Prazo de quarentena de contato (3.1) não definido | Médio |
| C5 | Obrigatoriedade legal de notificar cessão ao sacado não levantada | **Alto** — ver `docs/reference/regulacao-brasil.md` |

**C5 é a lacuna mais relevante**: se a cessão de duplicata exigir notificação ao
sacado, isso não é preferência de produto, é requisito legal — e traz o ator A10
(`95-personas-jornadas.md`) de volta ao escopo.

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 1 | `09`, `10` | Estado de exceção sem comunicação |
| 3.1 | — | Controle C2.4 sem implementação |
| 4 | Todos | Nenhum evento tinha destinatário definido |
| 5 | `09-prd-effects-orchestrator.md` | `UNKNOWN_OUTCOME` do ponto de vista do cliente |
| 6.3 | `08-prd-contracting.md` | "Disponibilizar cópia às partes" sem mecanismo seguro |
