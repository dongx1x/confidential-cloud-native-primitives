package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const VolumeAnnotation = "ccnp.cc-api/require"
const CcnpServerSock = "/run/ccnp/uds/ccnp-server.sock"

type podCcnpAdmission struct {
	Client         client.Client
	decoder        *admission.Decoder
	Annotation     bool
	StorageClasses []string
}

func (a *podCcnpAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	if err := a.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// check for the existence of a pod annotation if enabled
	if a.Annotation {
		value, ok := pod.Annotations[VolumeAnnotation]
		if !ok {
			return admission.Allowed("Got no pod annotation.")
		}

		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if !parsed {
			return admission.Allowed("Pod annotation says false.")
		}
	}

	pathType := corev1.HostPathSocket
	sockName := "ccnp-server-sock"
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: sockName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: CcnpServerSock,
				Type: &pathType,
			},
		},
	})

	for c := range pod.Spec.Containers {
		container := &pod.Spec.Containers[c]
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      sockName,
			ReadOnly:  false,
			MountPath: CcnpServerSock,
		})
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (a *podCcnpAdmission) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
