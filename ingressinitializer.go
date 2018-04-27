package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/frodehus/dnsinit/config"
	"github.com/frodehus/dnsinit/dns"
	"github.com/ghodss/yaml"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"k8s.io/client-go/kubernetes"
)

//IngressInitializer handles creation/updates of DNS record sets matching the configured hosts in the Ingress
type IngressInitializer struct {
	kubeclient       kubernetes.Interface
	config           config.DnsInitializerConfig
	dnsClient        dns.DNSClient
	servicePrincipal string
	clientSecret     string
}

const (
	annotation      = "initializer.sample.io/ingress"
	initializerName = "ingress.initializer.sample.io"
)

//NewIngressInitializer will create a initializor for handling ingresses
func NewIngressInitializer(clientset kubernetes.Interface) (*IngressInitializer, error) {
	initializer := &IngressInitializer{
		kubeclient: clientset,
	}

	initializer.configure()
	return initializer, nil
}

//Create will check if a DNS record set exists for the Ingress - creates new as necessary
func (i *IngressInitializer) Create(ingress *extv1beta1.Ingress) error {
	ingress, err := i.initialize(ingress)
	if err != nil {
		return err
	}

	if ingress == nil {
		return nil
	}

	for _, rule := range ingress.Spec.Rules {
		hostname := rule.Host[:strings.Index(rule.Host, ".")]
		i.dnsClient.LookupRecord(hostname)
	}
	return nil
}

//Update will update a DNS record set if the host has changed
func (i *IngressInitializer) Update(oldIngress *extv1beta1.Ingress, updatedIngress *extv1beta1.Ingress) error {

	return nil
}

//Delete will remove a DNS record set
func (i *IngressInitializer) Delete(ingress *extv1beta1.Ingress) error {
	//we dont need to do anything about this object, just grab the host info and delete the DNS record
	for _, rule := range ingress.Spec.Rules {
		log.Printf("\tDeleting %s", rule.Host)
	}
	return nil
}

func (i *IngressInitializer) configure() {
	cm, err := i.kubeclient.CoreV1().ConfigMaps("default").Get("ingress-config", metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Unable to load config from 'ingress-config': %s", err.Error())
		panic("Failed to load configuration")
	}
	var initializerConfig config.DnsInitializerConfig
	err = yaml.Unmarshal([]byte(cm.Data["config"]), &initializerConfig)
	if err != nil {
		log.Fatalf("Unable to parse YAML config from 'ingress-config': %s", err.Error())
		panic("Failed to load configuration")
	}
	secret, err := i.kubeclient.CoreV1().Secrets("default").Get(initializerConfig.Azure.Secret, metav1.GetOptions{})
	i.servicePrincipal = string(secret.Data["servicePrincipal"])
	i.clientSecret = string(secret.Data["secret"])
	i.config = initializerConfig

	i.dnsClient, _ = dns.NewDNSClient(i.servicePrincipal, i.clientSecret, i.config.Azure)

}

func (i *IngressInitializer) initialize(ingress *extv1beta1.Ingress) (*extv1beta1.Ingress, error) {
	initializedIngress := ingress.DeepCopy()

	if !i.isPendingInitialization(initializedIngress) {
		log.Printf("\tIngress skipped")
		return nil, nil
	}

	i.removeSelfFromPendingInitializers(initializedIngress)
	defer i.saveIngress(ingress, initializedIngress)

	if !i.hasRequiredAnnotation(initializedIngress) {
		return nil, nil
	}
	log.Printf("\tInitializing %s", initializedIngress.GetName())
	i.setAnnotations(initializedIngress)
	_, err := i.kubeclient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Update(initializedIngress)
	if err != nil {
		log.Printf("Error initializing ingress: %s", err.Error())
		return nil, err
	}
	return initializedIngress, nil
}

func (i *IngressInitializer) isPendingInitialization(ingress *extv1beta1.Ingress) bool {
	pendingInitializers := i.getPendingInitializers(ingress)
	if pendingInitializers == nil || initializerName != pendingInitializers[0].Name {
		return false
	}
	return true
}

func (i *IngressInitializer) hasRequiredAnnotation(ingress *extv1beta1.Ingress) bool {
	annotations := ingress.ObjectMeta.GetAnnotations()
	_, hasRequiredAnnotation := annotations[annotation]
	if !hasRequiredAnnotation {
		log.Printf("\t%s requires '%s' annotation; skipping initialization", initializerName, annotation)
		return false
	}
	return true
}

func (i *IngressInitializer) setAnnotations(ingress *extv1beta1.Ingress) {
	ingress.ObjectMeta.Annotations["ingress.sample.io"] = "mutillidae.pepperprovesapoint.com"
}

func (i *IngressInitializer) getPendingInitializers(ingress *extv1beta1.Ingress) []metav1.Initializer {
	if ingress.ObjectMeta.GetInitializers() == nil {
		return nil
	}
	pendingInitializers := ingress.ObjectMeta.GetInitializers().Pending
	return pendingInitializers
}

func (i *IngressInitializer) removeSelfFromPendingInitializers(ingress *extv1beta1.Ingress) {
	pendingInitializers := i.getPendingInitializers(ingress)
	if pendingInitializers == nil {
		return
	}
	if len(pendingInitializers) == 1 {
		ingress.ObjectMeta.Initializers = nil
	} else {
		ingress.ObjectMeta.Initializers.Pending = append(pendingInitializers[:0], pendingInitializers[1:]...)
	}
}

func (i *IngressInitializer) saveIngress(ingress *extv1beta1.Ingress, initializedIngress *extv1beta1.Ingress) error {
	oldData, err := json.Marshal(ingress)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(initializedIngress)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, extv1beta1.Ingress{})
	if err != nil {
		return err
	}

	_, err = i.kubeclient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Patch(ingress.Name, types.StrategicMergePatchType, patchBytes)
	if err != nil {
		return err
	}
	return nil
}
