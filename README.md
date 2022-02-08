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
1. use static discovery: `GOLOG_STDOUT=true BRAFT_DISCOVERY="192.168.126.16,192.168.126.18,192.168.126.182" braft`  on multiple nodes.


## env VARIABLES

| NAME            | ACRONYM | USAGE                                | DEFAULT          | EXAMPELE                                                                                                                                             |
|-----------------|---------|--------------------------------------|------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| GOLOG_STDOUT    | N/A     | print log on stdout                  | N/A              | `export GOLOG_STDOUT=true`                                                                                                                           |
| BRAFT_DISCOVERY | BDI     | discovery configuration              | mdns:_braft._tcp | `export BRAFT_DISCOVERY="mdns:_braft._tcp"`<p>`export BRAFT_DISCOVERY="static:192.168.1.1,192.168.1.2,192.168.1.3"`<p>`export BRAFT_DISCOVERY="k8s"` |
| BRAFT_IP        | BIP     | specify the IP                       | first host IP    | `export BRAFT_IP=192.168.1.1`                                                                                                                        |
| BRAFT_IF        | BIF     | specify the IF name                  | N/A              | `export BRAFT_IF=eth0`                                                                                                                               |
| BRAFT_RPORT     | BRP     | specify the raft port                | 15000            | `export BRAFT_RPORT=15000`                                                                                                                           |
| BRAFT_DPORT     | BDP     | specify the discovery port           | BRAFT_RPORT+1    | `export BRAFT_DPORT=15001`                                                                                                                           |
| BRAFT_HPORT     | BHP     | specify the http port                | BRAFT_DPORT+1    | `export BRAFT_HPORT=15002`                                                                                                                           |
| BRAFT_SLEEP     | BSL     | random sleep to startup raft cluster | 100ms-3s         | `export BRAFT_SLEEP=100ms-3s`                                                                                                                        |
| K8S_NAMESPACE   | K8N     | k8s namespace                        | (empty)          | `export K8S_NAMESPACE=prod`                                                                                                                          |
| K8S_LABELS      | K8L     | service labels                       | (empty)          | `export K8S_LABELS=svc=braft`                                                                                                                        |
| K8S_PORTNAME    | K8P     | container tcp port name              | (empty)          | `export K8S_PORTNAME=http`                                                                                                                           |
| K8S_SLEEP       | N/A     | k8s discovery sleep before start     | 15-30s           | `export K8S_SLEEP=30-50s`                                                                                                                            |

## demo

```sh
$ gurl :15002/raft
GET /raft? HTTP/1.1
Host: localhost:15002
Accept: application/json
Accept-Encoding: gzip, deflate
Content-Type: application/json
User-Agent: gurl/0.1.0


HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Date: Fri, 28 Jan 2022 10:12:23 GMT
Content-Length: 1644

{
  "CurrentLeader": true,
  "Discovery": "mdns:_braft._tcp",
  "Leader": "192.168.162.88:15000",
  "NodeNum": 3,
  "Nodes": [
    {
      "Leader": "192.168.162.88:15000",
      "ServerID": "hqJJRLsyNEsxeWM4ZUw2T3pMM0xDaUk1MHFaNGZFTE2lUnBvcnTNOpilRHBvcnTNOpmlSHBvcnTNOpqoSG9zdG5hbWWlYm9nb26iSVCUrjE5Mi4xNjguMTYyLjg4rjE5Mi4xNjguMjE3LjE5qzE3Mi4xNi42Ny4xrTE5Mi4xNjguMjI4LjE",
      "Address": "192.168.162.88:15000",
      "RaftState": "Leader",
      "RaftPort": 15000,
      "DiscoveryPort": 15001,
      "HTTPPort": 15002,
      "RaftID": {
        "ID": "24K1yc8eL6OzL3LCiI50qZ4fELM",
        "Rport": 15000,
        "Dport": 15001,
        "Hport": 15002,
        "Hostname": "bogon",
        "IP": [
          "192.168.162.88",
          "192.168.217.19",
          "172.16.67.1",
          "192.168.228.1"
        ]
      }
    },
    {
      "Leader": "192.168.162.88:15000",
      "ServerID": "hqJJRLsyNEsxelYzcjJMeXZ3MmkzWmQxNTQ2TmtFcWmlUnBvcnTN3_2lRHBvcnTN3_6lSHBvcnTN3_-oSG9zdG5hbWWlYm9nb26iSVCUrjE5Mi4xNjguMTYyLjg4rjE5Mi4xNjguMjE3LjE5qzE3Mi4xNi42Ny4xrTE5Mi4xNjguMjI4LjE",
      "Address": "192.168.217.19:57341",
      "RaftState": "Follower",
      "RaftPort": 57341,
      "DiscoveryPort": 57342,
      "HTTPPort": 57343,
      "RaftID": {
        "ID": "24K1zV3r2Lyvw2i3Zd1546NkEqi",
        "Rport": 57341,
        "Dport": 57342,
        "Hport": 57343,
        "Hostname": "bogon",
        "IP": [
          "192.168.162.88",
          "192.168.217.19",
          "172.16.67.1",
          "192.168.228.1"
        ]
      }
    },
    {
      "Leader": "192.168.162.88:15000",
      "ServerID": "hqJJRLsyNEsyMDJNcGZ5d1BNNk1aOTEzSkhyeUVJb3WlUnBvcnTN4BqlRHBvcnTN4BulSHBvcnTN4ByoSG9zdG5hbWWlYm9nb26iSVCUrjE5Mi4xNjguMTYyLjg4rjE5Mi4xNjguMjE3LjE5qzE3Mi4xNi42Ny4xrTE5Mi4xNjguMjI4LjE",
      "Address": "192.168.217.19:57370",
      "RaftState": "Follower",
      "RaftPort": 57370,
      "DiscoveryPort": 57371,
      "HTTPPort": 57372,
      "RaftID": {
        "ID": "24K202MpfywPM6MZ913JHryEIou",
        "Rport": 57370,
        "Dport": 57371,
        "Hport": 57372,
        "Hostname": "bogon",
        "IP": [
          "192.168.162.88",
          "192.168.217.19",
          "172.16.67.1",
          "192.168.228.1"
        ]
      }
    }
  ]
}
```
