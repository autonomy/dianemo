// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/google/uuid"
	"github.com/jsimonetti/rtnetlink"

	"github.com/talos-systems/talos/internal/pkg/provision"
	talosnet "github.com/talos-systems/talos/pkg/net"
)

func (p *provisioner) createNetwork(ctx context.Context, state *state, network provision.NetworkRequest) error {
	// build bridge interface name by taking part of checksum of the network name
	// so that interface name is defined by network name, and different networks have
	// different bridge interfaces
	networkNameHash := sha256.Sum256([]byte(network.Name))
	state.BridgeName = fmt.Sprintf("%s%s", "talos", hex.EncodeToString(networkNameHash[:])[:8])

	// bring up the bridge interface for the first time to get gateway IP assigned
	t := template.Must(template.New("bridge").Parse(bridgeTemplate))

	var buf bytes.Buffer

	err := t.Execute(&buf, struct {
		NetworkName   string
		InterfaceName string
		MTU           string
	}{
		NetworkName:   network.Name,
		InterfaceName: state.BridgeName,
		MTU:           strconv.Itoa(network.MTU),
	})
	if err != nil {
		return err
	}

	bridgeConfig, err := libcni.ConfFromBytes(buf.Bytes())
	if err != nil {
		return err
	}

	cniConfig := libcni.NewCNIConfigWithCacheDir(network.CNI.BinPath, network.CNI.CacheDir, nil)

	ns, err := testutils.NewNS()
	if err != nil {
		return err
	}

	defer func() {
		ns.Close()              //nolint: errcheck
		testutils.UnmountNS(ns) //nolint: errcheck
	}()

	// pick a fake address to use for provisioning an interface
	fakeIP, err := talosnet.NthIPInNetwork(&network.CIDR, 2)
	if err != nil {
		return err
	}

	ones, _ := network.CIDR.IP.DefaultMask().Size()
	containerID := uuid.New().String()
	runtimeConf := libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       ns.Path(),
		IfName:      "veth0",
		Args: [][2]string{
			{"IP", fmt.Sprintf("%s/%d", fakeIP, ones)},
			{"GATEWAY", network.GatewayAddr.String()},
		},
	}

	_, err = cniConfig.AddNetwork(ctx, bridgeConfig, &runtimeConf)
	if err != nil {
		return fmt.Errorf("error provisioning bridge CNI network: %w", err)
	}

	err = cniConfig.DelNetwork(ctx, bridgeConfig, &runtimeConf)
	if err != nil {
		return fmt.Errorf("error deleting bridge CNI network: %w", err)
	}

	// prepare an actual network config to be used by the VMs
	t = template.Must(template.New("network").Parse(networkTemplate))

	f, err := os.Create(filepath.Join(network.CNI.ConfDir, fmt.Sprintf("%s.conflist", network.Name)))
	if err != nil {
		return err
	}

	defer f.Close() //nolint: errcheck

	err = t.Execute(f, struct {
		NetworkName   string
		InterfaceName string
		MTU           string
	}{
		NetworkName:   network.Name,
		InterfaceName: state.BridgeName,
		MTU:           strconv.Itoa(network.MTU),
	})
	if err != nil {
		return err
	}

	return f.Close()
}

func (p *provisioner) destroyNetwork(state *state) error {
	// destroy bridge interface by name to clean up
	iface, err := net.InterfaceByName(state.BridgeName)
	if err != nil {
		return fmt.Errorf("error looking up bridge interface %q: %w", state.BridgeName, err)
	}

	rtconn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rnetlink: %w", err)
	}

	if err = rtconn.Link.Delete(uint32(iface.Index)); err != nil {
		return fmt.Errorf("error deleting bridge interface: %w", err)
	}

	return nil
}

const bridgeTemplate = `
{
	"name": "{{ .NetworkName }}",
	"cniVersion": "0.4.0",
	"type": "bridge",
	"bridge": "{{ .InterfaceName }}",
	"ipMasq": true,
	"isGateway": true,
	"isDefaultGateway": true,
	"ipam": {
		  "type": "static"
	},
	"mtu": {{ .MTU }}
}
`

const networkTemplate = `
{
	"name": "{{ .NetworkName }}",
	"cniVersion": "0.4.0",
	"plugins": [
	  {
		"type": "bridge",
		"bridge": "{{ .InterfaceName }}",
		"ipMasq": true,
		"isGateway": true,
		"isDefaultGateway": true,
		"ipam": {
		  "type": "static"
		},
		"mtu": {{ .MTU }}
	},
	  {
		"type": "firewall"
	  },
	  {
		"type": "tc-redirect-tap"
	  }
	]
}
`
