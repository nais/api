package main

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	client, err := apiclient.New("localhost:3001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// Create deployment
	repo := "nais/testapp"
	resp, err := client.Deployments().CreateDeployment(ctx, &protoapi.CreateDeploymentRequest{
		TeamSlug:         "devteam",
		GithubRepository: &repo,
		Environment:      "dev",
	})
	if err != nil {
		panic(err)
	}

	id := resp.Id
	fmt.Println("Deployment ID:", id)

	// Add k8s resource
	_, err = client.Deployments().CreateDeploymentK8SResource(ctx, &protoapi.CreateDeploymentK8SResourceRequest{
		Group:        "nais.io",
		Version:      "v1alpha1",
		DeploymentId: id,
		Kind:         "Application",
		Name:         "testapp",
		Namespace:    "default",
	})
	if err != nil {
		panic(err)
	}

	// Set status
	_, err = client.Deployments().CreateDeploymentStatus(ctx, &protoapi.CreateDeploymentStatusRequest{
		DeploymentId: id,
		State:        protoapi.DeploymentState_in_progress,
		Message:      "Hello world",
	})
	if err != nil {
		panic(err)
	}
}
