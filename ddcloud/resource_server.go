package ddcloud

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hhakkaev/dd-cloud-compute-terraform/models"
	"github.com/hhakkaev/dd-cloud-compute-terraform/retry"
	"github.com/hhakkaev/dd-cloud-compute-terraform/validators"
	"github.com/hhakkaev/go-dd-cloud-compute/compute"
)

const (
	resourceKeyServerName                     = "name"
	resourceKeyServerDescription              = "description"
	resourceKeyServerAdminPassword            = "admin_password"
	resourceKeyServerImage                    = "image"
	resourceKeyServerImageType                = "image_type"
	resourceKeyServerOSType                   = "os_type"
	resourceKeyServerOSFamily                 = "os_family"
	resourceKeyServerNetworkDomainID          = "networkdomain"
	resourceKeyServerMemoryGB                 = "memory_gb"
	resourceKeyServerCPUCount                 = "cpu_count"
	resourceKeyServerCPUCoreCount             = "cores_per_cpu"
	resourceKeyServerCPUSpeed                 = "cpu_speed"
	resourceKeyServerPrimaryAdapterVLAN       = "primary_adapter_vlan"
	resourceKeyServerPrimaryAdapterIPv4       = "primary_adapter_ipv4"
	resourceKeyServerPrimaryAdapterIPv6       = "primary_adapter_ipv6"
	resourceKeyServerPublicIPv4               = "public_ipv4"
	resourceKeyServerPrimaryDNS               = "dns_primary"
	resourceKeyServerSecondaryDNS             = "dns_secondary"
	resourceKeyServerPowerState               = "power_state"
	resourceKeyServerGuestOSCustomization     = "guest_os_customization"
	resourceKeyServerStarted                  = "started"
	resourceKeyServerBackupEnabled            = "backup_enabled"
	resourceKeyServerBackupClientDownloadURLs = "backup_client_urls"

	// Obsolete properties
	resourceKeyServerOSImageID          = "os_image_id"
	resourceKeyServerOSImageName        = "os_image_name"
	resourceKeyServerCustomerImageID    = "customer_image_id"
	resourceKeyServerCustomerImageName  = "customer_image_name"
	resourceKeyServerPrimaryAdapterType = "primary_adapter_type"
	resourceKeyServerAutoStart          = "auto_start"

	resourceCreateTimeoutServer = 30 * time.Minute
	resourceUpdateTimeoutServer = 10 * time.Minute
	resourceDeleteTimeoutServer = 15 * time.Minute
	serverShutdownTimeout       = 5 * time.Minute
)

func resourceServer() *schema.Resource {
	return &schema.Resource{
		SchemaVersion: 5,
		Create:        resourceServerCreate,
		Read:          resourceServerRead,
		Update:        resourceServerUpdate,
		Delete:        resourceServerDelete,
		Importer: &schema.ResourceImporter{
			State: resourceServerImport,
		},

		Schema: map[string]*schema.Schema{
			resourceKeyServerName: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "A name for the server",
			},
			resourceKeyServerDescription: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "A description for the server",
			},
			resourceKeyServerAdminPassword: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Default:     "",
				Description: "The initial administrative password (if applicable) for the deployed server",
			},
			resourceKeyServerMemoryGB: &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Default:     nil,
				Description: "The amount of memory (in GB) allocated to the server",
			},
			resourceKeyServerCPUCount: &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Default:     nil,
				Description: "The number of CPUs allocated to the server",
			},
			resourceKeyServerCPUCoreCount: &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Default:     nil,
				Description: "The number of cores per CPU allocated to the server",
			},
			resourceKeyServerCPUSpeed: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Default:     nil,
				Description: "The speed (quality-of-service) for CPUs allocated to the server",
			},
			resourceKeyServerImage: &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name or Id of the image from which the server is created",
			},
			resourceKeyServerImageType: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     serverImageTypeAuto,
				Description: "The type of image from which the server is created (default is auto-detect)",
				ValidateFunc: func(value interface{}, key string) (warnings []string, errors []error) {
					imageType := value.(string)

					switch imageType {
					case serverImageTypeOS:
					case serverImageTypeCustomer:
					case serverImageTypeAuto:
						return
					default:
						errors = append(errors,
							fmt.Errorf("invalid image type '%s'", imageType),
						)
					}

					return
				},
			},
			resourceKeyServerOSType: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The server operating system type",
			},
			resourceKeyServerOSFamily: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The server operating system family",
			},
			resourceKeyServerDisk: schemaDisk(),
			resourceKeyServerNetworkDomainID: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "The Id of the network domain in which the server is deployed",
			},
			resourceKeyServerPrimaryNetworkAdapter:    schemaServerPrimaryNetworkAdapter(),
			resourceKeyServerAdditionalNetworkAdapter: schemaServerAdditionalNetworkAdapter(),
			resourceKeyServerPrimaryAdapterVLAN: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Default:     nil,
				Description: "The Id of the VLAN to which the server's primary network adapter will be attached (the first available IPv4 address will be allocated)",
			},
			resourceKeyServerPrimaryAdapterIPv4: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Default:     nil,
				Description: "The IPv4 address for the server's primary network adapter",
			},
			resourceKeyServerPublicIPv4: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Default:     nil,
				Description: "The server's public IPv4 address (if any)",
			},
			resourceKeyServerPrimaryAdapterIPv6: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The IPv6 address of the server's primary network adapter",
			},
			resourceKeyServerPrimaryDNS: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Default:     "",
				Description: "The IP address of the server's primary DNS server",
			},
			resourceKeyServerSecondaryDNS: &schema.Schema{
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Default:     "",
				Description: "The IP address of the server's secondary DNS server",
			},
			resourceKeyServerPowerState: &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "disabled",
				Description:  "Start or shutdown a server. If set to start upon server creation it will Auto Start the server",
				ValidateFunc: validators.StringIsOneOf("Server Power State", "disabled", "autostart", "start", "shutdown", "shutdown-hard"),
			},

			resourceKeyServerStarted: &schema.Schema{
				Type:        schema.TypeBool,
				Description: "Is the server currently running",
				Computed:    true,
			},

			resourceKeyTag: schemaTag(),

			resourceKeyServerBackupEnabled: &schema.Schema{
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Is cloud backup enabled for the server",
			},
			resourceKeyServerBackupClientDownloadURLs: &schema.Schema{
				Type:        schema.TypeMap,
				Computed:    true,
				Description: "Download URLs for the server's backup clients (if any)",
			},

			// Obsolete properties
			resourceKeyServerPrimaryAdapterType: &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Default:     nil,
				Description: "The type of the server's primary network adapter (E1000 or VMXNET3)",
				Removed:     "This property has been removed because it is not exposed via the CloudControl API and will not be available until the provider uses the new (v2.4) API",
			},
			resourceKeyServerOSImageID: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     nil,
				Description: "The Id of the OS (built-in) image from which the server is created",
				Removed:     fmt.Sprintf("This property has been removed; use %s instead.", resourceKeyServerImage),
			},
			resourceKeyServerOSImageName: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     nil,
				Description: "The name of the OS (built-in) image from which the server is created",
				Removed:     fmt.Sprintf("This property has been removed; use %s instead.", resourceKeyServerImage),
			},
			resourceKeyServerCustomerImageID: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     nil,
				Description: "The Id of the customer (custom) image from which the server is created",
				Removed:     fmt.Sprintf("This property has been removed; use %s instead.", resourceKeyServerImage),
			},
			resourceKeyServerCustomerImageName: &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     nil,
				Description: "The name of the customer (custom) image from which the server is created",
				Removed:     fmt.Sprintf("This property has been removed; use %s instead.", resourceKeyServerImage),
			},
			resourceKeyServerAutoStart: &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Should the server be started automatically once it has been deployed",
				Removed:     fmt.Sprintf("This propery as been removed; set %s to autostart instead", resourceKeyServerPowerState),
			},
		},
		MigrateState: resourceServerMigrateState,
	}
}

// Create a server resource.
func resourceServerCreate(data *schema.ResourceData, provider interface{}) error {
	name := data.Get(resourceKeyServerName).(string)
	description := data.Get(resourceKeyServerDescription).(string)
	networkDomainID := data.Get(resourceKeyServerNetworkDomainID).(string)

	log.Printf("Create server '%s' in network domain '%s' (description = '%s').", name, networkDomainID, description)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	networkDomain, err := apiClient.GetNetworkDomain(networkDomainID)
	if err != nil {
		return err
	}

	if networkDomain == nil {
		return fmt.Errorf("no network domain was found with Id '%s'", networkDomainID)
	}

	dataCenterID := networkDomain.DatacenterID
	log.Printf("Server will be deployed in data centre '%s'.", dataCenterID)

	configuredImage := data.Get(resourceKeyServerImage).(string)
	configuredImageType := data.Get(resourceKeyServerImageType).(string)
	image, err := resolveServerImage(configuredImage, configuredImageType, dataCenterID, apiClient)
	if err != nil {
		return err
	}
	if image == nil {
		return fmt.Errorf("an unexpected error occurred while resolving the configured server image")
	}

	if image.RequiresCustomization() {
		return deployCustomizedServer(data, providerState, networkDomain, image)
	}

	return deployUncustomizedServer(data, providerState, networkDomain, image)
}

// Read a server resource.
func resourceServerRead(data *schema.ResourceData, provider interface{}) error {
	log.Printf("resource_server > resourceServerRead")
	propertyHelper := propertyHelper(data)

	id := data.Id()
	name := data.Get(resourceKeyServerName).(string)
	description := data.Get(resourceKeyServerDescription).(string)
	networkDomainID := data.Get(resourceKeyServerNetworkDomainID).(string)

	log.Printf("Read server '%s' (Id = '%s') in network domain '%s' (description = '%s').", name, id, networkDomainID, description)

	apiClient := provider.(*providerState).Client()
	server, err := apiClient.GetServer(id)
	if err != nil {
		return err
	}

	if server == nil {
		log.Printf("Server '%s' has been deleted.", id)

		// Mark as deleted.
		data.SetId("")

		return nil
	}
	data.Set(resourceKeyServerName, server.Name)
	data.Set(resourceKeyServerDescription, server.Description)
	data.Set(resourceKeyServerMemoryGB, server.MemoryGB)
	data.Set(resourceKeyServerCPUCount, server.CPU.Count)
	data.Set(resourceKeyServerCPUCoreCount, server.CPU.CoresPerSocket)
	data.Set(resourceKeyServerCPUSpeed, server.CPU.Speed)
	data.Set(resourceKeyServerOSType, server.OperatingSystem)

	powerState := propertyHelper.GetOptionalString(resourceKeyServerPowerState, false)

	isServerStarted := server.Started

	if powerState != nil {
		switch strings.ToLower(*powerState) {
		case "start":
			if !isServerStarted {
				data.Set(resourceKeyServerPowerState, "shutdown")
			}
		case "autostart":
			if !isServerStarted {
				data.Set(resourceKeyServerPowerState, "shutdown")
			}
		case "shutdown":
			if isServerStarted {
				data.Set(resourceKeyServerPowerState, "start")
			}
		case "shutdown-hard":
			if isServerStarted {
				data.Set(resourceKeyServerPowerState, "start")
			}

		default:
			data.Set(resourceKeyServerPowerState, "disabled")
		}
	}

	captureServerNetworkConfiguration(server, data, false)

	var publicIPv4Address string
	publicIPv4Address, err = findPublicIPv4Address(apiClient,
		networkDomainID,
		*server.Network.PrimaryAdapter.PrivateIPv4Address,
	)
	if err != nil {
		return err
	}
	if !isEmpty(publicIPv4Address) {
		data.Set(resourceKeyServerPublicIPv4, publicIPv4Address)
	} else {
		data.Set(resourceKeyServerPublicIPv4, nil)
	}

	err = readTags(data, apiClient, compute.AssetTypeServer)
	if err != nil {
		return err
	}

	propertyHelper.SetDisks(
		models.NewDisksFromVirtualMachineSCSIControllers(server.SCSIControllers),
	)

	networkAdapters := propertyHelper.GetServerNetworkAdapters()
	propertyHelper.SetServerNetworkAdapters(networkAdapters, false)

	return readServerBackupClientDownloadURLs(server.ID, data, apiClient)
}

// Update a server resource.
func resourceServerUpdate(data *schema.ResourceData, provider interface{}) error {
	serverID := data.Id()

	log.Printf("Update server '%s'.", serverID)

	providerState := provider.(*providerState)

	apiClient := providerState.Client()
	server, err := apiClient.GetServer(serverID)
	if err != nil {
		return err
	}

	if server == nil {
		log.Printf("Server '%s' has been deleted.", serverID)
		data.SetId("")

		return nil
	}

	data.Partial(true)

	propertyHelper := propertyHelper(data)

	var name, description *string
	if data.HasChange(resourceKeyServerName) {
		name = propertyHelper.GetOptionalString(resourceKeyServerName, true)
	}

	if data.HasChange(resourceKeyServerDescription) {
		description = propertyHelper.GetOptionalString(resourceKeyServerDescription, true)
	}

	if name != nil || description != nil {
		log.Printf("Server name / description change detected.")

		err = apiClient.EditServerMetadata(serverID, name, description)
		if err != nil {
			return err
		}

		if name != nil {
			data.SetPartial(resourceKeyServerName)
		}
		if description != nil {
			data.SetPartial(resourceKeyServerDescription)
		}
	}

	var memoryGB, cpuCount, cpuCoreCount *int
	var cpuSpeed *string
	if data.HasChange(resourceKeyServerMemoryGB) {
		memoryGB = propertyHelper.GetOptionalInt(resourceKeyServerMemoryGB, false)
	}
	if data.HasChange(resourceKeyServerCPUCount) {
		cpuCount = propertyHelper.GetOptionalInt(resourceKeyServerCPUCount, false)
	}
	if data.HasChange(resourceKeyServerCPUCoreCount) {
		cpuCoreCount = propertyHelper.GetOptionalInt(resourceKeyServerCPUCoreCount, false)
	}
	if data.HasChange(resourceKeyServerCPUSpeed) {
		cpuSpeed = propertyHelper.GetOptionalString(resourceKeyServerCPUSpeed, false)
	}

	if memoryGB != nil || cpuCount != nil || cpuCoreCount != nil || cpuSpeed != nil {
		log.Printf("Server CPU / memory configuration change detected.")

		err = updateServerConfiguration(apiClient, server, memoryGB, cpuCount, cpuCoreCount, cpuSpeed)
		if err != nil {
			return err
		}

		if data.HasChange(resourceKeyServerMemoryGB) {
			data.SetPartial(resourceKeyServerMemoryGB)
		}

		if data.HasChange(resourceKeyServerCPUCount) {
			data.SetPartial(resourceKeyServerCPUCount)
		}
	}

	// Primary adapter
	if data.HasChange(resourceKeyServerPrimaryNetworkAdapter) {
		log.Printf("[DD] resource_server resourceKeyServerPrimaryNetworkAdapter has changed ")
		actualPrimaryNetworkAdapter := models.NewNetworkAdapterFromVirtualMachineNetworkAdapter(server.Network.PrimaryAdapter)

		configuredPrimaryNetworkAdapter := propertyHelper.GetServerNetworkAdapters().GetPrimary()

		log.Printf("Configured primary network adapter = %#v", configuredPrimaryNetworkAdapter)
		log.Printf("Actual primary network adapter     = %#v", actualPrimaryNetworkAdapter)

		if (configuredPrimaryNetworkAdapter.PrivateIPv4Address != actualPrimaryNetworkAdapter.PrivateIPv4Address) ||
			(configuredPrimaryNetworkAdapter.PrivateIPv6Address != actualPrimaryNetworkAdapter.PrivateIPv6Address) {
			err = modifyServerNetworkAdapterIP(providerState, serverID, *configuredPrimaryNetworkAdapter)

			if err != nil {
				return err
			}
		}

		if configuredPrimaryNetworkAdapter.AdapterType != actualPrimaryNetworkAdapter.AdapterType {
			err = modifyServerNetworkAdapterType(providerState, serverID, *configuredPrimaryNetworkAdapter)
			if err != nil {
				return err
			}
		}

		// Capture updated public IPv4 address (if any).
		var publicIPv4Address string
		publicIPv4Address, err = findPublicIPv4Address(apiClient,
			server.Network.NetworkDomainID,
			*server.Network.PrimaryAdapter.PrivateIPv4Address,
		)
		if err != nil {
			return err
		}
		if !isEmpty(publicIPv4Address) {
			data.Set(resourceKeyServerPublicIPv4, publicIPv4Address)
		} else {
			data.Set(resourceKeyServerPublicIPv4, nil)
		}

		// Persist final state.
		server, err = apiClient.GetServer(serverID)
		if err != nil {
			return err
		}
		if server == nil {
			return fmt.Errorf("cannot find server with Id '%s'", serverID)
		}
	}

	// TODO: Find a better solution to be able to update, add and remove additional adapters.
	// Additional Network Adapters
	if data.HasChange(resourceKeyServerAdditionalNetworkAdapter) {
		log.Printf("[Resource_server] resourceKeyServerAdditionalNetworkAdapter has changed ")

		// Note: Tried modifying network adapter; it won't work in adding and removing scenarios, the index of NIC confuse terraform additional network adapter schema list.
		// Current implementation treat additional nic(s) as an attribute in server resource instead of a network adapter resource by itself.
		// At the moment, we had to disable the feature to attach and detach network adapter post-server provisioning and restrict update to modifying existing list of network adapter at server provision.

		// Refresh additional network adapters by removing and re-adding.
		log.Printf("[DD] Updating network adapters")

		// Update all additional network adapters
		configuredAdditionalAdapters := propertyHelper.GetServerNetworkAdapters().GetAdditional()
		for _, configured := range configuredAdditionalAdapters {

			err = modifyServerNetworkAdapterIP(providerState, serverID, configured)
			if err != nil {
				return err
			}
		}

		// Persist final state.
		server, err = apiClient.GetServer(serverID)
		if err != nil {
			return err
		}
		if server == nil {
			return fmt.Errorf("cannot find server with Id '%s'", serverID)
		}

	}

	if data.HasChange(resourceKeyTag) {
		err = applyTags(data, apiClient, compute.AssetTypeServer, providerState.Settings())
		if err != nil {
			return err
		}

		data.SetPartial(resourceKeyTag)
	}

	if data.HasChange(resourceKeyServerDisk) {
		err = updateDisks(data, providerState)
		if err != nil {
			return err
		}
	}

	if data.HasChange(resourceKeyServerPowerState) {
		log.Printf("Server power state change has been detected.")
		powerState := propertyHelper.GetOptionalString(resourceKeyServerPowerState, false)

		isServerStartedActual := server.Started

		if powerState != nil {
			switch strings.ToLower(*powerState) {
			case "start":
				if !isServerStartedActual {
					err = serverStart(providerState, serverID)
				}
			case "shutdown":
				if isServerStartedActual {
					err = serverShutdown(providerState, serverID)
				}
			case "shutdown-hard":
				err = serverPowerOff(providerState, serverID)
			case "disabled", "autostart":
				// do nothing
				break
			default:
				err = fmt.Errorf("Invalid power State (%s); Valid Power states are start, shutdown, shutdown-hard, disabled", *powerState)
			}

			if err != nil {
				return err
			}

			data.SetPartial(resourceKeyServerPowerState)
		}

	}
	// Refresh Server State after Power State
	server, err = apiClient.GetServer(serverID)
	if err != nil {
		return err
	}

	if server.Started {
		data.Set(resourceKeyServerStarted, true)
	} else {
		data.Set(resourceKeyServerStarted, false)
	}
	data.Partial(false)

	return nil
}

// Delete a server resource.
func resourceServerDelete(data *schema.ResourceData, provider interface{}) error {
	id := data.Id()
	name := data.Get(resourceKeyServerName).(string)
	networkDomainID := data.Get(resourceKeyServerNetworkDomainID).(string)

	log.Printf("Delete server '%s' ('%s') in network domain '%s'.", id, name, networkDomainID)

	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	server, err := apiClient.GetServer(id)
	if err != nil {
		return err
	}
	if server == nil {
		log.Printf("Server '%s' not found; will treat the server as having already been deleted.", id)

		return nil
	}

	if server.Started {
		log.Printf("Server '%s' is currently running. The server will be powered off.", id)
		err = serverPowerOff(providerState, id)
		if err != nil {
			return err
		}
	}

	operationDescription := fmt.Sprintf("Delete server '%s'", id)
	err = providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		deleteError := apiClient.DeleteServer(id)
		if compute.IsResourceBusyError(deleteError) {
			context.Retry()
		} else if deleteError != nil {
			context.Fail(deleteError)
		}
	})
	if err != nil {
		return err
	}

	log.Printf("Server '%s' is being deleted...", id)

	return apiClient.WaitForDelete(compute.ResourceTypeServer, id, resourceDeleteTimeoutServer)
}

// Import data for an existing server.
func resourceServerImport(data *schema.ResourceData, provider interface{}) (importedData []*schema.ResourceData, err error) {
	providerState := provider.(*providerState)
	apiClient := providerState.Client()

	serverID := data.Id()
	log.Printf("Import server '%s'.", serverID)

	server, err := apiClient.GetServer(serverID)
	if err != nil {
		return
	}
	if server == nil {
		err = fmt.Errorf("Server '%s' not found", serverID)

		return
	}

	data.Set(resourceKeyServerName, server.Name)
	data.Set(resourceKeyServerDescription, server.Description)
	data.Set(resourceKeyServerMemoryGB, server.MemoryGB)
	data.Set(resourceKeyServerCPUCount, server.CPU.Count)
	data.Set(resourceKeyServerCPUCoreCount, server.CPU.CoresPerSocket)
	data.Set(resourceKeyServerCPUSpeed, server.CPU.Speed)
	data.Set(resourceKeyServerOSType, server.OperatingSystem)

	captureServerNetworkConfiguration(server, data, false)

	var publicIPv4Address string
	publicIPv4Address, err = findPublicIPv4Address(apiClient,
		server.Network.NetworkDomainID,
		*server.Network.PrimaryAdapter.PrivateIPv4Address,
	)
	if err != nil {
		return nil, err
	}
	if !isEmpty(publicIPv4Address) {
		data.Set(resourceKeyServerPublicIPv4, publicIPv4Address)
	} else {
		data.Set(resourceKeyServerPublicIPv4, nil)
	}

	err = readTags(data, apiClient, compute.AssetTypeServer)
	if err != nil {
		return nil, err
	}

	propertyHelper(data).SetDisks(
		models.NewDisksFromVirtualMachineSCSIControllers(server.SCSIControllers),
	)

	if server.Started {
		data.Set(resourceKeyServerStarted, true)
	} else {
		data.Set(resourceKeyServerStarted, false)
	}

	readServerBackupClientDownloadURLs(server.ID, data, apiClient)

	importedData = []*schema.ResourceData{data}

	return
}

// TODO: Refactor deployCustomizedServer / deployUncustomizedServer and move common logic to shared functions.

// Deploy a server with guest OS customisation.
func deployCustomizedServer(data *schema.ResourceData, providerState *providerState, networkDomain *compute.NetworkDomain, image compute.Image) error {
	name := data.Get(resourceKeyServerName).(string)
	description := data.Get(resourceKeyServerDescription).(string)
	adminPassword := data.Get(resourceKeyServerAdminPassword).(string)
	primaryDNS := data.Get(resourceKeyServerPrimaryDNS).(string)
	secondaryDNS := data.Get(resourceKeyServerSecondaryDNS).(string)
	powerState := data.Get(resourceKeyServerPowerState).(string)

	dataCenterID := networkDomain.DatacenterID
	log.Printf("Server will be deployed in data centre '%s' with guest OS customisation.", dataCenterID)

	propertyHelper := propertyHelper(data)

	log.Printf("Server will be deployed from %s image named '%s' (Id = '%s').",
		compute.ImageTypeName(image.GetType()),
		image.GetName(),
		image.GetID(),
	)

	powerOn := false
	if strings.ToLower(powerState) == "start" || strings.ToLower(powerState) == "autostart" {
		powerOn = true
	}

	deploymentConfiguration := compute.ServerDeploymentConfiguration{
		Name:                  name,
		Description:           description,
		AdministratorPassword: adminPassword,
		Start:                 powerOn,
	}

	err := validateAdminPassword(deploymentConfiguration.AdministratorPassword, image)

	if err != nil {
		return err
	}
	image.ApplyTo(&deploymentConfiguration)

	operatingSystem := image.GetOS()
	data.Set(resourceKeyServerOSType, operatingSystem.DisplayName)
	data.SetPartial(resourceKeyServerOSType)
	data.Set(resourceKeyServerOSFamily, operatingSystem.Family)
	data.SetPartial(resourceKeyServerOSFamily)

	// Validate disk configuration.
	configuredDisks := propertyHelper.GetDisks()
	err = validateServerDisks(configuredDisks)
	if err != nil {
		return err
	}

	// Image disk speeds
	configuredDisksBySCSIPath := configuredDisks.BySCSIPath()
	for controllerIndex := range deploymentConfiguration.SCSIControllers {
		deploymentSCSIController := &deploymentConfiguration.SCSIControllers[controllerIndex]
		for diskIndex := range deploymentSCSIController.Disks {
			deploymentDisk := &deploymentSCSIController.Disks[diskIndex]

			configuredDisk, ok := configuredDisksBySCSIPath[models.SCSIPath(deploymentSCSIController.BusNumber, deploymentDisk.SCSIUnitID)]
			if ok {
				deploymentDisk.Speed = configuredDisk.Speed
			}
		}
	}

	// Memory and CPU
	memoryGB := propertyHelper.GetOptionalInt(resourceKeyServerMemoryGB, false)
	if memoryGB != nil {
		deploymentConfiguration.MemoryGB = *memoryGB
	} else {
		data.Set(resourceKeyServerMemoryGB, deploymentConfiguration.MemoryGB)
	}

	cpuCount := propertyHelper.GetOptionalInt(resourceKeyServerCPUCount, false)
	if cpuCount != nil {
		deploymentConfiguration.CPU.Count = *cpuCount
	} else {
		data.Set(resourceKeyServerCPUCount, deploymentConfiguration.CPU.Count)
	}

	cpuCoreCount := propertyHelper.GetOptionalInt(resourceKeyServerCPUCoreCount, false)
	if cpuCoreCount != nil {
		deploymentConfiguration.CPU.CoresPerSocket = *cpuCoreCount
	} else {
		data.Set(resourceKeyServerCPUCoreCount, deploymentConfiguration.CPU.CoresPerSocket)
	}

	cpuSpeed := propertyHelper.GetOptionalString(resourceKeyServerCPUSpeed, false)
	if cpuSpeed != nil {
		deploymentConfiguration.CPU.Speed = *cpuSpeed
	} else {
		data.Set(resourceKeyServerCPUSpeed, deploymentConfiguration.CPU.Speed)
	}

	// Network
	deploymentConfiguration.Network = compute.VirtualMachineNetwork{
		NetworkDomainID: networkDomain.ID,
	}

	// Initial configuration for network adapters.
	networkAdapters := propertyHelper.GetServerNetworkAdapters()
	networkAdapters.UpdateVirtualMachineNetwork(&deploymentConfiguration.Network)

	deploymentConfiguration.PrimaryDNS = primaryDNS
	deploymentConfiguration.SecondaryDNS = secondaryDNS

	log.Printf("Server deployment configuration: %+v", deploymentConfiguration)
	log.Printf("Server CPU deployment configuration: %+v", deploymentConfiguration.CPU)

	apiClient := providerState.Client()

	var serverID string
	operationDescription := fmt.Sprintf("Deploy customised server '%s'", name)
	err = providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		var deployError error
		serverID, deployError = apiClient.DeployServer(deploymentConfiguration)
		if compute.IsResourceBusyError(deployError) {
			context.Retry()
		} else if deployError != nil {
			context.Fail(deployError)
		}
	})
	if err != nil {
		return err
	}
	data.SetId(serverID)

	log.Printf("Server '%s' is being provisioned...", name)
	resource, err := apiClient.WaitForDeploy(compute.ResourceTypeServer, serverID, resourceCreateTimeoutServer)
	if err != nil {
		return err
	}

	server := resource.(*compute.Server)

	// Capture additional properties (those only available after deployment) and modify auto-assinged IPs to the one specified in tf file.
	err = captureCreatedServerProperties(data, providerState, server, networkAdapters)
	if err != nil {
		return err
	}

	data.Partial(true)

	err = applyTags(data, apiClient, compute.AssetTypeServer, providerState.Settings())
	if err != nil {
		return err
	}
	data.SetPartial(resourceKeyTag)

	err = createDisks(server, data, providerState)
	if err != nil {
		return err
	}

	data.Partial(false)

	return nil
}

// Deploy a server without guest OS customisation.
func deployUncustomizedServer(data *schema.ResourceData, providerState *providerState, networkDomain *compute.NetworkDomain, image compute.Image) error {
	name := data.Get(resourceKeyServerName).(string)
	description := data.Get(resourceKeyServerDescription).(string)
	powerState := data.Get(resourceKeyServerPowerState).(string)

	dataCenterID := networkDomain.DatacenterID
	log.Printf("Server will be deployed in data centre '%s' with guest OS customisation.", dataCenterID)

	apiClient := providerState.Client()

	log.Printf("Server will be deployed from %s image named '%s' (Id = '%s').",
		compute.ImageTypeName(image.GetType()),
		image.GetName(),
		image.GetID(),
	)
	powerOn := false
	if strings.ToLower(powerState) == "start" || strings.ToLower(powerState) == "autostart" {
		powerOn = true
	}

	deploymentConfiguration := compute.UncustomizedServerDeploymentConfiguration{
		Name:        name,
		Description: description,
		Start:       powerOn,
	}
	image.ApplyToUncustomized(&deploymentConfiguration)

	operatingSystem := image.GetOS()
	data.Set(resourceKeyServerOSType, operatingSystem.DisplayName)
	data.SetPartial(resourceKeyServerOSType)
	data.Set(resourceKeyServerOSFamily, operatingSystem.Family)
	data.SetPartial(resourceKeyServerOSFamily)

	// Validate disk configuration.
	propertyHelper := propertyHelper(data)
	configuredDisks := propertyHelper.GetDisks()
	err := validateServerDisks(configuredDisks)
	if err != nil {
		return err
	}

	// Image disk speeds (for uncustomised servers, only a single SCSI controller is supported for initial deployment).
	configuredDisksBySCSIPath := configuredDisks.BySCSIPath()
	for diskIndex := range deploymentConfiguration.Disks {
		deploymentDisk := &deploymentConfiguration.Disks[diskIndex]

		configuredDisk, ok := configuredDisksBySCSIPath[models.SCSIPath(0, deploymentDisk.SCSIUnitID)]
		if ok {
			deploymentDisk.Speed = configuredDisk.Speed
		}
	}

	// Memory and CPU
	memoryGB := propertyHelper.GetOptionalInt(resourceKeyServerMemoryGB, false)
	if memoryGB != nil {
		deploymentConfiguration.MemoryGB = *memoryGB
	} else {
		data.Set(resourceKeyServerMemoryGB, deploymentConfiguration.MemoryGB)
	}

	cpuCount := propertyHelper.GetOptionalInt(resourceKeyServerCPUCount, false)
	if cpuCount != nil {
		deploymentConfiguration.CPU.Count = *cpuCount
	} else {
		data.Set(resourceKeyServerCPUCount, deploymentConfiguration.CPU.Count)
	}

	cpuCoreCount := propertyHelper.GetOptionalInt(resourceKeyServerCPUCoreCount, false)
	if cpuCoreCount != nil {
		deploymentConfiguration.CPU.CoresPerSocket = *cpuCoreCount
	} else {
		data.Set(resourceKeyServerCPUCoreCount, deploymentConfiguration.CPU.CoresPerSocket)
	}

	cpuSpeed := propertyHelper.GetOptionalString(resourceKeyServerCPUSpeed, false)
	if cpuSpeed != nil {
		deploymentConfiguration.CPU.Speed = *cpuSpeed
	} else {
		data.Set(resourceKeyServerCPUSpeed, deploymentConfiguration.CPU.Speed)
	}

	// Network
	deploymentConfiguration.Network = compute.VirtualMachineNetwork{
		NetworkDomainID: networkDomain.ID,
	}

	// Initial configuration for network adapters.
	networkAdapters := propertyHelper.GetServerNetworkAdapters()
	networkAdapters.UpdateVirtualMachineNetwork(&deploymentConfiguration.Network)

	log.Printf("Server deployment configuration: %+v", deploymentConfiguration)
	log.Printf("Server CPU deployment configuration: %+v", deploymentConfiguration.CPU)

	var serverID string
	operationDescription := fmt.Sprintf("Deploy uncustomised server '%s'", name)
	err = providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		var deployError error
		serverID, deployError = apiClient.DeployUncustomizedServer(deploymentConfiguration)
		if compute.IsResourceBusyError(deployError) {
			context.Retry()
		} else if deployError != nil {
			context.Fail(deployError)
		}
	})
	if err != nil {
		return err
	}
	data.SetId(serverID)

	log.Printf("Server '%s' is being provisioned...", name)
	resource, err := apiClient.WaitForDeploy(compute.ResourceTypeServer, serverID, resourceCreateTimeoutServer)
	if err != nil {
		return err
	}

	server := resource.(*compute.Server)

	// Capture additional properties that may only be available after deployment.
	err = captureCreatedServerProperties(data, providerState, server, networkAdapters)
	if err != nil {
		return err
	}

	data.Partial(true)

	err = applyTags(data, apiClient, compute.AssetTypeServer, providerState.Settings())
	if err != nil {
		return err
	}
	data.SetPartial(resourceKeyTag)

	err = createDisks(server, data, providerState)
	if err != nil {
		return err
	}

	data.Partial(false)

	return nil
}

// Capture additional properties that may only be available after deployment.
func captureCreatedServerProperties(data *schema.ResourceData, providerState *providerState, server *compute.Server, networkAdapters models.NetworkAdapters) error {
	networkDomainID := data.Get(resourceKeyServerNetworkDomainID).(string)

	apiClient := providerState.Client()

	propertyHelper := propertyHelper(data)

	networkAdapters.CaptureIDs(server.Network)
	propertyHelper.SetServerNetworkAdapters(networkAdapters, true)
	captureServerNetworkConfiguration(server, data, true)

	// Public IPv4
	publicIPv4Address, err := findPublicIPv4Address(apiClient,
		networkDomainID,
		*server.Network.PrimaryAdapter.PrivateIPv4Address,
	)
	if err != nil {
		return err
	}
	if !isEmpty(publicIPv4Address) {
		data.Set(resourceKeyServerPublicIPv4, publicIPv4Address)
	} else {
		data.Set(resourceKeyServerPublicIPv4, nil)
	}
	data.SetPartial(resourceKeyServerPublicIPv4)

	// Started
	data.Set(resourceKeyServerStarted, server.Started)
	data.SetPartial(resourceKeyServerStarted)

	// Update network adapter IPs from Auto-Assigned to user-defined
	for _, nic := range networkAdapters {
		log.Printf("[DD] resource_server > captureCreatedServerProperties() nic id:%s ipv4:%s ipv6:%s",
			nic.ID, nic.PrivateIPv4Address, nic.PrivateIPv6Address)
		err = modifyServerNetworkAdapterIP(providerState, server.ID, nic)
		if err != nil {
			return err
		}
	}

	return nil
}

func findPublicIPv4Address(apiClient *compute.Client, networkDomainID string, privateIPv4Address string) (publicIPv4Address string, err error) {
	page := compute.DefaultPaging()
	for {
		var natRules *compute.NATRules
		natRules, err = apiClient.ListNATRules(networkDomainID, page)
		if err != nil {
			return
		}
		if natRules.IsEmpty() {
			break // We're done
		}

		for _, natRule := range natRules.Rules {
			if natRule.InternalIPAddress == privateIPv4Address {
				return natRule.ExternalIPAddress, nil
			}
		}

		page.Next()
	}

	return
}

func validateAdminPassword(adminPassword string, image compute.Image) error {
	validPassword := true
	lettersCount, numCount, specialCount, upperCaseCount := 0, 0, 0, 0

	for _, s := range adminPassword {
		switch {
		case unicode.IsDigit(s):
			numCount++
		case unicode.IsUpper(s):
			lettersCount++
			upperCaseCount++
		case unicode.IsPunct(s) || unicode.IsSymbol(s):
			specialCount++
		case unicode.IsLetter(s) || s == ' ':
			lettersCount++
		default:
			//
		}
	}
	if (len(adminPassword) < 8) || (numCount < 1) || (lettersCount < 1) || (specialCount < 1) || (upperCaseCount < 1) {
		validPassword = false
	}

	switch image.GetType() {

	case compute.ImageTypeOS:
		// Admin password is always mandatory for OS images.
		if !validPassword {
			log.Printf("Password validation failed: Length=%d, LetterCount=%d, DigitCount=%d, Special=%d, UpperCount=%d",
				len(adminPassword), lettersCount, numCount, specialCount, upperCaseCount,
			)

			return fmt.Errorf("A password is mandatory for OS images. Either you have not supplied a password, or the password does not meet complexity requirements. Needs at least 9 characters, 1 upper, 1 lower, 1 number and a special char")
		}

	case compute.ImageTypeCustomer:
		imageOS := image.GetOS()

		// Admin password cannot be supplied for Linux customer images.
		if imageOS.Family == "UNIX" && adminPassword != "" {
			return fmt.Errorf("cannot specify an initial admin password when deploying a Linux OS image")
		}

		// Admin password is only mandatory for some types of Windows images
		if imageOS.Family != "WINDOWS" {
			return nil
		}

		// Mandatory for Windows Server 2008.
		if strings.HasPrefix(imageOS.ID, "WIN2008") && !validPassword {

			return fmt.Errorf("Must specify an initial admin password when deploying a customer image for Windows Server 2008")
		}

		// Mandatory for Windows Server 2012 R2.
		if strings.HasPrefix(imageOS.ID, "WIN2012R2") && !validPassword {
			return fmt.Errorf("Must specify an initial admin password when deploying a customer image for Windows Server 2012 R2")
		}

		// Mandatory for Windows Server 2012.
		if strings.HasPrefix(imageOS.ID, "WIN2012") && !validPassword {
			return fmt.Errorf("Must specify an initial admin password when deploying a customer image for Windows Server 2012")
		}

	default:
		return fmt.Errorf("Unknown image type (%d)", image.GetType())
	}

	return nil
}

// Start a server.
//
// Respects providerSettings.AllowServerReboots.
func serverStart(providerState *providerState, serverID string) error {
	providerSettings := providerState.Settings()
	apiClient := providerState.Client()

	if !providerSettings.AllowServerReboots {
		return fmt.Errorf("cannot start server '%s' because server reboots have not been enabled via the 'allow_server_reboot' provider setting or 'DDCLOUD_ALLOW_SERVER_REBOOT' environment variable", serverID)
	}

	operationDescription := fmt.Sprintf("Start server '%s'", serverID)
	err := providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		startError := apiClient.StartServer(serverID)
		if compute.IsResourceBusyError(startError) {
			context.Retry()
		} else if startError != nil {
			context.Fail(startError)
		}

		asyncLock.Release()
	})
	if err != nil {
		return err
	}

	_, err = apiClient.WaitForChange(compute.ResourceTypeServer, serverID, "Start server", serverShutdownTimeout)
	if err != nil {
		return err
	}

	return nil
}

// Gracefully stop a server.
//
// Respects providerSettings.AllowServerReboots.
func serverShutdown(providerState *providerState, serverID string) error {
	providerSettings := providerState.Settings()
	apiClient := providerState.Client()

	if !providerSettings.AllowServerReboots {
		return fmt.Errorf("cannot shut down server '%s' because server reboots have not been enabled via the 'allow_server_reboot' provider setting or 'DDCLOUD_ALLOW_SERVER_REBOOT' environment variable", serverID)
	}

	operationDescription := fmt.Sprintf("Shut down server '%s'", serverID)
	err := providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		shutdownError := apiClient.ShutdownServer(serverID)
		if compute.IsResourceBusyError(shutdownError) {
			context.Retry()
		} else if shutdownError != nil {
			context.Fail(shutdownError)
		}

		asyncLock.Release()
	})
	if err != nil {
		return err
	}

	_, err = apiClient.WaitForChange(compute.ResourceTypeServer, serverID, "Shut down server", serverShutdownTimeout)
	if err != nil {
		return err
	}

	return nil
}

// Forcefully stop a server.
//
// Does not respect providerSettings.AllowServerReboots.
func serverPowerOff(providerState *providerState, serverID string) error {
	apiClient := providerState.Client()

	operationDescription := fmt.Sprintf("Power off server '%s'", serverID)
	err := providerState.RetryAction(operationDescription, func(context retry.Context) {
		asyncLock := providerState.AcquireAsyncOperationLock(operationDescription)
		defer asyncLock.Release()

		powerOffError := apiClient.PowerOffServer(serverID)
		if compute.IsResourceBusyError(powerOffError) {
			context.Retry()
		} else if powerOffError != nil {
			context.Fail(powerOffError)
		}
	})
	if err != nil {
		return err
	}

	_, err = apiClient.WaitForChange(compute.ResourceTypeServer, serverID, "Power off server", serverShutdownTimeout)
	if err != nil {
		return err
	}

	return nil
}

// Capture summary information for server backup.
func readServerBackupClientDownloadURLs(serverID string, data *schema.ResourceData, apiClient *compute.Client) error {
	log.Printf("Read backup details for server '%s'.", serverID)

	// Skip backup details as backup is not supported in appliances, a.k.a. otherunix.
	os := strings.ToUpper(data.Get(resourceKeyServerOSType).(string))

	if strings.Contains(os, "OTHER") || strings.Contains(os, "UN") || strings.Contains(os, "WIN") || strings.Contains(os, "LIN") || strings.Contains(os, "CEN") || os == "" {
		log.Printf("Backup is not supported for server '%s'.", serverID)
		data.Set(resourceKeyServerBackupEnabled, false)
		data.Set(resourceKeyServerBackupClientDownloadURLs, nil)
		return nil
	}

	backupDetails, err := apiClient.GetServerBackupDetails(serverID)

	if err != nil {
		return err
	}
	if backupDetails == nil {
		log.Printf("Backup is not enabled for server '%s'.", serverID)

		data.Set(resourceKeyServerBackupEnabled, false)
		data.Set(resourceKeyServerBackupClientDownloadURLs, nil)

		return nil
	}

	data.Set(resourceKeyServerBackupEnabled, true)

	clientDownloadURLs := make(map[string]interface{})
	for _, clientDetail := range backupDetails.Clients {
		clientType := strings.Replace(clientDetail.Type, ".", "_", -1)
		clientDownloadURLs[clientType] = clientDetail.DownloadURL
	}
	data.Set(resourceKeyServerBackupClientDownloadURLs, clientDownloadURLs)

	return nil
}
