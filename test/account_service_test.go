package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kvetinski/account/internal/domain"
	accountsvc "github.com/kvetinski/account/internal/service/account"
)

type fakeRepo struct {
	createFn     func(ctx context.Context, id uuid.UUID, nick, phone string) (domain.Account, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (domain.Account, error)
	updateNickFn func(ctx context.Context, id uuid.UUID, nick string) (domain.Account, error)
	deleteFn     func(ctx context.Context, id uuid.UUID) error
}

func (f fakeRepo) Create(ctx context.Context, id uuid.UUID, nick, phone string) (domain.Account, error) {
	return f.createFn(ctx, id, nick, phone)
}

func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return f.getByIDFn(ctx, id)
}

func (f fakeRepo) UpdateNick(ctx context.Context, id uuid.UUID, nick string) (domain.Account, error) {
	return f.updateNickFn(ctx, id, nick)
}

func (f fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}

func TestCreateRejectsInvalidPhone(t *testing.T) {
	svc := accountsvc.New(fakeRepo{})

	_, err := svc.Create(context.Background(), "123")
	if err != domain.ErrInvalidPhone {
		t.Fatalf("expected ErrInvalidPhone, got %v", err)
	}
}

func TestCreateGeneratesNickAndPassesPhoneToRepo(t *testing.T) {
	var gotNick string
	var gotPhone string
	now := time.Now()
	svc := accountsvc.New(fakeRepo{
		createFn: func(_ context.Context, id uuid.UUID, nick, phone string) (domain.Account, error) {
			gotNick = nick
			gotPhone = phone
			return domain.Account{ID: id, Nick: nick, Phone: phone, CreatedAt: now, UpdatedAt: now}, nil
		},
	})

	acc, err := svc.Create(context.Background(), "+15551234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPhone != "+15551234567" {
		t.Fatalf("expected phone +15551234567, got %s", gotPhone)
	}
	if gotNick == "" || gotNick[0] != '@' {
		t.Fatalf("expected generated nick starting with @, got %s", gotNick)
	}
	if acc.Phone != "+15551234567" {
		t.Fatalf("expected returned phone +15551234567, got %s", acc.Phone)
	}
}

func TestGetByIDForwardsRepoResult(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	expected := domain.Account{ID: id, Nick: "@nick", Phone: "+15551234567", CreatedAt: now, UpdatedAt: now}
	svc := accountsvc.New(fakeRepo{
		getByIDFn: func(_ context.Context, gotID uuid.UUID) (domain.Account, error) {
			if gotID != id {
				t.Fatalf("expected id %s, got %s", id, gotID)
			}
			return expected, nil
		},
	})

	acc, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if acc.ID != expected.ID || acc.Nick != expected.Nick || acc.Phone != expected.Phone {
		t.Fatalf("unexpected account: %#v", acc)
	}
}

func TestUpdateNickRejectsInvalidNick(t *testing.T) {
	svc := accountsvc.New(fakeRepo{})

	_, err := svc.UpdateNick(context.Background(), uuid.New(), "@")
	if err != domain.ErrInvalidNick {
		t.Fatalf("expected ErrInvalidNick, got %v", err)
	}
}

func TestDeleteForwardsRepoError(t *testing.T) {
	svc := accountsvc.New(fakeRepo{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrAccountNotFound
		},
	})

	err := svc.Delete(context.Background(), uuid.New())
	if err != domain.ErrAccountNotFound {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}
