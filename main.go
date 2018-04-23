package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	annotation      = "initializer.sample.io/ingress"
	initializerName = "ingress.initializer.sample.io"
)

func main() {
	var config *rest.Config
	stop := make(chan struct{})
	var err error
	kubeconfig := filepath.Join(
		os.Getenv("HOME"), ".kube", "config",
	)
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	defer close(stop)
	start(clientset, stop)
	<-stop
}

func start(clientset *kubernetes.Clientset, stopCh <-chan struct{}) {
	log.Print("Starting ingress watcher...")
	restClient := clientset.ExtensionsV1beta1().RESTClient()
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
			log.Printf("Ingress added: %s", i.GetName())
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			existing := oldObj.(*extv1beta1.Ingress)
			updated := newObj.(*extv1beta1.Ingress)
			if existing.GetResourceVersion() == updated.GetResourceVersion() {
				return
			}
			log.Printf("Ingress %s v%s updated: v%s", existing.GetName(), existing.GetResourceVersion(), updated.GetResourceVersion())
		},
		DeleteFunc: func(deletedObj interface{}) {
			i := deletedObj.(*extv1beta1.Ingress)
			log.Printf("Ingress deleted: %s", i.GetName())
		},
	})

	go controller.Run(stopCh)
}
