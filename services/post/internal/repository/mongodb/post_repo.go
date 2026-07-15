package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"social-network-system/services/post/internal/domain"
)

type postRepo struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewPostRepository creates a new PostRepository instance and initializes indexes.
func NewPostRepository(db *mongo.Database) domain.PostRepository {
	collection := db.Collection("posts")

	// Create indexes asynchronously for optimal query performance
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				// Compound index for feed queries: find posts by author sorted by time
				Keys: bson.D{
					{Key: "author_id", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},
			{
				// Single index for sorting by creation time
				Keys: bson.D{{Key: "created_at", Value: -1}},
			},
		})
	}()

	return &postRepo{
		db:         db,
		collection: collection,
	}
}

func (r *postRepo) Create(ctx context.Context, post *domain.Post) error {
	post.ID = primitive.NewObjectID()
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()

	if post.MediaURLs == nil {
		post.MediaURLs = []string{}
	}
	if post.LikeUserIDs == nil {
		post.LikeUserIDs = []primitive.ObjectID{}
	}

	_, err := r.collection.InsertOne(ctx, post)
	return err
}

func (r *postRepo) FindByID(ctx context.Context, id string) (*domain.Post, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var post domain.Post
	err = r.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&post)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (r *postRepo) FindByAuthorIDs(ctx context.Context, authorIDs []primitive.ObjectID, cursor time.Time, limit int) ([]*domain.Post, error) {
	if len(authorIDs) == 0 {
		return []*domain.Post{}, nil
	}

	filter := bson.M{
		"author_id": bson.M{"$in": authorIDs},
	}

	// Apply cursor-based pagination: only fetch posts older than cursor
	if !cursor.IsZero() {
		filter["created_at"] = bson.M{"$lt": cursor}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var posts []*domain.Post
	if err := cur.All(ctx, &posts); err != nil {
		return nil, err
	}

	return posts, nil
}

func (r *postRepo) Delete(ctx context.Context, id string, authorID string) error {
	postObjID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	authorObjID, err := primitive.ObjectIDFromHex(authorID)
	if err != nil {
		return err
	}

	// Only delete if the post belongs to the author
	result, err := r.collection.DeleteOne(ctx, bson.M{
		"_id":       postObjID,
		"author_id": authorObjID,
	})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("post not found or unauthorized")
	}

	return nil
}
