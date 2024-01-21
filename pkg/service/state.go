package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/state"
)

type StateService struct {
	pool *pgxpool.Pool
}

func InitStateService(pool *pgxpool.Pool) *StateService {
	stateService := StateService{pool: pool}
	return &stateService
}

func (s *StateService) AddState(ctx context.Context, entry *model.DbState) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := state.Create(ctx, tx.Conn(), entry)
		return err
	})
}
