/*
Copyright 2022.

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

package controllers

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	blobv1 "arcblob.poc/storageaccount/api/v1"
)

// StorageAccountKeyRegeneratorReconciler reconciles a StorageAccountKeyRegenerator object
type StorageAccountKeyRegeneratorReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	SecretClient *coreV1Types.SecretInterface
}

func (r *StorageAccountKeyRegeneratorReconciler) GenerateStorageAccountKey(name string, permission blobv1.StorageAccountPermission) blobv1.StorageAccountKey {
	key := make([]byte, 64)
	rand.Read(key)

	return blobv1.StorageAccountKey{
		KeyName:      name,
		Permissions:  permission,
		Value:        base64.StdEncoding.EncodeToString(key),
		CreationTime: v1.Now(),
	}
}

func (r *StorageAccountKeyRegeneratorReconciler) GenerateRandomKey() string {
	key := make([]byte, 64)
	rand.Read(key)

	return base64.StdEncoding.EncodeToString(key)
}

//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccountkeyregenerators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccountkeyregenerators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccountkeyregenerators/finalizers,verbs=update
//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccounts,verbs=get
//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccounts/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StorageAccountKeyRegenerator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *StorageAccountKeyRegeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var regenerator blobv1.StorageAccountKeyRegenerator
	if err := r.Get(ctx, req.NamespacedName, &regenerator); err != nil {
		log.Error(err, "unable to fetch storage account key regenerator")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if regenerator.Status.Status == "" {
		err := r.RegenerateKey(ctx, &regenerator)
		r.Status().Update(ctx, &regenerator)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *StorageAccountKeyRegeneratorReconciler) RegenerateKey(ctx context.Context, regenerator *blobv1.StorageAccountKeyRegenerator) error {
	log := log.FromContext(ctx)

	account := blobv1.StorageAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: regenerator.Spec.AccountName, Namespace: regenerator.Namespace}, &account)
	if err != nil {
		log.Error(err, "Failed to get specified storage account")
		if errors.IsNotFound(err) {
			regenerator.Status.Status = "Failed"
			return nil
		} else {
			regenerator.Status.Status = "Running"
			return err
		}
	}

	// 2. check if it contains specific key
	keyIdToRegenerate := -1
	for i, key := range account.Status.Keys {
		if key.KeyName == regenerator.Spec.KeyName {
			keyIdToRegenerate = i
			log.Info("Regenerating key for storage account " + account.Name)
			break
		}
	}
	if keyIdToRegenerate < 0 {
		// mark the status to fail and don't requeue
		regenerator.Status.Status = "Failed"
		return nil
	}

	// 3. regenerate the key
	account.Status.Keys[keyIdToRegenerate] = r.GenerateStorageAccountKey(account.Status.Keys[keyIdToRegenerate].KeyName, account.Status.Keys[keyIdToRegenerate].Permissions)

	err = r.Status().Update(ctx, &account)

	if err != nil {
		log.Error(err, "Failed to update storage account")
		regenerator.Status.Status = "Running"
		return err
	} else {
		log.Info("Updated storage account key")
		regenerator.Status.Status = "Done"
		regenerator.Status.Keys = account.Status.Keys
		return nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageAccountKeyRegeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&blobv1.StorageAccountKeyRegenerator{}).
		Complete(r)
}
