package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	dpub "cixing/internal/modules/publishing/domain"
)

func TestUploadSessionServiceCreateRejectsUnknownContextAsInvalidInput(t *testing.T) {
	svc := &UploadSessionService{
		publishing: &Service{},
	}

	_, err := svc.Create(context.Background(), CreateUploadSessionInput{
		OwnerUserID: uuid.MustParse("51111111-1111-1111-1111-111111111111"),
		ContextType: dpub.PublishContextType("unexpected"),
	})
	if !errors.Is(err, ErrInvalidUploadPublishInput) {
		t.Fatalf("Create() error = %v, want invalid upload publish input", err)
	}
}

func TestServiceCreateOfficialSessionRejectsNilIDsAsInvalidInput(t *testing.T) {
	svc := &Service{db: new(pgxpool.Pool)}

	_, err := svc.CreateOfficialSession(context.Background(), CreateOfficialSessionInput{})
	if !errors.Is(err, ErrInvalidUploadPublishInput) {
		t.Fatalf("CreateOfficialSession() error = %v, want invalid upload publish input", err)
	}
}

func TestServiceCreateCustomSessionRejectsNilIDsAsInvalidInput(t *testing.T) {
	svc := &Service{db: new(pgxpool.Pool)}

	_, err := svc.CreateCustomSession(context.Background(), CreateCustomSessionInput{})
	if !errors.Is(err, ErrInvalidUploadPublishInput) {
		t.Fatalf("CreateCustomSession() error = %v, want invalid upload publish input", err)
	}
}

func TestServiceCommitSessionRejectsNilIDsAsInvalidInput(t *testing.T) {
	svc := &Service{db: new(pgxpool.Pool)}

	_, err := svc.CommitSession(context.Background(), CommitSessionInput{})
	if !errors.Is(err, ErrInvalidUploadPublishInput) {
		t.Fatalf("CommitSession() error = %v, want invalid upload publish input", err)
	}
}
