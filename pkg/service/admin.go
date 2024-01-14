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
	ctx := context.Background()
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		_, err = car.DeleteByEventId(ctx, tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = state.DeleteByEventId(ctx, tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = speedmap.DeleteByEventId(ctx, tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = analysis.DeleteByEventId(ctx, tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = event.DeleteExtraById(ctx, tx.Conn(), id)
		if err != nil {
			return err
		}
		_, err = event.DeleteById(ctx, tx.Conn(), id)
		return err
	})
}
