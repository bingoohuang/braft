package discovery

import "strings"

type staticDiscovery struct {
	Peers         []string
	discoveryChan chan string
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

func (k *staticDiscovery) Stop() {}

func (k *staticDiscovery) Start(_ string, _ int) (chan string, error) {
	go func() {
		for _, peer := range k.Peers {
			k.discoveryChan <- peer
		}
		close(k.discoveryChan)
	}()
	return k.discoveryChan, nil
}
