package grpcapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kvetinski/account/internal/adapters/grpcapi/accountv1"
	"github.com/kvetinski/account/internal/domain"
	accountsvc "github.com/kvetinski/account/internal/service/account"
)

type Server struct {
	accountv1.UnimplementedAccountServiceServer

	svc    *accountsvc.Service
	logger *slog.Logger
}

func NewServer(svc *accountsvc.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{svc: svc, logger: logger}
}

func (s *Server) CreateAccount(ctx context.Context, req *accountv1.CreateAccountRequest) (*accountv1.AccountResponse, error) {
	acc, err := s.svc.Create(ctx, req.GetPhone())
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &accountv1.AccountResponse{Account: toProtoAccount(acc)}, nil
}

func (s *Server) GetAccount(ctx context.Context, req *accountv1.GetAccountRequest) (*accountv1.AccountResponse, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}

	acc, err := s.svc.GetByID(ctx, id)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &accountv1.AccountResponse{Account: toProtoAccount(acc)}, nil
}

func (s *Server) UpdateNick(ctx context.Context, req *accountv1.UpdateNickRequest) (*accountv1.AccountResponse, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}

	acc, err := s.svc.UpdateNick(ctx, id, req.GetNick())
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &accountv1.AccountResponse{Account: toProtoAccount(acc)}, nil
}

func (s *Server) DeleteAccount(ctx context.Context, req *accountv1.DeleteAccountRequest) (*emptypb.Empty, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}

	if err = s.svc.Delete(ctx, id); err != nil {
		return nil, mapDomainError(err)
	}

	return &emptypb.Empty{}, nil
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid account id")
	}

	return id, nil
}

func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidNick):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidPhone):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNickAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrPhoneAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrAccountNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func toProtoAccount(acc domain.Account) *accountv1.Account {
	out := &accountv1.Account{
		Id:        acc.ID.String(),
		Nick:      acc.Nick,
		Phone:     acc.Phone,
		CreatedAt: timestamppb.New(acc.CreatedAt),
		UpdatedAt: timestamppb.New(acc.UpdatedAt),
	}

	if acc.DeletedAt != nil {
		out.DeletedAt = timestamppb.New(*acc.DeletedAt)
	}

	return out
}
