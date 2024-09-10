package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/konflux-ci/namespace-generator/pkg/handlers"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func getK8sClient(logger echo.Logger) (client.Reader, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	cl, err := cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	go cl.Start(context.TODO())

	if !cl.WaitForCacheSync(context.TODO()) {
		logger.Error("Failed to sync k8s client cache")
		return nil, err
	}

	return cl, nil
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

	keyPath := getKeyPath()
	api := e.Group("/api")
	api.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		validKey, err := os.ReadFile(keyPath)
		if err != nil {
			panic(fmt.Sprintf("Failed to read key file, %s\n", err.Error()))
		}
		return subtle.ConstantTimeCompare([]byte(key), validKey) == 1, nil
	}))

	gph := handlers.NewGetParamsHandler(getK8sClient)

	api.POST("/v1/getparams.execute", gph.GetParams)

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
