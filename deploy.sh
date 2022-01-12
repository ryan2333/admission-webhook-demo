: ${1?'missing imagename:tag'}

version=$1

imageName="registry.ke.com/cloud-virtual/cloud-engine/${version}"

kubectl delete -f docs/webhook.yaml
kubectl delete deployment nginx-test
rm -f ./webhook-server \
&& CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-server \
&& docker build -t ${imageName} . \
&& docker push ${imageName} \
&& sed "s#IMAGENAME#${imageName}#" docs/deployment.yaml.tmpl > docs/deployment.yaml

if [ "$?" -eq 0 ];then
kubectl apply -f docs/deployment.yaml
sleep 5

kubectl apply -f docs/webhook.yaml


sleep 10;
kubectl create deployment nginx-test --image=nginx:1.18 --replicas=1

fi