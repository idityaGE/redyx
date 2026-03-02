package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUser holds the user information retrieved from Google's userinfo endpoint.
type GoogleUser struct {
	ID    string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// OAuthManager handles Google OAuth code exchange and user info retrieval.
type OAuthManager struct {
	config *oauth2.Config
}

// NewOAuthManager creates an OAuthManager for Google OAuth.
// Returns nil if clientID is empty (OAuth disabled).
func NewOAuthManager(clientID, clientSecret, redirectURL string) *OAuthManager {
	if clientID == "" {
		return nil
	}

	return &OAuthManager{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// Exchange exchanges an authorization code for an OAuth token, then fetches
// the user's profile from Google's userinfo endpoint.
func (m *OAuthManager) Exchange(ctx context.Context, code string) (*GoogleUser, error) {
	token, err := m.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	client := m.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed: %s %s", resp.Status, string(body))
	}

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	if user.ID == "" {
		return nil, fmt.Errorf("google user ID is empty")
	}

	return &user, nil
}
