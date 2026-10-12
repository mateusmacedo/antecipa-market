# PRD — Consent & Mandates

## Propósito

Controlar autorizações de consulta, compartilhamento e atuação em nome do cedente, com escopo granular, validade, prova e propagação de revogação.

## Conceitos separados

- `DataProcessingBasis`: fundamento para tratamento de dados pessoais.
- `DataSharingConsent`: autorização de compartilhamento quando aplicável.
- `ReceivablesOptIn`: autorização para acesso à agenda de recebíveis.
- `OperationalMandate`: poderes para a plataforma enviar comandos em nome do cliente.

Um aceite de termos não deve representar implicitamente todos esses instrumentos.

## Modelo de domínio

- `ConsentGrant`: autorização versionada.
- `ConsentScope`: ativo, período, dados, finalidade e destinatários.
- `Mandate`: poderes operacionais concedidos.
- `ConsentEvidence`: texto, versão, canal, autenticação e instante.
- `Revocation`: evento e alcance da retirada.
- `ExternalAuthorizationReference`: identificador mantido pela infraestrutura.

## Requisitos funcionais

1. Apresentar finalidade, dados, destinatários e validade antes da concessão.
2. Registrar aceite forte pelo representante com poderes válidos.
3. Permitir consentimento por financiador, tipo de ativo e período.
4. Criar ou refletir o opt-in na infraestrutura competente.
5. Exibir autorizações ativas e histórico ao cedente.
6. Revogar novas consultas sem apagar evidências de operações anteriores.
7. Propagar revogação aos parceiros e acompanhar confirmação.
8. Impedir utilização além do escopo ou após expiração.
9. Renovar autorização sem reutilizar silenciosamente o aceite anterior.

## Estados

`DRAFT → PENDING_ACCEPTANCE → ACTIVE → REVOCATION_PENDING → REVOKED`

Estados adicionais: `EXPIRED`, `REJECTED`, `EXTERNAL_SYNC_FAILED`.

## Invariantes

- Toda consulta de agenda referencia autorização ativa.
- Escopo efetivo é a interseção entre consentimento interno e autorização externa.
- Revogação não desfaz contratos válidos nem obrigações de retenção.
- O texto exibido e o aceite são preservados por hash e versão.
- Um mandato nunca concede à plataforma poderes maiores que os do representante.

## Comandos e eventos

Comandos: `DraftConsent`, `GrantConsent`, `ActivateExternalOptIn`, `RenewConsent`, `RequestRevocation`, `ConfirmRevocation`.

Eventos: `ConsentGranted`, `ExternalOptInActivated`, `ConsentExpired`, `ConsentRevocationRequested`, `ConsentRevoked`, `AuthorizationSyncFailed`.

## APIs principais

- `POST /organizations/{id}/consents`
- `GET /organizations/{id}/consents`
- `POST /consents/{id}/revocations`
- `GET /consents/{id}/evidence`

## Critérios de aceitação

- Consultas sem consentimento ativo são bloqueadas antes da chamada externa.
- É possível provar exatamente o que foi autorizado.
- Revogação fica visível até confirmação de cada destinatário.
- Autorizações expiradas não são renovadas automaticamente.

## Métricas

- Tempo para ativação externa.
- Tempo para propagação de revogação.
- Consultas bloqueadas por escopo inválido.
- Falhas de sincronização por parceiro.

## Dependências

Identity & Organization valida o representante. Partner Management fornece capacidades externas. Receivables Gateway executa opt-ins e consultas. Compliance Ledger preserva a prova.

