// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import v1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1"

// GenOption controls generate options specific to input generation.
type GenOption func(o *GenOptions) error

// WithEndpointList specifies endpoints to use when acessing Talos cluster.
func WithEndpointList(endpoints []string) GenOption {
	return func(o *GenOptions) error {
		o.EndpointList = endpoints

		return nil
	}
}

// WithInstallDisk specifies install disk to use in Talos cluster.
func WithInstallDisk(disk string) GenOption {
	return func(o *GenOptions) error {
		o.InstallDisk = disk

		return nil
	}
}

// WithAdditionalSubjectAltNames specifies additional SANs.
func WithAdditionalSubjectAltNames(sans []string) GenOption {
	return func(o *GenOptions) error {
		o.AdditionalSubjectAltNames = sans

		return nil
	}
}

// WithInstallImage specifies install container image to use in Talos cluster.
func WithInstallImage(imageRef string) GenOption {
	return func(o *GenOptions) error {
		o.InstallImage = imageRef

		return nil
	}
}

// WithNetworkConfig allows to pass network config to be used.
func WithNetworkConfig(network *v1alpha1.NetworkConfig) GenOption {
	return func(o *GenOptions) error {
		o.NetworkConfig = network

		return nil
	}
}

// GenOptions describes generate parameters.
type GenOptions struct {
	EndpointList              []string
	InstallDisk               string
	InstallImage              string
	AdditionalSubjectAltNames []string
	NetworkConfig             *v1alpha1.NetworkConfig
}

// DefaultGenOptions returns default options.
func DefaultGenOptions() GenOptions {
	return GenOptions{}
}
