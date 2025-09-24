package opensearch

import "github.com/nais/api/internal/graph/apierror"

type machineType struct {
	AivenPlan  string
	Tier       OpenSearchTier
	Size       OpenSearchSize
	StorageMin StorageGB
	StorageMax StorageGB
}

var machineTypes = []machineType{
	{AivenPlan: "hobbyist", StorageMin: 16, StorageMax: 16, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM2gb},
	{AivenPlan: "startup-4", StorageMin: 80, StorageMax: 400, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM4gb},
	{AivenPlan: "startup-8", StorageMin: 175, StorageMax: 875, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM8gb},
	{AivenPlan: "startup-16", StorageMin: 350, StorageMax: 1750, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM16gb},
	{AivenPlan: "startup-32", StorageMin: 700, StorageMax: 3500, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM32gb},
	{AivenPlan: "startup-64", StorageMin: 1400, StorageMax: 5120, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM64gb},
	{AivenPlan: "business-4", StorageMin: 240, StorageMax: 1200, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM4gb},
	{AivenPlan: "business-8", StorageMin: 525, StorageMax: 2625, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM8gb},
	{AivenPlan: "business-16", StorageMin: 1050, StorageMax: 5250, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM16gb},
	{AivenPlan: "business-32", StorageMin: 2100, StorageMax: 10500, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM32gb},
	{AivenPlan: "business-64", StorageMin: 4200, StorageMax: 15360, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM64gb},
}

// tierAndSize transposes machineTypes for lookup by OpenSearchTier and OpenSearchSize
var tierAndSize map[OpenSearchTier]map[OpenSearchSize]machineType

// aivenPlans transposes machineTypes for lookup by an Aiven plan string
var aivenPlans map[string]machineType

func init() {
	tierAndSize = make(map[OpenSearchTier]map[OpenSearchSize]machineType)
	for _, m := range machineTypes {
		if _, ok := tierAndSize[m.Tier]; !ok {
			tierAndSize[m.Tier] = make(map[OpenSearchSize]machineType)
		}
		if _, ok := tierAndSize[m.Tier][m.Size]; ok {
			panic("duplicate tier and size combination [" + string(m.Tier) + ", " + string(m.Size) + "] in machineTypes")
		}
		tierAndSize[m.Tier][m.Size] = m
	}

	aivenPlans = make(map[string]machineType)
	for _, m := range machineTypes {
		if _, ok := aivenPlans[m.AivenPlan]; ok {
			panic("duplicate Aiven plan '" + m.AivenPlan + "' in machineTypes")
		}
		aivenPlans[m.AivenPlan] = m
	}
}

func machineTypeFromTierAndSize(tier OpenSearchTier, size OpenSearchSize) (*machineType, error) {
	sizes, ok := tierAndSize[tier]
	if !ok {
		return nil, apierror.Errorf("Invalid OpenSearch tier: %s", tier)
	}

	machine, ok := sizes[size]
	if !ok {
		return nil, apierror.Errorf("Invalid OpenSearch size for tier. %v cannot have size %v", tier, size)
	}

	return &machine, nil
}

func machineTypeFromPlan(plan string) (*machineType, error) {
	machine, ok := aivenPlans[plan]
	if !ok {
		return nil, apierror.Errorf("Invalid OpenSearch plan: %s", plan)
	}
	return &machine, nil
}
