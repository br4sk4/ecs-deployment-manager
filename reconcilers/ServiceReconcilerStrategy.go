package reconcilers

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"k8s.io/apimachinery/pkg/types"
	"naffets.eu/ecs-deployment-manager/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const serviceFinalizerName = "naffets.eu/ServiceFinalizer"

func NewServiceReconcilerClient(client client.Client, service *v1alpha1.Service) ReconcilerClient {
	return ReconcilerClient{
		Client: client,
		Strategy: &ServiceReconcilerStrategy{
			Client:  client,
			Service: service,
		},
	}
}

type ServiceReconcilerStrategy struct {
	client.Client
	Service *v1alpha1.Service
}

func (s *ServiceReconcilerStrategy) GetResourceName() string {
	return s.Service.Name
}

func (s *ServiceReconcilerStrategy) IsDeletionPending() bool {
	return !s.Service.ObjectMeta.DeletionTimestamp.IsZero()
}

func (s *ServiceReconcilerStrategy) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(s.Service, serviceFinalizerName)
}

func (s *ServiceReconcilerStrategy) AddFinalizer(ctx context.Context) (ctrl.Result, error) {
	controllerutil.AddFinalizer(s.Service, serviceFinalizerName)
	if err := s.Update(ctx, s.Service); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	} else {
		return ctrl.Result{}, nil
	}
}

func (s *ServiceReconcilerStrategy) ExecuteReconcilation(ctx context.Context) (ctrl.Result, error) {
	if !s.Service.Status.Synced {
		config := getConfigAWS(ctx, s.Client)

		if serviceArn, err := s.createService(ctx, config, s.Service); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		} else {
			s.Service.Status.Synced = true
			s.Service.Status.ServiceArn = serviceArn
		}

		if err := s.Status().Update(ctx, s.Service); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}
	return ctrl.Result{}, nil
}

func (s *ServiceReconcilerStrategy) ExecuteFinalizer(ctx context.Context) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(s.Service, serviceFinalizerName) {
		config := getConfigAWS(ctx, s)

		if err := s.deleteService(ctx, config, s.Service); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		controllerutil.RemoveFinalizer(s.Service, serviceFinalizerName)
		if err := s.Update(ctx, s.Service); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}
	return ctrl.Result{}, nil
}

func (s *ServiceReconcilerStrategy) createService(ctx context.Context, config *aws.Config, service *v1alpha1.Service) (string, error) {
	ecsClient := ecs.NewFromConfig(*config)
	ecsConfig, err := getConfigECS(ctx, s, service.Namespace)
	if err != nil {
		return "", err
	}

	targetGroupArn, err := s.getTargetGroupArn(ctx, types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	})
	if err != nil {
		return "", err
	}

	taskDefinitionArn, err := s.getTaskDefinitionArn(ctx, types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	})
	if err != nil {
		return "", err
	}

	awsService, err := ecsClient.CreateService(context.Background(), &ecs.CreateServiceInput{
		ServiceName:             &service.Name,
		Cluster:                 s.getOrDefaultCluster(service, ecsConfig),
		DeploymentConfiguration: s.getDeploymentConfig(),
		DesiredCount:            s.getDesiredCount(service),
		LaunchType:              service.Spec.LaunchType,
		LoadBalancers:           s.getLoadBalancerConfig(service, targetGroupArn),
		NetworkConfiguration:    s.getNetworkConfig(service, ecsConfig),
		TaskDefinition:          &taskDefinitionArn,
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(awsService.Service.ServiceArn), nil
}

func (s *ServiceReconcilerStrategy) deleteService(ctx context.Context, config *aws.Config, service *v1alpha1.Service) error {
	ecsClient := ecs.NewFromConfig(*config)
	ecsConfig, err := getConfigECS(ctx, s, service.Namespace)
	if err != nil {
		return err
	}

	cluster := s.getOrDefaultCluster(service, ecsConfig)

	s.stopTasks(ctx, ecsClient, service, cluster)

	forceDeletion := true
	if _, err := ecsClient.DeleteService(ctx, &ecs.DeleteServiceInput{Cluster: cluster, Service: &service.Status.ServiceArn, Force: &forceDeletion}); err != nil {
		return err
	}

	return nil
}

func (s *ServiceReconcilerStrategy) getTargetGroupArn(ctx context.Context, targetGroupName types.NamespacedName) (string, error) {
	var targetGroup v1alpha1.TargetGroup
	if err := s.Get(ctx, targetGroupName, &targetGroup); err != nil {
		return "", errors.New("could not determine targetgroup")
	}

	if targetGroup.Status.TargetGroupArn == "" {
		return "", errors.New("could not determine arn of targetgroup")
	}

	return targetGroup.Status.TargetGroupArn, nil
}

func (s *ServiceReconcilerStrategy) getTaskDefinitionArn(ctx context.Context, taskDefinitionName types.NamespacedName) (string, error) {
	var taskDefinition v1alpha1.TaskDefinition
	if err := s.Get(ctx, taskDefinitionName, &taskDefinition); err != nil {
		return "", errors.New("could not determine taskdefinition")
	}

	if taskDefinition.Status.TaskDefinitionArn == "" {
		return "", errors.New("could not determine arn of taskdefinition")
	}

	return taskDefinition.Status.TaskDefinitionArn, nil
}

func (s *ServiceReconcilerStrategy) getOrDefaultCluster(service *v1alpha1.Service, ecsConfig *v1alpha1.ECSConfig) *string {
	cluster := service.Spec.Cluster
	if cluster == "" {
		cluster = ecsConfig.Spec.Cluster
	}
	return &cluster
}

func (s *ServiceReconcilerStrategy) getNetworkConfig(service *v1alpha1.Service, ecsConfig *v1alpha1.ECSConfig) *ecsTypes.NetworkConfiguration {
	subnets := service.Spec.Subnets
	if subnets == nil || len(subnets) == 0 {
		subnets = ecsConfig.Spec.Subnets
	}

	securityGroups := service.Spec.SecurityGroups
	if securityGroups == nil || len(securityGroups) == 0 {
		securityGroups = ecsConfig.Spec.SecurityGroups
	}

	networkConfiguration := &ecsTypes.NetworkConfiguration{
		AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
			Subnets:        subnets,
			SecurityGroups: securityGroups,
		},
	}

	return networkConfiguration
}

func (s *ServiceReconcilerStrategy) getLoadBalancerConfig(service *v1alpha1.Service, targetGroupArn string) []ecsTypes.LoadBalancer {
	containerPort := int32(service.Spec.ContainerPort)

	loadBalancers := []ecsTypes.LoadBalancer{
		{
			ContainerName:  &service.Name,
			ContainerPort:  &containerPort,
			TargetGroupArn: &targetGroupArn,
		},
	}

	return loadBalancers
}

func (s *ServiceReconcilerStrategy) getDesiredCount(service *v1alpha1.Service) *int32 {
	desiredCount := int32(service.Spec.DesiredCount)
	return &desiredCount
}

func (s *ServiceReconcilerStrategy) getDeploymentConfig() *ecsTypes.DeploymentConfiguration {
	maximumPercent := int32(200)
	minimumHealthyPercent := int32(100)
	deploymentConfig := &ecsTypes.DeploymentConfiguration{
		DeploymentCircuitBreaker: &ecsTypes.DeploymentCircuitBreaker{
			Enable:   true,
			Rollback: true,
		},
		MaximumPercent:        &maximumPercent,
		MinimumHealthyPercent: &minimumHealthyPercent,
	}
	return deploymentConfig
}

func (s *ServiceReconcilerStrategy) stopTasks(ctx context.Context, ecsClient *ecs.Client, service *v1alpha1.Service, cluster *string) {
	logger := log.FromContext(ctx)
	taskList, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     cluster,
		ServiceName: &service.Name,
	})
	if err == nil {
		for _, task := range taskList.TaskArns {
			if _, taskStopError := ecsClient.StopTask(ctx, &ecs.StopTaskInput{Task: &task, Cluster: cluster}); taskStopError != nil {
				logger.Error(taskStopError, "could not stop task "+task)
			}
		}
	} else {
		logger.Error(err, "could not determine tasks to stop")
	}
}
