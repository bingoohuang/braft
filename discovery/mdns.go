package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/grandcat/zeroconf"
)

type mdnsDiscovery struct {
	nodeID        string
	serviceName   string
	nodePort      int
	mdnsServer    *zeroconf.Server
	discoveryChan chan string
	stopChan      chan bool
	tempQueue     *util.UniqueQueue
}

func NewMdnsDiscovery(serviceName string) Discovery {
	return &mdnsDiscovery{
		nodeID:        "",
		serviceName:   ss.Or(serviceName, "_braft._tcp"),
		nodePort:      0,
		mdnsServer:    &zeroconf.Server{},
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
		tempQueue:     util.NewUniqueQueue(100),
	}
}

// Name gives the name of the discovery.
func (d *mdnsDiscovery) Name() string { return "mdns://" + d.serviceName }

func (d *mdnsDiscovery) Start(nodeID string, nodePort int) (chan string, error) {
	if len(nodeID) > 27 {
		nodeID = nodeID[:27]
	}
	d.nodeID, d.nodePort = nodeID, nodePort
	go d.discovery()

	// 防止各个节点同时启动太快，随机休眠
	util.RandSleep(100*time.Millisecond, 1*time.Second, true)
	return d.discoveryChan, nil
}

func (k *mdnsDiscovery) Search() (dest []string, err error) { return k.tempQueue.Get(), nil }

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
				if entry != nil {
					value := fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
					d.discoveryChan <- value
					d.tempQueue.Put(value)
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
			util.RandSleep(time.Second, 5*time.Second, false)
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
