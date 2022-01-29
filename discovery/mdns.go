package discovery

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/grandcat/zeroconf"
)

type mdnsDiscovery struct {
	nodeID        string
	serviceName   string
	nodePort      int
	mdnsServer    *zeroconf.Server
	discoveryChan chan string
	stopChan      chan bool
	tempQueue     []string
}

func NewMdnsDiscovery(serviceName string) Discovery {
	if serviceName == "" {
		serviceName = "_braft._tcp"
	}
	rand.Seed(time.Now().UnixNano())

	return &mdnsDiscovery{
		nodeID:        "",
		serviceName:   serviceName,
		nodePort:      0,
		mdnsServer:    &zeroconf.Server{},
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
	}
}

// Name gives the name of the discovery.
func (d *mdnsDiscovery) Name() string { return "mdns:" + d.serviceName }

func (d *mdnsDiscovery) Start(nodeID string, nodePort int) (chan string, error) {
	if len(nodeID) > 27 {
		nodeID = nodeID[:27]
	}
	d.nodeID, d.nodePort = nodeID, nodePort
	go d.discovery()

	// 防止各个节点同时启动太快，随机休眠
	time.Sleep(time.Duration(rand.Intn(500)+500) * time.Millisecond)
	return d.discoveryChan, nil
}

func (k *mdnsDiscovery) Search() (dest []string, err error) { return k.tempQueue, nil }

const maxQueueSize = 10

func (d *mdnsDiscovery) discovery() {
	// expose mdns server
	mdnsServer, err := d.exposeMDNS()
	if err != nil {
		log.Fatal(err)
	}
	d.mdnsServer = mdnsServer

	// fetch mDNS enabled raft nodes
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize mDNS resolver:", err.Error())
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case <-d.stopChan:
				return
			case entry := <-entries:
				value := fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
				d.discoveryChan <- value
				d.tempQueue = append(d.tempQueue, value)
				if len(d.tempQueue) > maxQueueSize {
					copy(d.tempQueue, d.tempQueue[1:])
					d.tempQueue = d.tempQueue[:maxQueueSize]
				}
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		select {
		case <-d.stopChan:
			return
		default:
			if err = resolver.Browse(ctx, d.serviceName, "local.", entries); err != nil {
				log.Printf("Error during mDNS lookup: %v", err)
			}
			time.Sleep(time.Duration(rand.Intn(5)+1) * time.Second)
		}
	}
}

func (d *mdnsDiscovery) exposeMDNS() (*zeroconf.Server, error) {
	return zeroconf.Register(d.nodeID, d.serviceName, "local.", d.nodePort, []string{"txtv=0", "lo=1", "la=2"}, nil)
}

func (d *mdnsDiscovery) IsStatic() bool { return false }

func (d *mdnsDiscovery) Stop() {
	d.stopChan <- true
	d.mdnsServer.Shutdown()
}
