# Estados — Organization

```mermaid
stateDiagram-v2
  [*] --> New
  New --> Active : RegisterOrganization aceita
  note right of Active : Suspended e Offboarded existem como valores do enum, mas este app não implementa comandos que façam essas transições.
```

A organização precisa estar `Active` para inclusão de estabelecimento, representação, convite e solicitação de troca bancária. O caso de uso de registro é a única transição de status atualmente implementada.
