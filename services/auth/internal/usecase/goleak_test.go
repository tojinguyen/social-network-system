package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"social-network-system/pkg/hash"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/internal/domain"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestAuthUseCase_NoGoroutineLeak(t *testing.T) {
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("secret_key_123")
	userRepo := &mockUserRepo{users: make(map[string]*domain.User)}
	tokenRepo := &mockTokenRepo{tokens: make(map[string]string)}

	uc := NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)

	err := uc.Register(context.Background(), &domain.RegisterRequest{
		Username: "goleak_user",
		Email:    "goleak@example.com",
		Password: "Password123!",
	})
	require.NoError(t, err)

	res, err := uc.Login(context.Background(), &domain.LoginRequest{
		Email:    "goleak@example.com",
		Password: "Password123!",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestAuthUseCase_GoroutineLeakDetection(t *testing.T) {
	errBefore := goleak.Find()
	assert.NoError(t, errBefore, "Ban đầu không được có goroutine leak")

	unbufferedChan := make(chan string)
	go func() {
		_ = <-unbufferedChan
	}()

	time.Sleep(20 * time.Millisecond)

	errAfter := goleak.Find()
	require.Error(t, errAfter, "goleak phải phát hiện ra Goroutine đang bị kẹt!")

	t.Logf("=== goleak đã phát hiện rò rỉ thành công! ===\nChi tiết Stack Trace:\n%v", errAfter)

	close(unbufferedChan)
}

func TestRealWorld_GoroutineLeakDemo(t *testing.T) {
	go func() {
		ch := make(chan int)
		<-ch
	}()
}
