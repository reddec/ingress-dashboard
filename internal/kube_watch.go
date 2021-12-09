package internal

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	v12 "k8s.io/api/networking/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const (
	AnnoDescription = "ingress-dashboard/description"
	AnnoLogoURL     = "ingress-dashboard/logo-url"
	AnnoTitle       = "ingress-dashboard/title"
	AnnoHide        = "ingress-dashboard/hide" // do not display ingress in dashboard
	syncInterval    = 30 * time.Second
)

type Receiver interface {
	Set(ingresses []Ingress)
}

func WatchKubernetes(global context.Context, clientset *kubernetes.Clientset, reciever interface {
	Set(ingresses []Ingress)
}) {
	ctx, cancel := context.WithCancel(global)
	defer cancel()

	watcher := newWatcher(reciever)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		watcher.runWatcher(ctx, clientset)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		watcher.runLogoFetcher(ctx)
	}()
	wg.Wait()
}

func newWatcher(receiver Receiver) *kubeWatcher {
	return &kubeWatcher{
		cache:      make(map[string]Ingress),
		receiver:   receiver,
		checkLogos: make(chan struct{}, 1),
	}
}

type kubeWatcher struct {
	cache      map[string]Ingress
	lock       sync.RWMutex
	receiver   Receiver
	checkLogos chan struct{}
}

func (kw *kubeWatcher) OnAdd(obj interface{}) {
	kw.upsertIngress(obj)
}

func (kw *kubeWatcher) OnUpdate(_, newObj interface{}) {
	kw.upsertIngress(newObj)
}

func (kw *kubeWatcher) OnDelete(obj interface{}) {
	defer kw.notify()

	kw.lock.Lock()
	defer kw.lock.Unlock()
	ing := obj.(*v12.IngressClass)
	delete(kw.cache, string(ing.UID))
}

func (kw *kubeWatcher) runLogoFetcher(ctx context.Context) {
	for {
		for _, ing := range kw.items() {
			if !ing.Hide && ing.LogoURL == "" && len(ing.URLs) > 0 {
				ing.LogoURL = detectIconURL(ctx, ing.URLs[0])
				if ing.LogoURL != "" {
					kw.updateLogo(ing)
				}
			}
		}
		kw.receiver.Set(kw.items())
		select {
		case <-ctx.Done():
			return
		case <-kw.checkLogos:
		}
	}
}

func (kw *kubeWatcher) runWatcher(ctx context.Context, clientset *kubernetes.Clientset) {
	informerFactory := informers.NewSharedInformerFactory(clientset, syncInterval)
	informer := informerFactory.Networking().V1().Ingresses().Informer()

	informer.AddEventHandler(kw)
	informer.Run(ctx.Done())
}

func (kw *kubeWatcher) upsertIngress(obj interface{}) {
	defer kw.notify()
	ing := obj.(*v12.Ingress)
	ingress := inspectIngress(ing)

	kw.lock.Lock()
	defer kw.lock.Unlock()
	// preserve discovered logo
	oldLogoURL := kw.cache[ingress.UID].LogoURL
	if oldLogoURL != "" && ingress.LogoURL == "" {
		ingress.LogoURL = oldLogoURL
	}
	kw.cache[ingress.UID] = ingress
}

func (kw *kubeWatcher) notify() {
	kw.receiver.Set(kw.items())
	select {
	case kw.checkLogos <- struct{}{}:
	default:
	}
}

func (kw *kubeWatcher) items() []Ingress {
	kw.lock.RLock()
	defer kw.lock.RUnlock()
	return toList(kw.cache)
}

func (kw *kubeWatcher) updateLogo(ingress Ingress) {
	kw.lock.Lock()
	defer kw.lock.Unlock()
	old, exists := kw.cache[ingress.UID]
	if !exists || old.LogoURL != "" {
		return
	}
	old.LogoURL = ingress.LogoURL
	kw.cache[ingress.UID] = old
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
		Hide:        toBool(ing.Annotations[AnnoHide], false),
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

func toBool(value string, defaultValue bool) bool {
	if v, err := strconv.ParseBool(value); err == nil {
		return v
	}
	return defaultValue
}
