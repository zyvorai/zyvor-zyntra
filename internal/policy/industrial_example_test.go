// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package policy

import "testing"

func TestIndustrialExamplePolicyLoads(t *testing.T) {
	if _, err := Load("../../examples/policy-industrial.yaml"); err != nil {
		t.Fatal(err)
	}
}
