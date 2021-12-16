package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	stateSize = 16
	nonceSize = 16
)

type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	ServerURL    string // optional
}

func NewOIDC(ctx context.Context, cfg Config, next http.Handler) (http.Handler, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("get OIDC provider: %w", err)
	}
	var claims struct {
		EndSessionURL string `json:"end_session_endpoint"`
	}
	_ = provider.Claims(&claims)

	return &oauthMiddleware{
		oauthConfig: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID},
		},
		verifier: provider.Verifier(&oidc.Config{
			ClientID: cfg.ClientID,
		}),
		serverURL: cfg.ServerURL,
		logoutURL: claims.EndSessionURL,
		next:      next,
	}, nil
}

type oauthMiddleware struct {
	oauthConfig oauth2.Config
	verifier    *oidc.IDTokenVerifier
	serverURL   string
	logoutURL   string
	next        http.Handler
}

func (svc *oauthMiddleware) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if strings.HasSuffix(request.URL.Path, "/oauth2/callback") {
		log.Println("handle callback")
		svc.handlerCallback(writer, request)

		return
	}
	cookie, err := request.Cookie("token")
	if err != nil {
		log.Println("unauthorized - no cookie")
		svc.unauthorizedRequest(writer, request)

		return
	}

	idToken, err := svc.verifier.Verify(request.Context(), cookie.Value)
	if err != nil {
		log.Println("unauthorized - invalid token")
		svc.unauthorizedRequest(writer, request)

		return
	}
	if strings.HasSuffix(request.URL.Path, "/logout") {
		log.Println("handle logout")
		svc.logout(writer, request, cookie.Value)

		return
	}
	svc.next.ServeHTTP(writer, request.WithContext(WithUser(request.Context(), User{Name: getUserName(idToken)})))
}

func (svc *oauthMiddleware) unauthorizedRequest(writer http.ResponseWriter, request *http.Request) {
	state, err := randString(stateSize)
	if err != nil {
		http.Error(writer, "Internal error", http.StatusInternalServerError)

		return
	}
	nonce, err := randString(nonceSize)
	if err != nil {
		http.Error(writer, "Internal error", http.StatusInternalServerError)

		return
	}

	setCallbackCookie(writer, request, "state", state)
	setCallbackCookie(writer, request, "nonce", nonce)

	http.Redirect(writer, request, svc.getConfig(request).AuthCodeURL(state, oidc.Nonce(nonce), oauth2.SetAuthURLParam("max_auth_age", "0")), http.StatusFound)
}

func (svc *oauthMiddleware) handlerCallback(writer http.ResponseWriter, request *http.Request) {
	state, err := request.Cookie("state")
	if err != nil {
		http.Error(writer, "state not found", http.StatusBadRequest)

		return
	}
	if request.URL.Query().Get("state") != state.Value {
		http.Error(writer, "state did not match", http.StatusBadRequest)

		return
	}
	oauth2Token, err := svc.getConfig(request).Exchange(request.Context(), request.URL.Query().Get("code"))
	if err != nil {
		http.Error(writer, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)

		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(writer, "No id_token field in oauth2 token.", http.StatusInternalServerError)

		return
	}
	idToken, err := svc.verifier.Verify(request.Context(), rawIDToken)
	if err != nil {
		http.Error(writer, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)

		return
	}

	nonce, err := request.Cookie("nonce")
	if err != nil {
		http.Error(writer, "nonce not found", http.StatusBadRequest)

		return
	}
	if idToken.Nonce != nonce.Value {
		http.Error(writer, "nonce did not match", http.StatusBadRequest)

		return
	}
	// store rawIDToken in cookie to re-use it later
	http.SetCookie(writer, &http.Cookie{
		Name:     "token",
		Path:     "/",
		Value:    rawIDToken,
		Expires:  idToken.Expiry,
		Secure:   isSecure(request),
		HttpOnly: true,
	})
	http.Redirect(writer, request, "/", http.StatusFound)
}

func (svc *oauthMiddleware) logout(writer http.ResponseWriter, request *http.Request, hint string) {
	cookie := &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		Secure:   isSecure(request),
		HttpOnly: true,
		MaxAge:   -1,
	}
	http.SetCookie(writer, cookie)
	svc.requestEndSession(request.Context(), hint)
	http.Redirect(writer, request, "/", http.StatusFound)
}

func (svc *oauthMiddleware) getServerURL(req *http.Request) string {
	if svc.serverURL != "" {
		return svc.serverURL
	}
	host := req.Host
	proto := "http"
	if v := req.Header.Get("X-Forwarded-Host"); v != "" {
		host = v
	}
	if isSecure(req) {
		proto = "https"
	}

	return proto + "://" + host
}

func (svc *oauthMiddleware) getConfig(req *http.Request) *oauth2.Config {
	cp := svc.oauthConfig
	cp.RedirectURL = svc.getServerURL(req) + "/oauth2/callback"

	return &cp
}

func (svc *oauthMiddleware) requestEndSession(ctx context.Context, rawToken string) {
	if svc.logoutURL == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.logoutURL+"?id_token_hint="+rawToken, nil)
	if err != nil {
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
}

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(writer http.ResponseWriter, request *http.Request, name, value string) {
	cookie := &http.Cookie{
		Name:     name,
		Path:     "/",
		Value:    value,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   isSecure(request),
		HttpOnly: true,
	}
	http.SetCookie(writer, cookie)
}

func isSecure(req *http.Request) bool {
	if v := req.Header.Get("X-Forwarded-Proto"); v != "" {
		return v == "https"
	}

	return req.TLS != nil
}

func getUserName(token *oidc.IDToken) string {
	var claims struct {
		Username string `json:"preferred_username"`
		Email    string `json:"email"`
	}
	_ = token.Claims(&claims)
	if claims.Username != "" {
		return claims.Username
	}
	if claims.Email != "" {
		return claims.Email
	}

	return token.Subject
}
