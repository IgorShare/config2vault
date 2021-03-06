// +build integration
/*
 * Copyright 2016 Igor Moochnick
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package injest

import (
	"config2vault/log"
	"crypto/x509"
	"encoding/pem"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestInjestPkiBackend(t *testing.T) {
	t.Skip("skipping test for now.")

	log.SetLevel(log.ErrorLevel)

	testEnvPath := "../testing/integration/pki/docker-compose.yml"
	testEnvDir := filepath.Dir(testEnvPath)

	vault, key, deferFn, err := createTestProject(testEnvPath, "", "", "", nil, false)
	if deferFn != nil {
		defer deferFn()
	}
	if err != nil {
		t.Fatal("Failed to initialize Vault client")
	}
	if key == "" {
		t.Fatal("Got an Empty security key")
	}

	Convey("Enable PKI", t, func() {
		Convey("Role is created", func() {
			mountType := "pki"
			mountPath := "pki"
			mountDescr := "PKI backend"
			policies := vaultConfig{
				Mounts: []mountInfo{
					{
						Path:        mountPath,
						Type:        mountType,
						Description: mountDescr,
						MaxLeaseTTL: "87600h",
						Config: []map[string]interface{}{
							{
								"path": "ca",
								"ca_bundle": map[string]string{
									"key":  "@" + filepath.Join(testEnvDir, "ssl/ca.key"),
									"cert": "@" + filepath.Join(testEnvDir, "ssl/ca.crt"),
								},
							},
						},
					},
				},
				Roles: []rolePolicy{
					{
						Name: "example-dot-com",
						Path: mountPath,
						Properties: map[string]string{
							"allowed_domains":  "example.com",
							"allow_subdomains": "true",
							"max_ttl":          "72h",
						},
					},
					{
						Name: "test-dot-local",
						Path: mountPath,
						Properties: map[string]string{
							"allow_any_name":   "true",
							"allowed_domains":  "example.com",
							"allow_subdomains": "true",
							"allow_ip_sans":    "true",
							"max_ttl":          "420h",
							"allow_localhost":  "true",
						},
					},
				},
			}
			err := injestConfig(vault, &policies)
			So(err, ShouldBeEmpty)

			mounts, err := vault.ListMounts()
			if err != nil {
				t.Fatal("Failed to get mounts")
			}
			pkiMount := (*mounts)[mountPath]

			So(pkiMount, ShouldNotBeNil)
			So(pkiMount.Description, ShouldEqual, mountDescr)

			data := map[string]interface{}{
				"common_name": "blah.example.com",
			}
			secret, err := vault.Client.Logical().Write("/pki/issue/test-dot-local", data)
			if err != nil {
				t.Fatalf("Failed to issue a certificate. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)

			cert := getStringFromMap(&secret.Data, "certificate", "")
			pubKey, _ := pem.Decode([]byte(cert))
			pub, _ := x509.ParseCertificate(pubKey.Bytes)

			So(pub, ShouldNotBeNil)
			commonName := pub.Subject.CommonName
			So(commonName, ShouldEqual, "blah.example.com")
		})
		Convey("Use intermediate CA", func() {
			mountType := "pki"
			mountPath := "pki"
			mountDescr := "PKI backend"
			policies := vaultConfig{
				Mounts: []mountInfo{
					{
						Path:        mountPath,
						Type:        mountType,
						Description: mountDescr,
						MaxLeaseTTL: "87600h",
						Config: []map[string]interface{}{
							{
								"path": "ca",
								"ca_bundle": map[string]string{
									"key":  "@" + filepath.Join(testEnvDir, "ssl/ca_intermediate.key"),
									"cert": "@" + filepath.Join(testEnvDir, "ssl/ca_intermediate.crt"),
								},
							},
						},
					},
				},
				Roles: []rolePolicy{
					{
						Name: "example-dot-com",
						Path: mountPath,
						Properties: map[string]string{
							"allowed_domains":  "example.com",
							"allow_subdomains": "true",
							"max_ttl":          "72h",
						},
					},
					{
						Name: "test-dot-local",
						Path: mountPath,
						Properties: map[string]string{
							"allow_any_name":   "true",
							"allowed_domains":  "example.com",
							"allow_subdomains": "true",
							"allow_ip_sans":    "true",
							"max_ttl":          "420h",
							"allow_localhost":  "true",
						},
					},
				},
			}
			err := injestConfig(vault, &policies)
			So(err, ShouldBeEmpty)

			mounts, err := vault.ListMounts()
			if err != nil {
				t.Fatal("Failed to get mounts")
			}
			pkiMount := (*mounts)[mountPath]

			So(pkiMount, ShouldNotBeNil)
			So(pkiMount.Description, ShouldEqual, mountDescr)

			data := map[string]interface{}{
				"common_name": "blah.example.com",
			}
			secret, err := vault.Client.Logical().Write("/pki/issue/test-dot-local", data)
			if err != nil {
				t.Fatalf("Failed to issue a certificate. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)

			cert := getStringFromMap(&secret.Data, "certificate", "")
			pubKey, _ := pem.Decode([]byte(cert))
			pub, _ := x509.ParseCertificate(pubKey.Bytes)

			So(pub, ShouldNotBeNil)
			commonName := pub.Subject.CommonName
			So(commonName, ShouldEqual, "blah.example.com")
		})
		Convey("Generate signed cert", func() {
			mountType := "pki"
			mountPath := "pki-intermediate"
			mountDescr := "PKI intermediate"
			policies := vaultConfig{
				Mounts: []mountInfo{
					{
						Path:        "pki-root",
						Type:        mountType,
						Description: "PKI root",
						MaxLeaseTTL: "87600h",
						Config: []map[string]interface{}{
							{
								"path": "ca",
								"ca_bundle": map[string]string{
									"key":  "@" + filepath.Join(testEnvDir, "ssl/ca.key"),
									"cert": "@" + filepath.Join(testEnvDir, "ssl/ca.crt"),
								},
							},
						},
					},
					{
						Path:        mountPath,
						Type:        mountType,
						Description: mountDescr,
						MaxLeaseTTL: "87600h",
					},
				},
				Roles: []rolePolicy{
					{
						Name: "example-dot-com",
						Path: mountPath,
						Properties: map[string]string{
							"allowed_domains":  "example.com",
							"allow_subdomains": "true",
							"max_ttl":          "8h",
						},
					},
				},
			}
			err := injestConfig(vault, &policies)
			So(err, ShouldBeEmpty)

			data := map[string]interface{}{
				"common_name": "intermediate.com",
				"ip_sans":     "127.0.0.1",
			}
			secret, err := vault.Client.Logical().Write("/pki-intermediate/intermediate/generate/exported", data)
			if err != nil {
				t.Fatalf("Failed to get csr. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)
			csr := getStringFromMap(&secret.Data, "csr", "")
			So(csr, ShouldNotBeBlank)

			data = map[string]interface{}{
				"common_name": "intermediate.com",
				"csr":         csr,
			}
			secret, err = vault.Client.Logical().Write("/pki-root/root/sign-intermediate", data)
			if err != nil {
				t.Fatalf("Failed to issue a signed certificate. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)
			certificate := getStringFromMap(&secret.Data, "certificate", "")
			So(certificate, ShouldNotBeBlank)

			data = map[string]interface{}{
				"certificate": certificate,
			}
			secret, err = vault.Client.Logical().Write("/pki-intermediate/intermediate/set-signed", data)
			if err != nil {
				t.Fatalf("Failed to set an intermediate signed certificate. %#v", err)
			}
			So(secret, ShouldBeNil)

			data = map[string]interface{}{
				"common_name": "blah.example.com",
			}
			secret, err = vault.Client.Logical().Write("/pki-intermediate/issue/example-dot-com", data)
			if err != nil {
				t.Fatalf("Failed to issue a certificate. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)

			cert := getStringFromMap(&secret.Data, "certificate", "")
			pubKey, _ := pem.Decode([]byte(cert))
			pub, _ := x509.ParseCertificate(pubKey.Bytes)

			So(pub, ShouldNotBeNil)
			commonName := pub.Subject.CommonName
			So(commonName, ShouldEqual, "blah.example.com")
			issuer := pub.Issuer.CommonName
			So(issuer, ShouldEqual, "intermediate.com")
		})
		Convey("Generate signing cert", func() {
			mountType := "pki"
			mountPath := "pki-intermediate"
			mountDescr := "PKI intermediate"
			policies := vaultConfig{
				Mounts: []mountInfo{
					{
						Path:        "pki-root",
						Type:        mountType,
						Description: "PKI root",
						MaxLeaseTTL: "87600h",
						Config: []map[string]interface{}{
							{
								"path": "ca",
								"ca_bundle": map[string]string{
									"key":  "@" + filepath.Join(testEnvDir, "ssl/ca.key"),
									"cert": "@" + filepath.Join(testEnvDir, "ssl/ca.crt"),
								},
							},
						},
					},
					{
						Path:        mountPath,
						Type:        mountType,
						Description: mountDescr,
						MaxLeaseTTL: "87600h",
					},
				},
				Roles: []rolePolicy{
					{
						Name: "sign-cert",
						Path: mountPath,
						Properties: map[string]string{
							//"allowed_domains":     "sign",
							"allow_bare_domains": "true",
							//"organization ":       "Some Organization",
							//"ou":                  "Another Team",
							"server_flag":         "false",
							"client_flag":         "false",
							"allow_ip_sans":       "false",
							"enforce_hostnames":   "false",
							"allow_any_name":      "true",
							"code_signing_flag":   "true",
							"key_bits":            "4096",
							"use_csr_common_name": "false",
							"key_usage":           "DigitalSignature",
							"max_ttl":             "8h",
							"no_store":            "true",
						},
					},
				},
			}
			err := injestConfig(vault, &policies)
			So(err, ShouldBeEmpty)

			data := map[string]interface{}{
				"common_name": "intermediate.com",
				"ip_sans":     "127.0.0.1",
			}
			secret, err := vault.Client.Logical().Write("/pki-intermediate/intermediate/generate/exported", data)
			if err != nil {
				t.Fatalf("Failed to get csr. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)
			csr := getStringFromMap(&secret.Data, "csr", "")
			So(csr, ShouldNotBeBlank)

			data = map[string]interface{}{
				"common_name": "intermediate.com",
				"csr":         csr,
			}
			secret, err = vault.Client.Logical().Write("/pki-root/root/sign-intermediate", data)
			if err != nil {
				t.Fatalf("Failed to issue a signed certificate. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)
			certificate := getStringFromMap(&secret.Data, "certificate", "")
			So(certificate, ShouldNotBeBlank)

			data = map[string]interface{}{
				"certificate": certificate,
			}
			secret, err = vault.Client.Logical().Write("/pki-intermediate/intermediate/set-signed", data)
			if err != nil {
				t.Fatalf("Failed to set an intermediate signed certificate. %#v", err)
			}
			So(secret, ShouldBeNil)

			data = map[string]interface{}{
				"common_name":          "sign",
				"exclude_cn_from_sans": "true",
			}
			secret, err = vault.Client.Logical().Write("/pki-intermediate/issue/sign-cert", data)
			if err != nil {
				t.Fatalf("Failed to issue a certificate. %#v", err)
			}
			log.Infof("Got secret: %#v", *secret)

			cert := getStringFromMap(&secret.Data, "certificate", "")
			pubKey, _ := pem.Decode([]byte(cert))
			pub, _ := x509.ParseCertificate(pubKey.Bytes)

			So(pub, ShouldNotBeNil)
			commonName := pub.Subject.CommonName
			So(commonName, ShouldEqual, "sign")
			So(pub.KeyUsage, ShouldEqual, x509.KeyUsageDigitalSignature)
			So(pub.ExtKeyUsage, ShouldContain, x509.ExtKeyUsageCodeSigning)
		})
	})
}
