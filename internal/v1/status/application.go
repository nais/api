package status

// func forApplication(app *application.Application, instances, failingInstances []*application.Instance) []WorkloadStatusError {
// 	var ret []WorkloadStatusError
// 	for _, ingress := range app.Ingresses() {
// 		i := strings.Join(strings.Split(ingress, ".")[1:], ".")
// 		for _, deprecatedIngress := range deprecatedIngresses[app.EnvironmentName] {
// 			if i == deprecatedIngress {
// 				ret = append(ret, &DeprecatedIngressError{
// 					Level:   WorkloadStatusErrorLevelTodo,
// 					Ingress: ingress,
// 				})
// 			}
// 		}
// 	}

// 	resources := app.Resources()
// 	if (len(instances) == 0 || len(failingInstances) == len(instances)) && resources.Scaling.MinInstances > 0 && resources.Scaling.MaxInstances > 0 {
// 		ret = append(ret, &NoRunningInstancesError{
// 			Level: WorkloadStatusErrorLevelError,
// 		})
// 	}

// 	return ret
// }
