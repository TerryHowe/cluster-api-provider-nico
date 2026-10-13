// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"maps"
	"testing"

	infrav1 "github.com/dsx-ai-factory/cluster-api-provider-nico/api/v1alpha1"
	"github.com/dsx-ai-factory/cluster-api-provider-nico/internal/nico"
)

func topologyTestMachine() *infrav1.NicoMachine {
	m := &infrav1.NicoMachine{}
	m.Status.MachineID = "machine-1"
	m.Status.SiteID = "site-1"
	m.Status.SiteName = "Site One"
	m.Status.VPCID = "vpc-1"
	m.Status.VPCName = "VPC One"
	return m
}

func TestObservedTopologyLabelsUsesConfiguredDomain(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg    nico.ProviderConfig
		domain string
	}{
		"zero value defaults to legacy": {domain: "nke.nvidia.com"},
		"configured domain":             {cfg: nico.ProviderConfig{TopologyLabelDomain: "nico.example.com"}, domain: "nico.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			want := map[string]string{
				tc.domain + "/machine-id": "machine-1",
				tc.domain + "/site-id":    "site-1",
				tc.domain + "/site-name":  "site-one",
				tc.domain + "/vpc-id":     "vpc-1",
				tc.domain + "/vpc-name":   "vpc-one",
			}
			if got := observedTopologyLabels(tc.cfg, topologyTestMachine()); !maps.Equal(got, want) {
				t.Fatalf("observedTopologyLabels() = %v, want %v", got, want)
			}
		})
	}
}

func TestWithoutLegacyTopologyLabels(t *testing.T) {
	existing := map[string]string{
		"nke.nvidia.com/site-id":   "site-1",
		"nke.nvidia.com/vpc-id":    "vpc-1",
		"nke.nvidia.com/unrelated": "keep",
		"team":                     "infra",
	}

	t.Run("default domain leaves labels untouched", func(t *testing.T) {
		got := withoutLegacyTopologyLabels(existing, nico.ProviderConfig{})
		if !maps.Equal(got, existing) {
			t.Fatalf("got %v, want %v", got, existing)
		}
	})

	t.Run("custom domain drops legacy topology keys only", func(t *testing.T) {
		cfg := nico.ProviderConfig{TopologyLabelDomain: "nico.example.com"}
		got := withoutLegacyTopologyLabels(existing, cfg)
		want := map[string]string{"nke.nvidia.com/unrelated": "keep", "team": "infra"}
		if !maps.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		if len(existing) != 4 {
			t.Fatalf("input map was modified: %v", existing)
		}
	})

	t.Run("migrates legacy values to the configured domain", func(t *testing.T) {
		cfg := nico.ProviderConfig{TopologyLabelDomain: "nico.example.com"}
		desired := mergeLabels(withoutLegacyTopologyLabels(existing, cfg), observedTopologyLabels(cfg, topologyTestMachine()))
		if _, ok := desired["nke.nvidia.com/site-id"]; ok {
			t.Fatalf("legacy key survived: %v", desired)
		}
		if desired["nico.example.com/site-id"] != "site-1" || desired["team"] != "infra" {
			t.Fatalf("desired = %v", desired)
		}
	})
}
