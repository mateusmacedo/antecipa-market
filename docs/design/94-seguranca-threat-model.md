# Segurança e Modelo de Ameaças

> **Camada**: construtiva
> **Origem**: lacuna identificada — `01-prd-identity-organization.md` tem uma
> seção de cinco linhas sobre segurança e privacidade;
> `04-prd-receivables-gateway.md` cita mTLS e segregação de credenciais;
> `12-prd-compliance-ledger.md` cita WORM e criptografia. Não há modelo de
> ameaças, não há classificação de superfície de ataque e não há tratamento dos
> vetores específicos deste domínio.
> **Status**: proposta

## 1. O que está sendo protegido

O ativo de maior valor não é o dinheiro — a plataforma não o custodia. É a
**capacidade de constituir efeito jurídico sobre recebíveis de terceiros**. Quem
comprometer a plataforma não rouba saldo; consegue algo mais durável: gravar ônus
sobre ativos alheios, redirecionar desembolso, ou obter a base de clientes e
preços de financiadores concorrentes.

| Ativo | Valor para o atacante | Impacto se comprometido |
| --- | --- | --- |
| Capacidade de enviar comando à infraestrutura | Muito alto | Efeito jurídico indevido, irreversível sem processo |
| Destino do desembolso | Muito alto | Desvio direto de recursos |
| Credenciais de parceiro (mTLS, tokens) | Muito alto | Personificação da plataforma perante infraestrutura |
| Base de cedentes e agendas | Alto | Vantagem competitiva; venda de dados |
| Ofertas e taxas de financiadores | Alto | Colusão; vazamento entre concorrentes |
| Dossiês KYB e dados pessoais | Alto | Sanção regulatória; fraude de identidade |
| Integridade do ledger | Alto | Perda de valor probatório; encobrimento de fraude |
| Disponibilidade | Médio | Perda comercial, sem dano permanente |

**Observação sobre a integridade do ledger**: seu comprometimento é o único que
degrada a capacidade de detectar todos os outros. Por isso recebe tratamento
distinto (seção 6).

## 2. Fronteiras de confiança

```text
[Navegador do cedente]
        | TLS, OIDC
[Client API]  <-- fronteira 1: usuário externo, autenticado, não confiável
        |
[Núcleo de domínio]
        |
[Partner API] <-- fronteira 2: sistema externo, mTLS, semi-confiável
        |
[Receivables Gateway]
        | mTLS, credencial por parceiro e ambiente
[Infraestruturas externas] <-- fronteira 3: autoridade externa, confiável quanto
        |                       ao conteúdo, não quanto à disponibilidade
[Webhooks recebidos] <-- fronteira 4: entrada não solicitada; a mais exposta
        |
[Backoffice] <-- fronteira 5: usuário interno com poder amplo; a mais perigosa
```

**Fronteira 4 é a mais exposta**: o endpoint de webhook é público por
necessidade, recebe tráfego não solicitado e processa conteúdo que altera estado
de negócio. Um webhook forjado que declare `EffectConfirmed` libera desembolso.

**Fronteira 5 é a mais perigosa**: o backoffice tem, por desenho, capacidade de
reenviar comando externo, resolver divergência e ajustar conciliação. Uma conta
de operações comprometida é funcionalmente equivalente a comprometer a
plataforma.

## 3. Ameaças por vetor

### T1 — Webhook forjado declarando confirmação de efeito

**Cenário**: atacante envia ao endpoint público uma mensagem que simula
confirmação de efeito de um parceiro. A plataforma marca o efeito como
`EFFECTIVE`, libera Funding, e o financiador desembolsa sobre efeito inexistente.

**Impacto**: crítico. Desembolso sem garantia constituída.

**Controles**:

| # | Controle |
| --- | --- |
| C1.1 | Verificação criptográfica de origem antes de qualquer interpretação: mTLS com certificado do parceiro, ou assinatura do payload com chave do parceiro |
| C1.2 | Correlação obrigatória: o callback só é aceito se referenciar comando previamente enviado por esta plataforma, com o identificador que ela gerou |
| C1.3 | Confirmação de efeito jamais deriva exclusivamente de mensagem recebida — exige consulta de estado confirmando a posição |
| C1.4 | Restrição de origem de rede quando o parceiro oferecer faixa fixa |
| C1.5 | Alarme em callback sem comando correlacionado |

**C1.3 é o controle decisivo, e tem custo.** Consultar após cada callback dobra o
tráfego com o parceiro e adiciona latência. A alternativa — confiar na
assinatura — concentra todo o risco na gestão da chave do parceiro, que a
plataforma não controla. Dado que o efeito é irreversível e o desembolso o segue,
o custo se justifica.

**Aplicação diferenciada**: confirmação que libera desembolso exige consulta.
Notificação informativa (mudança de posição, por exemplo) pode operar só com
assinatura.

### T2 — Desvio de desembolso por alteração de destino

**Cenário**: atacante com acesso à conta do cedente altera a conta de destino
antes do desembolso. O financiador paga à conta do atacante. Vetor clássico e o
de maior perda esperada.

**Controles**:

| # | Controle |
| --- | --- |
| C2.1 | Alteração de conta exige dupla aprovação, sem exceção (ver `95-personas-jornadas.md`, matriz) |
| C2.2 | Quarentena: conta recém-cadastrada ou alterada não recebe desembolso antes de prazo configurado |
| C2.3 | Validação de titularidade: a conta pertence ao CNPJ do cedente, verificada por fonte independente |
| C2.4 | Notificação por canal independente (e-mail e telefone cadastrados anteriormente) a todos os administradores |
| C2.5 | Alteração após seleção de oferta e antes do desembolso bloqueia a operação e exige reconfirmação |
| C2.6 | Confirmação de desembolso compara `destinationHash` com o validado; divergência abre caso de severidade máxima |
| C2.7 | Histórico de contas preservado; retorno a conta anteriormente usada tem tratamento menos restritivo |

**C2.5 fecha a janela mais crítica**: entre assinatura e desembolso, o contrato já
está fechado e o dinheiro ainda não saiu. `10-prd-funding.md` menciona "mudança de
conta após assinatura exige controle reforçado" sem definir — a definição é:
bloqueio da operação em curso, não apenas aprovação adicional.

### T3 — Uso de representação revogada ou fora de alçada

**Cenário**: procuração revogada, ou representante assina operação acima do
limite de poderes. O contrato é questionável e o efeito, potencialmente nulo.

**Controles**:

| # | Controle |
| --- | --- |
| C3.1 | Verificação de vigência no instante da assinatura, não na criação do contrato |
| C3.2 | Verificação de alçada, quando o documento de poderes a impõe (ver `90-modelo-de-dados.md`, 3.1) |
| C3.3 | `Representation` imutável: alteração revoga e cria nova, preservando qual poder existia em cada instante |
| C3.4 | Revogação propagada em menos de 60 s a todas as sessões ativas |
| C3.5 | Evento carrega `representationId`; ausência impede a ação, não a registra como anônima |

**Lacuna que persiste**: a plataforma verifica a representação que conhece. Se a
procuração foi revogada em cartório e ninguém informou, não há como saber.
Mitigação parcial: revalidação periódica por fonte externa, quando disponível, e
declaração de que a responsabilidade pela atualização é do cliente — o que precisa
estar no contrato de uso, não apenas no sistema.

### T4 — Vazamento entre financiadores concorrentes

**Cenário**: financiador A obtém a base de cedentes, volumes e taxas do
financiador B. Dano comercial grave e potencial problema concorrencial.

**Controles**:

| # | Controle |
| --- | --- |
| C4.1 | Isolamento por linha com RLS (`00-arquitetura-tecnica.md`, 4.1) — consulta sem contexto retorna vazio, não dados alheios |
| C4.2 | Divulgação em níveis: triagem sem identificação do cedente (`91-contratos-api.md`, 4.1) |
| C4.3 | Oferta de um financiador nunca visível a outro, em nenhum nível de agregação |
| C4.4 | Métricas agregadas de mercado, se existirem, com limiar mínimo que impeça reidentificação |
| C4.5 | Trilha de acesso a dados de pedido, com detecção de volume anômalo de consulta |
| C4.6 | Ausência de rota que aceite identificador de outro tenant, mesmo autenticada |

**C4.4 merece atenção**: exibir ao cedente "sua oferta está X% acima da média do
mercado" parece útil e vaza informação de precificação de concorrentes. Com poucos
financiadores conectados, uma média é quase reversível. Só faz sentido com número
suficiente de participantes e limiar declarado.

### T5 — Comprometimento de credencial de parceiro

**Cenário**: chave privada ou token usado para falar com a infraestrutura é
obtido. O atacante envia comandos como se fosse a plataforma.

**Controles**:

| # | Controle |
| --- | --- |
| C5.1 | Credenciais em cofre externo; o domínio referencia por identificador (`13-prd`, invariante) |
| C5.2 | Segregação por parceiro **e** ambiente, sem caminho de leitura entre ambientes |
| C5.3 | Rotação máxima de 90 dias; certificado com sobreposição de 30 dias |
| C5.4 | Nenhuma credencial em variável de ambiente de processo de longa duração — obtenção sob demanda com cache curto |
| C5.5 | Chave privada em módulo de segurança quando o parceiro suportar |
| C5.6 | Alarme em uso de credencial fora do padrão de horário ou volume |
| C5.7 | Procedimento de revogação de emergência testado, com prazo declarado |

**C5.7 é frequentemente esquecido**: revogar certificado com um parceiro exige
coordenação com ele. Se o procedimento não foi combinado antes, o tempo de
resposta a um comprometimento é de dias. O procedimento de revogação deveria ser
item de homologação em `13-prd-partner-management.md`.

### T6 — Adulteração do ledger

**Cenário**: quem tenha acesso ao banco altera ou remove registro de auditoria
para encobrir ação indevida.

**Controles**:

| # | Controle |
| --- | --- |
| C6.1 | Encadeamento por hash (`90-modelo-de-dados.md`, 3.5): alteração invalida todos os registros posteriores |
| C6.2 | Permissão de banco sem `UPDATE` nem `DELETE` para a aplicação; escrita só por inserção |
| C6.3 | Evidências em armazenamento com retenção imutável imposta pelo armazenamento, não pela aplicação |
| C6.4 | Verificação periódica da cadeia, com alarme em qualquer descontinuidade |
| C6.5 | Chaves de criptografia do ledger segregadas das do transacional, com administradores distintos |
| C6.6 | Notarização externa periódica do hash da cabeça de cada partição — **decisão aberta** |

**Limite honesto de C6.1**: o encadeamento detecta adulteração posterior; não a
impede, e não protege contra quem tenha acesso para recalcular a cadeia inteira.
Proteção real contra administrador com acesso total exige publicação externa
(C6.6) ou armazenamento em que nem o administrador possa sobrescrever (C6.3).
Declarar isso importa: um encadeamento apresentado como prova absoluta é pior que
nenhum, porque gera confiança indevida.

### T7 — Abuso do backoffice

**Cenário**: operador interno, comprometido ou mal-intencionado, resolve
divergência de conciliação a favor de terceiro, reenvia comando ou ajusta
expectativa de liquidação.

**Controles**:

| # | Controle |
| --- | --- |
| C7.1 | MFA obrigatório e sessão curta para todos os papéis de plataforma |
| C7.2 | Alçada por valor: acima do limite exige segundo aprovador |
| C7.3 | Ajuste nunca altera o dado bruto — cria registro de ajuste vinculado (`11-prd`, invariante) |
| C7.4 | Justificativa obrigatória, em texto livre, com revisão periódica por amostragem |
| C7.5 | Acesso a dado de cliente exige motivo declarado, registrado e associado a caso aberto |
| C7.6 | Segregação: quem trata conciliação não administra parceiros nem credenciais |
| C7.7 | Revisão periódica de acessos privilegiados, com remoção do que não for usado |

**C7.5 muda a postura de acesso**: em vez de "operador vê tudo do seu escopo", o
acesso é vinculado a caso. Consultar dados de um cliente sem caso aberto é
possível, mas produz registro destacado e revisável. Isso não impede o abuso —
torna-o visível.

### T8 — Sequestro de conta do cedente

**Cenário**: credencial do usuário do cedente é obtida por phishing ou
reutilização de senha. O atacante opera como o cliente.

**Controles**:

| # | Controle |
| --- | --- |
| C8.1 | MFA obrigatório para `APPROVER` e `ORG_ADMIN` |
| C8.2 | Ações críticas exigem reautenticação, mesmo com sessão ativa |
| C8.3 | Notificação a todos os administradores em: novo dispositivo, alteração de conta, novo representante, primeira operação |
| C8.4 | Detecção de sessão anômala (geografia, dispositivo, horário) |
| C8.5 | Separação entre quem opera e quem aprova limita o dano de uma única conta |
| C8.6 | Convite de usuário sempre validado por identidade, nunca só por posse de e-mail |

**C8.5 é o controle estrutural**: com segregação efetiva, comprometer uma conta
de operador não conclui uma operação. Comprometer uma de aprovador não a inicia.
É o argumento mais forte para não permitir que uma organização opere com um só
usuário acumulando papéis — e a razão pela qual a dispensa de dupla aprovação
precisa de aceitação explícita de risco.

### T9 — Reenvio indevido causando efeito duplicado

**Cenário**: comando em `UNKNOWN_OUTCOME` é reenviado, o original havia surtido
efeito, e constitui-se segundo ônus sobre o mesmo ativo.

**Controles**:

| # | Controle |
| --- | --- |
| C9.1 | Chave idempotente derivada de dados de negócio (`00-arquitetura-tecnica.md`, 7.3) |
| C9.2 | `UNKNOWN_OUTCOME` proíbe reenvio automático — resolução só por consulta |
| C9.3 | Reenvio manual pelo backoffice exige consulta prévia com resultado registrado |
| C9.4 | Parceiro sem operação de consulta: comando classificado como não reenviável sem intervenção |
| C9.5 | Alarme em efeito duplicado detectado na posição externa |

**Já bem tratado nos PRDs** — `04-prd` e `09-prd` são explícitos. O que falta é a
imposição mecânica: a operação de reenvio no backoffice deve exigir o
identificador da consulta como parâmetro obrigatório, não confiar no procedimento.

### T10 — Exfiltração por API legítima

**Cenário**: usuário ou parceiro autenticado extrai volume massivo de dados por
uso normal da API, dentro de suas permissões.

**Controles**:

| # | Controle |
| --- | --- |
| C10.1 | Limites por escopo e por operação (`91-contratos-api.md`, 6) |
| C10.2 | Detecção de padrão anômalo: volume, horário, enumeração sequencial |
| C10.3 | Exportação em massa é operação distinta, com autorização própria e registro |
| C10.4 | Marca d'água por destinatário em dossiês exportados, quando o formato permitir |
| C10.5 | Trilha de acesso ao próprio ledger (`12-prd`, requisito 6) |

## 4. Tratamento de dados pessoais

| Requisito | Implementação |
| --- | --- |
| Minimização | Coleta limitada ao necessário ao produto e aos parceiros selecionados (`02-prd`, requisito 2) |
| Mascaramento em log | Aplicado na serialização; o dado não chega ao log |
| Criptografia em repouso | Envelope adicional para `PII` e `PII_SENSITIVE`, com chave distinta |
| Compartilhamento | Somente escopo autorizado; nível de divulgação por evento |
| Retenção | Por categoria, com fundamento jurídico declarado (`93-requisitos-nao-funcionais.md`, 6) |
| Eliminação | Sujeita a obrigação de retenção; a resposta ao titular declara o fundamento da recusa parcial |
| Portabilidade | Exportação em formato estruturado — ausente de todos os PRDs |
| Incidente | Procedimento com prazo de comunicação — ausente de todos os PRDs |

**Duas lacunas concretas**: nenhum PRD trata de portabilidade nem de resposta a
incidente de dados. Ambos são obrigações, não opções.

## 5. Ciclo de desenvolvimento

| Controle | Definição |
| --- | --- |
| Revisão de código | Obrigatória; mudanças em autenticação, autorização, criptografia e Gateway exigem revisor com conhecimento de segurança |
| Análise estática | Em cada mudança, bloqueando classificação alta |
| Dependências | Verificação contínua; correção conforme prazos de `93-requisitos-nao-funcionais.md`, 7 |
| Segredos no repositório | Verificação automatizada bloqueando o envio |
| Teste de penetração | Periodicidade definida; obrigatório antes da primeira operação em produção |
| Ambiente | Credencial de produção inexistente fora de produção |
| Dados em ambiente inferior | Sintéticos ou anonimizados; jamais cópia de produção |

## 6. Resposta a incidente

Ausente de todos os PRDs. Estrutura mínima:

| Fase | Requisito |
| --- | --- |
| Detecção | Alarmes da seção 3 com destinatário definido e prazo de resposta |
| Classificação | Severidade por impacto: efeito indevido e desvio de recurso são severidade máxima |
| Contenção | Suspender capacidade de parceiro, revogar sessão, bloquear operação — sem parar a plataforma inteira |
| Comunicação | Matriz por tipo: cliente, parceiro, autoridade, titular de dados |
| Preservação | O ledger é evidência; congelamento por `LegalHold` antes de qualquer correção |
| Reparação | Correção gera novo registro; nunca altera o anterior |
| Revisão | Análise posterior sem atribuição de culpa, com ações rastreadas |

**Particularidade deste domínio**: efeito jurídico constituído indevidamente não
se desfaz tecnicamente. A contenção é operacional e jurídica — solicitar
liberação do efeito à infraestrutura, comunicar as partes, abrir disputa. O
procedimento precisa estar acordado com as infraestruturas antes de ser
necessário.

## 7. Lacunas

| # | Lacuna | Impacto |
| --- | --- | --- |
| S1 | Mecanismo de validação de titularidade de conta (C2.3) | Alto — depende de provedor disponível |
| S2 | Viabilidade de exigir consulta após todo callback crítico (C1.3) | Alto — depende de capacidade do parceiro |
| S3 | Notarização externa do ledger (C6.6) | Médio |
| S4 | Procedimento de revogação de emergência com cada parceiro (C5.7) | Alto |
| S5 | Procedimento de resposta a incidente com as infraestruturas (6) | Alto |
| S6 | Portabilidade e resposta a titular de dados (4) | Alto |
| S7 | Limiar de agregação para métricas de mercado (C4.4) | Baixo |

---

## Rastreabilidade

| Seção | PRD de origem | Lacuna fechada |
| --- | --- | --- |
| 2 | Nenhum | Fronteiras de confiança inexistentes |
| T1 | `04-prd-receivables-gateway.md` | Webhook tratado como entrada confiável |
| T2 | `10-prd-funding.md` | "Controle reforçado" sem definição |
| T3 | `01-prd-identity-organization.md`, `08-prd-contracting.md` | Alçada de representação |
| T4 | `01-prd-identity-organization.md` | "Proibição de acesso cruzado" sem mecanismo |
| T5 | `13-prd-partner-management.md` | Revogação de emergência |
| T6 | `12-prd-compliance-ledger.md` | Detecção de adulteração sem mecanismo |
| T7 | `11-prd-settlement-reconciliation.md` | Backoffice sem modelo de ameaça |
| T9 | `04`, `09` | Imposição mecânica do procedimento de reenvio |
| 4 | `02-prd-kyb-kyc.md` | Portabilidade e incidente ausentes |
| 6 | Nenhum | Resposta a incidente ausente |
