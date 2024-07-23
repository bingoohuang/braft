# Simple HTTP-based key-value store example

This example demonstrates how easily you can implement an HTTP-based in-memory distributed
key value store using BRaft.

Usage
---
1. Build example:
   1. `go install ./...`
2. Run nodes locally:
   1. `BRAFT_RPORT=15000 braft`
   2. `BRAFT_RPORT=16000 braft`
   3. `BRAFT_RPORT=17000 braft`
3. Put value on any node:
   1. `curl -X POST 'http://localhost:15002/kv?map=test&k=somekey&v=somevalue'`
4. Get value from all the nodes:
   1. `curl 'http://localhost:15002/kv?map=test&k=somekey'`
   2. `curl 'http://localhost:16002/kv?map=test&k=somekey'`
   3. `curl 'http://localhost:17002/kv?map=test&k=somekey'`
5. Distribute some items `gurl POST :15002/distribute`

本机启动两个节点:

1. `BRAFT_DISCOVERY=:15001,:16001 BRAFT_RPORT=15000 braft`
2. `BRAFT_DISCOVERY=:15001,:16001 BRAFT_RPORT=16000 braft`

防火墙调整规则，模拟脑裂:

1. 从防火墙拒绝对端的访问: `iptables -A INPUT -s 172.16.25.57 -j DROP`
2. 查询规则: `iptables -L -n --line-numbers`
3. 删除规则: `iptables -D INPUT 1`
