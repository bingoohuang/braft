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