An easy to use customizable library to make your Go application Distributed, Highly available, Fault Tolerant etc...
using Hashicorp's [Raft](https://github.com/hashicorp/raft) library which implements the
[Raft Consensus Algorithm](https://raft.github.io/). Original fork from [ksrichard/easyraf](https://github.com/ksrichard/easyraft)

Features
---

- **Configure and start** a fully functional Raft node by writing ~10 lines of code
- **Automatic Node discovery** (nodes are discovering each other using Discovery method)
    1. **Built-in discovery methods**:
        1. **Static Discovery** (having a fixed list of nodes addresses)
        2. **mDNS Discovery** for local network node discovery
        3. **Kubernetes discovery**
- **Cloud Native** because of kubernetes discovery and easy to load balance features
- **Automatic forward to leader** - you can contact any node to perform operations, everything will be forwarded to the
  actual leader node
- **Node monitoring/removal** - the nodes are monitoring each other and if there are some failures then the offline
  nodes get removed automatically from cluster
- **Simplified state machine** - there is an already implemented generic state machine which handles the basic
  operations and routes requests to State Machine Services (see **Examples**)
- **All layers are customizable** - you can select or implement your own **State Machine Service, Message Serializer**
  and **Discovery Method**
- **gRPC transport layer** - the internal communications are done through gRPC based communication, if needed you can
  add your own services

**Note:** snapshots are not supported at the moment, will be handled at later point
**Note:** at the moment the communication between nodes are insecure, I recommend to not expose that port

Get Started
---
You can create a simple EasyRaft Node with local mDNS discovery, an in-memory Map service and MsgPack as serializer(this
is the only one built-in at the moment)

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bingoohuang/braft"
	"github.com/bingoohuang/braft/fsm"
)

func main() {
	log.Printf("Starting, rport:%d, p:%d, hport:%d, discovery:%s",
		braft.EnvRport, braft.EnvDport, braft.EnvHport, braft.EnvDiscoveryMethod.Name())
	node, err := braft.NewNode(braft.WithServices(fsm.NewMemMapService()))
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	stoppedCh, err := node.Start()
	if err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}
	defer node.Stop()

	// wait for interruption/termination
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	<-stoppedCh
}
```
