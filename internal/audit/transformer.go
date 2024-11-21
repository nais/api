package audit

type Transformer = func(entry GenericAuditEntry) (AuditEntry, error)

var knownTransformers = map[AuditResourceType]Transformer{}

func RegisterTransformer(resourceType AuditResourceType, transformer Transformer) {
	if _, ok := knownTransformers[resourceType]; ok {
		panic("transformer already registered: " + string(resourceType))
	}

	knownTransformers[resourceType] = transformer
}
