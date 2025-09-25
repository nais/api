package valkey

import "github.com/nais/api/internal/graph/apierror"

type machineType struct {
	AivenPlan string
	Tier      ValkeyTier
	Memory    ValkeyMemory
}

var machineTypes = []machineType{
	{AivenPlan: "hobbyist", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM1gb},
	{AivenPlan: "startup-4", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM4gb},
	{AivenPlan: "startup-8", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM8gb},
	{AivenPlan: "startup-14", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM14gb},
	{AivenPlan: "startup-28", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM28gb},
	{AivenPlan: "startup-56", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM56gb},
	{AivenPlan: "startup-112", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM112gb},
	{AivenPlan: "startup-200", Tier: ValkeyTierSingleNode, Memory: ValkeyMemoryRAM200gb},
	{AivenPlan: "business-4", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM4gb},
	{AivenPlan: "business-8", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM8gb},
	{AivenPlan: "business-14", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM14gb},
	{AivenPlan: "business-28", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM28gb},
	{AivenPlan: "business-56", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM56gb},
	{AivenPlan: "business-112", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM112gb},
	{AivenPlan: "business-200", Tier: ValkeyTierHighAvailability, Memory: ValkeyMemoryRAM200gb},
}

// tierAndMemory transposes machineTypes for lookup by ValkeyTier and ValkeyMemory
var tierAndMemory map[ValkeyTier]map[ValkeyMemory]machineType

// aivenPlans transposes machineTypes for lookup by an Aiven plan string
var aivenPlans map[string]machineType

func init() {
	tierAndMemory = make(map[ValkeyTier]map[ValkeyMemory]machineType)
	for _, m := range machineTypes {
		if _, ok := tierAndMemory[m.Tier]; !ok {
			tierAndMemory[m.Tier] = make(map[ValkeyMemory]machineType)
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

func machineTypeFromTierAndMemory(tier ValkeyTier, memory ValkeyMemory) (*machineType, error) {
	memories, ok := tierAndMemory[tier]
	if !ok {
		return nil, apierror.Errorf("Invalid Valkey tier: %s", tier)
	}

	machine, ok := memories[memory]
	if !ok {
		return nil, apierror.Errorf("Invalid Valkey memory for tier. %v cannot have memory %v", tier, memory)
	}

	return &machine, nil
}

func machineTypeFromPlan(plan string) (*machineType, error) {
	machine, ok := aivenPlans[plan]
	if !ok {
		return nil, apierror.Errorf("Invalid Valkey plan: %s", plan)
	}
	return &machine, nil
}
