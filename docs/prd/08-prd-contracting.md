# PRD — Contracting

## Propósito

Formalizar a operação entre cedente e financiador, garantindo correspondência entre oferta aceita, ativos revalidados, documentos assinados e efeito regulatório solicitado.

## Escopo

- Geração ou recepção do instrumento contratual do financiador.
- Validação de dados contra a oferta selecionada.
- Revalidação de poderes, consentimentos e ativos.
- Assinatura eletrônica e evidências.
- Dossiê contratual imutável.
- Entrega do comando ao Effects Orchestrator.

## Modelo de domínio

- `ContractCase`: processo de formalização.
- `ContractInstrument`: documento e metadados.
- `ContractParty`: cedente, financiador, garantidor e representantes.
- `AssetSchedule`: ativos e condições alcançados.
- `SignatureEnvelope`: signatários, ordem e evidências.
- `ContractEvidenceBundle`: pacote final reproduzível.

## Requisitos funcionais

1. Criar contrato somente a partir de oferta válida selecionada.
2. Validar divergências de taxa, valor, prazo, partes e ativos.
3. Reconsultar disponibilidade antes da assinatura final quando exigido.
4. Confirmar poderes dos signatários no instante da assinatura.
5. Preservar documento apresentado e documento assinado por hash.
6. Suportar contratos sobre ativos específicos e recebíveis futuros.
7. Impedir alteração do instrumento após primeira assinatura; mudanças criam nova versão.
8. Coletar aceite explícito de condições materiais.
9. Disponibilizar cópia às partes.
10. Encaminhar ao registro somente após conclusão válida.

## Estados

`PREPARING → PENDING_VALIDATION → READY_FOR_SIGNATURE → SIGNING → SIGNED → SUBMITTED_FOR_EFFECT`

Exceções: `MISMATCH`, `EXPIRED`, `REJECTED`, `CANCELLED`, `SIGNATURE_FAILED`.

## Invariantes

- Partes do contrato são cedente e financiador; a plataforma atua conforme seu papel contratado.
- Documento assinado não é sobrescrito.
- Ativos e parâmetros enviados ao registro correspondem ao contrato assinado.
- Assinatura sem poder vigente não conclui o caso.
- Taxas e encargos não podem divergir silenciosamente da oferta selecionada.

## Eventos

`ContractCaseCreated`, `ContractValidated`, `ContractMismatchDetected`, `SignatureRequested`, `ContractSigned`, `ContractCancelled`, `ContractSubmittedForEffect`.

## Critérios de aceitação

- O dossiê contém oferta, revalidação, contrato, assinaturas e hashes.
- Qualquer divergência material exige nova apresentação e aceite.
- O usuário consegue baixar o instrumento firmado.
- O comando regulatório deriva de dados versionados do contrato.

## Métricas

- Tempo de formalização.
- Abandono durante assinatura.
- Divergências entre oferta e instrumento.
- Contratos invalidados por mudança no ativo.

## Dependências

Marketplace fornece a seleção. Identity & Organization valida poderes. Portfolio e Eligibility revalidam ativos. Effects Orchestrator registra os efeitos. Compliance Ledger preserva o dossiê.

