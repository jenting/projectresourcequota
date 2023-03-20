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

func SetupSecretWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Secret{}).
		WithDefaulter(&secretAnnotator{mgr.GetClient()}).
		WithValidator(&secretValidator{mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate--v1-secret,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=secrets,verbs=create;update,versions=v1,name=secret.jenting.io,admissionReviewVersions=v1

// secretAnnotator annotates Secrets
type secretAnnotator struct {
	client.Client
}

func (a *secretAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected a Secret but got a %T", obj)
	}

	// check whether one of the projectresourcequotas.jenting.io CR spec.hard.secrets is set
	prqList := &ProjectResourceQuotaList{}
	if err := a.Client.List(ctx, prqList); err != nil {
		return err
	}

	for _, prq := range prqList.Items {
		// skip the projectresourcequota CR that is being deleted
		if prq.DeletionTimestamp != nil {
			continue
		}

		for _, ns := range prq.Spec.Namespaces {
			// skip the namespace that is not within the prq.spec.namespaces
			if ns != secret.Namespace {
				continue
			}

			_, ok := prq.Spec.Hard[corev1.ResourceSecrets]
			if !ok {
				return nil
			}

			if secret.Labels == nil {
				secret.Labels = map[string]string{}
			}
			secret.Labels[ProjectResourceQuotaLabel] = prq.Name
			log.Info("Secret Labeled")
			return nil
		}
	}

	return nil
}

//+kubebuilder:webhook:path=/validate--v1-secret,mutating=false,failurePolicy=ignore,sideEffects=None,groups="",matchPolicy=Exact,resources=secrets,verbs=create;update,versions=v1,name=secret.jenting.io,admissionReviewVersions=v1

// secretValidator validates Secrets
type secretValidator struct {
	client.Client
}

func (v *secretValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected a Secret but got a %T", obj)
	}

	log.Info("Validating Secret Creates")
	prqName, found := secret.Labels[ProjectResourceQuotaLabel]
	if !found {
		return nil
	}

	// get the current projectresourcequotas.jenting.io CR
	prq := &ProjectResourceQuota{}
	if err := v.Client.Get(ctx, types.NamespacedName{Name: prqName}, prq); err != nil {
		return err
	}

	// check the status.used.secrets is less than spec.hard.secrets
	hard := prq.Spec.Hard[corev1.ResourceSecrets]
	used := prq.Status.Used[corev1.ResourceSecrets]

	if hard.Cmp(prq.Status.Used[corev1.ResourceSecrets]) != 1 {
		return fmt.Errorf("over project resource quota. current %s counts %s, hard limit count %s", corev1.ResourceSecrets, hard.String(), used.String())
	}
	return nil
}

func (v *secretValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

func (v *secretValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
