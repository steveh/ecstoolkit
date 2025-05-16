// Package cluster provides functionality for managing AWS ECS clusters, including
// service management, task definitions, and container operations.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/steveh/ecstoolkit/executor"
	"github.com/steveh/ecstoolkit/log"
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
	// ErrInvalidOption is returned when an invalid option is provided.
	ErrInvalidOption = errors.New("invalid option")
)

const maxWaitTime = 5 * time.Minute

// ServiceTaskDefinition represents a service and its associated task definition.
type ServiceTaskDefinition struct {
	Service        ecstypes.Service
	TaskDefinition ecstypes.TaskDefinition
}

// Cluster represents an AWS ECS cluster with methods for managing services and tasks.
type Cluster struct {
	ecsClient         *ecs.Client
	stsClient         *sts.Client
	autoscalingClient *applicationautoscaling.Client
	clusterName       string
	region            string
	userID            string
	accountID         string
	executor          *executor.Executor
	logger            log.T
}

// NewCluster creates a new Cluster instance with the provided AWS configuration and cluster name.
func NewCluster(awsCfg aws.Config, clusterName string, logger log.T) *Cluster {
	ecsClient := ecs.NewFromConfig(awsCfg)
	kmsClient := kms.NewFromConfig(awsCfg)
	ssmClient := ssm.NewFromConfig(awsCfg)
	stsClient := sts.NewFromConfig(awsCfg)
	autoscalingClient := applicationautoscaling.NewFromConfig(awsCfg)

	exec := executor.NewExecutor(ecsClient, kmsClient, ssmClient, logger)

	return &Cluster{
		region:            awsCfg.Region,
		ecsClient:         ecsClient,
		stsClient:         stsClient,
		autoscalingClient: autoscalingClient,
		clusterName:       clusterName,
		executor:          exec,
		logger:            logger.With("subsystem", "Cluster"),
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
func (c *Cluster) DescribeAllServices(ctx context.Context) ([]ecstypes.Service, error) {
	const (
		MaxListServices     = 100
		MaxDescribeServices = 10
	)

	p := ecs.NewListServicesPaginator(c.ecsClient, &ecs.ListServicesInput{
		Cluster:            aws.String(c.clusterName),
		SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
		MaxResults:         aws.Int32(MaxListServices),
	})

	var services []ecstypes.Service

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting next page: %w", err)
		}

		chunks := chunk(page.ServiceArns, MaxDescribeServices)
		for _, chunk := range chunks {
			c.logger.Debug("Describing services", "len", len(chunk))

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
	service, err := c.DescribeService(ctx, serviceName)
	if err != nil {
		return arn.ARN{}, err
	}

	userID, err := c.getUserID(ctx)
	if err != nil {
		return arn.ARN{}, fmt.Errorf("get caller identity: %w", err)
	}

	c.logger.Debug("Running task", "service", serviceName, "container", containerName)

	res, err := c.ecsClient.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:                  aws.String(c.clusterName),
		TaskDefinition:           service.TaskDefinition,
		CapacityProviderStrategy: nil,
		Count:                    aws.Int32(1),
		EnableECSManagedTags:     true,
		EnableExecuteCommand:     true,
		LaunchType:               ecstypes.LaunchTypeFargate,
		NetworkConfiguration: &ecstypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
				Subnets:        service.NetworkConfiguration.AwsvpcConfiguration.Subnets,
				AssignPublicIp: service.NetworkConfiguration.AwsvpcConfiguration.AssignPublicIp,
				SecurityGroups: service.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
			},
		},
		Overrides: &ecstypes.TaskOverride{
			ContainerOverrides: []ecstypes.ContainerOverride{
				{
					Name:    aws.String(containerName),
					Command: []string{"sleep", "86400"}, // Sleep for 24 hours to keep container running
				},
			},
		},
		PropagateTags: ecstypes.PropagateTagsTaskDefinition,
		StartedBy:     aws.String(userID),
		Tags: []ecstypes.Tag{
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

	c.logger.Info("Waiting for task to start", "maxWaitTime", maxWaitTime)

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

// DescribeTaskDefinition retrieves the task definition for a given family.
func (c *Cluster) DescribeTaskDefinition(ctx context.Context, family string) (*ecs.DescribeTaskDefinitionOutput, error) {
	c.logger.Debug("Describing task definition", "family", family)

	describe, err := c.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(family),
		Include:        []ecstypes.TaskDefinitionField{ecstypes.TaskDefinitionFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("describing task definition: %w", err)
	}

	return describe, nil
}

// ReplaceTaskDefinitionTag updates the image tag in a task definition.
func (c *Cluster) ReplaceTaskDefinitionTag(ctx context.Context, taskDefinitionARN arn.ARN, replacer func(repo, image, tag string) string) (arn.ARN, error) {
	family, _, err := splitTaskDefinitionARN(taskDefinitionARN)
	if err != nil {
		return arn.ARN{}, fmt.Errorf("split task definition arn: %w", err)
	}

	describe, err := c.DescribeTaskDefinition(ctx, family)
	if err != nil {
		return arn.ARN{}, err
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

	c.logger.Debug("Registering task definition", "family", family)

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

// AttachPortForwardingSession attaches to a running container and forwards a port to the local machine.
func (c *Cluster) AttachPortForwardingSession(ctx context.Context, taskARN arn.ARN, containerName string, host string, portNumber int, localPortNumber int) error {
	containerRuntimeID, err := c.getContainerRuntimeID(ctx, taskARN, containerName)
	if err != nil {
		return err
	}

	if err := c.executor.PortForwardSession(ctx, &executor.PortForwardSessionOptions{
		ClusterName:        c.clusterName,
		TaskARN:            taskARN.String(),
		ContainerRuntimeID: containerRuntimeID,
		Host:               host,
		PortNumber:         portNumber,
		LocalPortNumber:    localPortNumber,
	}); err != nil {
		return fmt.Errorf("starting port forwarding session: %w", err)
	}

	return nil
}

// AttachShellSession attaches to a running container and executes a command in a shell session.
func (c *Cluster) AttachShellSession(ctx context.Context, taskARN arn.ARN, containerName string, command []string) error {
	containerRuntimeID, err := c.getContainerRuntimeID(ctx, taskARN, containerName)
	if err != nil {
		return err
	}

	c.logger.Debug("Attaching shell session", "taskARN", taskARN.String(), "containerName", containerName, "containerRuntimeID", containerRuntimeID, "command", command)

	if err := c.executor.ShellSession(ctx, &executor.ShellSessionOptions{
		ClusterName:        c.clusterName,
		TaskARN:            taskARN.String(),
		ContainerName:      containerName,
		ContainerRuntimeID: containerRuntimeID,
		Command:            strings.Join(command, " "),
	}); err != nil {
		return fmt.Errorf("starting shell session: %w", err)
	}

	return nil
}

// Deploy updates a service with a new task definition.
func (c *Cluster) Deploy(ctx context.Context, serviceName string, taskDefinitionARN arn.ARN) error {
	c.logger.Debug("Updating service", "service", serviceName, "taskDefinitionARN", taskDefinitionARN.String())

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
	c.logger.Debug("Listing tasks", "service", serviceName)

	listTasks, err := c.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(c.clusterName),
		ServiceName:   aws.String(serviceName),
		DesiredStatus: ecstypes.DesiredStatusRunning,
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
		describe, err := c.DescribeTaskDefinition(ctx, *service.TaskDefinition)
		if err != nil {
			return nil, err
		}

		results = append(results, ServiceTaskDefinition{
			Service:        service,
			TaskDefinition: *describe.TaskDefinition,
		})
	}

	return results, nil
}

// DescribeService retrieves the details of a specific service in the cluster.
func (c *Cluster) DescribeService(ctx context.Context, serviceName string) (*ecstypes.Service, error) {
	c.logger.Debug("Describing service", "service", serviceName)

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

// Start starts a service by enabling autoscaling.
func (c *Cluster) Start(ctx context.Context, serviceName string) error {
	return c.Autoscale(ctx, serviceName, &AutoscalingOptions{
		ScaleIn:          aws.Bool(true),
		ScaleOut:         aws.Bool(true),
		ScheduledScaling: aws.Bool(true),
	})
}

// Stop stops a service by disabling autoscaling and setting the desired count to zero.
func (c *Cluster) Stop(ctx context.Context, serviceName string) error {
	if err := c.Autoscale(ctx, serviceName, &AutoscalingOptions{
		ScaleIn:          aws.Bool(false),
		ScaleOut:         aws.Bool(false),
		ScheduledScaling: aws.Bool(false),
	}); err != nil {
		return err
	}

	return c.setDesiredCount(ctx, serviceName, 0)
}

// Restart restarts a service by forcing a new deployment.
func (c *Cluster) Restart(ctx context.Context, serviceName string) error {
	c.logger.Debug("Updating service", "service", serviceName)

	if _, err := c.ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            &c.clusterName,
		Service:            &serviceName,
		ForceNewDeployment: true,
	}); err != nil {
		return fmt.Errorf("forcing new deployment: %w", err)
	}

	return nil
}

// AggregateLogGroupARNs aggregates the log group ARNs from the task definitions of the provided services.
func (c *Cluster) AggregateLogGroupARNs(ctx context.Context, services []ecstypes.Service) ([]string, error) {
	logGroupNames := make(map[string]struct{})

	for _, service := range services {
		taskDef, err := c.DescribeTaskDefinition(ctx, *service.TaskDefinition)
		if err != nil {
			return nil, err
		}

		for _, container := range taskDef.TaskDefinition.ContainerDefinitions {
			if container.LogConfiguration == nil {
				continue
			}

			if container.LogConfiguration.LogDriver != ecstypes.LogDriverAwslogs {
				continue
			}

			for k, v := range container.LogConfiguration.Options {
				if k != "awslogs-group" {
					continue
				}

				logGroupNames[v] = struct{}{}
			}
		}
	}

	accountID, err := c.getAccountID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get account ID: %w", err)
	}

	logGroupARNs := make([]string, 0, len(logGroupNames))

	for logGroupName := range logGroupNames {
		c.logger.Debug("Found log group", "logGroupName", logGroupName)

		logGroupARN := fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", c.region, accountID, logGroupName)
		logGroupARNs = append(logGroupARNs, logGroupARN)
	}

	return logGroupARNs, nil
}

func (c *Cluster) getUserID(ctx context.Context) (string, error) {
	if c.userID != "" {
		return c.userID, nil
	}

	if err := c.getCallerIdentity(ctx); err != nil {
		return "", err
	}

	return c.userID, nil
}

func (c *Cluster) getAccountID(ctx context.Context) (string, error) {
	if c.accountID != "" {
		return c.accountID, nil
	}

	if err := c.getCallerIdentity(ctx); err != nil {
		return "", err
	}

	return c.accountID, nil
}

func (c *Cluster) getCallerIdentity(ctx context.Context) error {
	c.logger.Debug("Getting caller identity")

	res, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("get caller identity: %w", err)
	}

	c.userID = *res.UserId
	c.accountID = *res.Account

	return nil
}

func (c *Cluster) getContainerRuntimeID(ctx context.Context, taskARN arn.ARN, containerName string) (string, error) {
	c.logger.Debug("Describing task", "taskARN", taskARN.String())

	describe, err := c.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(c.clusterName),
		Tasks:   []string{taskARN.String()},
	})
	if err != nil {
		return "", fmt.Errorf("describing tasks: %w", err)
	}

	if len(describe.Tasks) == 0 {
		return "", ErrTaskNotFound
	}

	for _, t := range describe.Tasks {
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

func (c *Cluster) describeScalableTargets(ctx context.Context, serviceName string) ([]autoscalingtypes.ScalableTarget, error) {
	resourceID := fmt.Sprintf("service/%s/%s", c.clusterName, serviceName)

	c.logger.Debug("Describing scalable targets", "service", serviceName, "cluster", c.clusterName)

	res, err := c.autoscalingClient.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace:  autoscalingtypes.ServiceNamespaceEcs,
		ScalableDimension: autoscalingtypes.ScalableDimensionECSServiceDesiredCount,
		ResourceIds:       []string{resourceID},
	})
	if err != nil {
		return nil, fmt.Errorf("describing scalable targets: %w", err)
	}

	return res.ScalableTargets, nil
}

func (c *Cluster) setDesiredCount(ctx context.Context, serviceName string, desiredCount int32) error {
	c.logger.Debug("Setting desired count", "service", serviceName, "desiredCount", desiredCount)

	if _, err := c.ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &c.clusterName,
		Service:      &serviceName,
		DesiredCount: aws.Int32(desiredCount),
	}); err != nil {
		return fmt.Errorf("updating desired count: %w", err)
	}

	return nil
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
