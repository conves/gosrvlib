package sqs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/nexmoinc/gosrvlib/pkg/awsopt"
)

const (
	// DefaultWaitTimeSeconds is the default duration (in seconds) for which the call waits for a message to arrive in the queue before returning.
	// This must be between 0 and 20 seconds.
	DefaultWaitTimeSeconds = 20

	// DefaultVisibilityTimeout is the default duration (in seconds) that the received messages are hidden from subsequent retrieve requests after being retrieved by a ReceiveMessage request.
	DefaultVisibilityTimeout = 600
)

type cfg struct {
	awsOpts           awsopt.Options
	awsConfig         aws.Config
	waitTimeSeconds   int32
	visibilityTimeout int32
}

func loadConfig(ctx context.Context, opts ...Option) (*cfg, error) {
	c := &cfg{
		waitTimeSeconds:   DefaultWaitTimeSeconds,
		visibilityTimeout: DefaultVisibilityTimeout,
	}

	for _, apply := range opts {
		apply(c)
	}

	if c.waitTimeSeconds < 0 || c.waitTimeSeconds > 20 {
		return nil, fmt.Errorf("waitTimeSeconds must be between 0 and 20 seconds")
	}

	if c.visibilityTimeout < 0 || c.visibilityTimeout > 43200 {
		return nil, fmt.Errorf("visibilityTimeout must be between 0 and 43200 seconds")
	}

	awsConfig, err := c.awsOpts.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	c.awsConfig = awsConfig

	return c, nil
}
