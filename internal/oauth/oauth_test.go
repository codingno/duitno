package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestGetAuthURL(t *testing.T) {
	configs := OAuthConfig{
		Google: &oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "http://localhost/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		},
	}

	t.Run("Success", func(t *testing.T) {
		url, err := GetAuthURL(Google, configs, "state")
		assert.NoError(t, err)
		assert.Contains(t, url, "https://accounts.google.com/o/oauth2/auth")
		assert.Contains(t, url, "client-id")
		assert.Contains(t, url, "state")
	})

	t.Run("ProviderNotConfigured", func(t *testing.T) {
		url, err := GetAuthURL(GitHub, configs, "state")
		assert.Error(t, err)
		assert.Empty(t, url)
		assert.Equal(t, "provider github not configured", err.Error())
	})
}
