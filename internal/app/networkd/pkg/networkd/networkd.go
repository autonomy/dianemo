// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package networkd handles the network interface configuration on a host.
// If no configuration is provided, automatic configuration via dhcp will
// be performed on interfaces ( eth, en, bond ).
package networkd

import (
	"log"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
)

// Set up default nameservers
const (
	DefaultPrimaryResolver   = "1.1.1.1"
	DefaultSecondaryResolver = "8.8.8.8"
)

// Networkd provides the high level interaction to configure network interfaces
// on a host system. This currently supports addressing configuration via dhcp
// and/or a specified configuration file.
type Networkd struct {
	Interfaces map[string]*nic.NetworkInterface

	hostname  string
	resolvers []string
}

// New takes the supplied configuration and creates an abstract representation
// of all interfaces (as nic.NetworkInterface).
// nolint: gocyclo
func New(config runtime.Configurator) (*Networkd, error) {
	var (
		hostname  string
		result    *multierror.Error
		resolvers []string
	)

	resolvers = []string{DefaultPrimaryResolver, DefaultSecondaryResolver}

	netconf := make(map[string][]nic.Option)

	// Gather settings for all config driven interfaces
	if config != nil {
		log.Println("parsing configuration file")

		for _, device := range config.Machine().Network().Devices() {
			name, opts, err := buildOptions(device)
			if err != nil {
				result = multierror.Append(result, err)
				continue
			}

			netconf[name] = opts
		}

		hostname = config.Machine().Network().Hostname()

		if len(config.Machine().Network().Resolvers()) > 0 {
			resolvers = config.Machine().Network().Resolvers()
		}
	}

	log.Println("discovering local interfaces")

	// Gather already present interfaces
	localInterfaces, err := net.Interfaces()
	if err != nil {
		result = multierror.Append(result, err)
		return &Networkd{}, result.ErrorOrNil()
	}

	// Add locally discovered interfaces to our list of interfaces
	// if they are not already present
	for _, device := range filterInterfaceByName(localInterfaces) {
		if _, ok := netconf[device.Name]; !ok {
			netconf[device.Name] = []nic.Option{nic.WithName(device.Name)}

			// Explicitly ignore bonded interfaces if no configuration was specified
			// This should speed up initial boot times since an unconfigured bond
			// does not provide any value.
			if strings.HasPrefix(device.Name, "bond") {
				netconf[device.Name] = append(netconf[device.Name], nic.WithIgnore())
			}
		}

		// Ensure lo has proper loopback address
		// Ensure MTU for loopback is 64k
		// ref: https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=0cf833aefaa85bbfce3ff70485e5534e09254773
		if strings.HasPrefix(device.Name, "lo") {
			netconf[device.Name] = append(netconf[device.Name], nic.WithAddressing(
				&address.Static{
					Device: &machine.Device{
						CIDR: "127.0.0.1/8",
						MTU:  nic.MaximumMTU,
					},
				},
			))
		}
	}

	interfaces := make(map[string]*nic.NetworkInterface)

	// Create nic.NetworkInterface representation of the interface
	for ifname, opts := range netconf {
		netif, err := nic.New(opts...)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}

		interfaces[ifname] = netif
	}

	// Set interfaces that are part of a bond to ignored
	for _, netif := range interfaces {
		if !netif.Bonded {
			continue
		}

		for _, subif := range netif.SubInterfaces {
			interfaces[subif.Name].Ignore = true
		}
	}

	return &Networkd{Interfaces: interfaces, hostname: hostname, resolvers: resolvers}, result.ErrorOrNil()
}

// Configure handles the lifecycle for an interface. This includes creation,
// configuration, and any addressing that is needed. We care about ordering
// here so that we can ensure any links that make up a bond will be in
// the correct state when we get to bonding configuration.
// nolint: gocyclo
func (n *Networkd) Configure() (err error) {
	var wg sync.WaitGroup

	log.Println("configuring non-bonded interfaces")

	for _, iface := range n.Interfaces {
		// Skip bonded interfaces so we can ensure a proper base state
		if iface.Bonded {
			continue
		}

		wg.Add(1)

		go func(netif *nic.NetworkInterface, wg *sync.WaitGroup) {
			defer wg.Done()

			// Ensure link exists
			if err = netif.Create(); err != nil {
				log.Println("error creating nic", err)
				return
			}

			if err = netif.Configure(); err != nil {
				log.Println("error configuring nic", err)
				return
			}

			if err = netif.Addressing(); err != nil {
				log.Println("error configuring addressing", err)
				return
			}
		}(iface, &wg)
	}

	wg.Wait()

	log.Println("configuring bonded interfaces")

	for _, iface := range n.Interfaces {
		// Only work on bonded interfaces
		if !iface.Bonded {
			continue
		}

		wg.Add(1)

		go func(netif *nic.NetworkInterface, wg *sync.WaitGroup) {
			defer wg.Done()

			log.Printf("setting up %s", netif.Name)

			// Ensure link exists
			if err = netif.Create(); err != nil {
				log.Println("error creating nic", err)
				return
			}

			if err = netif.Configure(); err != nil {
				log.Println("error configuring nic", err)
				return
			}

			if err = netif.Addressing(); err != nil {
				log.Println("error configuring addressing", err)
				return
			}
		}(iface, &wg)
	}

	wg.Wait()

	resolvers := []string{}

	for _, netif := range n.Interfaces {
		for _, method := range netif.AddressMethod {
			if !method.Valid() {
				continue
			}

			for _, resolver := range method.Resolvers() {
				resolvers = append(resolvers, resolver.String())
			}
		}
	}

	if len(resolvers) == 0 {
		resolvers = n.resolvers
	}

	return writeResolvConf(resolvers)
}

// Renew sets up a long running loop to refresh a network interfaces
// addressing configuration. Currently this only applies to interfaces
// configured by DHCP.
func (n *Networkd) Renew() {
	for _, iface := range n.Interfaces {
		iface.Renew()
	}
}

// Hostname returns the first hostname found from the addressing methods.
func (n *Networkd) Hostname() string {
	// Allow user supplied hostname to override what was returned
	// during dhcp
	if n.hostname != "" {
		return n.hostname
	}

	for _, iface := range n.Interfaces {
		for _, method := range iface.AddressMethod {
			if !method.Valid() {
				continue
			}

			if method.Hostname() != "" {
				return method.Hostname()
			}
		}
	}

	return ""
}

// Reset handles removing addresses from previously configured interfaces.
func (n *Networkd) Reset() {
	for _, iface := range n.Interfaces {
		iface.Reset()
	}
}
