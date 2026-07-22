package combos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoComboDocument struct {
	ID          string    `bson:"_id"`
	Name        string    `bson:"name"`
	Description string    `bson:"description,omitempty"`
	Models      []string  `bson:"models"`
	Enabled     bool      `bson:"enabled"`
	Source      string    `bson:"source"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

// MongoDBStore stores admin-created combos in MongoDB.
type MongoDBStore struct {
	collection *mongo.Collection
}

// NewMongoDBStore creates collection indexes if needed.
func NewMongoDBStore(database *mongo.Database) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	coll := database.Collection("combos")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "enabled", Value: 1}}},
		{Keys: bson.D{{Key: "updated_at", Value: -1}}},
	}
	if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create combos indexes: %w", err)
	}
	return &MongoDBStore{collection: coll}, nil
}

func (s *MongoDBStore) List(ctx context.Context) ([]Combo, error) {
	cursor, err := s.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	defer cursor.Close(ctx)

	out := make([]Combo, 0)
	for cursor.Next(ctx) {
		var doc mongoComboDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode combo: %w", err)
		}
		out = append(out, comboFromMongo(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate combos: %w", err)
	}
	return out, nil
}

func (s *MongoDBStore) Get(ctx context.Context, idOrName string) (*Combo, error) {
	var doc mongoComboDocument
	err := s.collection.FindOne(ctx, comboIDOrNameFilter(idOrName)).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get combo: %w", err)
	}
	combo := comboFromMongo(doc)
	return &combo, nil
}

func (s *MongoDBStore) Upsert(ctx context.Context, combo Combo) error {
	combo = prepareStoredCombo(combo)
	update := bson.M{
		"$set": bson.M{
			"name":        combo.Name,
			"description": combo.Description,
			"models":      combo.Models,
			"enabled":     combo.Enabled,
			"source":      combo.Source,
			"updated_at":  combo.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"created_at": combo.CreatedAt,
		},
	}
	_, err := s.collection.UpdateOne(ctx, bson.M{"_id": combo.ID}, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert combo: %w", err)
	}
	return nil
}

func (s *MongoDBStore) Delete(ctx context.Context, idOrName string) error {
	result, err := s.collection.DeleteOne(ctx, comboIDOrNameFilter(idOrName))
	if err != nil {
		return fmt.Errorf("delete combo: %w", err)
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MongoDBStore) Close() error { return nil }

func comboIDOrNameFilter(idOrName string) bson.M {
	key := strings.TrimSpace(idOrName)
	return bson.M{"$or": []bson.M{{"_id": key}, {"name": key}}}
}

func comboFromMongo(doc mongoComboDocument) Combo {
	return Combo{
		ID:          doc.ID,
		Name:        doc.Name,
		Description: doc.Description,
		Models:      append([]string(nil), doc.Models...),
		Enabled:     doc.Enabled,
		Source:      doc.Source,
		CreatedAt:   doc.CreatedAt.UTC(),
		UpdatedAt:   doc.UpdatedAt.UTC(),
	}
}
