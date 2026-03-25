package applyconfiguration

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1meta "k8s.io/client-go/applyconfigurations/meta/v1"

	"ocm.software/open-component-model/kubernetes/controller/api/v1alpha1"
	applyv1alpha1 "ocm.software/open-component-model/kubernetes/controller/api/v1alpha1/applyconfiguration/api/v1alpha1"
)

func ConditionsToApplyConfig(conditions ...metav1.Condition) []*v1meta.ConditionApplyConfiguration {
	result := make([]*v1meta.ConditionApplyConfiguration, len(conditions))
	for i, c := range conditions {
		result[i] = v1meta.Condition().
			WithType(c.Type).
			WithStatus(c.Status).
			WithObservedGeneration(c.ObservedGeneration).
			WithLastTransitionTime(c.LastTransitionTime).
			WithReason(c.Reason).
			WithMessage(c.Message)
	}
	return result
}

func OCMConfigToApplyConfig(configs ...v1alpha1.OCMConfiguration) []*applyv1alpha1.OCMConfigurationApplyConfiguration {
	result := make([]*applyv1alpha1.OCMConfigurationApplyConfiguration, len(configs))
	for i, c := range configs {
		result[i] = applyv1alpha1.OCMConfiguration().
			WithAPIVersion(c.APIVersion).
			WithKind(c.Kind).
			WithName(c.Name).
			WithNamespace(c.Namespace).
			WithPolicy(c.Policy)
	}
	return result
}

func DeployedToApplyConfig(deployed ...v1alpha1.DeployedObjectReference) []*applyv1alpha1.DeployedObjectReferenceApplyConfiguration {
	result := make([]*applyv1alpha1.DeployedObjectReferenceApplyConfiguration, len(deployed))
	for i, d := range deployed {
		result[i] = applyv1alpha1.DeployedObjectReference().
			WithAPIVersion(d.APIVersion).
			WithKind(d.Kind).
			WithName(d.Name).
			WithNamespace(d.Namespace)
	}
	return result
}
