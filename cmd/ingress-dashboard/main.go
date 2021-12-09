package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/jessevdk/go-flags"
	"github.com/reddec/ingress-dashboard/internal"
	"github.com/reddec/ingress-dashboard/internal/auth"
	httpserver "github.com/reddec/run-http-server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

type Config struct {
	httpserver.Server
	Master        string `long:"master" env:"MASTER" description:"Kuberenetes master URL"`
	Kubeconfig    string `long:"kubeconfig" env:"KUBECONFIG" description:"Path to kubeconfig for local setup"`
	OIDCIssuer    string `long:"oidc-issuer" env:"OIDC_ISSUER" description:"OIDC issuer URL"`
	ClientID      string `long:"client-id" env:"CLIENT_ID" description:"OAuth client ID"`
	ClientSecret  string `long:"client-secret" env:"CLIENT_SECRET" description:"OAuth client secret"`
	ServerURL     string `long:"server-url" env:"SERVER_URL" description:"Server URL used for OAuth redirects"`
	Auth          string `long:"auth" env:"AUTH" description:"Auth scheme" default:"none" choice:"none" choice:"oidc" choice:"basic"`
	BasicUser     string `long:"basic-user" env:"BASIC_USER" description:"Basic Auth username"`
	BasicPassword string `long:"basic-password" env:"BASIC_PASSWORD" description:"Basic Auth password"`
}

func main() {
	var config Config
	parser := flags.NewParser(&config, flags.Default)
	parser.ShortDescription = "Kubernetes-native dashboard for ingress"
	parser.LongDescription = fmt.Sprintf("Kubernetes-native dashboard for ingress\ningress-dashboard %s, commit %s, built at %s by %s\nAuthor: Aleksandr Baryshnikov <owner@reddec.net>", version, commit, date, builtBy)

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
	if err := run(config); err != nil {
		log.Panic(err)
	}
}

func run(cfg Config) error {
	config, err := clientcmd.BuildConfigFromFlags(cfg.Master, cfg.Kubeconfig)
	if err != nil {
		return fmt.Errorf("get kube config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	svc := internal.New()

	secured, err := cfg.secureHandler(ctx, svc)
	if err != nil {
		return fmt.Errorf("secure handler: %w", err)
	}

	go func() {
		defer cancel()
		internal.WatchKubernetes(ctx, clientset, svc)
	}()

	http.Handle("/", secured)
	return cfg.Run(ctx)
}

func (cfg Config) secureHandler(ctx context.Context, handler http.Handler) (http.Handler, error) {
	switch cfg.Auth {
	case "none":
		return handler, nil
	case "oidc":
		return auth.NewOIDC(ctx, auth.Config{
			IssuerURL:    cfg.OIDCIssuer,
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			ServerURL:    cfg.ServerURL,
		}, handler)
	case "basic":
		return auth.NewBasic(cfg.BasicUser, cfg.BasicPassword, handler), nil
	default:
		return nil, fmt.Errorf("unknown auth scheme %s", cfg.Auth)
	}
}
