package pubsublog

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type auditData struct {
	Reason string
	Data   map[string]any
}

var decoders = map[string]func(method string, l LogLine) (auditData, error){
	"/pods":            parsePod,
	"apps/deployments": parseDeployment,
	// Unmatches will use parseGeneric
}

func parsePod(method string, l LogLine) (auditData, error) {
	data := map[string]any{
		"user":   l.ProtoPayload.AuthenticationInfo.PrincipalEmail,
		"method": method,
	}

	reason := simpleReasonFromMethod(method)
	parts := strings.Split(method, ".")
	if len(parts) > 5 {
		reason = ""
		for _, part := range parts[5:] {
			reason += cases.Title(language.English).String(part)
		}
	}

	if strings.Contains(method, "pods.exec") || strings.Contains(method, "pods.ephemeralcontainers.patch") || strings.Contains(method, "pods.attach") {
		reason = "Exec"
	}

	return auditData{
		Reason: reason,
		Data:   data,
	}, nil
}

func parseDeployment(method string, l LogLine) (auditData, error) {
	data, err := parseGeneric(method, l)
	if err != nil {
		return data, err
	}

	if strings.Contains(method, "deployments.scale") {
		data.Reason = "Scale"
	}

	return data, err
}

func parseGeneric(method string, l LogLine) (auditData, error) {
	return auditData{
		Reason: simpleReasonFromMethod(method),
		Data: map[string]any{
			"user":   l.ProtoPayload.AuthenticationInfo.PrincipalEmail,
			"method": method,
		},
	}, nil
}

func simpleReasonFromMethod(method string) string {
	parts := strings.Split(method, ".")
	if len(parts) == 0 {
		return "unknown"
	}

	return cases.Title(language.English).String(parts[len(parts)-1])
}
