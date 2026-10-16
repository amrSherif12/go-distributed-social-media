//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"go_distributed_social_media/internal/user_service"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var gatewayURL string

func TestAuthLifecycleIntegration(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Jar:     jar,
	}

	testEmail := "integration_user@example.com"
	testPassword := "SecurePass123!"
	testUsername := "integration_hero"

	t.Run("Register New User", func(t *testing.T) {
		payload, _ := json.Marshal(user_service.RegisterDTO{
			Name:     testUsername,
			Email:    testEmail,
			Password: testPassword,
			PassConf: testPassword,
		})

		resp, err := client.Post(gatewayURL+"/auth/register", "application/json", bytes.NewBuffer(payload))
		require.NoError(t, err, "Gateway must be reachable")

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Register User with a used Email", func(t *testing.T) {
		payload, _ := json.Marshal(user_service.RegisterDTO{
			Name:     testUsername,
			Email:    testEmail,
			Password: testPassword,
			PassConf: testPassword,
		})

		resp, err := client.Post(gatewayURL+"/auth/register", "application/json", bytes.NewBuffer(payload))
		require.NoError(t, err, "Gateway must be reachable")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("Login and Verify Auth Cookie", func(t *testing.T) {
		payload, _ := json.Marshal(user_service.LoginDTO{
			Email:    testEmail,
			Password: testPassword,
		})

		resp, err := client.Post(gatewayURL+"/auth/login", "application/json", bytes.NewBuffer(payload))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var foundAuthCookie bool
		for _, cookie := range resp.Cookies() {
			if cookie.Name == "auth" && cookie.Value != "" {
				foundAuthCookie = true
			}
		}
		assert.True(t, foundAuthCookie, "Response must set 'auth' cookie")
	})

	t.Run("Fetch Profile with Cookie", func(t *testing.T) {
		resp, err := client.Get(gatewayURL + "/profile")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		bodyBytes, _ := io.ReadAll(resp.Body)
		var profile user_service.User
		err = json.Unmarshal(bodyBytes, &profile)
		require.NoError(t, err)

		assert.Equal(t, testUsername, profile.Name)
		assert.Equal(t, testEmail, profile.Email)
		assert.NotEmpty(t, profile.ID)
	})

	t.Run("Reject Unauthenticated Profile Request", func(t *testing.T) {
		unauthClient := &http.Client{Timeout: 5 * time.Second}

		resp, err := unauthClient.Get(gatewayURL + "/profile")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
