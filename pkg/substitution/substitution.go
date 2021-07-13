package substitution

import (
	"github.com/openshift/cluster-cloud-controller-manager-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Names in this list are unique and will be substituted with an image from config
	// cloudControllerManagerName is a name for default CCM controller container any provider may have
	cloudControllerManagerName = "cloud-controller-manager"
	cloudNodeManagerName       = "cloud-node-manager"

	infraNameEnvVar = "OCP_INFRASTRUCTURE_NAME"
)

// setCloudControllerImage substitutes controller containers in provided pod specs with correct image
func setCloudControllerImage(config config.OperatorConfig, p corev1.PodSpec) corev1.PodSpec {
	updatedPod := *p.DeepCopy()
	for i, container := range p.Containers {
		substituteName := ""
		switch container.Name {
		case cloudControllerManagerName:
			substituteName = config.ControllerImage
		case cloudNodeManagerName:
			substituteName = config.CloudNodeImage
		default:
			continue
		}

		if substituteName != "" {
			klog.Infof("Substituting container image for container %q with %q", container.Name, substituteName)
			updatedPod.Containers[i].Image = substituteName
		}
	}

	return updatedPod
}

// setInfrastructureNameVariable tries to find env variable with name OCP_INFRASTRUCTURE_NAME, if found put infra name from infra resource there.
func setInfrastructureNameVariable(infrastructureName string, p corev1.PodSpec) corev1.PodSpec {
	updatedPod := *p.DeepCopy()
	for _, container := range updatedPod.Containers {
		for i, envVar := range container.Env {
			if envVar.Name == infraNameEnvVar {
				container.Env[i].Value = infrastructureName
				break
			}
		}
	}
	return updatedPod
}

func FillConfigValues(config config.OperatorConfig, templates []client.Object) []client.Object {
	objects := make([]client.Object, len(templates))
	for i, objectTemplate := range templates {
		templateCopy := objectTemplate.DeepCopyObject().(client.Object)

		// Set namespaces for all object. Namespace on cluster-wide objects is stripped by API server and is not applied
		templateCopy.SetNamespace(config.ManagedNamespace)

		switch obj := templateCopy.(type) {
		case *appsv1.Deployment:
			obj.Spec.Template.Spec = setCloudControllerImage(config, obj.Spec.Template.Spec)
			obj.Spec.Template.Spec = setInfrastructureNameVariable(config.InfrastructureName, obj.Spec.Template.Spec)
			if config.IsSingleReplica {
				obj.Spec.Replicas = pointer.Int32(1)
			}
		case *appsv1.DaemonSet:
			obj.Spec.Template.Spec = setCloudControllerImage(config, obj.Spec.Template.Spec)
		case *corev1.Pod:
			obj.Spec = setCloudControllerImage(config, obj.Spec)
		}
		objects[i] = templateCopy
	}
	return objects
}
