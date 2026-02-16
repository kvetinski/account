package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/kvetinski/account/internal/domain"
	"github.com/kvetinski/account/internal/telemetry"
)

type Repository struct {
	db      *sql.DB
	metrics *telemetry.Metrics
}

func New(db *sql.DB) *Repository {
	return NewWithMetrics(db, nil)
}

func NewWithMetrics(db *sql.DB, metrics *telemetry.Metrics) *Repository {
	return &Repository{
		db:      db,
		metrics: metrics,
	}
}

func (r *Repository) Create(ctx context.Context, id uuid.UUID, nick, phone string) (domain.Account, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		r.metrics.ObserveDB("create", status, time.Since(start))
	}()

	const q = `
		INSERT INTO accounts (id, nick, phone)
		VALUES ($1, $2, $3)
		RETURNING id, nick, phone, created_at, updated_at, deleted_at
	`

	var a domain.Account
	if err := r.db.QueryRowContext(ctx, q, id, nick, phone).Scan(&a.ID, &a.Nick, &a.Phone, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			status = "conflict"
			switch pqErr.Constraint {
			case "accounts_nick_key":
				return domain.Account{}, domain.ErrNickAlreadyExists
			case "accounts_phone_key":
				return domain.Account{}, domain.ErrPhoneAlreadyExists
			default:
				return domain.Account{}, domain.ErrNickAlreadyExists
			}
		}

		status = "error"
		return domain.Account{}, fmt.Errorf("create account: %w", err)
	}

	return a, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		r.metrics.ObserveDB("get_by_id", status, time.Since(start))
	}()

	const q = `
		SELECT id, nick, phone, created_at, updated_at, deleted_at
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var a domain.Account
	if err := r.db.QueryRowContext(ctx, q, id).Scan(&a.ID, &a.Nick, &a.Phone, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status = "not_found"
			return domain.Account{}, domain.ErrAccountNotFound
		}

		status = "error"
		return domain.Account{}, fmt.Errorf("get account: %w", err)
	}

	return a, nil
}

func (r *Repository) UpdateNick(ctx context.Context, id uuid.UUID, nick string) (domain.Account, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		r.metrics.ObserveDB("update_nick", status, time.Since(start))
	}()

	const q = `
		UPDATE accounts
		SET nick = $2,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, nick, phone, created_at, updated_at, deleted_at
	`

	var a domain.Account
	if err := r.db.QueryRowContext(ctx, q, id, nick).Scan(&a.ID, &a.Nick, &a.Phone, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status = "not_found"
			return domain.Account{}, domain.ErrAccountNotFound
		}

		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			status = "conflict"
			return domain.Account{}, domain.ErrNickAlreadyExists
		}

		status = "error"
		return domain.Account{}, fmt.Errorf("update nick: %w", err)
	}

	return a, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	status := "ok"
	defer func() {
		r.metrics.ObserveDB("delete", status, time.Since(start))
	}()

	const q = `
		UPDATE accounts
		SET deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		status = "error"
		return fmt.Errorf("delete account: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		status = "error"
		return fmt.Errorf("delete account rows affected: %w", err)
	}

	if rows == 0 {
		status = "not_found"
		return domain.ErrAccountNotFound
	}

	return nil
}
