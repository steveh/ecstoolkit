// Package executor implements ECS task execution and session management.
package executor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/google/uuid"
	"github.com/steveh/ecstoolkit/communicator"
	"github.com/steveh/ecstoolkit/config"
	"github.com/steveh/ecstoolkit/datachannel"
	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/session"
	"github.com/steveh/ecstoolkit/session/portsession"
	"github.com/steveh/ecstoolkit/session/shellsession"
)

var (
	// ErrInvalidARN indicates that the provided ARN is invalid.
	ErrInvalidARN = errors.New("invalid ARN")

	// ErrInvalidSessionType indicates that the provided session type is invalid.
	ErrInvalidSessionType = errors.New("invalid session type")
)

// PortForwardSessionOptions contains the configuration options for port forwarding sessions.
type PortForwardSessionOptions struct {
	ClusterName        string
	TaskARN            string
	ContainerRuntimeID string
	PortNumber         int
	LocalPortNumber    int
}

// ShellSessionOptions contains the configuration options for executing a shell session.
type ShellSessionOptions struct {
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

// PortForwardSession starts a port forwarding session with the provided options.
func (e *Executor) PortForwardSession(ctx context.Context, options *PortForwardSessionOptions) error {
	targetID, err := buildTargetID(options.ClusterName, options.TaskARN, options.ContainerRuntimeID)
	if err != nil {
		return err
	}

	params := map[string][]string{
		"portNumber":      {strconv.Itoa(options.PortNumber)},
		"localPortNumber": {strconv.Itoa(options.LocalPortNumber)},
	}

	ss, err := e.ssmClient.StartSession(ctx, &ssm.StartSessionInput{
		Target:       aws.String(targetID),
		DocumentName: aws.String("AWS-StartPortForwardingSession"),
		Parameters:   params,
	})
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}

	sess, err := e.newSession(targetID, *ss.SessionId, *ss.StreamUrl, *ss.TokenValue)
	if err != nil {
		return err
	}

	if err := e.initSession(ctx, sess); err != nil {
		return err
	}

	return nil
}

// ShellSession starts a shell session with the provided options.
func (e *Executor) ShellSession(ctx context.Context, options *ShellSessionOptions) error {
	targetID, err := buildTargetID(options.ClusterName, options.TaskARN, options.ContainerRuntimeID)
	if err != nil {
		return err
	}

	ex, err := e.ecsClient.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(options.ClusterName),
		Task:        aws.String(options.TaskARN),
		Container:   aws.String(options.ContainerName),
		Command:     aws.String(options.Command),
		Interactive: true,
	})
	if err != nil {
		return fmt.Errorf("execute command: %w", err)
	}

	sess, err := e.newSession(targetID, *ex.Session.SessionId, *ex.Session.StreamUrl, *ex.Session.TokenValue)
	if err != nil {
		return err
	}

	if err := e.initSession(ctx, sess); err != nil {
		return err
	}

	return nil
}

func (e *Executor) newSession(targetID, sessionID, streamURL, tokenValue string) (*session.Session, error) {
	clientID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate UUID: %w", err)
	}

	wsChannel, err := communicator.NewWebSocketChannel(streamURL, tokenValue, e.logger)
	if err != nil {
		return nil, fmt.Errorf("creating websocket channel: %w", err)
	}

	encryptorBuilder, err := datachannel.NewKMSEncryptorBuilder(
		e.kmsClient,
		e.logger,
	)
	if err != nil {
		return nil, fmt.Errorf("creating encryptor builder: %w", err)
	}

	dataChannel, err := datachannel.NewDataChannel(
		wsChannel,
		encryptorBuilder,
		clientID.String(),
		sessionID,
		targetID,
		e.logger,
	)
	if err != nil {
		return nil, fmt.Errorf("creating data channel: %w", err)
	}

	sess, err := session.NewSession(e.ssmClient, dataChannel, sessionID, e.logger)
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}

	return sess, nil
}

func (e *Executor) initSession(ctx context.Context, sess *session.Session) error {
	e.logger.Debug("Opening data channel")

	sessionType, err := sess.OpenDataChannel(ctx)
	if err != nil {
		return fmt.Errorf("opening data channel: %w", err)
	}

	var sessionSubType session.ISessionPlugin

	e.logger.Debug("Initializing session", "sessionType", sessionType)

	switch sessionType {
	case config.ShellPluginName:
		sessionSubType, err = shellsession.NewShellSession(e.logger, sess)
	case config.PortPluginName:
		sessionSubType, err = portsession.NewPortSession(e.logger, sess)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidSessionType, sessionType)
	}

	if err != nil {
		return fmt.Errorf("creating session subtype: %w", err)
	}

	if err := sessionSubType.SetSessionHandlers(ctx); err != nil {
		if !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("ending with error: %w", err)
		}
	}

	return nil
}

func buildTargetID(clusterName, taskARN, containerRuntimeID string) (string, error) {
	parsedARN, err := arn.Parse(taskARN)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidARN, err)
	}

	// if we could guarantee the task ARN was in the newer long format we could extract the cluster name from there
	taskResourceParts := strings.Split(parsedARN.Resource, "/")
	if len(taskResourceParts) < 3 { //nolint:mnd
		return "", fmt.Errorf("%w: invalid resource ID: %s", ErrInvalidARN, parsedARN.Resource)
	}

	taskID := taskResourceParts[2]

	return fmt.Sprintf("ecs:%s_%s_%s", clusterName, taskID, containerRuntimeID), nil
}
