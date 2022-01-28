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

// Name gives the name of the discovery.
func (d *staticDiscovery) Name() string { return "static:" + strings.Join(d.Peers, ",") }

func (d *staticDiscovery) Stop() {}

func (d *staticDiscovery) IsStatic() bool { return true }

func (d *staticDiscovery) Start(_ string, _ int) (chan string, error) {
	go func() {
		for _, peer := range d.Peers {
			d.discoveryChan <- peer
		}
		close(d.discoveryChan)
	}()
	return d.discoveryChan, nil
}
