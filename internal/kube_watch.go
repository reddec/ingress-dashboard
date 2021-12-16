package internal

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	v12 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	watcher := newWatcher(ctx, reciever, clientset)

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

func newWatcher(global context.Context, receiver Receiver, clientset *kubernetes.Clientset) *kubeWatcher {
	return &kubeWatcher{
		global:     global,
		cache:      make(map[string]Ingress),
		receiver:   receiver,
		checkLogos: make(chan struct{}, 1),
		clientset:  clientset,
	}
}

type kubeWatcher struct {
	global     context.Context
	clientset  *kubernetes.Clientset
	cache      map[string]Ingress
	lock       sync.RWMutex
	receiver   Receiver
	checkLogos chan struct{}
}

func (kw *kubeWatcher) OnAdd(obj interface{}) {
	kw.upsertIngress(kw.global, obj)
}

func (kw *kubeWatcher) OnUpdate(_, newObj interface{}) {
	kw.upsertIngress(kw.global, newObj)
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
			if !ing.Hide && ing.LogoURL == "" && len(ing.Refs) > 0 {
				ing.LogoURL = detectIconURL(ctx, ing.Refs[0].URL)
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

func (kw *kubeWatcher) upsertIngress(ctx context.Context, obj interface{}) {
	defer kw.notify()
	ing := obj.(*v12.Ingress)
	ingress := kw.inspectIngress(ctx, ing)

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

func (kw *kubeWatcher) inspectIngress(ctx context.Context, ing *v12.Ingress) Ingress {
	return Ingress{
		Name:        ing.Name,
		Namespace:   ing.Namespace,
		Title:       ing.Annotations[AnnoTitle],
		ID:          ing.Namespace + "." + ing.Name,
		UID:         string(ing.UID),
		Description: ing.Annotations[AnnoDescription],
		LogoURL:     ing.Annotations[AnnoLogoURL],
		Hide:        toBool(ing.Annotations[AnnoHide], false),
		Refs:        kw.getRefs(ctx, ing),
		TLS:         len(ing.Spec.TLS) > 0,
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

func (kw *kubeWatcher) getRefs(ctx context.Context, ing *v12.Ingress) []Ref {
	proto := "http://"
	if len(ing.Spec.TLS) > 0 {
		proto = "https://"
	}

	var refs []Ref
	for _, rule := range ing.Spec.Rules {
		baseURL := proto + rule.Host
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				var ref = Ref{
					URL: baseURL + path.Path,
				}
				numPods, err := kw.getPodsNum(ctx, ing.Namespace, path.Backend.Service)
				if err != nil {
					log.Println("failed to get pods num for ingress", ing.Name, "in", ing.Namespace, "for path", path.Path, "-", err)
				} else {
					ref.Pods = numPods
				}
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

func (kw *kubeWatcher) getPodsNum(ctx context.Context, ns string, svc *v12.IngressServiceBackend) (int, error) {
	if svc == nil {
		return 0, nil
	}
	info, err := kw.clientset.CoreV1().Services(ns).Get(ctx, svc.Name, v1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("get service %s in %s: %w", svc.Name, ns, err)
	}

	var extHosts = len(info.Spec.ExternalIPs)
	if extHosts == 0 && info.Spec.ExternalName != "" {
		// reference by DNS to external host
		extHosts = 1
	}

	return len(info.Spec.ClusterIPs) + extHosts, nil
}

func toBool(value string, defaultValue bool) bool {
	if v, err := strconv.ParseBool(value); err == nil {
		return v
	}
	return defaultValue
}
