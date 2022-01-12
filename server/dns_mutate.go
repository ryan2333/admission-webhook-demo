package server

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	"github.com/wI2L/jsondiff"
	admission "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	defaultDnsConfig = &DnsConfig{}
	qos              = &Qos{}
)

type DnsConfig struct {
	Spec DnsConfigSpec `json:"spec,omitempty"`
}

type DnsConfigSpec struct {
	Nameservers []string `json:"nameServers,omitempty"`
	Options     []struct {
		Name  string `json:"name,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"options,omitempty"`
}

func MutateDnsConfig(req *admission.AdmissionRequest) (jsondiff.Patch, error) {
	kind := req.Kind.Kind
	raw := req.Object.Raw

	glog.Infoln("admission reqeust: ", string(raw), req.Operation)

	dnsConfig := corev1.PodDNSConfig{}

	if kind != "DnsConfig" && defaultDnsConfig == nil {
		return nil, nil
	} else {
		if len(defaultDnsConfig.Spec.Nameservers) != 0 {
			dnsConfig.Nameservers = defaultDnsConfig.Spec.Nameservers
		}
		if len(defaultDnsConfig.Spec.Options) != 0 {
			for _, v := range defaultDnsConfig.Spec.Options {
				dnsConfig.Options = append(dnsConfig.Options, corev1.PodDNSConfigOption{
					Name:  v.Name,
					Value: &v.Value,
				})
			}
		}
	}

	// glog.Infof("dnsconfig: nameservers[%v], options[%v]\n", defaultDnsConfig.Spec.Nameservers, dnsConfig.Options)

	switch kind {
	case "Deployment":
		deployment := appsv1.Deployment{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &deployment); err != nil {
			return nil, fmt.Errorf("deserialize [%s] object failed, err: %v\nrequest body: %v", kind, err.Error(), string(raw))
		}

		newDeployment := deployment.DeepCopy()
		podSpec := &newDeployment.Spec.Template.Spec

		podSpec.DNSConfig = &dnsConfig

		return jsondiff.Compare(deployment, newDeployment)

	case "Pod":
		pod := corev1.Pod{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
			return nil, fmt.Errorf("deserialize [%s] object failed, err: %v\nrequest body: %v", kind, err.Error(), string(raw))
		}
		newPod := pod.DeepCopy()
		newPod.Spec.DNSConfig = &dnsConfig

		if len(defaultDnsConfig.Spec.Nameservers) != 0 {
			newPod.Spec.DNSPolicy = corev1.DNSPolicy("None")
		}
		return jsondiff.Compare(pod, newPod)
	case "DnsConfig":
		if req.Operation == "DELETE" {
			defaultDnsConfig = &DnsConfig{}
			glog.Infoln("dnsconfig policy delete...")
			return nil, nil
		}

		if err := json.Unmarshal(raw, defaultDnsConfig); err != nil {
			return nil, fmt.Errorf("dnsconfig json unmarshal failed, err: %v\nrequest body: %v", err.Error(), string(raw))
		}

		glog.Infof("newDndConfig: nameservers[%v], options[%v]\n", defaultDnsConfig.Spec.Nameservers, defaultDnsConfig.Spec.Options)
		return nil, nil
	case "Qos":
		if req.Operation == "DELETE" {
			qos.Reset()
		}

		if err := qos.SetQos(raw); err != nil {
			return nil, fmt.Errorf("qos json unmarshal failed, err: %v\nrequest body: %v", err.Error(), string(raw))
		}

		glog.Infof("new qos: requests[cpu: %v, mem: %v], limits[cpu: %v, mem: %v]\n", qos.Spec.Requests.Cpu, qos.Spec.Requests.Mem, qos.Spec.Limits.Cpu, qos.Spec.Limits.Mem)
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupport Kind[%s], only support Deployment, Pod, DnsConfig", kind)
	}
}
