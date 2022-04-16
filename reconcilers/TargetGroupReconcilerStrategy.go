package reconcilers

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbTypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"naffets.eu/ecs-deployment-manager/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"
)

const targetGroupFinalizerName = "naffets.eu/TargetGroupFinalizer"

func NewTargetGroupReconcilerClient(client client.Client, targetGroup *v1alpha1.TargetGroup) ReconcilerClient {
	return ReconcilerClient{
		Client: client,
		Strategy: &TargetGroupReconcilerStrategy{
			Client:      client,
			TargetGroup: targetGroup,
		},
	}
}

type TargetGroupReconcilerStrategy struct {
	client.Client
	TargetGroup *v1alpha1.TargetGroup
}

func (t TargetGroupReconcilerStrategy) GetResourceName() string {
	return t.TargetGroup.Name
}

func (t TargetGroupReconcilerStrategy) IsDeletionPending() bool {
	return !t.TargetGroup.ObjectMeta.DeletionTimestamp.IsZero()
}

func (t TargetGroupReconcilerStrategy) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(t.TargetGroup, targetGroupFinalizerName)
}

func (t TargetGroupReconcilerStrategy) AddFinalizer(ctx context.Context) (ctrl.Result, error) {
	controllerutil.AddFinalizer(t.TargetGroup, targetGroupFinalizerName)
	if err := t.Update(ctx, t.TargetGroup); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	} else {
		return ctrl.Result{}, nil
	}
}

func (t TargetGroupReconcilerStrategy) ExecuteReconcilation(ctx context.Context) (ctrl.Result, error) {
	if !t.TargetGroup.Status.Synced {
		config := getConfigAWS(ctx, t, t.TargetGroup.Namespace)

		if targetGroupArn, err := t.createTargetGroup(ctx, config, t.TargetGroup); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		} else {
			t.TargetGroup.Status.Synced = true
			t.TargetGroup.Status.TargetGroupArn = targetGroupArn
		}

		if listenerRuleArn, err := t.createListenerRule(ctx, config, t.TargetGroup); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		} else {
			t.TargetGroup.Status.Synced = true
			t.TargetGroup.Status.ListenerRuleArn = listenerRuleArn
		}

		if err := t.Status().Update(ctx, t.TargetGroup); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}
	return ctrl.Result{}, nil
}

func (t TargetGroupReconcilerStrategy) ExecuteFinalizer(ctx context.Context) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(t.TargetGroup, targetGroupFinalizerName) {
		config := getConfigAWS(ctx, t, t.TargetGroup.Namespace)

		if err := t.deleteListenerRule(ctx, config, t.TargetGroup.Status.ListenerRuleArn); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		if err := t.deleteTargetGroup(ctx, config, t.TargetGroup.Status.TargetGroupArn); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		controllerutil.RemoveFinalizer(t.TargetGroup, targetGroupFinalizerName)
		if err := t.Update(ctx, t.TargetGroup); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}
	return ctrl.Result{}, nil
}

func (t *TargetGroupReconcilerStrategy) createTargetGroup(ctx context.Context, config *aws.Config, targetGroup *v1alpha1.TargetGroup) (string, error) {
	elbClient := elb.NewFromConfig(*config)

	ecsConfig, err := getConfigECS(ctx, t, targetGroup.Namespace)
	if err != nil {
		return "", err
	}

	protocolVersion := strings.ToUpper(targetGroup.Spec.ProtocolVersion)

	vpcId := targetGroup.Spec.VpcId
	if vpcId == "" {
		vpcId = ecsConfig.Spec.VpcId
	}

	awsTargetGroup, err := elbClient.CreateTargetGroup(ctx, &elb.CreateTargetGroupInput{
		Name:                       &targetGroup.Name,
		VpcId:                      &vpcId,
		Port:                       &targetGroup.Spec.Port,
		Protocol:                   elbTypes.ProtocolEnum(strings.ToUpper(targetGroup.Spec.Protocol)),
		ProtocolVersion:            &protocolVersion,
		TargetType:                 elbTypes.TargetTypeEnum(strings.ToLower(targetGroup.Spec.TargetType)),
		HealthCheckEnabled:         &targetGroup.Spec.HealthCheck.Enabled,
		HealthCheckPath:            &targetGroup.Spec.HealthCheck.Path,
		HealthCheckProtocol:        elbTypes.ProtocolEnum(strings.ToUpper(targetGroup.Spec.HealthCheck.Protocol)),
		HealthCheckIntervalSeconds: &targetGroup.Spec.HealthCheck.Interval,
		HealthCheckTimeoutSeconds:  &targetGroup.Spec.HealthCheck.Timeout,
		HealthyThresholdCount:      &targetGroup.Spec.HealthCheck.HealthyThreshold,
		UnhealthyThresholdCount:    &targetGroup.Spec.HealthCheck.UnhealthyThreshold,
	})

	if err != nil {
		return "", err
	}

	return aws.ToString(awsTargetGroup.TargetGroups[0].TargetGroupArn), nil
}

func (t *TargetGroupReconcilerStrategy) deleteTargetGroup(ctx context.Context, config *aws.Config, targetGroupArn string) error {
	elbClient := elb.NewFromConfig(*config)

	if _, err := elbClient.DeleteTargetGroup(ctx, &elb.DeleteTargetGroupInput{TargetGroupArn: &targetGroupArn}); err != nil {
		return err
	}

	return nil
}

func (t *TargetGroupReconcilerStrategy) createListenerRule(ctx context.Context, config *aws.Config, targetGroup *v1alpha1.TargetGroup) (string, error) {
	elbClient := elb.NewFromConfig(*config)

	loadBalancerArn, err := t.getLoadBalancerArn(*config, targetGroup.Spec.LoadBalancer.Name)
	if err != nil {
		return "", err
	}

	elbListenerList, err := elbClient.DescribeListeners(ctx, &elb.DescribeListenersInput{
		ListenerArns:    nil,
		LoadBalancerArn: &loadBalancerArn,
		Marker:          nil,
		PageSize:        nil,
	})
	if err != nil {
		return "", err
	}

	field := "path-pattern"
	pathPatterns := []string{targetGroup.Spec.LoadBalancer.PathPattern}
	pathPatternConfig := elbTypes.PathPatternConditionConfig{Values: pathPatterns}

	conditions := []elbTypes.RuleCondition{
		{
			Field:             &field,
			PathPatternConfig: &pathPatternConfig,
		},
	}

	actions := []elbTypes.Action{
		{
			Type:           elbTypes.ActionTypeEnumForward,
			TargetGroupArn: &targetGroup.Status.TargetGroupArn,
		},
	}

	var ruleArns = make([]string, 0)
	priority := int32(1)
	for _, elbListenerDescription := range elbListenerList.Listeners {
		rule, err := elbClient.CreateRule(context.Background(), &elb.CreateRuleInput{
			Actions:     actions,
			Conditions:  conditions,
			ListenerArn: elbListenerDescription.ListenerArn,
			Priority:    &priority,
			Tags:        nil,
		})
		if err != nil {
			return "", err
		}
		ruleArns = append(ruleArns, aws.ToString(rule.Rules[0].RuleArn))
	}

	return ruleArns[0], nil
}

func (t *TargetGroupReconcilerStrategy) deleteListenerRule(ctx context.Context, config *aws.Config, listenerRuleArn string) error {
	elbClient := elb.NewFromConfig(*config)

	if _, err := elbClient.DeleteRule(ctx, &elb.DeleteRuleInput{RuleArn: &listenerRuleArn}); err != nil {
		return err
	}

	return nil
}

func (t *TargetGroupReconcilerStrategy) getLoadBalancerArn(config aws.Config, loadBalancerName string) (string, error) {
	elbClient := elb.NewFromConfig(config)

	if elbList, err := elbClient.DescribeLoadBalancers(context.TODO(), &elb.DescribeLoadBalancersInput{}); err != nil {
		return "", err
	} else {
		for range elbList.LoadBalancers {
			loadBalancerMap := map[string]string{}
			for _, v := range elbList.LoadBalancers {
				loadBalancerMap[aws.ToString(v.LoadBalancerName)] = aws.ToString(v.LoadBalancerArn)
			}

			if v, ok := loadBalancerMap[loadBalancerName]; ok {
				return v, nil
			}
		}

		return "", errors.New("could not find a loadbalancer with name \"" + loadBalancerName + "\"")
	}
}
