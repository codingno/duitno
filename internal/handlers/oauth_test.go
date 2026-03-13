package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"duitno/internal/database"
	"duitno/internal/oauth"
)

func TestCallback_MissingCodeOrState(t *testing.T) {
	app := fiber.New()
	h := &OAuthHandler{
		Configs:    make(oauth.OAuthConfig),
		DB:         &database.DB{},
		StateStore: make(map[string]bool),
	}

	app.Get("/auth/:provider/callback", h.Callback)

	t.Run("MissingCode", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/google/callback?state=xyz", nil)
		resp, _ := app.Test(req)

		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("MissingState", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/google/callback?code=abc", nil)
		resp, _ := app.Test(req)

		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("InvalidState", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/google/callback?code=abc&state=wrong", nil)
		resp, _ := app.Test(req)

		assert.Equal(t, 400, resp.StatusCode)
	})
}
