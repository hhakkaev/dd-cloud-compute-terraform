package ddcloud

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hhakkaev/dd-cloud-compute-terraform/retry"
	"github.com/hhakkaev/go-dd-cloud-compute/compute"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

const (
	resourceKeyNATNetworkDomainID = "networkdomain"
	resourceKeyNATPrivateAddress  = "private_ipv4"
	resourceKeyNATPublicAddress   = "public_ipv4"
	resourceCreateTimeoutNAT      = 30 * time.Minute
	resourceUpdateTimeoutNAT      = 10 * time.Minute
	resourceDeleteTimeoutNAT      = 15 * time.Minute
)

func resourceNAT() *schema.Resource {
	return &schema.Resource{
		Exists: resourceNATExists,
		Create: resourceNATCreate,
		Read:   resourceNATRead,
		Update: resourceNATUpdate,
		Delete: resourceNATDelete,
		Importer: &schema.ResourceImporter{
			State: resourceNATImport,
		},

		Schema: map[string]*schema.Schema{
			resourceKeyNATNetworkDomainID: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The Id of the network domain that the NAT rule applies to.",
			},
			resourceKeyNATPrivateAddress: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The private (internal) IPv4 address.",
			},
			resourceKeyNATPublicAddress: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Default:     nil,
				Description: "The public (external) IPv4 address.",
			},
		},
	}
}

// Check if a NAT resource exists.
func resourceNATExists(data *schema.ResourceData, provider interface{}) (bool, error) {
	id := data.Id()
	log.Printf("Check if NAT rule '%s' exists.", id)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	natRule, err := apiClient.GetNATRule(id)
	if err != nil {
		return false, err
	}

	exists := natRule != nil

	log.Printf("NAT rule '%s' exists: %t.", id, exists)

	return exists, nil
}

// Create a NAT resource.
func resourceNATCreate(data *schema.ResourceData, provider interface{}) error {
	var err error

	propertyHelper := propertyHelper(data)

	networkDomainID := data.Get(resourceKeyNATNetworkDomainID).(string)
	privateIP := data.Get(resourceKeyNATPrivateAddress).(string)
	publicIP := propertyHelper.GetOptionalString(resourceKeyNATPublicAddress, false)

	publicIPDescription := "<computed>"
	if publicIP != nil {
		publicIPDescription = *publicIP
	}
	log.Printf("Create NAT rule (from public IP '%s' to private IP '%s') in network domain '%s'.", publicIPDescription, privateIP, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	var (
		natRuleID   string
		createError error
	)

	operationDescription := fmt.Sprintf("Create NAT rule (from public IP '%s' to private IP '%s')", publicIPDescription, privateIP)
	err = providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		natRuleID, createError = apiClient.AddNATRule(networkDomainID, privateIP, publicIP)
		if createError != nil {
			if compute.IsResourceBusyError(createError) {
				context.Retry()
			} else if compute.IsNoIPAddressAvailableError(createError) {
				log.Printf("There are no free public IPv4 addresses in network domain '%s'; requesting allocation of a new address block...", networkDomainID)

				publicIPBlock, addBlockError := addPublicIPBlock(networkDomainID, apiClient)
				if addBlockError != nil {
					context.Fail(addBlockError)

					return
				}
				log.Printf("Allocated a new public IPv4 address block '%s' (%d addresses, starting at '%s').",
					publicIPBlock.ID, publicIPBlock.Size, publicIPBlock.BaseIP,
				)

				context.Retry() // We'll use the new block next time around.
			} else {
				context.Fail(createError)
			}
		}
	})
	if err != nil {
		return err
	}

	data.SetId(natRuleID)
	log.Printf("Successfully created NAT rule '%s'.", natRuleID)

	natRule, err := apiClient.GetNATRule(natRuleID)
	if err != nil {
		return err
	}

	if natRule == nil {
		return fmt.Errorf("cannot find newly-added NAT rule '%s'", natRuleID)
	}

	data.Set(resourceKeyNATPublicAddress, natRule.ExternalIPAddress)
	data.Set(resourceKeyNATPrivateAddress, natRule.InternalIPAddress)

	return nil
}

// Read a NAT resource.
func resourceNATRead(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	networkDomainID := data.Get(resourceKeyNATNetworkDomainID).(string)
	privateIP := data.Get(resourceKeyNATPrivateAddress).(string)
	publicIP := data.Get(resourceKeyNATPublicAddress).(string)

	log.Printf("Read NAT '%s' (private IP = '%s', public IP = '%s') in network domain '%s'.", id, privateIP, publicIP, networkDomainID)

	apiClient := provider.(*providerState).Client()

	natRule, err := apiClient.GetNATRule(id)
	if err != nil {
		return err
	}
	if natRule == nil {
		data.SetId("") // NAT rule has been deleted

		return nil
	}

	return nil
}

// Update a NAT resource.
func resourceNATUpdate(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	networkDomainID := data.Get(resourceKeyNATNetworkDomainID).(string)
	privateIP := data.Get(resourceKeyNATPrivateAddress).(string)
	publicIP := data.Get(resourceKeyNATPublicAddress).(string)

	log.Printf("Update NAT '%s' (private IP = '%s', public IP = '%s') in network domain '%s' - nothing to update (NAT rules are read-only)", id, privateIP, publicIP, networkDomainID)

	return nil
}

// Delete a NAT resource.
func resourceNATDelete(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	networkDomainID := data.Get(resourceKeyNATNetworkDomainID).(string)
	privateIP := data.Get(resourceKeyNATPrivateAddress).(string)
	publicIP := data.Get(resourceKeyNATPublicAddress).(string)

	log.Printf("Delete NAT '%s' (private IP = '%s', public IP = '%s') in network domain '%s'.", id, privateIP, publicIP, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	operationDescription := fmt.Sprintf("Delete NAT '%s", id)

	return providerState.RetryAction(operationDescription, func(context retry.Context) {
		// CloudControl has issues if more than one asynchronous operation is initated at a time (returns UNEXPECTED_ERROR).
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release() // Released at the end of the current attempt.

		err := apiClient.DeleteNATRule(id)
		if err != nil {
			if compute.IsResourceBusyError(err) {
				context.Retry()
			} else {
				context.Fail(err)
			}
		}
	})
}

// Import data for an existing network domain.
func resourceNATImport(data *schema.ResourceData, provider interface{}) (importedData []*schema.ResourceData, err error) {
	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	natRuleID := data.Id()
	log.Printf("Import NAT rule '%s'.", natRuleID)

	var natRule *compute.NATRule
	natRule, err = apiClient.GetNATRule(natRuleID)
	if err != nil {
		return
	}
	if natRule == nil {
		err = fmt.Errorf("NAT rule '%s' not found", natRuleID)

		return
	}

	data.Set(resourceKeyNATNetworkDomainID, natRule.NetworkDomainID)
	data.Set(resourceKeyNATPrivateAddress, natRule.InternalIPAddress)
	data.Set(resourceKeyNATPublicAddress, natRule.ExternalIPAddress)

	importedData = []*schema.ResourceData{data}

	return
}

func calculateBlockAddresses(block compute.PublicIPBlock) ([]string, error) {
	addresses := make([]string, block.Size)

	baseAddressComponents := strings.Split(block.BaseIP, ".")
	if len(baseAddressComponents) != 4 {
		return addresses, fmt.Errorf("invalid base IP address '%s'", block.BaseIP)
	}
	baseOctet, err := strconv.Atoi(baseAddressComponents[3])
	if err != nil {
		return addresses, err
	}

	for index := range addresses {
		// Increment the last octet to determine the next address in the block.
		baseAddressComponents[3] = strconv.Itoa(baseOctet + index)
		addresses[index] = strings.Join(baseAddressComponents, ".")
	}

	return addresses, nil
}
