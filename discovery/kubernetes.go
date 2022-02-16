package discovery

import (
	"context"
	"log"
	"time"

	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/ss"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type kubernetesDiscovery struct {
	namespace     string
	serviceLabels map[string]string
	portName      string
	discoveryChan chan string
	stopChan      chan bool

	clientSet *kubernetes.Clientset
}

func NewKubernetesDiscovery(namespace, portName string, serviceLabels map[string]string) Discovery {
	return &kubernetesDiscovery{
		namespace:     namespace,
		serviceLabels: serviceLabels,
		portName:      portName,
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
	}
}

// Name gives the name of the discovery.
func (k *kubernetesDiscovery) Name() string {
	return "k8s://ns=" + k.namespace +
		"/labels=" + ss.JoinMap(k.serviceLabels, "=", ",") +
		"/portName=" + k.portName
}

func (k *kubernetesDiscovery) Start(_ string, _ int) (chan string, error) {
	util.Think(ss.Or(util.Env("K8S_SLEEP"), "15-30s"))

	cc, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	if k.clientSet, err = kubernetes.NewForConfig(cc); err != nil {
		return nil, err
	}
	// do a search at first.
	k.search(true)
	go k.discovery()
	return k.discoveryChan, nil
}

func (k *kubernetesDiscovery) discovery() {
	for {
		select {
		case <-k.stopChan:
			return
		case <-time.After(time.Duration(randx.IntBetween(1, 6)) * time.Second):
			k.search(false)
		}
	}
}

func (k *kubernetesDiscovery) search(gorun bool) {
	result, err := k.Search()
	if err != nil {
		log.Printf("search k8s error: %v", err)
	}

	f := func() {
		for _, item := range result {
			k.discoveryChan <- item
		}
	}

	if gorun {
		go f()
	} else {
		f()
	}
}

func (k *kubernetesDiscovery) Search() (dest []string, err error) {
	log.Printf("[L:1m] search services with namespace: %s, labels: %v", k.namespace, k.serviceLabels)
	start := time.Now()
	defer log.Printf("[L:1m] search completed with cost %s", time.Since(start))

	services, err := k.clientSet.CoreV1().Services(k.namespace).List(context.Background(),
		meta.ListOptions{
			LabelSelector: labels.SelectorFromSet(k.serviceLabels).String(),
			Watch:         false,
		})
	if err != nil {
		log.Printf("search services error: %v", err)
		return nil, err
	}

	for _, svc := range services.Items {
		set := labels.Set(svc.Spec.Selector)
		listOptions := meta.ListOptions{
			LabelSelector: labels.SelectorFromSet(set).String(),
		}
		pods, err := k.clientSet.CoreV1().Pods(svc.Namespace).List(context.Background(), listOptions)
		if err != nil {
			return dest, err
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == core.PodRunning {
				podIp := pod.Status.PodIP
				log.Printf("[L:1m] pod phase: %s, pod iP: %s", pod.Status.Phase, podIp)
				if p := k.findPort(pod); p.ContainerPort > 0 {
					dest = append(dest, podIp)
				}
			}
		}
	}

	return dest, nil
}

func (k *kubernetesDiscovery) findPort(pod core.Pod) (p core.ContainerPort) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			log.Printf("[L:1m] port: %+v", port)
			if k.portName == "" || port.Name == k.portName {
				return port
			}
		}
	}
	return p
}

func (k *kubernetesDiscovery) Stop() { k.stopChan <- true }
