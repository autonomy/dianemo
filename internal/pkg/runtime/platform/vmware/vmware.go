// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vmware

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (v *VMware) Name() string {
	return "vmware"
}

// Configuration implements the platform.Platform interface.
func (v *VMware) Configuration() ([]byte, error) {
	ok, err := vmcheck.IsVirtualWorld()
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("not a virtual world")
	}

	config := rpcvmx.NewConfig()

	val, err := config.String(constants.VMwareGuestInfoConfigKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get guestinfo.%s: %w", constants.VMwareGuestInfoConfigKey, err)
	}

	if val == "" {
		return nil, fmt.Errorf("config is required, no value found for guestinfo.%s: %w", constants.VMwareGuestInfoConfigKey, err)
	}

	b, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, fmt.Errorf("failed to decode guestinfo.%s: %w", constants.VMwareGuestInfoConfigKey, err)
	}

	return b, nil
}

// Hostname implements the platform.Platform interface.
func (v *VMware) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// Mode implements the platform.Platform interface.
func (v *VMware) Mode() runtime.Mode {
	return runtime.Cloud
}

// ExternalIPs implements the runtime.Platform interface.
func (v *VMware) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (v *VMware) KernelArgs() kernel.Parameters {
	return []*kernel.Parameter{
		kernel.NewParameter("console").Append("tty0"),
		kernel.NewParameter("earlyprintk").Append("ttyS0,115200"),
	}
}
