// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package archiver provides a service to archive part of the filesystem into tar archive
package archiver_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/archiver"
)

type WalkerSuite struct {
	CommonSuite
}

func (suite *WalkerSuite) TestIterationDir() {
	ch, err := archiver.Walker(context.Background(), suite.tmpDir, archiver.WithSkipRoot())
	suite.Require().NoError(err)

	relPaths := []string(nil)

	for fi := range ch {
		suite.Require().NoError(fi.Error)
		relPaths = append(relPaths, fi.RelPath)

		if fi.RelPath == "usr/bin/mv" {
			suite.Assert().Equal("/usr/bin/cp", fi.Link)
		}
	}

	suite.Assert().Equal([]string{
		"dev", "dev/random",
		"etc", "etc/certs", "etc/certs/ca.crt", "etc/hostname",
		"lib", "lib/dynalib.so",
		"usr", "usr/bin", "usr/bin/cp", "usr/bin/mv",
	},
		relPaths)
}

func (suite *WalkerSuite) TestIterationMaxRecurseDepth() {
	ch, err := archiver.Walker(context.Background(), suite.tmpDir, archiver.WithMaxRecurseDepth(1))
	suite.Require().NoError(err)

	relPaths := []string(nil)

	for fi := range ch {
		suite.Require().NoError(fi.Error)
		relPaths = append(relPaths, fi.RelPath)
	}

	suite.Assert().Equal([]string{
		".", "dev", "etc", "lib", "usr",
	},
		relPaths)
}

func (suite *WalkerSuite) TestIterationFile() {
	ch, err := archiver.Walker(context.Background(), filepath.Join(suite.tmpDir, "usr/bin/cp"))
	suite.Require().NoError(err)

	relPaths := []string(nil)

	for fi := range ch {
		suite.Require().NoError(fi.Error)
		relPaths = append(relPaths, fi.RelPath)
	}

	suite.Assert().Equal([]string{"cp"},
		relPaths)
}

func (suite *WalkerSuite) TestIterationSymlink() {
	original := filepath.Join(suite.tmpDir, "original")
	err := os.Mkdir(original, 0755)
	suite.Require().NoError(err)

	newname := filepath.Join(suite.tmpDir, "new")

	// NB: We make this a relative symlink to make the test more complete.
	err = os.Symlink("original", newname)
	suite.Require().NoError(err)

	err = ioutil.WriteFile(filepath.Join(original, "original.txt"), []byte{}, 0666)
	suite.Require().NoError(err)

	ch, err := archiver.Walker(context.Background(), newname)
	suite.Require().NoError(err)

	relPaths := []string(nil)

	for fi := range ch {
		suite.Require().NoError(fi.Error)
		relPaths = append(relPaths, fi.RelPath)
	}

	suite.Assert().Equal([]string{".", "original.txt"}, relPaths)
}

func (suite *WalkerSuite) TestIterationNotFound() {
	_, err := archiver.Walker(context.Background(), filepath.Join(suite.tmpDir, "doesntlivehere"))
	suite.Require().Error(err)
}

func TestWalkerSuite(t *testing.T) {
	suite.Run(t, new(WalkerSuite))
}
