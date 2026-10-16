//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go_distributed_social_media/internal/common/config"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

type postResponse struct {
	PostID    string          `json:"postId"`
	UserID    string          `json:"userId"`
	Content   string          `json:"content"`
	CreatedAt json.RawMessage `json:"createdAt"`
}

func requestJSON(t *testing.T, client *http.Client, method, path string, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
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
