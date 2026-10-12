# Design — Funding

> **PRD de origem**: `docs/prd/10-prd-funding.md`
> **Camada**: construtiva
> **Status**: proposta

## 1. Lacunas do PRD que este documento fecha

| # | Lacuna | Onde aparece no PRD |
| --- | --- | --- |
| D1 | Validação do destino sem mecanismo | Req. 2 |
| D2 | Efeito parcial delegado sem decisão | Invariante |
| D3 | "Controle reforçado" para mudança de conta sem definição | Invariante |
| D4 | Comprovante sem requisitos de conteúdo | Req. 9 |
| D5 | Estorno sem tratamento do efeito constituído | `REVERSED` |
| D6 | Cancelamento "quando permitido" sem critério | Escopo |

## 2. Posição da plataforma

O PRD é claro e correto: sem saldo, sem conta de pagamento, sem execução de
transferência, sem custódia. A plataforma emite **autorização lógica** e registra
**confirmação informada** pelo financiador.

Consequências construtivas:

1. `Disbursement` é registro de fato informado por terceiro, nunca execução.
2. A plataforma não confirma pagamento por conta própria — depende do retorno do
   financiador (req. 6).
3. A plataforma não aparece como destino em nenhuma circunstância (invariante).
4. A divergência entre o ordenado e o confirmado é detectada e registrada, não
   corrigida — corrigir exigiria mover recursos.

**Invariante a impor mecanicamente**: `DisbursementDestination` nunca pode
referenciar conta de titularidade da plataforma. Verificação na validação do
destino, não apenas convenção.

## 3. Validação do destino

Req. 2 exige validar que o destino pertence ao cedente, sem mecanismo.

| Camada | Verificação | Confiança |
| --- | --- | --- |
| 1 | Identificador fiscal do titular da conta igual ao da organização | Alta, se verificável por fonte |
| 2 | Confirmação de titularidade por serviço de validação | Alta |
| 3 | Micro-depósito com confirmação de valor | Alta; lento |
| 4 | Declaração do cliente com documento comprobatório | Média; exige análise |
| 5 | Declaração sem comprovação | **Insuficiente** |

**Camada 5 não é validação.** Aceitar conta declarada sem verificação abre o vetor
de maior perda esperada (`94-seguranca-threat-model.md`, T2).

**Exceção prevista no PRD**: "ou é permitido contratualmente" — conta de terceiro
autorizada em contrato. Nesse caso a verificação é da autorização contratual, não
da titularidade, e o registro precisa deixar explícito que se trata de destino de
terceiro. É situação de risco elevado e deve exigir aprovação reforçada.

## 4. Mudança de conta

A invariante exige "controle reforçado e nova validação" sem definir. Definição,
alinhada aos controles C2.1 a C2.7:

| Momento da mudança | Controle |
| --- | --- |
| Sem operação em curso | Dupla aprovação + validação + quarentena + notificação aos contatos anteriores |
| Com operação em curso, antes da assinatura | Idem; operação segue com a conta antiga até a nova sair da quarentena |
| **Após a assinatura, antes do desembolso** | **Bloqueia a operação**; exige reconfirmação por canal independente; não basta aprovação na plataforma |
| Após o desembolso | Não afeta a operação concluída |

**A terceira linha é o controle decisivo.** É a janela em que o contrato está
fechado e o dinheiro não saiu — exatamente onde o atacante age. Aprovar a mudança
e prosseguir seria aprovar o desvio.

## 5. Efeito parcial

A invariante do PRD delega a decisão sem tomá-la. Resolvida em
`09-design-effects-orchestrator.md`, 5. Do lado de Funding:

| Caminho | Comportamento |
| --- | --- |
| Contrato prevê ajuste proporcional | Instrução emitida sobre o valor confirmado, recalculando deduções proporcionalmente |
| Contrato prevê tudo ou nada | Nenhuma instrução; solicitar liberação do confirmado |
| Contrato omisso | **Nenhuma instrução**; caso operacional; aguardar revisão contratual |

**Instrução nunca é emitida com valor inferido.** Se o caminho não está definido no
contrato, o sistema não escolhe.

**Recálculo proporcional**: deságio e tarifas percentuais acompanham a proporção;
tarifas fixas não. A regra precisa constar do contrato — caso contrário, o valor
líquido recalculado pode divergir do que o cliente esperava, e não há base
contratual para sustentá-lo.

## 6. Ciclo da instrução

```text
PENDING_PREREQUISITES -> READY -> REQUESTED -> PROCESSING -> DISBURSED
```

### 6.1 Pré-condições

Verificadas na transição para `READY`, todas obrigatórias:

| # | Pré-condição |
| --- | --- |
| 1 | Contrato assinado e válido |
| 2 | Efeito regulatório confirmado (`EFFECTIVE`) |
| 3 | Nenhum efeito em `UNKNOWN_OUTCOME` relacionado |
| 4 | Destino validado e fora de quarentena |
| 5 | Nenhum bloqueio de risco vigente |
| 6 | Nenhuma disputa aberta sobre o efeito |

**Reavaliadas na emissão**, não apenas na entrada em `READY`. Entre `READY` e
`REQUESTED` pode haver intervalo, e uma condição pode deixar de valer.

### 6.2 Resultado desconhecido

Mesma disciplina do Gateway: timeout na solicitação produz `UNKNOWN_OUTCOME`, que
exige consulta ao financiador — nunca reenvio. Reenviar pode produzir desembolso
duplicado, cuja recuperação depende de devolução pelo cedente.

**Se o financiador não oferece consulta de estado de desembolso**, toda ambiguidade
vira caso manual. Como no Gateway, isso deveria ser critério de homologação em
`13-prd-partner-management.md`.

## 7. Comprovante

Req. 9 exige disponibilizar comprovante, sem requisitos.

| Elemento | Obrigatório |
| --- | --- |
| Valor efetivamente creditado | sim |
| Data e hora da execução | sim |
| Identificação do financiador pagador | sim |
| Referência da transação no financiador | sim |
| Identificação da conta de destino, mascarada | sim |
| Referência do meio de pagamento | sim, quando houver |
| Vínculo com o contrato | sim |
| Documento do financiador | quando fornecido |

**A referência do meio de pagamento é o que torna o comprovante verificável pelo
cedente** no extrato do seu banco. Sem ela, o comprovante é afirmação da
plataforma sobre fato de terceiro.

**O comprovante não é emitido pela plataforma como se fosse dela.** É o registro de
informação fornecida pelo financiador, e deve deixar isso claro — a plataforma não
movimentou recursos e não pode atestar que o pagamento ocorreu.

## 8. Estorno

`DisbursementReversed` existe sem tratamento do efeito já constituído.

| Situação | Tratamento |
| --- | --- |
| Estorno antes da confirmação | Instrução volta a `READY`, se as pré-condições persistirem |
| Estorno após a confirmação, efeito vigente | **Efeito permanece**; caso crítico; decisão das partes |
| Estorno após a liberação do efeito | Pior caso; sem garantia; jurídico |
| Estorno por erro do financiador | Nova instrução, se as partes concordarem |

**O efeito não é liberado automaticamente por estorno.** A cessão foi constituída;
desfazê-la exige comando de liberação, que depende de decisão das partes e de
confirmação da infraestrutura. Automatizar seria decidir unilateralmente que a
operação não existe — o que a plataforma não pode fazer.

## 9. Cancelamento

O escopo admite "cancelamento antes da execução, quando permitido", sem critério.

| Estado | Cancelável | Por quem |
| --- | --- | --- |
| `PENDING_PREREQUISITES` | sim | Cedente ou financiador |
| `READY` | sim | Cedente ou financiador |
| `REQUESTED` | somente se o financiador confirmar | Financiador |
| `PROCESSING` | não | — |
| `DISBURSED` | não — apenas estorno | — |

**Cancelamento de instrução não cancela o efeito.** Se o efeito está constituído e
o desembolso é cancelado, o cedente tem recebíveis gravados sem ter recebido — a
pior situação possível para ele. Cancelar instrução com efeito vigente exige
solicitação de liberação no mesmo fluxo, e o sistema deve impedir que uma ocorra
sem a outra.

## 10. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| D7 | Serviço de validação de titularidade de conta (3) | **Alto** |
| D8 | Prazo de quarentena de conta (4) | Médio |
| D9 | Financiadores oferecem consulta de estado de desembolso? (6.2) | Alto |
| D10 | Regra de recálculo proporcional (5) exige jurídico | Alto |
| D11 | Formato de comprovante aceito pelos financiadores | Médio |

---

## Rastreabilidade

| Seção | Requisito do PRD | Lacuna fechada |
| --- | --- | --- |
| 2 | Invariante | Proibição de destino próprio sem imposição |
| 3 | 2 | Validação de destino sem mecanismo |
| 4 | Invariante | "Controle reforçado" sem definição |
| 5 | Invariante | Efeito parcial sem decisão |
| 6.1 | 1 | Pré-condições sem lista nem reavaliação |
| 7 | 9 | Comprovante sem requisitos |
| 8 | `REVERSED` | Estorno sem tratamento do efeito |
| 9 | Escopo | "Quando permitido" sem critério |
