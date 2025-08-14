package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/salesworks/s-works/api/internal/platform/database"
)

type FabricPostgresRepository struct {
	db *database.PostgresDB
}

func NewFabricPostgresRepository(db *database.PostgresDB) *FabricPostgresRepository {
	return &FabricPostgresRepository{
		db: db,
	}
}

func (r *FabricPostgresRepository) Save(ctx context.Context, fabric *domain.Fabric) (*domain.Fabric, error) {
	tx, err := r.db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	findQuery := `SELECT version, code, name, measure_unit, offer_status, status FROM fabrics WHERE code = $1 FOR UPDATE`
	existingFabric := &domain.Fabric{}
	err = tx.QueryRowContext(ctx, findQuery, fabric.Code).Scan(
		&existingFabric.Version, &existingFabric.Code, &existingFabric.Name,
		&existingFabric.MeasureUnit, &existingFabric.OfferStatus, &existingFabric.Status,
	)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed during select for update: %w", err)
	}

	if err == nil && existingFabric.Status == domain.StatusActive {
		return nil, domain.ErrDuplicateFabricCode
	}

	if err == nil && existingFabric.Status == domain.StatusDeleted {
		err = existingFabric.Reactivate(fabric.Name, fabric.MeasureUnit, fabric.OfferStatus, existingFabric.Version)
		if err != nil {
			return nil, err
		}

		updateQuery := `UPDATE fabrics SET name = $1, measure_unit = $2, offer_status = $3, status = $4, version = $5 WHERE code = $6`
		_, err = tx.ExecContext(ctx, updateQuery, existingFabric.Name, existingFabric.MeasureUnit, existingFabric.OfferStatus, existingFabric.Status, existingFabric.Version, existingFabric.Code)
		if err != nil {
			return nil, fmt.Errorf("failed to reactivate fabric: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existingFabric, nil
	}

	insertQuery := `INSERT INTO fabrics (version, code, name, measure_unit, offer_status, status) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = tx.ExecContext(ctx, insertQuery, fabric.Version, fabric.Code, fabric.Name, fabric.MeasureUnit, fabric.OfferStatus, fabric.Status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrDuplicateFabricCode
		}
		return nil, fmt.Errorf("failed to insert new fabric: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return fabric, nil
}

func (r *FabricPostgresRepository) GetByCode(ctx context.Context, code string) (*domain.Fabric, error) {
	query := `
		SELECT version, code, name, measure_unit, offer_status, status
		FROM fabrics
		WHERE code = $1 AND status = 'ACTIVE'
	`

	fabric := &domain.Fabric{}
	err := r.db.Pool.QueryRowContext(ctx, query, code).Scan(
		&fabric.Version,
		&fabric.Code,
		&fabric.Name,
		&fabric.MeasureUnit,
		&fabric.OfferStatus,
		&fabric.Status, // The 6th variable
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fabric with code %s not found: %w", code, domain.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("failed to get fabric by code: %w", err)
	}

	return fabric, nil
}

func (r *FabricPostgresRepository) Update(ctx context.Context, fabric *domain.Fabric) error {
	query := `
		UPDATE fabrics
		SET name = $1, measure_unit = $2, offer_status = $3, version = $4
		WHERE code = $5 AND version = $6 AND status = 'ACTIVE'
	`
	args := []any{fabric.Name, fabric.MeasureUnit, fabric.OfferStatus, fabric.Version, fabric.Code, fabric.Version - 1}

	result, err := r.db.Pool.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update fabric: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrRecordNotFound
	}

	return nil
}

func (r *FabricPostgresRepository) Delete(ctx context.Context, fabric *domain.Fabric) error {
	query := `
		UPDATE fabrics
		SET status = $1
		WHERE code = $2 AND version = $3
	`
	args := []any{domain.StatusDeleted, fabric.Code, fabric.Version}

	result, err := r.db.Pool.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete fabric: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected post-delete: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrRecordNotFound
	}

	return nil
}

func (r *FabricPostgresRepository) GetByCodeIncludingDeleted(ctx context.Context, code string) (*domain.Fabric, error) {
	query := `
		SELECT version, code, name, measure_unit, offer_status, status
		FROM fabrics
		WHERE code = $1
	`

	fabric := &domain.Fabric{}
	err := r.db.Pool.QueryRowContext(ctx, query, code).Scan(
		&fabric.Version,
		&fabric.Code,
		&fabric.Name,
		&fabric.MeasureUnit,
		&fabric.OfferStatus,
		&fabric.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fabric with code %s not found: %w", code, domain.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("failed to get fabric by code: %w", err)
	}

	return fabric, nil
}
