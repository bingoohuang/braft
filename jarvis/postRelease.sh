#!/usr/bin/env bash

/usr/local/ops/ops-docker/script/deploy --src braft --dockerTemplateDir common --port 15002 --serviceType NodePort --nodePort 30010  --cmd "K8S_SLEEP=30-50s BDI=k8s K8L=svc=braft ./braft" --requestCPU 100m --limitCPU 2 --replicaCount 3   --namespace footstone-common  --labels "svc=braft" -serviceAccountName braftdemo --extraK8sFiles="k8s-rbac.yaml"
