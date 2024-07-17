package discovery

import (
	"context"
	"fmt"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/grandcat/zeroconf"
	"log"
)

type mdnsDiscovery struct {
	discoveryChan chan string
	tempQueue     *util.UniqueQueue
	nodeID        string
	serviceName   string
	nodePort      int
}

func NewMdnsDiscovery(serviceName string) Discovery {
	return &mdnsDiscovery{
		// https://github.com/grandcat/zeroconf
		// Multiple subtypes may be added to service name, separated by commas.
		// e.g. _workstation._tcp,_windows has subtype _windows.
		serviceName:   serviceName,
		discoveryChan: make(chan string),
		tempQueue:     util.NewUniqueQueue(100),
	}
}

// Name gives the name of the discovery.
func (k *mdnsDiscovery) Name() string { return "mdns://" + k.serviceName }

func (k *mdnsDiscovery) Start(ctx context.Context, nodeID string, nodePort int) (chan string, error) {
	k.nodeID, k.nodePort = ss.Left(nodeID, 27), nodePort

	go k.discovery(ctx)

	return k.discoveryChan, nil
}

func (k *mdnsDiscovery) Search() (dest []string, err error) { return k.tempQueue.Get(), nil }

func (k *mdnsDiscovery) discovery(ctx context.Context) {
	// expose mdns server
	mdnsServer, err := zeroconf.Register(k.nodeID, k.serviceName,
		"local.", k.nodePort, []string{"txtv=0", "lo=1", "la=2"}, nil)
	if err != nil {
		log.Fatal(err)
	}

	// fetch mDNS enabled raft nodes
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize mDNS resolver:", err.Error())
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go k.receive(ctx, mdnsServer, entries)

	if err = resolver.Browse(ctx, k.serviceName, "local.", entries); err != nil {
		log.Printf("Error during mDNS lookup: %v", err)
	}
}

func (k *mdnsDiscovery) receive(ctx context.Context, mdnsServer *zeroconf.Server, entries chan *zeroconf.ServiceEntry) {
	defer mdnsServer.Shutdown()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-entries:
			if !ok {
				break
			}

			value := fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
			k.discoveryChan <- value
			k.tempQueue.Put(value)
		}
	}
}
