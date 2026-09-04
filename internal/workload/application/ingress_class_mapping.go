package application

var ingressClassMapping = map[string]IngressType{
	"nais-ingress":          IngressTypeInternal,
	"nais-ingress-external": IngressTypeExternal,
	"nais-ingress-fa":       IngressTypeAuthenticated,
	"internal-haproxy":      IngressTypeInternal,
	"external-haproxy":      IngressTypeExternal,
	"external-fa-haproxy":   IngressTypeAuthenticated,
}

func ClassifyIngressClassName(className string) IngressType {
	if t, ok := ingressClassMapping[className]; ok {
		return t
	}
	return IngressTypeUnknown
}
