package discovery

import (
	"context"
	"strings"
)

type staticDiscovery struct {
	discoveryChan chan string
	Peers         []string
}

func NewStaticDiscovery(peers []string) Discovery {
	return &staticDiscovery{
		Peers:         peers,
		discoveryChan: make(chan string),
	}
}

func (k *staticDiscovery) Search() (dest []string, err error) {
	return k.Peers, nil
}

// Name gives the name of the discovery.
func (k *staticDiscovery) Name() string { return "static://" + strings.Join(k.Peers, ",") }

func (k *staticDiscovery) Start(ctx context.Context, nodeID string, nodePort int) (chan string, error) {
	go func() {
		for _, peer := range k.Peers {
			select {
			case <-ctx.Done():
				return
			case k.discoveryChan <- peer:
			}
		}
	}()
	return k.discoveryChan, nil
}
