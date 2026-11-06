package outbox

import "github.com/jmoiron/sqlx"

type OutboxRepository struct {
	db *sqlx.DB
}

func NewOutboxRepository(db *sqlx.DB) *OutboxRepository {
	return &OutboxRepository{
		db: db,
	}
}

func (r *OutboxRepository) GetSprintData() {
}
