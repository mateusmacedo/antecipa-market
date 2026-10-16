# Contexto do sistema — identity

Este diagrama mostra `identity` como serviço interno. O BFF REST é uma borda prevista, mas não implementada neste workspace; o sistema não acessa Keycloak diretamente.

```mermaid
C4Context
  title Contexto do Sistema — identity-organization

  Person(backoffice, "Backoffice", "Atribui o tenant ao provisionar a organização")
  System(identity, "Identity", "Mantém organizações, pessoas, representações, contas, vínculos e eventos")
  System_Ext(callers, "Chamadores internos", "Contextos consumidores; futuro BFF para as rotas REST")
  System_Ext(keycloak, "Keycloak", "Provedor OIDC; autentica o sujeito e fornece o tenant ao chamador")
  System_Ext(kafka, "Kafka", "Recebe os eventos de integração publicados por identity")

  Rel(backoffice, keycloak, "Provisiona organização e atributo tenant_id", "Console/API do provedor")
  Rel(callers, identity, "Consulta e comanda identidade", "gRPC interno; contexto de execução com tenant")
  Rel(keycloak, callers, "Emite identidade federada e claim tenant_id", "OIDC/JWT")
  Rel(identity, kafka, "Publica eventos de integração", "CloudEvents sobre Kafka; canal identity.events")
```

## Limites

- **Implementado:** serviço gRPC; o contexto isola leituras e escritas por tenant e publica os eventos descritos em [eventos](./journeys/outbox-relay.md).
- **Implementado:** outros contextos consomem identidade e organização por gRPC; `identity` não consome eventos.
- **Futuro/fora do escopo:** o BFF e a borda REST indicada pelo OpenAPI. O relacionamento com Keycloak é parte da federação/provisionamento da identidade no chamador; `identity` não faz chamada OIDC direta.
- **Armazenamento e runtime:** detalhados em [containers](./c4-containers.md).
