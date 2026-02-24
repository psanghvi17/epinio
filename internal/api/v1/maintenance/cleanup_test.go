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

package maintenance_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/epinio/epinio/internal/api/v1/maintenance"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Maintenance Cleanup API", func() {
	var c *gin.Context
	var w *httptest.ResponseRecorder

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		ctx := context.Background()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		Expect(err).ToNot(HaveOccurred())
		c.Request = req
	})

	Describe("CleanupStaleCaches (POST)", func() {
		When("request body is invalid JSON", func() {
			It("returns 400 Bad Request", func() {
				body := strings.NewReader(`{"staleDays": "not a number"`)
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "/api/v1/maintenance/cleanup-stale-caches", body)
				Expect(err).ToNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")
				c.Request = req

				apiErr := maintenance.CleanupStaleCaches(c)
				Expect(apiErr).ToNot(BeNil())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
			})

			It("returns 400 for malformed JSON", func() {
				body := strings.NewReader(`{invalid`)
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "/api/v1/maintenance/cleanup-stale-caches", body)
				Expect(err).ToNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")
				c.Request = req

				apiErr := maintenance.CleanupStaleCaches(c)
				Expect(apiErr).ToNot(BeNil())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
			})
		})

		When("request body is empty", func() {
			It("does not return a validation error", func() {
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "/api/v1/maintenance/cleanup-stale-caches", nil)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				apiErr := maintenance.CleanupStaleCaches(c)
				// Without a cluster we expect 500 (GetCluster fails), not 400
				Expect(apiErr).ToNot(BeNil())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("CleanupStaleCachesQuery (GET)", func() {
		When("staleDays query parameter is invalid", func() {
			It("returns 400 Bad Request for non-numeric staleDays", func() {
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, "/api/v1/maintenance/cleanup-stale-caches?staleDays=abc", nil)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				apiErr := maintenance.CleanupStaleCachesQuery(c)
				Expect(apiErr).ToNot(BeNil())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
				Expect(apiErr.Errors()).To(HaveLen(1))
				Expect(apiErr.Errors()[0].Title).To(ContainSubstring("invalid staleDays"))
			})

			It("returns 400 for staleDays that is not an integer", func() {
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, "/api/v1/maintenance/cleanup-stale-caches?staleDays=30.5", nil)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				apiErr := maintenance.CleanupStaleCachesQuery(c)
				Expect(apiErr).ToNot(BeNil())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
				Expect(apiErr.Errors()).To(HaveLen(1))
				Expect(apiErr.Errors()[0].Title).To(ContainSubstring("invalid staleDays"))
			})
		})

		When("staleDays is valid", func() {
			It("proceeds to cluster call (may return 500 without cluster)", func() {
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, "/api/v1/maintenance/cleanup-stale-caches?staleDays=7", nil)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				apiErr := maintenance.CleanupStaleCachesQuery(c)
				// Without kubeconfig/cluster we get 500, not 400
				Expect(apiErr).ToNot(BeNil())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusInternalServerError))
			})
		})
	})

})
