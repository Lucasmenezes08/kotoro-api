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
| Health check | `GET /healthz` | `200 OK` |
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

## Limitações conhecidas da primeira entrega

- o sistema ainda não possui autenticação ou isolamento por usuário;
- as sprints e os ciclos ainda fazem parte do roadmap;
- RabbitMQ e Transactional Outbox ainda pertencem à arquitetura-alvo;
- o número de workers ainda é um limite fixo;
- a recuperação de ações executadas totalmente offline ainda não está implementada;
- a disponibilidade do PostgreSQL continua sendo necessária para operações de escrita;
- a restrição definitiva para nomes duplicados precisa estar coerente entre service e banco.
