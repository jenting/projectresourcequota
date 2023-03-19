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

func SetupReplicationControllerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.ReplicationController{}).
		WithDefaulter(&replicationControllerAnnotator{mgr.GetClient()}).
		WithValidator(&replicationControllerValidator{mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate--v1-replicationcontroller,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=replicationcontrollers,verbs=create;update,versions=v1,name=replicationcontroller.jenting.io,admissionReviewVersions=v1

// replicationControllerAnnotator annotates ReplicationControllers
type replicationControllerAnnotator struct {
	client.Client
}

func (a *replicationControllerAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	rc, ok := obj.(*corev1.ReplicationController)
	if !ok {
		return fmt.Errorf("expected a ReplicationController but got a %T", obj)
	}

	// check whether one of the projectresourcequotas.jenting.io CR spec.hard.replicationcontrollers is set
	prqList := &ProjectResourceQuotaList{}
	if err := a.Client.List(ctx, prqList); err != nil {
		return err
	}

	for _, prq := range prqList.Items {
		for _, ns := range prq.Spec.Namespaces {
			if ns == rc.Namespace {
				_, ok := prq.Spec.Hard[corev1.ResourceReplicationControllers]
				if !ok {
					return nil
				}

				if rc.Annotations == nil {
					rc.Annotations = map[string]string{}
				}
				rc.Annotations[ProjectResourceQuotaLabel] = prq.Name
				log.Info("Annotated ReplicationController")
				return nil
			}
		}
	}

	return nil
}

//+kubebuilder:webhook:path=/validate--v1-replicationcontroller,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=replicationcontrollers,verbs=create;update,versions=v1,name=replicationcontroller.jenting.io,admissionReviewVersions=v1

// replicationControllerValidator validates ReplicationControllers
type replicationControllerValidator struct {
	client.Client
}

// validate admits a replicationcontroller if a specific annotation exists.
func (v *replicationControllerValidator) validate(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	rc, ok := obj.(*corev1.ReplicationController)
	if !ok {
		return fmt.Errorf("expected a ReplicationController but got a %T", obj)
	}

	log.Info("Validating ReplicationController")
	prqName, found := rc.Annotations[ProjectResourceQuotaLabel]
	if !found {
		return fmt.Errorf("missing annotation %s", ProjectResourceQuotaLabel)
	}

	// get the current projectresourcequotas.jenting.io CR
	prq := &ProjectResourceQuota{}
	if err := v.Client.Get(ctx, types.NamespacedName{Name: prqName}, prq); err != nil {
		return err
	}

	// check the status.used.replicationcontrollers is less than spec.hard.replicationcontrollers
	hard := prq.Spec.Hard[corev1.ResourceReplicationControllers]
	used := prq.Status.Used[corev1.ResourceReplicationControllers]

	if hard.Cmp(prq.Status.Used[corev1.ResourceReplicationControllers]) != 1 {
		return fmt.Errorf("over project resource quota. current %s counts %s, hard limit count %s", corev1.ResourceReplicationControllers, hard.String(), used.String())
	}
	return nil
}

func (v *replicationControllerValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

func (v *replicationControllerValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj)
}

func (v *replicationControllerValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
