package main

import "testing"

func TestRequireTLSEnv(t *testing.T) {
	const cert, key, ca, tenantCert, tenantKey = "cert", "key", "ca", "tenant-cert", "tenant-key"

	tests := []struct {
		name                                                   string
		tlsCert, tlsKey, clientCA, tenantTLSCert, tenantTLSKey string
		wantErr                                                bool
	}{
		{"all set", cert, key, ca, tenantCert, tenantKey, false},
		{"missing mTLS cert", "", key, ca, tenantCert, tenantKey, true},
		{"missing mTLS key", cert, "", ca, tenantCert, tenantKey, true},
		{"missing client CA", cert, key, "", tenantCert, tenantKey, true},
		{"missing tenant cert", cert, key, ca, "", tenantKey, true},
		{"missing tenant key", cert, key, ca, tenantCert, "", true},
		{"nothing set", "", "", "", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireTLSEnv(tc.tlsCert, tc.tlsKey, tc.clientCA, tc.tenantTLSCert, tc.tenantTLSKey)
			if tc.wantErr && err == nil {
				t.Fatalf("requireTLSEnv() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("requireTLSEnv() = %v, want nil", err)
			}
		})
	}
}
