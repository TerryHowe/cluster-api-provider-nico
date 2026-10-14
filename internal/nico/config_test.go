// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nico

import (
	"context"
	"flag"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/dsx-ai-factory/cluster-api-provider-nico/internal/fake"
)

func TestLoadSecretConfigStaticToken(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			SecretKeyEndpoint: []byte("https://nico.example.com"),
			SecretKeyOrgID:    []byte("test-org"),
			SecretKeyToken:    []byte("static-token"),
		},
	}

	cfg, err := LoadSecretConfig(secret)
	if err != nil {
		t.Fatalf("LoadSecretConfig() error = %v", err)
	}
	if cfg.Token != "static-token" {
		t.Fatalf("expected static token, got %q", cfg.Token)
	}
	if cfg.TokenURL != "" {
		t.Fatalf("expected empty tokenURL for static token config, got %q", cfg.TokenURL)
	}
}

func TestLoadSecretConfigOAuthClientCredentials(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			SecretKeyEndpoint:     []byte("https://nico.example.com"),
			SecretKeyOrgID:        []byte("test-org"),
			SecretKeyTokenURL:     []byte("https://issuer.example.com/token"),
			SecretKeyClientID:     []byte("client-id"),
			SecretKeyClientSecret: []byte("client-secret"),
			SecretKeyScope:        []byte("carbide extra"),
		},
	}

	cfg, err := LoadSecretConfig(secret)
	if err != nil {
		t.Fatalf("LoadSecretConfig() error = %v", err)
	}
	if cfg.TokenURL != "https://issuer.example.com/token" {
		t.Fatalf("expected tokenURL to be loaded, got %q", cfg.TokenURL)
	}
	if got, want := strings.Join(cfg.Scopes, " "), "carbide extra"; got != want {
		t.Fatalf("expected scopes %q, got %q", want, got)
	}
}

func TestLoadSecretConfigRejectsMixedAuthModes(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			SecretKeyEndpoint:     []byte("https://nico.example.com"),
			SecretKeyOrgID:        []byte("test-org"),
			SecretKeyToken:        []byte("static-token"),
			SecretKeyTokenURL:     []byte("https://issuer.example.com/token"),
			SecretKeyClientID:     []byte("client-id"),
			SecretKeyClientSecret: []byte("client-secret"),
		},
	}

	if _, err := LoadSecretConfig(secret); err == nil {
		t.Fatalf("expected mixed auth modes to be rejected")
	}
}

func TestTokenSourceStaticToken(t *testing.T) {
	cfg := SecretConfig{
		Endpoint: "https://nico.example.com",
		OrgID:    "test-org",
		Token:    "static-token",
	}

	httpClient, err := cfg.HTTPClient()
	if err != nil {
		t.Fatalf("HTTPClient() error = %v", err)
	}

	source, err := cfg.TokenSource(context.Background(), httpClient)
	if err != nil {
		t.Fatalf("TokenSource() error = %v", err)
	}

	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.AccessToken != "static-token" {
		t.Fatalf("expected static token, got %q", token.AccessToken)
	}
}

func TestTokenSourceOAuthClientCredentials(t *testing.T) {
	api := fake.New()
	api.SeedClient("client-id", "client-secret")
	server := httptest.NewServer(api.Handler())
	t.Cleanup(server.Close)

	cfg := SecretConfig{
		Endpoint:     "https://nico.example.com",
		OrgID:        "test-org",
		TokenURL:     server.URL + "/token",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Scopes:       []string{"carbide"},
	}

	httpClient, err := cfg.HTTPClient()
	if err != nil {
		t.Fatalf("HTTPClient() error = %v", err)
	}

	source, err := cfg.TokenSource(context.Background(), httpClient)
	if err != nil {
		t.Fatalf("TokenSource() error = %v", err)
	}

	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if api.TokenRequestCount() != 1 {
		t.Fatalf("expected token endpoint to be called")
	}
	if token.AccessToken == "" {
		t.Fatalf("expected a non-empty access token")
	}
}

func TestProviderConfigIdentityValidationTimeout(t *testing.T) {
	parse := func(t *testing.T, args ...string) ProviderConfig {
		t.Helper()
		cfg := ProviderConfig{Credentials: types.NamespacedName{Namespace: "capnico-system", Name: "nico-credentials"}}
		fs := flag.NewFlagSet("manager", flag.ContinueOnError)
		cfg.BindFlags(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse(%q) error = %v", args, err)
		}
		return cfg
	}

	t.Run("accepts positive validation timeouts", func(t *testing.T) {
		for _, tc := range []struct {
			args []string
			want time.Duration
		}{
			{want: 30 * time.Second},
			{args: []string{"--provider-identity-validation-timeout=45s"}, want: 45 * time.Second},
			{args: []string{"--provider-identity-validation-timeout=500ms"}, want: 500 * time.Millisecond},
		} {
			cfg := parse(t, tc.args...)
			if err := cfg.Validate(); err != nil {
				t.Fatalf("args %q: Validate() error = %v", tc.args, err)
			}
			if cfg.IdentityValidationTimeout != tc.want {
				t.Fatalf("args %q: IdentityValidationTimeout = %s, want %s", tc.args, cfg.IdentityValidationTimeout, tc.want)
			}
		}
	})

	t.Run("zero timeout is rejected", func(t *testing.T) {
		const want = "--provider-identity-validation-timeout must be positive"
		cfg := parse(t, "--provider-identity-validation-timeout=0s")
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %v, want it to contain %q", err, want)
		}
	})
}

func TestProviderConfigTopologyLabelDomain(t *testing.T) {
	parse := func(t *testing.T, args ...string) ProviderConfig {
		t.Helper()
		var cfg ProviderConfig
		fs := flag.NewFlagSet("manager", flag.ContinueOnError)
		cfg.BindFlags(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse(%q) error = %v", args, err)
		}
		return cfg
	}

	t.Run("defaults to the legacy domain", func(t *testing.T) {
		cfg := parse(t)
		if cfg.TopologyLabelDomain != DefaultTopologyLabelDomain {
			t.Fatalf("TopologyLabelDomain = %q, want %q", cfg.TopologyLabelDomain, DefaultTopologyLabelDomain)
		}
		if got := cfg.TopologyLabelKey(TopologyLabelSiteID); got != "nke.nvidia.com/site-id" {
			t.Fatalf("TopologyLabelKey() = %q", got)
		}
	})

	t.Run("zero value falls back to the legacy domain", func(t *testing.T) {
		if got := (ProviderConfig{}).TopologyLabelKey(TopologyLabelVPCName); got != "nke.nvidia.com/vpc-name" {
			t.Fatalf("TopologyLabelKey() = %q", got)
		}
	})

	t.Run("flag overrides the domain", func(t *testing.T) {
		cfg := parse(t, "--topology-label-domain=nvke.nvidia.com")
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if got := cfg.TopologyLabelKey(TopologyLabelMachineID); got != "nvke.nvidia.com/machine-id" {
			t.Fatalf("TopologyLabelKey() = %q", got)
		}
	})

	t.Run("rejects an invalid domain", func(t *testing.T) {
		const want = "--topology-label-domain"
		cfg := parse(t, "--topology-label-domain=Not_A_Domain/")
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %v, want it to contain %q", err, want)
		}
	})

	// A well-formed domain is still rejected: the flag accepts two values, not
	// any DNS subdomain, so that the labels to migrate stay a closed set.
	t.Run("rejects an unsupported domain", func(t *testing.T) {
		cfg := parse(t, "--topology-label-domain=example.com")
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want an error")
		}
		for _, want := range []string{"example.com", LegacyTopologyLabelDomain, NVKETopologyLabelDomain} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("Validate() error = %v, want it to contain %q", err, want)
			}
		}
	})
}
