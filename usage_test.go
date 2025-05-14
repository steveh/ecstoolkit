package ecstoolkit

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/steveh/ecstoolkit/cluster"
	"github.com/steveh/ecstoolkit/executor"
	"github.com/steveh/ecstoolkit/log"
)

//nolint:govet,testableexamples,goconst
func ExampleWithoutClusterWrapper() {
	ctx := context.Background()
	logger := slog.Default()

	cfg, _ := config.LoadDefaultConfig(ctx)
	ecsClient := ecs.NewFromConfig(cfg)
	kmsClient := kms.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	exec := executor.NewExecutor(ecsClient, kmsClient, ssmClient, log.NewSlogger(logger))

	clusterName := "mycluster"
	taskARN := "arn:aws:ecs:us-east-1:123456789012:task/mycluster/1234567890abcdef"
	containerName := "mycontainer"
	shellCmd := "bash"
	containerRuntimeID := "abcdef1234567890"

	_ = exec.ShellSession(ctx, &executor.ShellSessionOptions{
		ClusterName:        clusterName,
		TaskARN:            taskARN,
		ContainerName:      containerName,
		ContainerRuntimeID: containerRuntimeID,
		Command:            shellCmd,
	})
}

//nolint:govet,testableexamples
func ExampleAttachShellSession() {
	ctx := context.Background()
	logger := slog.Default()
	cfg, _ := config.LoadDefaultConfig(ctx)

	clus := cluster.NewCluster(cfg, "mycluster", log.NewSlogger(logger))
	taskARN, _ := arn.Parse("arn:aws:ecs:us-east-1:123456789012:task/mycluster/1234567890abcdef")
	containerName := "mycontainer"
	shellCmd := []string{"bash"}

	_ = clus.AttachShellSession(ctx, taskARN, containerName, shellCmd)
}

//nolint:govet,testableexamples
func ExampleAttachPortForwardingSession() {
	ctx := context.Background()
	logger := slog.Default()
	cfg, _ := config.LoadDefaultConfig(ctx)

	clus := cluster.NewCluster(cfg, "mycluster", log.NewSlogger(logger))
	taskARN, _ := arn.Parse("arn:aws:ecs:us-east-1:123456789012:task/mycluster/1234567890abcdef")
	containerName := "mycontainer"

	_ = clus.AttachPortForwardingSession(ctx, taskARN, containerName, 8080, 8080)
}
