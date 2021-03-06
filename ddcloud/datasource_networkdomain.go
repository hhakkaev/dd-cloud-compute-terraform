package ddcloud

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func dataSourceNetworkDomain() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceNetworkDomainRead,

		Schema: map[string]*schema.Schema{
			resourceKeyNetworkDomainName: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the network domain",
			},
			resourceKeyNetworkDomainDataCenter: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The Id of the MCP 2.0 datacenter in which the network domain is located",
			},
			resourceKeyNetworkDomainDescription: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The network domain description",
			},
			resourceKeyNetworkDomainPlan: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The plan (service level) for the network domain (ESSENTIALS or ADVANCED)",
			},
			resourceKeyNetworkDomainNatIPv4Address: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The IPv4 address for the network domain's IPv6->IPv4 Source Network Address Translation (SNAT). This is the IPv4 address of the network domain's IPv4 egress",
			},
			resourceKeyNetworkDomainOutsideTransitIPv4Subnet: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The IPv4 subnet for transit outside of the network domain",
			},
		},
	}
}

// Read a network domain data source.
func dataSourceNetworkDomainRead(data *schema.ResourceData, provider interface{}) error {
	name := data.Get(resourceKeyNetworkDomainName).(string)
	dataCenterID := data.Get(resourceKeyNetworkDomainDataCenter).(string)

	log.Printf("Read network domain '%s' in data center '%s'.", name, dataCenterID)

	apiClient := provider.(*providerState).Client()

	networkDomain, err := apiClient.GetNetworkDomainByName(name, dataCenterID)
	if err != nil {
		return err
	}

	if networkDomain != nil {
		log.Printf("Found network domain '%s' ('%s') in data center '%s'.", name, networkDomain.ID, dataCenterID)

		data.SetId(networkDomain.ID)
		data.Set(resourceKeyNetworkDomainDescription, networkDomain.Description)
		data.Set(resourceKeyNetworkDomainPlan, networkDomain.Type)
		data.Set(resourceKeyNetworkDomainNatIPv4Address, networkDomain.NatIPv4Address)
		data.Set(resourceKeyNetworkDomainOutsideTransitIPv4Subnet, fmt.Sprintf(
			"%s/%d",
			networkDomain.OutsideTransitVLANIPv4Subnet.BaseAddress,
			networkDomain.OutsideTransitVLANIPv4Subnet.PrefixSize,
		))
	} else {
		return fmt.Errorf("failed to find network domain '%s' in data center '%s'", name, dataCenterID)
	}

	return nil
}
