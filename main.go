package main

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/rest"
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
	watcher, _ := NewIngressWatcher(clientset)
	watcher.Run(stop)
	<-stop
}
