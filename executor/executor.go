package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

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
	"github.com/steveh/ecstoolkit/session/sessionutil"
	"github.com/steveh/ecstoolkit/session/shellsession"
)

func init() {
	session.Register(&portsession.PortSession{})
	session.Register(&shellsession.ShellSession{})
}

type ExecuteSessionOptions struct {
	ClusterName        string
	TaskARN            string
	ContainerName      string
	ContainerRuntimeID string
	Command            string
}

type Executor struct {
	ecsClient *ecs.Client
	kmsClient *kms.Client
	ssmClient *ssm.Client
	logger    log.Slogger
}

func NewExecutor(ecsClient *ecs.Client, kmsClient *kms.Client, ssmClient *ssm.Client, logger *slog.Logger) *Executor {
	return &Executor{
		ecsClient: ecsClient,
		kmsClient: kmsClient,
		ssmClient: ssmClient,
		logger:    log.NewSlogger(logger),
	}
}

func NewFromConfig(cfg aws.Config, logger *slog.Logger) *Executor {
	ecsClient := ecs.NewFromConfig(cfg)
	kmsClient := kms.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	return NewExecutor(ecsClient, kmsClient, ssmClient, logger)
}

func (e *Executor) parseARN(taskARN string) (string, string, error) {
	parsedARN, err := arn.Parse(taskARN)
	if err != nil {
		return "", "", fmt.Errorf("invalid ARN: %w", err)
	}

	// if we could guarantee the task ARN was in the newer long format we could extract the cluster name from there
	taskResourceParts := strings.Split(parsedARN.Resource, "/")
	if len(taskResourceParts) < 3 {
		return "", "", fmt.Errorf("invalid resource ID: %s", parsedARN.Resource)
	}

	taskID := taskResourceParts[2]

	return parsedARN.Region, taskID, nil
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
	region, taskID, err := e.parseARN(options.TaskARN)
	if err != nil {
		return nil, err
	}

	endpoint := url.URL{Scheme: "https", Host: fmt.Sprintf("ssm.%s.amazonaws.com", region)}

	clientID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate UUID: %w", err)
	}

	dc := datachannel.DataChannel{
		KMSClient: e.kmsClient,
	}

	sess := session.Session{
		DataChannel: &dc,
		SessionId:   *execute.Session.SessionId,
		StreamUrl:   *execute.Session.StreamUrl,
		TokenValue:  *execute.Session.TokenValue,
		Endpoint:    endpoint.String(),
		ClientId:    clientID.String(),
		TargetId:    fmt.Sprintf("ecs:%s_%s_%s", options.ClusterName, taskID, options.ContainerRuntimeID),
		DisplayMode: sessionutil.NewDisplayMode(e.logger),
		SSMClient:   e.ssmClient,
	}

	return &sess, nil
}

func (e *Executor) initSession(ctx context.Context, sess *session.Session) error {
	if err := sess.OpenDataChannel(ctx, e.logger); err != nil {
		return fmt.Errorf("open data channel: %w", err)
	}

	sess.DataChannel.SetSessionType(config.ShellPluginName)

	go func() {
		for {
			// Repeat this loop for every 200ms
			time.Sleep(config.ResendSleepInterval)

			if <-sess.DataChannel.IsStreamMessageResendTimeout() {
				e.logger.Errorf("terminating session as the stream data was not processed before timeout: sessionID: %s", sess.SessionId)

				if err := sess.TerminateSession(ctx, e.logger); err != nil {
					e.logger.Errorf("unable to terminate session upon stream data timeout: %v", err)
				}

				return
			}
		}
	}()

	// The session type is set either by handshake or the first packet received.
	if !<-sess.DataChannel.IsSessionTypeSet() {
		return errors.New("unable to determine session type")
	}

	sess.SessionType = sess.DataChannel.GetSessionType()
	sess.SessionProperties = sess.DataChannel.GetSessionProperties()

	sessionSubType := session.SessionRegistry[sess.SessionType]
	if sessionSubType == nil {
		return fmt.Errorf("unknown session type %s", sess.SessionType)
	}

	sessionSubType.Initialize(ctx, e.logger, sess)

	if err := sessionSubType.SetSessionHandlers(ctx, e.logger); err != nil {
		return fmt.Errorf("ending with error: %w", err)
	}

	return nil
}

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
