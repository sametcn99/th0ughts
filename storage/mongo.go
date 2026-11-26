package storage

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoStore struct {
	client *mongo.Client
	c      *mongo.Collection
}

func openMongo(ctx context.Context, c Config) (Store, error) {
	u, e := url.Parse(c.Address)
	if e != nil {
		return nil, e
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = "th0ughts"
	}
	client, e := mongo.Connect(options.Client().ApplyURI(c.Address).SetServerSelectionTimeout(8 * time.Second))
	if e != nil {
		return nil, e
	}
	s := &mongoStore{client, client.Database(db).Collection("records")}
	_, e = s.c.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)}, {Keys: bson.D{{Key: "kind", Value: 1}, {Key: "published", Value: 1}, {Key: "rank", Value: -1}, {Key: "id", Value: -1}}}, {Keys: bson.D{{Key: "kind", Value: 1}, {Key: "rank", Value: -1}, {Key: "id", Value: -1}}}})
	if e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}
func (s *mongoStore) Get(ctx context.Context, k, id string) (Record, error) {
	var r Record
	e := s.c.FindOne(ctx, bson.M{"kind": k, "id": id}).Decode(&r)
	if errors.Is(e, mongo.ErrNoDocuments) {
		e = ErrMissing
	}
	return r, e
}
func (s *mongoStore) Put(ctx context.Context, r Record) error {
	_, e := s.c.ReplaceOne(ctx, bson.M{"kind": r.Kind, "id": r.ID}, r, options.Replace().SetUpsert(true))
	return e
}
func (s *mongoStore) Delete(ctx context.Context, k, id string) error {
	_, e := s.c.DeleteOne(ctx, bson.M{"kind": k, "id": id})
	return e
}
func (s *mongoStore) List(ctx context.Context, q Query) ([]Record, error) {
	f := bson.M{"kind": q.Kind}
	if q.Prefix != "" {
		f["id"] = bson.M{"$gte": q.Prefix, "$lt": q.Prefix + "~"}
	}
	if q.Unpublished {
		f["published"] = false
	}
	if q.Public {
		f["published"] = true
	}
	if q.Before != nil {
		f["$or"] = bson.A{bson.M{"rank": bson.M{"$lt": q.Before.Rank}}, bson.M{"rank": q.Before.Rank, "id": bson.M{"$lt": q.Before.ID}}}
	}
	opts := options.Find().SetSort(bson.D{{Key: "rank", Value: -1}, {Key: "id", Value: -1}})
	if q.Limit > 0 {
		opts.SetLimit(int64(q.Limit))
	}
	cur, e := s.c.Find(ctx, f, opts)
	if e != nil {
		return nil, e
	}
	defer cur.Close(ctx)
	out := []Record{}
	e = cur.All(ctx, &out)
	return out, e
}
func (s *mongoStore) Close() error { return s.client.Disconnect(context.Background()) }
