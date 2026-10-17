//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedBackfillIntegration(t *testing.T) {
	viewer := registerFollowUser(t)
	author := registerFollowUser(t)
	postIDs := make([]string, 0, 12)
	for index := range 12 {
		response := requestJSON(t, author.client, http.MethodPost, "/posts", map[string]string{
			"content": fmt.Sprintf("feed integration post %02d", index),
		})
		require.Equal(t, http.StatusOK, response.StatusCode)
		var created postResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&created))
		response.Body.Close()
		postIDs = append(postIDs, created.PostID)
	}

	response := requestJSON(t, viewer.client, http.MethodPost, followPath(author.id), nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	response.Body.Close()

	response = requestJSON(t, viewer.client, http.MethodGet, "/feed", nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var firstPage struct {
		Posts               []postResponse  `json:"posts"`
		NextBeforeCreatedAt json.RawMessage `json:"nextBeforeCreatedAt"`
		NextBeforePostID    string          `json:"nextBeforePostId"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&firstPage))
	response.Body.Close()
	require.Len(t, firstPage.Posts, 10)
	for index, post := range firstPage.Posts {
		assert.Equal(t, postIDs[len(postIDs)-1-index], post.PostID)
		assert.Equal(t, author.id, post.UserID)
	}
	nextBeforeCreatedAt := parseTimestamp(t, firstPage.NextBeforeCreatedAt)
	require.Greater(t, nextBeforeCreatedAt, int64(0))
	require.NotEmpty(t, firstPage.NextBeforePostID)

	cursor := fmt.Sprintf(
		"/feed?before_created_at=%d&before_post_id=%s",
		nextBeforeCreatedAt,
		url.QueryEscape(firstPage.NextBeforePostID),
	)
	response = requestJSON(t, viewer.client, http.MethodGet, cursor, nil)
	require.Equal(t, http.StatusOK, response.StatusCode)
	defer response.Body.Close()
	var secondPage struct {
		Posts []postResponse `json:"posts"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&secondPage))
	require.Len(t, secondPage.Posts, 2)
	assert.Equal(t, postIDs[1], secondPage.Posts[0].PostID)
	assert.Equal(t, postIDs[0], secondPage.Posts[1].PostID)
}
