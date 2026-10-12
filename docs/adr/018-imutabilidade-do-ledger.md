# ADR-018: Garantir a imutabilidade do Compliance Ledger em camadas — permissão, hash encadeado e selo em WORM

## Status

Aceito — 2026-10-12, para o mecanismo base; storage concreto, âncora externa e
extensões do kernel em Pendências. Implementa SPEC-GXGASP24 (DEVS-76).

## Contexto

A decisão D7 vem de `docs/design/00-arquitetura-tecnica.md` §5.2: as classes
"versionado imutável", "evidência" e "auditoria" não recebem `UPDATE` nem
`DELETE`, com imposição por permissão de banco. O PRD do Compliance Ledger pede
append-only, detecção de sequência ausente, duplicada ou inválida, "WORM ou
controle equivalente" para evidência crítica, chaves segregadas e exportação
assinada. `docs/design/12-design-compliance-ledger.md` §3 propõe permissão de
banco, hash encadeado, retenção imutável no storage e notarização externa, e
admite que um administrador pode recalcular a cadeia inteira. O modelo de ameaça
(`docs/design/94-seguranca-threat-model.md`, T6) lista os controles C6.1 a C6.6.
O volume esperado é de 8.000 registros por dia no MVP e 600.000 na fase 3.

O kernel DMPF não decide o mecanismo (classe **aberta**), mas o restringe:

- **A role do app é dona do banco**, sem ser superuser no rc.4
  (`infra/local/compose/provisioning.generated.yml:26-29`; o superuser do
  ADR-051 é anterior ao banco por app), e o dono pode desligar trigger e RLS.
- **O `postgres.Table` cria por `INSERT … ON CONFLICT DO NOTHING` e altera por
  `UPDATE … WHERE version = $3`** (`libs/backend/go/postgres/table.go:182-191`):
  a cabeça de cada partição da cadeia cabe como linha versionada, e só lê o
  tenant do portador (ADR-051).
- **`domain` não admite `encoding/json`** (`.golangci.yml:135-136`) **e
  `application` não admite `crypto/sha256`**, que entra por função injetada
  (ADR-056).
- **DAT-25**: auditoria com retenção, acesso e integridade próprios. **DAT-14 a
  DAT-18**: retenção ≤ teto de origem declarada, o mais estrito prevalece, purga
  com evidência.
- O kernel não tem porta nem provider de object storage nem de assinatura; a
  nuvem-alvo é a AWS (ADR-019).

## Decisão

1. **Permissão**: `audit_records` é escrita pela role de runtime com `INSERT` e
   `SELECT`, mais trigger que recusa `UPDATE`, `DELETE` e `TRUNCATE`; a role
   dona e migradora é outra role, fora do runtime.
2. **Hash encadeado por partição** `(tenantId, recordType)`:
   `recordHash = "sha256:" + hex(SHA-256(previousRecordHash ‖ c14n(registro)))`,
   com `c14n` igual à RFC 8785 sobre o registro sem o próprio `recordHash`, e todo
   inteiro de 64 bits (os instantes em nanossegundos) como string decimal, porque
   a RFC 8785 serializa número como double e perde precisão acima de 2^53. Hash e
   `c14n` entram no caso de uso por função injetada, como o `Digest` do ADR-056.
   Dado pessoal não entra na pré-imagem: o registro o referencia por hash.
3. **Selo periódico** (`SealManifest`): raiz Merkle das cabeças de todas as
   partições ativas e hash do selo anterior, assinado por chave assimétrica em
   AWS KMS e gravado em S3 Object Lock no escopo de plataforma da porta de
   object storage. O selo contém só hashes.
4. **Storage**: selos em COMPLIANCE, com prazo inicial declarado ≤ teto e
   prorrogação a cada novo prazo (COMPLIANCE só cresce). Evidência bruta em
   GOVERNANCE até os prazos de retenção serem conhecidos, promovida a
   COMPLIANCE, ou regravada nesse modo, quando forem; teto superveniente mais
   estrito é cumprido pelo caminho administrativo auditado da porta (DAT-16,
   DAT-18). Toda referência a objeto guarda `versionId` e `contentHash`.
5. **Contrato neutro de nuvem**: o ledger usa as portas de object storage e de
   assinatura do kernel, nunca o SDK da AWS direto (ADR-019).
6. **Verificação**: um verificador somente leitura confronta a cadeia no banco
   com os selos, com alarme em qualquer descontinuidade (C6.4).
7. **Purga**: expirado o prazo, a role migradora remove o início da cadeia de uma
   partição, grava o hash do último registro removido como âncora da cabeça e
   registra a evidência da purga (DAT-18).

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Só `REVOKE` na role atual | A role é dona do banco; o controle viraria convenção |
| Amazon QLDB | Fim de suporte em 31/07/2025 |
| Azure SQL Ledger, Azure Confidential Ledger | Fora da nuvem-alvo e incompatíveis com o PostgreSQL imposto |
| immudb | Segunda base de dados fora do kernel, licença BSL 1.1 |
| MinIO para WORM em dev e hmg | Repositório arquivado; o floci cobre a emulação (ADR-019) |
| Notarização por cabeça de cada partição (doc 94, C6.6) | Não escala com dezenas de milhares de partições; o selo por período com raiz Merkle cobre todas |
| Serialização Protobuf determinística como forma canônica | A documentação do Protobuf não garante forma canônica entre implementações e versões |

## Consequências

**Positivas:**

- Prevenção pelo banco, detecção pela cadeia e prova contra recálculo interno
  pelo selo em WORM; a purga cumpre o teto sem quebrar a cadeia restante.

**Negativas:**

- Quatro extensões do kernel antes de o Compliance Ledger escrever em produção.
- O DSN da role migradora fica no ambiente do processo que migra; a camada 1 não
  protege contra esse processo comprometido (risco declarado na SPEC-QPRR24C4).
- COMPLIANCE protege contra a aplicação e o operador, não contra a própria AWS.
- Administradores distintos para as chaves (C6.5) exigem conta separada, prevista
  para a fase 3. O teste de contrato contra a AWS real é obrigatório (ADR-019).

## Pendências

- **Camada 1**: SPEC-QPRR24C4 (DEVS-94). **Storage**: SPEC-CWBA4BPV (DEVS-95),
  com escopo de plataforma e caminho administrativo. **Selo**: SPEC-MSA607GT
  (DEVS-99). **Leitura das cabeças entre tenants** pelo selo e pelo verificador:
  SPEC-QVDGV6J0 (DEVS-100). Todas no repositório do kernel.
- **Âncora externa** RFC 3161 por ACT da ICP-Brasil: fase 2, sem spec dona ainda.
- **Prazos de retenção** (`#8`, `A13`): fixam o prazo dos selos e a promoção da
  evidência a COMPLIANCE. Se a plataforma for instituição autorizada (`A1`), a
  Circular BCB 3.978/2020, art. 67, pode impor dez anos.
- **Ajustes na SPEC-59BCT7T1**: selo, período e verificador, que ela deixa fora
  da versão; e os campos mutáveis `integrity` e `lastVerifiedAt`, que contrariam
  a proibição de `UPDATE` e passam a registro próprio.

## Referências

- SPEC-GXGASP24, SPEC-59BCT7T1 — `docs/specs/`; [ADR-019](./019-go-sobre-o-kernel-dmpf.md)
- Kernel DMPF, no `workflowRef` do rc.4 (`487e9de`), base `https://github.com/mateusmacedo/dmpf/blob/487e9de17099c0eb8d44e3e9c2dfbd58b577b958/`:
  - `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md`, `docs/adr/056-idempotencia-de-comando-pela-inbox.md`, `infra/local/compose/provisioning.generated.yml`, `.golangci.yml`, `libs/backend/go/postgres/table.go`
  - DAT-14 a DAT-18, DAT-25 — `docs/dmpf/contexto-erros-seguranca.md`
- S3 Object Lock — `https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html`
- RFC 8785 — `https://www.rfc-editor.org/rfc/rfc8785`; RFC 3161 — `https://www.rfc-editor.org/rfc/rfc3161`; Protobuf sem forma canônica — `https://protobuf.dev/programming-guides/serialization-not-canonical/`
- Circular BCB 3.978/2020 — `https://normativos.bcb.gov.br/Lists/Normativos/Attachments/50905/Circ_3978_v6_P.pdf`
