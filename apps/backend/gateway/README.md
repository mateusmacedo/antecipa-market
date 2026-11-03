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
| `appkit/partnersim` | `app` | `receivables-gateway/partnersim` | Simulador de parceiro (04:G12) atrás das portas de parceiro do contexto; só os testes o importam. |

`block` e `bounded_context` vêm das opções do generator e vão literalmente ao
`dmpf-units.json` — nunca são inferidos do nome do diretório (ADR-012).

## Papéis

O binário vira um de três processos por `--role` (`app.RunWith`); o relay
nunca divide processo com o caminho da requisição (BLK-02).

- **`api`**: abre a loja do bruto (`OpenRawStore`) e só então serve o gRPC por
  `serve.ServeAPI`, com o serviço de `NewGatewayService`. Declara
  `ExternalCommands: true`, então o `MIGRATE=true` migra também o ledger de
  tentativas do kernel (`external_attempt`), onde o `ResolveUnknownOutcome`
  grava a observação. Purga a inbox de comandos e esse ledger.
- **`relay`**: drena a outbox sobre o `RelayCatalog`, com os quatro destinos
  do contexto (`gateway.events`, `gateway.dispatch`, `gateway.interactions` e
  `gateway.normalizations`), porque o `serve.RunRelay` do kernel recebe um
  canal só. Espera os grants da outbox, recusa a partida pelo `AssertOwnOutbox`
  se a outbox tiver destino fora do catálogo e purga a outbox publicada. Não
  abre a loja do bruto.
- **`consumer`**: o `runConsumer` abre a loja do bruto, espera os grants de
  `consumerGrants()`, monta o orçamento do despacho e o serviço de
  `NewConsumerService` (o do runtime, mais `postgres.NewAttemptStore` e o
  orçamento) e roda juntos os cinco consumers de `Subscriptions` e a varredura
  de tentativas do kernel; o primeiro laço que termina para os outros. Cada
  consumer é validado antes de entrar, e cada inbox de mensagem tem a própria
  purga, `message-inbox.<consumer>`.

A varredura de tentativas é `kernelapp.RunSweep` a cada 15s, em lotes de 100,
sobre `kernelapp.SweepExternalAttempts`: a tentativa começada além do corte
`StaleAfter` do orçamento vai a `service.AbandonExternalAttempt`, que presume
`ClassUnknownSent` e não reenvia nada (RES-43), com `Timeout` igual ao
`Settle` do orçamento (decisão 21).

O orçamento do despacho (`app/dispatch_budget.go`) é o de um envio: a rota
`partner`/`submit` com `resilience.DefaultDeadline` (2s), porque não há
adaptador real até #5, mais duas vezes o prazo do `Put` da loja
(`RawStoreSheet()`, 2s), porque o envio grava o pedido antes da chamada e a
resposta depois (decisão 17); `Settle` de 5s e margem `kernelapp.MinExternalMargin` (1s). O
corte `StaleAfter` é 6s + 5s + 1s = 12s, que também é o `Processing`: prazo de
processamento e timeout do consumer de `gateway.dispatch`. Os outros quatro
consumers usam `ConsumerTimeout` (30s) e prazo de processamento de 5s.

`SweepExternalCommands` não roda no consumer: é um RPC de comando, acionado
por agendador externo (`application/sweep.go`), que pede o reenvio (inclusive
a liberação do despacho retido), consulta o estado dos comandos vencidos e
escala as esperas expiradas. O tempo do ciclo é de quem chama, não do kernel
(ADR-070 do DMPF, item 6).

## Canais, tópicos e grupos

O consumer lê cinco canais, cada um com grupo e inbox próprios, para o
rebalance de um nunca pausar os outros (`app/catalog.go`, `app/consumer.go`).
Os valores são os do `deploy/compose.yml`, do `deploy/.env.example` e do
ConfigMap da base; nenhum tem default no código.

| Canal | Tópico e DLQ (variáveis) | Grupo (variável) | Inbox | Publicado por | Tipos aceitos |
| --- | --- | --- | --- | --- | --- |
| `partner.events` | `partner.events`, `partner.events.dlq` (`KAFKA_PARTNER_TOPIC`, `KAFKA_PARTNER_DLQ`) | `gateway` (`KAFKA_GROUP`) | `gateway` | relay do Partner | `CapabilityEnabled`, `CapabilitySuspended` |
| `gateway.events` | `gateway.events`, `gateway.events.dlq` (`KAFKA_GATEWAY_TOPIC`, `KAFKA_GATEWAY_DLQ`) | `gateway.events` (`KAFKA_GATEWAY_EVENTS_GROUP`) | `gateway.events` | relay do gateway | `ExternalSchemaMismatchDetected` |
| `gateway.dispatch` | `gateway.dispatch`, `gateway.dispatch.dlq` (`KAFKA_GATEWAY_DISPATCH_TOPIC`, `KAFKA_GATEWAY_DISPATCH_DLQ`) | `gateway.dispatch` (`KAFKA_GATEWAY_DISPATCH_GROUP`) | `gateway.dispatch` | relay do gateway | `ExternalCommandDispatchRequested` |
| `gateway.interactions` | `gateway.interactions`, `gateway.interactions.dlq` (`KAFKA_GATEWAY_INTERACTIONS_TOPIC`, `KAFKA_GATEWAY_INTERACTIONS_DLQ`) | `gateway.interactions` (`KAFKA_GATEWAY_INTERACTIONS_GROUP`) | `gateway.interactions` | relay do gateway | `ExternalResponseCaptured` |
| `gateway.normalizations` | `gateway.normalizations`, `gateway.normalizations.dlq` (`KAFKA_GATEWAY_NORMALIZATIONS_TOPIC`, `KAFKA_GATEWAY_NORMALIZATIONS_DLQ`) | `gateway.normalizations` (`KAFKA_GATEWAY_NORMALIZATIONS_GROUP`) | `gateway.normalizations` | relay do gateway | `ExternalInteractionStored` |

`partner.events` carrega os onze tipos do Partner e `gateway.events` os nove
integration events do contexto; cada canal interno carrega um tipo só
(KFK-03). O `Accepts` de cada consumer lista só os tipos que o handler trata, e
o kernel descarta o resto do canal (KFK-03c). Em `partner.events` o consumer
admite só a source de `PARTNER_SOURCE` (`urn:dmpf:reference-partner`); nos
canais próprios, só a do relay do contexto (`urn:dmpf:reference-gateway`). O
transporte só conta como verificado quando o TLS confere o broker e o cliente
se autentica (IDN-03, IDN-04).

As ACLs do principal `gateway` estão em `deploy/infra.json`, de onde o
`infrasync` gera o `redpanda-init` local: escrita e describe nos oito tópicos e
DLQs do contexto e em `partner.events.dlq`; leitura e describe em
`partner.events` no grupo `gateway` e em cada canal próprio no grupo de mesmo
nome. O `deploy/k8s/overlays/hmg/secrets.example.yaml.tmpl` repete a mesma
exigência para o principal SASL de homologação.

## Borda, banco e configuração

O contexto serve só gRPC (`company.gateway.service.v1.GatewayService`), atrás da cadeia de
interceptors do kernel; o REST público é do `bff` (ADR-044, ADR-053). O
`ServiceDesc` de `app/rpc/service.go` declara 26 métodos, um por comando e por
consulta da SPEC-N1X148AW. A primeira fatia tinha 17 (dez comandos e sete
consultas); a segunda acrescentou os comandos `ReprocessInteraction`,
`LinkExternalReference`, `ReceivePartnerInteraction` (a entrada do webhook, que
o `bff` chama; decisão 19) e `ReadExternal` (a leitura externa genérica até #5;
decisão 20), e as cinco consultas de interação e de referência
(`FindExternalInteraction`, `FindExternalInteractionByPartnerEvent`,
`FindExternalInteractionsByCommand`, `FindExternalReference` e
`FindExternalReferencesBySubject`). Os 14 métodos de comando estão em
`rpc.Commands()` e exigem `idempotency-key`, inclusive `SweepExternalCommands`
e `ReadExternal`, que gravam; as 12 consultas não leem chave.

A borda recebe mensagem gRPC de até 4 MiB, o padrão que o `serve.ServeAPI` do
kernel não deixa o contexto subir. Por isso `application.MaxPayloadBytes` é
4.128.768 bytes (4 MiB menos 64 KiB, que ficam para o envelope de
`ReceivePartnerInteraction` ao lado do corpo), e o `DeclareCapabilityProfile`
recusa com `receivables-gateway/capability-profile/external-profile-invalid` a
operação que declara `maxPayloadBytes` acima dele. O `ReceivePartnerInteraction`
confere o corpo contra o `maxPayloadBytes` da operação antes de qualquer `Put`
na loja do bruto (`receivables-gateway/external-interaction/payload-too-large`),
e o `ReadExternal` limita pela mesma declaração a leitura normalizada que a
inbox de comandos guarda. O `TestABodyAtTheCeilingCrossesTheEdgeAndALargerMessageDoesNot`
(`app/rpc/partner_test.go`) prova os dois lados: o corpo no teto é gravado
(`stored`), e a mensagem de 4 MiB volta `ResourceExhausted`.

O banco é próprio: database `gateway`, cuja dona é a role migradora
`gateway_migrator`, e role de runtime `gateway`, sem posse (ADR-063);
schema `public`, tabelas sem prefixo e agregado no plural. O DDL vive em
`provider/schema.sql` — `connectors`, `capability_profiles`,
`query_authorizations`, `external_commands` e, desde a segunda fatia,
`external_interactions` e `external_references` — e é aplicado no ready do
`api` quando `MIGRATE=true`, pela migradora de `PG_MIGRATE_DSN`, junto com a
outbox, a inbox e o ledger `external_attempt` do kernel, seguido dos grants do
kernel dessas três capacidades e de `provider.Grants`; sem `MIGRATE`, o ready
espera que a role de runtime tenha esses grants. O `consumer` espera o mesmo
conjunto (`consumerGrants`), porque o despacho escreve no ledger, e o `relay`
só os da outbox. Toda tabela do `schema.sql` precisa de um
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
passar cada comando por `usecase.Execute`, que o executa em `usecase.RunIdempotent`,
e declarar o método em `rpc.Commands()`, que o interceptor usa para exigir a
metadata `idempotency-key`.
O `kernelgrpc.StatusOf`, com que os handlers devolvem o erro, já traduz a
divergência, o comando em andamento e a chave ausente.
Os prazos (`IdempotencyWait` 1s, `IdempotencyRetention` 24h, `OutboxRetention`
168h, `InboxRetention` 192h, `PurgeInterval` 15min, `PurgeBatch` 1000) são a
seção `Policies` da `Config` (`kernelapp.Policies`) e vêm de `Defaults()`; o
`api` purga as entradas de comando vencidas e o ledger `external_attempt`, o
`relay`, a outbox publicada, e o `consumer`, cada inbox de mensagem. As duas
últimas purgas usam a `InboxRetention`, que o `Validate` recusa abaixo da
janela de reentrega de qualquer canal que o consumer lê (INB-14).

A configuração vem só do ambiente, sem prefixo `DMPF_`. O `requirements()`
compõe, por papel, o `Missing()` da seção `API` (as variáveis `GRPC_*`, lidas
por `kernelgrpc.ReadAPIEnv`), o do Kafka, o da loja do bruto e as variáveis do
contexto, e a falta de uma variável obrigatória recusa a partida nomeando-a.

| Variável | `api` | `relay` | `consumer` |
| --- | --- | --- | --- |
| `PG_DSN` | obrigatória | obrigatória | obrigatória |
| `MIGRATE` e `PG_MIGRATE_DSN` | opcional; a DSN é obrigatória com `MIGRATE=true` | — | — |
| `GRPC_ADDR` | opcional (`:9090`) | — | — |
| `GRPC_INSECURE` ou as quatro do mTLS (`GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE`, `GRPC_CLIENT_CA_FILE` e `GRPC_TRUSTED_CLIENTS`) | obrigatória | — | — |
| `KAFKA_BROKERS` e `KAFKA_GROUP` | — | obrigatória | obrigatória |
| `KAFKA_INSECURE`; sem ele, `KAFKA_SASL_MECHANISM` (com `KAFKA_SASL_USERNAME` e `KAFKA_SASL_PASSWORD`) ou `KAFKA_CLIENT_CERT_FILE` | — | obrigatória | obrigatória |
| `KAFKA_GATEWAY_TOPIC`, `KAFKA_GATEWAY_DLQ` e os pares `KAFKA_GATEWAY_DISPATCH_*`, `KAFKA_GATEWAY_INTERACTIONS_*` e `KAFKA_GATEWAY_NORMALIZATIONS_*` (`_TOPIC` e `_DLQ`) | — | obrigatória | obrigatória |
| `KAFKA_PARTNER_TOPIC` e `KAFKA_PARTNER_DLQ` | — | — | obrigatória |
| `KAFKA_GATEWAY_EVENTS_GROUP`, `KAFKA_GATEWAY_DISPATCH_GROUP`, `KAFKA_GATEWAY_INTERACTIONS_GROUP` e `KAFKA_GATEWAY_NORMALIZATIONS_GROUP` | — | — | obrigatória |
| `PARTNER_SOURCE` | — | — | opcional (`urn:dmpf:reference-partner`) |
| `S3_BUCKET` e `AWS_REGION` | obrigatória | — | obrigatória |
| `S3_ENDPOINT`, `S3_KMS_KEY_ARN`, `S3_INSECURE` e `S3_PATH_STYLE` | opcional | — | opcional |

No `api`, os timeouts de sessão `lock_timeout` (2s) e
`idle_in_transaction_session_timeout` (30s) entram no `PG_DSN` quando o ambiente
não os define, porque o papel lê a série sob a guarda do ADR-067. O
`PG_MIGRATE_DSN` só é lido no `api` com `MIGRATE=true`, que o exige. Os dois
vêm do `deploy/.env.migrator.example`, que só o `serve-api` e o `docker:run`
carregam (este pela cópia `deploy/.env.migrator`, que o `deploy-env` cria): o
`deploy/.env.example` vale também para o `relay`, e a credencial da migradora
não chega a ele.

## Loja do bruto (S3)

O `api` e o `consumer` guardam o bruto das interações na porta
`ports.ObjectStore` do kernel, pelo provider `s3` (decisão 16;
`app/raw_store.go`). A loja tem escopo `tenant`, cifra `AES256` ou, com
`S3_KMS_KEY_ARN`, `aws:kms` com essa chave, e o contexto recebe só o escritor e
o leitor, nunca retenção nem purga; sem teto declarado, nenhum `Put` leva
retenção até 04:G11. `S3_ENDPOINT` vazio é o S3 da AWS; `S3_INSECURE` e
`S3_PATH_STYLE` são opt-outs de desenvolvimento, para o emulador.

O bucket é versionado, com Object Lock e sem retenção padrão, e é provisionado
fora do contexto: nenhum papel o cria. Na partida, `OpenRawStore` resolve as credenciais pela cadeia
padrão do SDK (`awsconfig.LoadDefaultConfig`) e repete o `Ready` do provider
até `ReadyAttempts` (5) vezes, a cada `ReadyInterval` (2s) (`awaitReady`). O
bucket que não guarda versões sob trava (`s3.ErrBucketNotImmutable`) é recusado
na hora, porque esperar não versiona nem trava bucket; em qualquer recusa, o
papel não parte. Como o Object Lock só se concede na criação e liga o
versionamento, criar o bucket com Object Lock e sem regra de retenção basta.

Na topologia local (`infra/local/docker-compose.yml`), quem cria o bucket é o
serviço `gateway-s3-init` do `deploy/compose.yml`: no floci
(`http://floci:4566`), `${GATEWAY_S3_BUCKET:-gateway-interactions}` com
`--object-lock-enabled-for-bucket`, sem tocar no bucket que já existe. O
`dmpf-gateway-api` e o `dmpf-gateway-consumer` só sobem depois de ele terminar
com sucesso. Ele roda nos profiles `dmpf` e `floci`, então também prepara o
bucket para os processos do host, que o `deploy/.env.example` aponta para
`http://localhost:4566`:

```bash
# Topologia inteira, com os processos do gateway em contêiner
docker compose -f infra/local/docker-compose.yml --profile dmpf up -d
# Só o floci e o bucket, para os processos no host (serve-api e afins)
docker compose -f infra/local/docker-compose.yml --profile floci up -d
```

À mão, contra o floci local (exposto em `127.0.0.1:${FLOCI_PORT:-4566}`):

```bash
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
  aws --endpoint-url http://localhost:4566 s3api create-bucket \
  --bucket gateway-interactions --object-lock-enabled-for-bucket
```

O `gateway-s3-init` usa endereçamento por caminho
(`aws configure set default.s3.addressing_style path`), como o
`S3_PATH_STYLE=true` dos papéis; no host, o CLI pode tentar o endereçamento
virtual (`gateway-interactions.localhost`) se não for configurado assim. A
infra de testes (`infra/test/compose.yml`) expõe o floci em `127.0.0.1:14566`,
e cada teste de integração cria o próprio bucket (`createBucket` em
`app/raw_store_integration_test.go`).

Em dev e hmg, o bucket é do ambiente e precisa existir, com as mesmas
propriedades, antes do primeiro deploy do `api` e do `consumer`. O ConfigMap da
base desliga os opt-outs (`S3_INSECURE: 'false'`, `S3_PATH_STYLE: 'false'`) e
não declara `S3_ENDPOINT`. O overlay `dev` põe no ConfigMap
`S3_BUCKET: gateway-interactions-dev` e `AWS_REGION: us-east-1`, sem
`S3_KMS_KEY_ARN`, então a cifra é `AES256`; o `hmg` espera `S3_BUCKET`,
`AWS_REGION` e `S3_KMS_KEY_ARN` no Secret `gateway`
(`deploy/k8s/overlays/hmg/secrets.example.yaml.tmpl`). As credenciais vêm da
identidade do pod: os três Deployments usam o ServiceAccount `gateway`
(`deploy/k8s/base/serviceaccount.yaml`, sem anotação e com
`automountServiceAccountToken: false`), e ligar a ele a role IAM (por exemplo,
por IRSA) é do ambiente. O mesmo template lista as permissões dessa role:
`s3:PutObject`, `s3:GetObjectVersion`, `s3:GetObjectRetention` e
`s3:GetObjectLegalHold` em `<bucket>/tenant/*`; `s3:GetBucketVersioning` e
`s3:GetBucketObjectLockConfiguration` no bucket; `kms:GenerateDataKey` e
`kms:Decrypt` na chave; nunca `s3:BypassGovernanceRetention`.

## Adaptadores de parceiro e simulador

Nenhum provedor está mapeado (#5), então o runtime não tem adaptador de
parceiro: em runtime, o serviço de `NewGatewayService` recebe `ports.NoPartnerAdapters`,
cujos `Command`, `Reader` e `Normalizer` devolvem
`ports.ErrPartnerAdapterUnavailable` para todo conector e operação. O
despacho, a consulta de estado da varredura, a leitura externa (`ReadExternal`)
e a normalização falham fechado como dependência indisponível
(`usecase.TransientDependency`), sem chamada externa (decisão 21).

O simulador de parceiro (`appkit/partnersim`) é o parceiro roteirizado do kernel
(`serviceskit.NewPartner`, KIT-12) atrás dessas portas, com o bruto num formato
JSON próprio. Só arquivos `_test.go` o importam, então ele nunca entra no
binário. As suítes de integração de `app` o injetam no `api` e no `consumer`
pelo campo não exportado `partners` da `Config`, que nenhuma variável de
ambiente preenche: fora delas, o papel sempre recebe `NoPartnerAdapters`.

## Resolução de dependências

O módulo importa o próprio contrato (`gateway/contract`) e o do Partner
(`partner/contract`, pelos eventos que o consumer lê). O `require` de cada irmão
é gravado pelo `dmpf-modsync`, que você roda de novo a cada import novo:

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/modsync@v1.0.0-rc.8 --root . --write
```

Dentro do workspace quem resolve é o `go.work` da raiz, onde o generator
registrou a entrada `use` e o modsync mantém o `replace` versionado; fora dele,
o `require` resolve pela tag publicada do irmão. O CI confere os dois lados com
`--check`.

## Antes do primeiro commit

Unidade nova muda a classificação: regrave o baseline do verificador.

```bash
go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@v1.0.0-rc.8 --root . --write-baseline
```
