package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/wI2L/jsondiff"
	admission "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

type admitFunc func(*admission.AdmissionRequest) (jsondiff.Patch, error)

func AdmitFuncHandle(admit admitFunc) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		serveAdmitFunc(rw, r, admit)
	})
}

func serveAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) {

	var writeErr error
	defer func() {
		if writeErr != nil {
			glog.Errorln("Write response failed, err: %v", writeErr)
		}
	}()

	glog.Infoln("====begin handling webhook request====")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, writeErr = w.Write([]byte(fmt.Sprintf("invalid method %s, only allowed POST", r.Method)))
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_, writeErr = w.Write([]byte(fmt.Sprintf("read reqeust body failed, err: %v", err.Error())))
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		_, writeErr = w.Write([]byte(fmt.Sprintf("read reqeust body failed, err: %v", err.Error())))
		return
	}

	var admissionRequestReview, admissionResponseReview admission.AdmissionReview

	if _, _, err := universalDeserializer.Decode(body, nil, &admissionRequestReview); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, writeErr = w.Write([]byte(fmt.Sprintf("request adminReview decode failed, err: %v", err.Error())))
		return
	} else if admissionRequestReview.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		_, writeErr = w.Write([]byte(fmt.Sprintf("invaild request adminReview, request is nil")))
		return
	}

	admissionResponseReview = admission.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admission.AdmissionResponse{
			UID:     admissionRequestReview.Request.UID,
			Allowed: true,
			PatchType: func() *admission.PatchType {
				pt := admission.PatchTypeJSONPatch
				return &pt
			}(),
		},
	}

	var patches jsondiff.Patch
	if !isKubeNamespace(admissionRequestReview.Request.Namespace) {
		subPatches, err := admit(admissionRequestReview.Request)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr = w.Write([]byte(fmt.Sprintf("get patches failed, err: %v", err.Error())))
			return
		} else {
			patches = append(patches, subPatches...)
		}
	}

	bytes, err := json.MarshalIndent(patches, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(fmt.Sprintf("patches json marshal failed, err: %v\npatches: %s", err.Error(), string(bytes))))
		return
	}

	admissionResponseReview.Response.Patch = bytes

	reviewBytes, err := json.Marshal(&admissionResponseReview)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(fmt.Sprintf("patches json marshal failed, err: %v\nadmissionResponseReview: %s", err.Error(), string(reviewBytes))))
		return
	}
	_, writeErr = w.Write(reviewBytes)

	glog.Infoln("====ended handling webhook request====")
}

func isKubeNamespace(ns string) bool {
	return ns == metav1.NamespacePublic || ns == metav1.NamespaceSystem
}
