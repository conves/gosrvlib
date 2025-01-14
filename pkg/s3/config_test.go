package s3

import (
	"context"
	"testing"

	"github.com/nexmoinc/gosrvlib/pkg/awsopt"
	"github.com/stretchr/testify/require"
)

// nolint: paralleltest
func Test_loadConfig(t *testing.T) {
	region := "eu-central-1"

	o := awsopt.Options{}
	o.WithRegion(region)
	o.WithEndpoint("https://test.endpoint.invalid", true)

	got, err := loadConfig(
		context.TODO(),
		WithAWSOptions(o),
	)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, region, got.awsConfig.Region)

	// force aws config.LoadDefaultConfig to fail
	t.Setenv("AWS_ENABLE_ENDPOINT_DISCOVERY", "ERROR")

	got, err = loadConfig(context.TODO())

	require.Error(t, err)
	require.Nil(t, got)
}
