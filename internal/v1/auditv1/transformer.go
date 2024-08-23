package auditv1

type Transformer = func(entry AuditLogGeneric) AuditEntry

var knownTransformers = map[AuditResourceType]Transformer{}

func RegisterTransformer(resourceType AuditResourceType, transformer Transformer) {
	if _, ok := knownTransformers[resourceType]; ok {
		panic("transformer already registered: " + string(resourceType))
	}

	knownTransformers[resourceType] = transformer
}
