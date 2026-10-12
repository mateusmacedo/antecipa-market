# Personas e Jornadas

> **Camada**: construtiva
> **Origem**: lacuna identificada — os PRDs em `docs/prd/` descrevem contextos e
> estados, mas não nomeiam atores. Sem ator definido não há como derivar tela,
> permissão, notificação ou critério de aceitação observável.
> **Status**: proposta para validação

## Por que este documento existe

Os 15 PRDs descrevem a plataforma pela ótica do sistema: agregados, invariantes,
máquinas de estado. Nenhum deles responde "quem faz isso e o que essa pessoa vê
quando dá errado". Esta lacuna tem consequência construtiva direta:

- `01-prd-identity-organization.md` exige segregação entre "operador, aprovador,
  administrador e auditor" sem definir o que cada um faz.
- `11-prd-settlement-reconciliation.md` cria `ReconciliationCase` com
  "responsável e SLA" sem definir quem é esse responsável nem onde ele trabalha.
- Todos os PRDs listam métricas sem dizer quem as consome.

## Mapa de atores

| # | Ator | Organização | Natureza | Acessa a plataforma? |
| --- | --- | --- | --- | --- |
| A1 | Operador financeiro | Cedente | Humano | Sim — portal do cedente |
| A2 | Aprovador / sócio-administrador | Cedente | Humano | Sim — portal do cedente |
| A3 | Contador / assessor externo | Cedente (terceirizado) | Humano | Sim — acesso delegado |
| A4 | Analista de crédito | Financiador | Humano | Sim — portal do financiador |
| A5 | Analista de compliance | Financiador | Humano | Sim — portal do financiador |
| A6 | Sistema do financiador | Financiador | Máquina | Sim — API |
| A7 | Analista de operações | Plataforma | Humano | Sim — backoffice |
| A8 | Analista de risco/fraude | Plataforma | Humano | Sim — backoffice |
| A9 | Auditor / regulador | Externo | Humano | Sim — acesso restrito a dossiê |
| A10 | Sacado / devedor | Terceiro | Humano | Não no MVP — ver abaixo |
| A11 | Sistema da registradora/escriturador | Infraestrutura | Máquina | Sim — webhook/callback |

### A10 — o ator ausente nos PRDs

O sacado da duplicata tem papel jurídico ativo: aceita ou recusa o título, é
notificado da cessão e é quem efetivamente paga. Nenhum PRD trata dele. Isso é
uma decisão implícita não declarada — a plataforma assume que a comunicação com o
sacado ocorre fora dela, via escriturador.

**Decisão a tomar**: manter o sacado fora do escopo (e declarar isso no
`00-prd-mapa-de-produto.md` como não-objetivo) ou criar um contexto de
relacionamento com o sacado. A segunda opção tem impacto grande: notificação de
cessão, portal de consulta do sacado, tratamento de recusa de aceite.

Recomendação: manter fora do MVP, declarar explicitamente, e registrar como
dívida de produto para a fase de duplicata escritural.

---

## Personas detalhadas

### P1 — Operador financeiro do cedente

**Perfil**: analista financeiro ou assistente administrativo em empresa de médio
porte. Usa ERP diariamente, planilha constantemente. Não é especialista em
mercado financeiro. Antecipa recebíveis quando o fluxo de caixa aperta — decisão
frequentemente urgente e no fim do dia.

**O que quer**: saber quanto dinheiro consegue, quando cai na conta e quanto
custa. Nessa ordem.

**O que teme**: descobrir tarde que a operação não vai sair; comprometer
recebíveis que já estão comprometidos; perder o prazo de pagamento de um
fornecedor.

**Como mede sucesso**: dinheiro na conta hoje ou amanhã.

**Papel na plataforma**: `OPERATOR`. Cria solicitações, visualiza ofertas,
prepara contratação. Não assina sozinho quando a organização exige dupla
aprovação.

**Fricção prevista**: a jornada exige KYB completo, consentimento, opt-in externo
e elegibilidade antes da primeira oferta. Para quem quer dinheiro hoje, isso é
uma parede. O produto precisa separar claramente onboarding (uma vez) de operação
(recorrente) e dar visibilidade de progresso no onboarding.

### P2 — Aprovador do cedente

**Perfil**: sócio, diretor financeiro ou procurador com poderes de assinatura.
Entra na plataforma poucas vezes, para decidir e assinar.

**O que quer**: entender a operação em menos de um minuto e assinar com
segurança.

**O que teme**: assinar sob informação incompleta; ser responsabilizado por
condição que não viu.

**Papel na plataforma**: `APPROVER`. Aprova seleção de oferta, assina contrato,
aprova mudança de dados bancários e de representantes.

**Requisito derivado**: a tela de assinatura deve apresentar as condições
materiais sem exigir leitura do instrumento completo, e o aceite dessas condições
deve ser registrado separadamente da assinatura do documento —
`08-prd-contracting.md` requisito 8 já prevê isso, mas sem definir o que é
"condição material".

**Definição proposta de condição material** (fecha a lacuna): valor bruto cedido,
valor líquido a receber, custo total em reais, taxa efetiva mensal, data prevista
de crédito, identificação do financiador, lista de ativos alcançados, tipo de
efeito constituído, existência de coobrigação ou direito de regresso.

### P3 — Contador ou assessor externo

**Perfil**: escritório contábil que opera em nome de vários clientes.

**O que quer**: operar para múltiplos cedentes sem trocar de login.

**Papel na plataforma**: `OPERATOR` com `Membership` em várias organizações.

**Requisito derivado**: `01-prd-identity-organization.md` já modela `Membership`
por organização, mas a experiência de troca de contexto (seletor de organização,
isolamento visual, prevenção de erro operacional entre clientes) não está
especificada. Risco concreto: solicitar antecipação para o cliente errado.

**Controle proposto**: identificação persistente e visualmente destacada da
organização ativa em toda a interface, e reconfirmação da organização na tela de
seleção de oferta.

### P4 — Analista de crédito do financiador

**Perfil**: analista em banco, FIDC, securitizadora ou fintech de crédito.
Avalia risco e decide a oferta.

**O que quer**: pedidos com dados suficientes para decidir, no formato que seu
sistema consome, sem retrabalho.

**O que teme**: comprar recebível já comprometido; cedente fraudulento;
concentração indevida.

**Papel na plataforma**: `PARTNER_ANALYST`, restrito à organização financiadora.

**Requisito derivado crítico**: `01-prd-identity-organization.md` estabelece
"proibição de acesso operacional cruzado entre financiadores" como requisito de
segurança, mas não define o que um financiador pode ver de um pedido antes de
ofertar. Se vê o dossiê completo, há vazamento de base de clientes entre
concorrentes. Se vê pouco, não decide.

**Proposta**: pacote de dados por convite, definido por `PartnerAgreement`, com
três níveis — dados de triagem (sem identificar o cedente), dados de análise
(identificado, mediante consentimento específico) e dados de contratação
(completo, após seleção). Cada transição é um evento auditado.

### P5 — Analista de compliance do financiador

**Perfil**: responsável por PLD/FT e aprovação cadastral no financiador.

**O que quer**: dossiê KYB reutilizável, com origem e data de cada evidência, para
aplicar sua própria política.

**Papel na plataforma**: `PARTNER_COMPLIANCE`.

**Requisito derivado**: `02-prd-kyb-kyc.md` deixa claro que a decisão é do
financiador, mas não define o mecanismo de devolução dessa decisão nem o que
acontece quando dois financiadores divergem sobre o mesmo cedente. Hoje o PRD diz
"sem transformá-la em decisão global" — falta dizer como isso aparece na
elegibilidade: um cedente reprovado pelo financiador A continua elegível para B.

### P6 — Analista de operações da plataforma

**Perfil**: back-office. Trata exceções: comando externo em estado desconhecido,
divergência de conciliação, falha de sincronização de consentimento.

**O que quer**: fila de trabalho priorizada, contexto completo do caso, ação
segura e rastreada.

**Papel na plataforma**: `PLATFORM_OPS`.

**Requisito derivado**: nenhum PRD especifica a ferramenta interna. Os contextos
produzem casos (`ReconciliationCase`), estados de exceção (`UNKNOWN_OUTCOME`,
`MISMATCH`, `DISPUTED`) e filas mortas (`DLQ operacional` em
`04-prd-receivables-gateway.md`) sem destino operacional.

Ver `docs/design/17-design-backoffice.md`.

### P7 — Analista de risco e fraude da plataforma

**Perfil**: avalia sinais de risco antes da distribuição do pedido.

**Papel na plataforma**: `PLATFORM_RISK`.

**Requisito derivado**: não há contexto de fraude nos PRDs. Ver
`docs/design/15-design-fraude-risco.md` e
`docs/reference/fraude-tipologias-controles.md`.

**Limite de papel**: a plataforma produz sinal, o financiador decide. Esse limite
já está correto em `02-prd-kyb-kyc.md` (`RiskSignal`: "sinal informativo, nunca
decisão automática da plataforma") e deve ser preservado no contexto de fraude —
com uma exceção declarada: a plataforma pode bloquear a própria operação por
risco de fraude sem isso constituir decisão de crédito.

### P8 — Auditor ou regulador

**Perfil**: auditoria interna do financiador, auditoria externa, ou autoridade.

**O que quer**: reconstruir uma operação específica integralmente.

**Papel na plataforma**: `AUDITOR`, somente leitura, com registro do próprio
acesso.

**Requisito derivado**: `12-prd-compliance-ledger.md` prevê `EvidenceBundle` e
registro de acesso, mas não define quem autoriza a emissão de um dossiê nem qual
o fluxo de aprovação. Dossiê contém dados pessoais e comerciais sensíveis.

---

## Jornadas

### J1 — Onboarding do cedente (uma vez)

```text
Convite ou auto-cadastro
  -> Criação da organização e do primeiro usuário
  -> Validação de identidade do usuário
  -> Registro de representantes e poderes
  -> Abertura do caso de verificação (KYB)
  -> Coleta de dados e documentos
  -> Resolução de pendências
  -> Dossiê completo
  -> Apresentação e concessão de consentimentos
  -> Ativação do opt-in na infraestrutura
  -> Primeira descoberta de portfólio
```

**Duração esperada**: horas a dias, dependendo de pendência documental e do SLA
da infraestrutura para ativar o opt-in.

**Ponto de abandono mais provável**: coleta documental. Mitigação: permitir
progresso parcial, salvar rascunho, e deixar claro o que falta e por quê.

**Lacuna nos PRDs**: não há tratamento de onboarding interrompido e retomado.
`02-prd-kyb-kyc.md` tem o estado `PENDING_INFORMATION` mas sem política de
expiração, lembrete ou reengajamento.

### J2 — Operação de antecipação (recorrente)

```text
Portfólio atualizado
  -> Simulação (sem reserva)
  -> Seleção de ativos e criação da solicitação
  -> Avaliação de elegibilidade
  -> Distribuição aos financiadores autorizados
  -> Recepção de ofertas
  -> Comparação
  -> Seleção da oferta
  -> Revalidação de ativos, poderes e consentimentos
  -> Assinatura
  -> Registro do efeito na infraestrutura
  -> Confirmação do efeito
  -> Ordem de desembolso
  -> Confirmação de desembolso
  -> Acompanhamento de liquidação
  -> Conciliação
  -> Liberação do efeito
```

**Gargalo esperado**: a confirmação do efeito pela infraestrutura, não a decisão
de crédito.

**Lacuna nos PRDs**: `00-prd-mapa-de-produto.md` mede "tempo entre solicitação e
primeira oferta" e "tempo entre efeito confirmado e desembolso", mas não há SLA
alvo declarado para nenhuma etapa. Sem alvo, a métrica não orienta engenharia.
Ver `docs/design/93-requisitos-nao-funcionais.md`.

### J3 — Exceção: efeito em resultado desconhecido

```text
Comando enviado à infraestrutura
  -> Sem resposta dentro da janela
  -> Estado UNKNOWN_OUTCOME
  -> Bloqueio de Funding
  -> Consulta de estado à infraestrutura
  -> [resolvido: EFFECTIVE ou REJECTED] ou [não resolvido: caso operacional]
  -> Comunicação ao cedente
```

**Quem trata**: P6. **O que o cedente vê**: precisa ser definido. Hoje nenhum PRD
especifica comunicação ao cedente em estado de exceção. Do ponto de vista dele,
assinou um contrato e o dinheiro não veio — a ausência de comunicação é o pior
resultado possível.

**Requisito derivado**: todo estado de exceção que dure mais que um limite
definido gera comunicação ao cedente, com linguagem não técnica e expectativa de
prazo.

### J4 — Exceção: divergência de conciliação

```text
Liquidação reportada
  -> Comparação com expectativa
  -> Divergência de valor, data ou beneficiário
  -> Abertura de ReconciliationCase
  -> Triagem e classificação de impacto
  -> Investigação (consulta à fonte, contato com parceiro)
  -> Resolução com justificativa e autorização
  -> Registro no ledger
```

**Quem trata**: P6. **Lacuna**: `11-prd-settlement-reconciliation.md` requisito 8
exige "ajuste somente com justificativa e autorização" sem definir quem autoriza
nem em que nível de alçada.

**Proposta de alçada**: divergência abaixo de limite configurável resolve-se com
um aprovador; acima, exige segundo aprovador; divergência com suspeita de fraude
escala para P7 independentemente do valor.

### J5 — Revogação de consentimento

```text
Cedente solicita revogação
  -> Registro da intenção
  -> Bloqueio imediato de novas consultas
  -> Propagação aos destinatários
  -> Acompanhamento de confirmação por destinatário
  -> Revogação efetiva
```

**Ponto sensível**: `03-prd-consent-mandates.md` invariante — "revogação não
desfaz contratos válidos nem obrigações de retenção". Isso precisa ser explicado
ao cedente no momento da revogação, não depois. Se a pessoa revoga esperando
apagar tudo e descobre que os contratos permanecem, o problema é de expectativa,
não de sistema.

---

## Matriz de papéis e permissões

Fecha a lacuna do requisito 5 de `01-prd-identity-organization.md`, que exige
segregação sem definir o conteúdo de cada papel.

| Ação | OPERATOR | APPROVER | ORG_ADMIN | AUDITOR | PARTNER_ANALYST | PARTNER_COMPLIANCE | PLATFORM_OPS | PLATFORM_RISK |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Visualizar portfólio | sim | sim | sim | leitura | não | não | leitura | leitura |
| Disparar descoberta | sim | sim | sim | não | não | não | sim | não |
| Simular | sim | sim | sim | não | não | não | não | não |
| Criar solicitação | sim | sim | sim | não | não | não | não | não |
| Selecionar oferta | não | sim | não | não | não | não | não | não |
| Assinar contrato | não | sim | não | não | não | não | não | não |
| Conceder consentimento | não | sim | não | não | não | não | não | não |
| Revogar consentimento | não | sim | sim | não | não | não | não | não |
| Alterar dados bancários | propõe | aprova | propõe | não | não | não | não | não |
| Convidar usuário | não | não | sim | não | não | não | não | não |
| Registrar representação | não | não | sim | não | não | não | não | não |
| Ver dossiê KYB próprio | sim | sim | sim | leitura | não | não | leitura | leitura |
| Ver pedido recebido | não | não | não | não | sim | sim | leitura | leitura |
| Enviar oferta | não | não | não | não | sim | não | não | não |
| Registrar decisão cadastral | não | não | não | não | não | sim | não | não |
| Tratar caso de conciliação | não | não | não | não | não | não | sim | não |
| Reenviar comando externo | não | não | não | não | não | não | sim | não |
| Suspender operação por risco | não | não | não | não | não | não | não | sim |
| Emitir dossiê de auditoria | não | não | não | solicita | não | não | aprova | não |

**Invariante derivada**: nenhum papel acumula proposição e aprovação da mesma
ação sensível. `OPERATOR` propõe alteração bancária, `APPROVER` aprova.
`ORG_ADMIN` administra acessos mas não movimenta operação — isso impede que o
comprometimento de uma única conta administrativa produza uma operação completa.

**Lacuna remanescente**: organizações pequenas frequentemente têm uma só pessoa.
A dupla aprovação precisa ser configurável por organização, com registro explícito
de que a organização optou por dispensá-la e aceitação do risco — `01-prd` já diz
"quando configurado", mas sem definir o padrão.

**Padrão proposto**: dupla aprovação obrigatória para alteração de dados
bancários e de representantes em qualquer organização; opcional para seleção de
oferta e assinatura, desativada por padrão.

---

## Rastreabilidade

| Persona/Jornada | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| P1, P2, P3 | `01-prd-identity-organization.md` | Conteúdo dos papéis (req. 5) |
| P2 | `08-prd-contracting.md` | Definição de "condição material" (req. 8) |
| P4 | `07-prd-marketplace.md` | Escopo de dados por convite (req. 2) |
| P5 | `02-prd-kyb-kyc.md` | Devolução e divergência de decisão (req. 9) |
| P6 | `11-prd-settlement-reconciliation.md` | Responsável e alçada (req. 8) |
| P7 | — | Contexto de fraude inexistente |
| P8 | `12-prd-compliance-ledger.md` | Autorização de emissão de dossiê |
| J3, J4 | `09-prd-effects-orchestrator.md`, `11-prd-settlement-reconciliation.md` | Comunicação ao cedente em exceção |
| Matriz | `01-prd-identity-organization.md` | Permissões efetivas |
