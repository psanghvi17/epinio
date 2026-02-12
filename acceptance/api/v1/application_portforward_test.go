// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1_test

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/portforward"
	gospdy "k8s.io/client-go/transport/spdy"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppPortForward Endpoint", LApplication, func() {
	var (
		appName   string
		namespace string
	)

	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		env.MakeContainerImageApp(appName, 1, containerImageURL)
	})

	AfterEach(func() {
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
	})

	Describe("GET /namespaces/:namespace/applications/:app/portforward", func() {

		When("you don't specify an instance", func() {
			It("runs a GET through the opened stream and gets the response back", func() {
				// Keep this resilient for transient websocket/SPDY setup issues seen in CI.
				Eventually(func() error {
					return runPortForwardGet(namespace, appName, "")
				}, "30s", "2s").Should(Succeed())
			})
		})

		When("you specify a non existing instance", func() {
			var connErr error

			BeforeEach(func() {
				_, connErr = setupConnection(namespace, appName, "nonexisting")
			})

			It("fails with a 400 bad request", func() {
				Expect(connErr).To(HaveOccurred())
			})
		})

		When("you specify a specific instance", func() {
			var appName string
			var instanceName string

			BeforeEach(func() {
				// Bug fix: Use separate application instead of the main of the suite
				appName = "portforward"

				env.MakeContainerImageApp(appName, 2, containerImageURL)

				out, err := proc.Kubectl("get", "pods",
					"-n", namespace,
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s", appName),
					"-o", "name",
				)
				Expect(err).ToNot(HaveOccurred())

				podNames := strings.Split(strings.TrimSpace(out), "\n")
				Expect(len(podNames)).To(Equal(2))

				instanceName = strings.Replace(podNames[1], "pod/", "", -1)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("runs a GET through the opened stream and gets the response back", func() {
				// Keep this resilient for transient websocket/SPDY setup issues seen in CI.
				Eventually(func() error {
					return runPortForwardGet(namespace, appName, instanceName)
				}, "30s", "2s").Should(Succeed())
			})
		})
	})
})

func runPortForwardGet(namespace, appName, instance string) error {
	conn, err := setupConnection(namespace, appName, instance)
	if err != nil {
		return err
	}
	defer conn.Close()

	streamData, streamErr := createStreams(conn)
	defer streamData.Close()
	defer streamErr.Close()

	// Send a GET request through the stream (apache inside sample-app listens on port 80).
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/", nil)
	if err = req.Write(streamData); err != nil {
		return err
	}

	reader := bufio.NewReader(streamData)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	errData, err := io.ReadAll(streamErr)
	if err != nil {
		return err
	}
	if len(errData) > 0 {
		return fmt.Errorf("unexpected data on error stream: %s", string(errData))
	}

	return nil
}

func setupConnection(namespace, appName, instance string) (httpstream.Connection, error) {
	endpoint := fmt.Sprintf("%s%s/%s?instance=%s", serverURL, api.WsRoot, api.WsRoutes.Path("AppPortForward", namespace, appName), instance)
	portForwardURL, err := url.Parse(endpoint)
	Expect(err).ToNot(HaveOccurred())

	token, err := authToken()
	Expect(err).ToNot(HaveOccurred())

	values := portForwardURL.Query()
	values.Add("authtoken", token)
	portForwardURL.RawQuery = values.Encode()

	// we need to use the spdy client to handle this connection
	upgradeRoundTripper, err := client.NewUpgrader(spdy.RoundTripperConfig{
		TLS:        http.DefaultTransport.(*http.Transport).TLSClientConfig, // See `ExtendLocalTrust`
		PingPeriod: time.Second * 5,
	})
	Expect(err).ToNot(HaveOccurred())

	httpClient := &http.Client{Transport: upgradeRoundTripper, Timeout: 180 * time.Second}
	dialer := gospdy.NewDialer(upgradeRoundTripper, httpClient, "GET", portForwardURL)
	conn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)

	return conn, err
}

func createStreams(conn httpstream.Connection) (httpstream.Stream, httpstream.Stream) {
	buildHeaders := func(streamType string) http.Header {
		headers := http.Header{}
		// sample-app image (php:8.2-apache) listens on container port 80
		headers.Set(v1.PortHeader, "80")
		headers.Set(v1.PortForwardRequestIDHeader, "0")
		headers.Set(v1.StreamType, streamType)
		return headers
	}

	// open streams
	streamData, err := conn.CreateStream(buildHeaders(v1.StreamTypeData))
	Expect(err).ToNot(HaveOccurred())
	streamErr, err := conn.CreateStream(buildHeaders(v1.StreamTypeError))
	Expect(err).ToNot(HaveOccurred())

	return streamData, streamErr
}
