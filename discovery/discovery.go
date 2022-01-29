package discovery

// Discovery gives the interface to perform automatic Node discovery
type Discovery interface {
	// Name gives the name of the discovery.
	Name() string

	// Start is about to start the discovery method
	// it returns a channel where the node will consume node addresses ("IP:NodeRaftPort") until the channel gets closed
	Start(nodeID string, nodePort int) (chan string, error)

	// IsStatic indicates whether the actual discovery is static or dynamic.
	IsStatic() bool

	// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
	Stop()
}

type Searchable interface {
	Search() (dest []string, err error)
}
