package handlers

import (
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"duitno/internal/database"
	"duitno/internal/oauth"
)

type OAuthHandler struct {
	Configs    oauth.OAuthConfig
	DB         *database.DB
	StateStore map[string]bool
	Mu         sync.Mutex
}

func NewOAuthHandler(configs oauth.OAuthConfig, db *database.DB) *OAuthHandler {
	return &OAuthHandler{
		Configs:    configs,
		DB:         db,
		StateStore: make(map[string]bool),
	}
}

func (h *OAuthHandler) GetAuthURL(c *fiber.Ctx) error {
	provider := oauth.Provider(c.Params("provider"))

	state := uuid.New().String()
	h.Mu.Lock()
	h.StateStore[state] = true
	h.Mu.Unlock()

	url, err := oauth.GetAuthURL(provider, h.Configs, state)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

func (h *OAuthHandler) Callback(c *fiber.Ctx) error {
	provider := oauth.Provider(c.Params("provider"))
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "missing code or state",
		})
	}

	h.Mu.Lock()
	_, valid := h.StateStore[state]
	if valid {
		delete(h.StateStore, state)
	}
	h.Mu.Unlock()

	if !valid {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid state",
		})
	}

	token, err := oauth.ExchangeCode(provider, h.Configs, code)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "failed to exchange code: " + err.Error(),
		})
	}

	userInfo, err := oauth.GetUserInfo(provider, token, h.Configs)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "failed to get user info: " + err.Error(),
		})
	}

	userID, err := h.DB.FindOrCreateUser(provider, userInfo, token)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to create user: " + err.Error(),
		})
	}

	userData, err := h.DB.GetUserInfo(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to get user info: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":   "login successful",
		"user_id":   userID,
		"email":     userInfo.Email,
		"name":      userInfo.Name,
		"full_name": userData.FullName.String,
		"provider":  provider,
	})
}
