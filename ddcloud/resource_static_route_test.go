package ddcloud

import (
	"fmt"
	"testing"

	"github.com/hhakkaev/go-dd-cloud-compute/compute"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

var staticRouteName string = "acc_test_static_route"

/*
 * Acceptance-test configurations.
 */

func testAccDDCloudStaticRouteBasic(name string, description string, ipVersion string,
	destinationNetworkAddress string, destinationPrefixSize int, nextHopAddress string) string {

	return fmt.Sprintf(`
		provider "ddcloud" {
			region		= "AU"	
		}
	    resource "ddcloud_networkdomain" "acc_test_domain" {
			name		= "acc-test-networkdomainsr"
			description	= "Network domain for Terraform acceptance test."
			datacenter	= "AU9"
			
		}
		resource "ddcloud_static_route" "acc_test_static_route" {
		    name = "%s"
		    description = "%s"
		    networkdomain = "${ddcloud_networkdomain.acc_test_domain.id}"
		    ip_version = "%s"
		    destination_network_address = "%s"
		    destination_prefix_size = %d
		    next_hop_address = "%s"
		}`,
		name, description, ipVersion, destinationNetworkAddress, destinationPrefixSize, nextHopAddress)
}

/*
 * Acceptance tests.
 */

// Acceptance test for ddcloud_static_route resource (basic):
//
// Create a static route and verify that it gets created with the correct configuration.
func TestAccStaticRouteBasicCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testCheckDDCloudStaticRouteDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccDDCloudStaticRouteBasic(
					// "3cecad11-0d41-4abf-a0c2-475563eef208",
					staticRouteName,
					"Static Route for Terraform acceptance test.",
					"IPV4",
					"100.103.0.0",
					16,
					"100.64.1.129",
				),
				Check: resource.ComposeTestCheckFunc(
					testCheckDDCloudStaticRouteExists(staticRouteName, true),
					testCheckDDCloudStaticRouteMatches(staticRouteName, compute.StaticRoute{
						Name:        staticRouteName,
						Description: "Static Route for Terraform acceptance test.",
						// NetworkDomainId:           "3cecad11-0d41-4abf-a0c2-475563eef208",
						IpVersion:                 "IPv4",
						DestinationNetworkAddress: "100.103.0.0",
						DestinationPrefixSize:     16,
						NextHopAddress:            "100.10.1.129",
					}),
				),
			},
		},
	})
}

// Acceptance test resource-destruction check for ddcloud_staticroute:
//
// Check all network domains specified in the configuration have been destroyed.
func testCheckDDCloudStaticRouteDestroy(state *terraform.State) error {
	for _, res := range state.RootModule().Resources {
		if res.Type != "ddcloud_static_route" {
			continue
		}

		staticRouteID := res.Primary.ID

		client := testAccProvider.Meta().(*providerState).Client()
		staticRoute, err := client.GetStaticRoute(staticRouteID)
		if err != nil {
			return nil
		}
		if staticRoute != nil {
			return fmt.Errorf("static route: '%s' still exists", staticRouteID)
		}
	}

	return nil
}

func testCheckDDCloudStaticRouteExists(name string, exists bool) resource.TestCheckFunc {
	name = ensureResourceTypePrefix(name, "ddcloud_static_route")

	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		staticRouteID := res.Primary.ID
		fmt.Sprintf("staticRouteID: %s", staticRouteID)

		client := testAccProvider.Meta().(*providerState).Client()

		staticRoute, err := client.GetStaticRoute(staticRouteID)
		if err != nil {
			return fmt.Errorf("bad: Get static route: %s", err)
		}
		if exists && staticRoute == nil {
			return fmt.Errorf("bad: Static Rute not found with Id '%s'", staticRouteID)
		} else if !exists && staticRoute != nil {
			return fmt.Errorf("bad: Static Route still exists with Id '%s'", staticRouteID)
		}

		return nil
	}
}

func testCheckDDCloudStaticRouteMatches(name string, expected compute.StaticRoute) resource.TestCheckFunc {
	name = ensureResourceTypePrefix(name, "ddcloud_static_route")
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		staticRouteID := res.Primary.ID

		client := testAccProvider.Meta().(*providerState).Client()
		staticRoute, err := client.GetStaticRoute(staticRouteID)
		if err != nil {
			return fmt.Errorf("bad: Get static route: %s", err)
		}
		if staticRoute == nil {
			return fmt.Errorf("bad: static route not found with ID '%s'", staticRouteID)
		}

		if staticRoute.Name != expected.Name {
			return fmt.Errorf("bad: static route '%s' has name '%s' (expected '%s')", staticRouteID, staticRoute.Name, expected.Name)
		}

		if staticRoute.Description != expected.Description {
			return fmt.Errorf("bad: static route '%s' has name '%s' (expected '%s')", staticRouteID, staticRoute.Description, expected.Description)
		}

		return nil
	}
}
