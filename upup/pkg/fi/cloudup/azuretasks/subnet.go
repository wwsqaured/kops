/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azuretasks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-05-01/network"
	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

// Subnet is an Azure subnet.
// +kops:fitask
type Subnet struct {
	Name      *string
	Lifecycle fi.Lifecycle

	ID                   *string
	ResourceGroup        *ResourceGroup
	VirtualNetwork       *VirtualNetwork
	NatGateway           *NatGateway
	NetworkSecurityGroup *NetworkSecurityGroup

	CIDR   *string
	Shared *bool
}

var (
	_ fi.CloudupTask   = &Subnet{}
	_ fi.CompareWithID = &Subnet{}
)

// CompareWithID returns the Name of the VM Scale Set.
func (s *Subnet) CompareWithID() *string {
	return s.ID
}

// Find discovers the Subnet in the cloud provider.
func (s *Subnet) Find(c *fi.CloudupContext) (*Subnet, error) {
	cloud := c.T.Cloud.(azure.AzureCloud)
	l, err := cloud.Subnet().List(context.TODO(), *s.ResourceGroup.Name, *s.VirtualNetwork.Name)
	if err != nil {
		return nil, err
	}
	var found *network.Subnet
	for _, v := range l {
		if *v.Name == *s.Name {
			found = &v
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	s.ID = found.ID

	fs := &Subnet{
		Name:      s.Name,
		Lifecycle: s.Lifecycle,
		Shared:    s.Shared,
		ResourceGroup: &ResourceGroup{
			Name: s.ResourceGroup.Name,
		},
		VirtualNetwork: &VirtualNetwork{
			Name: s.VirtualNetwork.Name,
		},
		ID:   found.ID,
		CIDR: found.AddressPrefix,
	}
	if found.NatGateway != nil {
		fs.NatGateway = &NatGateway{
			ID: found.NatGateway.ID,
		}
	}
	if found.NetworkSecurityGroup != nil {
		fs.NetworkSecurityGroup = &NetworkSecurityGroup{
			ID: found.NetworkSecurityGroup.ID,
		}
	}

	return fs, nil
}

// Run implements fi.Task.Run.
func (s *Subnet) Run(c *fi.CloudupContext) error {
	return fi.CloudupDefaultDeltaRunMethod(s, c)
}

// CheckChanges returns an error if a change is not allowed.
func (*Subnet) CheckChanges(a, e, changes *Subnet) error {
	if a == nil {
		// Check if required fields are set when a new resource is created.
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		return nil
	}

	// Check if unchangeable fields won't be changed.
	if changes.Name != nil {
		return fi.CannotChangeField("Name")
	}
	return nil
}

// RenderAzure creates or updates a subnet.
func (*Subnet) RenderAzure(t *azure.AzureAPITarget, a, e, changes *Subnet) error {
	if a == nil {
		klog.Infof("Creating a new Subnet with name: %s", fi.ValueOf(e.Name))
	} else {
		klog.Infof("Updating a Subnet with name: %s", fi.ValueOf(e.Name))
	}

	subnet := network.Subnet{
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix: e.CIDR,
		},
	}
	if e.NatGateway != nil {
		subnet.NatGateway = &network.SubResource{
			ID: e.NatGateway.ID,
		}
	}
	if e.NetworkSecurityGroup != nil {
		subnet.NetworkSecurityGroup = &network.SecurityGroup{
			ID: e.NetworkSecurityGroup.ID,
		}
	}

	sn, err := t.Cloud.Subnet().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.VirtualNetwork.Name,
		*e.Name,
		subnet)
	if err != nil {
		return err
	}

	e.ID = sn.ID

	return nil
}
