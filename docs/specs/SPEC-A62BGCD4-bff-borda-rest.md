---
id: SPEC-A62BGCD4
slug: bff-borda-rest
title: bff - borda REST pública sobre os contextos gRPC (fundação e identity)
stage: done
priority: P0
depends_on: [SPEC-GXGASP24, SPEC-7KW0HHF6, SPEC-SBW7HT6Y, SPEC-STEJG347]
ticket_url: https://linear.app/mmda/issue/DEVS-110
subtask_urls: [https://linear.app/mmda/issue/DEVS-111, https://linear.app/mmda/issue/DEVS-112, https://linear.app/mmda/issue/DEVS-113, https://linear.app/mmda/issue/DEVS-114]
created: 2026-10-15
---

# SPEC-A62BGCD4: bff - borda REST pública sobre os contextos gRPC (fundação e identity)

> **Addendum (2026-10-17)** — a referência passa ao kernel `1.0.0-rc.5`
> (`workflowRef` `b5131ee`), consumido desde o PR #4 (ADR-011). Entre `487e9de`
> (`1.0.0-rc.4`) e `b5131ee`, o `bff` de referência mudou em 31 arquivos. Com o
> verificador OIDC, a API consulta a deny-list de sessão no Redis. `REDIS_URL`
> é obrigatório na partida inclusive com `AUTH_DEV_MOCK`, que não consulta a
> deny-list. O papel `bff session-events` (subcomando da mesma imagem, com uma
> réplica) alimenta essa deny-list pelo back-channel logout e pelo polling da
> Admin REST (ADR-065 do kernel). O tenant vem da claim nativa `organization`
> por `OIDC_TENANT_CLAIM=organization.*.tenant_id`, com `OIDC_AUDIENCE=dmpf-bff`,
> sem mapper que achate (ADR-064 do kernel; addendum da ADR-016). SPEC-SBW7HT6Y
> e SPEC-STEJG347 estão `done` no kernel, e esta spec passa a `planning`.
> Divisão: o checker da API e o `REDIS_URL` entram na DEVS-111; o teste HTTP de
> revogação, sobre a primeira rota autenticada, na DEVS-112; o papel
> `session-events`, os manifestos dele e o back-channel do realm, na DEVS-114.
> A recusa em até 60 s contra o Keycloak é garantida pelo kernel, e o workspace
> prova a fiação. O texto original foi preservado onde não conflita com o rc.5.

> **Addendum (2026-10-17, fronteira da DEVS-111)** — o gate estrutural do DMPF
> trata o `bff` como borda e exige `app/rpc` e `deploy/infra.json` desde o
> primeiro commit. Para que a DEVS-111 saia com o CI verde, ela antecipa o
> mínimo real: `app/rpc/tls.go` (o `ClientTLS`, carregado na partida quando
> `GRPC_CA_FILE` está definido, recusando material ilegível), o
> `deploy/infra.json` com `grpc.client: bff` e o `infrasync --write`, que emite
> o cliente `bff` em `infra/local/compose/pki.yml` e atualiza
> `infra/local/.env.example`, o Alloy e os agregados
> `infra/k8s/overlays/{dev,hmg}/apps`. O dial gRPC e o `ClientOptions`, que
> reutiliza `rpc.ClientTLS`, ficam na DEVS-112, com a dependência do `/readyz`
> no `identity`. O `edge: bff` no `dmpf.json`, o `init` e a regeneração do
> `identity` ficam na DEVS-114, que também cria o `deploy/k8s` do `bff`. Até lá,
> os agregados `infra/k8s/overlays/{dev,hmg}/apps` referenciam overlays do
> `bff` que ainda não existem; nenhum gate renderiza kustomize.

> **Addendum (2026-10-18, planejamento da DEVS-112)** — quatro decisões ajustam
> o texto abaixo sem mudar o requisito.
> (1) **503/504 com o `identity` fora.** O `ResolveIdentity` do kernel
> (`libs/backend/go/http/identity.go:37-40`, igual no rc.5) converte qualquer
> erro de `Authenticate` em 401. O decorador sozinho, portanto, não produz o
> 503/504 que as Decisões atribuem a ele. O middleware envolve o decorador num
> `Authenticator` que só guarda o erro devolvido: quando o kernel responde 401
> e o erro guardado é a indisponibilidade das permissões, a borda responde 503
> ou 504 pela tradução de erros. Token ausente, inválido ou revogado segue 401,
> e o kernel continua decidindo 401 e 403 e chamando o decorador uma vez, sem
> extensão do kernel.
> (2) **Chamada única.** `FindEffectivePermissions` é chamada uma vez **pelo
> decorador** por requisição autenticada com tenant resolvido, e nunca sem
> token nem com tenant nulo. A rota `GET /identity/users/{userId}/permissions`
> faz uma segunda chamada, que é o RPC da própria rota, para o `userId` do
> path; reaproveitar a primeira seria errado quando `userId` difere do `sub` e
> equivaleria a cache (CTX-16).
> (3) **`AUTH_DEV_MOCK` decorado.** O mock só substitui a verificação do
> token; as permissões vêm sempre do `identity`, também em dev e testes.
> (4) **Readiness.** O `/readyz` lê o estado da conexão gRPC, com o health
> check declarado no service config (GRP-13). Chamar `Health/Check` direto
> esbarraria na política por método, que recusa método sem política (GRP-16).
> O texto original foi preservado.

> **Addendum (2026-10-19, code review da DEVS-112)** — a admissão por rota e
> tenant passa a rodar **antes** do salto de permissões. Na ordem anterior, uma
> requisição que receberia 429 já tinha chamado `FindEffectivePermissions`, e um
> tenant abusivo saturava o breaker e o bulkhead compartilhados do `identity`,
> derrubando os demais (RES-16). Ordem por rota: recuperação de pânico → prazo da
> rota → verificação do token (uma vez) → admissão pelo tenant verificado, só
> com token válido e tenant resolvido → `FindEffectivePermissions` e
> `ResolveIdentity` → `RefuseAssertedIdentity` → contexto de execução →
> `Idempotency-Key` → handler. Sem token, com token inválido ou com tenant nulo
> a requisição não consome admissão e responde 401 ou 403 como antes. A
> admissão segue a do kernel, sem extensão. Cliente que desiste durante o salto
> de permissões recebe 499 `client-closed-request`, sem contar como erro.
> Pelo checklist de qualidade da mesma task, o salto de permissões ganhou
> conexão gRPC própria ao mesmo `IDENTITY_GRPC_TARGET` (dependência
> `identity-permissions`, só `FindEffectivePermissions`, limite de 500 ms,
> orçamento de retry próprio), com breaker e bulkhead separados dos da conexão
> das rotas: uma rota que falha ou satura o pool não derruba mais a
> autenticação de todos os tenants. As duas conexões reconectam com intervalo
> máximo de 5 s, são exigidas pelo `/readyz` e fecham no shutdown. A recusa de
> admissão do `identity` nesse salto (`ResourceExhausted`) responde 503
> `unavailable` com `Retry-After: 1`. Dimensionamento de bulkhead e de
> admissão, paginação, keepalive e os demais achados ficam na DEVS-134.

> **Addendum (2026-10-19, planejamento da DEVS-113)** — quatro decisões ajustam
> o texto abaixo sem mudar o requisito.
> (1) **Lookup sem encaminhar a chave.** `POST /identity/organization-lookups`
> exige a `Idempotency-Key` (400 sem ela, e a rota declara a chave, o que
> satisfaz o `ValidateEdge`). Só as rotas de comando enviam ao contexto a chave
> derivada e leem o header de replay; o lookup não envia `idempotency-key` nem
> devolve `Idempotent-Replayed`. "Nada é armazenado nem repetido" passa a ser
> garantido na borda, sem depender de o `identity` ignorar a chave (a
> operação não está em `Commands()` do contexto).
> (2) **Forma na borda, conteúdo no domínio.** O handler valida o corpo só nos
> limites do schema publicado: presença dos campos obrigatórios, `maxLength`,
> `minLength` onde declarado, `pattern`, enums, `maxItems`, `uniqueItems` e
> campos desconhecidos, com 400 `invalid-request`. Um valor vazio que o schema
> admite segue ao `identity`, que o recusa com 422, como o contrato declara
> (`info.description` do OpenAPI do `identity`). A aprovação de troca de conta
> não tem corpo e recusa qualquer conteúdo.
> (3) **Replay.** `Idempotent-Replayed: true` sai em toda resposta de comando
> abaixo de 500 cujo RPC devolveu o header, inclusive a rejeição 422 repetida.
> A borda reflete só a tentativa final do retry em `UNAVAILABLE`, que reenvia a
> mesma chave. Nesta task a prova usa servidor gRPC falso; a prova contra o
> `identity` real, com o mesmo status e corpo, fica no e2e em container da
> DEVS-114.
> (4) **Status de sucesso.** `suspendUser`, `revokeMembership`,
> `approveBankAccountChange` e o lookup respondem 200; os demais comandos
> respondem 201, como declara o OpenAPI do `identity`.

> **Addendum (2026-10-19, code review da DEVS-113)** — três ajustes.
> (1) **Chave antes do salto de permissões.** O contrato recusa o `POST` sem
> chave ou com chave fora do formato "before any call to the context"
> (parâmetro `IdempotencyKey` do OpenAPI do `identity`), e
> `FindEffectivePermissions` é uma chamada ao contexto. A ordem por rota do
> addendum da DEVS-112 passa a ser: recuperação de pânico → prazo da rota →
> verificação do token → admissão → `Idempotency-Key` → `FindEffectivePermissions`
> e `ResolveIdentity` → `RefuseAssertedIdentity` → contexto de execução →
> handler. Sem token, com token inválido ou com tenant nulo, a resposta segue
> 401 ou 403, como antes.
> (2) **Corpo estrito.** O nome de cada campo precisa ser igual ao do schema,
> inclusive em maiúsculas e minúsculas; chave repetida, `null` em qualquer
> campo e corpo fora de UTF-8 respondem 400 `invalid-request`. Resposta de
> sucesso do contexto sem identificador válido, ou rejeição sem código,
> responde 500, como já acontece nas consultas.
> (3) **Limite de 64 KiB.** O limite de corpo da borda segue em 64 KiB. Um
> `addEstablishment` no limite máximo do schema, com 32 referências externas e
> textos de 256 caracteres de 4 bytes, passa desse tamanho e recebe 400; com
> valores ASCII, o mesmo corpo fica em torno de 25 KiB. O dimensionamento fica
> na DEVS-134.

> **Addendum (2026-10-19, checklist de qualidade da DEVS-113)** — os achados de
> segurança, robustez e performance foram tratados nesta task, sem extensão do
> kernel.
> (1) **Comandos em conexão própria.** Uma terceira conexão gRPC ao mesmo
> `IDENTITY_GRPC_TARGET`, dependência `identity-commands`, atende só os 10
> comandos, com breaker e bulkhead próprios; a `identity` fica com as consultas
> e a consulta por CNPJ. Comandos que saturam o pool não derrubam as consultas
> de outros tenants. As três conexões são exigidas pelo `/readyz`. Cada método
> do `IdentityService` tem política em exatamente uma conexão, e
> `GrantMembership`, fora da borda, em nenhuma.
> (2) **Retry dos comandos.** O orçamento de retry dos comandos é declarado
> (1,5 s, abaixo do prazo da rota, GRP-17); sem ele a segunda tentativa recebia
> cerca de 0,35 s e virava 504 evitável. A mesma chave segue em todas as
> tentativas. `Idempotent-Replayed: true` pode sair na primeira resposta que o
> cliente vê, quando a tentativa anterior falhou depois de o `identity` executar
> o comando: a resposta é, de fato, a guardada.
> (3) **`Retry-After`.** Toda resposta 429 ou 503 vinda do contexto, inclusive
> no salto de permissões, sai com `Retry-After: 1`, a mesma estratégia da
> admissão do kernel.
> (4) **Corpo.** A leitura do corpo respeita o prazo da rota, e o corpo que não
> chega a tempo responde 400 `invalid-request`, sem chamar o contexto. O teto de
> cada `POST` sai do maior corpo válido pelo schema, arredondado em KiB e
> limitado a 64 KiB; acima dele, 400 com mensagem de tamanho. A validação
> estrita roda numa só passada com a forma em cache por tipo e recusa o array
> acima de `maxItems` durante a leitura, com custo igual ou menor que o do
> decode anterior.
> (5) **Chave derivada.** Segue do sujeito, sem o tenant, como declara o
> contrato ("scopes the key to the authenticated subject"); com a chave do
> cliente ausente ou inválida, o log não registra a derivada.
> O `Content-Type` não é exigido: o contrato não declara 415, e o
> `Idempotency-Key` já força o preflight de CORS.

> **Addendum (2026-10-19, DEVS-114)** — decisões da integração de plataforma
> que resolvem os pontos a verificar desta task:
> (1) **e2e real (cenário 6).** Roda como o da referência: processos no host,
> com mTLS real (`tb.NewPKI`), dentro do `bff:test-race` com a tag
> `integration`. O `identity` sobe com `--role api` e um `PG_DSN` capaz de
> migrar, com `MIGRATE=true` e sem `PG_MIGRATE_DSN`, porque o `identity` ainda
> não adota a ADR-063. O teste percorre `api.Routes`, sem lista fixa, e exige
> o 403 com a mensagem da borda, para distinguir a recusa da borda da do
> `identity`. As rotas POST também são chamadas sem `Idempotency-Key` e recebem
> 400 `missing-idempotency-key` da borda, porque a chave é validada antes do
> salto de permissões (addendum do code review da DEVS-113, item 1). A
> cobertura das 21 rotas só se declara depois de um `bff:test-race` verde numa
> base que já contenha a DEVS-113. A prova em container é a subida do Compose
> local com os profiles `dmpf` e `keycloak`, verificada à mão e registrada no PR.
> O replay contra o `identity` real, que o addendum da DEVS-113 deixou para este
> e2e, não tem prova ponta a ponta. Sem caminho feliz, toda chamada para no 403
> antes do comando, e o replay fica provado só com o `identity` falso.
> (2) **Overlay de plataforma.** O overlay da referência foi portado para dev e
> hmg (`infra/k8s/overlays/{dev,hmg}/kustomization.yaml`, com o Secret `redis`
> e o Component `infra/keycloak/dev` no dev). No hmg, o overlay acrescenta ao
> ConfigMap `keycloak` a mesma URL de back-channel, que a referência deixa
> vazia; sem ela, o hmg revogaria só pelo polling. O `k8s-render` renderiza os dois
> overlays e confere o isolamento do papel: o Secret `keycloak-session-events`
> só no container do papel, o Service só com a 8091 e a NetworkPolicy só a
> partir de pods `keycloak`.
> (3) **Regeneração com `edge: bff`.** O `init --edge=bff` regravou os
> arquivos gerenciados sem `--force`. O `identity` e o `partner` foram
> regenerados num workspace descartável com os parâmetros originais: o render
> com `edge` vazio reproduz os arquivos atuais byte a byte, e o diff para
> `edge: bff` muda só a NetworkPolicy, o patch hmg e o README de cada contexto.
> O `partner` entra porque o `edge` vale para o workspace inteiro.
> (4) **Papel `session-events`.** O papel herda da API o teto de 10 s do
> `DRAIN_DELAY`, porque o Deployment dele também tem
> `terminationGracePeriodSeconds: 30`. Herda também a validação de
> `SESSION_EVENTS_ADDR` e `ADMIN_ADDR` com porta e IP normalizados, o encerramento
> sem erro quando o sinal chega durante a descoberta OIDC e o segundo sinal que
> encerra na hora. A gravação na deny-list tem prova determinística contra o
> Redis de testes, pelo polling e pelo back-channel.
> (5) **Medição dos 60 s (cenário 11).** Fica fora: vale a garantia do kernel
> `1.0.0-rc.5` (`revocation_keycloak_integration_test.go:184`) somada à prova
> de fiação do item 4, sem helper PKCE próprio.
> (6) **`docker:build` e `docker:run`.** O plugin `@nx/docker` passa a
> construir a partir da raiz (`cwd: "."`, `-f {projectRoot}/Dockerfile`), como
> na referência. Antes, o `docker:build` inferido falhava com o `COPY go.work`
> fora do contexto.
> (7) **Riscos residuais registrados no code review.** Ficam como estão:
> - O `k8s-render`, com as asserções de isolamento do papel, não roda no CI;
>   pô-lo lá exige mudar o workflow.
> - O PDB do `bff` tem `minAvailable: 1` e o dev roda uma réplica, então a
>   evicção voluntária fica bloqueada (igual à referência).
> - O `/readyz` do papel não depende do Redis (igual à referência). Com o Redis
>   fora, o papel segue pronto, e o receptor e o polling falham.
> - A NetworkPolicy gerada do `identity` seleciona `app.kubernetes.io/name In
>   [bff]`, que também casa os pods do `bff-session-events`. No hmg, o mTLS
>   barra o papel, que não tem certificado de cliente. O ajuste do seletor é do
>   plugin.
> - O `serve-session-events` acrescenta `service.instance.id` e
>   `dmpf.process.role` ao `OTEL_RESOURCE_ATTRIBUTES` do `deploy/.env`, e o
>   último valor de cada chave vence, como no `docker:run-relay` do `identity`
>   e na referência.
> (8) **Checklist de qualidade.** Foram corrigidos:
> - o prazo de 3 s por requisição do polling, com teto na espera pelo fim do
>   polling no desligamento, para caber nos 30 s;
> - a recusa de intervalo de polling maior que metade da janela (TTL + skew);
> - o cache de módulos no Dockerfile do `bff`;
> - `GOMEMLIMIT` nos Deployments;
> - `https://` no issuer de exemplo do hmg.
>
> Ficam registrados, todos iguais à referência, salvo onde indicado:
> - O polling do kernel repete a cada ciclo sem backoff e sem métrica de falha,
>   e o papel segue pronto. A rotação de `keycloak-session-events` exige
>   rollout do papel.
> - A API e o papel usam a mesma credencial do Redis, com escrita. Separar
>   leitor e escritor exige dois usuários ACL e dois Secrets.
> - O back-channel do hmg vai em HTTP dentro do cluster. Isto difere da
>   referência e vem do item 2. O token é assinado, e só os metadados ficam
>   expostos.
> - A NetworkPolicy do receptor confia no rótulo `keycloak`. O papel e a API
>   compartilham o ServiceAccount `bff`. As imagens base não são fixadas por
>   digest.
> - A partida do papel depende da descoberta OIDC e entra em backoff de
>   reinício enquanto o Keycloak não responde.
> - O default vazio de `KEYCLOAK_BACKCHANNEL_LOGOUT_URL` no Compose local é de
>   arquivo gerado.
> - Em escala, o polling relê a janela inteira a cada ciclo, com um `EVALSHA`
>   por evento. O `PageSize` não é exposto, e não há alerta sobre
>   `dmpf.auth.revocation.apply_delay` (cursor ou pipeline são do kernel;
>   dimensionamento na DEVS-134).
> - `adminEventsDetailsEnabled: true` no realm aumenta o volume relido pelo
>   polling.

## Resumo

O workspace não tem borda pública. O `identity` serve só gRPC, e o contrato REST
dele (`apps/backend/identity/contract/openapi/v1/openapi.yaml`, 22 operações) não
é servido por ninguém. Esta spec cria o app `bff` em `apps/backend/bff`, porte do
`bff` de referência do kernel DMPF (`1.0.0-rc.5`, `workflowRef` `b5131ee`; ver o
addendum de 2026-10-17). O app traz a cadeia completa da borda (OIDC com
consulta à deny-list de sessão, permissões efetivas resolvidas no `identity` a
cada requisição, tenant, idempotência, erros, admissão, CORS, OpenAPI servido e
mTLS de cliente), o papel `session-events`, que alimenta a deny-list, e 21 das
22 rotas do `identity`. Como usuário autenticado de uma organização cliente,
quero chamar o `identity` por HTTPS com o meu token, para operar cadastro,
vínculos e dados bancários sem acesso direto ao gRPC interno.

## Contexto

- **Problema**: a ADR-012 põe todo REST externo no `bff` e os contextos em gRPC,
  mas o `bff` não existe (`apps/backend/identity/README.md:36-38`). Com
  `"edge": ""` em `dmpf.json:6`, a NetworkPolicy do `identity` tem `ingress: []`
  (`apps/backend/identity/deploy/k8s/base/networkpolicy.yaml`), o overlay hmg
  tem `GRPC_TRUSTED_CLIENTS: ''`, e `infra/local/compose/pki.yml` só emite o
  certificado de servidor `dmpf-identity-api`, sem cliente para a borda. Em
  container, o `identity` fica inalcançável.
- **Impacto**: abre o primeiro caminho externo verificável de ponta a ponta
  (token, borda, gRPC, contexto) e destrava o Swagger UI local, que já aponta
  para `http://localhost:8080/openapi/identity/v1/openapi.yaml`
  (`infra/local/compose/swagger-ui.yml`). Cada contexto futuro acrescenta as
  próprias rotas à borda, sem refazer a fundação.
- **Inspiração**: o `bff` de referência do kernel, em
  `lidercap-platform/apps/backend/bff` (repositório `mateusmacedo/dmpf`), no
  commit `b5131ee` (`1.0.0-rc.5`). Entre `487e9de` (rc.4) e `b5131ee`, o `bff`
  passou a consultar a deny-list na API, com `REDIS_URL` obrigatório na
  partida, e ganhou o papel `bff session-events`, que alimenta a deny-list (31
  arquivos). O kernel ganhou as ADR-063, ADR-064 e ADR-065. Decisões de
  origem: ADR-044 do kernel (bff REST e contextos gRPC de referência), ADR-064
  (Keycloak de referência sem extensão) e ADR-065 (revogação de sessão no
  `authn`).
- **Links relevantes**:
  - [ADR-012](../adr/012-um-app-por-bounded-context.md): um app por contexto,
    REST só no `bff`.
  - [ADR-016](../adr/016-provedor-de-identidade-oidc.md) (addendum de
    2026-10-17): Keycloak, tenant lido da claim nativa `organization` por
    `organization.*.tenant_id`, deny-list de sessão e origem das permissões.
  - [ADR-022](../adr/022-precedencia-dmpf-sobre-design.md): o kernel prevalece
    sobre `docs/design/`; extensão do kernel nasce como spec no kernel.
  - SPEC-7KW0HHF6 (identity-organization): `FindEffectivePermissions` chamada
    pelo `bff` a cada requisição, sem cache (:59, :413, :452, :590).
  - `docs/design/91-contratos-api.md`: superfícies Client API, Partner API e
    webhooks (requisito de produto; divergências reconciliadas abaixo).
- **Dependências do kernel (satisfeitas em 2026-10-17)**: SPEC-SBW7HT6Y (DEVS-97,
  Keycloak de referência sem extensão, tenant pela claim nativa `organization`,
  audiência `dmpf-bff` e eventos de sessão) e SPEC-STEJG347 (DEVS-101, deny-list
  por `sub`/`sid` no Redis, receptor de back-channel e polling no `authn`) estão
  `done` no kernel e publicadas no `1.0.0-rc.5` (`b5131ee`), consumido por este
  workspace desde o PR #4 (ADR-011). A condição do discovery (2026-10-15) para
  entrar em `building` está cumprida.
- **Referências externas** (no `b5131ee`):
  - `lidercap-platform/apps/backend/bff/app/api/routes.go`: `bindings()`,
    `surface.command`/`surface.query`, `NewHandler` e ordem da cadeia
    (:156-179).
  - `lidercap-platform/apps/backend/bff/app/rpc/errors.go`, `metadata.go`,
    `clients.go`: mapeamento de erros, metadata gRPC e política por método.
  - `lidercap-platform/libs/backend/go/http/identity.go:29-58`:
    `ResolveIdentity` confere `route.Permission` contra
    `ports.Identity.Permissions`.
  - `lidercap-platform/libs/backend/go/ports/authn.go:25-48`: `Identity`
    (`Subject`, `Permissions`) e interface `Authenticator`.

<constraints>
- [P0] O `bff` não importa domínio nem aplicação de contexto: só módulos
  `contract` dos contextos e bibliotecas do kernel (ADR-012; ADR-044 do kernel;
  regra `DMPF-D002`).
- [P0] Uma rota encaminha para exatamente uma chamada gRPC. Sem orquestração,
  sem saga, sem banco. O Redis não é banco do `bff`: é um recurso compartilhado
  da infraestrutura, no qual o provider `libs/backend/go/redis` guarda a
  deny-list que a API só consulta e o papel `session-events` só alimenta
  (ADR-065 do kernel, itens 1 e 6).
- [P0] Toda rota declara `Permission` e `ContractRef`; `ValidateEdge` recusa na
  partida rota sem permissão ou `POST` sem exigência de `Idempotency-Key`
  (IDN-16, IDN-17, RST-04).
- [P0] Permissões efetivas vêm só de `FindEffectivePermissions` do `identity`,
  a cada requisição e sem cache (SPEC-7KW0HHF6:59, CTX-16). Claims de
  permissão do token são descartadas.
- [P0] Sujeito e tenant vêm só do token verificado; `X-Subject-ID`,
  `X-Tenant-ID` ou `?tenant_id` divergentes recusam com 403 (CTX-06). Todo
  campo `*ByUserId` da mensagem gRPC recebe o `sub`.
- [P0] Nenhuma extensão do kernel nem do IdP é escrita neste workspace:
  deny-list (`redis.RevocationStore`), receptor de back-channel, polling de
  eventos de sessão, leitura do tenant (`organization.*.tenant_id`) e
  verificador OIDC vêm do kernel. No Keycloak, sem provider, script mapper, SPI
  próprio nem feature Preview, e sem mapper que achate o tenant (ADR-022, item
  3; ADR-064 do kernel, itens 1 e 2; ADR-065 do kernel).
- [P0] Arquivos gerados pelo `dmpf-plugin` e pelo `infrasync` mudam só por
  regeneração, nunca por edição manual.
- [P1] Corpo de erro, headers e status de idempotência seguem o kernel, não o
  doc 91 (ADR-022, item 1).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] App `bff`**: criar o módulo Go
  `github.com/mateusmacedo/antecipa-market/apps/backend/bff` (`go 1.26.9`,
  bibliotecas do kernel na versão consumida), projeto Nx `bff` com tags
  `type:app`, `scope:backend`, `stack:go`, `layer:apps`, `dmpf-units.json` com
  a unidade `bff/app` (`bounded_context: bff`) e entrada no `go.work`.
- [ ] **[P0] Cadeia da borda**: montar, de fora para dentro, a ordem da
  referência: `otelhttp`, rota, access log, CORS, mux, recover, prazo da rota,
  `ExecutionContext` (`ResolveIdentity`), admissão, `Idempotency-Key`,
  handler.
- [ ] **[P0] Autenticação OIDC**: `authn.NewVerifier` com `OIDC_ISSUER` do
  realm do ambiente, `OIDC_AUDIENCE=dmpf-bff` e
  `OIDC_TENANT_CLAIM=organization.*.tenant_id`, sem mapper que achate o tenant
  (ADR-064 do kernel, item 2). O curinga casa exatamente uma organização, e o
  `tenant_id` dela precisa ter um único valor; senão, o tenant fica nulo e a
  rota responde 403 `tenant-unresolved` (IDN-20). `AUTH_DEV_MOCK` só em dev e
  testes, nunca junto de issuer. Token ausente, inválido ou revogado responde
  401 `unauthenticated`.
- [ ] **[P0] Permissões efetivas**: um `ports.Authenticator` decorador que
  verifica o token pelo `authn`, chama `IdentityService.FindEffectivePermissions`
  com `userId` igual ao `sub` e `x-tenant-id` do token, e substitui
  `Identity.Permissions` pelo resultado.
  - Lista vazia: a rota responde 403 `permission-denied`.
  - `identity` indisponível ou prazo esgotado: responde 503 ou 504, nunca
    segue com permissões das claims nem com lista vazia silenciosa.
  - Sem cache de resultado nem de ausência.
  - Tenant nulo no token (sem organização ativa): o decorador não chama
    `FindEffectivePermissions` e devolve a identidade sem permissões, e
    `ResolveIdentity` responde 403 `tenant-unresolved`
    (`libs/backend/go/http/identity.go:44-45` no `b5131ee`).
- [ ] **[P0] Revogação de sessão, consulta na API** (DEVS-111): `WithRevocations`
  no composition root liga `redis.NewRevocationStore` em
  `Config.Auth.Revocations` e é chamado em `Run`, antes de `boot.Boot`, e em
  `RunWith`. `REDIS_URL` é obrigatório na partida, inclusive com
  `AUTH_DEV_MOCK`: ausente, inválido ou com banco diferente de 0, a partida sai
  com exit 2 sem ecoar a URL. `REDIS_DIAL_TIMEOUT_MS` (1000),
  `REDIS_COMMAND_TIMEOUT_MS` (500) e `REDIS_REQUIRE_TLS` (`false`) são
  opcionais. Só o verificador OIDC consulta a deny-list; o `AUTH_DEV_MOCK`
  exige `REDIS_URL`, mas não a consulta.
  - Com o verificador, sessão ou sujeito revogado responde 401
    `unauthenticated`, e o registro `auth: token rejected` traz
    `dmpf.auth.refusal_reason=session_revoked`.
  - Redis fora ou mais lento que `OIDC_REVOCATION_TIMEOUT_MS` (250) recusa o
    token com 401 `unauthenticated` (falha fechada), com o registro
    `auth: revocation store unavailable` (`error.type=TransientDependency`),
    garantido pelo `authn` do kernel (ver Não-funcionais).
  - O `/readyz` da API não depende do Redis.
- [ ] **[P0] Papel `bff session-events`** (DEVS-114): subcomando da mesma imagem,
  com uma réplica e configuração própria (`SessionEventsFromEnv`), despachada
  antes de `FromEnv` e sem exigir variável da API. Compartilha com a API
  `ADMIN_ADDR`, `REDIS_*`, `DRAIN_DELAY` e `OTEL_*`. Serve
  `POST /backchannel-logout` em `SESSION_EVENTS_ADDR` (`:8091`) por
  `authn.NewBackchannelLogout` e faz polling da Admin REST por
  `authn.NewSessionPolling` (eventos `LOGOUT` e admin events, a cada
  `OIDC_SESSION_EVENTS_INTERVAL_SECONDS`, padrão 5). Grava no mesmo
  `redis.RevocationStore` que a API consulta.
  - Obrigatórias: `OIDC_ISSUER`, `OIDC_REVOCATION_TTL_SECONDS` (300, o
    `accessTokenLifespan` do realm), `OIDC_BACKCHANNEL_CLIENT_ID=dmpf-portal`,
    `OIDC_SESSION_EVENTS_CLIENT_ID=dmpf-session-events`,
    `OIDC_SESSION_EVENTS_CLIENT_SECRET` e `REDIS_URL`.
  - Opcionais: `SESSION_EVENTS_ADDR` (`:8091`), `ADMIN_ADDR` (`:8092`,
    distinto do receptor), `OIDC_SESSION_EVENTS_INTERVAL_SECONDS` (5),
    `OIDC_DISCOVERY_TIMEOUT_SECONDS` (10), `OIDC_REVOCATION_SKEW_SECONDS` (60),
    `DRAIN_DELAY`, `REDIS_*` e `OTEL_*`.
  - Variável obrigatória ausente ou inválida sai com 2, nomeando-a. Issuer fora
    da forma `<base>/realms/<realm>` ou sem descoberta na partida sai com 1.
  - Entre os papéis do `bff`, só o `session-events` recebe o Secret
    `keycloak-session-events`; o Keycloak também o lê para importar o realm. O
    back-channel do realm aponta para o papel (`KEYCLOAK_BACKCHANNEL_LOGOUT_URL`
    no Compose e `DMPF_BACKCHANNEL_LOGOUT_URL` no ConfigMap `keycloak` do dev).
  - Sessão revogada responde 401 em até 60 s na API com o verificador OIDC
    (ADR-016, condição 2; ADR-065 do kernel).
- [ ] **[P0] Rotas do identity**: registrar as 21 rotas da tabela abaixo, cada
  uma com `Requires: RequireSubjectAndTenant` (como o `surface.route` da
  referência), `ContractRef` para a operação do OpenAPI, `Permission`
  `identity:<operação-em-kebab>` (`apps/backend/identity/app/authorization.go`)
  e, nos `POST`, `Idempotency-Key` obrigatório.
- [ ] **[P0] Preenchimento do ator**: preencher todo campo `*ByUserId` das
  mensagens gRPC com o `sub` do token; o corpo externo não carrega esses campos
  (OpenAPI do identity, `info.description`).
- [ ] **[P0] Erros**: traduzir a resposta gRPC para HTTP pela tabela do kernel
  (`grpc.HTTPStatus` e `app/rpc/errors.go` da referência). Rejeição no
  `oneof result` responde 422 com o código de domínio; o `reason` do
  `ErrorInfo` é lido antes do código gRPC; corpo `{"code","message"}` em
  `application/json; charset=utf-8`.
- [ ] **[P0] Metadata gRPC**: enviar `x-correlation-id`, `x-causation-id`,
  `x-tenant-id`, `traceparent`, `idempotency-key` derivado
  (`hex(sha256(subject || 0x00 || key))`) e `x-locale`. Sujeito e permissões não
  atravessam o salto (CTX-12, IDN-02).
- [ ] **[P0] Idempotência**: `Idempotency-Key` no formato
  `^[A-Za-z0-9._-]{1,128}$`; ausência ou formato inválido responde 400. Replay
  devolve o mesmo status e corpo com `Idempotent-Replayed: true`, só abaixo de
  500 (ADR-056 do kernel, §2 e §6). Em `POST /identity/organization-lookups` a
  chave é exigida, mas nada é armazenado nem repetido.
- [ ] **[P0] Cliente gRPC**: `kernelgrpc.Config` com `MethodPolicy` para cada
  método do descriptor de `company.identity.service.v1.IdentityService`; prazo
  do método menor que o da rota (GRP-17); retry só em `UNAVAILABLE`; breaker
  contando só indisponibilidade. Teste reprova método do descriptor sem
  política.
- [ ] **[P0] mTLS de cliente**: `GRPC_CA_FILE`, `GRPC_CLIENT_CERT_FILE`,
  `GRPC_CLIENT_KEY_FILE` e `GRPC_SERVER_NAME`, com identidade
  `spiffe://dmpf/bff` (ADR-052 do kernel). `GRPC_INSECURE` só no host local.
  - DEVS-111: `app/rpc/tls.go` com `ClientTLS`, carregado na partida quando
    `GRPC_CA_FILE` está definido; material ilegível impede a partida.
  - DEVS-112: dial gRPC e `ClientOptions`, que reutiliza `rpc.ClientTLS` e
    substitui a carga da partida.
- [ ] **[P0] Borda declarada**: `dmpf.json` com `"edge": "bff"`; regenerar os
  arquivos do `identity` que dependem de `edgeApp` (NetworkPolicy,
  `GRPC_TRUSTED_CLIENTS` do overlay hmg, `project.json`, `README.md`) pelo
  procedimento definido em Pontos a verificar (Regeneração com `edge: bff`);
  `deploy/infra.json` do `bff` com `grpc.client: bff` e rodar
  `infrasync --write` para emitir o certificado de cliente em `pki.yml`.
  Com `edge: bff`, a regeneração do `init` liga o back-channel do realm ao papel
  `session-events` e sobe o Redis local também nos profiles `dmpf` e `keycloak`
  (ver Arquivos a modificar).
  - DEVS-111: `deploy/infra.json` do `bff` e `infrasync --write` (cliente `bff`
    no `pki.yml`), o que satisfaz a pré-condição do `init`.
  - DEVS-114: `"edge": "bff"` no `dmpf.json`, `init --edge=bff` e regeneração
    dos arquivos do `identity` que dependem de `edgeApp`.
- [ ] **[P0] OpenAPI servido**: servir `/openapi/identity/v1/openapi.yaml` a
  partir de `OPENAPI_IDENTITY_PATH`. Teste estrutural garante que cada rota
  registrada existe no contrato com o método e o caminho declarados.
- [ ] **[P1] Admissão**: limite por rota e tenant, recusa 429
  `admission-refused`; tenants com balde próprio por `METRIC_TENANTS`
  (RES-16, RES-17).
- [ ] **[P1] CORS**: allowlist `CORS_ORIGINS`, vazia por padrão; no compose
  local, a origem do Swagger UI (`http://localhost:8082`).
- [ ] **[P1] Limites de forma**: corpo até 64 KiB e `DisallowUnknownFields`
  (TRP-49, TRP-51).
- [ ] **[P1] Saúde**: `/livez` e `/readyz` em `ADMIN_ADDR` (`:8090`), fora do
  contrato e sem autenticação; `/readyz` falha enquanto a conexão gRPC com o
  `identity` não estiver pronta.
  - DEVS-111: `/readyz` trivial, que só cai para 503 na drenagem.
  - DEVS-112: `/readyz` passa a depender da conexão gRPC com o `identity`.
  O papel `session-events` serve `/livez` e `/readyz` em `ADMIN_ADDR` (`:8092`):
  o `/readyz` dele só cai para 503 no desligamento (draining) e não depende do
  Redis nem do Keycloak, e a porta do receptor não serve saúde (404).
  `bff session-events healthcheck` lê só `ADMIN_ADDR` e sai com 0 ou 1.
- [ ] **[P1] Observabilidade**: `otelboot` próprio, access log, span por
  requisição e `X-Correlation-ID` propagado ou gerado e exposto na resposta.
  O registro `process configured` da API inclui `redis` (sem a senha) e
  `authn.revocation` (DEVS-111); `authn.revocation=enabled` mostra o checker
  ligado, não a consulta, que o `AUTH_DEV_MOCK` não faz. O papel
  `session-events` faz o próprio `boot.Boot` (`Service=bff`,
  `Role=session-events`, `ClassWrite`), com `dmpf.process.role=session-events`
  e `service.instance.id` próprio em `OTEL_RESOURCE_ATTRIBUTES`. O registro de
  configuração dele mostra o client secret só como presença e omite a senha do
  Redis (DEVS-114).
- [ ] **[P1] Deploy**: `Dockerfile`, `deploy/compose.yml` estendendo
  `dmpf-http-edge`, `deploy/.env.example` com `IDENTITY_GRPC_TARGET`, porta
  HTTP 8080 (`HTTP_PORT`, `infra/local/env.platform.example`), `deploy/k8s/base`
  e overlays `dev` e `hmg`, e targets Nx equivalentes aos da referência
  (`build`, `vet`, `fmt-check`, `tidy`, `test-race`, `govulncheck`, `serve`,
  `docker:run`, `deploy-env`, `k8s-render`).
  - DEVS-111: o `deploy/.env.example` traz
    `REDIS_URL=redis://revocation:dmpf-local-revocation@localhost:6379/0`
    (único usuário ACL que `infra/local/compose/redis.yml`, saída do
    dmpf-plugin, libera no Redis local). O `serve`
    depende de um target do workspace que sobe o profile `redis` do compose
    local, porque o `infra-session` e o `infra-up` da referência chamam
    `tools/infra-session.sh`, que não existe aqui. O `test-race` depende de um
    `test-infra-up` disponível no workspace (Redis de testes em 16379).
    Também entram o `deploy/infra.json` e as saídas do `infrasync` (ver Borda
    declarada) e os targets `redis-up`, `test-infra-up` e `test-infra-down`
    (ver Pontos a verificar, Targets de Redis).
  - DEVS-114: entram `deploy/.env.session-events.example` (portas
    `127.0.0.1:8091` e `127.0.0.1:8092`, issuer e client secret locais) e, no
    `.env.example`, as variáveis `OIDC_REVOCATION_TTL_SECONDS`,
    `OIDC_BACKCHANNEL_CLIENT_ID` e `OIDC_SESSION_EVENTS_CLIENT_ID`. O override
    de `deploy-env` passa a copiar os dois arquivos. Entram os targets
    `keycloak-up` e `serve-session-events` (dependente de `keycloak-up` e do
    mesmo target de Redis local que o `serve` usa), com o `serve` dependendo de
    `serve-session-events`. O `k8s-render` da referência renderiza os overlays
    de plataforma `infra/k8s/overlays/{dev,hmg}`, que o workspace não compõe
    (ver Pontos a verificar).
  - DEVS-114, compose: o `dmpf-bff` recebe `REDIS_URL` e `depends_on` do Redis,
    e entra o serviço `dmpf-bff-session-events` (estende `dmpf-app`, profile
    `keycloak`, `command: [session-events]`, healthcheck
    `/bff session-events healthcheck`).
  - DEVS-114, k8s base: `REDIS_URL` do Secret `redis` (chave `url`) no
    Deployment da API e `OIDC_REVOCATION_TTL_SECONDS: '300'` no ConfigMap
    `bff`. O Deployment `bff-session-events` tem uma réplica, `envFrom` do
    ConfigMap `bff`, `env` fixo para `SESSION_EVENTS_ADDR=:8091`,
    `ADMIN_ADDR=:8092`, os dois client IDs e o recurso OTel, `secretKeyRef` não
    opcional de `keycloak-session-events/secret` e `redis/url`, e portas de
    container 8091 (back-channel) e 8092 (administração, probes). O Service
    ClusterIP expõe só a 8091, e a NetworkPolicy, só de Ingress, admite pods
    `keycloak` na porta do back-channel.
  - DEVS-114, overlays: o patch `deployment-session-events-patch.yaml`
    (`OIDC_ISSUER` no dev; no hmg, `envFrom` de `bff` e `bff-oidc` e a CA
    montada) e `REDIS_REQUIRE_TLS: 'true'` no hmg.
  - Os nomes `bff-session-events` e `dmpf-bff-session-events` são obrigatórios,
    porque a URL de back-channel gerada pelo `init` deriva do `edge`.

#### Rotas do identity nesta versão

| Rota | RPC | Permissão | Chave |
| --- | --- | --- | --- |
| `POST /identity/organizations` | `RegisterOrganization` | `identity:register-organization` | sim |
| `POST /identity/organization-lookups` | `FindOrganizationByTaxId` | `identity:find-organization-by-tax-id` | sim, sem replay |
| `GET /identity/organizations/{organizationId}` | `FindOrganization` | `identity:find-organization` | não |
| `POST /identity/organizations/{organizationId}/establishments` | `AddEstablishment` | `identity:add-establishment` | sim |
| `GET /identity/organizations/{organizationId}/establishments` | `FindEstablishmentsByOrganization` | `identity:find-establishments-by-organization` | não |
| `POST /identity/organizations/{organizationId}/representations` | `RegisterRepresentation` | `identity:register-representation` | sim |
| `GET /identity/organizations/{organizationId}/representations` | `FindRepresentationsByOrganization` | `identity:find-representations-by-organization` | não |
| `POST /identity/organizations/{organizationId}/memberships` | `InviteUser` | `identity:invite-user` | sim |
| `GET /identity/organizations/{organizationId}/memberships` | `FindMembershipsByOrganization` | `identity:find-memberships-by-organization` | não |
| `POST /identity/organizations/{organizationId}/bank-account-changes` | `RequestBankAccountChange` | `identity:request-bank-account-change` | sim |
| `GET /identity/organizations/{organizationId}/bank-account-changes` | `FindBankAccountChangesByOrganization` | `identity:find-bank-account-changes-by-organization` | não |
| `POST /identity/persons` | `RegisterPerson` | `identity:register-person` | sim |
| `GET /identity/persons/{personId}` | `FindPerson` | `identity:find-person` | não |
| `GET /identity/representations/{representationId}` | `FindRepresentation` | `identity:find-representation` | não |
| `POST /identity/users` | `ProvisionUserAccount` | `identity:provision-user-account` | sim |
| `POST /identity/users/{userId}/suspend` | `SuspendUser` | `identity:suspend-user` | sim |
| `GET /identity/users/{userId}/memberships` | `FindMembershipsByUser` | `identity:find-memberships-by-user` | não |
| `GET /identity/users/{userId}/permissions` | `FindEffectivePermissions` | `identity:find-effective-permissions` | não |
| `POST /identity/memberships/{membershipId}/revoke` | `RevokeMembership` | `identity:revoke-membership` | sim |
| `GET /identity/bank-account-changes/{bankAccountChangeId}` | `FindBankAccountChange` | `identity:find-bank-account-change` | não |
| `POST /identity/bank-account-changes/{bankAccountChangeId}/approve` | `ApproveBankAccountChange` | `identity:approve-bank-account-change` | sim |

No catálogo congelado da SPEC-7KW0HHF6, só `operator` concede permissões
`identity:*`, e apenas três: `request-bank-account-change`,
`find-bank-account-change` e `find-bank-account-changes-by-organization`. As
outras 18 rotas ficam registradas e respondem 403 a todo usuário até a fatia de
papéis seguinte. É o comportamento esperado (fail-closed), não um defeito.

### Não-funcionais

- [ ] Segurança: toda rota de negócio exige token verificado e permissão
  declarada; nenhuma permissão é lida de claim; o acesso a id de outro tenant
  volta como 404 (ADR-051 do kernel).
- [ ] Segurança: sessão revogada é recusada em até 60 s (ADR-016). Com o
  verificador OIDC e o Redis de `REDIS_URL` fora ou lento além de
  `OIDC_REVOCATION_TIMEOUT_MS`, a borda recusa todo token verificado com 401
  (falha fechada, ADR-065 do kernel, itens 3 e 4), sem cache de "não
  revogado". O comportamento é garantido pelo `authn` do kernel.
- [ ] Tempo: o prazo de cada rota é declarado no `Budget` da rota, e o de cada
  método gRPC é estritamente menor (GRP-17); o deadline é propagado, nunca
  reiniciado (ADR-024 do kernel).
- [ ] Resiliência: com o `identity` fora, a borda responde 503 em vez de travar,
  e `/readyz` reprova.
- [ ] Compatibilidade: Go `1.26.9`, kernel na versão consumida pelo
  `dmpf.json`/`go.work`, mesmo runner de CI dos projetos `stack:go`
  (`dmpf-ci.yml`).

### Pontos a verificar na implementação

- O `surface.query` da referência é `GET`. Confirmar como registrar
  `POST /identity/organization-lookups` (consulta por `POST` com chave exigida e
  sem replay) sem burlar `ValidateEdge`.
- O teste estrutural da referência (`app/api/openapi_test.go:77`) exige os
  status 400/404/409/422/429/503/504/default; as consultas do `identity` não
  declaram 409/422. Ajustar o teste ao tipo da rota sem relaxar o contrato.
  Resolvido em 2026-10-17: o teste compara os status por tipo de rota (comando,
  consulta por id, consulta por `POST` e listagem, esta sem 404), com
  `Idempotent-Replayed` só em comando. Na DEVS-111, sem rota registrada, ele exige inventário vazio e
  exercita os tipos com rotas sintéticas; o inventário das 21 rotas (método,
  caminho e `operationId`) entra com as rotas, na DEVS-112 e na DEVS-113.
- Resolvido em 2026-10-17: `1.0.0-rc.5` (`workflowRef` `b5131ee`), com o bump
  feito no PR #4 (ADR-011). O `go.mod` do `bff` fixa as bibliotecas do kernel em
  `v1.0.0-rc.5`, inclusive `libs/backend/go/redis`.
- Pré-condições operacionais do papel `session-events` (DEVS-114), fora do
  repositório: reimport do realm em dev e hmg (ADR-065 do kernel, item 12),
  sem o qual o logout de usuário depende só do back-channel; Secrets
  `keycloak-session-events` e `redis` (chave `url`) fornecidos em hmg, porque o
  `secretKeyRef` deles não é opcional; Secret `bff-oidc` do hmg com
  `OIDC_AUDIENCE=dmpf-bff` e `OIDC_TENANT_CLAIM=organization.*.tenant_id`; e
  `OIDC_REVOCATION_TTL_SECONDS` igual ao `accessTokenLifespan` do realm (300).
- Overlay de plataforma (DEVS-114): no kernel, o Secret `redis` do dev (chaves
  `users.acl` e `url`) e a composição de `infra/k8s/base/redis`,
  `infra/keycloak` e do Component `infra/keycloak/dev` ficam num overlay de
  plataforma (`infra/k8s/overlays/dev/kustomization.yaml:12-36` no `b5131ee`)
  que o plugin não gera. Aqui, `infra/k8s/overlays/dev` tem só `apps/` e
  `databases/`, e `overlays/hmg`, só `apps/`. Definir se esse overlay entra na
  DEVS-114 ou fica fora, com o `k8s-render` limitado aos overlays do `bff`.
- Regeneração com `edge: bff` (DEVS-114): o `init` regrava `dmpf.json` e os
  arquivos gerenciados por `--edge=bff`, mas o generator `bounded-context`
  recusa módulo existente
  (`tools/dmpf-plugin/src/generators/bounded-context/generator.ts:404-405` no
  `b5131ee`), e o bump do rc.5 aplicou o delta dos templates do `identity` à
  mão (commit `d6b92e2`).
  Definir o procedimento que conta como regeneração para os arquivos do
  `identity` que dependem de `edgeApp` (por exemplo, renderizar num workspace
  descartável e copiar), sem ferir a P0 de arquivos gerados, e confirmar a
  forma de rodar o `infrasync` (`tools/dmpf-conformance`, tag `conformance`
  do `versions.json` do plugin).
- Prova ponta a ponta dos 60 s (DEVS-114): todos os clients do realm têm
  `directAccessGrantsEnabled: false`, e o redirect do `dmpf-portal` é só
  `http://127.0.0.1/callback` (`infra/keycloak/dmpf-realm.json`). O helper de
  PKCE do kernel vive em `_test.go` do `authn` e não é importável, e o e2e do
  `bff` de referência usa `AUTH_DEV_MOCK`. A prova do kernel está em
  `libs/backend/go/authn/revocation_keycloak_integration_test.go:184`
  (orçamento de 60 s em `:26`). Definir se o workspace escreve um helper
  próprio de fluxo de código com PKCE ou se fica com a garantia do kernel, a
  medição manual e os testes de fiação.
- Duas roles de banco (DEVS-114, ADR-063 do kernel): o harness do kernel sobe
  as APIs de contexto com `PG_DSN` de runtime e `PG_MIGRATE_DSN`
  (`apps/backend/bff/app/harness_test.go:280-288` no `b5131ee`), mas o
  `identity` daqui não lê `PG_MIGRATE_DSN` (só `MIGRATE=true` em
  `apps/backend/identity/deploy/.env.example:9`). Definir se o porte do e2e
  segue o `identity` atual ou espera a adoção da ADR-063, em spec própria.
- Targets de Redis (DEVS-111): definir o target que sobe o Redis local para o
  `serve` (nome e executor) e o `test-infra-up` do `test-race`. A referência
  usa `infra-session` (`tools/infra-session.sh`) e `testkit:test-infra-up`,
  ausentes aqui. O `identity` usa `test-race` com o executor
  `@mateusmacedo/dmpf-plugin:test-env` e um `test-infra-up` próprio
  (`@mateusmacedo/dmpf-plugin:test-infra`), e `infra/test/compose.yml` já tem
  o Redis em 16379.
  Resolvido em 2026-10-17: o projeto `bff` tem o target `redis-up`
  (`docker compose -f infra/local/docker-compose.yml --profile redis up -d
  --wait redis`), do qual o `serve` depende, e segue o padrão do `identity` no
  `test-race`, com `test-infra-up` e `test-infra-down` próprios.
- Riscos residuais do checklist de qualidade da DEVS-111. Resolvido em
  2026-10-18:
  - `DRAIN_DELAY` acima de 10 s recusa a partida, porque o
    `terminationGracePeriodSeconds` de 30 s precisa cobrir a drenagem e as
    duas janelas de `ShutdownGrace` (10 s) do kernel, a do `Shutdown` HTTP e a
    do flush da telemetria. Um segundo `SIGTERM` ou `SIGINT` encerra o
    processo na hora, sem o desligamento gracioso.
  - `HTTP_ADDR` ou `ADMIN_ADDR` fora de `host:porta`, ou os dois na mesma
    porta com host igual ou curinga, recusam a partida com exit 2.
  - Contrato OpenAPI vazio recusa a partida. O contrato sai com `ETag`, `304`
    a `If-None-Match` e gzip quando o cliente o aceita.
  - `SIGTERM` durante a descoberta OIDC encerra a partida sem erro.
  - A amostragem forçada pelo `traceparent` de um cliente anônimo fica com a
    SPEC-Z2HM6NAP do kernel (DEVS-58): tratá-la no `bff` contrariaria a
    decisão de continuar o trace do cliente.

## Camadas afetadas

| Camada | Afetada? | Descrição |
| --- | --- | --- |
| Borda HTTP (`bff/app/api`) | [x] | Rotas, cadeia de middleware, handlers, saúde |
| Cliente gRPC (`bff/app/rpc`) | [x] | Cliente do `IdentityService`, erros, metadata, replay, TLS |
| Autenticação (`bff/app`) | [x] | Verificador OIDC do kernel e decorador de permissões |
| Papel `session-events` (`bff/app`, `cmd`) | [x] | Receptor de back-channel e polling da Admin REST, gravando a deny-list no Redis |
| Contexto `identity` | [x] | Só arquivos gerados que dependem de `edgeApp` (regeneração) |
| Infra local e k8s | [x] | `dmpf.json`, `go.work`, `pki.yml` (infrasync), arquivos do `init` que dependem do `edge`, compose e overlays do `bff`, referências (`secretKeyRef`) aos Secrets `redis` e `keycloak-session-events`, fornecidos fora do `bff` (ver Pontos a verificar) |
| Domínio e aplicação | [ ] | Nenhuma mudança; o `bff` não tem domínio |
| Banco | [ ] | O `bff` não declara banco; usa o Redis compartilhado indicado por `REDIS_URL`, sem schema próprio |

## Localização de código

```text
apps/backend/bff/
  cmd/main.go                — binário único; sobe HTTP e porta de administração; subcomandos `healthcheck` (DEVS-111), `session-events` e `session-events healthcheck` (DEVS-114)
  app/config.go              — leitura de variáveis (HTTP_ADDR, ADMIN_ADDR, OIDC_*, OIDC_REVOCATION_TIMEOUT_MS, REDIS_URL, REDIS_DIAL_TIMEOUT_MS, REDIS_COMMAND_TIMEOUT_MS, REDIS_REQUIRE_TLS, IDENTITY_GRPC_TARGET, OPENAPI_IDENTITY_PATH, GRPC_*, CORS_ORIGINS, METRIC_TENANTS)
  app/wiring.go              — composition root: authn, `WithRevocations`, decorador, clientes, handler
  app/sessionevents.go, sessionevents_config.go — papel `session-events` (DEVS-114); reutiliza `newServer`, `serverAddress`, `keyDrainDelay`, `presence`, `readRedis`, `validateRedis` e `api.NewAdminHandler` da DEVS-111; o `session-events healthcheck` do `cmd/main.go` reutiliza `Probe`
  app/revocation_integration_test.go — token aceito, sessão gravada no Redis, mesmo token 401 (DEVS-112, sobre a primeira rota autenticada)
  app/authn.go               — Authenticator decorador com FindEffectivePermissions
  app/telemetry.go, probe.go — otelboot e prontidão
  app/api/routes.go          — bindings das 21 rotas do identity
  app/api/handlers_identity.go — tradução JSON <-> mensagens do IdentityService
  app/api/requests_identity.go — corpos dos comandos e da consulta por CNPJ, validação de forma pelo schema (DEVS-113)
  app/api/strict_body.go     — corpo estrito: nomes exatos, sem repetição nem `null`, UTF-8 e `maxItems` na leitura (DEVS-113)
  app/api/views_identity.go, views_identity_commands.go — respostas das consultas e dos comandos
  app/api/middleware.go, handlers.go, health.go, traceparent.go — porte da referência
  app/rpc/clients.go, errors.go, metadata.go, replay.go, ready.go, tls.go — porte da referência (tls.go na DEVS-111; replay.go e a chave na metadata na DEVS-113; os demais na DEVS-112)
  deploy/                    — compose.yml, .env.example, .env.session-events.example (DEVS-114), infra.json, k8s/base (com `*-session-events.yaml`), overlays dev e hmg (com `deployment-session-events-patch.yaml`)
  dmpf-units.json, project.json, go.mod (com `libs/backend/go/redis v1.0.0-rc.5`), Dockerfile, README.md
```

**Arquivos a modificar**:

- `dmpf.json` (`"edge": "bff"`) e os arquivos gerenciados do `init` que
  dependem do `edge` (`infra/local/env.platform.example`,
  `infra/local/compose/redis.yml`, `infra/local/compose/app-base.yml`,
  `infra/keycloak/dev/kustomization.yaml` e o dashboard `containers.json`): só
  por `pnpm nx g @mateusmacedo/dmpf-plugin:init --edge=bff`, depois de criar o
  `deploy/infra.json` do `bff`. O `init` também roda o `infrasync`; se acusar
  arquivo editado, avaliar antes de `--force`. O resultado tem
  `http://dmpf-bff-session-events:8091/backchannel-logout` no Compose e
  `http://bff-session-events:8091/backchannel-logout` no k8s dev.
- `go.work`: `use ./apps/backend/bff`.
- `apps/backend/identity/deploy/k8s/base/networkpolicy.yaml`,
  `.../overlays/hmg/configmap-identity-patch.yaml`,
  `apps/backend/identity/project.json`, `apps/backend/identity/README.md`: só
  por regeneração.
- `infra/local/compose/pki.yml`, `infra/local/.env.example` e demais arquivos
  do `infrasync`: só por `infrasync --write`, rodado na DEVS-111 assim que o
  `deploy/infra.json` do `bff` existe.
- `infra/local/docker-compose.yml`: incluir
  `../../apps/backend/bff/deploy/compose.yml` à mão (o `init` só cria o arquivo).

## Design

### Arquitetura

```text
cliente HTTPS --(Bearer)--> bff :8080
                             | authn.Verifier (kernel) -> sub, tenant; deny-list no Redis (revogado ou Redis fora -> 401)
                             | decorador -> IdentityService.FindEffectivePermissions (gRPC mTLS)
                             | ResolveIdentity confere route.Permission
                             | admissão, Idempotency-Key, prazo
                             v
                   IdentityService (gRPC :9090, mTLS, só spiffe://dmpf/bff)
```

Revogação de sessão:

```text
Keycloak --POST /backchannel-logout--> bff session-events :8091 --Feed.Apply--> Redis (deny-list)
bff session-events --client credentials--> Admin REST (/admin-events, /events?type=LOGOUT) a cada 5 s
bff :8080 (API, verificador OIDC) --depois de verificar o token--> Redis: revogado ou Redis fora -> 401
```

### Fluxo principal

1. A requisição entra; `otelhttp` abre o span e a rota é resolvida no mux.
2. O prazo da rota é aplicado; `ResolveIdentity` chama o decorador.
3. O decorador verifica o token (assinatura, issuer, audiência `dmpf-bff`,
   expiração, `sub` e deny-list no Redis). Com tenant nulo, devolve a identidade
   sem permissões e sem chamar o `identity`; senão, consulta
   `FindEffectivePermissions` com o tenant do token.
4. `ResolveIdentity` confere a permissão da rota; a admissão por rota e tenant
   decide; nos `POST`, a `Idempotency-Key` é validada e derivada.
5. O handler monta a mensagem gRPC, preenche `*ByUserId` com o `sub` e chama o
   RPC com a metadata da borda.
6. A resposta vira JSON; rejeição vira 422; falha técnica vira o status da
   tabela do kernel.

### Pseudocódigo do decorador

```text
Authenticate(ctx, credential):
  identity = verifier.Authenticate(ctx, credential)      # 401 se falhar ou se revogado
  if identity.Tenant == nil: return identity sem permissões  # 403 tenant-unresolved
  perms = identity_client.FindEffectivePermissions(ctx with x-tenant-id=identity.tenant, userId=identity.Subject)
  if erro técnico: devolve erro -> 503/504 (nunca segue)
  identity.Permissions = perms                           # claims descartadas
  return identity
```

## Decisões técnicas

- **Porte da referência do kernel, à mão**: o `@mateusmacedo/dmpf-plugin` só tem
  os generators `init` e `bounded-context`, e o agente `dmpf-context-author`
  não edita o `bff`. Alternativa descartada: pedir um generator de `bff` ao
  kernel antes, porque atrasa a borda sem mudar o resultado.
- **Permissões por decorador de `Authenticator`**: cumpre a SPEC-7KW0HHF6:59 sem
  alterar o kernel, porque `ResolveIdentity` autoriza só por
  `Identity.Permissions`, depois de exigir `Subject` e `Tenant`
  (`libs/backend/go/http/identity.go:41-53` no `b5131ee`).
  Alternativa descartada: `OIDC_PERMISSION_CLAIMS`, já rejeitada na
  SPEC-7KW0HHF6 por criar duas fontes de verdade.
- **Falha do identity nega**: sem permissões confirmadas não há autorização.
  Alternativa descartada: seguir com lista vazia, que esconde a indisponibilidade
  como 403.
- **Convenções do kernel sobre o doc 91**: o corpo de erro `{code,message}`,
  `Idempotent-Replayed` e o 422 para chave reusada seguem o kernel (ADR-022,
  item 1). O contrato publicado do `identity` já usa essa forma.
- **Caminhos sem `/v1`**: mantém o contrato publicado do `identity`
  (`/identity/...`). O kernel não decide versionamento de rota (ADR-024 do
  kernel deixa sem dona), e mudar o caminho quebraria o contrato já aprovado.
- **Dependências do kernel (satisfeitas no rc.5)**: a revogação em 60 s e o
  tenant da organização ativa eram condições da ADR-016, e o discovery decidiu
  não entregar sem elas. O `1.0.0-rc.5` cumpre as duas: a revogação pela
  ADR-065 do kernel e o tenant pela claim nativa `organization` (ADR-064 do
  kernel, item 2), sem mapper que achate nem extensão no Keycloak.
- **Papel `session-events` separado da API, com uma réplica**: a API só
  consulta a deny-list, e um subcomando da mesma imagem a alimenta (ADR-065 do
  kernel, item 6). O client secret do polling fica, entre os papéis do `bff`,
  só no pod do papel. Alternativa descartada: polling dentro da API, que faria
  cada réplica reler a janela e espalharia o segredo.
- **`REDIS_URL` desde o esqueleto (DEVS-111)**: a API do `bff` rc.5 não parte
  sem ele, inclusive com `AUTH_DEV_MOCK`, então o checker entra junto com a
  cadeia HTTP. O teste HTTP de revogação fica na DEVS-112, porque a DEVS-111
  não tem rota autenticada. O papel que alimenta a deny-list fica na
  integração de plataforma (DEVS-114).
- **`grantMembership` fora desta versão**: o contrato diz que o `bff` conduz a
  verificação do canal e dos dados pessoais e define `channelConfirmed` e
  `verificationLevel`, mas nenhum documento define como. Expor a rota exigiria
  inventar esse fluxo ou enviar valores não verificados.
- **e2e caixa-preta sem caminho feliz de negócio**: sem `org_admin` nem fluxo de
  primeiro vínculo (SPEC-7KW0HHF6:594), nenhum usuário obtém permissão pela API,
  e o e2e não lê nem semeia tabela de contexto (ADR-053 do kernel, §6). Os
  caminhos felizes ficam em testes com servidor gRPC falso (`bufconn`); o e2e
  real cobre autenticação, negação, contrato servido e saúde.

## Regras relacionadas

- `docs/adr/012-um-app-por-bounded-context.md`, `016-provedor-de-identidade-oidc.md`,
  `022-precedencia-dmpf-sobre-design.md`.
- ADR-064 e ADR-065 do kernel (no `b5131ee`): tenant pela claim nativa
  `organization` e revogação de sessão no `authn`.
- SPEC-7KW0HHF6: catálogo de papéis, `FindEffectivePermissions`, Escopo fora.
- SPEC-GXGASP24: ADRs de fundação.
- `.claude/rules/process-enforcement.md`: cadeia de validação Go.

## Verificação e testes

### Critérios de aceite

- [ ] `pnpm nx run bff:build`, `vet`, `fmt-check`, `test-race` e `govulncheck`
  passam; `dmpf-ci.yml` verde para `bff` e `identity`.
- [ ] O `bff` recusa partir com rota sem permissão ou `POST` sem chave
  (`ValidateEdge`).
- [ ] As 21 rotas existem no OpenAPI do `identity` com o mesmo método e
  caminho (teste estrutural).
- [ ] Todo método do `IdentityService` tem `MethodPolicy` (teste pelo
  descriptor).
- [ ] Token ausente, expirado, de issuer errado, sem `dmpf-bff` em `aud` ou de
  sessão revogada responde 401; usuário sem vínculo ativo no tenant do token
  responde 403 `permission-denied` em todas as 21 rotas.
- [ ] Token sem a claim `organization`, com mais de uma organização (por
  exemplo, `organization:*`) ou com `tenant_id` ausente ou multivalorado
  responde 403 `tenant-unresolved` em todas as 21 rotas, sem chamar
  `FindEffectivePermissions`.
- [ ] `FindEffectivePermissions` é chamada uma vez por requisição autenticada
  com tenant resolvido, e nunca em requisição sem token nem com tenant nulo
  (teste com servidor falso).
- [ ] Com o `identity` fora, a rota responde 503 e `/readyz` reprova.
- [ ] Replay de `POST` com a mesma chave devolve o mesmo status e corpo com
  `Idempotent-Replayed: true`; chave reusada com corpo diferente responde 422.
- [ ] (DEVS-111) Sem `REDIS_URL`, com URL inválida ou com banco diferente de 0,
  a API recusa partir com exit 2, nomeando `REDIS_URL` e sem ecoar a URL,
  inclusive com `AUTH_DEV_MOCK`. O registro `process configured` traz
  `authn.revocation=enabled` (checker ligado) e as chaves `redis.*` sem a senha.
  O `/readyz` da API não depende do Redis.
- [ ] (DEVS-112) Com issuer OIDC falso e Redis de testes, um token aceito numa
  rota autenticada passa a 401 `unauthenticated` depois que a sessão é gravada
  no Redis, e o registro `auth: token rejected` traz
  `dmpf.auth.refusal_reason=session_revoked`.
- [ ] (DEVS-114) O papel `session-events`, contra um realm falso, responde 400
  a logout token não verificável e 405 com `Allow: POST` a `GET`, lê as duas
  fontes do polling a cada ciclo e não serve saúde na porta do receptor (404),
  como no cenário 9; a gravação na deny-list vem do `Feed.Apply` do `authn` do
  kernel. A recusa em até 60 s contra o Keycloak de referência é garantida
  pelo kernel `1.0.0-rc.5`
  (`libs/backend/go/authn/revocation_keycloak_integration_test.go:184`; ADR-065
  do kernel, medida em cerca de 0,2 s pelo back-channel e 2 s pelo polling).
- [ ] (DEVS-114) `bff session-events` sem variável obrigatória sai com 2,
  nomeando-a e sem citar variável da API. Com o issuer fora da forma
  `<base>/realms/<realm>` ou sem descoberta, sai com 1.
  `bff session-events healthcheck` sai com 0 ou 1, e o registro de
  configuração do papel não contém o client secret nem a senha do Redis.
- [ ] `GET /openapi/identity/v1/openapi.yaml` devolve o contrato, e o Swagger UI
  local o carrega.
- [ ] Em hmg, a NetworkPolicy do `identity` admite só o pod do `bff` na porta
  9090, e `GRPC_TRUSTED_CLIENTS` é `spiffe://dmpf/bff`.

### Cenários de teste

1. **Caminho feliz (servidor falso)**: DADO um token válido de `sub` S no
   tenant T e `FindEffectivePermissions` devolvendo
   `identity:request-bank-account-change` QUANDO chega
   `POST /identity/organizations/{id}/bank-account-changes` com
   `Idempotency-Key` válida ENTÃO o RPC recebe `requestedByUserId = S`,
   `x-tenant-id = T` e a chave derivada, e a resposta é 201.
2. **Fail-closed**: DADO um token válido e `FindEffectivePermissions` devolvendo
   lista vazia QUANDO chega `GET /identity/organizations/{id}` ENTÃO a resposta
   é 403 `permission-denied` e o RPC `FindOrganization` não é chamado.
3. **Identidade forjada**: DADO um token do tenant T QUANDO a requisição traz
   `X-Tenant-ID` com outro tenant ENTÃO a resposta é 403 `identity-mismatch`.
4. **Rejeição de domínio**: DADO o RPC devolvendo `Rejection` com código
   `identity-organization/bank-account-change/...` QUANDO o handler traduz
   ENTÃO a resposta é 422 com esse código no corpo.
5. **Identity indisponível**: DADO o servidor gRPC fora QUANDO chega qualquer
   rota autenticada ENTÃO a resposta é 503 `unavailable` e nenhuma permissão de
   claim é usada.
6. **e2e real**: DADO `bff` e `identity` em container com mTLS, e um Redis
   fornecido ao `bff` por `REDIS_URL`, QUANDO um usuário sem vínculo
   chama as 21 rotas ENTÃO todas respondem 403, e
   `GET /openapi/identity/v1/openapi.yaml` responde 200.
7. **Revogação na API (DEVS-112)**: DADO um issuer OIDC falso com JWKS,
   `OIDC_TENANT_CLAIM=organization.*.tenant_id`, um token com
   `{"organization":{"<alias>":{"tenant_id":["T"]}}}` e o Redis de testes
   QUANDO a sessão do token aceito é gravada por
   `redis.RevocationStore.Revoke` ENTÃO a mesma requisição responde 401
   `unauthenticated`, com `refusal_reason=session_revoked` no registro.
8. **Sem organização ativa (DEVS-112)**: DADO um token verificado sem a claim
   `organization`, com duas organizações ou com `tenant_id` ausente ou
   multivalorado QUANDO chega qualquer das 21 rotas ENTÃO a resposta é 403
   `tenant-unresolved`, e `FindEffectivePermissions` não é chamada.
9. **Receptor e polling (DEVS-114)**: DADO o papel `session-events` contra um
   realm falso QUANDO chega `POST /backchannel-logout` com token não verificável
   ENTÃO a resposta é 400; QUANDO chega `GET` ENTÃO a resposta é 405 com
   `Allow: POST`; e a cada ciclo o polling lê as duas fontes (admin events e
   `LOGOUT`). Na porta de administração, `/livez` e `/readyz` respondem 2xx; na
   porta do receptor, 404.
10. **Desligamento e partida do papel (DEVS-114)**: DADO o SIGTERM QUANDO o
    papel retorna ENTÃO nenhum ciclo de polling continua e a porta de
    administração está fechada; DADO o issuer sem descoberta QUANDO o papel
    parte ENTÃO a partida falha.
11. **Medição de revogação (DEVS-114, opcional)**: condicionada ao ponto sobre
    a prova ponta a ponta. DADO o profile `keycloak`, o papel em container e a
    API em OIDC por um override próprio (`OIDC_ISSUER` do realm local,
    `OIDC_AUDIENCE=dmpf-bff`, `OIDC_TENANT_CLAIM=organization.*.tenant_id` e
    sem `AUTH_DEV_MOCK`), que a referência não tem, QUANDO o usuário faz logout
    no realm ENTÃO o mesmo token responde 401 na API em até 60 s.

<critical_constraints>
- [P0] O `bff` não importa domínio nem aplicação de contexto: só módulos
  `contract` dos contextos e bibliotecas do kernel (ADR-012; ADR-044 do kernel;
  regra `DMPF-D002`).
- [P0] Uma rota encaminha para exatamente uma chamada gRPC. Sem orquestração,
  sem saga, sem banco. O Redis não é banco do `bff`: é um recurso compartilhado
  da infraestrutura, no qual o provider `libs/backend/go/redis` guarda a
  deny-list que a API só consulta e o papel `session-events` só alimenta
  (ADR-065 do kernel, itens 1 e 6).
- [P0] Toda rota declara `Permission` e `ContractRef`; `ValidateEdge` recusa na
  partida rota sem permissão ou `POST` sem exigência de `Idempotency-Key`
  (IDN-16, IDN-17, RST-04).
- [P0] Permissões efetivas vêm só de `FindEffectivePermissions` do `identity`,
  a cada requisição e sem cache (SPEC-7KW0HHF6:59, CTX-16). Claims de
  permissão do token são descartadas.
- [P0] Sujeito e tenant vêm só do token verificado; `X-Subject-ID`,
  `X-Tenant-ID` ou `?tenant_id` divergentes recusam com 403 (CTX-06). Todo
  campo `*ByUserId` da mensagem gRPC recebe o `sub`.
- [P0] Nenhuma extensão do kernel nem do IdP é escrita neste workspace:
  deny-list (`redis.RevocationStore`), receptor de back-channel, polling de
  eventos de sessão, leitura do tenant (`organization.*.tenant_id`) e
  verificador OIDC vêm do kernel. No Keycloak, sem provider, script mapper, SPI
  próprio nem feature Preview, e sem mapper que achate o tenant (ADR-022, item
  3; ADR-064 do kernel, itens 1 e 2; ADR-065 do kernel).
- [P0] Arquivos gerados pelo `dmpf-plugin` e pelo `infrasync` mudam só por
  regeneração, nunca por edição manual.
- [P1] Corpo de erro, headers e status de idempotência seguem o kernel, não o
  doc 91 (ADR-022, item 1).
</critical_constraints>

## Escopo fora

- **Rotas dos outros 16 contextos**: cada contexto acrescenta as próprias rotas
  ao `bff` na sua spec ou task, quando o servidor gRPC dele existir.
- **`POST /identity/memberships/{membershipId}/grant`**: depende da verificação
  de canal e de dados pessoais conduzida pelo `bff`, sem definição em nenhum
  documento [ABERTA].
- **Partner API** (mTLS e credencial de cliente, doc 91 §4) e **webhooks de
  entrada e de saída** (doc 91 §5): sem ADR no kernel; a assinatura de saída
  depende de SPEC-5AQHHMJ0 e SPEC-MSA607GT do kernel.
- **Versionamento `/v1` nas rotas e paginação por cursor** (doc 91 §1): o kernel
  não decide; ficam como lacuna até uma norma.
- **Organização ativa por `X-Organization-Id`** (doc 91): o tenant vem só do
  token. A organização ativa é escolhida no pedido de autorização, pelo scope
  `organization:<alias>` (ou `organization`, com a escolha no login do
  Keycloak), e trocar de organização é obter outro token (SPEC-SBW7HT6Y do
  kernel).
- **Step-up e MFA por ação crítica** (`acr`, `auth_time`): o `authn` não lê esses
  claims [ABERTA: SPEC-7KW0HHF6:454].
- **Limites numéricos do doc 91** (600/min por usuário): a admissão do kernel é
  por rota e tenant; os valores ficam em configuração.
- **Leitura entre tenants do backoffice**: SPEC-QVDGV6J0 (DEVS-100).
- **Primeiro vínculo e papéis `approver`, `org_admin`, de financiador e de
  plataforma**: Escopo fora da SPEC-7KW0HHF6.
- **Ambiente de produção**: só dev e hmg pelo baseline (ADR-019).
- **Frontend e telas**: fora do workspace backend.
- **Revogação sem evento lido**: ficam cobertos só pela vida do token o
  `logout-all` do realm, a revogação por SCIM ou federação sem evento lido pelo
  polling, a expiração de sessão e o logout sem evento guardado com o
  back-channel perdido (ADR-065 do kernel, item 16). O polling não escala
  horizontalmente: o papel tem uma réplica por desenho.
