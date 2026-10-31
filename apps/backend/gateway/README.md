# gateway

Bounded context `receivables-gateway`, gerado pelo generator `bounded-context`
(`tools/dmpf-plugin`): um módulo Go, um package por bloco.

Projeto Nx `gateway`, tags `type:app`, `scope:backend`, `stack:go` e
`layer:apps` — a camada é a do estágio mais alto entre os blocos, porque é
o primeiro em que toda dependência do módulo está de pé. Import path do módulo:
`github.com/mateusmacedo/antecipa-market/apps/backend/gateway`.

## Blocos e unidades

| Package | Bloco | Unidade | Resumo |
| --- | --- | --- | --- |
| `domain` | `domain` | `receivables-gateway/domain` | Agregados síncronos e determinísticos, com o desfecho da UPR — sem porta, relógio, contexto nem tipo de wire. |
| `ports` | `port` | `receivables-gateway/ports` | Fronteira declarada — repositórios, outbox e inbox em tipos do domínio, sem nenhuma realização. |
| `application` | `application` | `receivables-gateway/application` | Lado do chamador da UPR: casos de uso de escrita na sequência canônica e a disposição do consumo. |
| `provider` | `provider` | `receivables-gateway/provider-postgres` | Realização em PostgreSQL das portas transacionais do contexto, com esquema e mapeadores próprios. |
| `app` | `app` | `receivables-gateway/app` | Composição entre a entrega do transporte e o serviço de aplicação, com a borda gRPC do contexto. |

`block` e `bounded_context` vêm das opções do generator e vão literalmente ao
`dmpf-units.json` — nunca são inferidos do nome do diretório (ADR-012).

## Borda, banco e configuração

O contexto serve só gRPC (`company.gateway.service.v1.GatewayService`), atrás da cadeia de
interceptors do kernel; o REST público é do `bff` (ADR-044, ADR-053). O
`ServiceDesc` de `app/rpc/service.go` declara 17 métodos, um por comando e por
consulta da primeira fatia da SPEC-N1X148AW: os nove comandos de conector,
perfil, autorização de consulta e comando externo, a varredura
`SweepExternalCommands` (chamada por agendador externo) e sete consultas. Os dez
métodos de comando estão em `rpc.Commands()` e exigem `idempotency-key`.

O banco é próprio: database `gateway`, cuja dona é a role migradora
`gateway_migrator`, e role de runtime `gateway`, sem posse (ADR-063);
schema `public`, tabelas sem prefixo e agregado no plural. O DDL vive em
`provider/schema.sql` e é aplicado no ready do `api` quando `MIGRATE=true`, pela
migradora de `PG_MIGRATE_DSN`, junto com a outbox e a inbox do kernel, seguido
dos grants do kernel e de `provider.Grants`; sem `MIGRATE`, o ready espera que
a role de runtime tenha esses grants. Toda tabela do `schema.sql` precisa de um
grant em `provider.Grants` (`postgres.Mutable` para a que um `Table` escreve),
ou a partida é recusada.
O agregado cujo estado inteiro vive na coluna `snapshot` declara a tabela com
`postgres.SnapshotTable`, sobre um struct de estado privado do provider cujas
tags JSON fixam os bytes gravados:

```go
var orderTable = postgres.SnapshotTable("orders", "order_id",
	func(s domain.Snapshot) orderState { return orderState{Status: int(s.Status)} },
	func(state orderState) domain.Snapshot { return domain.Snapshot{Status: domain.Status(state.Status)} },
	func(s domain.Snapshot, id domain.OrderID) domain.Snapshot { s.ID = id; return s },
)
```

Todo comando passa pela inbox, com `consumer_name` `gateway.commands`
(`application.CommandConsumer`), na mesma transação do efeito (FND-04 §7.6,
ADR-056). O generator não escreve casos de uso: cabe a quem escreve o contexto
passar cada comando por `usecase.Execute`, que o corre em `usecase.RunIdempotent`,
e declarar o método em `rpc.Commands()`, que o interceptor usa para exigir a
metadata `idempotency-key`.
O `kernelgrpc.StatusOf`, com que os handlers devolvem o erro, já traduz a
divergência, o comando em andamento e a chave ausente.
Os prazos (`IdempotencyWait` 1s, `IdempotencyRetention` 24h, `OutboxRetention`
168h, `PurgeInterval` 15min, `PurgeBatch` 1000) são a seção `Policies` da
`Config` (`kernelapp.Policies`) e vêm de `Defaults()`; o `api` purga as entradas
de comando vencidas e o `relay`, a outbox publicada.

A configuração vem só do ambiente, sem prefixo `DMPF_`: `PG_DSN`, `GRPC_ADDR`
(`:9090`), `GRPC_INSECURE` ou o trio mTLS (`GRPC_TLS_CERT_FILE`,
`GRPC_TLS_KEY_FILE`, `GRPC_CLIENT_CA_FILE` e `GRPC_TRUSTED_CLIENTS`); no
relay, `KAFKA_BROKERS`, `KAFKA_GATEWAY_TOPIC`, `KAFKA_GATEWAY_DLQ`,
`KAFKA_GATEWAY_DISPATCH_TOPIC`, `KAFKA_GATEWAY_DISPATCH_DLQ` e `KAFKA_GROUP`; no
consumer, que lê `CapabilityEnabled` e `CapabilitySuspended` de
`partner.events`, `KAFKA_BROKERS`, `KAFKA_PARTNER_TOPIC`, `KAFKA_PARTNER_DLQ` e
`KAFKA_GROUP`. No `api`, os timeouts de sessão `lock_timeout` (2s) e
`idle_in_transaction_session_timeout` (30s) entram no `PG_DSN` quando o ambiente
não os define, porque o papel lê a série sob a guarda do ADR-067. As variáveis `GRPC_*` formam a seção `API` da `Config`, lida por
`kernelgrpc.ReadAPIEnv`; o `requirements()` compõe o `Missing()` dela com o do
Kafka e com as variáveis do contexto, e a falta de uma variável obrigatória
recusa a partida nomeando-a. O `PG_MIGRATE_DSN` só é lido no `api` com
`MIGRATE=true`, que o exige. Os dois vêm do `deploy/.env.migrator.example`, que
só o `serve-api` e o `docker:run` carregam (este pela cópia
`deploy/.env.migrator`, que o `deploy-env` cria): o `deploy/.env.example` vale
também para o `relay`, e a credencial da migradora não chega a ele.

## Resolução de dependências

O módulo importa o próprio contrato (`gateway/contract`) e o do Partner
(`partner/contract`, pelos eventos que o consumer lê). O `require` de cada irmão
é gravado pelo `dmpf-modsync`, que você roda de novo a cada import novo:

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/modsync@v1.0.0-rc.7 --root . --write
```

Dentro do workspace quem resolve é o `go.work` da raiz, onde o generator
registrou a entrada `use` e o modsync mantém o `replace` versionado; fora dele,
o `require` resolve pela tag publicada do irmão. O CI confere os dois lados com
`--check`.

## Antes do primeiro commit

Unidade nova muda a classificação: regrave o baseline do verificador.

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@v1.0.0-rc.7 --root . --write-baseline
```
