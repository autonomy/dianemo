/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"log"
	"time"

	"github.com/autonomy/dhcp/dhcpv4"
	"github.com/autonomy/dhcp/dhcpv4/client4"
	"github.com/autonomy/dhcp/netboot"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Setup creates the network.
func Setup(platform string) (err error) {
	link, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}
	if err = netlink.LinkSetUp(link); err != nil {
		return err
	}

	ifname := "eth0"
	if _, err = netboot.IfUp(ifname, 5*time.Second); err != nil {
		return err
	}
	optionList := dhcpv4.OptionCodeList{
		dhcpv4.OptionSubnetMask,
		dhcpv4.OptionBroadcastAddress,
		dhcpv4.OptionTimeOffset,
		dhcpv4.OptionRouter,
		dhcpv4.OptionDomainName,
		dhcpv4.OptionDomainNameServer,
		dhcpv4.OptionDNSDomainSearchList,
		dhcpv4.OptionHostName,
		dhcpv4.OptionNetBIOSOverTCPIPNameServer,
		dhcpv4.OptionNetBIOSOverTCPIPScope,
		dhcpv4.OptionInterfaceMTU,
		dhcpv4.OptionClasslessStaticRouteOption,
		dhcpv4.OptionNTPServers,
	}
	modifiers := []dhcpv4.Modifier{dhcpv4.WithRequestedOptions(optionList...)}
	var netconf *netboot.NetConf
	if netconf, err = dhclient4(ifname, modifiers...); err != nil {
		return err
	}
	if err = netboot.ConfigureInterface(ifname, netconf); err != nil {
		return err
	}

	return nil
}

// nolint: gocyclo
func dhclient4(ifname string, modifiers ...dhcpv4.Modifier) (*netboot.NetConf, error) {
	attempts := 10
	client := client4.NewClient()
	var (
		conv []*dhcpv4.DHCPv4
		err  error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		log.Printf("requesting DHCP lease: attempt %d of %d", attempt+1, attempts)
		conv, err = client.Exchange(ifname, modifiers...)
		if err != nil && attempt < attempts {
			log.Printf("failed to request DHCP lease: %v", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		break
	}

	for _, m := range conv {
		if m.OpCode == dhcpv4.OpcodeBootReply && m.MessageType() == dhcpv4.MessageTypeOffer {
			if m.YourIPAddr != nil {
				log.Printf("using IP address %s", m.YourIPAddr.String())
			}

			hostname := m.YourIPAddr.String()
			if m.HostName() != "" {
				hostname = m.HostName()
			}
			log.Printf("using hostname: %s", hostname)
			if err = unix.Sethostname([]byte(hostname)); err != nil {
				return nil, err
			}
			break
		}
	}

	netconf, _, err := netboot.ConversationToNetconfv4(conv)
	if err != nil {
		return nil, err
	}

	return netconf, err
}
