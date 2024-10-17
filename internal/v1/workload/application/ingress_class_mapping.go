package application

var ingressClassMapping = map[string]IngressType{
	"nais-ingress":          IngressTypeInternal,
	"nais-ingress-external": IngressTypeExternal,
	"nais-ingress-fa":       IngressTypeAuthenticated,
}
