//go:build integration

package test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/kvetinski/account/internal/adapters/grpcapi"
	"github.com/kvetinski/account/internal/adapters/grpcapi/accountv1"
	"github.com/kvetinski/account/internal/adapters/repository"
	accountsvc "github.com/kvetinski/account/internal/service/account"
)

func TestIntegrationGRPCCreateGetUpdateDelete(t *testing.T) {
	client, cleanup := setupIntegrationGRPCClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	created, err := client.CreateAccount(ctx, &accountv1.CreateAccountRequest{Phone: "+15550000001"})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	if created.GetAccount().GetNick() == "" {
		t.Fatal("expected generated nick, got empty")
	}

	got, err := client.GetAccount(ctx, &accountv1.GetAccountRequest{Id: created.GetAccount().GetId()})
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if got.GetAccount().GetPhone() != "+15550000001" {
		t.Fatalf("expected phone +15550000001, got %s", got.GetAccount().GetPhone())
	}

	updated, err := client.UpdateNick(ctx, &accountv1.UpdateNickRequest{Id: created.GetAccount().GetId(), Nick: "@int_nick_2"})
	if err != nil {
		t.Fatalf("UpdateNick failed: %v", err)
	}
	if updated.GetAccount().GetNick() != "@int_nick_2" {
		t.Fatalf("expected nick @int_nick_2, got %s", updated.GetAccount().GetNick())
	}

	if _, err = client.DeleteAccount(ctx, &accountv1.DeleteAccountRequest{Id: created.GetAccount().GetId()}); err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}

	_, err = client.GetAccount(ctx, &accountv1.GetAccountRequest{Id: created.GetAccount().GetId()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound after delete, got %v", status.Code(err))
	}
}

func TestIntegrationGRPCDuplicatePhone(t *testing.T) {
	client, cleanup := setupIntegrationGRPCClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.CreateAccount(ctx, &accountv1.CreateAccountRequest{Phone: "+15550000002"}); err != nil {
		t.Fatalf("first CreateAccount failed: %v", err)
	}

	_, err := client.CreateAccount(ctx, &accountv1.CreateAccountRequest{Phone: "+15550000002"})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", status.Code(err))
	}
}

func setupIntegrationGRPCClient(t *testing.T) (accountv1.AccountServiceClient, func()) {
	t.Helper()

	db := openIntegrationDB(t)
	resetSchema(t, db)

	repo := repository.NewWithMetrics(db, nil)
	svc := accountsvc.New(repo)
	server := grpcapi.NewServer(svc, slog.Default())

	grpcServer := grpc.NewServer()
	accountv1.RegisterAccountServiceServer(grpcServer, server)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc: %v", err)
	}

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDial()

	conn, err := grpc.DialContext(
		dialCtx,
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}

	cleanup := func() {
		conn.Close()
		grpcServer.Stop()
		lis.Close()
		db.Close()
	}

	return accountv1.NewAccountServiceClient(conn), cleanup
}

func openIntegrationDB(t *testing.T) *sql.DB {
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
		db.Close()
		t.Fatalf("ping db (%s): %v", uri, err)
	}

	return db
}

func resetSchema(t *testing.T, db *sql.DB) {
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

	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Fatalf("reset schema: %v", err)
	}

	if testing.Verbose() {
		fmt.Println("integration schema reset")
	}
}
