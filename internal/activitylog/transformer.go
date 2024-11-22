package activitylog

type Transformer = func(entry GenericActivityLogEntry) (ActivityLogEntry, error)

var knownTransformers = map[ActivityLogResourceType]Transformer{}

func RegisterTransformer(resourceType ActivityLogResourceType, transformer Transformer) {
	if _, ok := knownTransformers[resourceType]; ok {
		panic("transformer already registered: " + string(resourceType))
	}

	knownTransformers[resourceType] = transformer
}
