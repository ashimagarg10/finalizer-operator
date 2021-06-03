/*
Copyright 2021.

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
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	finalizerv1 "github.com/example/finalizer-operator/api/v1"
)

// FinalizerOperatorReconciler reconciles a FinalizerOperator object
type FinalizerOperatorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=finalizer.example.com,resources=finalizeroperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finalizer.example.com,resources=finalizeroperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=finalizer.example.com,resources=finalizeroperators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FinalizerOperator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *FinalizerOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("finalizeroperator", req.NamespacedName)

	instance := &finalizerv1.FinalizerOperator{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("FinalizerOperator resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get FinalizerOperator")
		return ctrl.Result{}, err
	}

	resources := instance.Spec.Resources
	namespace := instance.Spec.Namespace

	fmt.Println(resources)
	fmt.Println(namespace)

	finalizer_name := "testing/finalizer"

	flag := true

	for flag {
		for index := range resources {
			resourceType := resources[index].Name
			resourceName := resources[index].Value

			if resourceType == "deployment" {
				fmt.Println("Getting Deployment")
				res := &appsv1.Deployment{}
				err = r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, res)
				if err != nil {
					fmt.Print("Error in Getting deployment")
					return ctrl.Result{}, err
				}

				fmt.Println("Check for Finalizer")

				// examine DeletionTimestamp to determine if object is under deletion
				if res.ObjectMeta.DeletionTimestamp.IsZero() {
					// The object is not being deleted, so if it does not have our finalizer,
					// then lets add the finalizer and update the object. This is equivalent
					// registering our finalizer.
					if !containsString(res.GetFinalizers(), finalizer_name) {
						controllerutil.AddFinalizer(res, finalizer_name)
						err = r.Update(ctx, res)
						if err != nil {
							log.Error(err, "Error is updating resource ", resourceName)
							return ctrl.Result{}, err
						}
					}
				} else {
					// The object is being deleted
					if containsString(res.GetFinalizers(), finalizer_name) {
						// our finalizer is present
						fmt.Println("Finalizer Present")
						// remove external dependencies

						_, out, _ := ExecuteCommand("kubectl patch tprov trident -n " + namespace + " --type=merge -p '{\"spec\":{\"uninstall\":true}}'")
						fmt.Println(out)
						_, out, _ = ExecuteCommand("kubectl patch tprov trident -n " + namespace + " --type=merge -p '{\"spec\":{\"wipeout\":[\"crds\"],\"uninstall\":true}}'")
						fmt.Println(out)
					}
					// remove our finalizer from the list and update it.
					controllerutil.RemoveFinalizer(res, finalizer_name)
					err = r.Update(ctx, res)
					if err != nil {
						log.Error(err, "Error is updating resource ", resourceName)
						return ctrl.Result{}, err
					}
					flag = false
				}
			} else if resourceType == "pod" {
				fmt.Println("Getting POD")
				res := &corev1.Pod{}
				err = r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, res)
				if err != nil {
					fmt.Print("Error in Getting pod")
					return ctrl.Result{}, err
				}

				fmt.Println("Check for Finalizer")

				// examine DeletionTimestamp to determine if object is under deletion
				if res.ObjectMeta.DeletionTimestamp.IsZero() {
					// The object is not being deleted, so if it does not have our finalizer,
					// then lets add the finalizer and update the object. This is equivalent
					// registering our finalizer.
					if !containsString(res.GetFinalizers(), finalizer_name) {
						controllerutil.AddFinalizer(res, finalizer_name)
						err = r.Update(ctx, res)
						if err != nil {
							log.Error(err, "Error is updating resource ", resourceName)
							return ctrl.Result{}, err
						}
					}
				} else {
					// The object is being deleted
					if containsString(res.GetFinalizers(), finalizer_name) {
						// our finalizer is present
						fmt.Println("Finalizer Present")
					}
					// remove our finalizer from the list and update it.
					controllerutil.RemoveFinalizer(res, finalizer_name)
					err = r.Update(ctx, res)
					if err != nil {
						log.Error(err, "Error is updating resource ", resourceName)
						return ctrl.Result{}, err
					}
				}
			} else if resourceType == "pvc" {
				fmt.Println("Getting PVC")
				// res := &v1.OperatorGroup{}
				res := &corev1.PersistentVolumeClaim{}
				err = r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: "default"}, res)
				if err != nil {
					fmt.Print("Error in Getting pvc")
					return ctrl.Result{}, err
				}

				fmt.Println("Check for Finalizer")

				// examine DeletionTimestamp to determine if object is under deletion
				if res.ObjectMeta.DeletionTimestamp.IsZero() {
					// The object is not being deleted, so if it does not have our finalizer,
					// then lets add the finalizer and update the object. This is equivalent
					// registering our finalizer.
					if !containsString(res.GetFinalizers(), finalizer_name) {
						controllerutil.AddFinalizer(res, finalizer_name)
						err = r.Update(ctx, res)
						if err != nil {
							log.Error(err, "Error is updating resource ", resourceName)
							return ctrl.Result{}, err
						}
					}
				} else {
					// The object is being deleted
					if containsString(res.GetFinalizers(), finalizer_name) {
						// our finalizer is present, so handle any external dependency
						fmt.Println("Finalizer Present")

						command := "kubectl -n default patch persistentvolumeclaim/" + resourceName + " -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge"
						_, out, _ := ExecuteCommand(command)
						fmt.Println(out)
					}
					// remove our finalizer from the list and update it.
					controllerutil.RemoveFinalizer(res, finalizer_name)
					err = r.Update(ctx, res)
					if err != nil {
						log.Error(err, "Error is updating resource ", resourceName)
						return ctrl.Result{}, err
					}
				}
			}
		}
		time.Sleep(time.Duration(10) * time.Second)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FinalizerOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&finalizerv1.FinalizerOperator{}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ExecuteCommand to execute shell commands
func ExecuteCommand(command string) (int, string, string) {
	fmt.Println("in ExecuteCommand")
	var cmd *exec.Cmd
	var cmdErr bytes.Buffer
	var cmdOut bytes.Buffer
	cmdErr.Reset()
	cmdOut.Reset()

	cmd = exec.Command("bash", "-c", command)
	cmd.Stderr = &cmdErr
	cmd.Stdout = &cmdOut
	err := cmd.Run()

	var waitStatus syscall.WaitStatus

	errStr := strings.TrimSpace(cmdErr.String())
	outStr := strings.TrimSpace(cmdOut.String())
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
		}
		if errStr != "" {
			fmt.Println(command)
			fmt.Println(errStr)
		}
	} else {
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
	}
	if waitStatus.ExitStatus() == -1 {
		fmt.Print(time.Now().String() + " Timed out " + command)
	}
	return waitStatus.ExitStatus(), outStr, errStr
}
