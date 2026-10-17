package post_service

import (
	"context"
	"errors"
	"fmt"
	"go_distributed_social_media/internal/common/errs"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MetadataRepo interface {
	Create(ctx context.Context, dto CreatePostDTO) (CreatePostResponse, error)
	Get(ctx context.Context, postID string) (Post, error)
	GetByUsers(ctx context.Context, dto GetPostsByUsersDTO) ([]Post, error)
}

type postDocument struct {
	ID        string           `bson:"_id"`
	UserID    string           `bson:"user_id"`
	Content   string           `bson:"content"`
	Media     *MediaAttachment `bson:"media,omitempty"`
	CreatedAt time.Time        `bson:"created_at"`
	UpdatedAt time.Time        `bson:"updated_at"`
}

type MongoRepo struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func NewMongoRepo(db *mongo.Database) *MongoRepo {
	repo := &MongoRepo{
		db:         db,
		collection: db.Collection("posts"),
	}

	repo.ensureIndexes()
	return repo
}

func (m *MongoRepo) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "created_at", Value: -1},
		},
		Options: options.Index().SetName("idx_user_posts"),
	}

	_, _ = m.collection.Indexes().CreateOne(ctx, indexModel)
}

func (m *MongoRepo) Create(ctx context.Context, dto CreatePostDTO) (CreatePostResponse, error) {
	now := time.Now().UTC()
	docID := bson.NewObjectID().Hex()

	doc := postDocument{
		ID:        docID,
		UserID:    dto.UserID,
		Content:   dto.Content,
		Media:     dto.Media,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := m.collection.InsertOne(ctx, doc)
	if err != nil {
		return CreatePostResponse{}, fmt.Errorf("failed to insert post: %w", err)
	}

	return CreatePostResponse{
		PostID:    docID,
		CreatedAt: int(now.Unix()),
	}, nil
}

func (m *MongoRepo) Get(ctx context.Context, postID string) (Post, error) {
	var doc postDocument

	filter := bson.M{"_id": postID}

	err := m.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Post{}, &errs.Err{Err: errs.NotFound, Message: "post not found"}
		}
		return Post{}, fmt.Errorf("failed to fetch post: %w", err)
	}

	return Post{
		ID:        doc.ID,
		UserID:    doc.UserID,
		Content:   doc.Content,
		Media:     doc.Media,
		CreatedAt: doc.CreatedAt.Unix(),
	}, nil
}

func (m *MongoRepo) GetByUsers(ctx context.Context, dto GetPostsByUsersDTO) ([]Post, error) {
	if len(dto.UserIDs) == 0 {
		return []Post{}, nil
	}

	match := bson.M{"user_id": bson.M{"$in": dto.UserIDs}}
	if dto.BeforeCreatedAt > 0 {
		boundary := time.Unix(dto.BeforeCreatedAt, 0).UTC()
		match["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": boundary}},
			bson.M{
				"created_at": bson.M{"$gte": boundary, "$lt": boundary.Add(time.Second)},
				"_id":        bson.M{"$lt": dto.BeforePostID},
			},
		}
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$addFields", Value: bson.M{
			"created_at_second": bson.M{"$dateTrunc": bson.M{"date": "$created_at", "unit": "second"}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "created_at_second", Value: -1}, {Key: "_id", Value: -1}}}},
		{{Key: "$limit", Value: MaxPostsByUsers}},
	}
	cursor, err := m.collection.Aggregate(ctx, pipeline, options.Aggregate())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts by users: %w", err)
	}
	defer cursor.Close(ctx)

	documents := make([]postDocument, 0, MaxPostsByUsers)
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, fmt.Errorf("failed to decode posts by users: %w", err)
	}

	posts := make([]Post, len(documents))
	for i, doc := range documents {
		posts[i] = Post{
			ID:        doc.ID,
			UserID:    doc.UserID,
			Content:   doc.Content,
			Media:     doc.Media,
			CreatedAt: doc.CreatedAt.Unix(),
		}
	}
	return posts, nil
}
