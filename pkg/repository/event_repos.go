package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

type EventRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

func (r *EventRepository) ListEvents(ctx context.Context) []*model.DbEvent {
	return nil
}
