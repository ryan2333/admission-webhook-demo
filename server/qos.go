package server

import "encoding/json"

type Qos struct {
	Spec *QosSpec `json:"spec,omitempty"`
}

type QosSpec struct {
	Requests *QosResource `json:"requests"`
	Limits   *QosResource `json:"limits"`
}

type QosResource struct {
	Cpu string `json:"cpu,omitempty"`
	Mem string `json:"mem,omitempty"`
}

func (q *Qos) SetQos(raw []byte) error {
	return json.Unmarshal(raw, q)
}

func (q *Qos) Reset() {
	q.Spec = &QosSpec{}
}
