package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var githubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

var twitchEndpoint = oauth2.Endpoint{
	AuthURL:  "https://id.twitch.tv/oauth2/authorize",
	TokenURL: "https://id.twitch.tv/oauth2/token",
}

type Provider string

const (
	Google  Provider = "google"
	GitHub  Provider = "github"
	Twitch  Provider = "twitch"
	YouTube Provider = "youtube"
)

type UserInfo struct {
	ID    string
	Email string
	Name  string
}

type OAuthConfig map[Provider]*oauth2.Config

func LoadOAuthConfigs() OAuthConfig {
	configs := OAuthConfig{}

	if clientID := os.Getenv("GOOGLE_CLIENT_ID"); clientID != "" {
		configs[Google] = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     google.Endpoint,
		}
	}

	if clientID := os.Getenv("GITHUB_CLIENT_ID"); clientID != "" {
		configs[GitHub] = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     githubEndpoint,
		}
	}

	if clientID := os.Getenv("TWITCH_CLIENT_ID"); clientID != "" {
		configs[Twitch] = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: os.Getenv("TWITCH_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("TWITCH_REDIRECT_URL"),
			Scopes:       []string{"user:read:email", "user:read:subscriptions"},
			Endpoint:     twitchEndpoint,
		}
	}

	return configs
}

func GetAuthURL(provider Provider, configs OAuthConfig, state string) (string, error) {
	config, ok := configs[provider]
	if !ok {
		return "", fmt.Errorf("provider %s not configured", provider)
	}
	return config.AuthCodeURL(state), nil
}

func ExchangeCode(provider Provider, configs OAuthConfig, code string) (*oauth2.Token, error) {
	config, ok := configs[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	return config.Exchange(context.Background(), code)
}

func GetUserInfo(provider Provider, token *oauth2.Token, configs OAuthConfig) (*UserInfo, error) {
	switch provider {
	case Google:
		return getGoogleUserInfo(token, configs)
	case GitHub:
		return getGitHubUserInfo(token, configs)
	case Twitch:
		return getTwitchUserInfo(token, configs)
	default:
		return nil, fmt.Errorf("provider %s user info not implemented", provider)
	}
}

func getGoogleUserInfo(token *oauth2.Token, configs OAuthConfig) (*UserInfo, error) {
	config := configs[Google]
	client := config.Client(context.Background(), token)

	type GoogleUserResponse struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	var userResp GoogleUserResponse
	err := fetchJSON(client, "https://www.googleapis.com/oauth2/v2/userinfo", &userResp)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		ID:    userResp.ID,
		Email: userResp.Email,
		Name:  userResp.Name,
	}, nil
}

func fetchJSON(client *http.Client, url string, target interface{}) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func getGitHubUserInfo(token *oauth2.Token, configs OAuthConfig) (*UserInfo, error) {
	config := configs[GitHub]
	client := config.Client(context.Background(), token)

	type GitHubUserResponse struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Login string `json:"login"`
	}

	var userResp GitHubUserResponse
	err := fetchJSON(client, "https://api.github.com/user", &userResp)
	if err != nil {
		return nil, err
	}

	email := userResp.Email
	if email == "" {
		email, _ = getGitHubPrimaryEmail(client)
	}

	name := userResp.Name
	if name == "" {
		name = userResp.Login
	}

	return &UserInfo{
		ID:    strconv.Itoa(userResp.ID),
		Email: email,
		Name:  name,
	}, nil
}

func getGitHubPrimaryEmail(client *http.Client) (string, error) {
	type GitHubEmail struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []GitHubEmail
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no primary verified email found")
}

func getTwitchUserInfo(token *oauth2.Token, configs OAuthConfig) (*UserInfo, error) {
	config := configs[Twitch]
	client := config.Client(context.Background(), token)

	type TwitchUserResponse struct {
		Data []struct {
			ID    string `json:"id"`
			Login string `json:"login"`
			Name  string `json:"display_name"`
			Email string `json:"email"`
		} `json:"data"`
	}

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Client-ID", config.ClientID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userResp TwitchUserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, err
	}

	if len(userResp.Data) == 0 {
		return nil, fmt.Errorf("no user data returned from Twitch")
	}

	user := userResp.Data[0]
	return &UserInfo{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}, nil
}
