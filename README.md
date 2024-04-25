An easy to use a customizable library to make your Go application Distributed, Highly available, Fault Tolerant etc...
using Hashicorp's [Raft](https://github.com/hashicorp/raft) library which implements the
[Raft Consensus Algorithm](https://raft.github.io/). Original fork
from [ksrichard/easyraft](https://github.com/ksrichard/easyraft)

## Features

- **Configure and start** a fully functional Raft node by writing ~10 lines of code
- **Automatic Node discovery** (nodes are discovering each other using Discovery method)
    1. **Built-in discovery methods**:
        1. **Static Discovery** (having a fixed list of node addresses)
        2. **mDNS Discovery** for local network node discovery
        3. **Kubernetes discovery**
- **Cloud Native** because of kubernetes discovery and easy to load balance features
- **Automatic forward to leader** - you can contact any node to perform operations; everything will be forwarded to the
  actual leader node
- **Node monitoring/removal** - the nodes are monitoring each other and if there are some failures, then the offline
  nodes get removed automatically from the cluster
- **Simplified state machine** - there is an already implemented generic state machine which handles the basic
  operations and routes requests to State Machine Services (see **Examples**)
- **All layers are customizable** - you can select or implement your own **State Machine Service, Message Serializer**
  and **Discovery Method**
- **gRPC transport layer** - the internal communications are done through gRPC based communication, if needed you can
  add your own services

**Note:** snapshots are not supported at the moment, will be handled at later point
**Note:** at the moment the communication between nodes is insecure, I recommend not exposing that port

## Get Started

You can create a simple BRaft Node with local mDNS discovery, an in-memory Map service and MsgPack as serializer(this is
the only one built-in at the moment)

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
}
```

1. use mDNS discovery: `braft` on multiple nodes.
2. use static discovery: `BRAFT_DISCOVERY="192.168.126.16,192.168.126.18,192.168.126.182" braft`  on
   multiple nodes.

## env VARIABLES

| NAME                    | ACRONYM | USAGE                                        | DEFAULT              | EXAMPLE                                                                                                                                              |
|-------------------------|---------|----------------------------------------------|----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| GOLOG_STDOUT            | N/A     | print log on stdout                          | false                | `export GOLOG_STDOUT=true`                                                                                                                           |
| BRAFT_DISCOVERY         | BDI     | discovery configuration                      | mdns                 | `export BRAFT_DISCOVERY="mdns:_braft._tcp"`<p>`export BRAFT_DISCOVERY="static:192.168.1.1,192.168.1.2,192.168.1.3"`<p>`export BRAFT_DISCOVERY="k8s"` |
| BRAFT_IP                | BIP     | specify the IP                               | first host IP        | `export BRAFT_IP=192.168.1.1`                                                                                                                        |
| BRAFT_IF                | BIF     | specify the IF name                          | N/A                  | `export BRAFT_IF=eth0`                                                                                                                               |
| BRAFT_RESTART_MIN       | N/A     | specify restart min wait if no leader        | 90s                  | `export BRAFT_RESTART_MIN=30s`                                                                                                                       |
| BRAFT_LEADER_STEADY     | N/A     | specify the delay time after leader changing | 60s                  | `export BRAFT_LEADER_STEADY=30s`                                                                                                                     |
| BRAFT_RPORT             | BRP     | specify the raft port                        | 15000                | `export BRAFT_RPORT=15000`                                                                                                                           |
| BRAFT_DPORT             | BDP     | specify the discovery port                   | $BRAFT_RPORT + 1     | `export BRAFT_DPORT=15001`                                                                                                                           |
| BRAFT_HPORT             | BHP     | specify the http port                        | $DRAFT_DPORT + 1     | `export BRAFT_HPORT=15002`                                                                                                                           |
| BRAFT_SLEEP             | BSL     | random sleep to startup raft cluster         | 10ms-15s             | `export BRAFT_SLEEP=100ms-3s`                                                                                                                        |
| MDNS_SERVICE            | MDS     | mDNS Service name (e.g. _http._tcp.)         | _braft._tcp,_windows | `export MDS=_braft._tcp,_windows`                                                                                                                    |
| K8S_NAMESPACE           | K8N     | k8s namespace                                | (empty)              | `export K8S_NAMESPACE=prod`                                                                                                                          |
| K8S_LABELS              | K8L     | service labels                               | (empty)              | `export K8S_LABELS=svc=braft`                                                                                                                        |
| K8S_PORTNAME            | K8P     | container tcp port name                      | (empty)              | `export K8S_PORTNAME=http`                                                                                                                           |
| K8S_SLEEP               | N/A     | k8s discovery sleep before start             | 15-30s               | `export K8S_SLEEP=30-50s`                                                                                                                            |
| DISABLE_GRPC_REFLECTION | DGR     | disable grpc reflection                      | off                  | `export DISABLE_GRPC_REFLECTION=off`                                                                                                                 |

## demo

### use static discovery

At localhost:

1. `BRAFT_RESTART_MIN=10s BRAFT_LEADER_STEADY=10s BRAFT_RPORT=15000 BRAFT_DISCOVERY="127.0.0.1:15000,127.0.0.1:16000,127.0.0.1:17000" braft`
2. `BRAFT_RESTART_MIN=10s BRAFT_LEADER_STEADY=10s BRAFT_RPORT=16000 BRAFT_DISCOVERY="127.0.0.1:15000,127.0.0.1:16000,127.0.0.1:17000" braft`
3. `BRAFT_RESTART_MIN=10s BRAFT_LEADER_STEADY=10s BRAFT_RPORT=17000 BRAFT_DISCOVERY="127.0.0.1:15000,127.0.0.1:16000,127.0.0.1:17000" braft`

At 3-different hosts:

1. `BRAFT_RPORT=15000 BRAFT_DISCOVERY="host1,host2,host3" braft`

### use mDNS discovery

1. `braft`
2. `braft` (same)
3. `braft` (same)

### use k8s discovery

1. `K8S_SLEEP=50-80s K8N=footstone BDI=k8s K8L=svc=braft ./braft`

### example /raft http rest api result

```sh
$ gurl :15002/raft
GET /raft? HTTP/1.1
Host: localhost:15002
Accept: application/json
Accept-Encoding: gzip, deflate
Content-Type: application/json
Gurl-Date: Tue, 08 Feb 2022 14:09:48 GMT
User-Agent: gurl/0.1.0


HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Date: Tue, 08 Feb 2022 14:09:48 GMT

{
  "CurrentLeader": true,
  "Discovery": "mdns://_braft._tcp",
  "Leader": "192.168.0.103:15000",
  "NodeNum": 3,
  "Nodes": [
    {
      "Leader": "192.168.0.103:15000",
      "ServerID": "hqJJRLsyNHBZeUliV0YxaVpwRHlSTlZzcjd6SDhuSG2lUnBvcnTNOpilRHBvcnTNOpmlSHBvcnTNOpqoSG9zdG5hbWWpbG9jYWxob3N0oklQka0xOTIuMTY4LjAuMTAz",
      "Address": "192.168.0.103:15000",
      "RaftState": "Leader",
      "RaftID": {
        "ID": "24pYyIbWF1iZpDyRNVsr7zH8nHm",
        "Rport": 15000,
        "Dport": 15001,
        "Hport": 15002,
        "Hostname": "localhost",
        "IP": [
          "192.168.0.103"
        ]
      },
      "DiscoveryNodes": [
        "192.168.0.103:17000",
        "192.168.0.103:15000",
        "192.168.0.103:16000"
      ],
      "StartTime": "2022-02-08T22:07:27.941626+08:00",
      "Duration": "2m20.548861052s",
      "GitCommit": "22b4c8a@2022-02-08T21:57:51+08:00",
      "BuildTime": "2022-02-08T22:06:22+0800",
      "GoVersion": "go1.17.6_darwin/amd64",
      "AppVersion": "1.0.0"
    },
    {
      "Leader": "192.168.0.103:15000",
      "ServerID": "hqJJRLsyNHBZejRnYjZRTFM3cmpINFBGMzkxcEY4Y1OlUnBvcnTNPoClRHBvcnTNPoGlSHBvcnTNPoKoSG9zdG5hbWWpbG9jYWxob3N0oklQka0xOTIuMTY4LjAuMTAz",
      "Address": "192.168.0.103:16000",
      "RaftState": "Follower",
      "RaftID": {
        "ID": "24pYz4gb6QLS7rjH4PF391pF8cS",
        "Rport": 16000,
        "Dport": 16001,
        "Hport": 16002,
        "Hostname": "localhost",
        "IP": [
          "192.168.0.103"
        ]
      },
      "DiscoveryNodes": [
        "192.168.0.103:17000",
        "192.168.0.103:16000",
        "192.168.0.103:15000"
      ],
      "StartTime": "2022-02-08T22:07:34.031526+08:00",
      "Duration": "2m14.45963851s",
      "GitCommit": "22b4c8a@2022-02-08T21:57:51+08:00",
      "BuildTime": "2022-02-08T22:06:22+0800",
      "GoVersion": "go1.17.6_darwin/amd64",
      "AppVersion": "1.0.0"
    },
    {
      "Leader": "192.168.0.103:15000",
      "ServerID": "hqJJRLsyNHBZemxxRWFxTHNuMHdxQ3p3clY4ZnJGZVWlUnBvcnTNQmilRHBvcnTNQmmlSHBvcnTNQmqoSG9zdG5hbWWpbG9jYWxob3N0oklQka0xOTIuMTY4LjAuMTAz",
      "Address": "192.168.0.103:17000",
      "RaftState": "Follower",
      "RaftID": {
        "ID": "24pYzlqEaqLsn0wqCzwrV8frFeU",
        "Rport": 17000,
        "Dport": 17001,
        "Hport": 17002,
        "Hostname": "localhost",
        "IP": [
          "192.168.0.103"
        ]
      },
      "DiscoveryNodes": [
        "192.168.0.103:16000",
        "192.168.0.103:15000",
        "192.168.0.103:17000"
      ],
      "StartTime": "2022-02-08T22:07:39.590406+08:00",
      "Duration": "2m8.90139164s",
      "GitCommit": "22b4c8a@2022-02-08T21:57:51+08:00",
      "BuildTime": "2022-02-08T22:06:22+0800",
      "GoVersion": "go1.17.6_darwin/amd64",
      "AppVersion": "1.0.0"
    }
  ]
}
```

```sh
# gurl :30010/raft
GET /raft? HTTP/1.1
Host: localhost:30010
Accept: application/json
Accept-Encoding: gzip, deflate
Content-Type: application/json
User-Agent: gurl/0.1.0


HTTP/1.1 200 OK
Date: Fri, 11 Feb 2022 03:58:50 GMT
Content-Type: application/json; charset=utf-8

{
  "CurrentLeader": false,
  "Discovery": "k8s://ns=footstone-common/labels=svc=braft/portName=",
  "Leader": "10.42.6.90:15000",
  "NodeNum": 3,
  "Nodes": [
    {
      "Leader": "10.42.6.90:15000",
      "ServerID": "hqJJRLsyNHdtNGh1WXJaS0dmS3VBdW80OHRaeHVWTUWlUnBvcnTNOpilRHBvcnTNOpmlSHBvcnTNOpqoSG9zdG5hbWW1YnJhZnQtZDliZmY0YjliLWt6ZGhxoklQkaoxMC40Mi42Ljky",
      "Address": "10.42.6.92:15000",
      "RaftState": "Follower",
      "RaftID": {
        "ID": "24wm4huYrZKGfKuAuo48tZxuVME",
        "Rport": 15000,
        "Dport": 15001,
        "Hport": 15002,
        "Hostname": "braft-d9bff4b9b-kzdhq",
        "IP": [
          "10.42.6.92"
        ]
      },
      "DiscoveryNodes": [
        "10.42.6.92",
        "10.42.6.90",
        "10.42.6.91"
      ],
      "StartTime": "2022-02-11T11:23:53.950293142+08:00",
      "Duration": "34m56.054300649s",
      "Rss": 57172,
      "RaftLogSum": 0,
      "Pid": 12,
      "GitCommit": "e4d9145@2022-02-11T10:50:34+08:00",
      "BuildTime": "2022-02-11T11:23:12+0800",
      "GoVersion": "go1.17.5_linux/amd64",
      "AppVersion": "1.0.0",
      "Pcpu": 2.7027028
    },
    {
      "Leader": "10.42.6.90:15000",
      "ServerID": "hqJJRLsyNHdtM2VKSEp5WGQ4RWYydDRHT0NWMmpXWE6lUnBvcnTNOpilRHBvcnTNOpmlSHBvcnTNOpqoSG9zdG5hbWW1YnJhZnQtZDliZmY0YjliLXhqYjJjoklQkaoxMC40Mi42Ljkx",
      "Address": "10.42.6.91:15000",
      "RaftState": "Follower",
      "RaftID": {
        "ID": "24wm3eJHJyXd8Ef2t4GOCV2jWXN",
        "Rport": 15000,
        "Dport": 15001,
        "Hport": 15002,
        "Hostname": "braft-d9bff4b9b-xjb2c",
        "IP": [
          "10.42.6.91"
        ]
      },
      "DiscoveryNodes": [
        "10.42.6.92",
        "10.42.6.90",
        "10.42.6.91"
      ],
      "StartTime": "2022-02-11T11:23:45.672142228+08:00",
      "Duration": "35m4.353394194s",
      "Rss": 58504,
      "RaftLogSum": 0,
      "Pid": 12,
      "GitCommit": "e4d9145@2022-02-11T10:50:34+08:00",
      "BuildTime": "2022-02-11T11:23:12+0800",
      "GoVersion": "go1.17.5_linux/amd64",
      "AppVersion": "1.0.0",
      "Pcpu": 2.739726
    },
    {
      "Leader": "10.42.6.90:15000",
      "ServerID": "hqJJRLsyNHdtMmdDV2lDbmI2SjRINFFydWxSeHZhZFSlUnBvcnTNOpilRHBvcnTNOpmlSHBvcnTNOpqoSG9zdG5hbWW1YnJhZnQtZDliZmY0YjliLWxxdnpuoklQkaoxMC40Mi42Ljkw",
      "Address": "10.42.6.90:15000",
      "RaftState": "Leader",
      "RaftID": {
        "ID": "24wm2gCWiCnb6J4H4QrulRxvadT",
        "Rport": 15000,
        "Dport": 15001,
        "Hport": 15002,
        "Hostname": "braft-d9bff4b9b-lqvzn",
        "IP": [
          "10.42.6.90"
        ]
      },
      "DiscoveryNodes": [
        "10.42.6.92",
        "10.42.6.90",
        "10.42.6.91"
      ],
      "StartTime": "2022-02-11T11:23:37.367006837+08:00",
      "Duration": "35m12.673937071s",
      "Rss": 57216,
      "RaftLogSum": 0,
      "Pid": 12,
      "GitCommit": "e4d9145@2022-02-11T10:50:34+08:00",
      "BuildTime": "2022-02-11T11:23:12+0800",
      "GoVersion": "go1.17.5_linux/amd64",
      "AppVersion": "1.0.0",
      "Pcpu": 2.631579
    }
  ]
}
```

```sh
# gurl http://a.b.c/rig-braft-service/raft
GET /rig-braft-service/raft? HTTP/1.1
Host: beta.isignet.cn:36131
Accept: application/json
Accept-Encoding: gzip, deflate
Content-Type: application/json
Gurl-Date: Wed, 16 Feb 2022 03:22:20 GMT
User-Agent: gurl/1.0.0


HTTP/1.1 200 OK
Server: nginx/1.19.2
Date: Wed, 16 Feb 2022 03:22:22 GMT
Content-Type: application/json; charset=utf-8
Connection: keep-alive
Vary: Accept-Encoding
Content-Encoding: gzip

{
  "CurrentLeader": true,
  "Discovery": "static://rig-braft-service-0.rig-braft-service,rig-braft-service-1.rig-braft-service,rig-braft-service-2.rig-braft-service",
  "Leader": "10.42.6.198:11469",
  "NodeNum": 3,
  "Nodes": [
    {
      "Leader": "10.42.6.198:11469",
      "ServerID": "hqJJRLsyNUFyZDFqclp2Qmt5bnBWRUI5aGNRS2ZIZGelUnBvcnTNLM2lRHBvcnTNLM6lSHBvcnTNLM-oSG9zdG5hbWWzcmlnLWJyYWZ0LXNlcnZpY2UtMqJJUJGrMTAuNDIuNi4xOTg",
      "Address": "10.42.6.198:11469",
      "RaftState": "Leader",
      "RaftID": {
        "ID": "25Ard1jrZvBkynpVEB9hcQKfHdg",
        "Rport": 11469,
        "Dport": 11470,
        "Hport": 11471,
        "Hostname": "rig-braft-service-2",
        "IP": [
          "10.42.6.198"
        ]
      },
      "DiscoveryNodes": [
        "rig-braft-service-0.rig-braft-service",
        "rig-braft-service-1.rig-braft-service",
        "rig-braft-service-2.rig-braft-service"
      ],
      "StartTime": "2022-02-16T11:06:55.5760445+08:00",
      "Duration": "15m27.069639007s",
      "Rss": 49672,
      "RaftLogSum": 0,
      "Pid": 12,
      "GitCommit": "ca3ff05@2022-02-16T11:03:28+08:00",
      "BuildTime": "2022-02-16T11:05:56+0800",
      "GoVersion": "go1.17.5_linux/amd64",
      "AppVersion": "1.2.0",
      "Pcpu": 2.3118222
    },
    {
      "Leader": "10.42.6.198:11469",
      "ServerID": "hqJJRLsyNUFyZGRuS1R3bDNDREdMUXpRU3h2U1EzekWlUnBvcnTNLM2lRHBvcnTNLM6lSHBvcnTNLM-oSG9zdG5hbWWzcmlnLWJyYWZ0LXNlcnZpY2UtMaJJUJGrMTAuNDIuNi4xOTk",
      "Address": "10.42.6.199:11469",
      "RaftState": "Follower",
      "RaftID": {
        "ID": "25ArddnKTwl3CDGLQzQSxvSQ3zE",
        "Rport": 11469,
        "Dport": 11470,
        "Hport": 11471,
        "Hostname": "rig-braft-service-1",
        "IP": [
          "10.42.6.199"
        ]
      },
      "DiscoveryNodes": [
        "rig-braft-service-0.rig-braft-service",
        "rig-braft-service-1.rig-braft-service",
        "rig-braft-service-2.rig-braft-service"
      ],
      "StartTime": "2022-02-16T11:07:00.867960857+08:00",
      "Duration": "15m21.77895389s",
      "Rss": 46216,
      "RaftLogSum": 0,
      "Pid": 12,
      "GitCommit": "ca3ff05@2022-02-16T11:03:28+08:00",
      "BuildTime": "2022-02-16T11:05:56+0800",
      "GoVersion": "go1.17.5_linux/amd64",
      "AppVersion": "1.2.0",
      "Pcpu": 1.1629363
    },
    {
      "Leader": "10.42.6.198:11469",
      "ServerID": "hqJJRLsyNUFyaUsyTXhNbDNjTjlIMUJ6MUY4TEZDZTClUnBvcnTNLM2lRHBvcnTNLM6lSHBvcnTNLM-oSG9zdG5hbWWzcmlnLWJyYWZ0LXNlcnZpY2UtMKJJUJGrMTAuNDIuMi4yMTA",
      "Address": "10.42.2.210:11469",
      "RaftState": "Follower",
      "RaftID": {
        "ID": "25AriK2MxMl3cN9H1Bz1F8LFCe0",
        "Rport": 11469,
        "Dport": 11470,
        "Hport": 11471,
        "Hostname": "rig-braft-service-0",
        "IP": [
          "10.42.2.210"
        ]
      },
      "DiscoveryNodes": [
        "rig-braft-service-0.rig-braft-service",
        "rig-braft-service-1.rig-braft-service",
        "rig-braft-service-2.rig-braft-service"
      ],
      "StartTime": "2022-02-16T11:07:37.501965271+08:00",
      "Duration": "14m45.146689237s",
      "Rss": 43672,
      "RaftLogSum": 0,
      "Pid": 13,
      "GitCommit": "ca3ff05@2022-02-16T11:03:28+08:00",
      "BuildTime": "2022-02-16T11:05:56+0800",
      "GoVersion": "go1.17.5_linux/amd64",
      "AppVersion": "1.2.0",
      "Pcpu": 1.5182345
    }
  ]
}
```

## grpcui

1. check the env `DISABLE_GRPC_REFLECTION` is not enabled.
2. install [grpcui](https://github.com/fullstorydev/grpcui)
3. `grpcui -plaintext localhost:15000`

![image](https://user-images.githubusercontent.com/1940588/154780806-bf7b88e3-27b8-416e-bbab-34be474e2db0.png)
