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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	blobv1 "arcblob.poc/storageaccount/api/v1"
)

// StorageAccountReconciler reconciles a StorageAccount object
type StorageAccountReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	SecretClient *coreV1Types.SecretInterface
}

func (r *StorageAccountReconciler) GenerateStorageAccountKey(name string, permission blobv1.StorageAccountPermission) blobv1.StorageAccountKey {
	key := make([]byte, 64)
	rand.Read(key)

	return blobv1.StorageAccountKey{
		KeyName:      name,
		Permissions:  permission,
		Value:        base64.StdEncoding.EncodeToString(key),
		CreationTime: v1.Now(),
	}

}

func (r *StorageAccountReconciler) CreateOrUpdateSecretForAccount(ctx context.Context, account *blobv1.StorageAccount) error {
	log := log.FromContext(ctx)

	secret, err := (*r.SecretClient).Get(ctx, "accounts", v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			secret = &coreV1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: "accounts",
				},
				Type: "Opaque",
			}
			secret, err = (*r.SecretClient).Create(ctx, secret, v1.CreateOptions{})
			if err != nil {
				log.Error(err, "Failed to create account secret")
				return err
			}
		} else {
			log.Error(err, "Failed to fetch account secret")
			return err
		}
	}

	if len(account.Status.Keys) == 0 {
		log.Info("Creating keys for new storage account")
		keys := []blobv1.StorageAccountKey{
			r.GenerateStorageAccountKey("key1", "Full"),
			r.GenerateStorageAccountKey("key2", "Full"),
		}

		account.Status.Keys = keys

		err = r.Status().Update(ctx, account)
		if err != nil {
			log.Error(err, "Failed to create keys for new storage account")
		} else {
			log.Info("Created keys for new storage account")
		}
	}

	serializedKeys, err := json.Marshal(account.Status.Keys)
	if err != nil {
		log.Error(err, "Failed to encode secret for storage account")
		return err
	}

	if !bytes.Equal(serializedKeys, secret.Data[account.Name]) {
		log.Info("Updating secret for storage account")
		patch, err := json.Marshal(coreV1.Secret{
			Data: map[string][]byte{
				account.Name: serializedKeys,
			},
		})

		if err != nil {
			log.Error(err, "Failed to encode secret for storage account")
			return err
		}

		_, err = (*r.SecretClient).Patch(ctx, "accounts", types.StrategicMergePatchType, patch, v1.PatchOptions{})
		if err != nil {
			log.Error(err, "Failed to update secret for storage account")
			return err
		}

		log.Info("Updated secret for storage account")
	}

	return err
}

func (r *StorageAccountReconciler) DeleteSecretForAccount(ctx context.Context, accountName string) error {
	log := log.FromContext(ctx)

	log.Info("Deleting secret for storage account")

	patch := []byte(fmt.Sprintf("[{\"op\":\"remove\", \"path\":\"/data/%s\"}]", accountName))

	_, err := (*r.SecretClient).Patch(ctx, "accounts", types.JSONPatchType, patch, v1.PatchOptions{})
	if err != nil {
		log.Error(err, "Failed to delete secret for storage account")
		return err
	}

	log.Info("Deleted secret for storage account")

	return nil
}

//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=blob.arcblob.poc,resources=storageaccounts/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StorageAccount object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *StorageAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var account blobv1.StorageAccount
	if err := r.Get(ctx, req.NamespacedName, &account); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch storage account")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizerName := "storageaccounts.arcblob.poc/finalizer"

	if account.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&account, finalizerName) {
			controllerutil.AddFinalizer(&account, finalizerName)
			if err := r.Update(ctx, &account); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&account, finalizerName) {
			if err := r.DeleteSecretForAccount(ctx, account.Name); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&account, finalizerName)
			if err := r.Update(ctx, &account); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.CreateOrUpdateSecretForAccount(ctx, &account); err != nil {
		log.Error(err, "failed to fetch current account")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&blobv1.StorageAccount{}).
		Complete(r)
}
