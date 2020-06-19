// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"fmt"
	"sort"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/client"
)

// ErrServiceNotFound is an error that indicates that a service was not found.
var ErrServiceNotFound = fmt.Errorf("service not found")

// ServiceStateAssertion checks whether service reached some specified state.
//
//nolint: gocyclo
func ServiceStateAssertion(ctx context.Context, cluster ClusterInfo, service string, states ...string) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	var node string

	switch {
	case len(cluster.NodesByType(runtime.MachineTypeInit)) > 0:
		nodes := cluster.NodesByType(runtime.MachineTypeInit)
		if len(nodes) != 1 {
			return fmt.Errorf("expected 1 init node, got %d", len(nodes))
		}

		node = nodes[0]
	case len(cluster.NodesByType(runtime.MachineTypeControlPlane)) > 0:
		nodes := cluster.NodesByType(runtime.MachineTypeControlPlane)

		sort.Strings(nodes)

		node = nodes[0]
	default:
		return fmt.Errorf("no bootstrap node found")
	}

	nodeCtx := client.WithNodes(ctx, node)

	servicesInfo, err := cli.ServiceInfo(nodeCtx, service)
	if err != nil {
		return err
	}

	serviceOk := false

	acceptedStates := map[string]struct{}{}
	for _, state := range states {
		acceptedStates[state] = struct{}{}
	}

	for _, serviceInfo := range servicesInfo {
		if len(serviceInfo.Service.Events.Events) == 0 {
			return fmt.Errorf("no events recorded yet for service %q", service)
		}

		lastEvent := serviceInfo.Service.Events.Events[len(serviceInfo.Service.Events.Events)-1]
		if _, ok := acceptedStates[lastEvent.State]; !ok {
			return fmt.Errorf("service %q not in expected state %q: current state [%s] %s", service, states, lastEvent.State, lastEvent.Msg)
		}

		serviceOk = true
	}

	if !serviceOk {
		return ErrServiceNotFound
	}

	return nil
}

// ServiceHealthAssertion checks whether service reached some specified state.
//nolint: gocyclo
func ServiceHealthAssertion(ctx context.Context, cluster ClusterInfo, service string, setters ...Option) error {
	opts := DefaultOptions()

	for _, setter := range setters {
		if err := setter(opts); err != nil {
			return err
		}
	}

	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	var nodes []string

	if len(opts.Types) > 0 {
		for _, t := range opts.Types {
			nodes = append(nodes, cluster.NodesByType(t)...)
		}
	} else {
		nodes = cluster.Nodes()
	}

	count := len(nodes)

	nodesCtx := client.WithNodes(ctx, nodes...)

	servicesInfo, err := cli.ServiceInfo(nodesCtx, service)
	if err != nil {
		return err
	}

	if len(servicesInfo) != count {
		return fmt.Errorf("expected a response with %d node(s), got %d", count, len(servicesInfo))
	}

	for _, serviceInfo := range servicesInfo {
		if len(serviceInfo.Service.Events.Events) == 0 {
			return fmt.Errorf("no events recorded yet for service %q", service)
		}

		lastEvent := serviceInfo.Service.Events.Events[len(serviceInfo.Service.Events.Events)-1]
		if lastEvent.State != "Running" {
			return fmt.Errorf("service %q not in expected state %q: current state [%s] %s", service, "Running", lastEvent.State, lastEvent.Msg)
		}

		if !serviceInfo.Service.GetHealth().GetHealthy() {
			return fmt.Errorf("service is not healthy: %s", service)
		}
	}

	return nil
}
