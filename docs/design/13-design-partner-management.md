# Design — Partner Management

> **PRD de origem**: `docs/prd/13-prd-partner-management.md`
> **Camada**: construtiva
> **Status**: proposta
> **Complementa**: `04-design-receivables-gateway.md` 2 (`CapabilityProfile`).
> Não repetido aqui.

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| N1 | Processo de homologação sem critérios | Estado `HOMOLOGATING` |
| N2 | `CommercialRule` sem conteúdo | Modelo de domínio |
| N3 | Rollout gradual sem mecanismo | Req. 5 |
| N4 | Plano de saída sem procedimento | Req. 10 |
| N5 | "Capacidade efetiva" sem definição de resolução | Req. 8 |
| N6 | Registro de mudança de schema sem fluxo | Req. 7 |
| N7 | Suspensão granular sem definição do que continua | Req. 6 |

## 2. Critérios de homologação

O PRD define `HOMOLOGATING` como estado sem dizer o que se homologa. Sem
critérios, a transição para `ACTIVE` é decisão subjetiva — e a invariante
"capacidade ativa exige contrato, homologação e configuração válidos" fica sem
conteúdo.

### 2.1 Critérios técnicos por operação

| # | Critério | Bloqueante |
| --- | --- | --- |
| H1 | Operação executada com sucesso em homologação | sim |
| H2 | Idempotência verificada: reenvio com a mesma chave não duplica | sim |
| H3 | **Consulta de estado disponível e verificada** | **sim, para operações de escrita** |
| H4 | Tratamento de erro mapeado à taxonomia canônica | sim |
| H5 | Comportamento sob timeout verificado | sim |
| H6 | Limites documentados e verificados | sim |
| H7 | Calendário e horário de corte documentados | sim |
| H8 | Qualidade de dado declarada e verificada por amostragem | sim |
| H9 | Callback com verificação de origem funcionando | sim, se aplicável |
| H10 | Procedimento de revogação de credencial acordado e testado | sim |
| H11 | Canal de suporte e prazo de resposta acordados | sim |
| H12 | Contato para incidente definido | sim |

**H3 é o critério que mais falta nos PRDs.** Sem consulta de estado, todo
`UNKNOWN_OUTCOME` com aquele parceiro vira trabalho manual irrecuperável
automaticamente (`04-design-receivables-gateway.md`, 2.2). Homologar operação de
escrita sem consulta pareada é aceitar dívida operacional permanente.

**Se o parceiro não oferece consulta de estado**: a operação pode ser homologada
com restrição declarada, e o `CapabilityProfile` registra
`stateQuerySupported: false`. A consequência — todo timeout vira caso humano —
passa a ser decisão consciente, não descoberta em produção.

**H10 e H12 são frequentemente esquecidos** e só se percebe a falta durante o
incidente, quando é tarde (`94-seguranca-threat-model.md`, C5.7).

### 2.2 Critérios contratuais e de conformidade

| # | Critério |
| --- | --- |
| H13 | Contrato assinado e vigente |
| H14 | Responsabilidades e limites definidos |
| H15 | Tratamento de dados pessoais acordado, com papéis definidos |
| H16 | Situação regulatória verificada, quando aplicável |
| H17 | Modelo comercial acordado e parametrizado |
| H18 | SLA acordado |

## 3. Regra comercial

`CommercialRule` é definido como "remuneração e impacto permitido na apresentação"
sem estrutura. Depende das decisões de `18-design-billing.md`, seção 2.

```text
CommercialRule
  ├─ ruleId, version, effectiveFrom, effectiveUntil
  ├─ remunerationModel: NONE | PER_OPERATION | SUBSCRIPTION | MIXED
  ├─ payer: CEDENT | FUNDER | BOTH
  ├─ calculationBasis, rates[]
  ├─ triggerEvent
  ├─ presentationImpact
  │    ├─ sponsoredPlacementAllowed
  │    ├─ maxSponsoredPositions
  │    └─ canOverrideMerit        <- decisão de produto (07-design-marketplace, R6)
  └─ disclosureRequired           <- sempre verdadeiro
```

**`disclosureRequired` sempre verdadeiro**: não é configurável. Decorre da
invariante "remuneração comercial não pode ser escondida no ranking". Um campo
configurável admitiria a hipótese de não divulgar.

**Invariante a impor**: `CommercialRule` não altera valor de oferta — apenas
posição e divulgação. Verificável: o `PricingBreakdown` normalizado não referencia
`CommercialRule` em nenhum cálculo.

## 4. Capacidade efetiva

Req. 8 exige disponibilizar "capacidade efetiva" aos demais contextos, sem
definir a resolução.

```text
capacidadeEfetiva(partner, operation, assetType, tenant, instante) =
       contrato vigente no instante
  AND  capacidade habilitada e homologada
  AND  produto vigente
  AND  ausência de suspensão da operação
  AND  tenant incluído no rollout
  AND  parâmetros compatíveis
```

| Requisito | Definição |
| --- | --- |
| Consulta | Síncrona, com cache curto invalidado por evento |
| Propagação de suspensão | Menos de 30 s |
| Padrão em indisponibilidade da consulta | **Negar** |
| Auditoria | Toda negação registra a condição que falhou |

**Negar em caso de dúvida**: se a capacidade não pode ser confirmada, a operação
não prossegue. Permitir por padrão levaria a enviar comando a parceiro suspenso —
com consequência externa irreversível.

## 5. Suspensão granular

Req. 6 exige suspender operação específica sem desativar o parceiro, sem definir o
que continua.

| Escopo da suspensão | Continua funcionando |
| --- | --- |
| Uma operação | Todas as demais operações do parceiro |
| Um produto | Demais produtos; operações de infraestrutura |
| Um tipo de ativo | Demais tipos |
| Parceiro inteiro | Nada de novo; acompanhamento do existente permanece |

**Regra fundamental** (invariante do PRD): suspensão bloqueia **novos** comandos e
nunca interrompe o acompanhamento dos existentes. Um efeito submetido antes da
suspensão continua sendo consultado até o desfecho — abandoná-lo deixaria a
operação em estado indefinido no mundo real.

| Em curso no momento da suspensão | Tratamento |
| --- | --- |
| Comando enviado, aguardando confirmação | Acompanhamento continua |
| Contrato assinado, efeito não submetido | Bloqueado; caso operacional |
| Pedido aberto com convite ao parceiro | Convite encerrado; demais seguem |
| Efeito vigente | Permanece; liberação continua possível |

## 6. Rollout gradual

Req. 5 exige rollout por tenant e produto, sem mecanismo.

```text
RolloutPolicy
  ├─ capabilityId
  ├─ mode: DISABLED | ALLOWLIST | PERCENTAGE | ENABLED
  ├─ allowlistTenantIds[]
  ├─ percentage
  └─ excludedTenantIds[]
```

| Requisito | Definição |
| --- | --- |
| Determinismo | Um tenant em `PERCENTAGE` permanece do mesmo lado entre avaliações |
| Exclusão | `excludedTenantIds` prevalece sobre qualquer modo |
| Reversão | Redução de exposição tem efeito em menos de 30 s |
| Operação em curso | Não é interrompida por redução de rollout |
| Auditoria | Mudanças registradas no ledger |

**Operação em curso não é interrompida**: reduzir o rollout impede novas
operações, não cancela as existentes. Cancelar operação com contrato assinado por
mudança de configuração interna seria inaceitável.

## 7. Mudança de contrato de integração

Req. 7 exige registrar mudança de schema, certificado e endpoint, sem fluxo.

| Tipo | Detecção | Fluxo |
| --- | --- | --- |
| Mudança anunciada | Comunicação do parceiro | Registro; nova versão; homologação; janela de convivência; ativação |
| Mudança não anunciada | `ExternalSchemaMismatchDetected` | Suspensão automática da operação; caso; contato; homologação |
| Certificado a vencer | Monitoramento | Alerta em 60, 30 e 7 dias; coordenação |
| Endpoint alterado | Anunciada ou por falha | Registro; nova versão de perfil; validação |

**Mudança não anunciada suspende automaticamente a operação afetada.** Continuar
operando com contrato quebrado produz interpretação errada de dado — pior que
indisponibilidade, porque é silencioso.

## 8. Descontinuidade

Req. 10 exige plano de saída e tratamento de operações em curso, sem procedimento.

```text
OFFBOARDING
  1. Encerrar novos pedidos e comandos (suspensão de capacidade)
  2. Inventariar operações em curso, por estado
  3. Concluir o que é concluível
  4. Tratar individualmente o que não é
  5. Garantir liberação de efeitos quando as obrigações se cumprirem
  6. Preservar evidências conforme retenção
  7. Encerrar credenciais
  8. Registrar encerramento
INACTIVE
```

| Categoria | Tratamento |
| --- | --- |
| Pedido aberto | Encerrar convites |
| Contrato assinado sem efeito | Cancelar ou concluir, conforme acordo |
| Efeito vigente | **Permanece** até a liberação; exige acesso residual |
| Liquidação em curso | Acompanhar até conciliar |
| Evidências | Preservadas independentemente do offboarding |

**Acesso residual é o ponto crítico**: um efeito vigente numa infraestrutura da
qual se está saindo precisa ser liberado quando a obrigação se cumprir. Isso exige
manter capacidade de comunicação com aquela infraestrutura depois do encerramento
comercial — e precisa estar previsto no contrato de saída, não improvisado.

**`INACTIVE` só é alcançável quando não houver efeito vigente pendente de
liberação.** Invariante a impor.

## 9. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| N8 | Critérios de homologação precisam ser acordados com cada parceiro | Alto |
| N9 | Modelo comercial depende de `18-design-billing.md` G1-G3 | **Bloqueante** para o módulo comercial |
| N10 | Situação regulatória a verificar por tipo de parceiro | Alto |
| N11 | Cláusula de acesso residual no contrato de saída | Alto |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | Estado `HOMOLOGATING` | Homologação sem critérios |
| 3 | `CommercialRule` | Regra comercial sem conteúdo |
| 4 | 8 | Capacidade efetiva sem resolução |
| 5 | 6 | Suspensão sem definição do que continua |
| 6 | 5 | Rollout sem mecanismo |
| 7 | 7 | Mudança sem fluxo |
| 8 | 10 | Saída sem procedimento |
