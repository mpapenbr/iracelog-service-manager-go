package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/speedmap"
)

type SpeedmapService struct {
	pool *pgxpool.Pool
}

func InitSpeedmapService(pool *pgxpool.Pool) *SpeedmapService {
	speedmapService := SpeedmapService{pool: pool}
	return &speedmapService
}

//nolint:whitespace // can't make both linter and editor happy
func (s *SpeedmapService) AddSpeedmap(
	ctx context.Context,
	entry *model.DbSpeedmap,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := speedmap.Create(ctx, tx.Conn(), entry)
		return err
	})
}
