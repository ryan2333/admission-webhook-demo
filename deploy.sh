#!/bin/bash 
# kubectl -n webhook-demo create secret tls webhook-server-tls     --cert "webhook-server-tls.crt" --key "webhook-server-tls.key"

: ${1?'missing version'}

version=$1

kubectl delete -f doc/webhook.yaml
rm -f ./webhook-server \
&& CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-server \
&& docker build -t registry.ke.com/cloud-virtual/cloud-engine/webhook-server:${version} . \
&& docker push registry.ke.com/cloud-virtual/cloud-engine/webhook-server:${version} \
&& sed 's/VERSION/'${version}'/' doc/deployment.yaml.tmpl > doc/deployment.yaml

kubectl apply -f doc/deployment.yaml
sleep 5

kubectl apply -f doc/webhook.yaml

