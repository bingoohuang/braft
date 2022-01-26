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

const (
	defaultK8sDiscoveryNodePortName = "easyraft"
	defaultK8sDiscoverySvcType      = "easyraft"
)

type KubernetesDiscovery struct {
	namespace             string
	matchingServiceLabels map[string]string
	nodePortName          string
	discoveryChan         chan string
	stopChan              chan bool
	delayTime             time.Duration
}

func NewKubernetesDiscovery(namespace string, serviceLabels map[string]string, raftPortName string) DiscoveryMethod {
	if raftPortName == "" {
		raftPortName = defaultK8sDiscoveryNodePortName
	}
	if len(serviceLabels) == 0 {
		serviceLabels = map[string]string{"svcType": defaultK8sDiscoverySvcType}
	}

	return &KubernetesDiscovery{
		namespace:             namespace,
		matchingServiceLabels: serviceLabels,
		nodePortName:          raftPortName,
		discoveryChan:         make(chan string),
		stopChan:              make(chan bool),
		delayTime:             time.Duration(rand.Intn(5)+1) * time.Second,
	}
}

// Name gives the name of the discovery.
func (d *KubernetesDiscovery) Name() string {
	var labels []string

	for k, v := range d.matchingServiceLabels {
		labels = append(labels, k+":"+v)
	}

	return "K8s:namespace=" + d.namespace + ",labels=" + strings.Join(labels, "&")
}

func (k *KubernetesDiscovery) Start(_ string, _ int) (chan string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	go k.discovery(clientSet)
	return k.discoveryChan, nil
}

func (k *KubernetesDiscovery) discovery(clientSet *kubernetes.Clientset) {
	for {
		select {
		case <-k.stopChan:
			return
		default:
			k.search(clientSet)
			time.Sleep(k.delayTime)
		}
	}
}

func (k *KubernetesDiscovery) search(clientSet *kubernetes.Clientset) {
	services, err := clientSet.CoreV1().Services(k.namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(k.matchingServiceLabels).String(),
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

func (k *KubernetesDiscovery) findPort(pod v1.Pod) (p v1.ContainerPort) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == k.nodePortName {
				return port
			}
		}
	}
	return p
}

func (k *KubernetesDiscovery) SupportsNodeAutoRemoval() bool { return true }

func (k *KubernetesDiscovery) Stop() {
	k.stopChan <- true
}
