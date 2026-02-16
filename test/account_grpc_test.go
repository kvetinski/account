package test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/kvetinski/account/internal/adapters/grpcapi"
	"github.com/kvetinski/account/internal/adapters/grpcapi/accountv1"
	"github.com/kvetinski/account/internal/domain"
	accountsvc "github.com/kvetinski/account/internal/service/account"
)

const bufSize = 1024 * 1024

type grpcRepoStub struct {
	account domain.Account
	err     error
}

func (s grpcRepoStub) Create(_ context.Context, _ uuid.UUID, _, _ string) (domain.Account, error) {
	panic("unexpected call")
}

func (s grpcRepoStub) GetByID(_ context.Context, _ uuid.UUID) (domain.Account, error) {
	if s.err != nil {
		return domain.Account{}, s.err
	}
	return s.account, nil
}

func (s grpcRepoStub) UpdateNick(_ context.Context, _ uuid.UUID, _ string) (domain.Account, error) {
	panic("unexpected call")
}

func (s grpcRepoStub) Delete(_ context.Context, _ uuid.UUID) error {
	panic("unexpected call")
}

func startGRPCClient(t *testing.T, repo grpcRepoStub) accountv1.AccountServiceClient {
	t.Helper()

	listener := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	svc := accountsvc.New(repo)
	accountv1.RegisterAccountServiceServer(s, grpcapi.NewServer(svc, slog.Default()))

	go func() {
		_ = s.Serve(listener)
	}()

	t.Cleanup(func() {
		s.Stop()
		_ = listener.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return accountv1.NewAccountServiceClient(conn)
}

func TestGetAccountGRPCSuccess(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	acc := domain.Account{ID: uuid.New(), Nick: "@john", Phone: "+15551234567", CreatedAt: now, UpdatedAt: now}
	client := startGRPCClient(t, grpcRepoStub{account: acc})

	resp, err := client.GetAccount(context.Background(), &accountv1.GetAccountRequest{Id: acc.ID.String()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.GetAccount().GetId() != acc.ID.String() {
		t.Fatalf("expected id %s, got %s", acc.ID, resp.GetAccount().GetId())
	}
	if resp.GetAccount().GetNick() != acc.Nick {
		t.Fatalf("expected nick %s, got %s", acc.Nick, resp.GetAccount().GetNick())
	}
	if resp.GetAccount().GetPhone() != acc.Phone {
		t.Fatalf("expected phone %s, got %s", acc.Phone, resp.GetAccount().GetPhone())
	}
}

func TestGetAccountGRPCNotFound(t *testing.T) {
	client := startGRPCClient(t, grpcRepoStub{err: domain.ErrAccountNotFound})

	_, err := client.GetAccount(context.Background(), &accountv1.GetAccountRequest{Id: uuid.New().String()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}
}

func TestGetAccountGRPCInvalidID(t *testing.T) {
	client := startGRPCClient(t, grpcRepoStub{})

	_, err := client.GetAccount(context.Background(), &accountv1.GetAccountRequest{Id: "not-a-uuid"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}
