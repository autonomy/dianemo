// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: scopelint
package runtime

import (
	"reflect"
	"testing"
)

func TestMachineType_String(t *testing.T) {
	tests := []struct {
		name string
		t    MachineType
		want string
	}{
		{
			name: "bootstrap",
			t:    MachineTypeBootstrap,
			want: "bootstrap",
		},
		{
			name: "controlplane",
			t:    MachineTypeControlPlane,
			want: "controlplane",
		},
		{
			name: "worker",
			t:    MachineTypeWorker,
			want: "worker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.String(); got != tt.want {
				t.Errorf("MachineType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseType(t *testing.T) {
	type args struct {
		t string
	}

	tests := []struct {
		name    string
		args    args
		want    MachineType
		wantErr bool
	}{
		{
			name:    "bootstrap",
			args:    args{"bootstrap"},
			want:    MachineTypeBootstrap,
			wantErr: false,
		},
		{
			name:    "controlplane",
			args:    args{"controlplane"},
			want:    MachineTypeControlPlane,
			wantErr: false,
		},
		{
			name:    "worker",
			args:    args{"worker"},
			want:    MachineTypeWorker,
			wantErr: false,
		},
		{
			name:    "invalid",
			args:    args{"invalid"},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseType(tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseType() = %v, want %v", got, tt.want)
			}
		})
	}
}
