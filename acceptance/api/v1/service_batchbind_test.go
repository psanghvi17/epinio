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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceBatchBind Endpoint", LService, func() {
	var namespace, containerImageURL string
	var catalogService models.CatalogService

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Use MySQL as it creates actual secrets
		catalogService = models.CatalogService{
			Meta: models.MetaLite{
				Name: catalog.NewCatalogServiceName(),
			},
			HelmChart: "mysql",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
			Values: "",
		}
		catalog.CreateCatalogService(catalogService)
	})

	AfterEach(func() {
		catalog.DeleteCatalogService(catalogService.Meta.Name)
		env.DeleteNamespace(namespace)
	})

	When("the application doesn't exist", func() {
		var serviceName1, serviceName2 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()
			catalog.CreateService(serviceName1, namespace, catalogService)
			catalog.CreateService(serviceName2, namespace, catalogService)
		})

		AfterEach(func() {
			catalog.DeleteService(serviceName1, namespace)
			catalog.DeleteService(serviceName2, namespace)
		})

		It("returns 404", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBatchBind", namespace, "bogus-app"))
			requestBody, err := json.Marshal(models.ServiceBatchBindRequest{
				AppName:      "bogus-app",
				ServiceNames: []string{serviceName1, serviceName2},
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	When("one of the services doesn't exist", func() {
		var app, serviceName1 string

		BeforeEach(func() {
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)

			serviceName1 = catalog.NewServiceName()
			catalog.CreateService(serviceName1, namespace, catalogService)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			catalog.DeleteService(serviceName1, namespace)
		})

		It("returns 404 for the missing service", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBatchBind", namespace, app))
			requestBody, err := json.Marshal(models.ServiceBatchBindRequest{
				AppName:      app,
				ServiceNames: []string{serviceName1, "bogus-service"},
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	When("binding multiple services at once", func() {
		var app, serviceName1, serviceName2, serviceName3 string
		var chartName1, chartName2, chartName3 string

		BeforeEach(func() {
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)

			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()
			serviceName3 = catalog.NewServiceName()

			chartName1 = names.ServiceReleaseName(serviceName1)
			chartName2 = names.ServiceReleaseName(serviceName2)
			chartName3 = names.ServiceReleaseName(serviceName3)

			catalog.CreateService(serviceName1, namespace, catalogService)
			catalog.CreateService(serviceName2, namespace, catalogService)
			catalog.CreateService(serviceName3, namespace, catalogService)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			catalog.DeleteService(serviceName1, namespace)
			catalog.DeleteService(serviceName2, namespace)
			catalog.DeleteService(serviceName3, namespace)
		})

		It("binds all services' secrets to the application", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBatchBind", namespace, app))
			requestBody, err := json.Marshal(models.ServiceBatchBindRequest{
				AppName:      app,
				ServiceNames: []string{serviceName1, serviceName2, serviceName3},
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			// Verify all services are bound by checking app show output
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())

			// All three service secrets should be in bound configurations
			Expect(appShowOut).To(MatchRegexp(chartName1))
			Expect(appShowOut).To(MatchRegexp(chartName2))
			Expect(appShowOut).To(MatchRegexp(chartName3))
		})

		It("triggers only one pod restart (performance test)", func() {
			// Get initial pod names
			initialPods := env.GetPodNames(app, namespace)
			Expect(len(initialPods)).To(BeNumerically(">", 0))
			initialPod := initialPods[0]

			// Batch bind all three services
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBatchBind", namespace, app))
			requestBody, err := json.Marshal(models.ServiceBatchBindRequest{
				AppName:      app,
				ServiceNames: []string{serviceName1, serviceName2, serviceName3},
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusOK))

			// Wait for new pod to be ready (pod name should change after restart)
			Eventually(func() bool {
				newPods := env.GetPodNames(app, namespace)
				if len(newPods) == 0 {
					return false
				}
				// Should have new pod with different name
				return newPods[0] != initialPod
			}, "2m", "5s").Should(BeTrue())

			// Verify app is healthy with all services bound
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).To(MatchRegexp("Status.*1/1"))
		})
	})

	When("binding with empty service list", func() {
		var app string

		BeforeEach(func() {
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
		})

		AfterEach(func() {
			env.DeleteApp(app)
		})

		It("returns a 400 bad request", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBatchBind", namespace, app))
			requestBody, err := json.Marshal(models.ServiceBatchBindRequest{
				AppName:      app,
				ServiceNames: []string{},
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
		})
	})

	When("binding a single service via batch endpoint", func() {
		var app, serviceName, chartName string

		BeforeEach(func() {
			app = catalog.NewAppName()
			serviceName = catalog.NewServiceName()
			chartName = names.ServiceReleaseName(serviceName)

			env.MakeContainerImageApp(app, 1, containerImageURL)
			catalog.CreateService(serviceName, namespace, catalogService)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			catalog.DeleteService(serviceName, namespace)
		})

		It("works correctly (backward compatibility)", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBatchBind", namespace, app))
			requestBody, err := json.Marshal(models.ServiceBatchBindRequest{
				AppName:      app,
				ServiceNames: []string{serviceName},
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).To(MatchRegexp(chartName))
		})
	})
})

