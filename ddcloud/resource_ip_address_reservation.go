package ddcloud

import (
	"fmt"
	"github.com/hhakkaev/dd-cloud-compute-terraform/retry"
	"log"

	"github.com/hhakkaev/dd-cloud-compute-terraform/validators"
	"github.com/hhakkaev/go-dd-cloud-compute/compute"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

const (
	resourceKeyIPAddressReservationVLANID      = "vlan"
	resourceKeyIPAddressReservationAddress     = "address"
	resourceKeyIPAddressReservationAddressType = "address_type"
	resourceKeyIPAddressReservationDescription = "description"
	addressTypeIPv4                            = "ipv4"
	addressTypeIPv6                            = "ipv6"
)

func resourceIPAddressReservation() *schema.Resource {
	return &schema.Resource{
		Create: resourceIPAddressReservationCreate,
		Read:   resourceIPAddressReservationRead,
		Delete: resourceIPAddressReservationDelete,

		Schema: map[string]*schema.Schema{
			resourceKeyIPAddressReservationVLANID: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The Id of the VLAN in which the IP address is reserved.",
			},
			resourceKeyIPAddressReservationAddress: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The reserved IP address.",
			},
			resourceKeyIPAddressReservationDescription: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The description reserved IP address.",
			},
			resourceKeyIPAddressReservationAddressType: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The reserved IP address type ('ipv4' or 'ipv6').",
				ValidateFunc: validators.StringIsOneOf("IP address type",
					addressTypeIPv4,
					addressTypeIPv6,
				),
			},
		},
	}
}

func resourceIPAddressReservationExists(data *schema.ResourceData, provider interface{}) (exists bool, err error) {
	vlanID := data.Get(resourceKeyIPAddressReservationVLANID).(string)
	address := data.Get(resourceKeyIPAddressReservationAddress).(string)
	addressType := data.Get(resourceKeyIPAddressReservationAddressType).(string)
	description, _ := data.GetOk(resourceKeyIPAddressReservationDescription)

	log.Printf("Check if Reserved IP address:'%s' description:'%s' type:'%s') exists in VLAN '%s'...",
		address,
		description,
		addressType,
		vlanID,
	)

	providerState := provider.(*providerState)

	var reservedIPAddresses map[string]compute.ReservedIPAddress
	reservedIPAddresses, err = getReservedIPAddresses(vlanID, addressType, providerState)
	if err != nil {
		return
	}

	_, exists = reservedIPAddresses[address]

	if exists {
		log.Printf("IP address '%s' description: %s type:'%s') is reserved in VLAN '%s'. ",
			address,
			description,
			addressType,
			vlanID,
		)
	} else {
		log.Printf("IP address '%s' description: %s type:'%s') is not reserved in VLAN '%s'. ",
			address,
			description,
			addressType,
			vlanID,
		)
	}

	return exists, nil
}

func resourceIPAddressReservationCreate(data *schema.ResourceData, provider interface{}) (err error) {
	vlanID := data.Get(resourceKeyIPAddressReservationVLANID).(string)
	address := data.Get(resourceKeyIPAddressReservationAddress).(string)
	addressType := data.Get(resourceKeyIPAddressReservationAddressType).(string)
	description := data.Get(resourceKeyIPAddressReservationDescription).(string)

	log.Printf("Reserving IP address '%s'. desciption: %s ('%s') in VLAN '%s'...",
		address,
		description,
		addressType,
		vlanID,
	)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	operationDescription := fmt.Sprintf("Reserve IP Address '%s'", description)
	err = providerState.RetryAction(operationDescription, func(context retry.Context) {
		// CloudControl has issues if more than one asynchronous operation is initated at a time (returns UNEXPECTED_ERROR).
		asyncLock := providerState.AcquireAsyncOperationLock("Reserve IP Address '%s'", description)
		defer asyncLock.Release()

		var deployError error
		switch addressType {

		case addressTypeIPv4:
			deployError = apiClient.ReservePrivateIPv4Address(vlanID, address, description)
		case addressTypeIPv6:
			deployError = apiClient.ReserveIPv6Address(vlanID, address, description)
		default:
			deployError = fmt.Errorf("invalid address type '%s'", addressType)
		}

		if deployError != nil {
			if compute.IsResourceBusyError(deployError) {
				context.Retry()
			} else {
				context.Fail(deployError)
			}
		}

		asyncLock.Release()
	})

	if err != nil {
		return err
	}

	data.SetId(fmt.Sprintf("%s/%s",
		address, description,
	))

	log.Printf("Reserved IP address:'%s' description:%s ('%s') in VLAN '%s' successfully.",
		address,
		description,
		addressType,
		vlanID,
	)

	return
}

func resourceIPAddressReservationRead(data *schema.ResourceData, provider interface{}) (err error) {

	log.Println("resourceIPAddressReservationRead")

	vlanID := data.Get(resourceKeyIPAddressReservationVLANID).(string)
	address := data.Get(resourceKeyIPAddressReservationAddress).(string)
	addressType := data.Get(resourceKeyIPAddressReservationAddressType).(string)
	description := data.Get(resourceKeyIPAddressReservationDescription).(string)

	log.Printf("Reading Reserved IP address:'%s' description:'%s' type:'%s') in VLAN '%s'...",
		address,
		description,
		addressType,
		vlanID,
	)

	providerState := provider.(*providerState)

	var reservedIPAddresses map[string]compute.ReservedIPAddress
	reservedIPAddresses, err = getReservedIPAddresses(vlanID, addressType, providerState)
	if err != nil {
		return
	}

	ipAddrReserved, exists := reservedIPAddresses[address]
	if exists {
		log.Printf("IP Address: %s is reserved.", ipAddrReserved.IPAddress)
		data.Set(resourceKeyIPAddressReservationAddress, ipAddrReserved.IPAddress)
		data.Set(resourceKeyIPAddressReservationVLANID, ipAddrReserved.VLANID)
		data.Set(resourceKeyIPAddressReservationDescription, ipAddrReserved.Description)
	} else {
		data.SetId("")
		log.Printf("IP Address: %s is NOT reserved.", ipAddrReserved.IPAddress)
	}

	return nil
}

func resourceIPAddressReservationDelete(data *schema.ResourceData, provider interface{}) (err error) {
	vlanID := data.Get(resourceKeyIPAddressReservationVLANID).(string)
	address := data.Get(resourceKeyIPAddressReservationAddress).(string)
	addressType := data.Get(resourceKeyIPAddressReservationAddressType).(string)
	description := data.Get(resourceKeyIPAddressReservationDescription).(string)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	operationDescription := fmt.Sprintf("Delete Reserved IP Address '%s'", description)
	err = providerState.RetryAction(operationDescription, func(context retry.Context) {
		// CloudControl has issues if more than one asynchronous operation is initated at a time (returns UNEXPECTED_ERROR).
		asyncLock := providerState.AcquireAsyncOperationLock("Delete Reserved IP Address '%s'", description)
		defer asyncLock.Release()

		switch addressType {
		case addressTypeIPv4:
			err = apiClient.UnreservePrivateIPv4Address(vlanID, address, description)
		case addressTypeIPv6:
			err = apiClient.UnreserveIPv6Address(vlanID, address, description)
		default:
			err = fmt.Errorf("invalid address type '%s'", addressType)
		}

		if err != nil {
			return
		}
		asyncLock.Release()
	})

	data.SetId("")

	return
}

func getReservedIPAddresses(vlanID string, addressType string, providerState *providerState) (map[string]compute.ReservedIPAddress, error) {
	switch addressType {
	case addressTypeIPv4:
		return getReservedPrivateIPv4Addresses(vlanID, providerState)
	case addressTypeIPv6:
		return getReservedIPv6Addresses(vlanID, providerState)
	default:
		return nil, fmt.Errorf("invalid address type '%s'", addressType)
	}
}

func getReservedPrivateIPv4Addresses(vlanID string, providerState *providerState) (map[string]compute.ReservedIPAddress, error) {
	apiClient := providerState.Client()

	reservedIPAddresses := make(map[string]compute.ReservedIPAddress)

	reservations, err := apiClient.ListReservedPrivateIPv4AddressesInVLAN(vlanID)
	if err != nil {
		return nil, err
	}
	if reservations.IsEmpty() {
		return nil, err
	}

	for _, reservedIPAddress := range reservations.Items {
		reservedIPAddresses[reservedIPAddress.IPAddress] = reservedIPAddress
	}

	return reservedIPAddresses, nil
}

func getReservedIPv6Addresses(vlanID string, providerState *providerState) (map[string]compute.ReservedIPAddress, error) {
	apiClient := providerState.Client()

	reservedIPAddresses := make(map[string]compute.ReservedIPAddress)

	reservations, err := apiClient.ListReservedIPv6AddressesInVLAN(vlanID)
	if err != nil {
		return nil, err
	}
	if reservations.IsEmpty() {
		return nil, err
	}

	for _, reservedIPAddress := range reservations.Items {
		reservedIPAddresses[reservedIPAddress.IPAddress] = reservedIPAddress
	}

	return reservedIPAddresses, nil
}
