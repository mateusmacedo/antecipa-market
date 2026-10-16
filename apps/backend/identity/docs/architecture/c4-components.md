# Componentes internos — identity

Cada chamada percorre as camadas do app; nos comandos, o kernel DMPF coordena a transação e a idempotência. Consultas leem pelo pool de leitura, sem UoW.

```mermaid
C4Component
  title Componentes — caminho da borda ao núcleo

  Container_Boundary(identity, "identity — Go") {
    Component(rpc, "RPC IdentityService", "gRPC/Protobuf", "Valida forma, converte DTOs e mapeia status")
    Component(authz, "Autorização", "app.Authorization", "Mapeia operação para permissão e exige tenant")
    Component(usecases, "Casos de uso", "application.Service", "Executa 11 comandos e 11 consultas")
    Component(domain, "Agregados e regras", "domain", "Decide aceitações, recusas, estados e eventos")
    Component(ports, "Ports", "interfaces Go", "Define repositories, readers e proteção de identificadores")
    Component(provider, "Provider PostgreSQL", "provider", "Implementa repositories, readers e mapeamento de snapshots")
    Component(kernel, "Kernel DMPF", "application / postgres / grpc", "Executor, UoW, inbox, outbox, relay e transporte")
  }
  ContainerDb(db, "PostgreSQL identity", "PostgreSQL", "Tabelas tenant-scoped, snapshots e tabelas DMPF")
  ContainerQueue(kafka, "identity.events", "Kafka", "Eventos já persistidos na outbox")

  Rel(rpc, usecases, "Invoca o caso de uso", "DTO convertido")
  Rel(usecases, authz, "Autoriza pelo tenant", "Permissão da operação")
  Rel(usecases, kernel, "Executa comando ou query", "UoW/idempotência ou leitura")
  Rel(usecases, domain, "Carrega e decide sobre agregado", "Comando de domínio")
  Rel(usecases, ports, "Depende de contratos de persistência e proteção", "Interfaces")
  Rel(provider, ports, "Implementa os contratos", "Repositories/readers")
  Rel(provider, db, "Lê e grava snapshots", "SQL/pgx")
  Rel(kernel, db, "Coordena UoW, inbox e outbox", "Transação PostgreSQL")
  Rel(kernel, kafka, "Relay publica eventos confirmados", "Kafka")
```

## Caminho de execução

1. O servidor RPC valida o formato da requisição; a validação de negócio permanece no domínio.
2. A aplicação autoriza a operação pelo tenant no contexto de execução. A permissão do usuário é resolvida pelo BFF chamador por `FindEffectivePermissions`; o interceptor gRPC deste contexto não monta o sujeito do usuário.
3. Para comandos, `Execute` abre UoW, verifica a inbox pelo consumer `identity.commands` e fingerprint, então chama o caso de uso; `Decide` carrega o agregado, aplica a regra, salva com controle de versão e enfileira eventos na mesma transação.
4. Recusa de domínio é resultado de negócio persistido pela idempotência, sem salvar estado ou evento. Erros operacionais retornam como erro/status gRPC.
5. Consultas usam `Query`/`findRelated`, autorizam e leem pelos readers sem abrir UoW.
6. Readers/repositorios usam `provider`; a proteção HMAC é aplicada pela aplicação antes de lookups/criações de identificadores.

Os detalhes por operação estão nas [22 sequências](./sequences/README.md); as jornadas que compõem mais de uma RPC estão em [journeys](./journeys/README.md).
