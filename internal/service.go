package internal

import (
	"html/template"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/reddec/ingress-dashboard/internal/auth"
	"github.com/reddec/ingress-dashboard/internal/static"
)

type Ingress struct {
	ID          string   `yaml:"id"`
	UID         string   `yaml:"uid"`
	Title       string   `yaml:"title"`
	Name        string   `yaml:"name"`
	Namespace   string   `yaml:"namespace"`
	Description string   `yaml:"description"`
	Hide        bool     `yaml:"hide"`
	URLs        []string `yaml:"urls"`
	LogoURL     string   `yaml:"logo_url"`
}

func (ingress Ingress) Label() string {
	if ingress.Title != "" {
		return ingress.Title
	}
	return ingress.Name
}

func (ingress Ingress) Logo() string {
	if ingress.LogoURL == "" {
		return ""
	}
	if strings.HasPrefix(ingress.LogoURL, "/") {
		// relative to domain
		for _, u := range ingress.URLs {
			return strings.TrimRight(u, "/") + ingress.LogoURL
		}
	}
	return ingress.LogoURL

}

type UIContext struct {
	Ingresses []Ingress
	User      *auth.User
}

func New() *Service {
	var router = http.NewServeMux()
	svc := &Service{
		page:   template.Must(template.ParseFS(static.Templates, "assets/templates/*.gotemplate")),
		router: router,
	}
	router.HandleFunc("/", svc.getIndex)
	router.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.FS(static.Static()))))
	return svc
}

type Service struct {
	cache  atomic.Value // []Ingress
	page   *template.Template
	router *http.ServeMux
}

func (svc *Service) Set(ingress []Ingress) {
	svc.cache.Store(ingress)
}

func (svc *Service) Get() []Ingress {
	return svc.cache.Load().([]Ingress)
}

func (svc *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	svc.router.ServeHTTP(writer, request)
}

func (svc *Service) getIndex(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html")
	_ = svc.page.Execute(writer, UIContext{
		Ingresses: visibleIngresses(svc.Get()),
		User:      auth.UserFromContext(request.Context()),
	})
}

func visibleIngresses(list []Ingress) []Ingress {
	cp := make([]Ingress, 0, len(list))
	for _, ing := range list {
		if !ing.Hide {
			cp = append(cp, ing)
		}
	}
	return cp
}
