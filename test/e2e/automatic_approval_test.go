/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kuadrant/developer-portal-controller/test/utils"
)

var _ = Describe("Automatic Approval", Ordered, func() {
	const (
		ownerNamespace    = "api-owner-test"
		consumerNamespace = "api-consumer-test"
		kuadrantNamespace = "kuadrant-ns"
		apiProductName    = "auto-approve-api"
		apiKeyName        = "test-auto-apikey"
	)

	BeforeAll(func() {
		By("creating the owner namespace")
		cmd := exec.Command("kubectl", "create", "ns", ownerNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create owner namespace")

		By("creating the consumer namespace")
		cmd = exec.Command("kubectl", "create", "ns", consumerNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create consumer namespace")

		By("creating the kuadrant namespace")
		cmd = exec.Command("kubectl", "create", "ns", kuadrantNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create kuadrant namespace")

		By("creating the kuadrant instance")
		kuadrantYAML := fmt.Sprintf(`
apiVersion: kuadrant.io/v1beta1
kind: Kuadrant
metadata:
  name: kuadrant
  namespace: %s
spec: {}
`, kuadrantNamespace)

		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = utils.StringReader(kuadrantYAML)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create Kuadrant")
	})

	AfterAll(func() {
		By("cleaning up kuadrant namespace")
		cmd := exec.Command("kubectl", "delete", "ns", kuadrantNamespace, "--wait=false")
		_, _ = utils.Run(cmd)

		By("cleaning up owner namespace")
		cmd = exec.Command("kubectl", "delete", "ns", ownerNamespace, "--wait=false")
		_, _ = utils.Run(cmd)

		By("cleaning up consumer namespace")
		cmd = exec.Command("kubectl", "delete", "ns", consumerNamespace, "--wait=false")
		_, _ = utils.Run(cmd)
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	Context("APIKey with automatic approval mode", func() {
		It("should create APIKeyRequest, automatic approval, and approve the APIKey", func() {
			By("creating an HTTPRoute as a reference target")
			httpRouteYAML := fmt.Sprintf(`
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: test-route
  namespace: %s
spec:
  parentRefs:
  - name: test-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api
    backendRefs:
    - name: test-service
      port: 8080
`, ownerNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(httpRouteYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create HTTPRoute")

			By("creating an AuthPolicy with API key authentication")
			authPolicyYAML := fmt.Sprintf(`
apiVersion: kuadrant.io/v1
kind: AuthPolicy
metadata:
  name: test-auth-policy
  namespace: %s
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: test-route
  rules:
    authentication:
      "api-key":
        apiKey:
          selector:
            matchLabels:
              kuadrant.io/apikeys: "true"
        credentials:
          authorizationHeader:
            prefix: "API-KEY"
`, ownerNamespace)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(authPolicyYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create AuthPolicy")

			By("updating AuthPolicy status to Accepted and Enforced")
			authPolicyStatusPatch := `{
				"status": {
					"conditions": [
						{
							"type": "Accepted",
							"status": "True",
							"reason": "Accepted",
							"message": "AuthPolicy has been accepted",
							"lastTransitionTime": "2024-01-01T00:00:00Z"
						},
						{
							"type": "Enforced",
							"status": "True",
							"reason": "Enforced",
							"message": "AuthPolicy has been successfully enforced",
							"lastTransitionTime": "2024-01-01T00:00:00Z"
						}
					]
				}
			}`

			cmd = exec.Command("kubectl", "patch", "authpolicy", "test-auth-policy",
				"-n", ownerNamespace,
				"--type=merge",
				"--subresource=status",
				"-p", authPolicyStatusPatch)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to update AuthPolicy status")

			By("creating an APIProduct with automatic approval mode")
			apiProductYAML := fmt.Sprintf(`
apiVersion: devportal.kuadrant.io/v1alpha1
kind: APIProduct
metadata:
  name: %s
  namespace: %s
spec:
  displayName: "Auto Approval Test API"
  description: "API Product for testing automatic approval"
  approvalMode: automatic
  publishStatus: Published
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: test-route
`, apiProductName, ownerNamespace)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(apiProductYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create APIProduct")

			By("verifying APIProduct was created")
			verifyAPIProductCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apiproduct", apiProductName,
					"-n", ownerNamespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(apiProductName))
			}
			Eventually(verifyAPIProductCreated).Should(Succeed())

			By("creating a secret with API key in the consumer namespace")
			secretYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s-secret
  namespace: %s
type: Opaque
stringData:
  api_key: test-api-key-value-12345
`, apiKeyName, consumerNamespace)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(secretYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create secret")

			By("creating an APIKey in the consumer namespace")
			apiKeyYAML := fmt.Sprintf(`
apiVersion: devportal.kuadrant.io/v1alpha1
kind: APIKey
metadata:
  name: %s
  namespace: %s
spec:
  apiProductRef:
    name: %s
    namespace: %s
  secretRef:
    name: %s-secret
  planTier: premium
  useCase: "Testing automatic approval flow"
  requestedBy:
    userId: test-user-123
    email: test@example.com
`, apiKeyName, consumerNamespace, apiProductName, ownerNamespace, apiKeyName)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(apiKeyYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create APIKey")

			By("verifying APIKey was created")
			verifyAPIKeyCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikey", apiKeyName,
					"-n", consumerNamespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(apiKeyName))
			}
			Eventually(verifyAPIKeyCreated).Should(Succeed())

			By("verifying APIKeyRequest was created in the owner namespace")
			apiKeyRequestName := fmt.Sprintf("%s-%s", consumerNamespace, apiKeyName)
			verifyAPIKeyRequestCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikeyrequest", apiKeyRequestName,
					"-n", ownerNamespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(apiKeyRequestName))
			}
			Eventually(verifyAPIKeyRequestCreated).Should(Succeed())

			By("verifying APIKeyApproval was automatically created")
			verifyApprovalCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikeyapproval",
					"-n", ownerNamespace, "-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "APIKeyApproval should be created")
				g.Expect(output).To(ContainSubstring("auto"), "APIKeyApproval name should contain 'auto'")
			}
			Eventually(verifyApprovalCreated).Should(Succeed())

			By("verifying the approval was created by 'system'")
			verifySystemReviewer := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikeyapproval",
					"-n", ownerNamespace, "-o", "jsonpath={.items[0].spec.reviewedBy}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("system"), "APIKeyApproval should be reviewed by 'system'")
			}
			Eventually(verifySystemReviewer).Should(Succeed())

			By("verifying the approval is approved")
			verifyApproved := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikeyapproval",
					"-n", ownerNamespace, "-o", "jsonpath={.items[0].spec.approved}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "APIKeyApproval should be approved")
			}
			Eventually(verifyApproved).Should(Succeed())

			By("verifying the approval reason is AutoApproved")
			verifyReason := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikeyapproval",
					"-n", ownerNamespace, "-o", "jsonpath={.items[0].spec.reason}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("AutoApproved"), "APIKeyApproval reason should be AutoApproved")
			}
			Eventually(verifyReason).Should(Succeed())

			By("verifying APIKeyRequest gets Approved condition")
			verifyRequestApproved := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikeyrequest", apiKeyRequestName,
					"-n", ownerNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Approved')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "APIKeyRequest should have Approved=True condition")
			}
			Eventually(verifyRequestApproved).Should(Succeed())

			By("verifying APIKey eventually gets Approved condition")
			verifyAPIKeyApproved := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "apikey", apiKeyName,
					"-n", consumerNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Approved')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "APIKey should have Approved=True condition")
			}
			Eventually(verifyAPIKeyApproved).Should(Succeed())
		})
	})
})
