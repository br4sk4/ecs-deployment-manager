package reconcilers

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsCredentials "github.com/aws/aws-sdk-go-v2/credentials"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ecsdeploymentmanagerv1alpha1 "naffets.eu/ecs-deployment-manager/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type ReconcilerStrategy interface {
	GetResourceName() string
	IsDeletionPending() bool
	ContainsFinalizer() bool
	AddFinalizer(ctx context.Context) (ctrl.Result, error)
	ExecuteReconcilation(ctx context.Context) (ctrl.Result, error)
	ExecuteFinalizer(ctx context.Context) (ctrl.Result, error)
}

type ReconcilerClient struct {
	Client   client.Client
	Strategy ReconcilerStrategy
}

func (c *ReconcilerClient) Reconcile(ctx context.Context) ctrl.Result {
	if c.Strategy.GetResourceName() != "" {
		if !c.Strategy.IsDeletionPending() {
			if !c.Strategy.ContainsFinalizer() {
				if result, err := c.addFinalizer(ctx); err != nil {
					return result
				}
			} else {
				if result, err := c.executeReconcilation(ctx); err != nil {
					return result
				}
			}
		} else {
			if result, err := c.executeFinalization(ctx); err != nil {
				return result
			}
		}
	}
	return ctrl.Result{}
}

func (c *ReconcilerClient) addFinalizer(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	result, err := c.Strategy.AddFinalizer(ctx)
	if err != nil {
		logger.Error(err, err.Error())
		return result, err
	} else {
		return ctrl.Result{}, nil
	}
}

func (c *ReconcilerClient) executeReconcilation(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	result, err := c.Strategy.ExecuteReconcilation(ctx)
	if err != nil {
		logger.Error(err, err.Error())
		return result, err
	} else {
		return ctrl.Result{}, nil
	}
}

func (c *ReconcilerClient) executeFinalization(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	result, err := c.Strategy.ExecuteFinalizer(ctx)
	if err != nil {
		logger.Error(err, err.Error())
		return result, err
	} else {
		return ctrl.Result{}, nil
	}
}

func getConfigECS(ctx context.Context, client client.Client, namespace string) (*ecsdeploymentmanagerv1alpha1.ECSConfig, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      "ecsconfig-defaults",
	}
	var ecsConfig ecsdeploymentmanagerv1alpha1.ECSConfig

	err := client.Get(ctx, key, &ecsConfig)
	return &ecsConfig, err
}

func getConfigAWS(ctx context.Context, client client.Client) *aws.Config {
	logger := log.FromContext(ctx)

	secretName := types.NamespacedName{
		Namespace: "default",
		Name:      "aws-secret",
	}
	var awsSecret v1.Secret
	if err := client.Get(ctx, secretName, &awsSecret); err != nil {
		logger.Error(err, "")
	}

	region := strings.TrimSpace(string(awsSecret.Data["region"]))
	accessKey := strings.TrimSpace(string(awsSecret.Data["accessKey"]))
	secretKey := strings.TrimSpace(string(awsSecret.Data["secretKey"]))

	cfg := &aws.Config{
		Region: region,
		Credentials: awsCredentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"",
		),
	}

	return cfg
}
