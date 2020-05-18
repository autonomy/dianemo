// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	stdlibx509 "crypto/x509"
	stdlibnet "net"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"go.etcd.io/etcd/clientv3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/retry"
)

// Etcd implements the Service interface. It serves as the concrete type with
// the required methods.
type Etcd struct {
	Bootstrap bool

	args []string
}

// ID implements the Service interface.
func (e *Etcd) ID(r runtime.Runtime) string {
	return "etcd"
}

// PreFunc implements the Service interface.
func (e *Etcd) PreFunc(ctx context.Context, r runtime.Runtime) (err error) {
	errCh := make(chan error, 1)

	go e.setup(ctx, r, errCh)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// PostFunc implements the Service interface.
func (e *Etcd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (e *Etcd) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (e *Etcd) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd", "networkd"}
}

// Runner implements the Service interface.
func (e *Etcd) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := runner.Args{
		ID:          e.ID(r),
		ProcessArgs: append([]string{"/usr/local/bin/etcd"}, e.args...),
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.EtcdPKIPath, Source: constants.EtcdPKIPath, Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: constants.EtcdDataPath, Source: constants.EtcdDataPath, Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithNamespace(constants.SystemContainerdNamespace),
		runner.WithContainerImage(r.Config().Cluster().Etcd().Image()),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (e *Etcd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
		if err != nil {
			return err
		}

		return client.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (e *Etcd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}

// nolint: gocyclo
func generatePKI(r runtime.Runtime) (err error) {
	if err = os.MkdirAll(constants.EtcdPKIPath, 0644); err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCACert, r.Config().Cluster().Etcd().CA().Crt, 0500); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCAKey, r.Config().Cluster().Etcd().CA().Key, 0500); err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	ips, err := net.IPAddrs()
	if err != nil {
		return fmt.Errorf("failed to discover IP addresses: %w", err)
	}

	ips = append(ips, stdlibnet.ParseIP("127.0.0.1"))
	if net.IsIPv6(ips...) {
		ips = append(ips, stdlibnet.ParseIP("::1"))
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	dnsNames, err := net.DNSNames()
	if err != nil {
		return fmt.Errorf("failed to get host DNS names: %w", err)
	}

	dnsNames = append(dnsNames, "localhost")

	opts := []x509.Option{
		x509.CommonName(hostname),
		x509.DNSNames(dnsNames),
		x509.RSA(true),
		x509.IPAddresses(ips),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	peerKey, err := x509.NewRSAKey()
	if err != nil {
		return fmt.Errorf("failled to create RSA key: %w", err)
	}

	pemBlock, _ := pem.Decode(peerKey.KeyPEM)
	if pemBlock == nil {
		return errors.New("failed to decode peer key pem")
	}

	peerKeyRSA, err := stdlibx509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failled to parse private key: %w", err)
	}

	csr, err := x509.NewCertificateSigningRequest(peerKeyRSA, opts...)
	if err != nil {
		return fmt.Errorf("failed to create CSR: %w", err)
	}

	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return errors.New("failed to decode csr pem")
	}

	ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failled to parse certificate request: %w", err)
	}

	caPemBlock, _ := pem.Decode(r.Config().Cluster().Etcd().CA().Crt)
	if caPemBlock == nil {
		return errors.New("failed to decode ca cert pem")
	}

	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA: %w", err)
	}

	caKeyPemBlock, _ := pem.Decode(r.Config().Cluster().Etcd().CA().Key)
	if caKeyPemBlock == nil {
		return errors.New("failed to decode ca key pem")
	}

	caKey, err := stdlibx509.ParsePKCS1PrivateKey(caKeyPemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key: %w", err)
	}

	peer, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr, opts...)
	if err != nil {
		return fmt.Errorf("failled to create peer certificate: %w", err)
	}

	if err := ioutil.WriteFile(constants.KubernetesEtcdPeerKey, peerKey.KeyPEM, 0500); err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubernetesEtcdPeerCert, peer.X509CertificatePEM, 0500); err != nil {
		return err
	}

	return nil
}

func addMember(r runtime.Runtime, addrs []string, name string) (*clientv3.MemberListResponse, uint64, error) {
	client, err := etcd.NewClientFromControlPlaneIPs(r.Config().Cluster().CA(), r.Config().Cluster().Endpoint())
	if err != nil {
		return nil, 0, err
	}

	// nolint: errcheck
	defer client.Close()

	list, err := client.MemberList(context.Background())
	if err != nil {
		return nil, 0, err
	}

	for _, member := range list.Members {
		if member.Name == name {
			return list, member.ID, nil
		}
	}

	add, err := client.MemberAdd(context.Background(), addrs)
	if err != nil {
		return nil, 0, err
	}

	list, err = client.MemberList(context.Background())
	if err != nil {
		return nil, 0, err
	}

	return list, add.Member.ID, nil
}

func buildInitialCluster(r runtime.Runtime, name, ip string) (initial string, err error) {
	err = retry.Constant(10*time.Minute, retry.WithUnits(3*time.Second), retry.WithJitter(time.Second)).Retry(func() error {
		var (
			peerAddrs = []string{"https://" + net.FormatAddress(ip) + ":2380"}
			resp      *clientv3.MemberListResponse
			id        uint64
		)

		resp, id, err = addMember(r, peerAddrs, name)
		if err != nil {
			// TODO(andrewrynhard): We should check the error type here and
			// handle the specific error accordingly.
			return retry.ExpectedError(err)
		}

		conf := []string{}

		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == id {
					n = name
				}

				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}
		}

		initial = strings.Join(conf, ",")

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to build cluster arguments: %w", err)
	}

	return initial, nil
}

// // nolint: gocyclo
func (e *Etcd) argsForInit(r runtime.Runtime) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	p, err := platform.CurrentPlatform()
	if err != nil {
		return err
	}

	var upgraded bool

	if p.Mode() != runtime.ModeContainer {
		var f *os.File

		if f, err = os.Open(syslinux.SyslinuxLdlinux); err != nil {
			return err
		}

		// nolint: errcheck
		defer f.Close()

		var adv syslinux.ADV

		if adv, err = syslinux.NewADV(f); err != nil {
			return err
		}

		_, upgraded = adv.ReadTag(syslinux.AdvUpgrade)
	}

	primaryAddr, listenAddress, err := primaryAndListenAddresses()
	if err != nil {
		return fmt.Errorf("failed to calculate etcd addresses: %w", err)
	}

	// TODO(scm): see issue #2121 and description below in argsForControlPlane.
	blackListArgs := argsbuilder.Args{
		"name":                  hostname,
		"data-dir":              constants.EtcdDataPath,
		"listen-peer-urls":      "https://" + net.FormatAddress(listenAddress) + ":2380",
		"listen-client-urls":    "https://" + net.FormatAddress(listenAddress) + ":2379",
		"cert-file":             constants.KubernetesEtcdPeerCert,
		"key-file":              constants.KubernetesEtcdPeerKey,
		"trusted-ca-file":       constants.KubernetesEtcdCACert,
		"peer-client-cert-auth": "true",
		"peer-cert-file":        constants.KubernetesEtcdPeerCert,
		"peer-trusted-ca-file":  constants.KubernetesEtcdCACert,
		"peer-key-file":         constants.KubernetesEtcdPeerKey,
	}

	extraArgs := argsbuilder.Args(r.Config().Cluster().Etcd().ExtraArgs())

	for k := range blackListArgs {
		if extraArgs.Contains(k) {
			return argsbuilder.NewBlacklistError(k)
		}
	}

	if !extraArgs.Contains("initial-cluster-state") {
		blackListArgs.Set("initial-cluster-state", "new")
	}

	// If the initial cluster isn't explicitly defined, we need to discover any
	// existing members.
	if !extraArgs.Contains("initial-cluster") {
		ok, err := IsDirEmpty(constants.EtcdDataPath)
		if err != nil {
			return err
		}

		if ok {
			initialCluster := fmt.Sprintf("%s=https://%s:2380", hostname, net.FormatAddress(primaryAddr))

			if upgraded {
				blackListArgs.Set("initial-cluster-state", "existing")

				initialCluster, err = buildInitialCluster(r, hostname, primaryAddr)
				if err != nil {
					return err
				}
			}

			blackListArgs.Set("initial-cluster", initialCluster)
		} else {
			blackListArgs.Set("initial-cluster-state", "existing")
		}
	}

	if !extraArgs.Contains("initial-advertise-peer-urls") {
		blackListArgs.Set("initial-advertise-peer-urls", fmt.Sprintf("https://%s:2380", net.FormatAddress(primaryAddr)))
	}

	if !extraArgs.Contains("advertise-client-urls") {
		blackListArgs.Set("advertise-client-urls", fmt.Sprintf("https://%s:2379", net.FormatAddress(primaryAddr)))
	}

	e.args = blackListArgs.Merge(extraArgs).Args()

	return nil
}

func (e *Etcd) setup(ctx context.Context, r runtime.Runtime, errCh chan error) {
	errCh <- func() error {
		var err error

		if err = os.MkdirAll(constants.EtcdDataPath, 0755); err != nil {
			return err
		}

		if err = generatePKI(r); err != nil {
			return fmt.Errorf("failed to generate etcd PKI: %w", err)
		}

		client, err := containerdapi.New(constants.ContainerdAddress)
		if err != nil {
			return err
		}
		// nolint: errcheck
		defer client.Close()

		// Pull the image and unpack it.
		containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)
		if _, err = image.Pull(containerdctx, r.Config().Machine().Registries(), client, r.Config().Cluster().Etcd().Image()); err != nil {
			return fmt.Errorf("failed to pull image %q: %w", r.Config().Cluster().Etcd().Image(), err)
		}

		switch r.Config().Machine().Type() {
		case runtime.MachineTypeInit:
			err = e.argsForInit(r)
			if err != nil {
				return err
			}
		case runtime.MachineTypeControlPlane:
			err = e.argsForControlPlane(r)
			if err != nil {
				return err
			}
		}

		return nil
	}()
}

// nolint: gocyclo
func (e *Etcd) argsForControlPlane(r runtime.Runtime) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	// TODO(scm):  With the current setup, the listen (bind) address is
	// essentially hard-coded because we need to calculate it before we process
	// extraArgs (which may contain special overrides from the user.
	// This needs to be refactored to allow greater binding flexibility.
	// Issue #2121.
	primaryAddr, listenAddress, err := primaryAndListenAddresses()
	if err != nil {
		return fmt.Errorf("failed to calculate etcd addresses: %w", err)
	}

	blackListArgs := argsbuilder.Args{
		"name":                  hostname,
		"data-dir":              constants.EtcdDataPath,
		"listen-peer-urls":      "https://" + net.FormatAddress(listenAddress) + ":2380",
		"listen-client-urls":    "https://" + net.FormatAddress(listenAddress) + ":2379",
		"cert-file":             constants.KubernetesEtcdPeerCert,
		"key-file":              constants.KubernetesEtcdPeerKey,
		"trusted-ca-file":       constants.KubernetesEtcdCACert,
		"peer-client-cert-auth": "true",
		"peer-cert-file":        constants.KubernetesEtcdPeerCert,
		"peer-trusted-ca-file":  constants.KubernetesEtcdCACert,
		"peer-key-file":         constants.KubernetesEtcdPeerKey,
	}

	extraArgs := argsbuilder.Args(r.Config().Cluster().Etcd().ExtraArgs())

	for k := range blackListArgs {
		if extraArgs.Contains(k) {
			return argsbuilder.NewBlacklistError(k)
		}
	}

	ok, err := IsDirEmpty(constants.EtcdDataPath)
	if err != nil {
		return err
	}

	// The only time that we need to build the initial cluster args, is when we
	// don't have any data.
	if ok {
		if !extraArgs.Contains("initial-cluster-state") {
			if e.Bootstrap {
				blackListArgs.Set("initial-cluster-state", "new")
			} else {
				blackListArgs.Set("initial-cluster-state", "existing")
			}
		}

		if !extraArgs.Contains("initial-cluster") {
			var initialCluster string

			if e.Bootstrap {
				initialCluster = fmt.Sprintf("%s=https://%s:2380", hostname, net.FormatAddress(primaryAddr))
			} else {
				initialCluster, err = buildInitialCluster(r, hostname, primaryAddr)
				if err != nil {
					return fmt.Errorf("failed to build initial etcd cluster: %w", err)
				}
			}

			blackListArgs.Set("initial-cluster", initialCluster)
		}

		if !extraArgs.Contains("initial-advertise-peer-urls") {
			blackListArgs.Set("initial-advertise-peer-urls", fmt.Sprintf("https://%s:2380", net.FormatAddress(primaryAddr)))
		}
	}

	if !extraArgs.Contains("advertise-client-urls") {
		blackListArgs.Set("advertise-client-urls", fmt.Sprintf("https://%s:2379", net.FormatAddress(primaryAddr)))
	}

	e.args = blackListArgs.Merge(extraArgs).Args()

	return nil
}

// IsDirEmpty checks if a directory is empty or not.
func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	// nolint: errcheck
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}

// primaryAndListenAddresses calculates the primary (advertised) and listen (bind) addresses for etcd.
func primaryAndListenAddresses() (primary string, listen string, err error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return "", "", fmt.Errorf("failed to discover interface IP addresses: %w", err)
	}

	if len(ips) == 0 {
		return "", "", errors.New("no valid unicast IP addresses on any interface")
	}

	// NOTE: we will later likely want to expose the primary IP selection to the
	// user or build it with greater flexibility.  For now, this maintains
	// previous behavior.
	primary = ips[0].String()

	// Regardless of primary selected IP, we should be liberal with our listen
	// address, for maximum compatibility.  Again, this should probably be
	// exposed later for greater control.
	listen = "0.0.0.0"
	if net.IsIPv6(ips...) {
		listen = "::"
	}

	return primary, listen, nil
}
