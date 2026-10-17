//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"testing"
	"time"

	"go_distributed_social_media/internal/follow_service"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowLifecycleIntegration(t *testing.T) {
	alice := registerFollowUser(t)
	bob := registerFollowUser(t)
	carol := registerFollowUser(t)

	response := requestJSON(t, alice.client, http.MethodPost, followPath(bob.id), nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var action followActionResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&action))
	response.Body.Close()
	assert.True(t, action.Changed)

	response = requestJSON(t, alice.client, http.MethodPost, followPath(bob.id), nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&action))
	response.Body.Close()
	assert.False(t, action.Changed)

	response = requestJSON(t, alice.client, http.MethodPost, followPath(carol.id), nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	response = requestJSON(t, alice.client, http.MethodGet, "/users/"+url.PathEscape(alice.id)+"/follows", nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var follows followListResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&follows))
	response.Body.Close()
	assert.ElementsMatch(t, []string{bob.id, carol.id}, follows.UserIDs)

	response = requestJSON(t, bob.client, http.MethodGet, "/users/"+url.PathEscape(bob.id)+"/followers", nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var followers followListResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&followers))
	response.Body.Close()
	assert.Equal(t, []string{alice.id}, followers.UserIDs)

	response = requestJSON(t, alice.client, http.MethodPost, followPath(alice.id), nil)
	require.Equal(t, http.StatusBadRequest, response.StatusCode)
	response.Body.Close()

	response = requestJSON(t, alice.client, http.MethodDelete, followPath(bob.id), nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&action))
	response.Body.Close()
	assert.True(t, action.Changed)

	response = requestJSON(t, alice.client, http.MethodGet, "/users/"+url.PathEscape(alice.id)+"/follows", nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&follows))
	response.Body.Close()
	assert.Equal(t, []string{carol.id}, follows.UserIDs)

	response = requestJSON(t, bob.client, http.MethodGet, "/users/"+url.PathEscape(bob.id)+"/followers", nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&followers))
	response.Body.Close()
	assert.Empty(t, followers.UserIDs)
}

type followIntegrationUser struct {
	client *http.Client
	id     string
}

type followActionResponse struct {
	Changed bool `json:"changed"`
}

type followListResponse struct {
	UserIDs []string `json:"userIds"`
}

func registerFollowUser(t *testing.T) followIntegrationUser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}

	email := fmt.Sprintf("follow-integration-%d@example.com", time.Now().UnixNano())
	password := "SecurePass123!"
	response := requestJSON(t, client, http.MethodPost, "/auth/register", map[string]string{
		"name":                  "follow_integration",
		"email":                 email,
		"password":              password,
		"password_confirmation": password,
	})
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	response = requestJSON(t, client, http.MethodPost, "/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	request, err := http.NewRequest(http.MethodGet, gatewayURL+"/profile", nil)
	require.NoError(t, err)
	response, err = client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var profile struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&profile))
	response.Body.Close()
	require.NotEmpty(t, profile.ID)

	return followIntegrationUser{client: client, id: profile.ID}
}

func followPath(userID string) string {
	return "/users/" + url.PathEscape(userID) + "/follow"
}

func TestFollowEndpointsRequireAuthentication(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	request, err := http.NewRequest(
		http.MethodGet,
		gatewayURL+"/users/"+url.PathEscape("someone")+"/followers",
		nil,
	)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	response.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)

	response, err = client.Get(gatewayURL + "/internal/follows/is-celebrity")
	require.NoError(t, err)
	response.Body.Close()
	assert.Equal(t, http.StatusNotFound, response.StatusCode)
}

func TestCelebrityThresholdIntegration(t *testing.T) {
	host := envOrDefault("FOLLOW_DB_HOST", "127.0.0.1")
	port := envOrDefault("FOLLOW_DB_PORT", "7687")
	user := os.Getenv("FOLLOW_DB_USER")
	password := os.Getenv("FOLLOW_DB_PASS")
	require.NotEmpty(t, user)
	require.NotEmpty(t, password)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	repo, err := follow_service.NewNeo4jRepo(ctx, host, port, user, password)
	require.NoError(t, err)
	service := follow_service.NewService(repo)
	driver, err := neo4j.NewDriverWithContext(
		"bolt://"+net.JoinHostPort(host, port),
		neo4j.BasicAuth(user, password, ""),
	)
	require.NoError(t, err)

	prefix := fmt.Sprintf("celebrity-threshold-%d-", time.Now().UnixNano())
	targetUnder := prefix + "under"
	targetAt := prefix + "at"
	viewer := prefix + "viewer"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		cleanupSession := driver.NewSession(cleanupCtx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		_, cleanupErr := cleanupSession.ExecuteWrite(cleanupCtx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(cleanupCtx,
				"MATCH (user:FollowUser) WHERE user.id STARTS WITH $prefix DETACH DELETE user",
				map[string]any{"prefix": prefix},
			)
			if err != nil {
				return nil, err
			}
			return result.Consume(cleanupCtx)
		})
		if cleanupErr != nil {
			t.Errorf("clean up celebrity test graph: %v", cleanupErr)
		}
		if err := cleanupSession.Close(cleanupCtx); err != nil {
			t.Errorf("close celebrity cleanup session: %v", err)
		}
		if err := driver.Close(cleanupCtx); err != nil {
			t.Errorf("close celebrity test driver: %v", err)
		}
		if err := repo.Close(cleanupCtx); err != nil {
			t.Errorf("close celebrity repository: %v", err)
		}
	})

	require.NoError(t, createTestFollowers(ctx, driver, targetUnder, prefix+"under-follower-", follow_service.CelebrityFollowerThreshold-2))
	require.NoError(t, createTestFollowers(ctx, driver, targetAt, prefix+"at-follower-", follow_service.CelebrityFollowerThreshold-1))
	_, err = repo.Follow(ctx, viewer, targetUnder)
	require.NoError(t, err)
	_, err = repo.Follow(ctx, viewer, targetAt)
	require.NoError(t, err)

	isCelebrity, err := service.IsCelebrity(ctx, targetUnder)
	require.NoError(t, err)
	assert.False(t, isCelebrity)

	isCelebrity, err = service.IsCelebrity(ctx, targetAt)
	require.NoError(t, err)
	assert.True(t, isCelebrity)

	celebrities, err := repo.GetCelebrityFollows(ctx, viewer)
	require.NoError(t, err)
	assert.Equal(t, []string{targetAt}, celebrities)

	nonCelebrities, err := service.GetNonCelebrityFollows(ctx, viewer)
	require.NoError(t, err)
	assert.Equal(t, []string{targetUnder}, nonCelebrities)
}

func createTestFollowers(ctx context.Context, driver neo4j.DriverWithContext, targetID, followerPrefix string, count int) error {
	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MERGE (target:FollowUser {id: $targetID}) "+
				"WITH target UNWIND range(1, $count) AS number "+
				"MERGE (follower:FollowUser {id: $followerPrefix + toString(number)}) "+
				"MERGE (follower)-[:FOLLOWS]->(target)",
			map[string]any{
				"targetID":       targetID,
				"followerPrefix": followerPrefix,
				"count":          count,
			},
		)
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})
	closeErr := session.Close(ctx)
	if err != nil {
		return err
	}
	return closeErr
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
