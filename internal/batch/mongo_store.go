package batch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoDoc struct {
	ID        string `bson:"_id"`
	CreatedAt int64  `bson:"created_at"`
	UpdatedAt int64  `bson:"updated_at"`
	Status    string `bson:"status"`
	Data      []byte `bson:"data"`
}

type MongoDBStore struct {
	coll *mongo.Collection
}

func NewMongoDBStore(database *mongo.Database) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	coll := database.Collection("batches")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	}
	if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create batches indexes: %w", err)
	}
	return &MongoDBStore{coll: coll}, nil
}

func (s *MongoDBStore) Create(ctx context.Context, batch *StoredBatch) error {
	payload, err := encodeBatch(batch)
	if err != nil {
		return err
	}
	doc := mongoDoc{
		ID:        batch.Batch.ID,
		CreatedAt: batch.Batch.CreatedAt,
		UpdatedAt: time.Now().Unix(),
		Status:    batch.Batch.Status,
		Data:      payload,
	}
	if _, err := s.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert batch: %w", err)
	}
	return nil
}

func (s *MongoDBStore) Get(ctx context.Context, id string) (*StoredBatch, error) {
	var doc mongoDoc
	err := s.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query batch: %w", err)
	}
	batch, err := decodeBatch(doc.Data)
	if err != nil {
		return nil, fmt.Errorf("decode batch: %w", err)
	}
	return batch, nil
}

func (s *MongoDBStore) List(ctx context.Context, limit int, after string) ([]*StoredBatch, error) {
	limit = clampPageSize(limit)
	filter := bson.M{}
	if after != "" {
		var cursor mongoDoc
		err := s.coll.FindOne(ctx, bson.M{"_id": after}).Decode(&cursor)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("query after cursor: %w", err)
		}
		filter = bson.M{
			"$or": bson.A{
				bson.M{"created_at": bson.M{"$lt": cursor.CreatedAt}},
				bson.M{
					"created_at": cursor.CreatedAt,
					"_id":        bson.M{"$lt": cursor.ID},
				},
			},
		}
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(limit))
	cursor, err := s.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	items := make([]*StoredBatch, 0, limit)
	for cursor.Next(ctx) {
		var doc mongoDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode batch document: %w", err)
		}
		batch, err := decodeBatch(doc.Data)
		if err != nil {
			return nil, fmt.Errorf("decode batch payload: %w", err)
		}
		items = append(items, batch)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate batches cursor: %w", err)
	}
	return items, nil
}

func (s *MongoDBStore) Update(ctx context.Context, batch *StoredBatch) error {
	payload, err := encodeBatch(batch)
	if err != nil {
		return err
	}
	result, err := s.coll.UpdateOne(ctx,
		bson.M{"_id": batch.Batch.ID},
		bson.M{"$set": bson.M{
			"updated_at": time.Now().Unix(),
			"status":     batch.Batch.Status,
			"data":       payload,
		}},
	)
	if err != nil {
		return fmt.Errorf("update batch: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MongoDBStore) Close() error {
	return nil
}
