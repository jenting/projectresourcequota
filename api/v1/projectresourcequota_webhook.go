/*
Copyright 2023 JenTing.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var resourceNameList = []corev1.ResourceName{
	corev1.ResourceCPU,
	corev1.ResourceMemory,
	corev1.ResourceStorage,
	corev1.ResourceEphemeralStorage,
	corev1.ResourcePods,
	corev1.ResourceServices,
	corev1.ResourceReplicationControllers,
	corev1.ResourceQuotas,
	corev1.ResourceSecrets,
	corev1.ResourceConfigMaps,
	corev1.ResourcePersistentVolumeClaims,
	corev1.ResourceServicesNodePorts,
	corev1.ResourceServicesLoadBalancers,
	corev1.ResourceRequestsCPU,
	corev1.ResourceRequestsMemory,
	corev1.ResourceRequestsStorage,
	corev1.ResourceRequestsEphemeralStorage,
	corev1.ResourceLimitsCPU,
	corev1.ResourceLimitsMemory,
	corev1.ResourceLimitsEphemeralStorage,
}

func SetupProjectResourceQuotaWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ProjectResourceQuota{}).
		WithDefaulter(&projectResourceQuotaAnnotator{mgr.GetClient()}).
		WithValidator(&projectResourceQuotaValidator{mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-jenting-io-v1-projectresourcequota,mutating=true,failurePolicy=fail,sideEffects=None,groups=jenting.io,resources=projectresourcequotas,verbs=create;update,versions=v1,name=mprojectresourcequota.kb.io,admissionReviewVersions=v1

// projectResourceQuotaAnnotator annotates ProjectResourceQuotas
type projectResourceQuotaAnnotator struct {
	client.Client
}

func (a *projectResourceQuotaAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-jenting-io-v1-projectresourcequota,mutating=false,failurePolicy=fail,sideEffects=None,groups=jenting.io,resources=projectresourcequotas,verbs=create;update,versions=v1,name=vprojectresourcequota.kb.io,admissionReviewVersions=v1

// projectResourceQuotaValidator validates ProjectResourceQuotas
type projectResourceQuotaValidator struct {
	client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *projectResourceQuotaValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *projectResourceQuotaValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	prq, ok := newObj.(*ProjectResourceQuota)
	if !ok {
		return fmt.Errorf("expected a ProjectResourceQuota but got a %T", newObj)
	}

	for _, resourceName := range resourceNameList {
		hard := prq.Spec.Hard[resourceName]
		used := prq.Status.Used[resourceName]
		if hard.Cmp(used) == -1 {
			return fmt.Errorf("hard limit %s is less than used %s", hard.String(), used.String())
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *projectResourceQuotaValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
