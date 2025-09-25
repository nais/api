package valkey

import "github.com/nais/api/internal/graph/apierror"

type machineType struct {
	AivenPlan string
	Tier      ValkeyTier
	Size      ValkeySize
}

var machineTypes = []machineType{
	{AivenPlan: "hobbyist", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM1gb},
	{AivenPlan: "startup-4", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM4gb},
	{AivenPlan: "startup-8", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM8gb},
	{AivenPlan: "startup-14", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM14gb},
	{AivenPlan: "startup-28", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM28gb},
	{AivenPlan: "startup-56", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM56gb},
	{AivenPlan: "startup-112", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM112gb},
	{AivenPlan: "startup-200", Tier: ValkeyTierSingleNode, Size: ValkeySizeRAM200gb},
	{AivenPlan: "business-4", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM4gb},
	{AivenPlan: "business-8", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM8gb},
	{AivenPlan: "business-14", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM14gb},
	{AivenPlan: "business-28", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM28gb},
	{AivenPlan: "business-56", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM56gb},
	{AivenPlan: "business-112", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM112gb},
	{AivenPlan: "business-200", Tier: ValkeyTierHighAvailability, Size: ValkeySizeRAM200gb},
}

// tierAndSize transposes machineTypes for lookup by ValkeyTier and ValkeySize
var tierAndSize map[ValkeyTier]map[ValkeySize]machineType

// aivenPlans transposes machineTypes for lookup by an Aiven plan string
var aivenPlans map[string]machineType

func init() {
	tierAndSize = make(map[ValkeyTier]map[ValkeySize]machineType)
	for _, m := range machineTypes {
		if _, ok := tierAndSize[m.Tier]; !ok {
			tierAndSize[m.Tier] = make(map[ValkeySize]machineType)
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

func machineTypeFromTierAndSize(tier ValkeyTier, size ValkeySize) (*machineType, error) {
	sizes, ok := tierAndSize[tier]
	if !ok {
		return nil, apierror.Errorf("Invalid Valkey tier: %s", tier)
	}

	machine, ok := sizes[size]
	if !ok {
		return nil, apierror.Errorf("Invalid Valkey size for tier. %v cannot have size %v", tier, size)
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
