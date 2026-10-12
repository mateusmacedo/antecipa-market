# Design — KYB/KYC

> **PRD de origem**: `docs/prd/02-prd-kyb-kyc.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| Y1 | Nenhuma fonte de dados nomeada | Escopo inteiro |
| Y2 | "Completude do dossiê" sem critério de cálculo | Req. 5 |
| Y3 | Divergência entre fontes sem regra de classificação | Req. 4 |
| Y4 | Revalidação sem gatilhos nem prazos | Req. 7 |
| Y5 | Compartilhamento em subconjunto sem definição dos subconjuntos | Req. 8 |
| Y6 | Decisões divergentes entre financiadores sem tratamento | Req. 9 |
| Y7 | Beneficiário final sem critério de identificação | `OwnershipStructure` |

## 2. O limite de responsabilidade

O PRD acerta ao colocar fora de escopo a "aprovação final de PLD/FTP" e a
"comunicação a autoridades em nome do financiador". Construtivamente isso
significa:

- a plataforma **coleta, verifica e organiza** evidência;
- a plataforma **não decide** conformidade;
- a plataforma **não comunica** a autoridades pelo financiador;
- o estado `READY_FOR_PARTNER` significa completude técnica, jamais aprovação.

**Ponto a apurar**: se a plataforma é ela própria obrigada por norma de PLD/FT, o
limite muda — passa a ter obrigações próprias, incluindo comunicação. Isso depende
do seu enquadramento regulatório e está em `docs/reference/regulacao-brasil.md`.
Até a apuração, o modelo assume que não é obrigada e permanece auxiliar.

**Risco de a suposição estar errada**: se a plataforma for obrigada e o sistema não
previr comunicação de operação suspeita, há descumprimento. O desenho deve manter
o ponto de extensão — `RiskSignal` com classificação de suspeição existe; falta
apenas o fluxo de comunicação, que pode ser acrescentado sem refazer o modelo.

## 3. Verificações

`Check` é definido no PRD como "validação individual com resultado e validade",
sem catálogo. Catálogo proposto, por categoria:

### 3.1 Existência e situação cadastral

| ID | Verificação | Fonte típica | Validade |
| --- | --- | --- | --- |
| `CHK-REG-001` | Inscrição existente e ativa | Fonte oficial de cadastro | Curta |
| `CHK-REG-002` | Atividade econômica compatível com o produto | Idem | Média |
| `CHK-REG-003` | Data de início da atividade | Idem | Longa |
| `CHK-REG-004` | Capital social | Idem | Média |
| `CHK-REG-005` | Endereço declarado confere com o cadastro | Idem | Média |

### 3.2 Estrutura societária

| ID | Verificação | Validade |
| --- | --- | --- |
| `CHK-OWN-001` | Quadro societário obtido | Média |
| `CHK-OWN-002` | Beneficiários finais identificados até o critério aplicável | Média |
| `CHK-OWN-003` | Sócios pessoa jurídica desdobrados até pessoa natural | Média |
| `CHK-OWN-004` | Nenhum sócio em situação impeditiva | Curta |

### 3.3 Representantes

| ID | Verificação | Validade |
| --- | --- | --- |
| `CHK-REP-001` | Identidade do representante verificada | Longa |
| `CHK-REP-002` | Poderes comprovados por documento | Conforme o documento |
| `CHK-REP-003` | Documento de poderes vigente | Conforme o documento |

### 3.4 Listas restritivas

| ID | Verificação | Validade |
| --- | --- | --- |
| `CHK-LST-001` | Listas sancionatórias internacionais | Curta |
| `CHK-LST-002` | Listas de restrição nacionais | Curta |
| `CHK-LST-003` | Exposição política | Curta |
| `CHK-LST-004` | Mídia adversa | Curta |

**As listas específicas dependem de apuração normativa** — quais são obrigatórias
e quais são boa prática está em `docs/reference/regulacao-brasil.md`.

### 3.5 Conta bancária

| ID | Verificação | Validade |
| --- | --- | --- |
| `CHK-BNK-001` | Conta pertence à organização | Curta |
| `CHK-BNK-002` | Conta ativa | Curta |

**`CHK-BNK-001` é controle antifraude, não apenas cadastral**
(`94-seguranca-threat-model.md`, C2.3). Sua ausência abre o vetor de maior perda
esperada.

## 4. Completude

Req. 5 exige calcular completude "separada da aprovação do parceiro", sem
definir o cálculo.

```text
completude(produto) = verificações obrigatórias do produto com resultado válido
                      / total de verificações obrigatórias do produto
```

| Regra | Definição |
| --- | --- |
| Conjunto obrigatório | Definido por produto e parceiro, em `PartnerAgreement` |
| Resultado válido | Executado, com resultado conclusivo e dentro da validade |
| Divergência bloqueante | Reduz a completude a zero — não é média ponderada |
| Verificação opcional | Não entra no denominador |
| `READY_FOR_PARTNER` | Completude igual a 1 e nenhuma divergência bloqueante |

**Divergência bloqueante zera a completude em vez de reduzi-la
proporcionalmente**: um dossiê 95% completo com divergência societária não
bloqueante não existe — ou a divergência é bloqueante e o dossiê não segue, ou não
é e não afeta a completude. Percentual intermediário criaria a tentação de
prosseguir com "quase pronto".

**Completude é por parceiro.** Um cedente pode estar completo para o financiador A
e incompleto para B, que exige verificação adicional. O estado é por par
organização-parceiro, não global — o que o PRD implica ao separar completude de
aprovação, sem tornar explícito.

## 5. Divergências

Req. 4 exige identificar divergências "sem sobrescrever silenciosamente", sem
regra de classificação.

| Classe | Exemplo | Efeito |
| --- | --- | --- |
| `BLOCKING` | Situação cadastral irregular; sócio em lista sancionatória; representante sem poderes | Impede `READY_FOR_PARTNER` |
| `MATERIAL` | Divergência de quadro societário entre fontes; endereço divergente | Exige resolução ou justificativa |
| `INFORMATIONAL` | Grafia de razão social; formatação de endereço | Registra; não bloqueia |

**Nenhuma fonte sobrescreve outra.** Cada `Evidence` mantém sua origem, instante e
hash. A visão consolidada de `BusinessProfile` é derivada por regra de precedência
declarada — e a precedência é dado, não código:

```text
PrecedenceRule
  ├─ field
  ├─ orderedSources[]
  └─ conflictBehavior: USE_HIGHEST_PRECEDENCE | FLAG_DISCREPANCY
```

**Para campos críticos, `FLAG_DISCREPANCY`**: quadro societário divergente entre
fontes não se resolve escolhendo a mais confiável. Vira divergência material, com
resolução documental.

## 6. Revalidação

Req. 7 exige revalidar por expiração, alteração societária ou evento de risco, sem
prazos nem mecanismo de detecção.

| Gatilho | Detecção | Escopo |
| --- | --- | --- |
| Expiração de verificação | Prazo por tipo de verificação | Apenas as expiradas |
| Alteração societária | Reconsulta periódica; comparação de estrutura | Bloco societário |
| Mudança de representante | Evento interno | Bloco de representantes |
| Inclusão em lista restritiva | Monitoramento periódico | Bloco de listas |
| Alteração de conta bancária | Evento interno | `CHK-BNK-001` |
| Exigência do parceiro | Evento externo | Conforme a exigência |
| Tempo desde a última revisão completa | Periodicidade por nível de risco | Dossiê inteiro |

**Revalidação é incremental.** Reexecutar tudo a cada gatilho é caro (cada consulta
tem custo) e desnecessário — a métrica "reutilização de evidências ainda válidas"
do próprio PRD reconhece isso.

**Alteração material invalida avaliações dependentes** (invariante): o evento
`ReverificationRequired` é consumido por Eligibility, que invalida avaliações da
organização (`06-design-eligibility.md`, 7).

**Impacto em operação em curso**: alteração societária detectada entre a assinatura
e o registro do efeito. Proposta: não interrompe operação com contrato assinado —
registra, notifica os financiadores envolvidos e bloqueia novas operações. A
decisão sobre operação em curso é do financiador, não da plataforma.

## 7. Compartilhamento

Req. 8 exige compartilhar subconjunto conforme consentimento e política, sem
definir os subconjuntos. Alinhado aos níveis de divulgação de
`91-contratos-api.md`, 4.1:

| Nível | Conteúdo | Requer |
| --- | --- | --- |
| `SCREENING` | Situação cadastral, atividade, tempo de atividade, porte, completude, sinais de risco — **sem identificar a organização** | Consentimento de compartilhamento |
| `ANALYSIS` | Identificação, quadro societário, beneficiários finais, resultado das verificações | Consentimento + solicitação registrada do parceiro |
| `CONTRACTING` | Dossiê completo, com documentos | Consentimento + oferta selecionada |

**Escalonamento registrado**: cada transição de nível gera evento auditado e é
visível ao cedente. O cedente pode ver quem acessou o quê e quando — requisito
implícito da LGPD, ausente do PRD.

**Documentos nunca trafegam no evento** (`92-schemas-eventos.md`, seção 2,
convenção 2): o evento carrega `EvidenceReference`; o parceiro autorizado resolve
por API com registro de acesso.

## 8. Decisões divergentes

Req. 9 registra decisão de cada financiador "sem transformá-la em decisão global",
sem dizer o efeito prático.

| Situação | Efeito |
| --- | --- |
| A aprova, B reprova | Cedente elegível para A, inelegível para B |
| Todos reprovam | Cedente sem financiador disponível; **não** é bloqueio da plataforma |
| A reprova por motivo cadastral | Sinal para revisão do dossiê; não bloqueia B |
| A reprova por apetite | Informação comercial; não afeta o dossiê |

**Distinção entre motivo cadastral e motivo de apetite**: a primeira indica
problema no dossiê e merece revisão; a segunda é decisão comercial e não diz nada
sobre o cedente. `PartnerDecision` precisa carregar a categoria do motivo, não
apenas o resultado.

**Reprovação não é comunicada com o motivo do parceiro ao cedente sem autorização
deste.** O motivo pode conter avaliação interna do financiador. A comunicação ao
cedente é de que não houve oferta, não do porquê — salvo quando o financiador
autorizar.

## 9. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| Y8 | Enquadramento da plataforma como obrigada de PLD/FT (2) | **Alto** |
| Y9 | Critério de identificação de beneficiário final | Alto |
| Y10 | Listas obrigatórias | Alto |
| Y11 | Provedores e custos (3) | Alto |
| Y12 | Prazos de validade por verificação (6) | Médio |
| Y13 | Viabilidade da validação de titularidade de conta (`CHK-BNK-001`) | Alto |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 3 | 3 | `Check` sem catálogo |
| 4 | 5 | Completude sem cálculo |
| 5 | 4 | Divergência sem classificação |
| 6 | 7 | Revalidação sem gatilho nem prazo |
| 7 | 8 | Subconjuntos sem definição |
| 8 | 9 | Decisões divergentes sem efeito prático |
