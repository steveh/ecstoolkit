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
	"github.com/steveh/ecstoolkit/sessionmanagerplugin/session"
	"github.com/steveh/ecstoolkit/sessionmanagerplugin/session/portsession"
	"github.com/steveh/ecstoolkit/sessionmanagerplugin/session/sessionutil"
	"github.com/steveh/ecstoolkit/sessionmanagerplugin/session/shellsession"
)

func init() {
	session.Register(&portsession.PortSession{})
	session.Register(&shellsession.ShellSession{})
}

type ExecuteSessionOptions struct {
	Cluster            string
	TaskARN            string
	ContainerName      string
	ContainerRuntimeID string
	Command            string
}

// ExecuteSession runs an ECS Exec command.
func ExecuteSession(ctx context.Context, ecsClient *ecs.Client, ssmClient *ssm.Client, kmsClient *kms.Client, logger *slog.Logger, options ExecuteSessionOptions) error {
	parsedARN, err := arn.Parse(options.TaskARN)
	if err != nil {
		return fmt.Errorf("invalid ARN: %w", err)
	}

	// if we could guarantee the task ARN was in the newer long format we could extract the cluster name from there
	taskResourceParts := strings.Split(parsedARN.Resource, "/")
	if len(taskResourceParts) < 3 {
		return fmt.Errorf("invalid resource ID: %s", parsedARN.Resource)
	}

	taskID := taskResourceParts[2]

	execute, err := ecsClient.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(options.Cluster),
		Task:        aws.String(parsedARN.String()),
		Container:   aws.String(options.ContainerName),
		Command:     aws.String(options.Command),
		Interactive: true,
	})
	if err != nil {
		return fmt.Errorf("execute command: %w", err)
	}

	endpoint := url.URL{Scheme: "https", Host: fmt.Sprintf("ssm.%s.amazonaws.com", parsedARN.Region)}

	clientID, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}

	log := log.NewSlogger(logger)

	dc := datachannel.DataChannel{
		KMSClient: kmsClient,
	}

	sess := session.Session{
		DataChannel: &dc,
		SessionId:   *execute.Session.SessionId,
		StreamUrl:   *execute.Session.StreamUrl,
		TokenValue:  *execute.Session.TokenValue,
		Endpoint:    endpoint.String(),
		ClientId:    clientID.String(),
		TargetId:    fmt.Sprintf("ecs:%s_%s_%s", options.Cluster, taskID, options.ContainerRuntimeID),
		DisplayMode: sessionutil.NewDisplayMode(log),
		SSMClient:   ssmClient,
	}

	if err := sess.OpenDataChannel(ctx, log); err != nil {
		return fmt.Errorf("open data channel: %w", err)
	}

	sess.DataChannel.SetSessionType(config.ShellPluginName)

	go func() {
		for {
			// Repeat this loop for every 200ms
			time.Sleep(config.ResendSleepInterval)

			if <-sess.DataChannel.IsStreamMessageResendTimeout() {
				log.Errorf("terminating session as the stream data was not processed before timeout: sessionID: %s", sess.SessionId)

				if err := sess.TerminateSession(ctx, log); err != nil {
					log.Errorf("unable to terminate session upon stream data timeout: %v", err)
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

	sessionSubType.Initialize(ctx, log, &sess)

	if err := sessionSubType.SetSessionHandlers(ctx, log); err != nil {
		return fmt.Errorf("ending with error: %w", err)
	}

	return nil
}
