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
	delayTime     time.Duration
	nodeID        string
	serviceName   string
	nodePort      int
	mdnsServer    *zeroconf.Server
	discoveryChan chan string
	stopChan      chan bool
}

func NewMdnsDiscovery(serviceName string) Method {
	if serviceName == "" {
		serviceName = "_braft._tcp"
	}
	rand.Seed(time.Now().UnixNano())
	delayTime := time.Duration(rand.Intn(5)+1) * time.Second
	return &mdnsDiscovery{
		delayTime:     delayTime,
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
	d.nodeID, d.nodePort = nodeID, nodePort
	go d.discovery()
	return d.discoveryChan, nil
}

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
				break
			case entry := <-entries:
				d.discoveryChan <- fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	for {
		select {
		case <-d.stopChan:
			cancel()
			break
		default:
			err = resolver.Browse(ctx, d.serviceName, "local.", entries)
			if err != nil {
				log.Printf("Error during mDNS lookup: %v", err)
			}
			time.Sleep(d.delayTime)
		}
	}
}

func (d *mdnsDiscovery) exposeMDNS() (*zeroconf.Server, error) {
	return zeroconf.Register(d.nodeID, d.serviceName, "local.", d.nodePort, []string{"txtv=0", "lo=1", "la=2"}, nil)
}

func (d *mdnsDiscovery) SupportsNodeAutoRemoval() bool { return true }

func (d *mdnsDiscovery) Stop() {
	d.stopChan <- true
	d.mdnsServer.Shutdown()
}
