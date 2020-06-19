// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"io/ioutil"
	"os"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ValidateSuite verifies dmesg command
type ValidateSuite struct {
	base.CLISuite

	tmpDir   string
	savedCwd string
}

// SuiteName ...
func (suite *ValidateSuite) SuiteName() string {
	return "cli.ValidateSuite"
}

func (suite *ValidateSuite) SetupTest() {
	var err error
	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)

	suite.Require().NoError(os.Chdir(suite.tmpDir))
}

func (suite *ValidateSuite) TearDownTest() {
	suite.Require().NoError(os.Chdir(suite.savedCwd))
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

// TestValidate generates config and validates it for all the modes.
func (suite *ValidateSuite) TestValidate() {
	suite.RunCLI([]string{"gen", "config", "foobar", "https://10.0.0.1"})

	for _, configFile := range []string{"init.yaml", "controlplane.yaml", "join.yaml"} {
		for _, mode := range []string{"cloud", "container", "metal"} {
			suite.RunCLI([]string{"validate", "-m", mode, "-c", configFile})
		}
	}
}

func init() {
	allSuites = append(allSuites, new(ValidateSuite))
}
