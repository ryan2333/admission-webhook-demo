package server

import (
	"fmt"

	"github.com/wI2L/jsondiff"
	admission "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func MutateDnsConfig(req *admission.AdmissionRequest) (jsondiff.Patch, error) {
	kind := req.Kind.Kind
	raw := req.Object.Raw

	ndots := "1"
	timeout := "1"

	switch kind {
	case "Deployment":
		deployment := appsv1.Deployment{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &deployment); err != nil {
			return nil, fmt.Errorf("deserialize [%s] object failed, err: %v\nrequest body: %v", kind, err.Error(), string(raw))
		}

		newDeployment := deployment.DeepCopy()
		podSpec := &newDeployment.Spec.Template.Spec

		podSpec.DNSConfig = &corev1.PodDNSConfig{
			Options: []corev1.PodDNSConfigOption{
				{Name: "ndots", Value: &ndots},
				{Name: "timeout", Value: &timeout},
			},
		}
		return jsondiff.Compare(deployment, newDeployment)

	case "Pod":
		pod := corev1.Pod{}
		if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
			return nil, fmt.Errorf("deserialize [%s] object failed, err: %v\nrequest body: %v", kind, err.Error(), string(raw))
		}
		newPod := pod.DeepCopy()
		newPod.Spec.DNSConfig = &corev1.PodDNSConfig{
			Options: []corev1.PodDNSConfigOption{
				{Name: "ndots", Value: &ndots},
				{Name: "timeout", Value: &timeout},
			},
		}
		return jsondiff.Compare(pod, newPod)
	default:
		return nil, fmt.Errorf("unsupport Kind[%s], only support Deployment or Pod", kind)
	}
}
