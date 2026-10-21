package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"shakedown/internal/config"
)

// Provider wraps go-oidc provider with oauth2 config.
type Provider struct {
	provider           *oidc.Provider
	verifier           *oidc.IDTokenVerifier
	oauthConfig        oauth2.Config
	endSessionEndpoint string
}

// NewProvider creates and configures the OIDC provider.
func NewProvider(ctx context.Context, cfg *config.Config) (*Provider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to create OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})

	oauthConfig := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.AppBaseURL + "/api/auth/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	var discovery struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&discovery); err != nil {
		return nil, fmt.Errorf("auth: failed to read OIDC discovery document: %w", err)
	}

	return &Provider{
		provider:           provider,
		verifier:           verifier,
		oauthConfig:        oauthConfig,
		endSessionEndpoint: discovery.EndSessionEndpoint,
	}, nil
}

// EndSessionURL is the RP-initiated logout URL, empty when the provider has no end_session_endpoint.
func (p *Provider) EndSessionURL() string {
	if p.endSessionEndpoint == "" {
		return ""
	}

	endpoint, err := url.Parse(p.endSessionEndpoint)
	if err != nil {
		return ""
	}

	query := endpoint.Query()
	query.Set("client_id", p.oauthConfig.ClientID)
	endpoint.RawQuery = query.Encode()

	return endpoint.String()
}

// AuthCodeURL generates the authorization URL with state and nonce.
func (p *Provider) AuthCodeURL(state, nonce string) string {
	return p.oauthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oidc.Nonce(nonce),
	)
}

// Exchange exchanges the authorization code for tokens and verifies the ID token.
func (p *Provider) Exchange(ctx context.Context, code, nonce string) (*oidc.IDToken, *oauth2.Token, error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, nil, fmt.Errorf("auth: no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: id_token verification failed: %w", err)
	}

	if idToken.Nonce != nonce {
		return nil, nil, fmt.Errorf("auth: nonce mismatch")
	}

	return idToken, token, nil
}

// GenerateRandomString creates a URL-safe random string of n bytes.
func GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: failed to generate random string: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
