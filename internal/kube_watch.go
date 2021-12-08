package internal

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	v12 "k8s.io/api/networking/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	cache2 "k8s.io/client-go/tools/cache"
)

const (
	AnnoDescription = "ingress-dashboard/description"
	AnnoLogoURL     = "ingress-dashboard/logo-url"
	AnnoTitle       = "ingress-dashboard/title"
	syncInterval    = 30 * time.Second
)

func WatchKubernetes(ctx context.Context, clientset *kubernetes.Clientset, reciever interface {
	Set(ingresses []Ingress)
}) {
	var cache = make(map[string]Ingress)
	var lock sync.Mutex
	var toDetect = make(chan Ingress, 1024)
	defer close(toDetect)

	go runLogoFetcher(ctx, toDetect, func(ing Ingress) {
		lock.Lock()
		defer lock.Unlock()
		if v, ok := cache[ing.UID]; ok && v.LogoURL == "" {
			v.LogoURL = ing.LogoURL
			cache[ing.UID] = v
			reciever.Set(toList(cache))
		}
	})
	informerFactory := informers.NewSharedInformerFactory(clientset, syncInterval)
	informer := informerFactory.Networking().V1().Ingresses().Informer()

	informer.AddEventHandler(cache2.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ing := obj.(*v12.Ingress)
			ingress := inspectIngress(ing)

			lock.Lock()
			defer lock.Unlock()
			cache[string(ing.UID)] = ingress

			log.Println("new ingress", ing.UID, ":", ing.Name, "in", ing.Namespace)
			reciever.Set(toList(cache))
			select {
			case toDetect <- ingress:
			default:
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			ing := newObj.(*v12.Ingress)
			ingress := inspectIngress(ing)

			lock.Lock()
			defer lock.Unlock()
			cache[string(ing.UID)] = ingress

			log.Println("updated ingress", ing.UID, ":", ing.Name, "in", ing.Namespace)
			reciever.Set(toList(cache))
			select {
			case toDetect <- ingress:
			default:
			}
		},
		DeleteFunc: func(obj interface{}) {
			ing := obj.(*v12.IngressClass)

			lock.Lock()
			defer lock.Unlock()
			delete(cache, string(ing.UID))

			log.Println("ingress removed", ing.UID, ":", ing.Name, "in", ing.Namespace)
		},
	})
	informer.Run(ctx.Done())
}

func inspectIngress(ing *v12.Ingress) Ingress {
	return Ingress{
		Name:        ing.Name,
		Namespace:   ing.Namespace,
		Title:       ing.Annotations[AnnoTitle],
		ID:          ing.Namespace + "." + ing.Name,
		UID:         string(ing.UID),
		Description: ing.Annotations[AnnoDescription],
		LogoURL:     ing.Annotations[AnnoLogoURL],
		URLs:        toURLs(ing.Spec),
	}
}

func toList(cache map[string]Ingress) []Ingress {
	var cp = make([]Ingress, 0, len(cache))
	for _, ing := range cache {
		cp = append(cp, ing)
	}
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].ID < cp[j].ID
	})
	return cp
}

func toURLs(spec v12.IngressSpec) []string {
	proto := "http://"
	if len(spec.TLS) > 0 {
		proto = "https://"
	}

	var urls []string
	for _, rule := range spec.Rules {
		baseURL := proto + rule.Host
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				urls = append(urls, baseURL+path.Path)
			}
		}
	}

	return urls
}

func runLogoFetcher(ctx context.Context, ch <-chan Ingress, fn func(ing Ingress)) {
	for ing := range ch {
		if ing.LogoURL == "" && len(ing.URLs) > 0 {
			ing.LogoURL = detectIconURL(ctx, ing.URLs[0])
			if ing.LogoURL != "" {
				fn(ing)
			}
		}
	}
}
