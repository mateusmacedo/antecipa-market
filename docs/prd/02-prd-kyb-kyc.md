# PRD — KYB/KYC

## Propósito

Produzir um dossiê cadastral verificável de organizações, representantes e beneficiários finais, reutilizável pelos financiadores sem transferir à plataforma a decisão regulatória ou de crédito.

## Escopo

- Coleta e validação de dados empresariais.
- Situação cadastral, atividade econômica e endereço.
- Quadro societário e beneficiários finais.
- Identificação e validação de representantes.
- Listas restritivas e sinais de risco configurados pelo parceiro.
- Renovação periódica e por evento.

## Fora do escopo

- Aprovação final de PLD/FTP.
- Rating de crédito.
- Decisão de financiamento.
- Comunicação a autoridades em nome do financiador.

## Modelo de domínio

- `VerificationCase`: processo versionado de verificação.
- `BusinessProfile`: dados cadastrais consolidados.
- `OwnershipStructure`: sócios, controladores e beneficiários finais.
- `Evidence`: documento ou retorno de fonte.
- `Check`: validação individual com resultado e validade.
- `RiskSignal`: sinal informativo, nunca decisão automática da plataforma.
- `PartnerDecision`: decisão recebida do financiador.

## Requisitos funcionais

1. Criar caso de verificação por organização e finalidade.
2. Coletar somente dados necessários ao produto e aos parceiros selecionados.
3. Versionar cada evidência com fonte, instante e hash.
4. Identificar divergências entre fontes sem sobrescrever silenciosamente.
5. Calcular completude do dossiê, separada da aprovação do parceiro.
6. Permitir solicitação e resposta de pendências.
7. Revalidar dados por expiração, alteração societária ou evento de risco.
8. Compartilhar subconjunto do dossiê conforme consentimento e política do parceiro.
9. Registrar a decisão de cada financiador sem transformá-la em decisão global.

## Estados

`OPEN → COLLECTING → VALIDATING → PENDING_INFORMATION → READY_FOR_PARTNER → PARTNER_REVIEW → APPROVED | REJECTED | EXPIRED`

## Invariantes

- `READY_FOR_PARTNER` significa completude técnica, não conformidade aprovada.
- Um parceiro não acessa evidências destinadas exclusivamente a outro.
- Resultado expirado não é usado sem política explícita de tolerância.
- Alterações materiais invalidam avaliações dependentes.

## Comandos e eventos

Comandos: `OpenVerificationCase`, `AttachEvidence`, `RunCheck`, `RequestInformation`, `SubmitToPartner`, `RecordPartnerDecision`, `ExpireVerification`.

Eventos: `VerificationOpened`, `EvidenceCollected`, `DiscrepancyDetected`, `VerificationReady`, `PartnerApproved`, `PartnerRejected`, `ReverificationRequired`.

## Critérios de aceitação

- O dossiê informa o que foi verificado, quando, por qual fonte e com qual resultado.
- Divergências impedem avanço quando classificadas como bloqueantes.
- O financiador recebe apenas os dados autorizados e necessários.
- A decisão final permanece atribuída ao financiador.

## Métricas

- Tempo até dossiê completo.
- Casos com pendência documental.
- Taxa de divergência por fonte.
- Reutilização de evidências ainda válidas.
- Reprovações por documentação incompleta.

## Dependências

Identity & Organization fornece sujeitos e representantes. Consent & Mandates autoriza compartilhamento. Partner Management define exigências. Compliance Ledger preserva evidências e decisões.

