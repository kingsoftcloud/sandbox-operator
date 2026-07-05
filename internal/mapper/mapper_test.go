package mapper

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	sandboxv1 "sandbox-operator/api/v1alpha1"
	"sandbox-operator/internal/credentials"
	"sandbox-operator/internal/openapi"
)

func TestTemplateTypeMapping(t *testing.T) {
	cases := map[string]string{
		"AIO":             "AIO",
		"aio":             "AIO",
		"BROWSER":         "browser",
		"browser":         "browser",
		"CODE":            "code",
		"CodeInterpreter": "code",
		"CUSTOM":          "custom",
		"custom":          "custom",
	}
	for in, want := range cases {
		if got := templateType(in); got != want {
			t.Fatalf("templateType(%q) = %q, want %q", in, got, want)
		}
	}

	displayCases := map[string]string{
		"All-in-one":      "AIO",
		"AIO":             "AIO",
		"browser":         "BROWSER",
		"Browser":         "BROWSER",
		"code":            "CODE",
		"CodeInterpreter": "CODE",
		"custom":          "CUSTOM",
		"Custom":          "CUSTOM",
	}
	for in, want := range displayCases {
		if got := displayTemplateType(in); got != want {
			t.Fatalf("displayTemplateType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyTemplateSpecDisplaysUppercaseType(t *testing.T) {
	var obj sandboxv1.SandboxTemplate
	ApplyTemplateSpecFromOpenAPI(&obj, openapi.Template{TemplateType: "browser", TemplateCategory: "Private"})
	if obj.Spec.Type != "BROWSER" {
		t.Fatalf("synced template type = %q, want BROWSER", obj.Spec.Type)
	}
}

func TestTemplateUpdateRequestFromDiffOnlySendsChangedTopLevelFields(t *testing.T) {
	old := templateWithKS3("old description", "/mnt/old")
	next := templateWithKS3("old description", "/mnt/old")
	next.Spec.Description = "new description"

	req := TemplateUpdateRequestFromDiff(next, old, RuntimeCredentials{})
	if req.Description != "new description" {
		t.Fatalf("description diff was not included: %#v", req)
	}
	if req.KS3MountConfig != nil || req.AccessKey != "" || req.SecretAccessKey != "" {
		t.Fatalf("unchanged mount config should not be included: %#v", req)
	}

	changedMount := templateWithKS3("old description", "/mnt/new")
	req = TemplateUpdateRequestFromDiff(changedMount, old, RuntimeCredentials{
		KS3ByName: map[string]*credentials.RuntimeCredential{
			"ks3-credential": {AccessKey: "ak", SecretAccessKey: "sk"},
		},
	})
	if req.KS3MountConfig == nil || !req.KS3MountConfig.EnableKS3 {
		t.Fatalf("changed mount config should be included: %#v", req)
	}
	if req.AccessKey != "ak" || req.SecretAccessKey != "sk" {
		t.Fatalf("mount credential should be included, got ak=%q sk=%q", req.AccessKey, req.SecretAccessKey)
	}

	deletedMount := templateWithKS3("old description", "/mnt/old")
	deletedMount.Spec.Template.Spec.Volumes = nil
	req = TemplateUpdateRequestFromDiff(deletedMount, old, RuntimeCredentials{})
	if req.KS3MountConfig == nil || req.KS3MountConfig.EnableKS3 {
		t.Fatalf("deleted KS3 mount must send Ks3Enable=false: %#v", req.KS3MountConfig)
	}
	if req.KPFSMountConfig == nil || req.KPFSMountConfig.EnableKPFS {
		t.Fatalf("mount diff must include KpfsEnable=false target state: %#v", req.KPFSMountConfig)
	}
}

func TestTemplateUpdateRequestFromDiffSendsDiskAsKecConfig(t *testing.T) {
	old := templateWithKS3("old description", "/mnt/old")
	next := templateWithKS3("old description", "/mnt/old")
	next.Spec.Template.Spec.Resources.Disk = resource.MustParse("80Gi")

	req := TemplateUpdateRequestFromDiff(next, old, RuntimeCredentials{})
	if req.KecConfig == nil || !req.KecConfig.Enabled || req.KecConfig.SystemDiskSizeGB != 80 {
		t.Fatalf("disk diff should send KecConfig with SystemDiskSize=80Gi, got %#v", req.KecConfig)
	}

	err := CompleteTemplateUpdateRequestFromRemote(&req, openapi.Template{KecConfig: &openapi.KecConfig{
		InstanceType:     "N3.2B",
		SystemDiskType:   "ESSD_PL0",
		SystemDiskSizeGB: 40,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if req.KecConfig.InstanceType != "N3.2B" || req.KecConfig.SystemDiskType != "ESSD_PL0" || req.KecConfig.SystemDiskSizeGB != 80 {
		t.Fatalf("remote KecConfig completion failed: %#v", req.KecConfig)
	}
}

func templateWithKS3(description, mountPath string) *sandboxv1.SandboxTemplate {
	memory := resource.MustParse("4Gi")
	return &sandboxv1.SandboxTemplate{
		Spec: sandboxv1.SandboxTemplateSpec{
			Description: description,
			Access:      "Private",
			Type:        "CUSTOM",
			Template: &sandboxv1.RuntimeTemplate{Spec: sandboxv1.RuntimeTemplateSpec{
				Resources: &sandboxv1.RuntimeResourceSpec{CPU: "2", Memory: memory},
				Volumes: []sandboxv1.TemplateVolume{{
					Name:      "ks3-data",
					Type:      "KS3",
					MountPath: mountPath,
					KS3: &sandboxv1.KS3VolumeSource{
						Bucket: "bucket-a",
						Path:   "/datasets",
						CredentialRef: &sandboxv1.LocalObjectReference{
							Name: "ks3-credential",
						},
					},
				}},
			}},
		},
	}
}
