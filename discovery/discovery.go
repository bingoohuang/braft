package discovery

import "context"

// Discovery gives the interface to perform automatic Node discovery
type Discovery interface {
	// Name gives the name of the discovery.
	Name() string

	// Start is about to start the discovery method
	// it returns a channel where the node will consume node addresses ("IP:NodeRaftPort") until the channel gets closed
	Start(ctx context.Context, nodeID string, nodePort int) (chan string, error)

	Search() (dest []string, err error)
}
