// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/installer/pkg/install"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/pkg/version"
)

// installCmd represents the install command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if err := runInstallCmd(); err != nil {
			if err = (err); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstallCmd() (err error) {
	log.Printf("running Talos installer %s", version.NewVersion().Tag)

	seq := runtime.SequenceInstall

	if options.Upgrade {
		seq = runtime.SequenceUpgrade
	}

	p, err := platform.NewPlatform(options.Platform)
	if err != nil {
		return err
	}

	return install.Install(p, seq, options)
}
