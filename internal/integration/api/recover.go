// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"sort"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/machine"
)

type RecoverSuite struct {
	base.K8sSuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *RecoverSuite) SuiteName() string {
	return "api.RecoverSuite"
}

// SetupTest ...
func (suite *RecoverSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	// make sure we abort at some point in time, but give enough room for Recovers
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *RecoverSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestRecoverControlPlane removes the control plane components and attempts to recover them with the recover API.
func (suite *RecoverSuite) TestRecoverControlPlane() {
	if !suite.Capabilities().SupportsRecover {
		suite.T().Skip("cluster doesn't support recovery")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state recover test is not reliable (can't wait for cluster readiness)")
	}

	pods, err := suite.Clientset.CoreV1().Pods("kube-system").List(suite.ctx, metav1.ListOptions{
		LabelSelector: "k8s-app in (kube-scheduler,kube-controller-manager)",
	})

	suite.Assert().NoError(err)

	var eg errgroup.Group

	for _, pod := range pods.Items {
		pod := pod

		eg.Go(func() error {
			suite.T().Logf("Deleting %s", pod.GetName())

			err := suite.Clientset.CoreV1().Pods(pod.GetNamespace()).Delete(suite.ctx, pod.GetName(), metav1.DeleteOptions{})

			return err
		})
	}

	suite.Assert().NoError(eg.Wait())

	nodes := suite.DiscoverNodes().NodesByType(machine.TypeControlPlane)
	suite.Require().NotEmpty(nodes)

	sort.Strings(nodes)

	node := nodes[0]

	suite.T().Log("Recovering control plane")

	ctx, ctxCancel := context.WithTimeout(suite.ctx, 5*time.Minute)
	defer ctxCancel()

	nodeCtx := client.WithNodes(ctx, node)

	in := &machineapi.RecoverRequest{
		Source: machineapi.RecoverRequest_APISERVER,
	}

	_, err = suite.Client.MachineClient.Recover(nodeCtx, in)

	suite.Assert().NoError(err)

	// NB: using `ctx` here to have client talking to init node by default
	suite.AssertClusterHealthy(ctx)
}

func init() {
	allSuites = append(allSuites, new(RecoverSuite))
}
