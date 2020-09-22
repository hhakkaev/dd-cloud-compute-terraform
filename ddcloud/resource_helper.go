package ddcloud

import (
	"log"
	"strconv"
	"strings"

	"github.com/hhakkaev/dd-cloud-compute-terraform/models"
	"github.com/hhakkaev/go-dd-cloud-compute/compute"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// resourcePropertyHelper provides commonly-used functionality for working with Terraform's schema.ResourceData.
type resourcePropertyHelper struct {
	data *schema.ResourceData
}

func propertyHelper(data *schema.ResourceData) resourcePropertyHelper {
	return resourcePropertyHelper{data}
}

func (helper resourcePropertyHelper) HasProperty(key string) bool {
	_, ok := helper.data.GetOk(key)

	return ok
}

func (helper resourcePropertyHelper) GetOptionalString(key string, allowEmpty bool) *string {
	value := helper.data.Get(key)
	switch typedValue := value.(type) {
	case string:
		if len(typedValue) > 0 || allowEmpty {
			return &typedValue
		}
	}

	return nil
}

func (helper resourcePropertyHelper) GetOptionalInt(key string, allowZero bool) *int {
	value := helper.data.Get(key)
	switch typedValue := value.(type) {
	case int:
		if typedValue != 0 || allowZero {
			return &typedValue
		}
	}

	return nil
}

func (helper resourcePropertyHelper) GetOptionalBool(key string) *bool {
	value := helper.data.Get(key)
	switch typedValue := value.(type) {
	case bool:
		return &typedValue
	default:
		return nil
	}
}

func (helper resourcePropertyHelper) GetStringSet(key string) (stringSet *schema.Set) {
	value, ok := helper.data.GetOk(key)
	if !ok || value == nil {
		return
	}

	stringSet = value.(*schema.Set)

	return
}

func (helper resourcePropertyHelper) SetStringSet(key string, stringSet *schema.Set) error {
	return helper.data.Set(key, stringSet)
}

func (helper resourcePropertyHelper) GetStringSetItems(key string) (items []string) {
	value, ok := helper.data.GetOk(key)
	if !ok || value == nil {
		return
	}
	rawItems := value.(*schema.Set).List()

	items = make([]string, len(rawItems))
	for index, item := range rawItems {
		items[index] = item.(string)
	}

	return
}

func (helper resourcePropertyHelper) SetStringSetItems(key string, items []string) error {
	rawItems := make([]interface{}, len(items))
	for index, item := range items {
		rawItems[index] = item
	}

	return helper.data.Set(key,
		schema.NewSet(schema.HashString, rawItems),
	)
}

func (helper resourcePropertyHelper) GetIntSetItems(key string) (items []int) {
	value, ok := helper.data.GetOk(key)
	if !ok || value == nil {
		return
	}
	rawItems := value.(*schema.Set).List()

	items = make([]int, len(rawItems))
	for index, item := range rawItems {
		items[index] = item.(int)
	}

	return
}

func (helper resourcePropertyHelper) SetIntSetItems(key string, items []int) error {
	rawItems := make([]interface{}, len(items))
	for index, item := range items {
		rawItems[index] = item
	}

	hashInt := func(value interface{}) int {
		return value.(int)
	}
	return helper.data.Set(key,
		schema.NewSet(hashInt, rawItems),
	)
}

func (helper resourcePropertyHelper) GetStringListItems(key string) (items []string) {
	value, ok := helper.data.GetOk(key)
	if !ok || value == nil {
		return
	}

	rawItems := value.([]interface{})
	items = make([]string, len(rawItems))
	for index, item := range rawItems {
		items[index] = item.(string)
	}

	return
}

func (helper resourcePropertyHelper) SetStringListItems(key string, items []string) error {
	rawItems := make([]interface{}, len(items))
	for index, item := range items {
		rawItems[index] = item
	}

	return helper.data.Set(key, rawItems)
}

func (helper resourcePropertyHelper) SetPartial(key string) {
	helper.data.SetPartial(key)
}

func (helper resourcePropertyHelper) GetTags(key string) (tags []compute.Tag) {
	value, ok := helper.data.GetOk(key)
	if !ok {
		return
	}
	tagData := value.(*schema.Set).List()

	tags = make([]compute.Tag, len(tagData))
	for index, item := range tagData {
		tagProperties := item.(map[string]interface{})
		tag := &compute.Tag{}

		value, ok = tagProperties[resourceKeyTagName]
		if ok {
			tag.Name = value.(string)
		}

		value, ok = tagProperties[resourceKeyTagValue]
		if ok {
			tag.Value = value.(string)
		}

		tags[index] = *tag
	}

	return
}

func (helper resourcePropertyHelper) SetTags(key string, tags []compute.Tag) {
	tagProperties := &schema.Set{F: hashTag}

	for _, tag := range tags {
		tagProperties.Add(map[string]interface{}{
			resourceKeyTagName:  tag.Name,
			resourceKeyTagValue: tag.Value,
		})
	}
	helper.data.Set(key, tagProperties)
}

func (helper resourcePropertyHelper) GetAddressListAddresses() (addresses []compute.IPAddressListEntry) {
	value, ok := helper.data.GetOk(resourceKeyAddressListAddress)
	if !ok {
		return
	}
	portListAddresses := value.([]interface{})

	addresses = make([]compute.IPAddressListEntry, len(portListAddresses))
	for index, item := range portListAddresses {
		entryProperties := item.(map[string]interface{})
		entry := &compute.IPAddressListEntry{}

		value, ok := entryProperties[resourceKeyAddressListAddressBegin]
		if ok {
			begin := value.(string)
			if len(begin) > 0 {
				log.Printf("Have address Begin '%s'", begin)
				entry.Begin = value.(string)

				value, ok = entryProperties[resourceKeyAddressListAddressEnd]
				if ok {
					endAddress := value.(string)
					log.Printf("Have address End '%s'", endAddress)
					if endAddress != "" {
						entry.End = &endAddress
					}
				}
			}
		}

		value, ok = entryProperties[resourceKeyAddressListAddressNetwork]
		if ok {
			network := value.(string)
			if len(network) > 0 {
				entry.Begin = network
				log.Printf("Have address Network '%s'", entry.Begin)

				value, ok = entryProperties[resourceKeyAddressListAddressPrefixSize]
				if ok {
					prefixSize := value.(int)
					log.Printf("Have address PrefixSize '%d'", prefixSize)
					entry.PrefixSize = &prefixSize
				}
			}
		}

		addresses[index] = *entry
	}

	return
}

func (helper resourcePropertyHelper) SetAddressListAddresses(addresses []compute.IPAddressListEntry) {
	addressProperties := make([]interface{}, len(addresses))
	for index, address := range addresses {
		if address.PrefixSize == nil {
			addressProperties[index] = map[string]interface{}{
				resourceKeyAddressListAddressBegin: address.Begin,
				resourceKeyAddressListAddressEnd:   address.End,
			}
		} else {
			addressProperties[index] = map[string]interface{}{
				resourceKeyAddressListAddressNetwork:    address.Begin,
				resourceKeyAddressListAddressPrefixSize: *address.PrefixSize,
			}
		}
	}

	helper.data.Set(resourceKeyAddressListAddress, addressProperties)
}

func (helper resourcePropertyHelper) GetPortListPorts() (ports []compute.PortListEntry) {
	value, ok := helper.data.GetOk(resourceKeyPortListPort)
	if !ok {
		return
	}
	portListPorts := value.([]interface{})

	ports = make([]compute.PortListEntry, len(portListPorts))
	for index, item := range portListPorts {
		portProperties := item.(map[string]interface{})
		port := &compute.PortListEntry{}

		value, ok := portProperties[resourceKeyPortListPortBegin]
		if ok {
			port.Begin = value.(int)
		}

		value, ok = portProperties[resourceKeyPortListPortEnd]
		if ok {
			endPort := value.(int)
			if endPort != 0 {
				port.End = &endPort
			}
		}

		ports[index] = *port
	}

	return
}

func (helper resourcePropertyHelper) SetPortListPorts(ports []compute.PortListEntry) {
	portProperties := make([]interface{}, len(ports))
	for index, port := range ports {
		portProperties[index] = map[string]interface{}{
			resourceKeyPortListPortBegin: port.Begin,
			resourceKeyPortListPortEnd:   port.End,
		}
	}

	helper.data.Set(resourceKeyPortListPort, portProperties)
}

func (helper resourcePropertyHelper) GetImage() *models.Image {
	value, ok := helper.data.GetOk(resourceKeyServerImage)
	if !ok {
		return nil
	}

	// Unfortunate limitation of Terraform's schema model - this has to be a list with a single item rather than simply a nested object.
	singleItemList := value.([]interface{})
	if len(singleItemList) < 1 {
		return nil
	}

	imageProperties := singleItemList[0].(map[string]interface{})
	image := models.NewImageFromMap(imageProperties)

	return &image
}

func (helper resourcePropertyHelper) SetImage(image *models.Image) {
	if image != nil {
		// Unfortunate limitation of Terraform's schema model - this has to be a list with a single item rather than simply a nested object.
		singleItemList := []interface{}{
			image.ToMap(),
		}

		helper.data.Set(resourceKeyServerImage, singleItemList)
	} else {
		helper.data.Set(resourceKeyServerImage, nil)
	}
}

func (helper resourcePropertyHelper) GetDisks() (disks models.Disks) {
	value, ok := helper.data.GetOk(resourceKeyServerDisk)
	if !ok {
		return
	}
	serverDisks, ok := value.([]interface{})
	if !ok {
		return
	}

	disks = models.NewDisksFromStateData(serverDisks)

	return
}

func (helper resourcePropertyHelper) GetOldDisks() (disks models.Disks) {
	if !helper.data.HasChange(resourceKeyServerDisk) {
		return helper.GetDisks()
	}

	oldValue, _ := helper.data.GetChange(resourceKeyServerDisk)
	serverDisks, ok := oldValue.([]interface{})
	if !ok {
		return
	}

	disks = models.NewDisksFromStateData(serverDisks)

	return
}

func (helper resourcePropertyHelper) SetDisks(disks models.Disks) {
	diskProperties := make([]interface{}, len(disks))
	for index, disk := range disks {
		diskProperties[index] = disk.ToMap()
	}
	helper.data.Set(resourceKeyServerDisk, diskProperties)
}

func (helper resourcePropertyHelper) GetServerBackupClients() (backupClients models.ServerBackupClients) {
	value, ok := helper.data.GetOk(resourceKeyServerBackupClients)
	if !ok {
		return
	}
	serverBackupClients, ok := value.([]interface{})
	if !ok {
		return
	}

	backupClients = models.NewServerBackupClientsFromStateData(serverBackupClients)

	return
}

func (helper resourcePropertyHelper) SetServerBackupClients(serverBackupClients models.ServerBackupClients) {
	serverBackupClientProperties := make([]interface{}, len(serverBackupClients))
	for index, serverBackupClient := range serverBackupClients {
		serverBackupClientProperties[index] = serverBackupClient.ToMap()
	}
	helper.data.Set(resourceKeyServerBackupClients, serverBackupClientProperties)
}

func (helper resourcePropertyHelper) GetServerNetworkAdapters() (networkAdapters models.NetworkAdapters) {
	// Primary network adapter.
	value, ok := helper.data.GetOk(resourceKeyServerPrimaryNetworkAdapter)
	if !ok {
		return
	}
	networkAdapters = models.NewNetworkAdaptersFromStateData(
		value.([]interface{}),
	)

	// Additional network adapter.
	value, ok = helper.data.GetOk(resourceKeyServerAdditionalNetworkAdapter)
	if !ok {
		return
	}

	networkAdapters = append(networkAdapters,
		models.NewNetworkAdaptersFromStateData(
			value.([]interface{}),
		)...,
	)

	return
}

func (helper resourcePropertyHelper) GetOldServerNetworkAdapters() (networkAdapters models.NetworkAdapters) {
	if !(helper.data.HasChange(resourceKeyServerPrimaryNetworkAdapter) || helper.data.HasChange(resourceKeyServerAdditionalNetworkAdapter)) {
		networkAdapters = helper.GetServerNetworkAdapters()

		return
	}

	// Primary network adapter.
	oldValue, _ := helper.data.GetChange(resourceKeyServerPrimaryNetworkAdapter)
	if oldValue == nil {
		return
	}
	networkAdapters = models.NewNetworkAdaptersFromStateData(
		oldValue.([]interface{}),
	)

	// Additional network adapters.
	oldValue, _ = helper.data.GetChange(resourceKeyServerAdditionalNetworkAdapter)
	if oldValue == nil {
		return
	}
	networkAdapters = append(networkAdapters,
		models.NewNetworkAdaptersFromStateData(
			oldValue.([]interface{}),
		)...,
	)

	return
}

func (helper resourcePropertyHelper) SetServerNetworkAdapters(networkAdapters models.NetworkAdapters, isPartial bool) {
	data := helper.data
	if isPartial {
		data.SetPartial(resourceKeyServerPrimaryNetworkAdapter)
		data.SetPartial(resourceKeyServerAdditionalNetworkAdapter)
	}

	if networkAdapters.IsEmpty() {
		data.Set(resourceKeyServerPrimaryNetworkAdapter, [0]interface{}{})
		data.Set(resourceKeyServerAdditionalNetworkAdapter, [0]interface{}{})

		return
	}

	// Primary network adapter.
	networkAdapterProperties := []interface{}{
		networkAdapters.GetPrimary().ToMap(),
	}
	data.Set(resourceKeyServerPrimaryNetworkAdapter, networkAdapterProperties)

	if len(networkAdapters) == 1 {
		data.Set(resourceKeyServerAdditionalNetworkAdapter, [0]interface{}{})

		return // No additional network adapters.
	}

	// Additional network adapters.
	networkAdapterPropertiesList := make([]interface{}, len(networkAdapters)-1)
	for index, additionalNetworkAdapter := range networkAdapters.GetAdditional() {
		networkAdapterPropertiesList[index] = additionalNetworkAdapter.ToMap()

		log.Printf("[DD] Resource Helper > SetServerNetworkAdapters ID:%s ipv4:%s ipv6:%s ",
			additionalNetworkAdapter.ID, additionalNetworkAdapter.PrivateIPv4Address, additionalNetworkAdapter.PrivateIPv6Address)
	}

	data.Set(resourceKeyServerAdditionalNetworkAdapter, networkAdapterPropertiesList)
}

func (helper resourcePropertyHelper) GetNetworkAdapter() models.NetworkAdapter {
	data := helper.data

	networkAdapter := &models.NetworkAdapter{
		ID: data.Id(),
	}

	value, ok := data.GetOk(resourceKeyNetworkAdapterMACAddress)
	if ok && value != nil {
		networkAdapter.MACAddress = value.(string)
	}
	value, ok = data.GetOk(resourceKeyNetworkAdapterVLANID)
	if ok && value != nil {
		networkAdapter.VLANID = value.(string)
	}
	value, ok = data.GetOk(resourceKeyNetworkAdapterPrivateIPV4)
	if ok && value != nil {
		networkAdapter.PrivateIPv4Address = value.(string)
	}
	value, ok = data.GetOk(resourceKeyNetworkAdapterPrivateIPV6)
	if ok && value != nil {
		networkAdapter.PrivateIPv6Address = value.(string)
	}
	value, ok = data.GetOk(resourceKeyNetworkAdapterType)
	if ok && value != nil {
		networkAdapter.AdapterType = value.(string)
	}

	return *networkAdapter
}

func (helper resourcePropertyHelper) SetNetworkAdapter(networkAdapter models.NetworkAdapter, isPartial bool) {
	data := helper.data
	if isPartial {
		data.SetPartial(resourceKeyNetworkAdapterMACAddress)
		data.SetPartial(resourceKeyNetworkAdapterVLANID)
		data.SetPartial(resourceKeyNetworkAdapterPrivateIPV4)
		data.SetPartial(resourceKeyNetworkAdapterPrivateIPV6)
		data.SetPartial(resourceKeyNetworkAdapterType)
	}

	data.Set(resourceKeyNetworkAdapterMACAddress, networkAdapter.MACAddress)
	data.Set(resourceKeyNetworkAdapterVLANID, networkAdapter.VLANID)
	data.Set(resourceKeyNetworkAdapterPrivateIPV4, networkAdapter.PrivateIPv4Address)
	data.Set(resourceKeyNetworkAdapterPrivateIPV6, networkAdapter.PrivateIPv6Address)
	data.Set(resourceKeyNetworkAdapterType, networkAdapter.AdapterType)
}

func (helper resourcePropertyHelper) GetVirtualListenerIRuleIDs(apiClient *compute.Client) (iRuleIDs []string, err error) {
	var iRules []compute.EntityReference
	iRules, err = helper.GetVirtualListenerIRules(apiClient)
	if err != nil {
		return
	}

	iRuleIDs = make([]string, len(iRules))
	for index, iRule := range iRules {
		iRuleIDs[index] = iRule.ID
	}

	return
}

func (helper resourcePropertyHelper) GetVirtualListenerIRuleNames(apiClient *compute.Client) (iRuleNames []string, err error) {
	var iRules []compute.EntityReference
	iRules, err = helper.GetVirtualListenerIRules(apiClient)
	if err != nil {
		return
	}

	iRuleNames = make([]string, len(iRules))
	for index, iRule := range iRules {
		iRuleNames[index] = iRule.Name
	}

	return
}

func (helper resourcePropertyHelper) GetVirtualListenerIRules(apiClient *compute.Client) (iRules []compute.EntityReference, err error) {
	value, ok := helper.data.GetOk(resourceKeyVirtualListenerIRuleNames)
	if !ok {
		return
	}
	iRuleNames := value.(*schema.Set)
	if iRuleNames.Len() == 0 {
		return
	}

	networkDomainID := helper.data.Get(resourceKeyVirtualListenerNetworkDomainID).(string)

	page := compute.DefaultPaging()
	for {
		var results *compute.IRules
		results, err = apiClient.ListDefaultIRules(networkDomainID, page)
		if err != nil {
			return
		}
		if results.IsEmpty() {
			break // We're done
		}

		for _, iRule := range results.Items {
			if iRuleNames.Contains(iRule.Name) {
				iRules = append(iRules, iRule.ToEntityReference())
			}
		}

		page.Next()
	}

	return
}

func (helper resourcePropertyHelper) SetVirtualListenerIRules(iRuleSummaries []compute.EntityReference) {
	iRuleNames := &schema.Set{F: schema.HashString}

	for _, iRuleSummary := range iRuleSummaries {
		iRuleNames.Add(iRuleSummary.Name)
	}

	helper.data.Set(resourceKeyVirtualListenerIRuleNames, iRuleNames)
}

func (helper resourcePropertyHelper) GetVirtualListenerPersistenceProfileID(apiClient *compute.Client) (persistenceProfileID *string, err error) {
	persistenceProfile, err := helper.GetVirtualListenerPersistenceProfile(apiClient)
	if err != nil {
		return nil, err
	}

	if persistenceProfile != nil {
		return &persistenceProfile.ID, nil
	}

	return nil, nil
}

func (helper resourcePropertyHelper) GetVirtualListenerPersistenceProfile(apiClient *compute.Client) (persistenceProfile *compute.EntityReference, err error) {
	value, ok := helper.data.GetOk(resourceKeyVirtualListenerPersistenceProfileName)
	if !ok {
		return
	}
	persistenceProfileName := value.(string)

	networkDomainID := helper.data.Get(resourceKeyVirtualListenerNetworkDomainID).(string)

	page := compute.DefaultPaging()
	for {
		var persistenceProfiles *compute.PersistenceProfiles
		persistenceProfiles, err = apiClient.ListDefaultPersistenceProfiles(networkDomainID, page)
		if err != nil {
			return
		}
		if persistenceProfiles.IsEmpty() {
			break // We're done
		}

		for _, profile := range persistenceProfiles.Items {
			if profile.Name == persistenceProfileName {
				persistenceProfileReference := profile.ToEntityReference()
				persistenceProfile = &persistenceProfileReference

				return
			}
		}

		page.Next()
	}

	return
}

func (helper resourcePropertyHelper) SetVirtualListenerPersistenceProfile(persistenceProfile compute.EntityReference) (err error) {
	return helper.data.Set(resourceKeyVirtualListenerPersistenceProfileName, persistenceProfile.Name)
}

func normalizeSpeed(value interface{}) string {
	speed := value.(string)

	return strings.ToUpper(speed)
}

func normalizeVIPMemberPort(port *int) string {
	if port != nil {
		return strconv.Itoa(*port)
	}

	return "ANY"
}
