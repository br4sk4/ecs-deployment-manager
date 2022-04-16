package reconcilers

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"naffets.eu/ecs-deployment-manager/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
	"time"
)

const taskDedfinitionFinalizerName = "naffets.eu/TaskDefinitionFinalizer"

func NewTaskDefinitionReconcilerClient(client client.Client, taskDefinition *v1alpha1.TaskDefinition) ReconcilerClient {
	return ReconcilerClient{
		Client: client,
		Strategy: &TaskDefinitionReconcilerStrategy{
			Client:         client,
			TaskDefinition: taskDefinition,
		},
	}
}

type TaskDefinitionReconcilerStrategy struct {
	client.Client
	TaskDefinition *v1alpha1.TaskDefinition
}

func (t TaskDefinitionReconcilerStrategy) GetResourceName() string {
	return t.TaskDefinition.Name
}

func (t TaskDefinitionReconcilerStrategy) IsDeletionPending() bool {
	return !t.TaskDefinition.ObjectMeta.DeletionTimestamp.IsZero()
}

func (t TaskDefinitionReconcilerStrategy) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(t.TaskDefinition, taskDedfinitionFinalizerName)
}

func (t TaskDefinitionReconcilerStrategy) AddFinalizer(ctx context.Context) (ctrl.Result, error) {
	controllerutil.AddFinalizer(t.TaskDefinition, taskDedfinitionFinalizerName)
	if err := t.Update(ctx, t.TaskDefinition); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	} else {
		return ctrl.Result{}, nil
	}
}

func (t TaskDefinitionReconcilerStrategy) ExecuteReconcilation(ctx context.Context) (ctrl.Result, error) {
	config := getConfigAWS(ctx, t, t.TaskDefinition.Namespace)
	if !t.TaskDefinition.Status.Synced {
		if taskDefinitionArn, err := t.createTaskDefinition(ctx, config, t.TaskDefinition); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		} else {
			t.TaskDefinition.Status.Synced = true
			t.TaskDefinition.Status.TaskDefinitionArn = taskDefinitionArn
		}

		if err := t.Status().Update(ctx, t.TaskDefinition); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	} else {
		if taskDefinitionArn, err := t.updateTaskDefinition(ctx, config, t.TaskDefinition); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		} else {
			if taskDefinitionArn != "" && taskDefinitionArn != t.TaskDefinition.Status.TaskDefinitionArn {
				t.TaskDefinition.Status.TaskDefinitionArn = taskDefinitionArn
				if err := t.Status().Update(ctx, t.TaskDefinition); err != nil {
					return ctrl.Result{RequeueAfter: 5 * time.Second}, err
				}
			}

		}
	}
	return ctrl.Result{}, nil
}

func (t TaskDefinitionReconcilerStrategy) ExecuteFinalizer(ctx context.Context) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(t.TaskDefinition, taskDedfinitionFinalizerName) {
		config := getConfigAWS(ctx, t, t.TaskDefinition.Namespace)

		if err := t.deleteTaskDefinition(ctx, config, t.TaskDefinition.Status.TaskDefinitionArn); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		controllerutil.RemoveFinalizer(t.TaskDefinition, taskDedfinitionFinalizerName)
		if err := t.Update(ctx, t.TaskDefinition); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}
	return ctrl.Result{}, nil
}

func (t *TaskDefinitionReconcilerStrategy) createTaskDefinition(ctx context.Context, config *aws.Config, taskDefinition *v1alpha1.TaskDefinition) (string, error) {
	ecsClient := ecs.NewFromConfig(*config)
	ecsConfig, err := getConfigECS(ctx, t, taskDefinition.Namespace)
	if err != nil {
		return "", err
	}

	cpuString := strconv.Itoa(taskDefinition.Spec.Cpu)
	memoryString := strconv.Itoa(taskDefinition.Spec.Memory)

	taskRoleArn := taskDefinition.Spec.TaskRoleArn
	if taskRoleArn == "" {
		taskRoleArn = ecsConfig.Spec.TaskRoleArn
	}

	awsTaskDefinition, err := ecsClient.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:                  &taskDefinition.Name,
		Cpu:                     &cpuString,
		Memory:                  &memoryString,
		RequiresCompatibilities: taskDefinition.Spec.Compatibilities,
		NetworkMode:             taskDefinition.Spec.NetworkMode,
		TaskRoleArn:             &taskRoleArn,
		ExecutionRoleArn:        &taskRoleArn,
		ContainerDefinitions:    t.getConainerDefinitions(taskDefinition, config, ecsConfig),
	})

	if err != nil {
		return "", err
	}

	return aws.ToString(awsTaskDefinition.TaskDefinition.TaskDefinitionArn), nil
}

func (t *TaskDefinitionReconcilerStrategy) updateTaskDefinition(ctx context.Context, config *aws.Config, taskDefinition *v1alpha1.TaskDefinition) (string, error) {
	ecsClient := ecs.NewFromConfig(*config)
	ecsConfig, err := getConfigECS(ctx, t, taskDefinition.Namespace)
	if err != nil {
		return "", err
	}

	awsTaskDefinition, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefinition.Name,
	})
	if err != nil {
		return "", nil
	}

	image := t.getOrDefaultImage(taskDefinition, ecsConfig)

	if *awsTaskDefinition.TaskDefinition.ContainerDefinitions[0].Image != *image {
		err = t.deleteTaskDefinition(ctx, config, taskDefinition.Status.TaskDefinitionArn)
		if err != nil {
			return "", err
		}

		newTaskDefinitionArn, err := t.createTaskDefinition(ctx, config, taskDefinition)
		if err != nil {
			return "", err
		}

		cluster := "naffets"
		_, err = ecsClient.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:            &cluster,
			Service:            &taskDefinition.Name,
			TaskDefinition:     &newTaskDefinitionArn,
			ForceNewDeployment: true,
		})
		if err != nil {
			return "", err
		}

		return aws.ToString(&newTaskDefinitionArn), nil
	}

	return "", nil
}

func (t *TaskDefinitionReconcilerStrategy) deleteTaskDefinition(ctx context.Context, config *aws.Config, taskDefinitionArn string) error {
	ecsClient := ecs.NewFromConfig(*config)

	if _, err := ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{TaskDefinition: &taskDefinitionArn}); err != nil {
		return err
	}

	return nil
}

func (t *TaskDefinitionReconcilerStrategy) getConainerDefinitions(taskDefinition *v1alpha1.TaskDefinition, config *aws.Config, ecsConfig *v1alpha1.ECSConfig) []ecsTypes.ContainerDefinition {
	essential := true
	containerPort := int32(taskDefinition.Spec.ContainerDefinition.ContainerPort)
	hostPort := int32(taskDefinition.Spec.ContainerDefinition.HostPort)

	logOptions := make(map[string]string)
	logOptions["awslogs-group"] = taskDefinition.Name
	logOptions["awslogs-region"] = config.Region
	logOptions["awslogs-stream-prefix"] = taskDefinition.Name
	logConfiguration := ecsTypes.LogConfiguration{
		LogDriver: "awslogs",
		Options:   logOptions,
	}

	return []ecsTypes.ContainerDefinition{
		{
			Name:      &taskDefinition.Name,
			Image:     t.getOrDefaultImage(taskDefinition, ecsConfig),
			Essential: &essential,
			PortMappings: []ecsTypes.PortMapping{
				{
					ContainerPort: &containerPort,
					HostPort:      &hostPort,
				},
			},
			LogConfiguration: &logConfiguration,
		},
	}
}

func (t *TaskDefinitionReconcilerStrategy) getOrDefaultImage(taskDefinition *v1alpha1.TaskDefinition, ecsConfig *v1alpha1.ECSConfig) *string {
	image := taskDefinition.Spec.ContainerDefinition.RegistryUrl
	if image == "" {
		image = ecsConfig.Spec.RegistryUrl
	}
	image = image + "/" + taskDefinition.Spec.ContainerDefinition.Image
	return &image
}
