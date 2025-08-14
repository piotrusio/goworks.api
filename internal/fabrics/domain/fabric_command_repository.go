package domain

import "context"

type FabricCommandRepository interface {
	Save(ctx context.Context, fabric *Fabric) (*Fabric, error)
	GetByCode(ctx context.Context, code string) (*Fabric, error)
	GetByCodeIncludingDeleted(ctx context.Context, code string) (*Fabric, error)
	Update(ctx context.Context, fabric *Fabric) error
	Delete(ctx context.Context, fabric *Fabric) error
}
