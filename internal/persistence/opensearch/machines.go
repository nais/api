package opensearch

import "github.com/nais/api/internal/graph/apierror"

type machineType struct {
	AivenPlan  string
	Tier       OpenSearchTier
	Memory     OpenSearchMemory
	StorageMin StorageGB
	StorageMax StorageGB
}

var machineTypes = []machineType{
	{AivenPlan: "hobbyist", StorageMin: 16, StorageMax: 16, Tier: OpenSearchTierSingleNode, Memory: OpenSearchMemoryGB2},
	{AivenPlan: "startup-4", StorageMin: 80, StorageMax: 400, Tier: OpenSearchTierSingleNode, Memory: OpenSearchMemoryGB4},
	{AivenPlan: "startup-8", StorageMin: 175, StorageMax: 875, Tier: OpenSearchTierSingleNode, Memory: OpenSearchMemoryGB8},
	{AivenPlan: "startup-16", StorageMin: 350, StorageMax: 1750, Tier: OpenSearchTierSingleNode, Memory: OpenSearchMemoryGB16},
	{AivenPlan: "startup-32", StorageMin: 700, StorageMax: 3500, Tier: OpenSearchTierSingleNode, Memory: OpenSearchMemoryGB32},
	{AivenPlan: "startup-64", StorageMin: 1400, StorageMax: 5120, Tier: OpenSearchTierSingleNode, Memory: OpenSearchMemoryGB64},
	{AivenPlan: "business-4", StorageMin: 240, StorageMax: 1200, Tier: OpenSearchTierHighAvailability, Memory: OpenSearchMemoryGB4},
	{AivenPlan: "business-8", StorageMin: 525, StorageMax: 2625, Tier: OpenSearchTierHighAvailability, Memory: OpenSearchMemoryGB8},
	{AivenPlan: "business-16", StorageMin: 1050, StorageMax: 5250, Tier: OpenSearchTierHighAvailability, Memory: OpenSearchMemoryGB16},
	{AivenPlan: "business-32", StorageMin: 2100, StorageMax: 10500, Tier: OpenSearchTierHighAvailability, Memory: OpenSearchMemoryGB32},
	{AivenPlan: "business-64", StorageMin: 4200, StorageMax: 15360, Tier: OpenSearchTierHighAvailability, Memory: OpenSearchMemoryGB64},
}

// tierAndMemory transposes machineTypes for lookup by OpenSearchTier and OpenSearchMemory
var tierAndMemory map[OpenSearchTier]map[OpenSearchMemory]machineType

// aivenPlans transposes machineTypes for lookup by an Aiven plan string
var aivenPlans map[string]machineType

func init() {
	tierAndMemory = make(map[OpenSearchTier]map[OpenSearchMemory]machineType)
	for _, m := range machineTypes {
		if _, ok := tierAndMemory[m.Tier]; !ok {
			tierAndMemory[m.Tier] = make(map[OpenSearchMemory]machineType)
		}
		if _, ok := tierAndMemory[m.Tier][m.Memory]; ok {
			panic("duplicate tier and memory combination [" + string(m.Tier) + ", " + string(m.Memory) + "] in machineTypes")
		}
		tierAndMemory[m.Tier][m.Memory] = m
	}

	aivenPlans = make(map[string]machineType)
	for _, m := range machineTypes {
		if _, ok := aivenPlans[m.AivenPlan]; ok {
			panic("duplicate Aiven plan '" + m.AivenPlan + "' in machineTypes")
		}
		aivenPlans[m.AivenPlan] = m
	}
}

func machineTypeFromTierAndMemory(tier OpenSearchTier, memory OpenSearchMemory) (*machineType, error) {
	memories, ok := tierAndMemory[tier]
	if !ok {
		return nil, apierror.Errorf("Invalid OpenSearch tier: %s", tier)
	}

	machine, ok := memories[memory]
	if !ok {
		return nil, apierror.Errorf("Invalid OpenSearch memory for tier. %v cannot have memory %v", tier, memory)
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
