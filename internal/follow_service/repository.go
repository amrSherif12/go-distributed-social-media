package follow_service

import (
	"context"
	"fmt"
	"net"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Repository interface {
	Follow(ctx context.Context, followerID, followeeID string) (bool, error)
	Unfollow(ctx context.Context, followerID, followeeID string) (bool, error)
	GetFollows(ctx context.Context, userID string) ([]string, error)
	GetFollowers(ctx context.Context, userID string) ([]string, error)
	GetCelebrityFollows(ctx context.Context, userID string) ([]string, error)
}

type Neo4jRepo struct {
	driver neo4j.DriverWithContext
}

func NewNeo4jRepo(ctx context.Context, host, port, user, password string) (*Neo4jRepo, error) {
	driver, err := neo4j.NewDriverWithContext(
		"bolt://"+net.JoinHostPort(host, port),
		neo4j.BasicAuth(user, password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("create Neo4j driver: %w", err)
	}

	repo := &Neo4jRepo{driver: driver}
	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(context.Background())
		return nil, fmt.Errorf("connect to Neo4j: %w", err)
	}
	if err := repo.ensureSchema(ctx); err != nil {
		_ = driver.Close(context.Background())
		return nil, err
	}
	return repo, nil
}

func (r *Neo4jRepo) Close(ctx context.Context) error {
	return r.driver.Close(ctx)
}

func (r *Neo4jRepo) ensureSchema(ctx context.Context) error {
	_, err := r.withSession(ctx, neo4j.AccessModeWrite, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"CREATE CONSTRAINT follow_user_id_unique IF NOT EXISTS FOR (user:FollowUser) REQUIRE user.id IS UNIQUE",
			nil,
		)
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})
	if err != nil {
		return fmt.Errorf("ensure follow graph schema: %w", err)
	}
	return nil
}

func (r *Neo4jRepo) Follow(ctx context.Context, followerID, followeeID string) (bool, error) {
	result, err := r.withSession(ctx, neo4j.AccessModeWrite, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MERGE (follower:FollowUser {id: $followerID}) "+
				"MERGE (followee:FollowUser {id: $followeeID}) "+
				"MERGE (follower)-[:FOLLOWS]->(followee)",
			map[string]any{"followerID": followerID, "followeeID": followeeID},
		)
		if err != nil {
			return nil, err
		}
		summary, err := result.Consume(ctx)
		if err != nil {
			return nil, err
		}
		return summary.Counters().RelationshipsCreated() > 0, nil
	})
	if err != nil {
		return false, fmt.Errorf("create follow relationship: %w", err)
	}
	changed, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected result creating follow relationship")
	}
	return changed, nil
}

func (r *Neo4jRepo) Unfollow(ctx context.Context, followerID, followeeID string) (bool, error) {
	result, err := r.withSession(ctx, neo4j.AccessModeWrite, func(tx neo4j.ManagedTransaction) (any, error) {
		query, err := tx.Run(ctx,
			"MATCH (follower:FollowUser {id: $followerID})-[relationship:FOLLOWS]->"+
				"(followee:FollowUser {id: $followeeID}) "+
				"DELETE relationship RETURN count(relationship) > 0 AS changed",
			map[string]any{"followerID": followerID, "followeeID": followeeID},
		)
		if err != nil {
			return nil, err
		}
		record, err := query.Single(ctx)
		if err != nil {
			return nil, err
		}
		changed, ok := record.Get("changed")
		if !ok {
			return nil, fmt.Errorf("Neo4j did not return relationship change status")
		}
		return changed, nil
	})
	if err != nil {
		return false, fmt.Errorf("delete follow relationship: %w", err)
	}
	changed, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected result deleting follow relationship")
	}
	return changed, nil
}

func (r *Neo4jRepo) GetFollows(ctx context.Context, userID string) ([]string, error) {
	return r.getIDs(ctx,
		"MATCH (:FollowUser {id: $userID})-[:FOLLOWS]->(followee:FollowUser) "+
			"RETURN followee.id AS id ORDER BY id",
		map[string]any{"userID": userID},
	)
}

func (r *Neo4jRepo) GetFollowers(ctx context.Context, userID string) ([]string, error) {
	return r.getIDs(ctx,
		"MATCH (follower:FollowUser)-[:FOLLOWS]->(:FollowUser {id: $userID}) "+
			"RETURN follower.id AS id ORDER BY id",
		map[string]any{"userID": userID},
	)
}

func (r *Neo4jRepo) GetCelebrityFollows(ctx context.Context, userID string) ([]string, error) {
	return r.getIDs(ctx,
		"MATCH (:FollowUser {id: $userID})-[:FOLLOWS]->(followee:FollowUser) "+
			"OPTIONAL MATCH (follower:FollowUser)-[:FOLLOWS]->(followee) "+
			"WITH followee, count(follower) AS followerCount "+
			"WHERE followerCount >= $threshold "+
			"RETURN followee.id AS id ORDER BY id",
		map[string]any{"userID": userID, "threshold": CelebrityFollowerThreshold},
	)
}

func (r *Neo4jRepo) getIDs(ctx context.Context, query string, params map[string]any) ([]string, error) {
	value, err := r.withSession(ctx, neo4j.AccessModeRead, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0)
		for result.Next(ctx) {
			value, ok := result.Record().Get("id")
			if !ok {
				return nil, fmt.Errorf("Neo4j did not return an id")
			}
			id, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("Neo4j returned an invalid id")
			}
			ids = append(ids, id)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return ids, nil
	})
	if err != nil {
		return nil, fmt.Errorf("query follow graph: %w", err)
	}
	ids, ok := value.([]string)
	if !ok {
		return nil, fmt.Errorf("unexpected result querying follow graph")
	}
	return ids, nil
}

func (r *Neo4jRepo) withSession(
	ctx context.Context,
	mode neo4j.AccessMode,
	work neo4j.ManagedTransactionWork,
) (any, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: mode})
	var result any
	var err error
	if mode == neo4j.AccessModeWrite {
		result, err = session.ExecuteWrite(ctx, work)
	} else {
		result, err = session.ExecuteRead(ctx, work)
	}
	closeErr := session.Close(ctx)
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close Neo4j session: %w", closeErr)
	}
	return result, nil
}
