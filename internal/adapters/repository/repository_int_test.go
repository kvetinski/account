//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/kvetinski/account/internal/adapters/repository"
	"github.com/kvetinski/account/internal/domain"
)

type integrationSuite struct {
	db   *sql.DB
	repo *repository.Repository
}

func newIntegrationSuite(t *testing.T) *integrationSuite {
	t.Helper()

	uri := os.Getenv("TEST_POSTGRES_URI")
	if uri == "" {
		uri = "postgres://account:account@localhost:5432/account?sslmode=disable"
	}

	db, err := sql.Open("postgres", uri)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ping db (%s): %v", uri, err)
	}

	return &integrationSuite{
		db:   db,
		repo: repository.NewWithMetrics(db, nil),
	}
}

func (s *integrationSuite) close() {
	_ = s.db.Close()
}

func (s *integrationSuite) resetSchema(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
DROP TABLE IF EXISTS accounts;
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY,
    nick VARCHAR(31) NOT NULL UNIQUE,
    phone VARCHAR(20) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);
`

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

func TestIntegrationRepositorySuite(t *testing.T) {
	s := newIntegrationSuite(t)
	defer s.close()

	t.Run("CreateGetUpdateDelete", s.testCreateGetUpdateDelete)
	t.Run("CreateDuplicatePhone", s.testCreateDuplicatePhone)
	t.Run("UpdateNickConflict", s.testUpdateNickConflict)
	t.Run("DeleteNotFound", s.testDeleteNotFound)
}

func (s *integrationSuite) testCreateGetUpdateDelete(t *testing.T) {
	s.resetSchema(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id := uuid.New()
	created, err := s.repo.Create(ctx, id, "@repo_first", "+15550000101")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID != id {
		t.Fatalf("expected id %s, got %s", id, created.ID)
	}
	if created.DeletedAt != nil {
		t.Fatal("expected deleted_at to be nil")
	}

	got, err := s.repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Nick != "@repo_first" || got.Phone != "+15550000101" {
		t.Fatalf("unexpected account from GetByID: %+v", got)
	}

	time.Sleep(5 * time.Millisecond)

	updated, err := s.repo.UpdateNick(ctx, id, "@repo_second")
	if err != nil {
		t.Fatalf("UpdateNick failed: %v", err)
	}
	if updated.Nick != "@repo_second" {
		t.Fatalf("expected nick @repo_second, got %s", updated.Nick)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("expected updated_at to move forward, before=%s after=%s", created.UpdatedAt, updated.UpdatedAt)
	}

	if err = s.repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = s.repo.GetByID(ctx, id)
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound after delete, got %v", err)
	}
}

func (s *integrationSuite) testCreateDuplicatePhone(t *testing.T) {
	s.resetSchema(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.repo.Create(ctx, uuid.New(), "@phone_1", "+15550000102"); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err := s.repo.Create(ctx, uuid.New(), "@phone_2", "+15550000102")
	if !errors.Is(err, domain.ErrPhoneAlreadyExists) {
		t.Fatalf("expected ErrPhoneAlreadyExists, got %v", err)
	}
}

func (s *integrationSuite) testUpdateNickConflict(t *testing.T) {
	s.resetSchema(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := s.repo.Create(ctx, uuid.New(), "@nick_conflict_1", "+15550000103")
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	second, err := s.repo.Create(ctx, uuid.New(), "@nick_conflict_2", "+15550000104")
	if err != nil {
		t.Fatalf("second Create failed: %v", err)
	}

	_, err = s.repo.UpdateNick(ctx, second.ID, first.Nick)
	if !errors.Is(err, domain.ErrNickAlreadyExists) {
		t.Fatalf("expected ErrNickAlreadyExists, got %v", err)
	}
}

func (s *integrationSuite) testDeleteNotFound(t *testing.T) {
	s.resetSchema(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.repo.Delete(ctx, uuid.New())
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}
