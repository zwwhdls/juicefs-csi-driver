package v1

import configv1 "k8s.io/client-go/applyconfigurations/meta/v1"

type JuiceMountApplyConfiguration struct {
	configv1.TypeMetaApplyConfiguration    `json:",inline"`
	*configv1.ObjectMetaApplyConfiguration `json:"metadata,omitempty"`
	Spec                                   *JuiceMountSpecApplyConfiguration `json:"spec,omitempty"`
}

type JuiceMountSpecApplyConfiguration struct {
	Refs int `json:"refs"`
}
