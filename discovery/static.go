package discovery

import "strings"

type StaticDiscovery struct {
	Peers         []string
	discoveryChan chan string
	stopChan      chan bool
}

func NewStaticDiscovery(peers []string) DiscoveryMethod {
	return &StaticDiscovery{
		Peers:         peers,
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
	}
}

// Name gives the name of the discovery.
func (d *StaticDiscovery) Name() string {
	return "Static:" + strings.Join(d.Peers, ",")
}

func (d *StaticDiscovery) SupportsNodeAutoRemoval() bool {
	return false
}

func (d *StaticDiscovery) Start(_ string, _ int) (chan string, error) {
	go func() {
		for _, peer := range d.Peers {
			d.discoveryChan <- peer
		}
	}()
	return d.discoveryChan, nil
}

func (d *StaticDiscovery) Stop() {
	d.stopChan <- true
}
