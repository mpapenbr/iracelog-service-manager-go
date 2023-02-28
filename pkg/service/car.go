package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/car"
)

type CarService struct {
	pool *pgxpool.Pool
}

func InitCarService(pool *pgxpool.Pool) *CarService {
	carService := CarService{pool: pool}
	return &carService
}

func (s *CarService) AddCar(entry *model.DbCar) error {
	return pgx.BeginFunc(context.Background(), s.pool, func(tx pgx.Tx) error {
		err := car.Create(tx.Conn(), entry)
		return err
	})
}
