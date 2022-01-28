package discovery

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

func NewKubernetesDiscovery(namespace string, serviceLabels map[string]string, raftPortName string) Discovery {
	if raftPortName == "" {
		raftPortName = "braft"
	}
	if len(serviceLabels) == 0 {
		serviceLabels = map[string]string{"svcType": "braft"}
	}

	return &kubernetesDiscovery{
		namespace:     namespace,
		serviceLabels: serviceLabels,
		portName:      raftPortName,
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
	}
}

// Name gives the name of the discovery.
func (d *kubernetesDiscovery) Name() string {
	var labels []string

	for k, v := range d.serviceLabels {
		labels = append(labels, k+":"+v)
	}

	return "k8s:ns=" + d.namespace + ",labels=" + strings.Join(labels, "&")
}

func (k *kubernetesDiscovery) Start(_ string, _ int) (chan string, error) {
	cc, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	kc, err := kubernetes.NewForConfig(cc)
	if err != nil {
		return nil, err
	}
	go k.discovery(kc)
	return k.discoveryChan, nil
}

func (k *kubernetesDiscovery) discovery(kc *kubernetes.Clientset) {
	for {
		select {
		case <-k.stopChan:
			return
		default:
			k.search(kc)
			time.Sleep(time.Duration(rand.Intn(5)+1) * time.Second)
		}
	}
}

func (k *kubernetesDiscovery) search(clientSet *kubernetes.Clientset) {
	services, err := clientSet.CoreV1().Services(k.namespace).List(context.Background(),
		metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(k.serviceLabels).String(),
			Watch:         false,
		})
	if err != nil {
		log.Println(err)
		return
	}

	for _, svc := range services.Items {
		set := labels.Set(svc.Spec.Selector)
		listOptions := metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(set).String(),
		}
		pods, err := clientSet.CoreV1().Pods(svc.Namespace).List(context.Background(), listOptions)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, pod := range pods.Items {
			if strings.ToLower(string(pod.Status.Phase)) == "running" {
				raftPort := k.findPort(pod)
				if podIp := pod.Status.PodIP; podIp != "" && raftPort.ContainerPort > 0 {
					k.discoveryChan <- fmt.Sprintf("%v:%v", podIp, raftPort.ContainerPort)
				}
			}
		}
	}
}

func (k *kubernetesDiscovery) findPort(pod v1.Pod) (p v1.ContainerPort) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == k.portName {
				return port
			}
		}
	}
	return p
}

func (k *kubernetesDiscovery) SupportsNodeAutoRemoval() bool { return true }
func (k *kubernetesDiscovery) Stop()                         { k.stopChan <- true }
