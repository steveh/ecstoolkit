package cluster

import (
	"context"
	"fmt"

	"github.com/steveh/ecstoolkit/util"
	"golang.org/x/sync/errgroup"
)

// ScaleOptions contains options for scaling a service.
type ScaleOptions struct {
	Min  bool
	Max  bool
	Size int
}

// Scale scales a service to the specified size.
// If min or max is specified, it will scale to the minimum or maximum capacity of the service.
func (c *Cluster) Scale(ctx context.Context, serviceName string, options *ScaleOptions) error {
	// if min or max is specified, fetch the capacity from the scalable target
	if options.Min || options.Max {
		return c.scaleMinMax(ctx, serviceName, options.Min, options.Max)
	}

	desiredCount, err := util.SafeInt32(options.Size)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOption, err)
	}

	return c.setDesiredCount(ctx, serviceName, desiredCount)
}

func (c *Cluster) scaleMinMax(ctx context.Context, serviceName string, isMin, isMax bool) error {
	eg, ctx := errgroup.WithContext(ctx)

	scaleTargets, err := c.describeScalableTargets(ctx, serviceName)
	if err != nil {
		return err
	}

	for _, scaleTarget := range scaleTargets {
		var desiredCount int32

		switch {
		case isMin:
			if scaleTarget.MinCapacity == nil {
				return fmt.Errorf("%w: tried to set service %s to min capacity, but min capacity is undefined", ErrInvalidOption, serviceName)
			}

			desiredCount = *scaleTarget.MinCapacity
		case isMax:
			if scaleTarget.MaxCapacity == nil {
				return fmt.Errorf("%w: tried to set service %s to max capacity, but max capacity is undefined", ErrInvalidOption, serviceName)
			}

			desiredCount = *scaleTarget.MaxCapacity
		}

		eg.Go(func() error {
			return c.setDesiredCount(ctx, serviceName, desiredCount)
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("scaling service %s: %w", serviceName, err)
	}

	return nil
}
