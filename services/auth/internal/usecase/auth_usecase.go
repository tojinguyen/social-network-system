package usecase

import (
	"context"
	"errors"
	"time"

	"social-network-system/pkg/hash"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/internal/domain"
)

var (
	// ErrUserAlreadyExists is returned when registering with an email already taken.
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrInvalidCredentials is returned on login failure.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrInvalidToken is returned when refresh token is invalid or expired.
	ErrInvalidToken = errors.New("invalid refresh token")
)

// AuthUseCase defines the business logic contract for authentication.
type AuthUseCase interface {
	Register(ctx context.Context, req *domain.RegisterRequest) error
	Login(ctx context.Context, req *domain.LoginRequest) (*domain.TokenResponse, error)
	Refresh(ctx context.Context, req *domain.RefreshRequest) (*domain.TokenResponse, error)
	Logout(ctx context.Context, req *domain.LogoutRequest) error
}

type authUseCase struct {
	userRepo   domain.UserRepository
	tokenRepo  domain.TokenRepository
	hasher     hash.PasswordHasher
	jwtManager jwtutil.TokenManager
}

// NewAuthUseCase creates a new AuthUseCase instance.
func NewAuthUseCase(
	userRepo domain.UserRepository,
	tokenRepo domain.TokenRepository,
	hasher hash.PasswordHasher,
	jwtManager jwtutil.TokenManager,
) AuthUseCase {
	return &authUseCase{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		hasher:     hasher,
		jwtManager: jwtManager,
	}
}

func (u *authUseCase) Register(ctx context.Context, req *domain.RegisterRequest) error {
	existingUser, err := u.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return ErrUserAlreadyExists
	}

	hashedPassword, err := u.hasher.HashPassword(req.Password)
	if err != nil {
		return err
	}

	user := &domain.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
	}

	return u.userRepo.Create(ctx, user)
}

func (u *authUseCase) Login(ctx context.Context, req *domain.LoginRequest) (*domain.TokenResponse, error) {
	user, err := u.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	err = u.hasher.ComparePassword(user.PasswordHash, req.Password)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, err := u.jwtManager.GenerateAccessToken(user.ID.Hex(), 15*time.Minute)
	if err != nil {
		return nil, err
	}

	refreshToken, err := u.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Store Refresh Token in Redis with 7 days TTL
	err = u.tokenRepo.StoreRefreshToken(ctx, refreshToken, user.ID.Hex(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (u *authUseCase) Refresh(ctx context.Context, req *domain.RefreshRequest) (*domain.TokenResponse, error) {
	userID, err := u.tokenRepo.GetUserIDByRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, ErrInvalidToken
	}

	// Generate new access token
	accessToken, err := u.jwtManager.GenerateAccessToken(userID, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	return &domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken,
	}, nil
}

func (u *authUseCase) Logout(ctx context.Context, req *domain.LogoutRequest) error {
	return u.tokenRepo.DeleteRefreshToken(ctx, req.RefreshToken)
}
