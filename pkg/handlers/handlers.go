package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"sigs.k8s.io/controller-runtime/pkg/client"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konflux-ci/namespace-generator/pkg/api/v1alpha1"
)

type K8sClientFactory func(echo.Logger) (client.Reader, error)

type GetParamsHandler struct {
	k8sClientFactory K8sClientFactory
}

func NewGetParamsHandler(k8sClientFactory K8sClientFactory) *GetParamsHandler {
	return &GetParamsHandler{k8sClientFactory: k8sClientFactory}
}

// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch;create;update;patch
func (gp *GetParamsHandler) GetParams(c echo.Context) error {
	generateRequest := &v1alpha1.GenerateRequest{}
	err := decodeJson(c.Request().Body, generateRequest)

	if err != nil {
		c.Logger().Errorf("Failed to parse request body, %s", err)
		return c.NoContent(http.StatusBadRequest)
	}

	selector, err := metav1.LabelSelectorAsSelector(&generateRequest.Input.Parameters.LabelSelector)
	if err != nil {
		c.Logger().Errorf("Failed to parse label selector %s, %s", err)
		return c.NoContent(http.StatusBadRequest)
	}

	cl, err := gp.k8sClientFactory(c.Logger())
	if err != nil {
		c.Logger().Errorf("Failed to get k8s client: %s", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	namespaceList := &core.NamespaceList{}
	err = cl.List(
		context.Background(),
		namespaceList,
		&client.ListOptions{LabelSelector: selector},
	)
	if err != nil {
		c.Logger().Errorf("Failed to list namespaces, %s", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	generateResponse := &v1alpha1.GenerateResponse{}
	for _, namespace := range namespaceList.Items {
		generateResponse.Output.Parameters = append(
			generateResponse.Output.Parameters,
			v1alpha1.OutParameters{
				Namespace: namespace.Name,
			},
		)
	}

	return c.JSON(http.StatusOK, generateResponse)
}

func decodeJson(input io.ReadCloser, v any) error {
	// Can't use Echo's Bind method since it allows UnknownFields
	defer input.Close()
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
