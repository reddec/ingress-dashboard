package auth

import (
	"net/http"
	"strings"
)

func NewBasic(username, password string, next http.Handler) http.Handler {
	return &basicAuth{
		username: username,
		password: password,
		next:     next,
	}
}

type basicAuth struct {
	username string
	password string
	next     http.Handler
}

func (ba *basicAuth) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if strings.HasSuffix(request.URL.Path, "/logout") {
		writer.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
		writer.Header().Set("Content-Type", "text/html")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("<html><body><a href=\"/\">bye! Log-in again</a></body></html>"))
		return
	}
	u, p, ok := request.BasicAuth()
	if !ok || u != ba.username || p != ba.password {
		writer.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	ba.next.ServeHTTP(writer, request.WithContext(WithUser(request.Context(), User{Name: u})))
}
