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
You can create a simple BRaft Node with local mDNS discovery, an in-memory Map service and MsgPack as serializer(this
is the only one built-in at the moment)

```go
package main

import (
	"log"

	"github.com/bingoohuang/braft"
)

func main() {
	node, err := braft.NewNode()
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	if err := node.Start(); err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}

	node.RunHTTP()
}
```

1. use mDNS discovery: `GOLOG_STDOUT=true braft` on multiple nodes.
1. use static discovery: `GOLOG_STDOUT=true BRAFT_RPORT=15000 BRAFT_DISCOVERY="192.168.126.16,192.168.126.18,192.168.126.182" braft`  on multiple nodes.
