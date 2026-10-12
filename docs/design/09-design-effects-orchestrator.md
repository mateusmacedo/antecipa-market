# Design — Effects Orchestrator

> **PRD de origem**: `docs/prd/09-prd-effects-orchestrator.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| F1 | Tipos de efeito sem mapeamento para códigos reais | "Tipos de efeito iniciais" — lista conceitual |
| F2 | "Mapear o contrato ao tipo de efeito aceito" sem tabela | Req. 2 |
| F3 | Confirmação parcial sem caminho de tratamento | Req. 7; delegado a Funding sem decisão |
| F4 | "Extensões automáticas a recebíveis futuros" sem modelo | Req. 8 |
| F5 | Invalidação de elegibilidade após assinatura não tratada | Ausente |
| F6 | `EffectDispute` sem ciclo de vida | Req. 10 |
| F7 | Ordem de prioridade entre efeitos declarada fora de escopo, sem consequência definida | Invariante |

## 2. O ponto de não retorno

Este é o único contexto que produz consequência irreversível. Antes dele, tudo é
compensável por cancelamento. Depois, o efeito existe no mundo, e desfazê-lo exige
comando de liberação que a infraestrutura pode recusar.

Três consequências de desenho:

1. **A revalidação imediatamente anterior ao comando é o último controle real.**
   Req. 3 do PRD ("reconsultar posição imediatamente antes do comando") não é
   redundância — é o único ponto onde a dupla negociação externa ainda é
   detectável.
2. **Nenhuma otimização pode pular a revalidação.** Cache de posição, reuso de
   consulta recente e "acabamos de consultar" são todos inaceitáveis aqui.
3. **O intervalo entre revalidação e comando deve ser o menor possível.** Toda a
   preparação ocorre antes; a revalidação é o penúltimo passo e o comando, o
   último.

## 3. Sequência de submissão

```text
1. Contrato assinado recebido
2. Validar completude técnica do contrato
3. Resolver tipo de efeito (ver 4)
4. Resolver infraestrutura de destino por ativo
5. Montar alvos (EffectTarget por ativo)
6. Verificar capacidade do parceiro
7. Calcular chave idempotente
8. Persistir RegulatoryEffect em PLANNED
--- a partir daqui, janela crítica ---
9. REVALIDAR posição de todos os ativos      <- último controle
10. Divergência? -> abortar, abrir caso, não enviar
11. Persistir estado PREVALIDATING -> SUBMITTED (antes de enviar)
12. Enviar comando pelo Gateway
13. Registrar resposta
--- fim da janela crítica ---
14. Aguardar confirmação (callback ou consulta)
15. Consultar estado para confirmação crítica (controle C1.3)
16. Processar confirmação por alvo
17. Derivar estado do agregado
18. Publicar evento
```

**Passo 11 antes do 12**: o estado é persistido antes do envio. Se o processo
falhar entre 11 e 12, o comando fica em `SUBMITTED` sem resposta — e é tratado
como `UNKNOWN_OUTCOME`, exigindo consulta. Isso é conservador e correto. A ordem
inversa (enviar e depois persistir) pode perder o registro de um comando que
surtiu efeito — o pior resultado possível.

**Passo 10**: divergência entre a posição revalidada e a do contrato **aborta**.
Não ajusta o lote, não envia parcial, não continua. O contrato foi assinado sobre
um conjunto específico; alterá-lo unilateralmente descasa contrato e efeito.

## 4. Mapeamento contrato → efeito

Req. 2 exige o mapeamento sem fornecê-lo. Estrutura da tabela de resolução:

| Entrada | Origem |
| --- | --- |
| Natureza jurídica da operação | Contrato |
| Tipo de ativo | Portfolio |
| Infraestrutura de destino | Partner Management, por tipo de ativo |
| Existência de coobrigação ou regresso | Contrato |
| Escopo (total ou parcial) | Contrato |
| Produto do financiador | Marketplace |

```text
EffectTypeResolution
  ├─ resolutionId, version
  ├─ conditions: { contractNature, assetType, registryPartnerId, hasRecourse, scope }
  ├─ canonicalEffectType
  ├─ providerEffectCode          <- depende de pesquisa regulatória
  └─ requiredFields[]            <- campos que o parceiro exige para este tipo
```

**Campo `providerEffectCode` não preenchível hoje.** Depende de
`docs/reference/regulacao-brasil.md` e `provedores-integracao.md`. Esta é a lacuna
bloqueante do contexto: sem o código correto, o comando não é enviável.

**Resolução versionada e não código condicional**: mudanças regulatórias alteram
códigos. Uma tabela versionada permite reproduzir qual mapeamento foi usado em
cada operação — necessário para `12-prd-compliance-ledger.md`.

**Regra**: mapeamento não resolvido bloqueia e abre caso. Escolher um tipo
"parecido" é inaceitável — constituir efeito de natureza diferente da contratada
produz divergência entre o que as partes acordaram e o que existe no registro.

## 5. Confirmação parcial

Req. 7 exige tratar confirmação parcial; `10-prd-funding.md` diz que exige
"instrução compatível com a parte confirmada ou revisão contratual" sem decidir.
Decisão construtiva:

### 5.1 Árvore de decisão

```text
Confirmação parcial recebida
  |
  +- O contrato prevê a hipótese? (cláusula de confirmação parcial)
  |    |
  |    +- SIM -> a cláusula define o caminho
  |    |         +- ajuste proporcional -> instrução proporcional ao confirmado
  |    |         +- tudo ou nada        -> solicitar liberação do confirmado;
  |    |                                   cancelar contrato
  |    |
  |    +- NÃO -> caminho padrão: REVISÃO CONTRATUAL
  |               +- efeito confirmado permanece
  |               +- Funding bloqueado
  |               +- caso operacional EFFECT_PARTIAL
  |               +- decisão conjunta cedente e financiador
  |
  +- Percentual confirmado abaixo do mínimo do produto?
       +- SIM -> tratar como rejeição; solicitar liberação
```

### 5.2 Por que revisão contratual como padrão

Desembolsar proporcionalmente sem previsão contratual significa executar operação
diferente da assinada. O valor muda, o conjunto de ativos muda, e possivelmente a
taxa efetiva muda — o cliente aceitou condições que já não se aplicam.

**Custo**: lentidão. Cada confirmação parcial vira caso manual.

**Mitigação**: prever a hipótese no instrumento desde o início. Se o contrato
padrão contiver cláusula de ajuste proporcional com limite (por exemplo, até 10%
de divergência), a maioria dos casos resolve-se automaticamente, e apenas
divergências grandes viram caso.

**Requisito derivado para Contracting**: o instrumento deve conter cláusula de
confirmação parcial. Ausente de `08-prd-contracting.md`.

### 5.3 Alvos não confirmados

| Situação do alvo | Tratamento |
| --- | --- |
| `REJECTED` com motivo de indisponibilidade | Ativo estava comprometido; liberar reserva; sinal de risco |
| `REJECTED` com motivo de validação | Erro de payload; corrigir e reenviar com nova chave |
| `UNKNOWN` | Consultar; não reenviar |

**Rejeição por indisponibilidade é sinal de fraude ou de falha de revalidação**.
Se a revalidação (passo 9) aprovou e o registro rejeitou por indisponibilidade,
algo ocorreu no intervalo — possivelmente dupla negociação em curso. Escala para
risco, não apenas para operações.

## 6. Recebíveis futuros

Req. 8 menciona "extensões automáticas a recebíveis futuros" sem modelar.

**Distinção necessária**: um efeito pode alcançar ativos que ainda não existem —
recebíveis que serão gerados. Isso muda a natureza do agregado: `EffectTarget`
deixa de ser lista fechada.

```text
EffectScope
  ├─ mode: ENUMERATED | RULE_BASED
  ├─ enumeratedTargets[]        (modo ENUMERATED)
  └─ rule                       (modo RULE_BASED)
       ├─ assetTypes[]
       ├─ periodFrom, periodUntil
       ├─ acquirerIds[] / schemeIds[]
       ├─ maxAmount
       └─ priority
```

**Consequências não tratadas no PRD**:

1. O efeito permanece ativo enquanto a regra vigorar — não termina com a
   confirmação.
2. Ativos futuros que entram no escopo são alcançados sem novo comando, se a
   infraestrutura suportar; se não suportar, exigem comando por período.
3. O Portfolio precisa refletir o comprometimento futuro na disponibilidade — um
   recebível que nascerá já comprometido não é disponível.
4. A liberação precisa encerrar a regra, não apenas os alvos existentes.

**Impacto em Eligibility**: `AST-ENC-001` (efeito incompatível conhecido) precisa
avaliar efeitos baseados em regra, não apenas os enumerados. Um ativo sem gravame
específico pode estar sob regra vigente.

**Esta é uma lacuna de médio porte com efeito em três contextos.** Se recebíveis
futuros entrarem no MVP, exige desenho adicional. Se ficarem para depois — o que a
estratégia de entrega de `00-prd` sugere, ao colocá-los na fase 4 — o modelo deve
ao menos não impedir a extensão.

## 7. Invalidação após assinatura

Situação ausente dos PRDs: `EligibilityInvalidated` chega depois da assinatura e
antes do registro.

| Momento | Tratamento |
| --- | --- |
| Antes do passo 9 | Abortar; não enviar; abrir caso; contrato cancelável |
| Entre 9 e 12 (janela crítica) | **Não interromper** — ver abaixo |
| Após 12, antes da confirmação | Não cancelar; aguardar desfecho; tratar depois |
| Após confirmação | Efeito existe; tratar por liberação, se aplicável |

**Por que não interromper na janela crítica**: entre a revalidação e o envio há
segundos. Interromper por evento que chega nesse intervalo introduz corrida sem
ganho — a revalidação acabou de confirmar a posição junto à fonte autoritativa,
que prevalece sobre qualquer projeção local (invariante do PRD: "a confirmação
externa prevalece sobre qualquer projeção local").

**Após o envio, cancelar é impossível**. O comando está em trânsito. A postura é
aguardar o desfecho e tratar — inclusive por liberação, se o efeito se confirmar e
não dever existir.

## 8. Disputas

Req. 10 cria `EffectDispute` sem ciclo de vida.

```text
OPENED -> INVESTIGATING -> [RESOLVED_INTERNAL | ESCALATED_TO_PARTNER]
       -> PARTNER_REVIEW -> [RESOLVED_PARTNER | UNRESOLVED]
```

| Gatilho | Severidade |
| --- | --- |
| Contrato e posição oficial divergem quanto ao titular | Crítica |
| Efeito confirmado que não corresponde ao solicitado | Crítica |
| Efeito de terceiro sobre ativo alcançado por contrato vigente | Crítica |
| Liberação solicitada e não confirmada após prazo | Alta |
| Posição externa diverge da confirmação recebida | Alta |

**A plataforma não resolve disputa de prioridade jurídica** — invariante do PRD.
Registra, evidencia, comunica as partes e acompanha. A resolução é das partes e da
infraestrutura.

**O que a plataforma faz**: preserva evidência íntegra de contrato, comando,
resposta e posição em cada instante. É exatamente o que torna a disputa
resolvível, e é o valor mais concreto do Compliance Ledger.

## 9. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| F8 | Códigos de efeito por infraestrutura (4) | **Bloqueante** |
| F9 | Cláusula de confirmação parcial no instrumento (5.2) | Alto — exige jurídico |
| F10 | Suporte a recebíveis futuros por infraestrutura (6) | Médio |
| F11 | Prazo de confirmação por infraestrutura | Alto — define o limiar de `UNKNOWN_OUTCOME` |
| F12 | Procedimento de disputa acordado com cada infraestrutura | Alto |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 3 | 3, 4 | Sequência e ordem de persistência |
| 4 | 2 | Mapeamento sem tabela |
| 5 | 7 | Confirmação parcial sem caminho |
| 6 | 8 | Recebíveis futuros sem modelo |
| 7 | — | Invalidação após assinatura |
| 8 | 10 | Disputa sem ciclo de vida |
