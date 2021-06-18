package controllers

import (
	"context"
	"fmt"
	"strings"

	localv1 "github.com/openshift/local-storage-operator/pkg/apis/local/v1"
	v1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	OPERATOR_GROUP = "local-operator-group"
	SUBSCRIPTION   = "local-storage-operator"
	LOCAl_VOLUME   = "local-disk"
)

// fetchRemovePV finds and deletes all local volume PVs
func (r *FinalizerOperatorReconciler) fetchRemovePV(ctx context.Context) bool {
	// Find PVs
	persistenceVolume := []corev1.PersistentVolume{}
	// labels := map[string]string{"storage.openshift.com/local-volume-owner-name": LOCAl_VOLUME, "storage.openshift.com/local-volume-owner-namespace": namespace}
	pvList := &corev1.PersistentVolumeList{}
	err := r.List(ctx, pvList)
	if err != nil {
		fmt.Print("Error in Getting PV List")
		return false
	}
	for _, pv := range pvList.Items {
		// if reflect.DeepEqual(labels, pv.Labels) {
		if strings.HasPrefix(pv.Name, "local-pv-") {
			persistenceVolume = append(persistenceVolume, pv)
			fmt.Println("PV", pv.Status.Phase)
		}
	}
	// PV Deletion
	for _, pv := range persistenceVolume {
		err = r.Delete(ctx, &pv)
		if err != nil && !errors.IsNotFound(err) {
			fmt.Print("Error in Deleting PV ", pv.Name)
			return false
		}
	}
	fmt.Println("PV Deleted.....")
	return true
}

// deleteMountedPath deletes mounted path from each node
func (r *FinalizerOperatorReconciler) deleteMountedPath(ctx context.Context) bool {
	// Remove Mounted Path
	nodesList := &corev1.NodeList{}
	err := r.List(ctx, nodesList)
	if err != nil {
		fmt.Print("Error in Getting Nodes List")
		return false
	}

	for _, node := range nodesList.Items {
		command := "oc debug node/" + node.Name + " -- chroot /host rm -rf /mnt"
		_, out, _ := ExecuteCommand(command)
		fmt.Println(out)
	}
	fmt.Println("Mounted Paths Removed....")
	return true
}

// patchLocalVolume patches finalizer in LocalVolume Resource
func patchLocalVolume(namespace string) {
	command := "kubectl patch --type=merge -n " + namespace + " localvolumes.local.storage.openshift.io " + LOCAl_VOLUME + " -p '{\"metadata\":{\"finalizers\":null}}'"
	_, out, _ := ExecuteCommand(command)
	fmt.Println(out)
}

//patchFinalizer patches finalizer in Resources
func patchFinalizer(rtype string, name string, namespace string) {
	_, out, _ := ExecuteCommand("kubectl patch " + rtype + " " + name + " -n " + namespace + " -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge")
	fmt.Println(out)
}

// localVolumeNSCleanUp performs cleanUp when namespace is in terminating state
func (r *FinalizerOperatorReconciler) localVolumeNSCleanUp(ctx context.Context, namespace string, resources []map[string]string, flag bool) bool {
	patchLocalVolume(namespace)
	if r.fetchRemovePV(ctx) && r.deleteMountedPath(ctx) {
		if flag {
			for index := range resources {
				resourceType := resources[index]["Type"]
				resourceName := resources[index]["Name"]
				resourceNamespace := resources[index]["Namespace"]
				if resourceType == "deployment" { //|| resourceType == "localvolume"
					patchFinalizer(resourceType, resourceName, resourceNamespace)
				}
			}
		}
		return true
	}
	return false
}

// deleteSubAndOg deletes Subscription and Operator Group
func (r *FinalizerOperatorReconciler) deleteSubAndOg(ctx context.Context, namespace string) bool {
	// Subscription Deletion
	sub := &v1alpha1.Subscription{}
	err := r.Get(ctx, types.NamespacedName{Name: SUBSCRIPTION, Namespace: namespace}, sub)
	if err != nil {
		fmt.Print("Error in Getting Subscription")
		return false
	}
	err = r.Delete(ctx, sub)
	if err != nil {
		fmt.Print("Error in Deleting Subscription", sub.Name)
		return false
	}
	fmt.Println("Subscription Deleted.....")

	// OperatorGroup Deletion
	og := &v1.OperatorGroup{}
	err = r.Get(ctx, types.NamespacedName{Name: OPERATOR_GROUP, Namespace: namespace}, og)
	if err != nil {
		fmt.Print("Error in Getting OperatorGroup")
		return false
	}
	err = r.Delete(ctx, og)
	if err != nil {
		fmt.Print("Error in Deleting OperatorGroup", og.Name)
		return false
	}
	fmt.Println("OperatorGroup Deleted.....")
	return true
}

// localVolumeCleanUp performs cleanup for local-volume template
func (r *FinalizerOperatorReconciler) localVolumeCleanUp(ctx context.Context, namespace string) bool {
	// LocalVolume Resource Deletion
	lv := &localv1.LocalVolume{}
	err := r.Get(ctx, types.NamespacedName{Name: LOCAl_VOLUME, Namespace: namespace}, lv)
	if err != nil {
		fmt.Print("Error in Getting LocalVolume")
		return false
	}

	patchLocalVolume(namespace)

	err = r.Delete(ctx, lv)
	if err != nil {
		fmt.Print("Error in Deleting LocalVolume", lv.Name)
		return false
	}
	fmt.Println("LocalVolume Deleted.....")

	if r.fetchRemovePV(ctx) && r.deleteMountedPath(ctx) && r.deleteSubAndOg(ctx, namespace) {
		return true
	}
	return false
}
