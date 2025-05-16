package cluster

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
)

// AutoscalingOptions contains options for autoscaling a service.
type AutoscalingOptions struct {
	ScaleIn          *bool
	ScaleOut         *bool
	ScheduledScaling *bool
}

// Autoscale autoscales a service by registering a scalable target with the specified options.
func (c *Cluster) Autoscale(ctx context.Context, serviceName string, options *AutoscalingOptions) error {
	resourceID := fmt.Sprintf("service/%s/%s", c.clusterName, serviceName)

	state := autoscalingtypes.SuspendedState{}

	args := []any{"service", serviceName}

	if options.ScaleIn != nil {
		state.DynamicScalingInSuspended = aws.Bool(!*options.ScaleIn)
		args = append(args, "scale_in", *options.ScaleIn)
	}

	if options.ScaleOut != nil {
		state.DynamicScalingOutSuspended = aws.Bool(!*options.ScaleOut)
		args = append(args, "scale_out", *options.ScaleOut)
	}

	if options.ScheduledScaling != nil {
		state.ScheduledScalingSuspended = aws.Bool(!*options.ScheduledScaling)
		args = append(args, "scheduled", *options.ScheduledScaling)
	}

	c.logger.Debug("Registering scalable target", args...)

	if _, err := c.autoscalingClient.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
		ServiceNamespace:  autoscalingtypes.ServiceNamespaceEcs,
		ScalableDimension: autoscalingtypes.ScalableDimensionECSServiceDesiredCount,
		ResourceId:        &resourceID,
		SuspendedState:    &state,
	}); err != nil {
		return fmt.Errorf("updating autoscaling: %w", err)
	}

	return nil
}
