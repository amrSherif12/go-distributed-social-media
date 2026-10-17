//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/gateway"
	"go_distributed_social_media/internal/post_service"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestMain(m *testing.M) {
	port := config.Get(config.GatewayPort)
	if port == "" {
		port = "8081"
	}
	gatewayURL = "http://127.0.0.1:" + port
	os.Exit(m.Run())
}

func TestPostLifecycleIntegration(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}

	email := fmt.Sprintf("post-integration-%d@example.com", time.Now().UnixNano())
	credentials := map[string]string{
		"name":                  "post_integration",
		"email":                 email,
		"password":              "SecurePass123!",
		"password_confirmation": "SecurePass123!",
	}
	response := requestJSON(t, client, http.MethodPost, "/auth/register", credentials)
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	response = requestJSON(t, client, http.MethodPost, "/auth/login", map[string]string{
		"email":    email,
		"password": credentials["password"],
	})
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	response = requestJSON(t, client, http.MethodPost, "/posts", map[string]string{
		"content": "integration post",
	})
	require.Equal(t, http.StatusOK, response.StatusCode)
	var created postResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&created))
	response.Body.Close()
	require.NotEmpty(t, created.PostID)
	assert.Greater(t, parseTimestamp(t, created.CreatedAt), int64(0))

	request, err := http.NewRequest(http.MethodGet, gatewayURL+"/posts/"+url.PathEscape(created.PostID), nil)
	require.NoError(t, err)
	response, err = client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()

	var post postResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&post))
	assert.Equal(t, created.PostID, post.PostID)
	assert.Equal(t, "integration post", post.Content)
	assert.NotEmpty(t, post.UserID)
	assert.Greater(t, parseTimestamp(t, post.CreatedAt), int64(0))
}

func TestGetPostsByUsersIntegration(t *testing.T) {
	port := config.Get(config.PostSrvPort)
	if port == "" {
		port = "8082"
	}
	conn, err := gateway.Dial("127.0.0.1", port)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := pb.NewPostServiceClient(conn)

	userID := fmt.Sprintf("feed-posts-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "uid", userID)
	postIDs := make([]string, 0, 12)
	var postCreatedAt int64
	for i := range 12 {
		created, err := client.CreatePost(ctx, &pb.CreatePostRequest{Content: fmt.Sprintf("feed post %02d", i)})
		require.NoError(t, err)
		postIDs = append(postIDs, created.GetPostId())
		postCreatedAt = created.GetCreatedAt()
	}

	response, err := client.GetPostsByUsers(ctx, &pb.GetPostsByUsersRequest{
		UserIds:         []string{userID},
		BeforeCreatedAt: time.Now().Add(time.Minute).Unix(),
	})
	require.NoError(t, err)
	require.Len(t, response.GetPosts(), post_service.MaxPostsByUsers)

	expectedPostIDs := postIDs[len(postIDs)-post_service.MaxPostsByUsers:]
	for i, post := range response.GetPosts() {
		assert.Equal(t, expectedPostIDs[len(expectedPostIDs)-1-i], post.GetPostId())
		assert.Equal(t, userID, post.GetUserId())
		if i > 0 {
			assert.LessOrEqual(t, post.GetCreatedAt(), response.GetPosts()[i-1].GetCreatedAt())
		}
	}

	response, err = client.GetPostsByUsers(ctx, &pb.GetPostsByUsersRequest{
		UserIds:         []string{userID},
		BeforeCreatedAt: postCreatedAt,
		BeforePostId:    postIDs[len(postIDs)-1],
	})
	require.NoError(t, err)
	require.Len(t, response.GetPosts(), post_service.MaxPostsByUsers)
	for i, post := range response.GetPosts() {
		assert.Equal(t, postIDs[len(postIDs)-2-i], post.GetPostId())
	}

	response, err = client.GetPostsByUsers(ctx, &pb.GetPostsByUsersRequest{
		UserIds:         []string{userID},
		BeforeCreatedAt: time.Now().Add(-time.Hour).Unix(),
	})
	require.NoError(t, err)
	assert.Empty(t, response.GetPosts())
}

type postResponse struct {
	PostID    string          `json:"postId"`
	UserID    string          `json:"userId"`
	Content   string          `json:"content"`
	CreatedAt json.RawMessage `json:"createdAt"`
}

func requestJSON(t *testing.T, client *http.Client, method, path string, payload any) *http.Response {
	t.Helper()
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		require.NoError(t, err)
	}
	request, err := http.NewRequest(method, gatewayURL+path, bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	require.NoError(t, err)
	return response
}

func parseTimestamp(t *testing.T, raw json.RawMessage) int64 {
	t.Helper()
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		timestamp, err := strconv.ParseInt(value, 10, 64)
		require.NoError(t, err)
		return timestamp
	}
	var timestamp int64
	require.NoError(t, json.Unmarshal(raw, &timestamp))
	return timestamp
}
