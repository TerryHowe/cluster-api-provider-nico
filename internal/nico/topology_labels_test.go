// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nico

import (
	"flag"
	"testing"
)

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
