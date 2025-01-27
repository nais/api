package main

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/utils/ptr"
)

func main() {
	client, err := apiclient.New("localhost:3001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// Create deployment
	repo := "nais/testapp"
	resp, err := client.Deployments().CreateDeployment(ctx, protoapi.CreateDeploymentRequest_builder{
		TeamSlug:        ptr.To("devteam"),
		Repository:      &repo,
		EnvironmentName: ptr.To("dev"),
	}.Build())
	if err != nil {
		panic(err)
	}

	id := resp.GetId()
	fmt.Println("Deployment ID:", id)

	// Add k8s resource
	_, err = client.Deployments().CreateDeploymentK8SResource(ctx, protoapi.CreateDeploymentK8SResourceRequest_builder{
		Group:        ptr.To("nais.io"),
		Version:      ptr.To("v1alpha1"),
		DeploymentId: ptr.To(id),
		Kind:         ptr.To("Application"),
		Name:         ptr.To("app-w-all-storage"),
		Namespace:    ptr.To("default"),
	}.Build())
	if err != nil {
		panic(err)
	}

	// Set status
	_, err = client.Deployments().CreateDeploymentStatus(ctx, protoapi.CreateDeploymentStatusRequest_builder{
		DeploymentId: ptr.To(id),
		State:        ptr.To(protoapi.DeploymentState_in_progress),
		Message:      ptr.To("Hello world"),
	}.Build())
	if err != nil {
		panic(err)
	}
}
