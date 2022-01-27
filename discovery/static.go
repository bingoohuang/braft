package discovery

import "strings"

type staticDiscovery struct {
	Peers         []string
	discoveryChan chan string
	stopChan      chan bool
}

func NewStaticDiscovery(peers []string) Method {
	return &staticDiscovery{
		Peers:         peers,
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
	}
}

// Name gives the name of the discovery.
func (d *staticDiscovery) Name() string {
	return "static:" + strings.Join(d.Peers, ",")
}

func (d *staticDiscovery) SupportsNodeAutoRemoval() bool { return false }

func (d *staticDiscovery) Start(_ string, _ int) (chan string, error) {
	go func() {
		for _, peer := range d.Peers {
			d.discoveryChan <- peer
		}
	}()
	return d.discoveryChan, nil
}

func (d *staticDiscovery) Stop() {
	d.stopChan <- true
}
