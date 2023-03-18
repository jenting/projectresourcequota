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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// log is for logging in this package.
// var resourceQuotaLog = logf.Log.WithName("resourcequota-resource")

func SetupResourceQuotaWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.ResourceQuota{}).
		WithDefaulter(&resourceQuotaAnnotator{mgr.GetClient()}).
		WithValidator(&resourceQuotaValidator{mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate--v1-resourcequota,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=resourcequotas,verbs=create;update,versions=v1,name=resourcequota.jenting.io,admissionReviewVersions=v1

// resourceQuotaAnnotator annotates ResourceQuotas
type resourceQuotaAnnotator struct {
	client.Client
}

func (a *resourceQuotaAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	rq, ok := obj.(*corev1.ResourceQuota)
	if !ok {
		return fmt.Errorf("expected a ResourceQuota but got a %T", obj)
	}

	// check whether one of the projectresourcequotas.jenting.io CR spec.hard.resourcequotas is set
	prqList := &ProjectResourceQuotaList{}
	if err := a.Client.List(ctx, prqList); err != nil {
		return err
	}

	for _, prq := range prqList.Items {
		for _, ns := range prq.Spec.Namespaces {
			if ns == rq.Namespace {
				_, ok := prq.Spec.Hard[corev1.ResourceQuotas]
				if !ok {
					return nil
				}

				if rq.Annotations == nil {
					rq.Annotations = map[string]string{}
				}
				rq.Annotations[ProjectNamespaceLabel] = ns
				rq.Annotations[ProjectResourceQuotaLabel] = prq.Name
				log.Info("Annotated ResourceQuota")
				return nil
			}
		}
	}

	return nil
}

//+kubebuilder:webhook:path=/validate--v1-resourcequota,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=resourcequotas,verbs=create;update,versions=v1,name=resourcequota.jenting.io,admissionReviewVersions=v1

// resourceQuotaValidator validates ResourceQuotas
type resourceQuotaValidator struct {
	client.Client
}

// validate admits a resourcequota if a specific annotation exists.
func (v *resourceQuotaValidator) validate(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	resourceQuota, ok := obj.(*corev1.ResourceQuota)
	if !ok {
		return fmt.Errorf("expected a ResourceQuota but got a %T", obj)
	}

	log.Info("Validating ResourceQuota")
	prqName, found := resourceQuota.Annotations[ProjectResourceQuotaLabel]
	if !found {
		return fmt.Errorf("missing annotation %s", ProjectResourceQuotaLabel)
	}

	// get the current projectresourcequotas.jenting.io CR
	prq := &ProjectResourceQuota{}
	if err := v.Client.Get(ctx, types.NamespacedName{Name: prqName}, prq); err != nil {
		return err
	}

	// check the status.used.resourcequotas is less than spec.hard.resourcequotas
	hard := prq.Spec.Hard[corev1.ResourceQuotas]
	used := prq.Status.Used[corev1.ResourceQuotas]

	if hard.Cmp(prq.Status.Used[corev1.ResourceQuotas]) != 1 {
		return fmt.Errorf("over project resource quota. current %s counts %s, hard limit count %s", corev1.ResourceQuotas, hard.String(), used.String())
	}
	return nil
}

func (v *resourceQuotaValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

func (v *resourceQuotaValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj)
}

func (v *resourceQuotaValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}