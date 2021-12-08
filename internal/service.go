package internal

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/reddec/ingress-dashboard/internal/auth"
	"github.com/reddec/ingress-dashboard/internal/static"
	"golang.org/x/net/html"
)

type Ingress struct {
	ID          string   `yaml:"id"`
	UID         string   `yaml:"uid"`
	Title       string   `yaml:"title"`
	Name        string   `yaml:"name"`
	Namespace   string   `yaml:"namespace"`
	Description string   `yaml:"description"`
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
		Ingresses: svc.Get(),
		User:      auth.UserFromContext(request.Context()),
	})
}

func detectIconURL(ctx context.Context, url string) string {
	u, err := mainSrcPageIcon(ctx, url)
	if err == nil {
		return u
	}
	log.Println("detect icon from main page", url, ":", err)
	// fallback to classical icon
	faviconURL := url + "/favicon.ico"
	if pingURL(ctx, faviconURL) == nil {
		return faviconURL
	}
	return ""
}

func mainSrcPageIcon(ctx context.Context, pageURL string) (string, error) {
	// try to load main page and check meta headers
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 code")
	}
	doc, err := html.Parse(res.Body)
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}
	logoURL := findIcons(doc)
	if logoURL == "" {
		return "", fmt.Errorf("no logo in meta")
	}
	if u, err := url.Parse(logoURL); err == nil && !u.IsAbs() && !strings.HasPrefix(logoURL, "/") {
		logoURL = "/" + logoURL
	}
	return logoURL, nil
}

func findIcons(doc *html.Node) string {
	priorityList := []string{"apple-touch-icon", "shortcut icon", "icon", "alternate icon"}

	root := findChild(doc, "html")
	if root == nil {
		return ""
	}

	head := findChild(root, "head")
	if head == nil {
		return ""
	}

	var links = make(map[string]string)
	for child := head.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "link" {
			var key string
			var value string
			for _, attr := range child.Attr {
				if attr.Key == "rel" {
					key = attr.Val
				} else if attr.Key == "href" {
					value = attr.Val
				}
			}
			if key != "" && value != "" {
				links[key] = value
			}
		}
	}
	for _, name := range priorityList {
		if u, ok := links[name]; ok {
			return u
		}
	}
	return ""
}

func pingURL(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 code")
	}
	return nil
}

func findChild(doc *html.Node, name string) *html.Node {
	for child := doc.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == name {
			return child
		}
	}
	return nil
}
