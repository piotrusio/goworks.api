package bootstrap

import (
	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/salesworks/s-works/api/internal/fabrics/handler"
	"github.com/salesworks/s-works/api/internal/fabrics/infrastructure/persistence"
	"github.com/salesworks/s-works/api/internal/platform/database"
)

type Repositories struct {
	postgres                *database.PostgresDB
	FabricCommandRepository domain.FabricCommandRepository
	FabricQueryRepository   handler.FabricQueryRepository
}

func NewRepositories(postgres *database.PostgresDB) Repositories {
	postgresRepo := persistence.NewFabricPostgresRepository(postgres)
	return Repositories{
		postgres:                postgres,
		FabricCommandRepository: postgresRepo,
		FabricQueryRepository:   postgresRepo,
	}
}
