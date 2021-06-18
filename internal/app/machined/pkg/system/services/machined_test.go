// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services //nolint:testpackage // to test unexported variable

import (
	"fmt"
	"testing"

	"google.golang.org/grpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	"github.com/talos-systems/talos/pkg/machinery/api/cluster"
	"github.com/talos-systems/talos/pkg/machinery/api/inspect"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/api/resource"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/api/time"
)

func collectMethods(t *testing.T) map[string]struct{} {
	methods := make(map[string]struct{})

	for _, service := range []grpc.ServiceDesc{
		cluster.ClusterService_ServiceDesc,
		inspect.InspectService_ServiceDesc,
		machine.MachineService_ServiceDesc,
		network.NetworkService_ServiceDesc,
		resource.ResourceService_ServiceDesc,
		// security.SecurityService_ServiceDesc, - not in machined
		storage.StorageService_ServiceDesc,
		time.TimeService_ServiceDesc,
	} {
		for _, method := range service.Methods {
			s := fmt.Sprintf("/%s/%s", service.ServiceName, method.MethodName)
			require.NotContains(t, methods, s)
			methods[s] = struct{}{}
		}

		for _, stream := range service.Streams {
			s := fmt.Sprintf("/%s/%s", service.ServiceName, stream.StreamName)
			require.NotContains(t, methods, s)
			methods[s] = struct{}{}
		}
	}

	return methods
}

func TestRules(t *testing.T) { //nolint:gocyclo
	t.Parallel()

	methods := collectMethods(t)

	// check that there are no rules without matching methods
	t.Run("NoMethodForRule", func(t *testing.T) {
		t.Parallel()

		for rule := range rules {
			var found bool
			for method := range methods {
				prefix := method
				for prefix != "/" {
					if prefix == rule {
						found = true

						break
					}

					prefix = authz.NextPrefix(prefix)
				}

				if found {
					break
				}
			}

			assert.True(t, found, "no method for rule %q", rule)
		}
	})

	// check that there are no methods without matching rules
	t.Run("NoRuleForMethod", func(t *testing.T) {
		t.Parallel()

		for method := range methods {
			var found bool
			for rule := range rules {
				prefix := method
				for prefix != "/" {
					if prefix == rule {
						found = true

						break
					}

					prefix = authz.NextPrefix(prefix)
				}

				if found {
					break
				}
			}

			assert.True(t, found, "no rule for method %q", method)
		}
	})
}
