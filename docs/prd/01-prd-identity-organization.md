# PRD — Identity & Organization

## Propósito

Manter a identidade de pessoas, empresas, estabelecimentos, representantes e usuários da plataforma, garantindo isolamento por tenant e prova dos poderes utilizados em cada ação.

## Escopo

- Cadastro de pessoa jurídica e seus estabelecimentos.
- Usuários e autenticação federada.
- Representantes legais, procuradores e operadores.
- Papéis e permissões por organização.
- Relação entre matriz, filiais e estabelecimentos recebedores.
- Isolamento de dados e ações por tenant.

Ficam fora do contexto a verificação cadastral aprofundada, tratada por KYB/KYC, e os mandatos transacionais, tratados por Consent & Mandates.

## Modelo de domínio

- `Organization`: pessoa jurídica cliente, parceiro ou financiador.
- `Establishment`: unidade operacional identificada por CNPJ/CPF e referências externas.
- `Person`: pessoa natural identificada de maneira protegida.
- `Representation`: vínculo entre pessoa, organização, papel, fonte e período de validade.
- `UserAccount`: identidade autenticável.
- `Membership`: acesso do usuário a uma organização.
- `Role` e `Permission`: autorização funcional.

## Requisitos funcionais

1. Cadastrar organização sem duplicidade por identificador fiscal e jurisdição.
2. Manter múltiplos estabelecimentos associados à mesma organização.
3. Vincular usuário somente após convite e validação de identidade.
4. Registrar origem, validade e evidência dos poderes de representação.
5. Permitir segregação entre operador, aprovador, administrador e auditor.
6. Exigir dupla aprovação para alteração de dados bancários e representantes, quando configurado.
7. Bloquear imediatamente acessos revogados sem apagar o histórico.
8. Disponibilizar visão efetiva das permissões de um usuário em determinado instante.

## Invariantes

- Todo recurso de negócio pertence a exatamente um tenant.
- Identificadores fiscais não são usados como chave técnica mutável.
- Uma ação em nome da empresa referencia uma `Representation` válida.
- Remoção de usuário não remove evidências anteriores.
- Papéis de plataforma e papéis de cliente permanecem separados.

## Comandos e eventos

Comandos: `RegisterOrganization`, `AddEstablishment`, `InviteUser`, `GrantMembership`, `RegisterRepresentation`, `RevokeRepresentation`, `SuspendUser`.

Eventos: `OrganizationRegistered`, `EstablishmentAdded`, `MembershipGranted`, `RepresentationVerified`, `RepresentationRevoked`, `UserSuspended`.

## APIs principais

- `POST /organizations`
- `POST /organizations/{id}/establishments`
- `POST /organizations/{id}/memberships`
- `POST /organizations/{id}/representations`
- `GET /users/{id}/effective-permissions`

## Segurança e privacidade

- MFA obrigatório para usuários privilegiados.
- Autorização combinando tenant, recurso, papel e representação.
- Dados pessoais minimizados e mascarados em logs.
- Sessões revogáveis e registro de autenticações de risco.
- Proibição de acesso operacional cruzado entre financiadores.

## Critérios de aceitação

- Um usuário sem vínculo ativo não acessa nenhum recurso da organização.
- A plataforma identifica quem agiu, por qual organização e sob qual poder.
- A revogação de representação impede novos consentimentos e contratos.
- O histórico permanece consultável para auditoria temporal.

## Métricas

- Tentativas de acesso cruzado bloqueadas.
- Representações expiradas usadas em tentativas de ação.
- Tempo para revogação efetiva de acesso.
- Organizações duplicadas detectadas.

## Dependências

KYB/KYC valida a organização e os representantes. Consent & Mandates utiliza as representações válidas. Compliance Ledger recebe todas as mudanças relevantes.

