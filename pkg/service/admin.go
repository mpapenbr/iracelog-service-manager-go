package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/analysis"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/speedmap"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/state"
)

type AdminService struct {
	pool *pgxpool.Pool
}

func InitAdminService(pool *pgxpool.Pool) *AdminService {
	adminService := AdminService{pool: pool}
	return &adminService
}

func (s *AdminService) DeleteEvent(id int) error {
	return pgx.BeginFunc(context.Background(), s.pool, func(tx pgx.Tx) error {
		var err error
		_, err = car.DeleteByEventId(tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = state.DeleteByEventId(tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = speedmap.DeleteByEventId(tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = analysis.DeleteByEventId(tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = event.DeleteExtraById(tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = event.DeleteById(tx.Conn(), id)
		return err
	})
}
