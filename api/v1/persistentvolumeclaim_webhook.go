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
// var persistentvolumeclaimlog = logf.Log.WithName("persistentvolumeclaim-resource")

func SetupPersistentVolumeClaimWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		WithDefaulter(&persistentVolumeClaimAnnotator{mgr.GetClient()}).
		WithValidator(&persistentVolumeClaimValidator{mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate--v1-persistentvolumeclaim,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=persistentvolumeclaims,verbs=create;update,versions=v1,name=persistentvolumeclaim.jenting.io,admissionReviewVersions=v1

// persistentVolumeClaimAnnotator annotates PersistentVolumeClaims
type persistentVolumeClaimAnnotator struct {
	client.Client
}

func (a *persistentVolumeClaimAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return fmt.Errorf("expected a PersistentVolumeClaim but got a %T", obj)
	}

	// check whether one of the projectresourcequotas.jenting.io CR spec.hard.persistentvolumeclaims is set
	prqList := &ProjectResourceQuotaList{}
	if err := a.Client.List(ctx, prqList); err != nil {
		return err
	}

	for _, prq := range prqList.Items {
		for _, ns := range prq.Spec.Namespaces {
			if ns == pvc.Namespace {
				_, ok := prq.Spec.Hard[corev1.ResourcePersistentVolumeClaims]
				if !ok {
					return nil
				}

				if pvc.Annotations == nil {
					pvc.Annotations = map[string]string{}
				}
				pvc.Annotations[ProjectNamespaceLabel] = ns
				pvc.Annotations[ProjectResourceQuotaLabel] = prq.Name
				log.Info("Annotated PersistentVolumeClaim")
				return nil
			}
		}
	}

	return nil
}

//+kubebuilder:webhook:path=/validate--v1-persistentvolumeclaim,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=persistentvolumeclaims,verbs=create;update,versions=v1,name=persistentvolumeclaim.jenting.io,admissionReviewVersions=v1

// persistentVolumeClaimValidator validates PersistentVolumeClaims
type persistentVolumeClaimValidator struct {
	client.Client
}

// validate admits a pvc if a specific annotation exists.
func (v *persistentVolumeClaimValidator) validate(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return fmt.Errorf("expected a PersistentVolumeClaim but got a %T", obj)
	}

	log.Info("Validating PersistentVolumeClaim")
	prqName, found := pvc.Annotations[ProjectResourceQuotaLabel]
	if !found {
		return fmt.Errorf("missing annotation %s", ProjectResourceQuotaLabel)
	}

	// get the current projectresourcequotas.jenting.io CR
	prq := &ProjectResourceQuota{}
	if err := v.Client.Get(ctx, types.NamespacedName{Name: prqName}, prq); err != nil {
		return err
	}

	// check the status.used.persistentvolumeclaims is less than spec.hard.persistentvolumeclaims
	hard := prq.Spec.Hard[corev1.ResourcePersistentVolumeClaims]
	used := prq.Status.Used[corev1.ResourcePersistentVolumeClaims]

	if hard.Cmp(prq.Status.Used[corev1.ResourcePersistentVolumeClaims]) != 1 {
		return fmt.Errorf("over project resource quota. current %s counts %s, hard limit count %s", corev1.ResourcePersistentVolumeClaims, hard.String(), used.String())
	}
	return nil
}

func (v *persistentVolumeClaimValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

func (v *persistentVolumeClaimValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj)
}

func (v *persistentVolumeClaimValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}