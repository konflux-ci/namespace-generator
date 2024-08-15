package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

// TODO: Support multiple clusters?
func getClientOrDie(logger echo.Logger) client.Reader {
	cfg, err := config.GetConfig()
	if err != nil {
		logger.Fatal(err)
	}

	cl, err := cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		logger.Fatal(err)
	}
	go cl.Start(context.TODO())

	return cl
}

func decodeJson(input io.ReadCloser, v any) error {
	// Can't use Echo's Bind method since it allows UnknownFields
	defer input.Close()
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

func getKeyPath() string {
	keyPath := os.Getenv("NS_GEN_KEY_PATH")
	if len(keyPath) == 0 {
		return "/mnt/key"
	}

	return keyPath
}

func main() {
	e := echo.New()
	e.Logger.SetLevel(log.INFO)
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	// TODO: add metrics

	// TODO: Add proper flags
	keyPath := getKeyPath()
	group := e.Group("/api")
	group.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		validKey, err := os.ReadFile(keyPath)
		if err != nil {
			panic(fmt.Sprintf("Failed to read key file, %s\n", err.Error()))
		}
		return subtle.ConstantTimeCompare([]byte(key), validKey) == 1, nil
	}))
	
	cl := getClientOrDie(e.Logger)

	e.POST("/api/v1/getparams.execute", func(c echo.Context) error {
		generateRequest := &GenerateRequest{}
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

		generateResponse := &GenerateResponse{}
		for _, namespace := range namespaceList.Items {
			generateResponse.Output.Parameters = append(
				generateResponse.Output.Parameters,
				OutParameters{
					Namespace: namespace.Name,
				},
			)
		}

		return c.JSON(http.StatusOK, generateResponse)
	})

	e.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	address := ":5000"
	if _, ok := os.LookupEnv("NS_GEN_USE_HTTP"); ok {
		e.Logger.Fatal(e.Start(":5000"))
	} else {
		e.Logger.Fatal(
			e.StartTLS(
				address,
				"/mnt/serving-certs/tls.crt",
				"/mnt/serving-certs/tls.key",
			),
		)
	}

}
