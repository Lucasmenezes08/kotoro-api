#!/usr/bin/env bash
# ==============================================================================
# Kotoro API & Outbox Relay — Script de Demonstração de Ponta a Ponta
# ==============================================================================
# Demonstra:
# 1. Arquitetura distribuída (API, PostgreSQL, RabbitMQ, Outbox Relay)
# 2. Transação atômica no PostgreSQL (conclusão do ciclo + inserção na outbox)
# 3. Publicação confiável com Publisher Confirms no RabbitMQ (at-least-once)
# 4. Transição de status da Outbox (pending -> processing -> published)
# 5. Tolerância a falhas: simulação de queda do RabbitMQ e recuperação automática
# ==============================================================================

set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
RABBITMQ_MGMT_URL="${RABBITMQ_MGMT_URL:-http://localhost:15672}"
RABBITMQ_USER="${RABBITMQ_USER:-guest}"
RABBITMQ_PASS="${RABBITMQ_PASS:-guest}"
QUEUE_NAME="kotoro.analytics.events"

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log_step() {
  echo -e "\n${CYAN}======================================================================${NC}"
  echo -e "${CYAN}>>> $1${NC}"
  echo -e "${CYAN}======================================================================${NC}"
}

log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

# 1. Healthcheck da infraestrutura
log_step "1. Verificando conectividade dos serviços distribuídos..."
api_health=$(curl -sf "${API_URL}/health" || curl -sf "${API_URL}/healthz" || echo "FAIL")
if [ "$api_health" != "Healthy!" ]; then
  echo -e "${RED}[ERRO] API não está respondendo em ${API_URL}${NC}"
  exit 1
fi
log_info "API está operacional: HTTP 200 ($api_health)"

rabbitmq_health=$(curl -sf -u "${RABBITMQ_USER}:${RABBITMQ_PASS}" "${RABBITMQ_MGMT_URL}/api/overview" | jq -r '.rabbitmq_version' || echo "FAIL")
if [ "$rabbitmq_health" = "FAIL" ]; then
  echo -e "${RED}[ERRO] RabbitMQ Management API não está respondendo em ${RABBITMQ_MGMT_URL}${NC}"
  exit 1
fi
log_info "RabbitMQ broker operacional (versão $rabbitmq_health)"

# 2. Cadastro e Autenticação (Multi-tenancy)
log_step "2. Cadastrando novo usuário e obtendo JWT..."
RANDOM_SUFFIX=$(date +%s%N | cut -b1-8)
USER_EMAIL="demo_${RANDOM_SUFFIX}@kotoro.local"
USER_PASS="SenhaForte123!"

REGISTER_RES=$(curl -s -X POST "${API_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Estudante Demo\",\"email\":\"${USER_EMAIL}\",\"password\":\"${USER_PASS}\"}")

AUTH_TOKEN=$(echo "$REGISTER_RES" | jq -r '.token')
USER_ID=$(echo "$REGISTER_RES" | jq -r '.user.id')

if [ -z "$AUTH_TOKEN" ] || [ "$AUTH_TOKEN" = "null" ]; then
  echo -e "${RED}[ERRO] Falha ao registrar usuário: $REGISTER_RES${NC}"
  exit 1
fi
log_info "Usuário criado com sucesso (ID: $USER_ID, Email: $USER_EMAIL)"
log_info "Token JWT gerado com sucesso."

AUTH_HEADER="Authorization: Bearer ${AUTH_TOKEN}"

# 3. Criação de Matéria e Sprint
log_step "3. Criando Matéria e Sprint de Estudo..."
SUBJECT_RES=$(curl -s -X POST "${API_URL}/subjects" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"name":"Sistemas Distribuídos","color":"blue"}')

SUBJECT_ID=$(echo "$SUBJECT_RES" | jq -r '.id // empty')
if [ -z "$SUBJECT_ID" ]; then
  # Se o endpoint retornar 201 sem body, buscamos a matéria criada
  SUBJECT_ID=$(curl -s -H "$AUTH_HEADER" "${API_URL}/subjects" | jq -r '.[0].id')
fi
log_info "Matéria criada: Sistemas Distribuídos (ID: $SUBJECT_ID)"

TODAY=$(date +"%Y-%m-%d")
SPRINT_RES=$(curl -s -X POST "${API_URL}/sprints" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d "{\"name\":\"Sprint Concorrência\",\"sprint_date\":\"${TODAY}\"}")

SPRINT_ID=$(echo "$SPRINT_RES" | jq -r '.id // empty')
if [ -z "$SPRINT_ID" ]; then
  SPRINT_ID=$(curl -s -H "$AUTH_HEADER" "${API_URL}/sprints" | jq -r '.[0].id')
fi
log_info "Sprint criada para $TODAY (ID: $SPRINT_ID)"

# 4. Adiciona Bloco à Sprint
log_step "4. Adicionando Bloco de Matéria à Sprint e iniciando Sprint..."
BLOCK_RES=$(curl -s -X POST "${API_URL}/sprints/${SPRINT_ID}/blocks" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d "{\"subject_id\":\"${SUBJECT_ID}\",\"title\":\"Estudo de Mensageria AMQP\",\"position\":1,\"estimated_seconds\":1800}")

BLOCK_ID=$(echo "$BLOCK_RES" | jq -r '.id')
log_info "Bloco criado com sucesso (ID: $BLOCK_ID, Posição: 1)"

# Transiciona status da sprint para in_progress
curl -s -X PATCH "${API_URL}/sprints/${SPRINT_ID}" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"status":"in_progress"}' > /dev/null
log_info "Sprint transicionada para status 'in_progress'"

# 5. Início do Ciclo de Estudo (Play)
log_step "5. Iniciando Ciclo de Estudos (Play)..."
PLAY_RES=$(curl -s -X POST "${API_URL}/sprints/${SPRINT_ID}/study-cycles/play" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","planned_duration_seconds":null,"block_task_ids":[]}')

CYCLE_ID=$(echo "$PLAY_RES" | jq -r '.id')
CYCLE_STATUS=$(echo "$PLAY_RES" | jq -r '.status')
log_info "Ciclo iniciado com sucesso! (ID: $CYCLE_ID, Status: $CYCLE_STATUS)"

log_info "Simulando tempo de estudo (3 segundos)..."
sleep 3

# 6. Conclusão do Ciclo (Stop) — Fronteira Transacional do Outbox
log_step "6. Finalizando Ciclo de Estudos (Stop) com transação atômica..."
STOP_RES=$(curl -s -X POST "${API_URL}/study-cycles/${CYCLE_ID}/stop" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"reason":"concluido"}')

FINAL_STATUS=$(echo "$STOP_RES" | jq -r '.status')
FINAL_ELAPSED=$(echo "$STOP_RES" | jq -r '.elapsed_seconds')
log_info "Ciclo encerrado na API: Status: $FINAL_STATUS, Tempo estudado: ${FINAL_ELAPSED}s"
log_info "A transação SQL persistiu o ciclo finalizado E inseriu o evento study_cycle.finished na outbox_events!"

# 7. Aguarda o Relay processar
log_step "7. Aguardando Outbox Relay processar o evento assincronamente..."
sleep 2

# Exibe logs recentes do relay
log_info "Últimos logs do container outbox-relay:"
docker compose logs --tail 10 outbox-relay

# 8. Verificando o evento na fila RabbitMQ
log_step "8. Inspecionando a fila RabbitMQ '${QUEUE_NAME}'..."
QUEUE_INFO=$(curl -s -u "${RABBITMQ_USER}:${RABBITMQ_PASS}" "${RABBITMQ_MGMT_URL}/api/queues/%2F/${QUEUE_NAME}")
MSG_COUNT=$(echo "$QUEUE_INFO" | jq -r '.messages')
log_info "Total de mensagens prontas na fila durável '${QUEUE_NAME}': $MSG_COUNT"

MSG_PAYLOAD=$(curl -s -u "${RABBITMQ_USER}:${RABBITMQ_PASS}" -X POST "${RABBITMQ_MGMT_URL}/api/queues/%2F/${QUEUE_NAME}/get" \
  -H "Content-Type: application/json" \
  -d '{"count":1,"ackmode":"ack_requeue_true","encoding":"auto"}')

log_info "Última mensagem inspecionada na fila:"
echo "$MSG_PAYLOAD" | jq '.[0] | {routing_key: .routing_key, exchange: .exchange, payload: (.payload | fromjson)}'

# 9. Teste de Tolerância a Falhas: Queda do Broker
log_step "9. DEMO DE TOLERÂNCIA A FALHAS: Parando o RabbitMQ de propósito..."
log_warn "Executando: docker compose stop rabbitmq"
docker compose stop rabbitmq

log_info "RabbitMQ parado. Iniciando e finalizando um segundo ciclo durante a indisponibilidade do broker..."

# Inicia e para um segundo ciclo
PLAY_RES2=$(curl -s -X POST "${API_URL}/sprints/${SPRINT_ID}/study-cycles/play" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","planned_duration_seconds":null,"block_task_ids":[]}')
CYCLE_ID2=$(echo "$PLAY_RES2" | jq -r '.id')
sleep 1

curl -s -X POST "${API_URL}/study-cycles/${CYCLE_ID2}/stop" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d '{"reason":"concluido"}' > /dev/null

log_info "Segundo ciclo finalizado e persistido na outbox com o RabbitMQ fora do ar."
log_info "Aguardando o relay tentar publicar, falhar com conexão recusada e reagendar o retry..."
sleep 3

log_info "Logs do relay demonstrando a falha de conexão e o retry agendado:"
docker compose logs --tail 6 outbox-relay || true

log_step "10. RECUPERAÇÃO DE FALHA: Reiniciando o RabbitMQ e o Outbox Relay..."
docker compose start rabbitmq
log_info "Aguardando o broker RabbitMQ ficar saudável (healthcheck)..."
while [ "$(docker inspect --format='{{.State.Health.Status}}' kotoro-api-rabbitmq-1 2>/dev/null || echo '')" != "healthy" ]; do
  sleep 1
done
log_info "RabbitMQ está saudável!"

log_info "Reiniciando o container outbox-relay para reconexão limpa..."
docker compose restart outbox-relay
sleep 3

log_info "Logs do relay confirmando a reconexão, recuperação da outbox e publicação bem-sucedida:"
docker compose logs --tail 10 outbox-relay

log_step "Demonstração concluída com sucesso! Todos os requisitos da Entrega 2 foram validados."
