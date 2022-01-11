package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/golang/glog"
	admission "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	jsonContentType = `application/json`
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	podResource           = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type admitFunc func(*admission.AdmissionRequest) ([]patchOperation, error)

func applySecurityDefaults(req *admission.AdmissionRequest) ([]patchOperation, error) {
	if req.Resource != podResource {
		log.Printf("expect resource to be %s", &podResource)
		return nil, nil
	}

	// parse the pod Object
	raw := req.Object.Raw
	pod := corev1.Pod{}

	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return nil, fmt.Errorf("deserialize pod object failed, err: %v", err.Error())
	}

	var containers []corev1.Container
	var patches []patchOperation

	for _, c := range pod.Spec.Containers {
		port := 80
		if len(c.Ports) != 0 {
			port = int(c.Ports[0].ContainerPort)
		}
		if c.ReadinessProbe == nil {
			c.ReadinessProbe = &corev1.Probe{
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/",
						Port: intstr.FromInt(int(port)),
					},
				},
				InitialDelaySeconds: 60,
				TimeoutSeconds:      5,
				FailureThreshold:    3,
				SuccessThreshold:    1,
				PeriodSeconds:       10,
			}

			// patch := patchOperation{
			// 	Op:   "add",
			// 	Path: fmt.Sprintf("/spec/containers/%s/readinessProbe", c.Name),
			// 	Value: map[string]interface{}{
			// 		"failureThreshold": 3,
			// 		"httpGet": map[string]interface{}{
			// 			"path": "/",
			// 			"port": port,
			// 		},
			// 		"initialDelaySeconds": 60,
			// 		"periodSeconds":       15,
			// 		"successThreshold":    1,
			// 		"timeoutSeconds":      5,
			// 	},
			// }
			// patches = append(patches, patch)
		}

		if c.LivenessProbe == nil {
			// patch := patchOperation{
			// 	Op:   "add",
			// 	Path: "",
			// 	Value: map[string]interface{}{
			// 		"failureThreshold": 3,
			// 		"httpGet": map[string]interface{}{
			// 			"path": "/",
			// 			"port": port,
			// 		},
			// 		"initialDelaySeconds": 60,
			// 		"periodSeconds":       15,
			// 		"successThreshold":    1,
			// 		"timeoutSeconds":      5,
			// 	},
			// }
			// patches = append(patches, patch)
			c.LivenessProbe = &corev1.Probe{
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/",
						Port: intstr.FromInt(int(port)),
					},
				},
				InitialDelaySeconds: 60,
				TimeoutSeconds:      5,
				FailureThreshold:    3,
				SuccessThreshold:    1,
				PeriodSeconds:       10,
			}

		}
		containers = append(containers, c)
	}
	patches = append(patches, patchOperation{
		Op:    "replace",
		Path:  "/spec/containers",
		Value: containers,
	})
	return patches, nil
}

func admitFuncHandler(admit admitFunc) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		serveAdmitFunc(rw, r, admit)
	})
}

func serveAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	log.Println("Handling webhook request...")
	var writeErr error

	if bytes, err := doServeAdmitFunc(w, r, admit); err != nil {
		log.Printf("Error handling webhook request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		log.Print("webhook request handled successfully...")
		_, writeErr = w.Write(bytes)
	}

	if writeErr != nil {
		log.Printf("Could not write response: %v", writeErr.Error())
	}
}

func doServeAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) ([]byte, error) {
	// validation request method, only support POST with a body and json content type
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil, fmt.Errorf("invalid method %s, only POST requests are allowed", r.Method)
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("read request body failed, err: %v", err.Error())
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("unsupport content type: %s, only support %s", contentType, jsonContentType)
	}

	log.Println("request body: ", string(body))

	// Parse the admissionReview request
	var ar admission.AdmissionReview

	if _, _, err := universalDeserializer.Decode(body, nil, &ar); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("deserializer request body failed, err: %v", err.Error())
	} else if ar.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("invalid admission review: request is nil")
	}

	// construct the adminssionReview response

	aw := admission.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admission.AdmissionResponse{
			UID: ar.Request.UID,
		},
	}

	// apply the admit() function only for non-kubernetes namespace. For objects in Kubernetes namespaces, return an empty set of patch operations
	var patchOps []patchOperation
	if !isKubeNamespace(ar.Request.Namespace) {
		patchOps, err = admit(ar.Request)
	}

	if err != nil {
		// if the handler returned error, incorporate the error message into response and deny the object creation
		aw.Response.Allowed = false
		aw.Response.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else {
		// otherwise, encode the patch operations to JSON and return a positive response
		patchBytes, err := json.Marshal(patchOps)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil, fmt.Errorf("patchOps json marshal failed, err: %v", err.Error())
		}

		glog.Info("admission response body: ", string(patchBytes))
		aw.Response.Allowed = true
		aw.Response.Patch = patchBytes

		// announce that we are returning a json patch
		aw.Response.PatchType = new(admission.PatchType)
		*aw.Response.PatchType = admission.PatchTypeJSONPatch
	}

	bytes, err := json.Marshal(&aw)
	if err != nil {
		return nil, fmt.Errorf("response json marshal failed, err: ", err.Error())
	}

	return bytes, nil
}

func isKubeNamespace(ns string) bool {
	return ns == metav1.NamespacePublic || ns == metav1.NamespaceSystem || ns == "webhook-demo"
}
