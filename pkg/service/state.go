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

func (s *StateService) AddState(entry *model.DbState) error {
	return pgx.BeginFunc(context.Background(), s.pool, func(tx pgx.Tx) error {
		err := state.Create(tx.Conn(), entry)
		return err
	})
}
