package account

import (
	"context"
	"crypto/rand"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/kvetinski/account/internal/domain"
)

const maxNickGenerationAttempts = 10

var (
	nickPattern  = regexp.MustCompile(`^@[a-zA-Z0-9_]{2,30}$`)
	phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

type Repository interface {
	Create(ctx context.Context, id uuid.UUID, nick, phone string) (domain.Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
	UpdateNick(ctx context.Context, id uuid.UUID, nick string) (domain.Account, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, phone string) (domain.Account, error) {
	phone = strings.TrimSpace(phone)
	if !isValidPhone(phone) {
		return domain.Account{}, domain.ErrInvalidPhone
	}

	id := uuid.New()
	for range maxNickGenerationAttempts {
		nick, err := generateNick()
		if err != nil {
			return domain.Account{}, err
		}

		acc, err := s.repo.Create(ctx, id, nick, phone)
		if err == nil {
			return acc, nil
		}

		if err == domain.ErrNickAlreadyExists {
			continue
		}

		return domain.Account{}, err
	}

	return domain.Account{}, domain.ErrNickAlreadyExists
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) UpdateNick(ctx context.Context, id uuid.UUID, nick string) (domain.Account, error) {
	nick = strings.TrimSpace(nick)
	if !isValidNick(nick) {
		return domain.Account{}, domain.ErrInvalidNick
	}

	return s.repo.UpdateNick(ctx, id, nick)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func isValidNick(nick string) bool {
	return nickPattern.MatchString(nick)
}

func isValidPhone(phone string) bool {
	return phonePattern.MatchString(phone)
}

func generateNick() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}

	return "@" + string(buf), nil
}
