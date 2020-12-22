/*


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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	istiocarotationv1 "istio-ca-rotation/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
)

// NewCAReconciler reconciles a NewCA object
type NewCAReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Namespaces []string
}

// +kubebuilder:rbac:groups=istiocarotation.intel.com,resources=newcas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=istiocarotation.intel.com,resources=newcas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;create;update;patch;watch

const (
	caCert    = "ca-cert.pem"
	caKey     = "ca-key.pem"
	rootCert  = "root-cert.pem"
	certChain = "cert-chain.pem"

	newCAName      = "new-ca"
	newCANamespace = "istio-system"
)

func checkFiles(secret *corev1.Secret, fileList []string) bool {
	if secret.Data == nil {
		return false
	}

	for _, file := range fileList {
		if _, found := secret.Data[file]; !found {
			return false
		}
	}

	return true
}

func isValidIstioSecret(secret *corev1.Secret) bool {
	requiredFiles := []string{caCert, caKey}
	return checkFiles(secret, requiredFiles)
}

func isValidCA(secret *corev1.Secret) bool {
	requiredFiles := []string{caCert, caKey, rootCert}
	return checkFiles(secret, requiredFiles)
}

func restartIstiod(client client.Client, log logr.Logger) error {
	ctx := context.Background()
	// Restart AuthService deployment by adding/updating an annotation.
	deploymentName := types.NamespacedName{
		Namespace: "istio-system",
		Name:      "istiod",
	}
	_ = log.WithValues("Restarting Istiod deployment", deploymentName)
	var deployment appsv1.Deployment
	if err := client.Get(ctx, deploymentName, &deployment); err != nil {
		_ = log.WithValues("Failed to find Istiod deployment", deploymentName)
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string, 0)
	}
	deployment.Spec.Template.Annotations["newca-controller/restartedAt"] = time.Now().Format(time.RFC3339)
	if err := client.Update(ctx, &deployment); err != nil {
		_ = log.WithValues("Failed to update Istiod deployment", deploymentName)
		return err
	}

	return nil
}

func (r *NewCAReconciler) setStatus(newca *istiocarotationv1.NewCA, status istiocarotationv1.RotationState) error {
	ctx := context.Background()
	_, err := ctrl.CreateOrUpdate(ctx, r, newca, func() error {
		newca.Status.Status = status
		return nil
	})
	return err
}

func (r *NewCAReconciler) getEnvoyCerts() error {
	return nil
}

func (r *NewCAReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("newca", req.NamespacedName)

	var newca istiocarotationv1.NewCA
	if err := r.Get(ctx, req.NamespacedName, &newca); err != nil {
		logger.Info("NewCA not found, ignoring")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check the name. We only accept one NewCA.
	if newca.Name != newCAName || newca.Namespace != newCANamespace {
		logger.Info("NewCA in wrong place")
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("Not expecting CR with this name/namespace"))
	}

	newSecretName := types.NamespacedName{
		Name:      newca.Spec.Secret,
		Namespace: newca.Spec.Namespace,
	}

	// Get the new cert from the secret.
	var newSecret corev1.Secret
	if err := r.Get(ctx, newSecretName, &newSecret); err != nil {
		r.setStatus(&newca, istiocarotationv1.FailedRotation)
		logger.Info("Failed to find new secret")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var secret *corev1.Secret

	// Get the user-provided CA from "cacerts" secret.
	cacertsSecretName := types.NamespacedName{
		Name:      "cacerts",
		Namespace: "istio-system",
	}
	var cacertsSecret corev1.Secret
	secret = &cacertsSecret

	if err := r.Get(ctx, cacertsSecretName, &cacertsSecret); err != nil {
		// Fall back to istio-generated "istio-ca-secret"
		istioCaSecretName := types.NamespacedName{
			Name:      "istio-ca-secret",
			Namespace: "istio-system",
		}
		var istioCaSecret corev1.Secret
		if err := r.Get(ctx, istioCaSecretName, &istioCaSecret); err != nil {
			r.setStatus(&newca, istiocarotationv1.FailedRotation)
			logger.Info("Failed to find original secret")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Since we'll need to create new cacerts, initialize a new object for it.
		// The new cert will be installed as "cacerts", since that's the name for user CA.
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cacertsSecretName.Name,
				Namespace: cacertsSecretName.Namespace,
			},
			Data: map[string][]byte{},
		}
	}

	if !isValidCA(&newSecret) {
		r.setStatus(&newca, istiocarotationv1.FailedRotation)
		logger.Info("Invalid new secret")
		return ctrl.Result{}, fmt.Errorf("Invalid new secret")
	}

	// If the certs don't match, make the status "In progress", else return.
	if newCert, found := newSecret.Data[caCert]; found {
		if oldCert, found := secret.Data[caCert]; found {
			if bytes.Compare(oldCert, newCert) == 0 {
				// Certificates are equal, no need to reconcile them.
				r.setStatus(&newca, istiocarotationv1.CompleteRotation)
				return ctrl.Result{}, nil
			}
		}
	} else {
		// The new certificate data is invalid, FIXME: we have already checked this.
		r.setStatus(&newca, istiocarotationv1.FailedRotation)
		logger.Info("Invalid certificate data")
		return ctrl.Result{}, fmt.Errorf("Invalid certificate data")
	}

        // TODO: Rotating certs when ROOTCA is changed
        // Now it rotates only if intermediateCA has changed and it should be
        // generated based on current ROOTCA
        // FIXME: How to verify this intermediate certs generated based on current root certs?
        if newRoot, found := newSecret.Data[rootCert]; found {
                if oldRoot, found := secret.Data[rootCert]; found {
                        if bytes.Compare(oldRoot, newRoot) != 0 {
                                r.setStatus(&newca, istiocarotationv1.CompleteRotation)
                                logger.Info("Root cert changed, rotation not supported")
                                return ctrl.Result{}, nil
                        }
                }
        } else {
                // The new certificate data is invalid, FIXME: we have already checked this.
                r.setStatus(&newca, istiocarotationv1.FailedRotation)
                logger.Info("Invalid root certificate data")
                return ctrl.Result{}, fmt.Errorf("Invalid root certificate data")
        }

	// Start the rotation
	err := r.setStatus(&newca, istiocarotationv1.InProgressRotation)
	if err != nil {
		logger.Info("Can't set rotation status")
		return ctrl.Result{}, err
	}

        // Install the new certs to "cacerts".
	_, err = ctrl.CreateOrUpdate(ctx, r, secret, func() error {
		secret.Data[caCert] = newSecret.Data[caCert]
		secret.Data[rootCert] = newSecret.Data[rootCert]
		secret.Data[caKey] = newSecret.Data[caKey]
		certChainValue, found := newSecret.Data[certChain]
		if found {
			secret.Data[certChain] = certChainValue
		}
		return nil
	})
	if err != nil {
		r.setStatus(&newca, istiocarotationv1.FailedRotation)
		logger.Info("Failed to update Istio secret with new secret")
		return ctrl.Result{}, err
	}

	// Restart istiod so that it uses the new CA.
	err = restartIstiod(r, logger)
	if err != nil {
		r.setStatus(&newca, istiocarotationv1.FailedRotation)
		logger.Info("Failed to restart Istiod with new secret")
		return ctrl.Result{}, err
	}

	// FIXME: Periodically check that the workload cert roots match with the new cert root.

	// When they are in sync, make the status "Complete".
	r.setStatus(&newca, istiocarotationv1.CompleteRotation)
	return ctrl.Result{}, nil
}

func (r *NewCAReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&istiocarotationv1.NewCA{}).
		Complete(r)
}
