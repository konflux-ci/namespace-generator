package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InParameters struct {
	LabelSelector metav1.LabelSelector `json:"labelSelector"`
}

type Input struct {
	Parameters InParameters `json:"parameters"`
}

type GenerateRequest struct {
	ApplicationSetName string `json:"applicationSetName"`
	Input              Input  `json:"input"`
}

type OutParameters struct {
	Namespace string `json:"namespace"`
}

type Output struct {
	Parameters []OutParameters `json:"parameters"`
}

type GenerateResponse struct {
	Output Output `json:"output"`
}
