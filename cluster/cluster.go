// Package cluster provides functionality for managing AWS ECS clusters, including
// service management, task definitions, and container operations.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/steveh/ecstoolkit/executor"
)

var (
	// ErrServiceNotFound is returned when a requested service cannot be found in the cluster.
	ErrServiceNotFound = errors.New("service not found")
	// ErrContainerNotFound is returned when a requested container cannot be found in a task.
	ErrContainerNotFound = errors.New("container not found")
	// ErrTaskNotFound is returned when a requested task cannot be found in the cluster.
	ErrTaskNotFound = errors.New("task not found")
	// ErrNoTasks is returned when there are no running tasks in the service.
	ErrNoTasks = errors.New("no tasks running")
	// ErrInvalidImage is returned when the image format is not valid.
	ErrInvalidImage = errors.New("invalid image format")
	// ErrInvalidTaskDefinition is returned when the task definition ARN format is not valid.
	ErrInvalidTaskDefinition = errors.New("invalid task definition ARN")
)

const maxWaitTime = 5 * time.Minute

// ServiceTaskDefinition represents a service and its associated task definition.
type ServiceTaskDefinition struct {
	Service        types.Service
	TaskDefinition types.TaskDefinition
}

// Cluster represents an AWS ECS cluster with methods for managing services and tasks.
type Cluster struct {
	ecsClient   *ecs.Client
	stsClient   *sts.Client
	clusterName string
	userID      string
	executor    *executor.Executor
}

// NewCluster creates a new Cluster instance with the provided AWS configuration and cluster name.
func NewCluster(awsCfg aws.Config, clusterName string) *Cluster {
	ecsClient := ecs.NewFromConfig(awsCfg)
	kmsClient := kms.NewFromConfig(awsCfg)
	ssmClient := ssm.NewFromConfig(awsCfg)
	stsClient := sts.NewFromConfig(awsCfg)

	exec := executor.NewExecutor(ecsClient, kmsClient, ssmClient, slog.Default())

	return &Cluster{
		ecsClient:   ecsClient,
		stsClient:   stsClient,
		clusterName: clusterName,
		executor:    exec,
	}
}

// chunk splits a slice into chunks of the specified size.
func chunk[T any](slice []T, size int) [][]T {
	if size <= 0 {
		return [][]T{slice}
	}

	var chunks [][]T

	for i := 0; i < len(slice); i += size {
		end := min(i+size, len(slice))
		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

// DescribeAllServices returns a list of all services in the cluster.
func (c *Cluster) DescribeAllServices(ctx context.Context) ([]types.Service, error) {
	const (
		MaxListServices     = 100
		MaxDescribeServices = 10
	)

	p := ecs.NewListServicesPaginator(c.ecsClient, &ecs.ListServicesInput{
		Cluster:            aws.String(c.clusterName),
		SchedulingStrategy: types.SchedulingStrategyReplica,
		MaxResults:         aws.Int32(MaxListServices),
	})

	var services []types.Service

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting next page: %w", err)
		}

		chunks := chunk(page.ServiceArns, MaxDescribeServices)
		for _, chunk := range chunks {
			describe, err := c.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(c.clusterName),
				Services: chunk,
			})
			if err != nil {
				return nil, fmt.Errorf("describing services: %w", err)
			}

			services = append(services, describe.Services...)
		}
	}

	return services, nil
}

// RunConsole starts a new task with an interactive console session.
func (c *Cluster) RunConsole(ctx context.Context, serviceName string, containerName string) (arn.ARN, error) {
	service, err := c.describeService(ctx, serviceName)
	if err != nil {
		return arn.ARN{}, err
	}

	userID, err := c.getUserID(ctx)
	if err != nil {
		return arn.ARN{}, fmt.Errorf("get caller identity: %w", err)
	}

	res, err := c.ecsClient.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:                  aws.String(c.clusterName),
		TaskDefinition:           service.TaskDefinition,
		CapacityProviderStrategy: nil,
		Count:                    aws.Int32(1),
		EnableECSManagedTags:     true,
		EnableExecuteCommand:     true,
		LaunchType:               types.LaunchTypeFargate,
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        service.NetworkConfiguration.AwsvpcConfiguration.Subnets,
				AssignPublicIp: service.NetworkConfiguration.AwsvpcConfiguration.AssignPublicIp,
				SecurityGroups: service.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
			},
		},
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{
				{
					Name:    aws.String(containerName),
					Command: []string{"sleep", "86400"}, // Sleep for 24 hours to keep container running
				},
			},
		},
		PropagateTags: types.PropagateTagsService,
		StartedBy:     aws.String(userID),
		Tags: []types.Tag{
			{
				Key:   aws.String("StartedBy"),
				Value: aws.String(userID),
			},
		},
	})
	if err != nil {
		return arn.ARN{}, fmt.Errorf("running task: %w", err)
	}

	taskARN, err := arn.Parse(*res.Tasks[0].TaskArn)
	if err != nil {
		return arn.ARN{}, fmt.Errorf("parsing running task arn: %w", err)
	}

	slog.Info("Waiting for task to start", "maxWaitTime", maxWaitTime)

	waiter := ecs.NewTasksRunningWaiter(c.ecsClient, func(o *ecs.TasksRunningWaiterOptions) {
		o.LogWaitAttempts = true
	})
	if err := waiter.Wait(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(c.clusterName),
		Tasks:   []string{taskARN.String()},
	}, maxWaitTime); err != nil {
		return arn.ARN{}, fmt.Errorf("waiting for task to start: %w", err)
	}

	return taskARN, nil
}

// ReplaceTaskDefinitionTag updates the image tag in a task definition.
func (c *Cluster) ReplaceTaskDefinitionTag(ctx context.Context, taskDefinitionARN arn.ARN, replacer func(repo, image, tag string) string) (arn.ARN, error) {
	family, _, err := splitTaskDefinitionARN(taskDefinitionARN)
	if err != nil {
		return arn.ARN{}, fmt.Errorf("split task definition arn: %w", err)
	}

	describe, err := c.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(family),
		Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
	})
	if err != nil {
		return arn.ARN{}, fmt.Errorf("describe task definition: %w", err)
	}

	taskDefinition := describe.TaskDefinition
	containerDefinitions := taskDefinition.ContainerDefinitions

	for i, container := range containerDefinitions {
		repo, image, tag, err := splitImage(*container.Image)
		if err != nil {
			return arn.ARN{}, fmt.Errorf("split image: %w", err)
		}

		replacement := replacer(repo, image, tag)

		containerDefinitions[i].Image = aws.String(fmt.Sprintf("%s/%s:%s", repo, image, replacement))
	}

	register, err := c.ecsClient.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:                  taskDefinition.Family,
		Cpu:                     taskDefinition.Cpu,
		EphemeralStorage:        taskDefinition.EphemeralStorage,
		ExecutionRoleArn:        taskDefinition.ExecutionRoleArn,
		InferenceAccelerators:   taskDefinition.InferenceAccelerators,
		IpcMode:                 taskDefinition.IpcMode,
		Memory:                  taskDefinition.Memory,
		NetworkMode:             taskDefinition.NetworkMode,
		PidMode:                 taskDefinition.PidMode,
		PlacementConstraints:    taskDefinition.PlacementConstraints,
		ProxyConfiguration:      taskDefinition.ProxyConfiguration,
		RequiresCompatibilities: taskDefinition.RequiresCompatibilities,
		RuntimePlatform:         taskDefinition.RuntimePlatform,
		Volumes:                 taskDefinition.Volumes,
		TaskRoleArn:             taskDefinition.TaskRoleArn,
		ContainerDefinitions:    containerDefinitions,
		Tags:                    describe.Tags,
	})
	if err != nil {
		return arn.ARN{}, fmt.Errorf("registering task definition: %w", err)
	}

	replacementARN, err := arn.Parse(*register.TaskDefinition.TaskDefinitionArn)
	if err != nil {
		return arn.ARN{}, fmt.Errorf("parsing replacement task definition arn: %w", err)
	}

	return replacementARN, nil
}

// Attach attaches to a running container and executes a command.
func (c *Cluster) Attach(ctx context.Context, taskARN arn.ARN, containerName string, command []string) error {
	describe, err := c.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(c.clusterName),
		Tasks:   []string{taskARN.String()},
	})
	if err != nil {
		return fmt.Errorf("describing tasks: %w", err)
	}

	if len(describe.Tasks) == 0 {
		return ErrTaskNotFound
	}

	containerRuntimeID, err := detectContainerRuntimeID(describe.Tasks, containerName)
	if err != nil {
		return err
	}

	if err := c.executor.ExecuteSession(ctx, &executor.ExecuteSessionOptions{
		ClusterName:        c.clusterName,
		TaskARN:            taskARN.String(),
		ContainerName:      containerName,
		ContainerRuntimeID: containerRuntimeID,
		Command:            strings.Join(command, " "),
	}); err != nil {
		return fmt.Errorf("executing command: %w", err)
	}

	return nil
}

// Deploy updates a service with a new task definition.
func (c *Cluster) Deploy(ctx context.Context, serviceName string, taskDefinitionARN arn.ARN) error {
	_, err := c.ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(c.clusterName),
		Service:            aws.String(serviceName),
		TaskDefinition:     aws.String(taskDefinitionARN.String()),
		ForceNewDeployment: true,
	})
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}

	return nil
}

// GetFirstTaskARN returns the ARN of the first running task for a service.
func (c *Cluster) GetFirstTaskARN(ctx context.Context, serviceName string) (arn.ARN, error) {
	listTasks, err := c.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(c.clusterName),
		ServiceName:   aws.String(serviceName),
		DesiredStatus: types.DesiredStatusRunning,
	})
	if err != nil {
		return arn.ARN{}, fmt.Errorf("listing tasks: %w", err)
	}

	if len(listTasks.TaskArns) == 0 {
		return arn.ARN{}, ErrNoTasks
	}

	parsedARN, err := arn.Parse(listTasks.TaskArns[0])
	if err != nil {
		return arn.ARN{}, fmt.Errorf("parsing task ARN: %w", err)
	}

	return parsedARN, nil
}

// DescribeServiceTaskDefinitions returns all services and their associated task definitions.
func (c *Cluster) DescribeServiceTaskDefinitions(ctx context.Context) ([]ServiceTaskDefinition, error) {
	services, err := c.DescribeAllServices(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]ServiceTaskDefinition, 0, len(services))

	for _, service := range services {
		res, err := c.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: service.TaskDefinition,
		})
		if err != nil {
			return nil, fmt.Errorf("describe task definition: %w", err)
		}

		results = append(results, ServiceTaskDefinition{
			Service:        service,
			TaskDefinition: *res.TaskDefinition,
		})
	}

	return results, nil
}

func (c *Cluster) getUserID(ctx context.Context) (string, error) {
	if c.userID != "" {
		return c.userID, nil
	}

	res, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("get caller identity: %w", err)
	}

	c.userID = *res.UserId

	return c.userID, nil
}

func (c *Cluster) describeService(ctx context.Context, serviceName string) (*types.Service, error) {
	describe, err := c.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &c.clusterName,
		Services: []string{serviceName},
	})
	if err != nil {
		return nil, fmt.Errorf("describe services: %w", err)
	}

	if len(describe.Services) == 0 {
		return nil, ErrServiceNotFound
	}

	return &describe.Services[0], nil
}

func detectContainerRuntimeID(tasks []types.Task, containerName string) (string, error) {
	for _, t := range tasks {
		for _, c := range t.Containers {
			if c.Name == nil || c.RuntimeId == nil {
				continue
			}

			if *c.Name == containerName {
				return *c.RuntimeId, nil
			}
		}
	}

	return "", ErrContainerNotFound
}

func splitImage(uri string) (string, string, string, error) {
	firstParts := strings.SplitN(uri, "/", 2) //nolint:mnd
	if len(firstParts) < 2 {                  //nolint:mnd
		return "", "", "", fmt.Errorf("%w: splitting repo and image: %s", ErrInvalidImage, uri)
	}

	secondParts := strings.Split(firstParts[1], ":")
	if len(secondParts) < 2 { //nolint:mnd
		return "", "", "", fmt.Errorf("%w: splitting image and tag: %s", ErrInvalidImage, uri)
	}

	repo := firstParts[0]
	image := secondParts[0]
	tag := secondParts[1]

	return repo, image, tag, nil
}

func splitTaskDefinitionARN(taskDefinitionARN arn.ARN) (string, int, error) {
	taskDefinitionResource := strings.Split(taskDefinitionARN.Resource, "/")
	if len(taskDefinitionResource) < 2 { //nolint:mnd
		return "", 0, fmt.Errorf("%w: splitting resource: %s", ErrInvalidTaskDefinition, taskDefinitionARN.Resource)
	}

	resourceParts := strings.Split(taskDefinitionResource[1], ":")
	if len(resourceParts) < 2 { //nolint:mnd
		return "", 0, fmt.Errorf("%w: splitting family and revision: %s", ErrInvalidTaskDefinition, taskDefinitionResource[1])
	}

	family := resourceParts[0]

	revision, err := strconv.ParseInt(resourceParts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("parsing revision: %w", err)
	}

	return family, int(revision), nil
}
