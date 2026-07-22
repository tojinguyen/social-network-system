package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"social-network-system/pkg/hash"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/internal/domain"
)

// Mock Repositories
type mockUserRepo struct {
	users             map[string]*domain.User
	findByEmailErr    error
	createErr         error
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	user.ID = primitive.NewObjectID()
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
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
	tokens         map[string]string
	storeErr       error
	getUserIDErr   error
	deleteErr      error
}

func (m *mockTokenRepo) StoreRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	m.tokens[token] = userID
	return nil
}

func (m *mockTokenRepo) GetUserIDByRefreshToken(ctx context.Context, token string) (string, error) {
	if m.getUserIDErr != nil {
		return "", m.getUserIDErr
	}
	if userID, ok := m.tokens[token]; ok {
		return userID, nil
	}
	return "", nil
}

func (m *mockTokenRepo) DeleteRefreshToken(ctx context.Context, token string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.tokens, token)
	return nil
}

type mockErrJWTManager struct {
	jwtutil.TokenManager
	genAccessErr error
}

func (m *mockErrJWTManager) GenerateAccessToken(userID string, duration time.Duration) (string, error) {
	if m.genAccessErr != nil {
		return "", m.genAccessErr
	}
	return m.TokenManager.GenerateAccessToken(userID, duration)
}

func TestAuthUseCase_Register(t *testing.T) {
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("test_secret")

	tests := []struct {
		name        string
		setupMocks  func() domain.UserRepository
		req         *domain.RegisterRequest
		expectedErr error
	}{
		{
			name: "Success",
			setupMocks: func() domain.UserRepository {
				return &mockUserRepo{users: make(map[string]*domain.User)}
			},
			req: &domain.RegisterRequest{
				Username: "newuser",
				Email:    "new@example.com",
				Password: "password123",
			},
			expectedErr: nil,
		},
		{
			name: "Error - Duplicate Email",
			setupMocks: func() domain.UserRepository {
				repo := &mockUserRepo{users: make(map[string]*domain.User)}
				repo.users["existing@example.com"] = &domain.User{
					ID:       primitive.NewObjectID(),
					Username: "existing",
					Email:    "existing@example.com",
				}
				return repo
			},
			req: &domain.RegisterRequest{
				Username: "newuser",
				Email:    "existing@example.com",
				Password: "password123",
			},
			expectedErr: ErrUserAlreadyExists,
		},
		{
			name: "Error - DB FindByEmail Failure",
			setupMocks: func() domain.UserRepository {
				return &mockUserRepo{
					users:          make(map[string]*domain.User),
					findByEmailErr: errors.New("db connection failure"),
				}
			},
			req: &domain.RegisterRequest{
				Username: "newuser",
				Email:    "new@example.com",
				Password: "password123",
			},
			expectedErr: errors.New("db connection failure"),
		},
		{
			name: "Error - DB Create Failure",
			setupMocks: func() domain.UserRepository {
				return &mockUserRepo{
					users:     make(map[string]*domain.User),
					createErr: errors.New("insert failed"),
				}
			},
			req: &domain.RegisterRequest{
				Username: "newuser",
				Email:    "new@example.com",
				Password: "password123",
			},
			expectedErr: errors.New("insert failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := tt.setupMocks()
			tokenRepo := &mockTokenRepo{tokens: make(map[string]string)}
			uc := NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)

			err := uc.Register(context.Background(), tt.req)
			if tt.expectedErr != nil {
				require.Error(t, err)
				if errors.Is(tt.expectedErr, ErrUserAlreadyExists) {
					assert.ErrorIs(t, err, tt.expectedErr)
				} else {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthUseCase_Login(t *testing.T) {
	hasher := hash.NewBcryptHasher(0)
	hashedPass, _ := hasher.HashPassword("password123")
	userObjID := primitive.NewObjectID()

	existingUser := &domain.User{
		ID:           userObjID,
		Username:     "loginuser",
		Email:        "login@example.com",
		PasswordHash: hashedPass,
	}

	jwtManager := jwtutil.NewJWTManager("test_secret")

	tests := []struct {
		name        string
		userRepo    domain.UserRepository
		tokenRepo   domain.TokenRepository
		customJWT   jwtutil.TokenManager
		req         *domain.LoginRequest
		expectedErr error
	}{
		{
			name: "Success",
			userRepo: &mockUserRepo{
				users: map[string]*domain.User{"login@example.com": existingUser},
			},
			tokenRepo: &mockTokenRepo{tokens: make(map[string]string)},
			req: &domain.LoginRequest{
				Email:    "login@example.com",
				Password: "password123",
			},
			expectedErr: nil,
		},
		{
			name: "Error - User Not Found",
			userRepo: &mockUserRepo{
				users: make(map[string]*domain.User),
			},
			tokenRepo: &mockTokenRepo{tokens: make(map[string]string)},
			req: &domain.LoginRequest{
				Email:    "notfound@example.com",
				Password: "password123",
			},
			expectedErr: ErrInvalidCredentials,
		},
		{
			name: "Error - Incorrect Password",
			userRepo: &mockUserRepo{
				users: map[string]*domain.User{"login@example.com": existingUser},
			},
			tokenRepo: &mockTokenRepo{tokens: make(map[string]string)},
			req: &domain.LoginRequest{
				Email:    "login@example.com",
				Password: "wrongpassword",
			},
			expectedErr: ErrInvalidCredentials,
		},
		{
			name: "Error - Redis Store Failure",
			userRepo: &mockUserRepo{
				users: map[string]*domain.User{"login@example.com": existingUser},
			},
			tokenRepo: &mockTokenRepo{
				tokens:   make(map[string]string),
				storeErr: errors.New("redis connection down"),
			},
			req: &domain.LoginRequest{
				Email:    "login@example.com",
				Password: "password123",
			},
			expectedErr: errors.New("redis connection down"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jm := jwtManager
			if tt.customJWT != nil {
				jm = tt.customJWT
			}
			uc := NewAuthUseCase(tt.userRepo, tt.tokenRepo, hasher, jm)

			res, err := uc.Login(context.Background(), tt.req)
			if tt.expectedErr != nil {
				require.Error(t, err)
				if errors.Is(tt.expectedErr, ErrInvalidCredentials) {
					assert.ErrorIs(t, err, tt.expectedErr)
				} else {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				}
				assert.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				assert.NotEmpty(t, res.AccessToken)
				assert.NotEmpty(t, res.RefreshToken)
			}
		})
	}
}

func TestAuthUseCase_Refresh(t *testing.T) {
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("test_secret")
	validUserID := primitive.NewObjectID().Hex()
	validRefreshToken := "valid-uuid-refresh-token"

	tokenRepo := &mockTokenRepo{
		tokens: map[string]string{
			validRefreshToken: validUserID,
		},
	}

	userRepo := &mockUserRepo{users: make(map[string]*domain.User)}
	uc := NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)

	t.Run("Success - Refresh Token", func(t *testing.T) {
		res, err := uc.Refresh(context.Background(), &domain.RefreshRequest{RefreshToken: validRefreshToken})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.NotEmpty(t, res.AccessToken)
		assert.Equal(t, validRefreshToken, res.RefreshToken)
	})

	t.Run("Error - Invalid or Expired Refresh Token", func(t *testing.T) {
		res, err := uc.Refresh(context.Background(), &domain.RefreshRequest{RefreshToken: "non-existent-token"})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Nil(t, res)
	})

	t.Run("Error - Redis Get Failure", func(t *testing.T) {
		errTokenRepo := &mockTokenRepo{
			tokens:       make(map[string]string),
			getUserIDErr: errors.New("redis timeout"),
		}
		errUC := NewAuthUseCase(userRepo, errTokenRepo, hasher, jwtManager)
		res, err := errUC.Refresh(context.Background(), &domain.RefreshRequest{RefreshToken: validRefreshToken})
		require.Error(t, err)
		assert.Equal(t, "redis timeout", err.Error())
		assert.Nil(t, res)
	})
}

func TestAuthUseCase_Logout(t *testing.T) {
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("test_secret")
	tokenRepo := &mockTokenRepo{tokens: map[string]string{"active_token": "user_123"}}
	userRepo := &mockUserRepo{users: make(map[string]*domain.User)}

	uc := NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)

	t.Run("Success - Logout", func(t *testing.T) {
		err := uc.Logout(context.Background(), &domain.LogoutRequest{RefreshToken: "active_token"})
		assert.NoError(t, err)

		// Verify token deleted
		userID, _ := tokenRepo.GetUserIDByRefreshToken(context.Background(), "active_token")
		assert.Empty(t, userID)
	})

	t.Run("Error - Delete Failure", func(t *testing.T) {
		errTokenRepo := &mockTokenRepo{
			tokens:    make(map[string]string),
			deleteErr: errors.New("redis failure"),
		}
		errUC := NewAuthUseCase(userRepo, errTokenRepo, hasher, jwtManager)
		err := errUC.Logout(context.Background(), &domain.LogoutRequest{RefreshToken: "active_token"})
		require.Error(t, err)
		assert.Equal(t, "redis failure", err.Error())
	})
}
