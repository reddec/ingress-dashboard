package internal

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hako/durafmt"
	"github.com/reddec/ingress-dashboard/internal/auth"
	"github.com/reddec/ingress-dashboard/internal/static"
	"gopkg.in/yaml.v3"
)

const (
	SoonExpiredInterval = 14 * 24 * time.Hour // 2 weeks
)

type Ingress struct {
	ID            string    `yaml:"-"`           // human readable ID (namespace with name)
	UID           string    `yaml:"-"`           // machine readable ID (guid in Kube)
	Title         string    `yaml:"-"`           // custom title in dashboard, overwrites Name
	Name          string    `yaml:"name"`        // ingress name as in Kube
	Namespace     string    `yaml:"namespace"`   // Kube namespace for ingress
	Description   string    `yaml:"description"` // optional, human-readable description of Ingress
	Hide          bool      `yaml:"-"`           // hidden Ingresses will not appear in UI
	LogoURL       string    `yaml:"logo_url"`    // custom URL for icon
	Class         string    `yaml:"-"`           // Ingress class
	Static        bool      `yaml:"-"`
	Refs          []Ref     `yaml:"-"`
	TLS           bool      `yaml:"-"`
	TLSExpiration time.Time `yaml:"-"`
}

type Ref struct {
	URL    string // link to ingress
	Pods   int    // number of pods linked to the service
	Static bool   // is reference defined statically (for static refs, pods number has no sense)
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
		for _, u := range ingress.Refs {
			return strings.TrimRight(u.URL, "/") + ingress.LogoURL
		}
	}
	return ingress.LogoURL
}

func (ingress Ingress) HasDeadRefs() bool {
	for _, ref := range ingress.Refs {
		if !ref.Static && ref.Pods == 0 {
			return true
		}
	}
	return false
}

func (ingress Ingress) IsTLSExpired() bool {
	return ingress.TLS && (time.Now().After(ingress.TLSExpiration))
}

func (ingress Ingress) IsTLSSoonExpire() bool {
	return ingress.TLS && (ingress.TLSExpiration.Sub(time.Now()) < SoonExpiredInterval)
}

func (ingress Ingress) WhenTLSExpires() string {
	return durafmt.Parse(ingress.TLSExpiration.Sub(time.Now())).String()
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
	sfs := http.FileServer(http.FS(static.Static()))
	router.HandleFunc("/", svc.getIndex)
	router.Handle("/static/", http.StripPrefix("/static", sfs))
	router.Handle("/favicon.ico", sfs)
	return svc
}

type Service struct {
	cache   atomic.Value // []Ingress
	prepend atomic.Value // []Ingres
	page    *template.Template
	router  *http.ServeMux
}

func (svc *Service) Set(ingress []Ingress) {
	svc.cache.Store(ingress)
}

func (svc *Service) Get() []Ingress {
	return svc.cache.Load().([]Ingress)
}

// Prepend static list of ingresses.
func (svc *Service) Prepend(ingress []Ingress) {
	svc.prepend.Store(ingress)
}

func (svc *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	svc.router.ServeHTTP(writer, request)
}

func (svc *Service) getList() []Ingress {
	prepend := svc.prepend.Load().([]Ingress)
	main := svc.cache.Load().([]Ingress)
	return append(prepend, main...)
}

func (svc *Service) getIndex(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html")
	_ = svc.page.Execute(writer, UIContext{
		Ingresses: visibleIngresses(svc.getList()),
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

// LoadDefinitions scans location (file or dir) for YAML/JSON (.yml, .yaml, .json) definitions of Ingress.
// Directories scanned recursive and each file can contain multiple definitions.
//
// Empty location is a special case and cause returning empty slice.
func LoadDefinitions(location string) ([]Ingress, error) {
	type YamlIngress struct {
		Ingress `yaml:",inline"`
		URLs    []string `yaml:"urls"`
	}

	if location == "" {
		return nil, nil
	}
	var ans []Ingress
	err := filepath.Walk(location, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			return err
		}
		ext := filepath.Ext(path)
		if !(ext == ".yml" || ext == ".yaml" || ext == ".json") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open config file: %w", err)
		}
		defer f.Close()

		var decoder = yaml.NewDecoder(f)
		for {
			var ingress YamlIngress
			ingress.Static = true
			err := decoder.Decode(&ingress)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("decode config %s: %w", path, err)
			}

			for _, u := range ingress.URLs {
				ingress.Refs = append(ingress.Refs, Ref{
					URL:    u,
					Static: true,
				})
			}

			ans = append(ans, ingress.Ingress)
		}

		return nil
	})
	return ans, err
}
