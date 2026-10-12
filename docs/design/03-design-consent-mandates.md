# Design — Consent & Mandates

> **PRD de origem**: `docs/prd/03-prd-consent-mandates.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| K1 | "Aceite forte" sem definição | Req. 2 |
| K2 | Interseção entre escopo interno e externo sem algoritmo | Invariante |
| K3 | Propagação de revogação sem mecanismo nem prazo | Req. 7 |
| K4 | `EXTERNAL_SYNC_FAILED` sem tratamento | Estados adicionais |
| K5 | Renovação sem definição do que muda | Req. 9 |
| K6 | Texto do instrumento sem estrutura | `ConsentEvidence` |
| K7 | Relação entre os quatro instrumentos não operacionalizada | Seção "Conceitos separados" |

## 2. Os quatro instrumentos

O PRD acerta ao separar `DataProcessingBasis`, `DataSharingConsent`,
`ReceivablesOptIn` e `OperationalMandate`, e ao proibir que um aceite represente
todos. Construtivamente, isso exige que cada um tenha ciclo de vida próprio.

| Instrumento | Natureza | Concedido por | Revogável | Expira |
| --- | --- | --- | --- | --- |
| `DataProcessingBasis` | Fundamento de tratamento | Registro da base aplicável | Depende da base | Não |
| `DataSharingConsent` | Autorização de compartilhamento | Representante com poderes | Sim | Sim |
| `ReceivablesOptIn` | Autorização regulatória de acesso à agenda | Representante, refletido na infraestrutura | Sim | Sim |
| `OperationalMandate` | Poderes para a plataforma comandar | Representante | Sim | Sim |

**`DataProcessingBasis` não é consentimento.** Quando a base legal é execução de
contrato ou obrigação legal, não há consentimento a coletar — há base a registrar.
Apresentar um botão de "aceito o tratamento de dados" quando a base é contratual
cria a impressão falsa de que a revogação interrompe o tratamento. Isso é problema
de conformidade, não de interface.

**Requisito derivado**: a interface de consentimentos distingue o que o titular
pode revogar do que apenas lhe é informado.

### 2.1 Matriz de dependência

| Ação | Exige |
| --- | --- |
| Consultar agenda de recebíveis | `ReceivablesOptIn` ativo + base de tratamento |
| Compartilhar dados com financiador | `DataSharingConsent` incluindo aquele financiador |
| Enviar comando em nome do cliente | `OperationalMandate` + representação vigente |
| Registrar efeito | `OperationalMandate` + contrato assinado |

**Uma consulta de agenda para simulação exige opt-in; um compartilhamento com
financiador exige consentimento adicional.** Tratá-los como um só levaria a
compartilhar dados sob autorização que só cobria a consulta.

## 3. Aceite forte

Req. 2 exige "aceite forte pelo representante com poderes válidos" sem definir.

**Definição proposta** — aceite forte exige, cumulativamente:

| Elemento | Requisito |
| --- | --- |
| Identidade | Usuário autenticado, com identidade verificada no cadastro |
| Poder | `Representation` vigente, com alçada para o ato |
| Reautenticação | Fator adicional no momento do aceite, mesmo com sessão ativa |
| Apresentação | Texto integral exibido antes do aceite, sem rolagem obrigatória substituída por resumo |
| Ação afirmativa | Ação explícita e específica; caixa pré-marcada não constitui aceite |
| Registro | Texto, versão, hash, canal, método de autenticação, instante, endereço de origem |
| Irretratabilidade da prova | Evidência preservada com hash |

**Caixa pré-marcada não constitui aceite** e nem aceite agrupado de instrumentos
distintos — decorre diretamente da regra do PRD de que "um aceite de termos não
deve representar implicitamente todos esses instrumentos".

## 4. Escopo efetivo

A invariante do PRD — "escopo efetivo é a interseção entre consentimento interno e
autorização externa" — precisa de algoritmo.

```text
escopoEfetivo = interseção(escopoInterno, escopoExterno)

Por dimensão:
  assetTypes:    interseção dos conjuntos
  período:       [max(inícios), min(fins)]
  destinatários: interseção dos conjuntos
  finalidades:   interseção dos conjuntos
  dataCategories: interseção dos conjuntos
```

| Situação | Resultado |
| --- | --- |
| Autorização externa ausente | Escopo efetivo vazio — nenhuma consulta permitida |
| Externa mais restrita que a interna | Vale a externa |
| Interna mais restrita que a externa | Vale a interna |
| Externa com escopo não representável internamente | Escopo efetivo vazio; abrir caso |

**Autorização externa ausente resulta em escopo vazio, não em escopo interno.** O
opt-in na infraestrutura é a autorização regulatória; sem ela, o consentimento
interno não habilita consulta. Tratar o interno como suficiente produziria consulta
sem autorização válida.

**Escopo não representável**: se a infraestrutura autoriza dimensão que o modelo
interno não expressa, o escopo efetivo não é computável com segurança. Esvaziar é
a postura conservadora.

## 5. Sincronização externa

### 5.1 Ativação

```text
ConsentGranted (interno, ACTIVE)
  -> ActivateExternalOptIn via AuthorizationPort
  -> [confirmado] -> ExternalOptInActivated; escopo efetivo calculado
  -> [falha recuperável] -> retentar
  -> [falha definitiva] -> EXTERNAL_SYNC_FAILED
  -> [desconhecido] -> consultar estado; nunca reenviar cegamente
```

**Durante a sincronização o escopo efetivo é vazio.** O consentimento interno
existe; a consulta não é permitida. A interface mostra "autorização em ativação",
não "ativa".

### 5.2 `EXTERNAL_SYNC_FAILED`

Estado listado no PRD sem tratamento.

| Aspecto | Definição |
| --- | --- |
| Efeito | Escopo efetivo vazio; consultas bloqueadas |
| Visibilidade | Comunicado ao cliente com linguagem de negócio |
| Tratamento | Caso `CONSENT_SYNC_FAILED` (`17-design-backoffice.md`) |
| Retomada | Nova tentativa não exige novo aceite, se o texto não mudou |
| Expiração | Após limite, exige novo aceite |

**Nova tentativa não exige novo aceite**: falha técnica de sincronização não
invalida a manifestação de vontade. Exigir novo aceite a cada falha transferiria
problema de integração para o cliente. A exceção é o decurso de prazo longo, em
que a manifestação perde atualidade.

### 5.3 Revogação

Req. 7 exige propagação e acompanhamento, sem mecanismo nem prazo.

```text
RequestRevocation
  -> bloqueio interno IMEDIATO (síncrono, antes de qualquer propagação)
  -> ConsentRevocationRequested
  -> propagação paralela:
       - infraestrutura: revokeOptIn
       - financiadores: notificação com confirmação
  -> acompanhamento por destinatário
  -> todos confirmados -> ConsentRevoked
  -> algum pendente além do prazo -> caso operacional
```

| Requisito | Prazo |
| --- | --- |
| Bloqueio interno de novas consultas | Imediato, síncrono |
| Comando à infraestrutura | Menos de 1 min |
| Notificação aos financiadores | Menos de 5 min |
| Confirmação de cada destinatário | Conforme acordo; padrão 1 dia útil |
| Escalonamento por não confirmação | Após o prazo acordado |

**Bloqueio interno é síncrono e precede a propagação.** A revogação produz efeito
interno antes de qualquer confirmação externa. Aguardar confirmação para bloquear
permitiria consulta após a revogação — o contrário do que o instrumento significa.

**Destinatário que não confirma**: o estado permanece `REVOCATION_PENDING` para
aquele destinatário, visível ao cliente. Não se declara revogação completa sem
confirmação — declarar seria afirmar o que não se sabe.

**O que a revogação não faz** (invariante): não desfaz contratos vigentes, não
elimina evidências, não interrompe retenção obrigatória. Isso é comunicado no
momento da solicitação, não depois (`95-personas-jornadas.md`, J5).

## 6. Renovação

Req. 9 exige renovar "sem reutilizar silenciosamente o aceite anterior".

| Situação | Tratamento |
| --- | --- |
| Texto inalterado, escopo inalterado | Novo aceite com apresentação simplificada; nova versão |
| Texto alterado | Apresentação integral; novo aceite pleno |
| Escopo ampliado | Apresentação integral do que muda; novo aceite pleno |
| Escopo reduzido | Novo aceite; ou revogação parcial, se suportada |

**Toda renovação gera nova versão de `ConsentGrant`**, com `supersedesId`. A
anterior não é apagada — permanece como prova do que valia em cada período.

**Renovação automática é proibida** (critério de aceitação: "autorizações
expiradas não são renovadas automaticamente"). O sistema pode lembrar e facilitar;
não pode presumir.

## 7. Estrutura do instrumento

`ConsentEvidence` preserva texto, versão e hash. O texto precisa de estrutura,
para ser apresentável e comparável entre versões.

| Bloco | Conteúdo |
| --- | --- |
| Identificação | Quem autoriza, quem recebe a autorização |
| Finalidade | Para que, em linguagem concreta |
| Dados | Quais categorias |
| Destinatários | Quem receberá, nominalmente |
| Período | Vigência |
| Revogação | Como revogar e qual o efeito |
| Limites | O que a revogação não desfaz |
| Fundamento | Base legal ou regulatória |

**Destinatários nominais e não genéricos**: "instituições financeiras parceiras" é
escopo indeterminado. O consentimento lista os financiadores. Acrescentar um novo
exige novo consentimento — e é exatamente a consequência pretendida.

**Custo operacional a aceitar**: cada novo financiador conectado exige recoletar
consentimento de todos os cedentes que queiram incluí-lo. Mitigação possível:
consentimento por categoria com opt-in explícito por parceiro na primeira
apresentação. Exige validação jurídica.

## 8. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| K8 | Natureza jurídica do opt-in na infraestrutura | **Alto** — ver `docs/reference/regulacao-brasil.md` |
| K9 | Prazo máximo de validade da autorização | Alto |
| K10 | Textos dos instrumentos exigem redação jurídica | Alto |
| K11 | Viabilidade de consentimento por categoria (7) | Alto |
| K12 | Revogação parcial é suportada pelas infraestruturas? | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | Conceitos separados | Instrumentos sem ciclo de vida próprio |
| 3 | 2 | "Aceite forte" sem definição |
| 4 | Invariante | Interseção sem algoritmo |
| 5.2 | Estados | `EXTERNAL_SYNC_FAILED` sem tratamento |
| 5.3 | 7 | Propagação sem mecanismo nem prazo |
| 6 | 9 | Renovação sem definição |
| 7 | `ConsentEvidence` | Instrumento sem estrutura |
