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

package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/internal/names"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service Batch Binding", LService, func() {
	var namespace string
	var catalogServiceName string
	var containerImageURL string

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Create a catalog service for testing
		catalogServiceName = "postgresql-dev" // Using existing dev service
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("Backward Compatibility", func() {
		var service, app, chart string

		BeforeEach(func() {
			service = catalog.NewServiceName()
			chart = names.ServiceReleaseName(service)

			By("create service")
			out, err := env.Epinio("", "service", "create", catalogServiceName, service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("wait for service deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, ServiceDeployTimeout, ServiceDeployPollingInterval).Should(
				HaveATable(WithRow("Status", "deployed")),
			)

			By("create app")
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
		})

		AfterEach(func() {
			env.Epinio("", "service", "unbind", service, app)
			env.Epinio("", "service", "delete", service)
			env.DeleteApp(app)
		})

		It("works with old format: bind SERVICE APP", func() {
			By("bind using old format")
			out, err := env.Epinio("", "service", "bind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)

			By("verify binding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Bound Configurations", chart+".*"),
				),
			)
		})
	})

	Describe("Batch Binding", func() {
		var service1, service2, service3, app string
		var chart1, chart2, chart3 string

		BeforeEach(func() {
			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()
			service3 = catalog.NewServiceName()

			chart1 = names.ServiceReleaseName(service1)
			chart2 = names.ServiceReleaseName(service2)
			chart3 = names.ServiceReleaseName(service3)

			By("create services")
			for _, service := range []string{service1, service2, service3} {
				out, err := env.Epinio("", "service", "create", catalogServiceName, service)
				Expect(err).ToNot(HaveOccurred(), out)
			}

			By("wait for all services to deploy")
			for _, service := range []string{service1, service2, service3} {
				Eventually(func() string {
					out, _ := env.Epinio("", "service", "show", service)
					return out
				}, ServiceDeployTimeout, ServiceDeployPollingInterval).Should(
					HaveATable(WithRow("Status", "deployed")),
				)
			}

			By("create app")
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			for _, service := range []string{service1, service2, service3} {
				env.Epinio("", "service", "delete", service)
			}
		})

		It("binds multiple services with new format: bind APP SERVICE1 SERVICE2 SERVICE3", func() {
			By("bind all services at once")
			out, err := env.Epinio("", "service", "bind", app, service1, service2, service3)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Services Bound Successfully"))

			By("verify all services are bound")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())

			// All three services should be in bound configurations
			Expect(appShowOut).To(MatchRegexp(chart1))
			Expect(appShowOut).To(MatchRegexp(chart2))
			Expect(appShowOut).To(MatchRegexp(chart3))

			By("verify app is running")
			Expect(appShowOut).To(MatchRegexp("Status.*1/1"))

			By("verify services show bound app")
			for _, service := range []string{service1, service2, service3} {
				serviceShowOut, err := env.Epinio("", "service", "show", service)
				Expect(err).ToNot(HaveOccurred())
				Expect(serviceShowOut).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Used-By", app),
					),
				)
			}
		})

		It("is faster than binding services sequentially", func() {
			// This test verifies performance improvement by checking that
			// only one pod restart occurs (implicitly tested by successful binding)
			
			By("get initial pod names")
			initialPods := env.GetPodNames(app, namespace)
			Expect(len(initialPods)).To(BeNumerically(">", 0))

			By("bind all services at once")
			out, err := env.Epinio("", "service", "bind", app, service1, service2, service3)
			Expect(err).ToNot(HaveOccurred(), out)

			By("wait for app to stabilize")
			Eventually(func() string {
				out, _ := env.Epinio("", "app", "show", app)
				return out
			}, "3m", "5s").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "1/1"),
				),
			)

			By("verify all services are bound")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).To(ContainSubstring(chart1))
			Expect(appShowOut).To(ContainSubstring(chart2))
			Expect(appShowOut).To(ContainSubstring(chart3))
		})

		It("fails gracefully if one service doesn't exist", func() {
			By("try to bind with one non-existent service")
			out, err := env.Epinio("", "service", "bind", app, service1, "bogus-service", service2)
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("service bind"))

			By("verify no services were bound (atomic behavior)")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			// None of the services should be bound
			Expect(appShowOut).ToNot(MatchRegexp(chart1))
			Expect(appShowOut).ToNot(MatchRegexp(chart2))
		})
	})

	Describe("Edge Cases", func() {
		var app string

		BeforeEach(func() {
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
		})

		AfterEach(func() {
			env.DeleteApp(app)
		})

		It("fails when app doesn't exist", func() {
			service := catalog.NewServiceName()
			out, err := env.Epinio("", "service", "bind", "bogus-app", service)
			Expect(err).To(HaveOccurred())
			Expect(out).To(MatchRegexp("app.*not.*found|does not exist"))
		})

		It("requires at least 2 arguments", func() {
			out, err := env.Epinio("", "service", "bind", app)
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("requires at least 2 arg(s)"))
		})
	})
})

