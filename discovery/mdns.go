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
	tempQueue     *util.UniqueQueue
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewMdnsDiscovery(serviceName string) Discovery {
	return &mdnsDiscovery{
		nodeID:        "",
		serviceName:   ss.Or(serviceName, "_braft._tcp"),
		nodePort:      0,
		mdnsServer:    &zeroconf.Server{},
		discoveryChan: make(chan string),
		tempQueue:     util.NewUniqueQueue(100),
	}
}

// Name gives the name of the discovery.
func (k *mdnsDiscovery) Name() string { return "mdns://" + k.serviceName }

func (k *mdnsDiscovery) Start(nodeID string, nodePort int) (chan string, error) {
	k.nodeID, k.nodePort = ss.Left(nodeID, 27), nodePort
	k.ctx, k.cancel = context.WithCancel(context.Background())

	go k.discovery()

	return k.discoveryChan, nil
}

func (k *mdnsDiscovery) Search() (dest []string, err error) { return k.tempQueue.Get(), nil }

func (k *mdnsDiscovery) discovery() {
	// expose mdns server
	mdnsServer, err := zeroconf.Register(k.nodeID, k.serviceName,
		"local.", k.nodePort, []string{"txtv=0", "lo=1", "la=2"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	k.mdnsServer = mdnsServer

	// fetch mDNS enabled raft nodes
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize mDNS resolver:", err.Error())
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case <-k.ctx.Done():
				return
			case entry := <-entries:
				if entry != nil {
					value := fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
					k.discoveryChan <- value
					k.tempQueue.Put(value)
				}
			}
		}
	}()

	ctx, cancel := context.WithTimeout(k.ctx, 15*time.Second)
	defer cancel()

	if err = resolver.Browse(ctx, k.serviceName, "local.", entries); err != nil {
		log.Printf("Error during mDNS lookup: %v", err)
	}
}

func (k *mdnsDiscovery) IsStatic() bool { return false }

func (k *mdnsDiscovery) Stop() {
	k.cancel()
	k.mdnsServer.Shutdown()
}
