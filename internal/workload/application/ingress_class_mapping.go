package application

var ingressClassMapping = map[string]IngressType{
	"nais-ingress":          IngressTypeInternal,
	"nais-ingress-external": IngressTypeExternal,
	"nais-ingress-fa":       IngressTypeAuthenticated,
	"internal-haproxy":      IngressTypeInternal,
	"external-haproxy":      IngressTypeExternal,
	"external-fa-haproxy":   IngressTypeAuthenticated,
}

// IsIngressClassExternallyExposed reports whether an ingress class represents
// external exposure of an application, including authenticated external
// ingress classes.
//
// Unknown or empty class names are treated as not externally exposed.
func IsIngressClassExternallyExposed(className string) bool {
	ingressType, ok := ingressClassMapping[className]
	if !ok {
		return false
	}

	return ingressType == IngressTypeExternal || ingressType == IngressTypeAuthenticated
}
