// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nico

import (
	"flag"
	"maps"
	"testing"
)

func TestDesiredTopologyLabels(t *testing.T) {
	existing := map[string]string{
		"nke.nvidia.com/site-id":   "site-1",
		"nke.nvidia.com/site-name": "site-one",
		"nke.nvidia.com/unrelated": "keep",
		"team":                     "infra",
	}
	custom := ProviderConfig{TopologyLabelDomain: "nico.example.com"}
	observed := map[string]string{"nico.example.com/site-id": "site-1", "nico.example.com/vpc-id": "vpc-1"}

	t.Run("default domain keeps existing labels and adds observed", func(t *testing.T) {
		got := ProviderConfig{}.DesiredTopologyLabels(existing, map[string]string{"nke.nvidia.com/vpc-id": "vpc-1"})
		want := map[string]string{
			"nke.nvidia.com/site-id":   "site-1",
			"nke.nvidia.com/site-name": "site-one",
			"nke.nvidia.com/unrelated": "keep",
			"nke.nvidia.com/vpc-id":    "vpc-1",
			"team":                     "infra",
		}
		if !maps.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("custom domain migrates legacy keys and keeps unrelated labels", func(t *testing.T) {
		got := custom.DesiredTopologyLabels(existing, observed)
		want := map[string]string{
			"nico.example.com/site-id":   "site-1",
			"nico.example.com/site-name": "site-one", // carried over: no observed name
			"nico.example.com/vpc-id":    "vpc-1",
			"nke.nvidia.com/unrelated":   "keep",
			"team":                       "infra",
		}
		if !maps.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("observed value wins over the legacy value", func(t *testing.T) {
		got := custom.DesiredTopologyLabels(existing, map[string]string{"nico.example.com/site-name": "renamed"})
		if got["nico.example.com/site-name"] != "renamed" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("inputs are not modified", func(t *testing.T) {
		custom.DesiredTopologyLabels(existing, observed)
		if len(existing) != 4 || len(observed) != 2 {
			t.Fatalf("inputs modified: %v %v", existing, observed)
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
		cfg := parse(t, "--topology-label-domain=nico.example.com")
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if got := cfg.TopologyLabelKey(TopologyLabelMachineID); got != "nico.example.com/machine-id" {
			t.Fatalf("TopologyLabelKey() = %q", got)
		}
	})

	t.Run("rejects an invalid domain", func(t *testing.T) {
		cfg := parse(t, "--topology-label-domain=Not_A_Domain/")
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
	})
}
