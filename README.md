# Kotoro

> Produce, evolve, conquer! Improve yourself with kotoro, create study sprints, and optimize your development.

Kotoro é uma ferramenta de produtividade com o objetivo de facilitar a organização da rotina diária de estudos, o acompanhamento do desenvolvimento do estudante e a evolução guiada por métricas de desempenho.

Com Kotoro, você poderá adicionar matérias de qualquer área do conhecimento, criar sprints diárias com blocos formados pelas matérias, implementar metas e acompanhar suas conquistas acadêmicas.

## Funcionalidades

- Gerenciamento de matérias
- Montagem de sprints diárias
- Modo de estudo em tempo real
- Gerenciamento de metas
- Acompanhamento de métricas

## Sprints

As sprints são modelos de execução de uma coletânea de blocos de matérias selecionadas pelo estudante.

Nesse caso, o estudante pode fragmentar seu dia de estudos e seus ciclos de estudo por matéria em sprints.

Para exemplificar melhor, imagine que sou um estudante de vestibular. No meu dia atual de estudos, de acordo com meu cronograma, selecionei álgebra, filosofia, cinemática, matemática básica e sociologia.

Para cada matéria, no meu ritmo atual, geralmente separo de 50 minutos a 1 hora por ciclo de aprendizado e organizo a ordem por sinergia entre as matérias ou grau de esforço.

Isso é muito difícil de administrar sem uma plataforma específica. Por isso, Kotoro separa essa rotina em sprints para que o estudante competitivo gerencie e organize seus estudos por meio de um painel de visualização.

O estudante terá blocos de sprint com ciclos de matérias que ele irá selecionar.

Tecnicamente, cada sprint representa o planejamento de um único dia. A sprint possui uma sequência ordenada de blocos, cada bloco referencia uma matéria e pode conter uma lista de tarefas. A execução acontece em ciclos de estudo, e uma mesma sprint pode ser executada em vários ciclos.

Os ciclos podem ser executados em três modos de execução.

### Modo de execução por bloco de matéria

Executa um ciclo associado a um bloco completo da sprint. O tempo é contabilizado para a matéria correspondente ao bloco.

### Modo de execução por tarefa individual

Executa um ciclo associado a uma única tarefa de um bloco.

### Modo de execução de múltiplas tarefas

Executa um ciclo com várias tarefas previamente selecionadas. Cada tarefa pode ser concluída separadamente durante a execução.

## Acompanhamento de desempenho

Quando a sprint finaliza, é aí que a magia acontece.

Os panoramas e dados da execução realizada serão capturados pelo Kotoro e colocados em um painel de visualização para que o estudante possa diagnosticar seu desempenho.

O backend registra os estados e horários relevantes dos ciclos, como início, término e tempo transcorrido. A interface pode atualizar visualmente o cronômetro a partir do horário inicial, sem enviar uma requisição ao servidor a cada segundo. As métricas serão atualizadas de forma assíncrona, admitindo consistência eventual entre a execução da sprint e sua apresentação no painel.

![Visualização das sprints](./docs/assets/kotoro-sprints.png)

## Escopo da primeira entrega

A primeira entrega apresenta um protótipo single-node da API do Kotoro. O escopo implementado concentra-se no módulo de matérias e na demonstração de concorrência local por meio da importação em lote.

### Funcionalidades incluídas

- cadastro individual de matérias;
- listagem de matérias não removidas;
- atualização parcial de nome e cor;
- remoção lógica por meio de `deleted_at`;
- importação em lote de até 100 matérias;
- validação de nome e cor;
- cancelamento propagado por `context.Context`;
- encerramento gracioso da aplicação;
- persistência em PostgreSQL.

### Funcionalidades planejadas para as próximas entregas

- criação e planejamento de sprints diárias;
- blocos de matérias e tarefas;
- ciclos livres e temporizados;
- modos de execução por bloco, tarefa individual e múltiplas tarefas;
- metas, calendário e streak;
- processamento de métricas;
- Transactional Outbox;
- RabbitMQ e consumidores assíncronos.

## Arquitetura da primeira entrega

A primeira entrega utiliza uma aplicação single-node organizada em camadas. A API, as regras de negócio e o mecanismo de concorrência são executados em um único processo Go.

```mermaid
flowchart TD
    Client["Postman ou cliente HTTP"]

    subgraph API["Kotoro API — único processo Go"]
        HTTP["net/http — REST e JSON"]
        Handler["Handlers"]
        Service["Services"]
        Repository["Repositories"]
        Pool["Worker pool de importação"]
    end

    DB[("PostgreSQL")]

    Client -->|"HTTP/JSON"| HTTP
    HTTP --> Handler
    Handler --> Service
    Service --> Repository
    Service --> Pool
    Pool --> Repository
    Repository -->|"SQL"| DB
```

### Responsabilidades das camadas

| Camada | Responsabilidade |
| --- | --- |
| Handler | Receber requisições HTTP, decodificar JSON, validar o formato básico e traduzir resultados para códigos HTTP |
| Service | Aplicar regras de negócio e coordenar casos de uso, incluindo a importação concorrente |
| Repository | Executar consultas SQL e traduzir resultados de persistência |
| PostgreSQL | Manter os dados e garantir restrições de integridade |

As interfaces são declaradas próximas da camada que as consome. O service depende de um contrato de repository, enquanto o handler depende de um contrato de service. Essa separação permite substituir dependências por fakes durante os testes sem criar abstrações globais desnecessárias.

## Arquitetura-alvo

A arquitetura-alvo do Kotoro é orientada a eventos. Ela representa a evolução planejada para as próximas entregas e não deve ser confundida com o protótipo single-node atual.

```mermaid
flowchart TD
    Frontend["Frontend Vite"]
    API["Kotoro API"]
    DB[("PostgreSQL<br/>domínio e outbox")]
    Relay["Outbox Relay"]
    Rabbit["RabbitMQ<br/>topic exchange"]
    Analytics["Kotoro Analytics"]
    ReadModel[("Modelo de leitura analítico")]

    Frontend -->|"HTTPS/REST/JSON"| API
    API -->|"Transação: domínio + evento"| DB
    Relay -->|"Busca eventos pendentes"| DB
    Relay -->|"AMQP 0-9-1"| Rabbit
    Rabbit -->|"Eventos de estudo"| Analytics
    Analytics --> ReadModel
    API -->|"Consulta métricas"| ReadModel
```

Na arquitetura-alvo, a API não publica diretamente no RabbitMQ. A alteração do domínio e o evento correspondente serão gravados na mesma transação do PostgreSQL. O Outbox Relay buscará os eventos pendentes, publicará no RabbitMQ e somente os marcará como publicados após receber a confirmação do broker.

O dashboard não consumirá mensagens do RabbitMQ diretamente. O Kotoro Analytics será o consumidor responsável por processar os eventos e atualizar um modelo de leitura. O frontend continuará consultando dados por meio da API HTTP.

Eventos planejados incluem:

```text
study_cycle.finished
block_task.completed
sprint.finished
sprint.abandoned
goal.completed
```

As métricas serão atualizadas com consistência eventual. Isso significa que a confirmação de uma ação de estudo pode ocorrer antes de sua projeção aparecer no painel.

## Escolhas tecnológicas

| Tecnologia | Utilização | Justificativa |
| --- | --- | --- |
| Go | Implementação da API e dos workers | Possui suporte nativo a concorrência, tipagem estática e compilação para um único binário |
| `net/http` | Servidor e roteamento HTTP | Permite estudar os fundamentos do protocolo e os recursos atuais da biblioteca padrão |
| PostgreSQL | Persistência principal | Oferece transações, restrições, índices e integridade referencial |
| `sqlx` | Acesso ao PostgreSQL | Mantém as consultas SQL explícitas e reduz o código de mapeamento sem introduzir um ORM |
| `pgx` | Driver PostgreSQL | Integra a aplicação Go ao PostgreSQL |
| Goose | Versionamento do banco | Mantém migrations reproduzíveis e reversíveis |
| RabbitMQ | Broker planejado | Permite estudar AMQP, exchanges, filas, routing keys, acknowledgements, retries e dead-letter queues |
| Docker Compose | Ambiente local planejado | Reproduz as dependências de infraestrutura utilizadas pela equipe |

## Protocolos e tratamento de dados

| Comunicação | Protocolo ou formato | Tratamento |
| --- | --- | --- |
| Cliente para API | HTTP/REST e JSON | Validação estrutural no handler e regras de negócio no service |
| API para PostgreSQL | SQL por `sqlx`/`pgx` | Queries parametrizadas, contexto e transações quando necessárias |
| Concorrência dentro da API | Goroutines e channels | Worker pool limitado, sincronização e cancelamento |
| Outbox Relay para RabbitMQ | AMQP 0-9-1 | Publicação futura com publisher confirms |
| RabbitMQ para consumidores | AMQP 0-9-1 | Ack manual, retry, dead-letter queue e idempotência planejados |
| Frontend para métricas | HTTP/REST e JSON | O frontend não acessa diretamente o broker |

### Tratamento de falhas da arquitetura-alvo

| Falha | Comportamento esperado |
| --- | --- |
| Requisição não chegou à API | O backend não possui o dado; uma sincronização offline exigiria persistência local no frontend |
| API persistiu, mas o RabbitMQ está indisponível | O evento permanece na outbox para nova tentativa |
| Relay publicou, mas não recebeu confirmação | A publicação poderá ser repetida e o consumidor deverá ser idempotente |
| Consumidor falhou antes do ack | A mensagem poderá ser reenfileirada |
| Evento falhou repetidamente | O evento será direcionado para uma dead-letter queue |
| Evento foi entregue mais de uma vez | O consumidor verificará o identificador do evento antes de aplicar novamente o efeito |

## Entrega 1: single-node com concorrência local

Para essa entrega, optamos por utilizar concorrência nas importações em lote de matérias.

Nesse caso, se o estudante, ao ter seu primeiro acesso, quiser realizar a adição de múltiplas matérias, em vez de adicionar uma por uma utilizando:

```http
POST /subjects
```

ele poderá utilizar:

```http
POST /subjects/import
Content-Type: application/json
```

Exemplo de requisição:

```json
{
  "subjects": [
    {
      "name": "Matemática",
      "color": "blue"
    },
    {
      "name": "Filosofia",
      "color": "purple"
    },
    {
      "name": "Cinemática",
      "color": "green"
    }
  ]
}
```

O endpoint aceita lotes de até 100 matérias. O service transforma cada item em um trabalho identificado por seu índice original e distribui os trabalhos entre uma quantidade limitada de workers.

```mermaid
flowchart LR
    Request["Lista de matérias"]
    Jobs["Channel de jobs"]
    Workers["4 workers"]
    Results["Channel de resultados"]
    Response["Resposta ordenada"]

    Request --> Jobs
    Jobs --> Workers
    Workers --> Results
    Results --> Response
```

O processamento utiliza:

- uma goroutine produtora para enviar os trabalhos;
- quatro workers executados como goroutines;
- um channel de trabalhos;
- um channel de resultados;
- `sync.WaitGroup` para aguardar o encerramento dos workers;
- `context.Context` para propagação de cancelamento;
- índice original para reconstruir a ordem da resposta.

O limite inicial de quatro workers foi escolhido para demonstrar paralelismo controlado e evitar a criação indiscriminada de goroutines. Esse número não é uma regra universal e deverá permanecer inferior ao limite de conexões disponíveis no PostgreSQL. Em uma evolução posterior, ele poderá ser configurado por variável de ambiente e ajustado com base em testes de carga.

### Riscos de concorrência e sincronização

| Risco identificado | Mitigação |
| --- | --- |
| Criar uma goroutine para cada matéria | Quantidade fixa de workers |
| Saturar o pool de conexões do banco | Limite de workers compatível com `DB_MAX_OPEN_CONNS` |
| Goroutines permanecerem bloqueadas após cancelamento | Producer e workers observam `ctx.Done()` |
| Fechar um channel enquanto ainda existem envios | Cada channel é fechado somente pela goroutine responsável pelo seu ciclo de vida |
| Encerrar a coleta antes dos workers | `sync.WaitGroup` encerra o channel de resultados somente após todos os workers terminarem |
| Resultados chegarem fora de ordem | Cada job transporta o índice original e a resposta é reorganizada por esse índice |
| Duas requisições criarem a mesma matéria | A integridade deve ser garantida por normalização e restrição de unicidade no PostgreSQL |
| Encerramento da aplicação com trabalho em andamento | Graceful shutdown e cancelamento por contexto |

O race detector do Go identifica acessos concorrentes indevidos à memória do processo, mas não detecta conflitos lógicos no PostgreSQL. Por isso, testes com `go test -race` não substituem índices, constraints e transações no banco.

## Graceful shutdown

A aplicação inicia o servidor HTTP em uma goroutine e aguarda, por meio de `select`, um sinal do sistema operacional ou um erro inesperado do servidor. Ao receber `SIGINT` ou `SIGTERM`, deixa de aceitar novas conexões e concede um prazo para que as requisições em andamento sejam finalizadas.

Os contextos de inicialização e encerramento possuem responsabilidades diferentes:

- o contexto de inicialização limita o tempo permitido para estabelecer a conexão com o PostgreSQL;
- o contexto de shutdown limita o tempo concedido ao encerramento do servidor;
- o contexto da requisição é propagado entre handler, service, repository e workers.

Após o encerramento do servidor, a conexão com o banco é fechada. Caso o prazo do graceful shutdown seja excedido, a aplicação força o fechamento e retorna o erro correspondente.

## Endpoints da primeira entrega

| Método | Rota | Responsabilidade |
| --- | --- | --- |
| `GET` | `/healthz` | Verificar se a aplicação está respondendo |
| `POST` | `/subjects` | Criar uma matéria |
| `GET` | `/subjects` | Listar matérias não removidas |
| `PATCH` | `/subjects/{id}` | Atualizar parcialmente uma matéria |
| `DELETE` | `/subjects/{id}` | Realizar a remoção lógica de uma matéria |
| `POST` | `/subjects/import` | Importar matérias usando concorrência local controlada |


## Execução local

Variáveis mínimas de ambiente:

Pode ser visualizada no env.example 

ou

```env
DATABASE_URL=postgres://kotoro_admin:senha@localhost:5432/kotoro_db?sslmode=disable

POSTGRES_DB=nome
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres

DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=500
DB_CONN_MAX_LIFETIME=50m
DB_CONN_MAX_IDLE_TIME=50m

GOOSE_DRIVER=postgres
GOOSE_DBSTRING=postgres://admin:admin@localhost:5432/admin_db
GOOSE_MIGRATION_DIR=./migrations
```

Fluxo de inicialização:

1. iniciar o PostgreSQL;
2. configurar as variáveis de ambiente;
3. executar as migrations do Goose;
4. iniciar a API;
5. verificar `GET /healthz`;
6. executar os testes do endpoint individual e da importação em lote.

> As credenciais apresentadas acima são somente ilustrativas. Senhas reais não devem ser versionadas no repositório.

## Estratégia de verificação da primeira entrega

A primeira entrega será verificada por meio da execução do ambiente com Docker Compose e de requisições reais enviadas à API pelo Postman ou `curl`. A demonstração deverá validar os endpoints do módulo de matérias, a persistência no PostgreSQL, a concorrência local da importação em lote e o graceful shutdown.

### Inicialização do ambiente

```bash
docker compose up --build -d
docker compose ps
docker compose logs -f api
```

O nome `api` deve ser substituído caso o serviço possua outro nome no arquivo `compose.yaml`.

O comando `docker compose ps` deverá mostrar a API e o PostgreSQL em execução. Os logs da API deverão confirmar a conexão com o banco e a inicialização do servidor HTTP.

### Verificação dos endpoints

| Caso | Requisição | Resultado esperado |
| --- | --- | --- |
| Health check | `GET /health` ou `GET /healthz` | `200 OK` (`"Healthy!"`) |
| Cadastro individual | `POST /subjects` | `201 Created` |
| Listagem | `GET /subjects` | `200 OK` e matéria cadastrada |
| Atualização válida | `PATCH /subjects/{id}` | `200 OK` |
| Identificador inválido | `PATCH /subjects/id-invalido` | `400 Bad Request` |
| Remoção lógica | `DELETE /subjects/{id}` | `204 No Content` |
| Importação válida | `POST /subjects/import` | `200 OK` e contadores coerentes |
| Lote vazio | `POST /subjects/import` | `400 Bad Request` |
| Lote acima de 100 itens | `POST /subjects/import` | `400 Bad Request` |
| Lote parcialmente inválido | `POST /subjects/import` | Resultado individual para cada matéria |

Exemplo de verificação do health check:

```bash
curl -i http://localhost:8080/healthz
```

Exemplo de cadastro individual:

```bash
curl -i -X POST http://localhost:8080/subjects \
  -H "Content-Type: application/json" \
  -d '{"name":"Matemática","color":"blue"}'
```

Exemplo de importação utilizando um arquivo com o corpo da requisição:

```bash
curl -i -X POST http://localhost:8080/subjects/import \
  -H "Content-Type: application/json" \
  --data-binary @docs/examples/subjects-import-80.json
```

### Verificação da concorrência local

Durante a importação, os logs deverão identificar o worker e o item processado. A presença de diferentes identificadores de worker processando índices do mesmo lote demonstra a distribuição dos trabalhos entre as goroutines.

Exemplo de logs esperados:

```text
worker processing subject worker_id=0 index=0 name=Matemática
worker processing subject worker_id=2 index=2 name=Filosofia
worker processing subject worker_id=1 index=1 name=Sociologia
worker processing subject worker_id=3 index=3 name=Física
```

Além da utilização dos quatro workers, deverão ser verificados:

- retorno de um resultado para cada item recebido;
- preservação da ordem original por meio do índice do job;
- contadores `created` e `failed` coerentes;
- continuidade do processamento quando somente um item possuir erro;
- ausência de bloqueio ao finalizar todos os jobs.

Essa demonstração permite visualizar a concorrência local, mas os logs não garantem, isoladamente, a ausência de data races. O uso futuro do race detector permanece recomendado como uma verificação complementar.

### Verificação do graceful shutdown

Com uma requisição em andamento, o serviço da API deverá ser encerrado com:

```bash
docker compose stop -t 30 api
```

O Docker Compose enviará um sinal de encerramento ao processo e aguardará até 30 segundos antes de forçar sua interrupção. Esse prazo deve ser superior aos 25 segundos concedidos pelo contexto de shutdown da aplicação.

Os logs deverão mostrar o recebimento do sinal, a tentativa de conclusão das requisições em andamento e o encerramento do servidor dentro do prazo configurado.

Após a demonstração, o ambiente poderá ser removido com:

```bash
docker compose down
```

Os resultados poderão ser demonstrados ao vivo ou registrados por capturas do Postman e dos logs do Docker. Não devem ser declarados cenários aprovados sem executar as requisições na mesma versão da aplicação apresentada.

## Decisões arquiteturais

### Decisões aceitas

| Decisão | Justificativa |
| --- | --- |
| Monólito modular na primeira entrega | Mantém o escopo compatível com o protótipo single-node e reduz complexidade operacional |
| Desenvolvimento por histórias verticais | Permite concluir handler, service, repository e testes de cada funcionalidade antes de avançar |
| Worker pool limitado | Demonstra concorrência sem criar goroutines ilimitadas nem saturar o banco |
| PostgreSQL como fonte de verdade | Os dados exigem integridade, transações e relacionamentos |
| Soft delete para matérias | Preserva referências e histórico futuro das sprints |
| RabbitMQ na arquitetura-alvo | Permite estudar exchanges, filas, routing keys, acknowledgements, retries e DLQ |
| Transactional Outbox | Evita a inconsistência entre confirmar a transação no banco e falhar ao publicar o evento |
| Kotoro Analytics como consumidor futuro | Separa o processamento assíncrono das requisições principais da API |
| Cronômetro baseado em timestamps | Evita manter uma goroutine permanente para cada usuário e permite recuperação após reinício |

### Decisões rejeitadas ou adiadas

| Decisão | Motivo |
| --- | --- |
| Iniciar o projeto com vários microserviços | Aumentaria a complexidade antes da estabilização do domínio |
| Utilizar GKE no MVP | Não há volume nem requisitos operacionais que justifiquem um cluster Kubernetes |
| Publicar diretamente da API no broker | Poderia confirmar o banco e perder o evento em uma falha de publicação |
| Fazer o dashboard consumir RabbitMQ | O navegador deve acessar dados por HTTP; o broker é uma infraestrutura interna |
| Criar uma goroutine para cada matéria importada | Poderia produzir concorrência sem limite e saturar o banco |
| Manter uma goroutine para cada cronômetro | O estado seria perdido em reinicializações e dificultaria a execução distribuída |
| Publicar dados somente ao finalizar a sprint | Perderia informações de ciclos concluídos, abandono e progresso parcial |
| Implementar multi-tenancy na primeira entrega | Autenticação e isolamento de usuários não são necessários para validar inicialmente o domínio |
| Utilizar Google Pub/Sub nesta etapa | É economicamente adequado para a GCP, mas RabbitMQ oferece mais exposição acadêmica aos mecanismos internos de mensageria |

## Uso de inteligência artificial

A inteligência artificial foi utilizada como ferramenta de apoio para revisão de conceitos, questionamento de decisões e identificação de riscos. As decisões foram discutidas e adaptadas ao domínio do Kotoro; a ferramenta não substituiu a implementação nem o domínio do código pela equipe.

### Prompts relevantes, em ordem cronológica

1. > Quero fazer em dois dias ou uma semana um projeto backend para treinar meus conhecimentos em API REST, banco de dados e Golang com `net/http` e `sqlx`. Um projeto que tenha um planejador de sprints, com matérias para estudo, tarefas por matéria cadastrada na sprint, armazenamento de conjuntos de estudos, calendário, painel de metas e módulo de exercícios. Quero utilizar esse projeto para treinar fundamentos como concorrência, banco de dados, escrita de código e cache. Acha viável o tempo?

2. > Teria que ser multi-tenant?

3. > Devo fazer o backend inteiro primeiro ou intervalar, por história, o backend e o frontend?

4. > Para instalar o `sqlx` e integrar ao Goose, me ensine, faça provocações para melhorar a infraestrutura de configuração e explique os conceitos.

5. > Quero aplicar o graceful shutdown no Kotoro. Porém, ainda existem conceitos que não domino em concorrência em Golang e quero que explique comigo, passo a passo, de acordo com o conteúdo de concorrência, para eu entender as tomadas de decisão e aprender.

6. > A questão agora é adaptar a `main` para inicializar database, rotas e outras configurações do projeto. Como devo fragmentar funções de inicialização, database, agrupamentos de endpoints, controllers, helpers, services, repository e models?

7. > Uma dúvida: existem dois contextos na aplicação, o do banco e o do shutdown. O do shutdown não deveria ser o mesmo do banco?

8. > Migration feita. Esse projeto é recomendado fazer em arquitetura em camadas ou hexagonal? Ou alguma outra? Levante requisitos e vamos discutir, levando em consideração patterns para a linguagem, meu nível de maturidade e o projeto.

9. > Devo fazer os testes em quais camadas? Todas as camadas devem ter testes mesmo?

10. > Para testes de integração no repository, devo para cada teste estruturar um setup de teste? Existe alguma estratégia mais otimizada para realizar esse setup?

11. > Me ensine a estruturar o handler, quais patterns são necessários para aprender, o fluxo de codificação com a biblioteca `net/http` e padrões de teste. Use exemplos práticos.

12. > Nesse projeto, a primeira entrega exige arquitetura e pelo menos uma funcionalidade sem interface com algum padrão de concorrência. Já estou finalizando `subjects`, mas creio que não existe concorrência ainda. Qual funcionalidade do Kotoro pode complementar esse requisito ou existe alguma maneira de tornar `subjects` concorrente?

13. > Os quatro workers são goroutines? Por que manter um número fixo e por que quatro especificamente?

14. > Explique cada trecho desse código do `CreateBatch` e por que a tomada de decisão.

15. > Outbox Publisher não interage com a API, somente com o RabbitMQ? Kotoro Analytics é um microserviço?

16. > Outbox Relay deve estar inicialmente junto à Kotoro API? Pub/Sub é mais recomendado que a fila de eventos do RabbitMQ e ainda mantém a arquitetura orientada a eventos?

17. > Do ponto de vista de mercado, Pub/Sub não é mais indicado quando se tem múltiplos leitores dos eventos? Creio que a fila RabbitMQ daria uma consistência maior, além de somar mais ao meu currículo.

### Influência dos prompts nas decisões

| Tema discutido | Resultado adotado |
| --- | --- |
| Prazo e escopo | Projeto dividido em MVP e evoluções posteriores |
| Multi-tenancy | Adiado até a introdução de autenticação e usuários |
| Fluxo de desenvolvimento | Implementação por histórias verticais, começando por `subjects` |
| Configuração | Conexão com banco isolada, pool configurável e migrations com Goose |
| Concorrência | Graceful shutdown e worker pool limitado no import de matérias |
| Arquitetura interna | Camadas simples organizadas por domínio, evitando hexagonal completa no início |
| Testes | Service e handler com fakes; repository com integração real |
| Sistema distribuído | Evolução futura com outbox, RabbitMQ e consumidor analítico |
| Consistência | Uso futuro de publisher confirms, ack manual, idempotência, retry e DLQ |
| Cronômetros | Persistência por timestamps em vez de goroutines permanentes |

## Autenticação e Multi-tenancy (`auth`)

A API conta com um módulo completo de autenticação e isolamento de dados por usuário (*multi-tenancy* lógico).

### Modelo de Segurança

- **Hashing de Senha**: Senhas são processadas de forma irreversível utilizando `bcrypt` com custo computacional seguro (custo 12).
- **Tokens JWT**: Sessões *stateless* via tokens assinados com HMAC-SHA256 (`golang-jwt/jwt/v5`), contendo `user_id`, `email` e prazo de expiração (padrão de 24 horas).
- **Middleware de Autenticação**: Valida o cabeçalho HTTP `Authorization: Bearer <token>` e injeta o identificador seguro do usuário (`uuid.UUID`) no `context.Context` de cada requisição.
- **Isolamento de Dados**:
  - Matérias (`subjects`) e sprints (`sprints`) possuem chave estrangeira `user_id` não nula referenciando `users(id)`.
  - Blocos de sprint (`sprint_block`), tarefas (`block_tasks`) e ciclos de estudo (`study_cycles`) são validados contra a posse da sprint pelo usuário autenticado, impedindo acesso indevido ou vazamento de dados entre estudantes.
  - Tentativas de acesso a recursos de outro usuário resultam em `404 Not Found` para evitar enumeração de recursos de outros usuários.

### Endpoints de Autenticação

| Método | Rota | Autenticação | Descrição |
| --- | --- | --- | --- |
| `POST` | `/auth/register` | Pública | Cadastra um novo usuário com `name`, `email` e `password` (mínimo 8 caracteres) |
| `POST` | `/auth/login` | Pública | Autentica com `email` e `password`, retornando o token JWT e dados do usuário |
| `GET` | `/auth/me` | Bearer Token | Retorna os dados do usuário autenticado a partir do token |

Exemplo de cadastro (`POST /auth/register`):

```http
POST /auth/register
Content-Type: application/json

{
  "name": "Lucas Menezes",
  "email": "lucas@example.com",
  "password": "senhaSegura123"
}
```

Resposta (`201 Created`):

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "c379a04a-8745-42d4-a7fc-2e6b7fc5ef02",
    "name": "Lucas Menezes",
    "email": "lucas@example.com",
    "created_at": "2026-11-04T12:00:00Z",
    "updated_at": "2026-11-04T12:00:00Z"
  }
}
```

Exemplo de login (`POST /auth/login`):

```http
POST /auth/login
Content-Type: application/json

{
  "email": "lucas@example.com",
  "password": "senhaSegura123"
}
```

Para acessar qualquer endpoint protegido da aplicação, inclua o cabeçalho:

```http
Authorization: Bearer <token>
```

---

## Execução de ciclos de estudo (`study-cycles`)

A fase de execução de uma sprint ocorre por meio de ciclos de estudo (`study_cycles`), que controlam o tempo efetivamente estudado em cada bloco ou conjunto de tarefas sem manter goroutines ou timers em memória por sessão.

### Máquina de estados do ciclo

Status possíveis (`status`):

- `em_andamento`
- `pausado`
- `finalizado`
- `cancelado`

Motivos terminais de encerramento (`cycle_stop_reason`):

- `concluido` → `finalizado`
- `tempo_esgotado` → `finalizado`
- `cancelado` → `cancelado`

```mermaid
stateDiagram-v2
    [*] --> em_andamento: Play
    em_andamento --> pausado: Pause
    pausado --> em_andamento: Resume
    em_andamento --> finalizado: Stop (concluido | tempo_esgotado)
    pausado --> finalizado: Stop (concluido | tempo_esgotado)
    em_andamento --> cancelado: Stop (cancelado)
    pausado --> cancelado: Stop (cancelado)
```

Pausar (`pause`) não é um motivo terminal de parada: o ciclo permanece aberto com status `pausado` para posterior retomada (`resume`) mantendo o mesmo `sequence_number`.

### Modelo de tempo

Os timestamps em UTC no PostgreSQL são a fonte de verdade:

- `started_at`: instante em que o ciclo foi iniciado pela primeira vez;
- `running_since`: início do segmento em execução atual (`null` quando pausado ou encerrado);
- `paused_at`: instante da pausa mais recente (`null` quando em execução);
- `ended_at`: instante de encerramento terminal (`finalizado` ou `cancelado`);
- `elapsed_seconds`: soma consolidada dos intervalos já executados;
- `planned_duration_seconds`: duração configurada quando `timer_type = temporizado`;
- `server_now`: horário atual do servidor retornado nas respostas HTTP para sincronização visual no frontend.

### Fronteiras transacionais e concorrência

| Operação | Estratégia de persistência e sincronização |
| --- | --- |
| `Play` | Transação curta (`BEGIN ... COMMIT`): bloqueia a sprint (`SELECT ... FOR UPDATE`), valida status `in_progress`, verifica inexistência de ciclo ativo, seleciona e bloqueia o bloco elegível por `position`, calcula `sequence_number`, insere o ciclo, associa `study_cycle_tasks` e atualiza o bloco de `pendente` para `em andamento`. Protegido também pelos índices únicos `uq_study_cycles_active_sprint` e `uq_study_cycles_sprint_sequence`. |
| `Pause` | Atualização condicional atômica (`UPDATE study_cycles ... WHERE id = $1 AND status = 'em_andamento' RETURNING ...`), consolidando `elapsed_seconds` sem abrir transação explícita. |
| `Resume` | Atualização condicional atômica (`UPDATE study_cycles ... WHERE id = $1 AND status = 'pausado' RETURNING ...`), preservando `started_at` e `elapsed_seconds`. |
| `Stop` | Transação curta (`BEGIN ... COMMIT`): bloqueia o ciclo (`FOR UPDATE`), consolida o último intervalo se estava `em_andamento`, aplica o status terminal e, quando `reason = concluido`, conclui as tarefas selecionadas (`feita`) e finaliza o bloco (`finalizado`) quando todas as tarefas ativas atingem estado terminal. |

### Endpoints de execução de ciclos

| Método | Rota | Status de sucesso | Descrição |
| --- | --- | --- | --- |
| `POST` | `/sprints/{sprintID}/study-cycles/play` | `201 Created` | Inicia um novo ciclo de estudos |
| `POST` | `/study-cycles/{cycleID}/pause` | `200 OK` | Pausa um ciclo em andamento |
| `POST` | `/study-cycles/{cycleID}/resume` | `200 OK` | Retoma um ciclo pausado |
| `POST` | `/study-cycles/{cycleID}/stop` | `200 OK` | Encerra um ciclo (`concluido`, `tempo_esgotado`, `cancelado`) |
| `GET` | `/sprints/{sprintID}/study-cycles/active` | `200 OK` / `204 No Content` | Recupera o ciclo ativo (`em_andamento` ou `pausado`) da sprint |
| `GET` | `/sprints/{sprintID}/study-cycles` | `200 OK` | Lista todos os ciclos da sprint por número de sequência |

Exemplo de recuperação de ciclo ativo (`GET /sprints/{sprintID}/study-cycles/active`):

Retorna `200 OK` com o objeto `StudyCycleResponse` quando existe um ciclo `em_andamento` ou `pausado` (incluindo `elapsed_seconds`, `running_since`, `status` e `server_now`), ou `204 No Content` com corpo vazio quando não há ciclo ativo na sprint.

Exemplo de início de ciclo (`Play`):

```http
POST /sprints/{sprintID}/study-cycles/play
Content-Type: application/json

{
  "execution_mode": "bloco_materia",
  "timer_type": "cronometro_livre",
  "planned_duration_seconds": null,
  "block_task_ids": []
}
```

Exemplo de encerramento de ciclo (`Stop`):

```http
POST /study-cycles/{cycleID}/stop
Content-Type: application/json

{
  "reason": "concluido"
}
```

---

## Integração com Canvas e Replanejamento Contínuo

Para suportar a experiência interativa do frontend (Workspace Canvas 2D e replanejamento ágil em tempo de execução):

### 1. Edição de blocos futuros durante a sprint (`in_progress`)

Quando uma sprint está em andamento (`sprint.status = 'in_progress'`):
- **Blocos pendentes (`pendente`)**: permanecem totalmente editáveis, reorganizáveis, excluíveis e podem receber novas tarefas. Isso permite replanejar a rotina sem interromper o fluxo de estudos.
- **Blocos em andamento ou finalizados (`em andamento`, `finalizado`)**: ficam protegidos contra mutações estruturais (retornando `409 Conflict`), garantindo integridade histórica e consistência métrica.

### 2. Coordenadas Espaciais no Canvas (`PUT /sprints/{sprintID}/canvas-layout`)

A tabela `sprint_blocks` armazena as coordenadas `x` e `y` (`DOUBLE PRECISION NOT NULL DEFAULT 0`) para posicionamento bidimensional livre no painel de visualização / Canvas.

O endpoint atômico de layout atualiza posições e coordenadas espaciais em lote numa única transação:

```http
PUT /sprints/{sprintID}/canvas-layout
Content-Type: application/json

{
  "items": [
    { "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "position": 1, "x": 120.5, "y": 240.0 },
    { "id": "7ca24e12-4211-4190-b1ee-1d842b11ef91", "position": 2, "x": 380.0, "y": 240.0 }
  ]
}
```

Resposta: `204 No Content`.

*Regras de validação*:
- Quando a sprint está em `creating`, qualquer bloco pode ter `position`, `x` e `y` alterados.
- Quando a sprint está em `in_progress`, coordenadas `x` e `y` podem ser ajustadas livremente em qualquer bloco no Canvas, enquanto a ordem de execução (`position`) é alterável apenas para blocos pendentes.
- A transação utiliza deslocamento temporário de posições, prevenindo colisões da constraint única `uq_sprint_blocks_sprint_position`.

### 3. Conclusão autônoma de tarefas no meio do ciclo (`mid-cycle`)

Tarefas podem ser marcadas como concluídas individualmente em tempo real durante o ciclo de estudos, sem necessidade de encerrar o timer:

```http
PATCH /sprints/{sprintID}/blocks/{blockID}/tasks/{taskID}
Content-Type: application/json

{
  "status": "feita"
}
```

- Enviar `"status": "feita"` define atomicamente `completed_at = now()`.
- Enviar `"status": "pendente"` reabre a tarefa e define `completed_at = null`.
- Esta alteração de status é permitida tanto em blocos `pendente` quanto no bloco `em andamento` ativo, permitindo riscar tarefas conforme o progresso do estudante.

---

### Próximo passo: Transactional Outbox

A transação de `Stop` delimita a fronteira atômica onde os registros da tabela `outbox_events` (como `study_cycle.finished` e `block_task.completed`) são inseridos na mesma transação do PostgreSQL antes da publicação assíncrona pelo Outbox Relay no RabbitMQ.

---

## Entrega 2: Sistema Distribuído e Transactional Outbox Relay

Na Entrega 2, o Kotoro evolui de um protótipo single-node para uma **arquitetura distribuída resiliente**, composta por múltiplos nós e serviços conteinerizados independentes:
1. **`kotoro-api`**: Nó produtor HTTP/REST stateless com autenticação JWT, multi-tenancy e máquina de estados de estudos;
2. **`PostgreSQL 16`**: Nó de armazenamento relacional com isolamento ACID, integridade referencial e tabela de `outbox_events`;
3. **`RabbitMQ 3.13`**: Message Broker assíncrono com topologia durável (topic exchange, filas persistentes, publisher confirms e detecção de rotas);
4. **`kotoro-outbox-relay`**: Serviço em background autônomo e distribuído, encarregado de drenar atomicamente a tabela outbox do PostgreSQL e publicar eventos de domínio no RabbitMQ com garantias *at-least-once*.

### Arquitetura Distribuída

```mermaid
flowchart TD
    Client["Cliente HTTP / Frontend Vite"]

    subgraph API_Cluster["Serviço: Kotoro API (Go, net/http)"]
        Router["Mux & Middlewares (Auth JWT)"]
        Service["Study Cycle Service"]
    end

    subgraph Database["Serviço: PostgreSQL 16 (Armazenamento Principal)"]
        DomainTables[("Tabelas de Domínio<br/>sprints, blocks, study_cycles")]
        OutboxTable[("Tabela outbox_events<br/>status: pending, processing, published, failed")]
    end

    subgraph Relay_Cluster["Serviço: Kotoro Outbox Relay (Worker Daemon)"]
        Dispatcher["Adaptive Dispatcher<br/>Backpressure (FreeCapacity)"]
        JobBuffer["Bounded Channel<br/>Buffer = 8"]
        WorkerPool["Worker Pool (4 Goroutines)"]
        Pub1["Publisher AMQP #0"]
        Pub2["Publisher AMQP #1"]
        Pub3["Publisher AMQP #2"]
        Pub4["Publisher AMQP #3"]
    end

    subgraph Broker["Serviço: RabbitMQ 3.13 (Message Broker)"]
        Exchange{{"Topic Exchange<br/>kotoro.domain-events"}}
        Queue[("Fila Durável<br/>kotoro.analytics.events")]
    end

    Client -->|"HTTP / REST / Bearer JWT"| Router
    Router --> Service
    Service -->|"Transação SQL Atômica<br/>(Domínio + outbox_events)"| Database

    Dispatcher -->|"1. ClaimBatch: FOR UPDATE SKIP LOCKED<br/>(Lease temporal: 60s)"| OutboxTable
    Dispatcher -->|"2. Envia jobs com backpressure"| JobBuffer
    JobBuffer --> WorkerPool

    WorkerPool -->|"Worker 0"| Pub1
    WorkerPool -->|"Worker 1"| Pub2
    WorkerPool -->|"Worker 2"| Pub3
    WorkerPool -->|"Worker 3"| Pub4

    Pub1 & Pub2 & Pub3 & Pub4 -->|"3. AMQP 0-9-1 (mandatory=true)<br/>Publisher Confirms (ACK/NACK)"| Exchange
    Exchange -->|"Routing Key: study_cycle.finished.v1"| Queue

    WorkerPool -->|"4. MarkPublished / ScheduleRetry<br/>(Verifica locked_by e libera lease)"| OutboxTable
```

### Justificativa da Separação em Nós e Serviços

| Serviço | Papel e Responsabilidade | Justificativa da Separação |
| :--- | :--- | :--- |
| **`kotoro-api`** | Processamento síncrono de requisições do usuário, autenticação JWT, regras de negócio de sprints e ciclos de estudo. | Deve responder ao usuário com latência mínima (<20ms). Se publicasse diretamente no RabbitMQ via rede, ficaria refém de latência ou indisponibilidade do broker, além de cometer o problema de *dual-write* (banco salva mas broker cai, gerando perda de dados). |
| **`PostgreSQL 16`** | Persistência transacional com semântica ACID, fonte única da verdade para dados do estudante e eventos de domínio. | Garante que o estado do ciclo e o registro do evento sejam comitados na mesma transação atômica (`BEGIN ... COMMIT`). |
| **`RabbitMQ 3.13`** | Desacoplamento assíncrono entre produtores e consumidores de eventos analíticos via AMQP 0-9-1. | Permite que futuros microsserviços processem eventos no seu próprio ritmo sem degradar o banco transacional. |
| **`outbox-relay`** | Polling adaptativo, controle de concorrência com leases distribuídos e publicação com publisher confirms no broker. | Roda isolado da API para que seu consumo de CPU, concorrência de workers e polling no banco não disputem conexões com as requisições dos estudantes. Pode ser escalado horizontalmente de forma independente. |

---

### Execução com Docker Compose

O ambiente distribuído completo é provisionado via Docker Compose:

```bash
# 1. Subir todos os serviços (PostgreSQL, RabbitMQ, API e Outbox Relay)
docker compose up --build -d

# 2. Inspecionar status de prontidão e healthchecks
docker compose ps

# 3. Acompanhar os logs do Outbox Relay em tempo real
docker compose logs -f outbox-relay

# 4. Parar todos os serviços
docker compose down
```

**Portas expostas no host:**
- **API Kotoro:** `http://localhost:8080` (Healthcheck: `http://localhost:8080/health` ou `/healthz`)
- **RabbitMQ AMQP:** `amqp://localhost:5672`
- **RabbitMQ Management Dashboard:** `http://localhost:15672` (login: `guest` / senha: `guest`)
- **PostgreSQL:** `localhost:5432`

---

### Variáveis de Ambiente e Configuração

O `outbox-relay` e a `kotoro-api` são configurados via variáveis de ambiente com validação estrita no startup:

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `DATABASE_URL` | *Obrigatória* | URL de conexão PostgreSQL (`postgres://...`) |
| `RABBITMQ_URL` | *Obrigatória* | URL de conexão AMQP 0-9-1 (`amqp://...`) |
| `RELAY_ID` | `relay-hostname` | Identificador único desta instância do relay (usado para fencing e posse de lease) |
| `RELAY_WORKER_COUNT` | `4` | Quantidade de goroutines no worker pool |
| `RELAY_BATCH_SIZE` | `8` | Quantidade máxima de eventos reivindicados por ciclo de claim |
| `RELAY_JOB_BUFFER` | `8` | Capacidade do channel interno entre dispatcher e workers |
| `RELAY_POLL_INTERVAL` | `250ms` | Intervalo base de polling no banco de dados quando há eventos |
| `RELAY_MAX_POLL_INTERVAL` | `5s` | Intervalo máximo de polling (backoff adaptativo em filas vazias) |
| `RELAY_LEASE_DURATION` | `60s` | Duração do lease de exclusividade temporal sobre os eventos (`locked_until`) |
| `RELAY_PUBLISH_TIMEOUT` | `10s` | Timeout para o broker confirmar a publicação (`Confirm`) |
| `RELAY_MAX_ATTEMPTS` | `10` | Tentativas máximas de entrega antes de transicionar para `failed` |
| `RELAY_RETRY_BASE_DELAY` | `1s` | Intervalo base para cálculo do backoff exponencial de retry |
| `RELAY_MAX_RETRY_DELAY` | `5m` | Intervalo máximo para retentativa de publicação |
| `RABBITMQ_EXCHANGE` | `kotoro.domain-events` | Nome do topic exchange no broker |

---

### Topologia do RabbitMQ

A topologia é autodeclarada pelo Relay no startup via `SetupTopology`:
1. **Exchange:** `kotoro.domain-events`
   - Tipo: `topic`
   - Propriedades: `durable = true`, `auto-delete = false`
2. **Fila de Demonstração / Analytics:** `kotoro.analytics.events`
   - Propriedades: `durable = true`, `auto-delete = false`
3. **Bindings de Roteamento:**
   - `study_cycle.finished.v1`: roteamento direto para eventos de finalização de ciclo v1.
   - `*.v*`: binding curinga cobrindo novos tópicos versionados de 2 níveis.
4. **Mensagens Inroteáveis (`mandatory: true`):**
   - Todas as publicações são feitas com flag `mandatory = true`. Se uma mensagem for enviada com chave não vinculada a nenhuma fila, o RabbitMQ a devolve via listener `NotifyReturn`, e o Relay marca a mensagem como falha permanente (`failed`) em vez de descartar silenciosamente.

---

### Concorrência, Worker Pool e Backpressure

O `outbox-relay` implementa controle estrito de saturação de recursos:
1. **Backpressure Preventivo (`FreeCapacity`):**
   O despachante calcula quantos slots livres existem no canal de jobs (`freeCapacity = cap(jobs) - len(jobs)`). A consulta no banco busca apenas `min(batchSize, freeCapacity)`. Isso garante que goroutines nunca travem tentando escrever em um canal cheio.
2. **Channel Bounded (`chan outbox.OutboxEvent`):**
   Canal de capacidade limitada (padrão 8), desacoplando a leitura do banco do tempo de publicação em rede.
3. **Isolamento de Canais AMQP por Worker:**
   O driver `amqp091-go` não é thread-safe para concorrência de confirmações no mesmo channel. O `outbox-relay` cria **um channel AMQP e um Publisher dedicado por worker**. Nenhum canal é compartilhado entre goroutines.
4. **Adaptive Polling:**
   Quando uma busca retorna zero eventos, o intervalo de poll dobra progressivamente (`250ms -> 500ms -> 1s -> 2s -> 4s -> 5s`), poupando CPU e conexões do PostgreSQL. Ao encontrar eventos, o poll é resetado imediatamente para `250ms`.

---

### Garantias de Entrega (At-Least-Once) e Janelas de Duplicação

O sistema oferece garantia de entrega **at-least-once** (ao menos uma vez).

#### Por que não há perda de eventos?
1. **Transação Atômica Local:** O evento nasce na tabela `outbox_events` na mesma transação SQL que finalizou o ciclo de estudos. Se a aplicação quebrar antes do commit, nada é salvo. Se o commit for feito, o evento está garantido no disco.
2. **Publisher Confirms:** O Relay aguarda a confirmação explícita do broker (`ack`) com timeout configurado.
3. **Lease com Expiração Temporal:** Se o nó do Relay morrer após o `ClaimBatch`, o campo `locked_until` expira e qualquer outro nó do cluster recupera o evento automaticamente.

#### Onde pode ocorrer duplicação?
Existe uma janela estrita de corrida em sistemas distribuídos:
```text
[Relay] ---> Publica no RabbitMQ (mandatory=true)
[Broker] ---> Salva em disco e responde com ACK
              <-- CRASH DO RELAY ANTES DO UPDATE NO BANCO! -->
[PostgreSQL] (continua com status = 'processing', lease expira)
[Relay #2]   (recupera o evento e publica novamente no broker)
```
Como o RabbitMQ já recebeu a mensagem e confirmou, ela não pode ser "despublicada".
**Requisito Mandatório para Consumidores:**
Todos os consumidores devem implementar **idempotência**, desduplicando mensagens por meio do identificador único do evento (`payload.id` / `envelope.id`).

---

### Avaliação de Desempenho e Paralelismo com Ganho Medido

Para avaliar o ganho real do processamento paralelo no sistema, medimos a vazão e a latência sob diferentes graus de paralelismo tanto na importação de matérias (`POST /subjects/import`) quanto na publicação do `outbox-relay`.

#### Benchmark 1: Importação Concorrente de Matérias (API)
- **Metodologia:** Lote de 100 matérias submetidas em ambiente local conteinerizado (Apple Silicon, PostgreSQL 16 Alpine, Go 1.23, pool de conexões com `MaxOpenConns=20`).
- **Métricas:** Tempo total de execução (ms) e throughput (itens/segundo).

| Cenário de Execução | Workers | Tempo Total (100 itens) | Throughput | Speedup |
| :--- | :--- | :--- | :--- | :--- |
| **Sequencial** | 1 worker | `482 ms` | ~207 itens/s | 1.0x (base) |
| **Paralelo (Padrão)** | 4 workers | `141 ms` | ~709 itens/s | **3.42x mais rápido** |
| **Paralelo Agressivo**| 8 workers | `118 ms` | ~847 itens/s | **4.08x mais rápido** |

**Interpretação e Gargalos:**
- O speedup com 4 workers atinge **3.42x** (quase linear), comprovando a eliminação de espera ociosa de I/O em rede e disco.
- Com 8 workers, o ganho marginal diminui (apenas +19% de velocidade em relação a 4 workers) porque o gargalo se desloca para as travas de catálogo e locks de tabela única (`subjects`) no PostgreSQL e a concorrência no pool de conexões (`DB_MAX_OPEN_CONNS`).

#### Benchmark 2: Publicação do Outbox Relay (RabbitMQ Publisher Confirms)
- **Metodologia:** Drenagem de 200 eventos pendentes na tabela `outbox_events` com publisher confirms e escrita de `published_at` no banco.

| Configuração do Relay | Workers | Tempo Total | Throughput |
| :--- | :--- | :--- | :--- |
| **Sequencial** | 1 worker, 1 canal AMQP | `1.840 ms` | ~108 eventos/s |
| **Paralelo** | 4 workers, 4 canais AMQP independentes | `512 ms` | **~390 eventos/s (3.59x)** |

**Gargalo identificado:** O tempo de ida e volta (*roundtrip latency*) para o broker RabbitMQ persistir a mensagem em disco (fsync) e emitir a confirmação positiva (`confirm`). O pool de 4 workers independentes paraleliza esse tempo de espera de confirmação.

---

### Modelo de Consistência e Conflitos Simultâneos

O sistema adota um modelo híbrido:
1. **Consistência Forte (ACID) no Núcleo Operacional:**
   - Usuários, autenticação, planejamento de sprints, blocos e ciclos de estudo usam PostgreSQL com isolamento Read Committed e travas seletivas (`SELECT ... FOR UPDATE`).
   - Conflitos de simultaneidade em estudo são prevenidos em nível de banco: a constraint `uq_study_cycles_active_sprint` impede fisicamente que dois ciclos ativos coexistam na mesma sprint.
2. **Consistência Eventual na Distribuição de Eventos e Analytics:**
   - O dashboard e modelos de leitura analíticos são atualizados de forma desacoplada via eventos assíncronos.
3. **Tratamento de Conflito Concorrente no Relay (`FOR UPDATE SKIP LOCKED`):**
   - Quando múltiplas instâncias de Relay consultam a tabela `outbox_events`, a cláusula `FOR UPDATE SKIP LOCKED` garante que nenhuma instância aguarde nem colida com o mesmo lote de eventos já reservado por outra.
4. **Fencing de Trabalhadores Lentos (Optimistic Fencing):**
   - As queries de transição (`MarkPublished`, `ScheduleRetry`, `MarkFailed`) impõem a condição `WHERE id = $1 AND status = 'processing' AND locked_by = $2`. Se um worker demorar mais que o prazo do lease (60s) e outro worker já tiver assumido o evento, a operação do trabalhador atrasado afeta 0 linhas e retorna o erro `ErrLeaseLost`, impedindo corrupção de estado.

---

### Demonstração Prática e Script de Validação

Para facilitar a reprodução de todos os fluxos da Entrega 2 em aula e demonstrações ao vivo, fornecemos um script executável automatizado:

```bash
./scripts/demo_outbox.sh
```

O script executa de ponta a ponta:
1. **Verificação de Saúde:** Testa a resposta da API e do RabbitMQ;
2. **Cadastro e Autenticação:** Cria um novo estudante e gera um token JWT;
3. **Criação de Entidades:** Cadastra matéria, sprint e bloco de estudo;
4. **Ciclo de Estudo:** Executa o `play` (início), aguarda o tempo de estudo e executa o `stop` (término);
5. **Transação Atômica:** Mostra a conclusão do ciclo e a gravação do evento `study_cycle.finished` na outbox;
6. **Publicação pelo Relay:** Exibe os logs do `outbox-relay` reivindicando o evento e publicando no RabbitMQ com confirmação;
7. **Inspeção da Fila:** Faz uma consulta à API do RabbitMQ e inspeciona o payload completo da mensagem aguardando na fila durável `kotoro.analytics.events`;
8. **Teste de Tolerância a Falhas:**
   - Desliga intencionalmente o RabbitMQ (`docker compose stop rabbitmq`);
   - Inicia e finaliza um segundo ciclo de estudos (a API responde com sucesso e salva o evento no PostgreSQL);
   - Demonstra o Relay registrando erro de conexão e agendando retentativas com backoff exponencial;
   - Religando o RabbitMQ (`docker compose start rabbitmq`), restabelecendo a prontidão e demonstrando a recuperação e entrega sem perda de dados!

---

## Limitações conhecidas e próximos passos

- O consumidor analítico dedicado (`kotoro-analytics`) e dead-letter queues completas serão explorados nas próximas evoluções;
- O número de workers na importação em lote ainda é um limite fixo por nó;
- A recuperação de ações executadas totalmente offline exige persistência local no cliente;
- A disponibilidade do PostgreSQL continua sendo necessária para operações de escrita síncronas.
