package activitylog

type Transformer = func(entry GenericActivityLogEntry) (ActivityLogEntry, error)

var (
	knownTransformers          = map[ActivityLogEntryResourceType]Transformer{}
	knownTransformersForAction = map[ActivityLogEntryAction]Transformer{}
)

func RegisterTransformer(resourceType ActivityLogEntryResourceType, transformer Transformer) {
	if _, ok := knownTransformers[resourceType]; ok {
		panic("transformer already registered: " + string(resourceType))
	}

	knownTransformers[resourceType] = transformer
}

func RegisterTransformerForAction(action ActivityLogEntryAction, transformer Transformer) {
	if _, ok := knownTransformersForAction[action]; ok {
		panic("transformer already registered for action: " + string(action))
	}

	knownTransformersForAction[action] = transformer
}
