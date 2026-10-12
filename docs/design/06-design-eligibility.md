# Design — Eligibility

> **PRD de origem**: `docs/prd/06-prd-eligibility.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| E1 | Nenhum mecanismo de expressão de regra | "Avaliar regras por tipo de ativo, parceiro e produto" (req. 1) sem dizer como a regra é escrita |
| E2 | Catálogo de regras é exemplificativo, não normativo | Seção "Exemplos de regras" — seis itens ilustrativos |
| E3 | "Justificativa legível" sem formato | Req. 2 |
| E4 | Reserva sem semântica de concorrência | Req. 7 e invariante "reserva interna não produz efeito perante terceiros" |
| E5 | Invalidação sem propagação definida | Req. 6 e evento `EligibilityInvalidated` |
| E6 | Limites de concentração sem definição de cálculo | Req. 5 |
| E7 | Relação entre simulação e avaliação não definida | Req. 9 |

## 2. Decisão estrutural: regras em código versionado, não em motor configurável

**Recomendação**: as regras são implementadas como código, agrupadas em políticas
versionadas, com parâmetros em dados. Não se adota motor de regras configurável em
tempo de execução.

**Razão**: `06-prd` exige três propriedades que um motor configurável dificulta:

1. *"Versionar políticas e parâmetros usados"* — regra em código versiona com o
   repositório, com revisão e histórico. Regra em banco exige construir todo o
   versionamento, revisão e auditoria de novo, pior.
2. *"Regra alterada não modifica avaliação histórica"* — reproduzir avaliação
   antiga exige executar a regra da época. Com código versionado, é a versão do
   artefato. Com motor configurável, é preciso guardar e reinterpretar a
   configuração antiga — incluindo o comportamento do motor da época.
3. *"Produzir justificativa legível para cada exclusão"* — mensagem contextual
   escrita por quem conhece o domínio é melhor que texto genérico gerado.

**O que fica em dados**: parâmetros — faixas de valor, prazos, percentuais de
concentração, listas de arranjos aceitos. Estes mudam com frequência comercial e
não exigem publicação de código.

**Contra-argumento a considerar**: se financiadores passarem a definir regras
próprias e mudá-las com autonomia, a decisão se inverte. O critério de revisão:
quando a frequência de mudança de regra exceder a cadência de publicação, ou
quando parceiros exigirem autonomia de configuração.

## 3. Estrutura da política

```text
EligibilityPolicy
  ├─ policyId, version, effectiveFrom, effectiveUntil
  ├─ scope: { assetTypes[], partnerId?, productId? }
  └─ rules[]
       ├─ ruleId, ruleVersion
       ├─ severity: BLOCKING | WARNING | INFO
       ├─ appliesTo: ASSET | LOT | ORGANIZATION
       └─ evaluate(context) -> RuleResult
```

### 3.1 Severidade

| Severidade | Efeito | Exemplo |
| --- | --- | --- |
| `BLOCKING` | Ativo ou lote excluído | Ativo vencido |
| `WARNING` | Incluído, com alerta ao usuário e ao financiador | Concentração próxima do limite |
| `INFO` | Registrado, sem efeito | Ativo próximo do vencimento |

**Regra de composição**: qualquer `BLOCKING` exclui. `WARNING` não exclui, mas
consta da avaliação, do lote e do evento — o financiador recebe o alerta junto com
o pedido.

### 3.2 Nível de aplicação

Distinção ausente do PRD e necessária: a regra "valor mínimo do financiador" não
se aplica a um ativo, e sim ao lote. Avaliar no nível errado produz resultado
incorreto — cada unidade de recebível isoladamente sempre estaria abaixo do
mínimo.

| Nível | Avalia | Exemplo |
| --- | --- | --- |
| `ASSET` | Um ativo | Vencimento dentro da faixa |
| `LOT` | Conjunto selecionado | Valor mínimo; concentração |
| `ORGANIZATION` | Pré-condições do cedente | KYB completo; consentimento ativo |

**Ordem de avaliação**: `ORGANIZATION` primeiro — se reprovar, não faz sentido
avaliar ativo. Depois `ASSET`, filtrando o conjunto. Por último `LOT`, sobre o que
sobrou. Inverter produz trabalho desperdiçado e mensagem de erro confusa.

## 4. Catálogo normativo de regras

Substitui os "exemplos" do PRD. Cada regra tem identificador estável, usado em
evento, API e interface.

### 4.1 Nível organização

| ID | Regra | Severidade | Parâmetro |
| --- | --- | --- | --- |
| `ORG-KYB-001` | Dossiê cadastral em estado mínimo exigido pelo produto | `BLOCKING` | Estado mínimo por produto |
| `ORG-KYB-002` | Dossiê não expirado | `BLOCKING` | Validade por tipo de evidência |
| `ORG-CNS-001` | Consentimento de consulta ativo e no escopo | `BLOCKING` | — |
| `ORG-CNS-002` | Consentimento de compartilhamento ativo para os financiadores convidados | `BLOCKING` | — |
| `ORG-CNS-003` | Consentimento não expira antes do fim da janela de oferta | `WARNING` | Margem em dias |
| `ORG-REP-001` | Representação válida para o ato e o valor | `BLOCKING` | Alçada |
| `ORG-STS-001` | Organização ativa, não suspensa | `BLOCKING` | — |
| `ORG-RSK-001` | Sem bloqueio de risco vigente | `BLOCKING` | — |
| `ORG-BNK-001` | Conta de destino validada e fora do período de quarentena | `BLOCKING` | Prazo de quarentena |

**`ORG-CNS-003`**: consentimento que expira durante a janela de oferta produz
operação que não pode ser contratada. Detectar na elegibilidade evita desperdiçar
o ciclo inteiro.

**`ORG-BNK-001`**: não está no PRD e decorre do controle C2.2 de
`94-seguranca-threat-model.md`. Conta em quarentena impede contratação, não
apenas desembolso — bloquear no fim da jornada seria pior experiência e sinal
tardio de fraude.

### 4.2 Nível ativo

| ID | Regra | Severidade | Parâmetro |
| --- | --- | --- | --- |
| `AST-MAT-001` | Não vencido na data de referência | `BLOCKING` | — |
| `AST-MAT-002` | Prazo até liquidação dentro da faixa do produto | `BLOCKING` | Mín. e máx. em dias |
| `AST-VAL-001` | Valor líquido acima do mínimo por ativo | `BLOCKING` | Valor |
| `AST-TYP-001` | Tipo de ativo suportado pelo parceiro | `BLOCKING` | Lista |
| `AST-ARR-001` | Arranjo de pagamento aceito pelo produto | `BLOCKING` | Lista |
| `AST-ACQ-001` | Credenciador aceito pelo produto | `BLOCKING` | Lista |
| `AST-ENC-001` | Sem efeito incompatível conhecido | `BLOCKING` | Tipos incompatíveis |
| `AST-ENC-002` | Cobertura de gravame da fonte é completa | `BLOCKING` | — |
| `AST-FRS-001` | Posição dentro da validade do SLA da fonte | `BLOCKING` | SLA por fonte |
| `AST-RSV-001` | Não reservado por outra contratação na plataforma | `BLOCKING` | — |
| `AST-STL-001` | Não liquidado nem baixado | `BLOCKING` | — |
| `AST-ADJ-001` | Sem ajuste pendente que altere o valor materialmente | `WARNING` | Percentual |

**`AST-ENC-002` merece destaque**: se a fonte declarou cobertura de gravame
`PARTIAL` ou `UNAVAILABLE` (ver `92-schemas-eventos.md`, 3.2), a plataforma não
sabe se o ativo está livre. Tratar como elegível é exatamente o que a invariante
do PRD de Portfolio proíbe: "ausência de informação não significa ausência de
gravame". A regra torna a invariante executável.

**Consequência a aceitar**: parceiro que não informa gravames de forma completa
inviabiliza operação com seus ativos. Isso é correto, e a alternativa — operar às
cegas — transfere o risco para o financiador sem que ele saiba.

### 4.3 Nível lote

| ID | Regra | Severidade | Parâmetro |
| --- | --- | --- | --- |
| `LOT-VAL-001` | Valor total entre o mínimo e o máximo do produto | `BLOCKING` | Faixa |
| `LOT-CNT-001` | Quantidade de ativos dentro do limite operacional do parceiro | `BLOCKING` | Máximo |
| `LOT-CNC-001` | Concentração por sacado abaixo do limite | `BLOCKING` | Percentual |
| `LOT-CNC-002` | Concentração por data de liquidação abaixo do limite | `WARNING` | Percentual |
| `LOT-CNC-003` | Concentração por credenciador abaixo do limite | `WARNING` | Percentual |
| `LOT-CNC-004` | Concentração por arranjo abaixo do limite | `INFO` | Percentual |
| `LOT-TRM-001` | Prazo médio ponderado dentro da faixa | `BLOCKING` | Faixa em dias |
| `LOT-HOM-001` | Lote homogêneo quanto ao tipo de ativo, quando o produto exigir | `BLOCKING` | — |

### 4.4 Cálculo de concentração

O PRD exige limites de concentração (req. 5) sem definir a fórmula. Definição:

```text
concentração(dimensão, valor) = soma(valorLíquido dos ativos com dimensão = valor)
                                / soma(valorLíquido de todos os ativos do lote)
```

Sempre sobre valor líquido, nunca sobre quantidade. Um lote com mil ativos
pequenos de um sacado e dez grandes de outro tem concentração baixa por contagem e
alta por valor — e o risco segue o valor.

**Tratamento de sacado desconhecido**: em recebíveis de cartão não há sacado
identificável; a dimensão equivalente é o credenciador. `LOT-CNC-001` não se
aplica a `CardReceivableUnit`; `LOT-CNC-003` sim. A aplicabilidade por tipo de
ativo faz parte da definição da regra, não de condicional no código de avaliação.

## 5. Justificativa legível

O PRD exige (req. 2) sem definir formato. Estrutura de `RuleResult`:

```json
{
  "ruleId": "AST-MAT-002",
  "ruleVersion": 3,
  "severity": "BLOCKING",
  "outcome": "FAIL",
  "reasonCode": "MATURITY_OUT_OF_RANGE",
  "explanation": "A liquidação ocorre em 412 dias; este produto aceita até 180 dias.",
  "observed": { "termDays": 412 },
  "expected": { "minTermDays": 1, "maxTermDays": 180 },
  "remediable": false,
  "remediationHint": null
}
```

| Campo | Papel |
| --- | --- |
| `reasonCode` | Estável, para agrupamento e métrica |
| `explanation` | Texto para o usuário, com os valores concretos |
| `observed` / `expected` | Torna a explicação verificável, não apenas afirmada |
| `remediable` | Distingue o que o usuário pode corrigir do que não pode |
| `remediationHint` | Ação concreta quando `remediable` |

**`remediable` é a distinção que muda a experiência.** "Consentimento expirado" é
corrigível em um clique. "Ativo vencido" não é. Apresentar as duas exclusões da
mesma forma faz o usuário procurar solução onde não existe — ou desistir de onde
existe.

**Regra de redação da `explanation`**: contém o valor observado e o esperado, em
linguagem do usuário, sem jargão interno e sem identificador técnico. Texto
genérico ("ativo inelegível") não cumpre o requisito.

## 6. Reserva

O PRD cria `Reservation` (req. 7) e a invariante "reserva interna não produz
efeito perante terceiros", sem definir a semântica de concorrência.

### 6.1 Escopo

A reserva impede que **a mesma plataforma** ofereça o mesmo ativo em duas
contratações simultâneas. Não impede nada fora dela. O cedente pode antecipar o
mesmo recebível em outro lugar, e só o registro na infraestrutura evita a dupla
negociação.

**Consequência**: a revalidação antes do registro do efeito (req. 8) não é
redundância defensiva — é o único controle real contra dupla negociação externa.
A reserva é conveniência interna.

### 6.2 Mecânica

| Aspecto | Definição |
| --- | --- |
| Granularidade | Por ativo, não por lote — ativo pode sair do lote sem liberar os demais |
| Criação | Na criação da solicitação, não na simulação |
| Duração | Igual à janela de oferta, prorrogada na seleção até o desfecho da contratação |
| Extensão máxima | Limite absoluto; além dele, exige nova avaliação |
| Liberação | Por conclusão, cancelamento, expiração ou invalidação |
| Concorrência | Inserção com restrição de unicidade sobre `(assetId, status=ACTIVE)` — o banco resolve a corrida, não a aplicação |
| Expiração | Processo periódico e verificação preguiçosa na leitura; não depender só do processo |

**Restrição de unicidade parcial e não bloqueio de aplicação**: duas solicitações
simultâneas sobre o mesmo ativo produzem violação de restrição na segunda, que
recebe `ASSET_ALREADY_RESERVED`. Resolver em aplicação exigiria serializar, o que
não escala e falha com múltiplas instâncias.

**Verificação preguiçosa**: reserva expirada que o processo periódico ainda não
limpou não pode bloquear nova solicitação. A leitura considera `expiresAt`, não
apenas `status`.

## 7. Invalidação e propagação

O PRD produz `EligibilityInvalidated` (req. 6) sem definir gatilhos nem alcance.

| Gatilho | Alcance da invalidação |
| --- | --- |
| Novo snapshot com mudança material nos ativos avaliados | Avaliações que referenciam o snapshot anterior e incluem os ativos alterados |
| Consentimento revogado ou expirado | Todas as avaliações da organização |
| Dossiê cadastral expirado ou revalidação exigida | Todas as avaliações da organização |
| Representação revogada | Avaliações cuja contratação dependa daquela representação |
| Capacidade de parceiro suspensa | Avaliações e lotes destinados ao parceiro |
| Produto expirado | Idem |
| Política de elegibilidade alterada | **Nenhuma** — ver abaixo |

**Política alterada não invalida avaliação existente.** Invariante do PRD: "regra
alterada não modifica avaliação histórica". Avaliações em curso seguem com a
política vigente na criação. A nova política se aplica a avaliações novas. O
contrário produziria invalidação em massa a cada ajuste de parâmetro.

**Propagação**: `EligibilityInvalidated` é consumido por Marketplace (encerra
pedido aberto), Contracting (bloqueia avanço) e pela interface (comunica). A
invalidação após a assinatura e antes do registro é o caso mais delicado —
tratado em `docs/design/09-design-effects-orchestrator.md`.

## 8. Simulação

O PRD exige simulação sem reserva (req. 9) sem definir a relação com a avaliação.

| Aspecto | Simulação | Avaliação |
| --- | --- | --- |
| Cria reserva | não | sim |
| Cria snapshot publicado | não | sim, ou reusa o vigente |
| Persistida | sim, com retenção curta | sim, com retenção plena |
| Vale para criar solicitação | não | sim |
| Consome cota de descoberta | sim, se disparar descoberta | sim |
| Gera evento | não | sim |

**Por que a simulação não fundamenta solicitação**: entre simular e solicitar, a
posição muda. Permitir criar solicitação a partir de simulação reintroduziria
exatamente o que a invariante do Portfolio proíbe — "impedir que uma projeção
antiga fundamente nova contratação".

**Implicação de produto**: o usuário simula e, ao solicitar, o resultado pode
diferir. A interface precisa antecipar isso, não apresentar como erro.

## 9. Avaliação e desempenho

Para um lote de 5.000 ativos e 21 regras, a avaliação ingênua faz 105.000
execuções. Estrutura proposta:

1. **Pré-filtro em consulta**: regras de ativo expressáveis como predicado de
   banco (vencimento, valor, tipo, arranjo, credenciador) filtram na consulta ao
   snapshot. Reduz o conjunto antes de qualquer execução em memória.
2. **Avaliação em memória** apenas do conjunto reduzido, para regras que exigem
   contexto (gravames, reservas, ajustes).
3. **Agregação para regras de lote**, em passagem única que acumula todas as
   dimensões de concentração.
4. **Registro agregado por `reasonCode`**, não por ativo — 3.000 ativos excluídos
   pela mesma razão produzem uma linha com contagem, não 3.000 linhas.

**Alvo**: avaliação de 5.000 ativos em menos de 2 s (p95), conforme
`93-requisitos-nao-funcionais.md`, 3.1.

**Cuidado com o passo 4**: agregar resultado por razão é suficiente para exibir e
para métrica, mas o detalhe por ativo é necessário quando o usuário pergunta "por
que este ativo específico?". Solução: agregado sempre; detalhe por ativo sob
demanda, recalculado com a mesma versão de política.

## 10. Lacunas remanescentes

| # | Lacuna | Impacto |
| --- | --- | --- |
| E8 | Parâmetros concretos (faixas, percentuais) dependem de negociação comercial | Alto |
| E9 | Tipos de efeito incompatíveis (`AST-ENC-001`) dependem do mapeamento regulatório | Alto — ver `docs/reference/regulacao-brasil.md` |
| E10 | SLA de validade da posição por fonte (`AST-FRS-001`) desconhecido | Alto — ver `docs/reference/provedores-integracao.md` |
| E11 | Extensão máxima de reserva não definida | Médio |
| E12 | Tratamento de invalidação após assinatura | Alto — ver `09-design-effects-orchestrator.md` |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | 3 | Versionamento sem mecanismo |
| 3.2 | 1 | Nível de aplicação não distinguido |
| 4 | 1, 5 | Catálogo exemplificativo |
| 4.4 | 5 | Concentração sem fórmula |
| 5 | 2 | Justificativa sem formato |
| 6 | 7 | Reserva sem semântica de concorrência |
| 7 | 6 | Invalidação sem gatilho nem alcance |
| 8 | 9 | Simulação sem relação com avaliação |
| 9 | — | Desempenho não tratado |
