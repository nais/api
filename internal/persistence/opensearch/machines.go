package opensearch

import "github.com/nais/api/internal/graph/apierror"

type machineType struct {
	AivenPlan   string
	DiskSizeMin OpenSearchDiskSize
	DiskSizeMax OpenSearchDiskSize
	Tier        OpenSearchTier
	Size        OpenSearchSize
}

var machineTypes = []machineType{
	{AivenPlan: "hobbyist", DiskSizeMin: 16, DiskSizeMax: 16, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM2gb},
	{AivenPlan: "startup-4", DiskSizeMin: 80, DiskSizeMax: 400, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM4gb},
	{AivenPlan: "startup-8", DiskSizeMin: 175, DiskSizeMax: 875, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM8gb},
	{AivenPlan: "startup-16", DiskSizeMin: 350, DiskSizeMax: 1750, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM16gb},
	{AivenPlan: "startup-32", DiskSizeMin: 700, DiskSizeMax: 3500, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM32gb},
	{AivenPlan: "startup-64", DiskSizeMin: 1400, DiskSizeMax: 5120, Tier: OpenSearchTierSingleNode, Size: OpenSearchSizeRAM64gb},
	{AivenPlan: "business-4", DiskSizeMin: 240, DiskSizeMax: 1200, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM4gb},
	{AivenPlan: "business-8", DiskSizeMin: 525, DiskSizeMax: 2625, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM8gb},
	{AivenPlan: "business-16", DiskSizeMin: 1050, DiskSizeMax: 5250, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM16gb},
	{AivenPlan: "business-32", DiskSizeMin: 2100, DiskSizeMax: 10500, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM32gb},
	{AivenPlan: "business-64", DiskSizeMin: 4200, DiskSizeMax: 15360, Tier: OpenSearchTierHighAvailability, Size: OpenSearchSizeRAM64gb},
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
