package main

import (
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/cache"
)

type IngressWatcher struct {
	kubeclient  kubernetes.Interface
	initializer IngressInitializer
}

func NewIngressWatcher(clientset kubernetes.Interface) (*IngressWatcher, error) {
	initializer, err := NewIngressInitializer(clientset)
	if err != nil {
		log.Fatalf("Failed to get initializer: %s", err.Error())
		return nil, err
	}
	watcher := &IngressWatcher{
		kubeclient:  clientset,
		initializer: *initializer,
	}

	return watcher, nil
}

func (w *IngressWatcher) Run(stopCh <-chan struct{}) {
	log.Print("Starting ingress watcher...")
	restClient := w.kubeclient.ExtensionsV1beta1().RESTClient()
	watchList := cache.NewListWatchFromClient(restClient, "ingresses", corev1.NamespaceAll, fields.Everything())

	watchListIncludingUninitialized := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return watchList.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return watchList.Watch(options)
		},
	}
	resyncPeriod := 30 * time.Second
	_, controller := cache.NewInformer(watchListIncludingUninitialized, &extv1beta1.Ingress{}, resyncPeriod, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			i := obj.(*extv1beta1.Ingress)
			log.Printf("Ingress added: %s", i.Name)
			w.initializer.Create(i)
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			existing := oldObj.(*extv1beta1.Ingress)
			updated := newObj.(*extv1beta1.Ingress)
			if existing.ResourceVersion == updated.ResourceVersion {
				return
			}
			log.Printf("Ingress %s v%s updated: v%s", existing.Name, existing.ResourceVersion, updated.ResourceVersion)
			w.initializer.Update(existing, updated)
		},
		DeleteFunc: func(deletedObj interface{}) {
			i := deletedObj.(*extv1beta1.Ingress)
			log.Printf("Ingress deleted: %s", i.Name)
			w.initializer.Delete(i)
		},
	})

	go controller.Run(stopCh)
}
