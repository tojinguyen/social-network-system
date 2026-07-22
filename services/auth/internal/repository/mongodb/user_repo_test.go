//go:build integration

package mongodb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcMongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"social-network-system/services/auth/internal/domain"
)

func setupTestMongoDB(t *testing.T) (*mongo.Client, *mongo.Database, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mongoContainer, err := tcMongo.Run(ctx, "mongo:6.0")
	require.NoError(t, err)

	endpoint, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(endpoint))
	require.NoError(t, err)

	db := client.Database("test_social_network")

	cleanup := func() {
		_ = client.Disconnect(context.Background())
		_ = mongoContainer.Terminate(context.Background())
	}

	return client, db, cleanup
}

func TestUserRepository_Integration(t *testing.T) {
	_, db, cleanup := setupTestMongoDB(t)
	defer cleanup()

	repo := NewUserRepository(db)
	ctx := context.Background()

	testUser := &domain.User{
		Username:     "integration_user",
		Email:        "integration@example.com",
		PasswordHash: "hashed_pass_123",
	}

	t.Run("Create User", func(t *testing.T) {
		err := repo.Create(ctx, testUser)
		require.NoError(t, err)
		assert.False(t, testUser.ID.IsZero())
		assert.False(t, testUser.CreatedAt.IsZero())
	})

	t.Run("Find User By Email", func(t *testing.T) {
		found, err := repo.FindByEmail(ctx, testUser.Email)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, testUser.Username, found.Username)
		assert.Equal(t, testUser.Email, found.Email)
	})

	t.Run("Find User By ID", func(t *testing.T) {
		found, err := repo.FindByID(ctx, testUser.ID.Hex())
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, testUser.Email, found.Email)
	})

	t.Run("Find User Not Found", func(t *testing.T) {
		found, err := repo.FindByEmail(ctx, "notfound@example.com")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}
