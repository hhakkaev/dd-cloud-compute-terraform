package ddcloud

import (
	"fmt"
	"log"

	"github.com/hhakkaev/dd-cloud-compute-terraform/retry"
	"github.com/hhakkaev/go-dd-cloud-compute/compute"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

const (
	resourceKeyVirtualListenerName                   = "name"
	resourceKeyVirtualListenerDescription            = "description"
	resourceKeyVirtualListenerType                   = "type"
	resourceKeyVirtualListenerProtocol               = "protocol"
	resourceKeyVirtualListenerIPv4Address            = "ipv4"
	resourceKeyVirtualListenerPort                   = "port"
	resourceKeyVirtualListenerEnabled                = "enabled"
	resourceKeyVirtualListenerConnectionLimit        = "connection_limit"
	resourceKeyVirtualListenerConnectionRateLimit    = "connection_rate_limit"
	resourceKeyVirtualListenerSourcePortPreservation = "source_port_preservation"
	resourceKeyVirtualListenerPoolID                 = "pool"
	resourceKeyVirtualListenerPersistenceProfileName = "persistence_profile"
	resourceKeyVirtualListenerSSLOffloadProfileID    = "ssl_offload_profile"
	resourceKeyVirtualListenerIRuleNames             = "irules"
	resourceKeyVirtualListenerOptimizationProfile    = "optimization_profile"
	resourceKeyVirtualListenerNetworkDomainID        = "networkdomain"
)

func resourceVirtualListener() *schema.Resource {
	return &schema.Resource{
		Create: resourceVirtualListenerCreate,
		Read:   resourceVirtualListenerRead,
		Exists: resourceVirtualListenerExists,
		Update: resourceVirtualListenerUpdate,
		Delete: resourceVirtualListenerDelete,

		Schema: map[string]*schema.Schema{
			resourceKeyVirtualListenerName: &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			resourceKeyVirtualListenerDescription: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			resourceKeyVirtualListenerType: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  compute.VirtualListenerTypeStandard,
				ValidateFunc: func(data interface{}, fieldName string) (messages []string, errors []error) {
					listenerType := data.(string)
					switch listenerType {
					case compute.VirtualListenerTypeStandard:
					case compute.VirtualListenerTypePerformanceLayer4:
						return
					default:
						errors = append(errors, fmt.Errorf("invalid virtual listener type '%s'", listenerType))
					}

					return
				},
			},
			resourceKeyVirtualListenerProtocol: &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			resourceKeyVirtualListenerIPv4Address: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Default:  nil,
			},
			resourceKeyVirtualListenerPort: &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
				ForceNew: true,
			},
			resourceKeyVirtualListenerEnabled: &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			resourceKeyVirtualListenerConnectionLimit: &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  20000,
				ValidateFunc: func(data interface{}, fieldName string) (messages []string, errors []error) {
					connectionRate := data.(int)
					if connectionRate > 0 {
						return
					}

					errors = append(errors,
						fmt.Errorf("Connection rate ('%s') must be greater than 0", fieldName),
					)

					return
				},
			},
			resourceKeyVirtualListenerConnectionRateLimit: &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  2000,
				ValidateFunc: func(data interface{}, fieldName string) (messages []string, errors []error) {
					connectionRate := data.(int)
					if connectionRate > 0 {
						return
					}

					errors = append(errors,
						fmt.Errorf("Connection rate limit ('%s') must be greater than 0", fieldName),
					)

					return
				},
			},
			resourceKeyVirtualListenerSourcePortPreservation: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  compute.SourcePortPreservationEnabled,
			},
			resourceKeyVirtualListenerPoolID: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			resourceKeyVirtualListenerPersistenceProfileName: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			resourceKeyVirtualListenerSSLOffloadProfileID: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			resourceKeyVirtualListenerIRuleNames: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Default:  nil,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Set: func(item interface{}) int {
					iRuleID := item.(string)

					return schema.HashString(iRuleID)
				},
			},
			resourceKeyVirtualListenerOptimizationProfile: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			resourceKeyVirtualListenerNetworkDomainID: &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			// TODO: Add remaining properties.
		},
	}
}

func resourceVirtualListenerCreate(data *schema.ResourceData, provider interface{}) error {
	networkDomainID := data.Get(resourceKeyVirtualListenerNetworkDomainID).(string)
	name := data.Get(resourceKeyVirtualListenerName).(string)
	description := data.Get(resourceKeyVirtualListenerDescription).(string)

	log.Printf("Create virtual listener '%s' ('%s') in network domain '%s'.", name, description, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	propertyHelper := propertyHelper(data)

	var virtualListenerID string

	operationDescription := fmt.Sprintf("Create virtual listener '%s' ", name)
	operationError := providerState.RetryAction(operationDescription, func(context retry.Context) {
		// Map from names to Ids, as required.
		persistenceProfileID, err := propertyHelper.GetVirtualListenerPersistenceProfileID(apiClient)
		if err != nil {
			context.Fail(err)

			return
		}

		iRuleIDs, err := propertyHelper.GetVirtualListenerIRuleIDs(apiClient)
		if err != nil {
			context.Fail(err)

			return
		}

		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		virtualListenerID, err = apiClient.CreateVirtualListener(compute.NewVirtualListenerConfiguration{
			Name:                   name,
			Description:            description,
			Type:                   data.Get(resourceKeyVirtualListenerType).(string),
			Protocol:               data.Get(resourceKeyVirtualListenerProtocol).(string),
			Port:                   data.Get(resourceKeyVirtualListenerPort).(int),
			ListenerIPAddress:      propertyHelper.GetOptionalString(resourceKeyVirtualListenerIPv4Address, false),
			Enabled:                data.Get(resourceKeyVirtualListenerEnabled).(bool),
			ConnectionLimit:        data.Get(resourceKeyVirtualListenerConnectionLimit).(int),
			ConnectionRateLimit:    data.Get(resourceKeyVirtualListenerConnectionRateLimit).(int),
			SourcePortPreservation: data.Get(resourceKeyVirtualListenerSourcePortPreservation).(string),
			PoolID:                 propertyHelper.GetOptionalString(resourceKeyVirtualListenerPoolID, false),
			PersistenceProfileID:   persistenceProfileID,
			SSLOffloadProfileID:    propertyHelper.GetOptionalString(resourceKeyVirtualListenerSSLOffloadProfileID, false),
			IRuleIDs:               iRuleIDs,
			OptimizationProfile:    propertyHelper.GetOptionalString(resourceKeyVirtualListenerOptimizationProfile, false),
			NetworkDomainID:        networkDomainID,
		})
		if err != nil {
			if compute.IsResourceBusyError(err) {
				context.Retry()
			} else if compute.IsNoIPAddressAvailableError(err) {
				log.Printf("There are no free public IPv4 addresses in network domain '%s'; requesting allocation of a new address block...", networkDomainID)

				publicIPBlock, err := addPublicIPBlock(networkDomainID, apiClient)
				if err != nil {
					context.Fail(err)

					return
				}
				log.Printf("Allocated a new public IPv4 address block '%s' (%d addresses, starting at '%s').",
					publicIPBlock.ID, publicIPBlock.Size, publicIPBlock.BaseIP,
				)

				context.Retry() // We'll use the new block next time around.
			} else {
				context.Fail(err)
			}
		}
	})
	if operationError != nil {
		return operationError
	}

	data.SetId(virtualListenerID)

	log.Printf("Successfully created virtual listener '%s'.", virtualListenerID)

	virtualListener, err := apiClient.GetVirtualListener(virtualListenerID)
	if err != nil {
		return err
	}
	if virtualListener == nil {
		return fmt.Errorf("cannot find newly-created virtual listener with Id '%s'", virtualListenerID)
	}

	data.Set(resourceKeyVirtualListenerIPv4Address, virtualListener.ListenerIPAddress)

	return nil
}

func resourceVirtualListenerExists(data *schema.ResourceData, provider interface{}) (bool, error) {
	id := data.Id()

	log.Printf("Check if virtual listener '%s' exists...", id)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	vipPool, err := apiClient.GetVirtualListener(id)
	if err != nil {
		return false, err
	}

	exists := vipPool != nil

	log.Printf("virtual listener '%s' exists: %t.", id, exists)

	return exists, nil
}

func resourceVirtualListenerRead(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()

	log.Printf("Read virtual listener '%s'...", id)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	virtualListener, err := apiClient.GetVirtualListener(id)
	if err != nil {
		return err
	}
	if virtualListener == nil {
		data.SetId("") // Virtual listener has been deleted

		return nil
	}

	data.Set(resourceKeyVirtualListenerDescription, virtualListener.Description)
	data.Set(resourceKeyVirtualListenerEnabled, virtualListener.Enabled)
	data.Set(resourceKeyVirtualListenerConnectionLimit, virtualListener.ConnectionLimit)
	data.Set(resourceKeyVirtualListenerConnectionRateLimit, virtualListener.ConnectionRateLimit)
	data.Set(resourceKeyVirtualListenerSourcePortPreservation, virtualListener.SourcePortPreservation)
	data.Set(resourceKeyVirtualListenerPersistenceProfileName, virtualListener.PersistenceProfile.Name)
	data.Set(resourceKeyVirtualListenerSSLOffloadProfileID, virtualListener.SSLOffloadProfile.ID)
	data.Set(resourceKeyVirtualListenerIPv4Address, virtualListener.ListenerIPAddress)

	propertyHelper := propertyHelper(data)
	propertyHelper.SetVirtualListenerIRules(virtualListener.IRules)

	// TODO: Capture other properties.

	return nil
}

func resourceVirtualListenerUpdate(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	log.Printf("Update virtual listener '%s'...", id)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	configuration := &compute.EditVirtualListenerConfiguration{}

	propertyHelper := propertyHelper(data)

	// Effectively, these properties must always be supplied.
	configuration.Description = propertyHelper.GetOptionalString(resourceKeyVirtualListenerDescription, true)
	configuration.Enabled = propertyHelper.GetOptionalBool(resourceKeyVirtualListenerEnabled)

	if data.HasChange(resourceKeyVirtualListenerConnectionLimit) {
		configuration.ConnectionLimit = propertyHelper.GetOptionalInt(resourceKeyVirtualListenerConnectionLimit, false)
	}

	if data.HasChange(resourceKeyVirtualListenerConnectionRateLimit) {
		configuration.ConnectionRateLimit = propertyHelper.GetOptionalInt(resourceKeyVirtualListenerConnectionRateLimit, false)
	}

	if data.HasChange(resourceKeyVirtualListenerSourcePortPreservation) {
		configuration.SourcePortPreservation = propertyHelper.GetOptionalString(resourceKeyVirtualListenerSourcePortPreservation, true)
	}

	if data.HasChange(resourceKeyVirtualListenerPoolID) {
		configuration.PoolID = propertyHelper.GetOptionalString(resourceKeyVirtualListenerPoolID, true)
	}

	if data.HasChange(resourceKeyVirtualListenerSSLOffloadProfileID) {
		configuration.SSLOffloadProfileID = propertyHelper.GetOptionalString(resourceKeyVirtualListenerSSLOffloadProfileID, false)
	}

	if data.HasChange(resourceKeyVirtualListenerPersistenceProfileName) {
		persistenceProfile, err := propertyHelper.GetVirtualListenerPersistenceProfile(apiClient)
		if err != nil {
			return err
		}

		configuration.PersistenceProfileID = &persistenceProfile.ID
	}

	if data.HasChange(resourceKeyVirtualListenerIRuleNames) {
		iRuleIDs, err := propertyHelper.GetVirtualListenerIRuleIDs(apiClient)
		if err != nil {
			return err
		}

		configuration.IRuleIDs = &iRuleIDs
	}

	return apiClient.EditVirtualListener(id, *configuration)
}

func resourceVirtualListenerDelete(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	name := data.Get(resourceKeyVirtualListenerName).(string)
	networkDomainID := data.Get(resourceKeyVirtualListenerNetworkDomainID)

	log.Printf("Delete virtual listener '%s' ('%s') from network domain '%s'...", name, id, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	operationDescription := fmt.Sprintf("Delete virtual listener '%s", id)

	return providerState.RetryAction(operationDescription, func(context retry.Context) {
		// CloudControl has issues if more than one asynchronous operation is initated at a time (returns UNEXPECTED_ERROR).
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release() // Released at the end of the current attempt.

		err := apiClient.DeleteVirtualListener(id)
		if err != nil {
			if compute.IsResourceBusyError(err) {
				context.Retry()
			} else {
				context.Fail(err)
			}
		}

		asyncLock.Release()
	})
}
