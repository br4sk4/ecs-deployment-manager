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

	targetGroupName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	targetGroupArn, err := s.getTargetGroupArn(ctx, targetGroupName)
	if err != nil {
		return "", err
	}

	taskDefinitionName := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	taskDefinitionArn, err := s.getTaskDefinitionArn(ctx, taskDefinitionName)
	if err != nil {
		return "", err
	}

	ecsConfig, err := getConfigECS(ctx, s, service.Namespace)
	if err != nil {
		return "", err
	}

	cluster := service.Spec.Cluster
	if cluster == "" {
		cluster = ecsConfig.Spec.Cluster
	}

	subnets := service.Spec.Subnets
	if subnets == nil || len(subnets) == 0 {
		subnets = ecsConfig.Spec.Subnets
	}

	securityGroups := service.Spec.SecurityGroups
	if securityGroups == nil || len(securityGroups) == 0 {
		securityGroups = ecsConfig.Spec.SecurityGroups
	}

	desiredCount := int32(service.Spec.DesiredCount)
	containerPort := int32(service.Spec.ContainerPort)

	networkConfiguration := &ecsTypes.NetworkConfiguration{
		AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
			Subnets:        subnets,
			SecurityGroups: securityGroups,
		},
	}

	loadBalancers := []ecsTypes.LoadBalancer{
		{
			ContainerName:  &service.Name,
			ContainerPort:  &containerPort,
			TargetGroupArn: &targetGroupArn,
		},
	}

	awsService, err := ecsClient.CreateService(context.Background(), &ecs.CreateServiceInput{
		ServiceName:          &service.Name,
		Cluster:              &cluster,
		LaunchType:           service.Spec.LaunchType,
		TaskDefinition:       &taskDefinitionArn,
		DesiredCount:         &desiredCount,
		NetworkConfiguration: networkConfiguration,
		LoadBalancers:        loadBalancers,
	})

	if err != nil {
		return "", err
	}

	return aws.ToString(awsService.Service.ServiceArn), nil
}

func (s *ServiceReconcilerStrategy) deleteService(ctx context.Context, config *aws.Config, service *v1alpha1.Service) error {
	logger := log.FromContext(ctx)
	ecsClient := ecs.NewFromConfig(*config)

	cluster := service.Spec.Cluster

	ecsConfig, err := getConfigECS(ctx, s, service.Namespace)
	if err == nil {
		cluster = ecsConfig.Spec.Cluster
	}

	taskList, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &cluster,
		ServiceName: &service.Name,
	})
	if err == nil {
		for _, task := range taskList.TaskArns {
			if _, taskStopError := ecsClient.StopTask(ctx, &ecs.StopTaskInput{Task: &task, Cluster: &cluster}); taskStopError != nil {
				logger.Error(taskStopError, "could not stop task "+task)
			}
		}
	} else {
		logger.Error(err, "could not determine tasks to stop")
	}

	forceDeletion := true
	if _, err := ecsClient.DeleteService(ctx, &ecs.DeleteServiceInput{Cluster: &cluster, Service: &service.Status.ServiceArn, Force: &forceDeletion}); err != nil {
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
