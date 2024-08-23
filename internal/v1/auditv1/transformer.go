package auditv1

type Transformer = func(entry AuditLogGeneric) AuditEntry

var knownTransformers = map[AuditLogResourceType]Transformer{}

func RegisterTransformer(resourceType AuditLogResourceType, transformer Transformer) {
	if _, ok := knownTransformers[resourceType]; ok {
		panic("transformer already registered: " + string(resourceType))
	}

	knownTransformers[resourceType] = transformer
}
