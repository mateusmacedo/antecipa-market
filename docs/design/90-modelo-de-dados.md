# Modelo de Dados

> **Camada**: construtiva
> **Origem**: lacuna identificada — os PRDs listam agregados por nome
> (`Organization`, `PortfolioSnapshot`, `RegulatoryEffect`…) sem um único
> atributo, tipo, cardinalidade ou chave. Não é possível construir schema,
> API ou evento a partir deles.
> **Status**: proposta

## 1. Convenções

### 1.1 Identificadores

| Regra | Definição |
| --- | --- |
| Chave primária | Identificador opaco, imutável, gerado pela plataforma, ordenável por tempo de criação |
| Formato | Prefixo do tipo + separador + valor. Ex.: `org_01HN4K…`, `eff_01HN4K…` |
| Identificador fiscal | Nunca é chave primária. `01-prd-identity-organization.md`, invariante: "identificadores fiscais não são usados como chave técnica mutável" |
| Identificador externo | Nunca é chave primária. Modelado como `ExternalReference`, com cardinalidade N |

**Razão do prefixo**: identificador aparece em log, evidência, suporte e dossiê de
auditoria. Um identificador que se autodescreve elimina uma classe inteira de erro
operacional — colar o ID de contrato onde se esperava o de efeito falha de imediato
em vez de retornar "não encontrado".

### 1.2 Valores monetários

```json
{ "currency": "BRL", "amountMinor": 150000 }
```

Inteiro em unidade menor (centavos). Ponto flutuante é proibido em qualquer
representação de dinheiro, incluindo cálculo intermediário. Percentuais e taxas
usam decimal de precisão fixa, com a escala declarada por campo.

**Definido em** `14-catalogo-contratos-eventos.md`; reafirmado aqui porque a
proibição de ponto flutuante não está escrita lá e é fonte recorrente de
divergência de centavos em conciliação.

### 1.3 Tempo

Conforme `docs/design/00-arquitetura-tecnica.md`, seção 8:

| Sufixo do campo | Tipo | Exemplo |
| --- | --- | --- |
| `…At` | Instante UTC, precisão de milissegundo | `signedAt` |
| `…Date` | Data pura, sem fuso | `maturityDate` |
| `asOf` | Instante ao qual a informação se refere | `asOf` em posição externa |

### 1.4 Classificação de dados

Toda coluna recebe uma classificação, que determina criptografia, mascaramento
em log, presença em evento e retenção.

| Classe | Descrição | Em log | Em payload de evento | Criptografia |
| --- | --- | --- | --- | --- |
| `PUBLIC` | Sem restrição | Sim | Sim | Padrão |
| `INTERNAL` | Operacional, não pessoal | Sim | Sim | Padrão |
| `CONFIDENTIAL` | Comercial sensível (taxa, oferta, volume) | Mascarado | Sim, com controle de destinatário | Padrão |
| `PII` | Dado pessoal identificável | Mascarado | Referência, não valor | Envelope adicional |
| `PII_SENSITIVE` | Documento, biometria, dado de conta | Nunca | Nunca | Envelope adicional + chave segregada |

**Regra derivada de `14-catalogo`, regra 7** ("payload não transporta documentos
sensíveis; usa referências autorizadas"): campos `PII_SENSITIVE` nunca entram em
evento. O evento carrega `EvidenceReference`, e o consumidor autorizado resolve.

### 1.5 Versionamento

Três padrões distintos, aplicados conforme a classe do dado (ver
`00-arquitetura-tecnica.md`, 5.2):

| Padrão | Aplicação | Mecânica |
| --- | --- | --- |
| Mutável com histórico de auditoria | `ContractCase`, `FundingRequest` | Atualização in-place; histórico no ledger |
| Versionado explícito | `ConsentGrant`, `EligibilityPolicy`, `FinancingOffer` | Nova linha por versão; `version` e `supersedesId` |
| Snapshot imutável | `PortfolioSnapshot` | Nunca alterado após publicação |

---

## 2. Tipos compartilhados

### 2.1 `ExternalReference`

Vínculo entre objeto interno e sua identidade em sistema externo. Um objeto tem
N referências (a mesma duplicata pode existir no escriturador e na registradora).

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `providerId` | ID | sim | `INTERNAL` | Referência a `Partner` |
| `system` | enum | sim | `INTERNAL` | `REGISTRY`, `BOOKKEEPER`, `ACQUIRER`, `FUNDER`, `DEPOSITORY` |
| `referenceType` | enum | sim | `INTERNAL` | `ASSET`, `EFFECT`, `AUTHORIZATION`, `COMMAND`, `SETTLEMENT` |
| `reference` | string | sim | `INTERNAL` | Valor exato do parceiro, sem normalização |
| `asOf` | instante | sim | `INTERNAL` | Quando a referência foi obtida |
| `observedVersion` | string | não | `INTERNAL` | Versão do objeto no parceiro, se fornecida |

**Invariante**: `reference` é preservado exatamente como recebido. Normalização
(trim, upper, remoção de máscara) produz campo derivado adicional, nunca substitui
o original — `04-prd-receivables-gateway.md`: "payload normalizado nunca elimina o
payload original".

### 2.2 `PartyRef`

| Campo | Tipo | Obrigatório | Classe |
| --- | --- | --- | --- |
| `organizationId` | ID | não | `INTERNAL` |
| `taxIdType` | enum | sim | `INTERNAL` |
| `taxIdHash` | hash | sim | `INTERNAL` |
| `taxIdMasked` | string | sim | `PII` |
| `legalName` | string | não | `PII` |

**Razão do hash**: permite correspondência e detecção de duplicidade sem manter o
identificador em claro em todo lugar. O valor completo vive apenas no agregado
proprietário, com classe `PII`.

**Cuidado**: o espaço de CNPJ é pequeno e enumerável. Hash sem `salt` secreto não
protege contra reversão por força bruta. O `salt` é segredo de aplicação, único
por ambiente, e rotacioná-lo exige reprocessar os hashes — decisão a registrar.

### 2.3 `MonetaryAmount`, `Percentage`, `EvidenceReference`

```json
{ "currency": "BRL", "amountMinor": 150000 }
{ "value": "0.0235", "scale": 4, "basis": "MONTHLY" }
{
  "evidenceId": "evd_…",
  "contentHash": "sha256:…",
  "contentType": "application/pdf",
  "storageUri": "…",
  "accessPolicyId": "pol_…"
}
```

`EvidenceReference` nunca carrega o conteúdo. `contentHash` é obrigatório e
calculado sobre os bytes originais, antes de qualquer transformação.

---

## 3. Agregados por contexto

Detalhamento dos agregados com maior densidade de lacuna. Os demais seguem as
convenções acima.

### 3.1 Identity & Organization

#### `Organization`

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `tenantId` | ID | sim | `INTERNAL` | |
| `kind` | enum | sim | `INTERNAL` | `CLIENT`, `FUNDER`, `PARTNER`, `PLATFORM` |
| `jurisdiction` | string | sim | `INTERNAL` | ISO 3166-1 alfa-2 |
| `taxIdType` | enum | sim | `INTERNAL` | `CNPJ`, `CPF` |
| `taxId` | string | sim | `PII` | Sem máscara |
| `taxIdHash` | hash | sim | `INTERNAL` | Índice único com `jurisdiction` |
| `legalName` | string | sim | `PII` | |
| `tradeName` | string | não | `PII` | |
| `status` | enum | sim | `INTERNAL` | `ACTIVE`, `SUSPENDED`, `OFFBOARDED` |
| `createdAt` | instante | sim | `INTERNAL` | |

**Unicidade**: `(taxIdHash, jurisdiction)` — fecha o requisito 1 do PRD
("cadastrar organização sem duplicidade por identificador fiscal e jurisdição").

**Lacuna resolvida**: o PRD não diz o que acontece quando a mesma empresa é
cliente e financiador. Com `kind` como atributo único, seria preciso duplicar a
organização. Proposta: `kind` vira conjunto, não valor único — uma organização
pode acumular papéis, e a autorização avalia o papel no contexto da ação. Isso
evita duas identidades para a mesma pessoa jurídica.

#### `Representation`

Agregado central para a invariante "uma ação em nome da empresa referencia uma
`Representation` válida".

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `organizationId` | ID | sim | `INTERNAL` | |
| `personId` | ID | sim | `INTERNAL` | |
| `powerType` | enum | sim | `INTERNAL` | `LEGAL_REPRESENTATIVE`, `ATTORNEY_IN_FACT`, `OPERATOR` |
| `sourceType` | enum | sim | `INTERNAL` | `ARTICLES_OF_ASSOCIATION`, `POWER_OF_ATTORNEY`, `BOARD_RESOLUTION`, `INTERNAL_DELEGATION` |
| `sourceEvidenceId` | ID | sim | `INTERNAL` | Aponta para `Evidence` |
| `scope` | objeto | não | `INTERNAL` | Limite de valor ou de tipo de ato, quando o documento impõe |
| `validFrom` | instante | sim | `INTERNAL` | |
| `validUntil` | instante | não | `INTERNAL` | Nulo = indeterminado |
| `verificationStatus` | enum | sim | `INTERNAL` | `DECLARED`, `VERIFIED`, `REJECTED` |
| `revokedAt` | instante | não | `INTERNAL` | |
| `revocationReason` | string | não | `INTERNAL` | |

**Lacuna crítica que este modelo expõe**: o campo `scope`. Procurações
frequentemente limitam poderes por valor ou por tipo de ato ("poderes para
representar em operações de crédito até R$ X"). Nenhum PRD trata disso. Se o
sistema não modela o limite, assina operação acima do poder conferido — e o
contrato pode ser questionado.

**Requisito derivado**: `08-prd-contracting.md` requisito 4 ("confirmar poderes dos
signatários no instante da assinatura") precisa incluir verificação de limite de
alçada, não só de vigência.

**Invariante adicional proposta**: uma `Representation` não é alterada. Mudança de
poderes revoga a existente e cria nova. Isso preserva a prova de qual poder
existia em cada instante — necessária para `12-prd-compliance-ledger.md`.

### 3.2 Portfolio

#### `PortfolioSnapshot`

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `tenantId` | ID | sim | `INTERNAL` | |
| `portfolioId` | ID | sim | `INTERNAL` | |
| `discoveryId` | ID | sim | `INTERNAL` | Execução que o originou |
| `sourceReferences` | `ExternalReference[]` | sim | `INTERNAL` | Uma por fonte consultada |
| `asOf` | instante | sim | `INTERNAL` | Instante da posição, informado pela fonte |
| `createdAt` | instante | sim | `INTERNAL` | Instante de criação local |
| `assetCount` | inteiro | sim | `INTERNAL` | |
| `grossTotal` | `MonetaryAmount` | sim | `CONFIDENTIAL` | |
| `contentHash` | hash | sim | `INTERNAL` | Sobre o conjunto ordenado de ativos |
| `publishedAt` | instante | não | `INTERNAL` | Nulo = em construção |
| `supersedesId` | ID | não | `INTERNAL` | |

**Invariante**: após `publishedAt` preenchido, nenhuma coluna muda. Imposto por
permissão de banco, não por convenção.

**Uso do `contentHash`**: resolve a lacuna de "alteração material" identificada em
`00-arquitetura-tecnica.md`, 5.3. Se o hash do conjunto normalizado for igual ao
do snapshot anterior, não se cria versão. A normalização (quais campos entram no
hash) é a decisão a tomar — incluir `asOf` faria todo snapshot ser diferente.

**Proposta de normalização**: entram no hash os campos que afetam decisão de
antecipação — identificador do ativo, valor líquido esperado, data de liquidação,
titularidade, situação e gravames conhecidos. Ficam fora: `asOf`, metadados de
consulta, ordem dos itens.

#### `CardReceivableUnit`

O ativo de maior volume do sistema.

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `snapshotId` | ID | sim | `INTERNAL` | |
| `externalReferences` | `ExternalReference[]` | sim | `INTERNAL` | Registradora, credenciador |
| `holderRef` | `PartyRef` | sim | `PII` | Titular original |
| `acquirerId` | ID | sim | `INTERNAL` | Credenciador |
| `subAcquirerId` | ID | não | `INTERNAL` | |
| `paymentScheme` | string | sim | `INTERNAL` | Arranjo de pagamento |
| `settlementDate` | data | sim | `INTERNAL` | Data prevista de liquidação |
| `grossAmount` | `MonetaryAmount` | sim | `CONFIDENTIAL` | |
| `adjustments` | objeto[] | não | `CONFIDENTIAL` | Tipo, valor e origem |
| `netAmount` | `MonetaryAmount` | sim | `CONFIDENTIAL` | Após ajustes conhecidos |
| `knownEncumbrances` | objeto[] | não | `CONFIDENTIAL` | Efeitos informados pela fonte |
| `availableAmount` | `MonetaryAmount` | não | `CONFIDENTIAL` | Derivado; ver invariante |
| `sourceAsOf` | instante | sim | `INTERNAL` | |
| `staleAfter` | instante | sim | `INTERNAL` | Calculado pelo SLA da fonte |

**Invariante crítica**, já presente no PRD e que o modelo precisa tornar
inevitável: "ausência de informação não significa ausência de gravame".
Construtivamente, `availableAmount` é nulo — e não igual a `netAmount` — quando a
fonte não informa gravames de forma completa. Um campo nulo força tratamento; um
campo preenchido com valor otimista produz operação sobre ativo indisponível.

**Requisito derivado**: `CapabilityProfile` do parceiro precisa declarar se a
consulta de agenda retorna gravames de forma completa, parcial ou não retorna.
Isso não está em `13-prd-partner-management.md`.

### 3.3 Marketplace

#### `FinancingOffer`

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `fundingRequestId` | ID | sim | `INTERNAL` | |
| `requestVersion` | inteiro | sim | `INTERNAL` | Versão do pedido a que responde |
| `funderOrganizationId` | ID | sim | `INTERNAL` | |
| `productId` | ID | sim | `INTERNAL` | |
| `originalPayload` | JSON | sim | `CONFIDENTIAL` | Exatamente como recebido |
| `originalPayloadHash` | hash | sim | `INTERNAL` | |
| `pricing` | `PricingBreakdown` | sim | `CONFIDENTIAL` | Normalizado |
| `coveredAssetIds` | ID[] | sim | `INTERNAL` | |
| `conditions` | objeto | não | `CONFIDENTIAL` | Coobrigação, regresso, garantias |
| `validUntil` | instante | sim | `INTERNAL` | |
| `receivedAt` | instante | sim | `INTERNAL` | |
| `status` | enum | sim | `INTERNAL` | `RECEIVED`, `PRESENTED`, `SELECTED`, `EXPIRED`, `REJECTED_AS_INVALID`, `WITHDRAWN` |

**Invariante do PRD**: "ordenação nunca altera os valores originais". O modelo
separa `originalPayload` (intocado) de `pricing` (derivado) — e `originalPayloadHash`
permite provar que a normalização não adulterou a proposta.

#### `PricingBreakdown`

Fecha a lacuna mais relevante do Marketplace: o PRD exige "calcular métricas
comparáveis sem reprecificar a operação" sem definir quais métricas.

| Campo | Tipo | Obrigatório | Observação |
| --- | --- | --- | --- |
| `grossAmount` | `MonetaryAmount` | sim | Valor de face cedido |
| `discountAmount` | `MonetaryAmount` | sim | Deságio |
| `feeItems` | objeto[] | não | Tarifa: código, descrição, valor, quem cobra |
| `taxItems` | objeto[] | não | Tributo: código, base, valor |
| `netAmount` | `MonetaryAmount` | sim | Valor líquido ao cedente |
| `weightedAverageTermDays` | inteiro | sim | Prazo médio ponderado pelo valor |
| `effectiveMonthlyRate` | `Percentage` | sim | Métrica comparável primária |
| `totalCostAmount` | `MonetaryAmount` | sim | `grossAmount - netAmount` |
| `calculationMethodId` | ID | sim | Versão do método usado |
| `platformRemuneration` | objeto | não | Remuneração da plataforma, se houver |

**Definição de `effectiveMonthlyRate`** (proposta):

```text
effectiveMonthlyRate = (grossAmount / netAmount)^(30 / weightedAverageTermDays) - 1
```

Onde `weightedAverageTermDays` é a média dos prazos de cada ativo, ponderada pelo
valor de face, contada da data prevista de crédito até a data de liquidação de
cada ativo.

**Por que essa métrica e não outra**: deságio isolado não é comparável entre
ofertas com prazos diferentes; taxa nominal não captura tarifas. A taxa efetiva
mensal, calculada sobre o líquido efetivamente recebido, é a única que permite
comparar propostas de estruturas distintas. `calculationMethodId` versiona o
método, atendendo a "a apresentação pode ser reconstruída exatamente para
auditoria".

**Ponto aberto**: verificar se há exigência regulatória de divulgação de CET para
tomador pessoa jurídica. Ver `docs/reference/regulacao-brasil.md`. Se houver, o
CET passa a ser campo obrigatório, não métrica proprietária.

**Campo `platformRemuneration`**: torna verificável a invariante "remuneração
comercial não pode ser escondida no ranking". Se o valor existe, é exibido.

### 3.4 Effects Orchestrator

#### `RegulatoryEffect`

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `contractId` | ID | sim | `INTERNAL` | |
| `effectType` | enum | sim | `INTERNAL` | Ver tipos em `09-prd` |
| `beneficiaryRef` | `PartyRef` | sim | `INTERNAL` | Financiador |
| `registryPartnerId` | ID | sim | `INTERNAL` | Infraestrutura destinatária |
| `terms` | `EffectTerms` | sim | `CONFIDENTIAL` | |
| `status` | enum | sim | `INTERNAL` | Máquina de `09-prd` |
| `externalReference` | `ExternalReference` | não | `INTERNAL` | Preenchido na confirmação |
| `confirmedAt` | instante | não | `INTERNAL` | |
| `idempotencyKey` | string | sim | `INTERNAL` | Ver `00-arquitetura-tecnica.md`, 7.3 |

#### `EffectTarget`

O agregado que torna tratável a confirmação parcial — requisito 7 de `09-prd`.

| Campo | Tipo | Obrigatório | Observação |
| --- | --- | --- | --- |
| `id` | ID | sim | |
| `effectId` | ID | sim | |
| `assetRef` | `AssetRef` | sim | |
| `requestedAmount` | `MonetaryAmount` | sim | Total ou parcial sobre o ativo |
| `confirmedAmount` | `MonetaryAmount` | não | Preenchido na confirmação |
| `status` | enum | sim | `PENDING`, `CONFIRMED`, `REJECTED`, `UNKNOWN` |
| `rejectionCode` | string | não | Código original do parceiro |
| `rejectionReason` | string | não | Texto original do parceiro |
| `externalReference` | `ExternalReference` | não | Alguns parceiros referenciam por alvo |

**Invariante do PRD tornada estrutural**: "confirmação parcial divide o resultado
por alvo; não promove o conjunto inteiro". Com o status por alvo, o status do
`RegulatoryEffect` é derivado — `EFFECTIVE` apenas quando todos os alvos estão
`CONFIRMED`; `PARTIALLY_EFFECTIVE` quando há mistura. É impossível representar
"confirmado no agregado, pendente no alvo".

**Consequência para Funding** que os PRDs não desenvolvem: com efeito parcial, o
valor a desembolsar não é o do contrato. `10-prd-funding.md` diz "efeito parcial
exige instrução compatível com a parte confirmada ou revisão contratual" sem
definir qual dos dois. São caminhos distintos:

- **Instrução proporcional** — desembolsa proporcional ao confirmado. Rápido, mas
  o contrato assinado deixa de corresponder à operação executada.
- **Revisão contratual** — aditivo ou novo contrato. Correto juridicamente, lento.

**Proposta**: o `FinancialProduct` do parceiro declara qual caminho aceita, e o
contrato prevê a hipótese desde a assinatura (cláusula de confirmação parcial).
Sem previsão contratual, o caminho é revisão. Decisão que exige validação
jurídica.

### 3.5 Compliance Ledger

#### `AuditRecord`

| Campo | Tipo | Obrigatório | Classe | Observação |
| --- | --- | --- | --- | --- |
| `id` | ID | sim | `INTERNAL` | |
| `sequence` | inteiro | sim | `INTERNAL` | Monotônico por partição |
| `tenantId` | ID | sim | `INTERNAL` | |
| `recordType` | string | sim | `INTERNAL` | Tipo de evento, versionado |
| `subjects` | `AuditSubject[]` | sim | `INTERNAL` | Entidades relacionadas |
| `actor` | objeto | sim | `INTERNAL` | Tipo, id, representação usada |
| `correlationId` | ID | sim | `INTERNAL` | |
| `causationId` | ID | não | `INTERNAL` | |
| `occurredAt` | instante | sim | `INTERNAL` | Relógio do produtor |
| `recordedAt` | instante | sim | `INTERNAL` | Relógio do ledger |
| `payloadHash` | hash | sim | `INTERNAL` | Do payload original |
| `payloadRef` | `EvidenceReference` | não | `INTERNAL` | Quando o payload é preservado |
| `previousRecordHash` | hash | sim | `INTERNAL` | Encadeamento |
| `recordHash` | hash | sim | `INTERNAL` | Deste registro |
| `retentionPolicyId` | ID | sim | `INTERNAL` | |
| `legalHoldIds` | ID[] | não | `INTERNAL` | |

**Encadeamento por hash**: `12-prd` exige "detectar sequência ausente, duplicada ou
inválida" (requisito 9) sem dizer como. `previousRecordHash` + `recordHash`
formam cadeia verificável: alteração de qualquer registro invalida todos os
posteriores, e a verificação é aritmética, não confiança no armazenamento.

**Partição**: encadear globalmente serializa toda escrita. Proposta: encadear por
`(tenantId, recordType)`, com `sequence` por partição. A verificação de
integridade percorre partições independentemente.

**Decisão aberta**: publicar periodicamente o hash da cabeça de cada partição em
meio externo (notarização) elevaria o valor probatório, ao custo de dependência
externa. Decisão de produto e jurídica, não de engenharia.

---

## 4. Diagrama de relacionamento essencial

```text
Organization 1---N Establishment
Organization 1---N Representation N---1 Person
Organization 1---N Membership N---1 UserAccount
Organization 1---N ConsentGrant 1---N ConsentScope
Organization 1---1 Portfolio 1---N PortfolioSnapshot 1---N FinancialAsset
                                                          |
PortfolioSnapshot 1---N EligibilityEvaluation 1---N RuleResult
EligibilityEvaluation 1---N EligibleLot 1---N Reservation
EligibleLot 1---1 FundingRequest 1---N PartnerInvitation
FundingRequest 1---N FinancingOffer 1---0..1 OfferSelection
OfferSelection 1---1 ContractCase 1---N ContractInstrument
ContractCase 1---1 AssetSchedule 1---N (FinancialAsset)
ContractCase 1---N SignatureEnvelope
ContractCase 1---N RegulatoryEffect 1---N EffectTarget
RegulatoryEffect 1---0..1 FundingInstruction 1---N Disbursement
FinancialAsset 1---N SettlementExpectation 1---N SettlementOccurrence
SettlementExpectation N---N ReconciliationItem N---1 ReconciliationRun
ReconciliationItem N---1 ReconciliationCase
(todos) ---> AuditRecord
```

**Cardinalidade que merece atenção**: `RegulatoryEffect` é N por `ContractCase`.
Um contrato pode gerar efeitos em infraestruturas diferentes — a duplicata no
escriturador e o recebível de cartão na registradora. Tratar como 1:1 quebra no
primeiro contrato multiativo.

---

## 5. Lacunas remanescentes

| # | Lacuna | Impacto | Onde resolver |
| --- | --- | --- | --- |
| L1 | Campos canônicos da UR conforme padrão regulatório | Alto — define o modelo de ingestão | `docs/reference/regulacao-brasil.md` + `provedores-integracao.md` |
| L2 | Campos da duplicata escritural conforme norma | Alto | idem |
| L3 | Limite de alçada em `Representation` (3.1) | Alto — risco jurídico | Validação jurídica |
| L4 | Caminho para efeito parcial (3.4) | Alto | Validação jurídica |
| L5 | Normalização do `contentHash` de snapshot (3.2) | Médio — custo de armazenamento | Decisão de engenharia |
| L6 | Obrigatoriedade de CET para PJ (3.3) | Médio | `docs/reference/regulacao-brasil.md` |
| L7 | Notarização externa do ledger (3.5) | Baixo | Decisão de produto |
| L8 | Rotação do `salt` de hash de identificador fiscal (2.2) | Médio | Decisão de engenharia |

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 1 | Todos | Convenções inexistentes |
| 2 | `14-catalogo-contratos-eventos.md` | Tipos compartilhados sem atributos |
| 3.1 | `01-prd-identity-organization.md` | Atributos; limite de alçada; acúmulo de papéis |
| 3.2 | `05-prd-portfolio.md` | Atributos; "alteração material"; gravame desconhecido |
| 3.3 | `07-prd-marketplace.md` | Métrica de comparabilidade indefinida |
| 3.4 | `09-prd-effects-orchestrator.md`, `10-prd-funding.md` | Confirmação parcial sem caminho definido |
| 3.5 | `12-prd-compliance-ledger.md` | Detecção de adulteração sem mecanismo |
