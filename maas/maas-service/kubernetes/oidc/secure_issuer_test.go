package oidc

import "testing"

func TestIsIssuerSecure(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		secure bool
	}{
		{
			name:   "on-premise",
			issuer: "https://kubernetes.default.svc.cluster.local",
			secure: true,
		},
		{
			name:   "aws",
			issuer: "https://oidc.eks.us-east-1.amazonaws.com/id/FAE7A17966F4B23E22378A5969AAE9AF",
			secure: false,
		},
		{
			name:   "gcp",
			issuer: "https://container.googleapis.com/v1/projects/linen-source-461706-s9/locations/europe-central2/clusters/autopilot-cluster-1",
			secure: false,
		},
		{
			name:   "azure",
			issuer: "https://nd-azurek8s01-ftpd-041aa776.3dff5254-0d69-41ce-b92c-ca529f3e1a1e.privatelink.eastus.azmk8s.io",
			secure: true,
		},
		{
			name:   "openshift",
			issuer: "https://api.ocp4-qa.openshift.sdntest.netcracker.com:6443",
			secure: true,
		},
		{
			name:   "localhost",
			issuer: "http://localhost:6443",
			secure: false,
		},
		{
			name:   "localhost-ip",
			issuer: "http://127.0.0.1:7932",
			secure: false,
		},
	}
	for i, test := range tests {
		secure, err := isSecureIssuer(test.issuer)
		if err != nil {
			t.Fatal(err)
		}
		if secure != test.secure {
			t.Errorf("%d. %s: expected secure issuer %t, got %t", i+1, test.name, test.secure, secure)
		}
	}
}
