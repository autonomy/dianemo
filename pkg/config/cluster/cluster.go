// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"net/url"

	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Cluster defines the requirements for a config that pertains to cluster
// related options.
type Cluster interface {
	Endpoint() *url.URL
	Token() Token
	CertSANs() []string
	SetCertSANs([]string)
	CA() *x509.PEMEncodedCertificateAndKey
	AESCBCEncryptionSecret() string
	Config(machine.Type) (string, error)
	Etcd() Etcd
	Network() Network
	LocalAPIServerPort() int
	ExtraManifestURLs() []string
}

// Network defines the requirements for a config that pertains to cluster
// network options.
type Network interface {
	CNI() CNI
	PodCIDR() string
	ServiceCIDR() string
}

// CNI defines the requirements for a config that pertains to Kubernetes
// cni.
type CNI interface {
	Name() string
	URLs() []string
}

// Etcd defines the requirements for a config that pertains to etcd related
// options.
type Etcd interface {
	Image() string
	CA() *x509.PEMEncodedCertificateAndKey
	ExtraArgs() map[string]string
}

// Token defines the requirements for a config that pertains to Kubernetes
// bootstrap token.
type Token interface {
	ID() string
	Secret() string
}
