package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konflux-ci/namespace-generator/pkg/api/v1alpha1"
	"github.com/konflux-ci/namespace-generator/pkg/test/utils"
)

func TestProvision(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var k8sClient client.Client
var testEnv *envtest.Environment
var serverProcess *exec.Cmd
var serverCancelFunc context.CancelFunc

var _ = BeforeSuite(func() {
	schema := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(schema))
	testEnv = &envtest.Environment{BinaryAssetsDirectory: "../../../bin/k8s/1.29.0-linux-amd64/"}
	k8sClient = utils.StartTestEnv(schema, testEnv)

	serverProcess, serverCancelFunc = utils.CreateServer(
		"../../../cmd/main.go",
		[]string{
			"NS_GEN_USE_HTTP=true",
			fmt.Sprintf("NS_GEN_KEY_PATH=%s", createKeyFile()),
		},
		"",
	)
	utils.WaitForServerToServe("http://localhost:5000/health")
	createNamespaces(k8sClient)
})

var _ = AfterSuite(func() {
	utils.StopServer(serverProcess, serverCancelFunc)
	utils.StopEnvTest(testEnv)
})

func createKeyFile() string {
	f, err := os.CreateTemp("", "ns-gen-pass-file")
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()
	_, err = f.WriteString("password")
	Expect(err).ToNot(HaveOccurred())
	return f.Name()
}

func createNamespaces(k8sClient client.Client) {
	createNS := func(name string, labels map[string]string) {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "",
				Name:      name,
				Labels:    labels,
			},
		}
		err := k8sClient.Create(context.Background(), ns)
		Expect(err).ToNot(HaveOccurred(), "Failed to create namespace: %s", name)
	}

	labels := map[string]string{"konflux.ci/type": "user"}
	createNS("ns1", labels)
	createNS("ns2", labels)
	createNS("ns3", nil)
}

var _ = Describe("simple test", func() {
	endpoint := "http://localhost:5000/api/v1/getparams.execute"
	httpClient := &http.Client{}

	Context("simple test context", func() {
		It("simple spec", func() {

			generateRequest := &v1alpha1.GenerateRequest{
				ApplicationSetName: "test-app",
				Input: v1alpha1.Input{
					Parameters: v1alpha1.InParameters{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"konflux.ci/type": "user"},
						},
					},
				},
			}
			buffer := &bytes.Buffer{}
			err := json.NewEncoder(buffer).Encode(generateRequest)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", endpoint, buffer)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Authorization", "bearer password")

			response, err := httpClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			parsedResponse := &v1alpha1.GenerateResponse{}
			err = json.NewDecoder(response.Body).Decode(parsedResponse)
			Expect(err).NotTo(HaveOccurred())
			expected := &v1alpha1.GenerateResponse{
				Output: v1alpha1.Output{
					Parameters: []v1alpha1.OutParameters{
						{Namespace: "ns1"},
						{Namespace: "ns2"},
					},
				},
			}
			Expect(parsedResponse).To(Equal(expected))
		})
	})

	Context("Request without a bearer token", func() {
		It("Should return status 400", func() {
			request, err := http.NewRequest("POST", endpoint, nil)
			Expect(err).NotTo(HaveOccurred())
			//request.Header.Set("Authorization", "bearer passworddd")
			response, err := httpClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Context("Request with a wrong bearer token", func() {
		It("Should return status 401", func() {
			request, err := http.NewRequest("POST", endpoint, nil)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Authorization", "bearer not a password")
			response, err := httpClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})
})
