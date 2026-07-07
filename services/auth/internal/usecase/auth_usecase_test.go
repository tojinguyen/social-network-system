package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"social-network-system/pkg/hash"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type mockUserRepo struct {
	users map[string]*domain.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	user.ID = primitive.NewObjectID()
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if user, ok := m.users[email]; ok {
		return user, nil
	}
	return nil, nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	for _, u := range m.users {
		if u.ID.Hex() == id {
			return u, nil
		}
	}
	return nil, nil
}

type mockTokenRepo struct {
	tokens map[string]string
}

func (m *mockTokenRepo) StoreRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	m.tokens[token] = userID
	return nil
}

func (m *mockTokenRepo) GetUserIDByRefreshToken(ctx context.Context, token string) (string, error) {
	if userID, ok := m.tokens[token]; ok {
		return userID, nil
	}
	return "", nil
}

func (m *mockTokenRepo) DeleteRefreshToken(ctx context.Context, token string) error {
	delete(m.tokens, token)
	return nil
}

func TestRegister(t *testing.T) {
	userRepo := &mockUserRepo{users: make(map[string]*domain.User)}
	tokenRepo := &mockTokenRepo{tokens: make(map[string]string)}
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("test_secret")

	uc := NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)

	req := &domain.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	// Test successful registration
	err := uc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Test registering duplicate email
	err = uc.Register(context.Background(), req)
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestLogin(t *testing.T) {
	userRepo := &mockUserRepo{users: make(map[string]*domain.User)}
	tokenRepo := &mockTokenRepo{tokens: make(map[string]string)}
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("test_secret")

	uc := NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)

	// Pre-register user
	regReq := &domain.RegisterRequest{
		Username: "loginuser",
		Email:    "login@example.com",
		Password: "password123",
	}
	_ = uc.Register(context.Background(), regReq)

	// Test successful login
	loginReq := &domain.LoginRequest{
		Email:    "login@example.com",
		Password: "password123",
	}
	res, err := uc.Login(context.Background(), loginReq)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens, got %+v", res)
	}

	// Test wrong password
	loginReq.Password = "wrongpass"
	_, err = uc.Login(context.Background(), loginReq)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}
