package mapper

import (
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sandboxv1 "sandbox-operator/api/v1alpha1"
	"sandbox-operator/internal/credentials"
	"sandbox-operator/internal/openapi"
)

func OpenAPICredential(in *credentials.OpenAPICredential) openapi.Credential {
	if in == nil {
		return openapi.Credential{}
	}
	return openapi.Credential{
		AccessKeyID:     in.AccessKeyID,
		SecretAccessKey: in.SecretAccessKey,
		AccountID:       in.AccountID,
		Region:          in.Region,
	}
}

func TemplateCreateRequest(in *sandboxv1.SandboxTemplate, runtime RuntimeCredentials) openapi.CreateTemplateRequest {
	spec := in.Spec
	return openapi.CreateTemplateRequest{
		TemplateName:     in.Name,
		Description:      spec.Description,
		TemplateCategory: templateAccess(spec.Category),
		TemplateType:     templateType(spec.Type),
		Image:            imageURL(spec.Image),
		ImageSource:      imageSource(spec.Image),
		Command:          spec.Command,
		Ports:            append([]int(nil), spec.Ports...),
		CPU:              resourceCPU(spec.Resources),
		Memory:           resourceMemoryMB(spec.Resources),
		Envs:             envsToMap(spec.Env),
		NetworkConfig:    networkToOpenAPI(spec.Network),
		TargetPoolSize:   preheatTargetPoolSize(spec.Preheat),
		InstanceQuota:    spec.InstanceQuota,
		KS3MountConfig:   mountToOpenAPI(spec.Ks3MountConfig, runtime.KS3, "ks3"),
		KPFSMountConfig:  mountToOpenAPI(spec.KpfsMountConfig, runtime.KPFS, "kpfs"),
		KlogConfig:       klogToOpenAPI(spec.Klog),
	}
}

func TemplateUpdateRequest(in *sandboxv1.SandboxTemplate, runtime RuntimeCredentials) openapi.UpdateTemplateRequest {
	req := TemplateCreateRequest(in, runtime)
	return openapi.UpdateTemplateRequest{
		TemplateID:            in.Status.TemplateID,
		CreateTemplateRequest: req,
	}
}

func SandboxStartRequest(in *sandboxv1.Sandbox, templateID string, runtime RuntimeCredentials) openapi.StartSandboxRequest {
	spec := in.Spec
	return openapi.StartSandboxRequest{
		TemplateID: templateID,
		Timeout:    spec.TimeoutSeconds,
		EnvVars:    envsToMap(spec.Env),
		Metadata:   sandboxMetadata(spec, runtime),
	}
}

type RuntimeCredentials struct {
	KS3  *credentials.RuntimeCredential
	KPFS *credentials.RuntimeCredential
}

func ApplyTemplateSpecFromOpenAPI(obj *sandboxv1.SandboxTemplate, remote openapi.Template) {
	obj.Spec.Description = remote.Description
	obj.Spec.Category = remote.TemplateCategory
	obj.Spec.Type = remote.TemplateType
	obj.Spec.Image = imageFromOpenAPI(remote, obj.Spec.Image)
	obj.Spec.Command = remote.Command
	obj.Spec.Ports = append([]int(nil), remote.Ports...)
	obj.Spec.Resources = &sandboxv1.ResourceSpec{CPU: remote.CPU, MemoryGB: memoryGBFromMB(remote.Memory)}
	obj.Spec.Env = envsFromMap(remote.Envs)
	obj.Spec.Network = networkFromOpenAPI(remote.NetworkConfig)
	obj.Spec.Preheat = preheatFromTargetPoolSize(remote.TargetPoolSize)
	obj.Spec.InstanceQuota = remote.InstanceQuota
}

func ApplyTemplateStatusFromOpenAPI(obj *sandboxv1.SandboxTemplate, remote openapi.Template) {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.TemplateID = remote.Identifier()
	obj.Status.Phase = TemplatePhase(remote.Status)
	obj.Status.RawStatus = remote.Status
	obj.Status.CanDelete = remote.CanDelete
	obj.Status.CreatedAt = metaTimeString(remote.CreatedAt)
	obj.Status.UpdatedAt = metaTimeString(remote.UpdatedAt)
	obj.Status.ExternalUpdatedAt = metaTimeString(remote.UpdatedAt)
	if remote.KlogConfig != nil {
		obj.Status.Klog = &sandboxv1.KlogStatus{ProjectName: remote.KlogConfig.ProjectName, PoolName: remote.KlogConfig.PoolNameContainer}
	}
	obj.Status.Quota = &sandboxv1.QuotaStatus{
		InstanceQuota:                remote.InstanceQuota,
		RemainingInstanceQuota:       remote.RemainingInstanceQuota,
		RemainingSystemInstanceQuota: remote.RemainingSystemInstanceQuota,
	}
	obj.Status.Preheat = &sandboxv1.PreheatStatus{
		Enabled:                 remote.TargetPoolSize > 0,
		Number:                  remote.TargetPoolSize,
		PreheatedInstanceNumber: remote.PreheatedInstanceNumber,
	}
	if remote.CredentialAccessKeyIDMasked != "" {
		now := metav1.Now()
		obj.Status.CredentialDrift = &sandboxv1.CredentialDriftSet{
			KS3: &sandboxv1.CredentialDriftStatus{
				InSync:            "unknown",
				Source:            "OpenAPI",
				AccessKeyIDMasked: remote.CredentialAccessKeyIDMasked,
				ObservedAt:        &now,
				Reason:            "SecretNotReconciled",
			},
		}
	}
}

func ApplySandboxSpecFromOpenAPI(obj *sandboxv1.Sandbox, remote openapi.Sandbox) {
	if obj.Spec.Name == "" {
		obj.Spec.Name = obj.Name
	}
	if remote.Timeout > 0 {
		obj.Spec.TimeoutSeconds = remote.Timeout
	}
}

func ApplySandboxStatusFromOpenAPI(obj *sandboxv1.Sandbox, remote openapi.Sandbox) {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.SandboxID = remote.SandboxID
	obj.Status.ExternalUpdatedAt = metaTimeString(remote.EndTime)
	obj.Status.Template = &sandboxv1.SandboxTemplateSummary{
		ID:       remote.TemplateID,
		Type:     remote.TemplateType,
		Category: remote.TemplateCategory,
	}
	obj.Status.Phase = SandboxPhase(remote.Status)
	obj.Status.RawStatus = remote.Status
	obj.Status.TimeoutSeconds = remote.Timeout
	obj.Status.CreateTime = metaTimeString(remote.CreateTime)
	obj.Status.EndTime = metaTimeString(remote.EndTime)
	obj.Status.Endpoint = remote.Endpoint
	if remote.CustomConfiguration != nil {
		obj.Status.CustomConfiguration = &sandboxv1.SandboxCustomConfiguration{
			ImageURL: remote.CustomConfiguration.ImageURL,
			Port:     remote.CustomConfiguration.Port,
			Command:  remote.CustomConfiguration.Command,
		}
	}
}

func TemplatePhase(raw string) sandboxv1.Phase {
	switch raw {
	case "Ready", "READY", "ready":
		return sandboxv1.PhaseReady
	case "creating", "CREATING":
		return sandboxv1.PhasePending
	case "error", "ERROR":
		return sandboxv1.PhaseFailed
	case "":
		return sandboxv1.PhasePending
	default:
		return sandboxv1.PhaseUnknown
	}
}

func SandboxPhase(raw string) sandboxv1.Phase {
	switch raw {
	case "STARTING", "starting":
		return sandboxv1.PhaseStarting
	case "RUNNING", "running":
		return sandboxv1.PhaseRunning
	case "KILLING", "killing":
		return sandboxv1.PhaseDeleting
	case "FAILED", "failed":
		return sandboxv1.PhaseFailed
	case "UNHEALTHY", "unhealthy":
		return sandboxv1.PhaseUnhealthy
	case "PAUSED", "paused":
		return sandboxv1.PhasePaused
	case "RESUMING", "resuming":
		return sandboxv1.PhaseResuming
	case "":
		return sandboxv1.PhaseUnknown
	default:
		return sandboxv1.PhaseUnknown
	}
}

func imageURL(in *sandboxv1.ImageSpec) string {
	if in == nil {
		return ""
	}
	if in.ImageURL != "" {
		return in.ImageURL
	}
	if in.ImageName != "" && in.ImageTag != "" {
		return in.ImageName + ":" + in.ImageTag
	}
	return in.ImageName
}

func imageSource(in *sandboxv1.ImageSpec) string {
	if in == nil {
		return ""
	}
	return in.Source
}

func templateAccess(value string) string {
	switch strings.ToLower(value) {
	case "private":
		return "private"
	case "public":
		return "public"
	default:
		return value
	}
}

func templateType(value string) string {
	switch strings.ToLower(value) {
	case "custom":
		return "custom"
	case "browser":
		return "browser"
	case "code":
		return "code"
	case "aio":
		return "AIO"
	default:
		return value
	}
}

func imageFromOpenAPI(remote openapi.Template, previous *sandboxv1.ImageSpec) *sandboxv1.ImageSpec {
	if remote.Image == "" && remote.ImageSource == "" {
		return previous
	}
	out := &sandboxv1.ImageSpec{}
	if previous != nil {
		out.CredentialRef = previous.CredentialRef
	}
	out.Source = remote.ImageSource
	out.ImageURL = remote.Image
	out.ImageEndpoint = remote.CredentialServer
	return out
}

func networkToOpenAPI(in *sandboxv1.NetworkSpec) *openapi.NetworkConfig {
	if in == nil {
		return nil
	}
	out := &openapi.NetworkConfig{
		PublicNetworkEnable:        in.PublicNetworkEnable,
		PrivateNetworkEnable:       in.PrivateNetworkEnable,
		SharedInternetAccessEnable: in.SharedInternetAccessEnable,
	}
	if in.VPC != nil {
		out.VPCID = in.VPC.VPCID
		out.SubnetID = in.VPC.SubnetID
	}
	return out
}

func networkFromOpenAPI(in *openapi.NetworkConfig) *sandboxv1.NetworkSpec {
	if in == nil {
		return nil
	}
	return &sandboxv1.NetworkSpec{
		PublicNetworkEnable:        in.PublicNetworkEnable,
		PrivateNetworkEnable:       in.PrivateNetworkEnable,
		SharedInternetAccessEnable: in.SharedInternetAccessEnable,
		VPC: &sandboxv1.VPCSpec{
			VPCID:    in.VPCID,
			SubnetID: in.SubnetID,
		},
	}
}

func preheatFromTargetPoolSize(size int) *sandboxv1.PreheatSpec {
	if size <= 0 {
		return nil
	}
	return &sandboxv1.PreheatSpec{Enabled: true, Number: size}
}

func mountToOpenAPI(in *sandboxv1.MountConfig, cred *credentials.RuntimeCredential, kind string) *openapi.MountConfig {
	if in == nil {
		return nil
	}
	out := &openapi.MountConfig{
		MountPoints: mountPointsToOpenAPI(in.MountPoints),
	}
	switch kind {
	case "ks3":
		out.EnableKS3 = in.Enabled
	case "kpfs":
		out.EnableKPFS = in.Enabled
	}
	if cred != nil {
		out.Credential = &openapi.MountCredential{
			AccessKey:       cred.AccessKey,
			SecretAccessKey: cred.SecretAccessKey,
		}
	}
	return out
}

func mountPointsToOpenAPI(in []sandboxv1.MountPoint) []openapi.MountPoint {
	out := make([]openapi.MountPoint, 0, len(in))
	for _, item := range in {
		out = append(out, openapi.MountPoint{
			BucketName:     item.BucketName,
			BucketPath:     item.RemotePath,
			FileSystemName: item.FileSystemName,
			RemotePath:     item.RemotePath,
			LocalMountPath: item.LocalMountPath,
			ReadOnly:       item.ReadOnly,
		})
	}
	return out
}

func envsToMap(in []sandboxv1.EnvVar) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for _, item := range in {
		out[item.Key] = item.Value
	}
	return out
}

func envsFromMap(in map[string]string) []sandboxv1.EnvVar {
	out := make([]sandboxv1.EnvVar, 0, len(in))
	for key, value := range in {
		out = append(out, sandboxv1.EnvVar{Key: key, Value: value})
	}
	return out
}

func resourceCPU(in *sandboxv1.ResourceSpec) int {
	if in == nil {
		return 0
	}
	return in.CPU
}

func resourceMemoryMB(in *sandboxv1.ResourceSpec) int {
	if in == nil {
		return 0
	}
	return in.MemoryGB * 1024
}

func memoryGBFromMB(value int) int {
	if value <= 0 {
		return 0
	}
	return (value + 1023) / 1024
}

func preheatTargetPoolSize(in *sandboxv1.PreheatSpec) int {
	if in == nil {
		return 0
	}
	if !in.Enabled {
		return 0
	}
	return in.Number
}

func klogToOpenAPI(in *sandboxv1.KlogSpec) *openapi.KlogConfig {
	if in == nil {
		return nil
	}
	return &openapi.KlogConfig{Enabled: in.Enabled}
}

func sandboxMetadata(spec sandboxv1.SandboxSpec, runtime RuntimeCredentials) map[string]interface{} {
	metadata := map[string]interface{}{}
	var mounts []map[string]interface{}
	mounts = appendVolumeMounts(mounts, "ks3", spec.Ks3MountConfig, runtime.KS3)
	mounts = appendVolumeMounts(mounts, "kpfs", spec.KpfsMountConfig, runtime.KPFS)
	if len(mounts) > 0 {
		metadata["volumeMounts"] = mounts
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func appendVolumeMounts(out []map[string]interface{}, kind string, cfg *sandboxv1.MountConfig, cred *credentials.RuntimeCredential) []map[string]interface{} {
	if cfg == nil || !cfg.Enabled {
		return out
	}
	for _, mount := range cfg.MountPoints {
		item := map[string]interface{}{
			"type":     kind,
			"target":   mount.LocalMountPath,
			"readOnly": mount.ReadOnly,
		}
		switch kind {
		case "ks3":
			item["source"] = mount.BucketName + mount.RemotePath
		case "kpfs":
			item["source"] = mount.FileSystemName + mount.RemotePath
		}
		if cred != nil {
			item["accessKeyId"] = cred.AccessKey
			item["accessKeySecret"] = cred.SecretAccessKey
			if cred.Token != "" {
				item["token"] = cred.Token
			}
		}
		out = append(out, item)
	}
	return out
}

func metaTimeString(value string) *metav1.Time {
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			mt := metav1.NewTime(parsed)
			return &mt
		}
	}
	return nil
}
