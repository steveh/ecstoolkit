// Package executor implements ECS task execution and session management.
package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/portsession"
	"github.com/steveh/ecstoolkit/session/shellsession"
)

// ExecuteSessionOptions contains the configuration options for executing a session.
type ExecuteSessionOptions struct {
	ClusterName        string
	TaskARN            string
	ContainerName      string
	ContainerRuntimeID string
	Command            string
}

// Executor manages ECS task execution and session management.
type Executor struct {
	ecsClient *ecs.Client
	kmsClient *kms.Client
	ssmClient *ssm.Client
	logger    log.T
}

// NewExecutor creates a new Executor instance with the provided AWS clients and logger.
func NewExecutor(ecsClient *ecs.Client, kmsClient *kms.Client, ssmClient *ssm.Client, logger log.T) *Executor {
	return &Executor{
		ecsClient: ecsClient,
		kmsClient: kmsClient,
		ssmClient: ssmClient,
		logger:    logger,
	}
}

// NewFromConfig creates a new Executor instance using the provided AWS configuration and logger.
func NewFromConfig(cfg aws.Config, logger log.T) *Executor {
	ecsClient := ecs.NewFromConfig(cfg)
	kmsClient := kms.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	return NewExecutor(ecsClient, kmsClient, ssmClient, logger)
}

// ExecuteSession executes a session with the provided options.
func (e *Executor) ExecuteSession(ctx context.Context, options *ExecuteSessionOptions) error {
	execute, err := e.executeCommand(ctx, options)
	if err != nil {
		return err
	}

	sess, err := e.newSession(options, execute)
	if err != nil {
		return err
	}

	if err := e.initSession(ctx, sess); err != nil {
		return err
	}

	return nil
}

func (e *Executor) parseTaskID(taskARN string) (string, error) {
	parsedARN, err := arn.Parse(taskARN)
	if err != nil {
		return "", fmt.Errorf("invalid ARN: %w", err)
	}

	// if we could guarantee the task ARN was in the newer long format we could extract the cluster name from there
	taskResourceParts := strings.Split(parsedARN.Resource, "/")
	if len(taskResourceParts) < 3 { //nolint:mnd
		return "", fmt.Errorf("invalid resource ID: %s", parsedARN.Resource)
	}

	taskID := taskResourceParts[2]

	return taskID, nil
}

func (e *Executor) executeCommand(ctx context.Context, options *ExecuteSessionOptions) (*ecs.ExecuteCommandOutput, error) {
	execute, err := e.ecsClient.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(options.ClusterName),
		Task:        aws.String(options.TaskARN),
		Container:   aws.String(options.ContainerName),
		Command:     aws.String(options.Command),
		Interactive: true,
	})
	if err != nil {
		return nil, fmt.Errorf("execute command: %w", err)
	}

	return execute, nil
}

func (e *Executor) newSession(options *ExecuteSessionOptions, execute *ecs.ExecuteCommandOutput) (*session.Session, error) {
	taskID, err := e.parseTaskID(options.TaskARN)
	if err != nil {
		return nil, err
	}

	clientID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate UUID: %w", err)
	}

	targetID := fmt.Sprintf("ecs:%s_%s_%s", options.ClusterName, taskID, options.ContainerRuntimeID)

	dataChannel, err := datachannel.NewDataChannel(
		e.kmsClient,
		clientID.String(),
		*execute.Session.SessionId,
		targetID,
		e.logger,
	)
	if err != nil {
		return nil, fmt.Errorf("creating data channel: %w", err)
	}

	sess, err := session.NewSession(e.ssmClient, dataChannel, execute.Session, e.logger)
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}

	return sess, nil
}

func (e *Executor) initSession(ctx context.Context, sess *session.Session) error {
	if err := sess.OpenDataChannel(ctx); err != nil {
		return fmt.Errorf("opening data channel: %w", err)
	}

	sessionType, err := sess.EstablishSessionType(ctx, config.ShellPluginName, config.ResendSleepInterval)
	if err != nil {
		return fmt.Errorf("establishing session type: %w", err)
	}

	var sessionSubType session.ISessionPlugin

	switch sessionType {
	case config.ShellPluginName:
		sessionSubType, err = shellsession.NewShellSession(ctx, e.logger, sess)
	case config.PortPluginName:
		sessionSubType, err = portsession.NewPortSession(ctx, e.logger, sess)
	default:
		return fmt.Errorf("unsupported session type: %s", sessionType)
	}

	if err != nil {
		return fmt.Errorf("creating session subtype: %w", err)
	}

	if err := sessionSubType.SetSessionHandlers(ctx); err != nil {
		return fmt.Errorf("ending with error: %w", err)
	}

	return nil
}
