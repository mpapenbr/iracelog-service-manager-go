package tcpostgres

import (
	"context"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer represents the postgres container type used in the module
type PostgresContainer struct {
	testcontainers.Container
}

type PostgresContainerOption func(req *testcontainers.ContainerRequest)

func WithWaitStrategy(strategies ...wait.Strategy) PostgresContainerOption {
	return func(req *testcontainers.ContainerRequest) {
		req.WaitingFor = wait.ForAll(strategies...).WithDeadline(1 * time.Minute)
	}
}

func WithPort(port string) PostgresContainerOption {
	return func(req *testcontainers.ContainerRequest) {
		req.ExposedPorts = append(req.ExposedPorts, port)
	}
}

func WithName(containerName string) PostgresContainerOption {
	return func(req *testcontainers.ContainerRequest) {
		req.Name = containerName
	}
}

func WithInitialDatabase(user, password, dbName string) PostgresContainerOption {
	return func(req *testcontainers.ContainerRequest) {
		req.Env["POSTGRES_USER"] = user
		req.Env["POSTGRES_PASSWORD"] = password
		req.Env["POSTGRES_DB"] = dbName
	}
}

// setupPostgres creates an instance of the postgres container type
func SetupPostgres(ctx context.Context, opts ...PostgresContainerOption) (
	*PostgresContainer, error,
) {
	// setup the container
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		Env:          map[string]string{},
		ExposedPorts: []string{},
		Cmd:          []string{"postgres", "-c", "fsync=off"},
	}

	for _, opt := range opts {
		opt(&req)
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
			Reuse:            true,
		})
	if err != nil {
		return nil, err
	}

	return &PostgresContainer{Container: container}, nil
}
