package openapi

import (
	"encoding/json"
	"testing"
)

func TestRequestJSONFieldsMatchOpenAPISource(t *testing.T) {
	updateBody, err := json.Marshal(UpdateTemplateRequest{TemplateID: "tpl-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonFieldEquals(updateBody, "TemplateId", "tpl-1") {
		t.Fatalf("UpdateTemplateRequest must use TemplateId, got %s", string(updateBody))
	}
	if jsonFieldEquals(updateBody, "templateId", "tpl-1") {
		t.Fatalf("UpdateTemplateRequest must not use templateId, got %s", string(updateBody))
	}

	listBody, err := json.Marshal(ListTemplatesRequest{PageNum: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonNumberEquals(listBody, "PageNum", 1) || !jsonNumberEquals(listBody, "PageSize", 100) {
		t.Fatalf("ListTemplatesRequest must use PageNum/PageSize, got %s", string(listBody))
	}

	sandboxListBody, err := json.Marshal(ListSandboxesRequest{State: "PAUSED", PageNum: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonFieldEquals(sandboxListBody, "State", "PAUSED") {
		t.Fatalf("ListSandboxesRequest must include State, got %s", string(sandboxListBody))
	}

	startBody, err := json.Marshal(StartSandboxRequest{TemplateID: "tpl-1", Envs: []Env{{Key: "APP_ENV", Value: "prod"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonFieldEquals(startBody, "TemplateId", "tpl-1") {
		t.Fatalf("StartSandboxRequest must use TemplateId, got %s", string(startBody))
	}
	if !jsonArrayFieldExists(startBody, "Envs") {
		t.Fatalf("StartSandboxRequest must use Envs array, got %s", string(startBody))
	}

	updateSandboxBody, err := json.Marshal(UpdateSandboxRequest{InstanceID: "ins-1", Timeout: 3600})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonFieldEquals(updateSandboxBody, "InstanceId", "ins-1") || !jsonNumberEquals(updateSandboxBody, "Timeout", 3600) {
		t.Fatalf("UpdateSandboxRequest must use InstanceId/Timeout, got %s", string(updateSandboxBody))
	}
}

func TestIDResponseNormalization(t *testing.T) {
	if got := (Template{TemplateID: "tpl-1"}).Identifier(); got != "tpl-1" {
		t.Fatalf("Template.Identifier() = %q", got)
	}
	if got := (Sandbox{InstanceID: "ins-1"}).Identifier(); got != "ins-1" {
		t.Fatalf("Sandbox.Identifier() = %q", got)
	}
	if got := (Sandbox{SandboxName: "sandbox-a"}).Name(); got != "sandbox-a" {
		t.Fatalf("Sandbox.Name() = %q", got)
	}
	if got := (Sandbox{Alias: "sandbox-alias"}).Name(); got != "sandbox-alias" {
		t.Fatalf("Sandbox.Name() fallback = %q", got)
	}
	if got := (StartSandboxResponse{InstanceID: "ins-1", TemplateID: "tpl-1"}).TemplateIdentifier(); got != "tpl-1" {
		t.Fatalf("StartSandboxResponse.TemplateIdentifier() = %q", got)
	}
}

func TestTemplateSourceResponseShape(t *testing.T) {
	var template Template
	raw := []byte(`{
		"TemplateId":"tpl-1",
		"TemplateName":"custom-app",
		"TemplateCategory":"Private",
		"TemplateType":"CUSTOM",
		"Cpu":2,
		"Memory":4,
		"Envs":[{"Key":"APP_ENV","Value":"prod"}],
		"ImageConfig":{"ImageSource":"Public","ImageUrl":"hub.kce.ksyun.com/sandbox/aio","ImageTag":"v1"},
		"PreheatConfig":{"PreheatEnable":true,"PreheatNumber":1,"PreheatedInstanceNumber":1}
	}`)
	if err := json.Unmarshal(raw, &template); err != nil {
		t.Fatal(err)
	}
	if template.Identifier() != "tpl-1" || template.ImageURL() != "hub.kce.ksyun.com/sandbox/aio:v1" || template.TargetPoolSize() != 1 {
		t.Fatalf("unexpected template decode: %#v", template)
	}
	if len(template.Envs) != 1 || template.Envs[0].Key != "APP_ENV" || template.Envs[0].Value != "prod" {
		t.Fatalf("env decode failed: %#v", template.Envs)
	}
}

func TestSandboxSourceResponseShape(t *testing.T) {
	var sandbox Sandbox
	raw := []byte(`{
		"InstanceId":"ins-1",
		"TemplateId":"tpl-1",
		"Domain":"https://domain.example.com",
		"Endpoint":"https://endpoint.example.com",
		"Urls":{"CodeUrl":"https://code.example.com","TerminalUrl":"https://terminal.example.com"},
		"SdnsUrls":{"app":"https://sdns.example.com"},
		"CustomConfiguration":{"ImageUrl":"hub.kce.ksyun.com/sandbox/aio:v1","Port":8000,"Command":"/entrypoint.sh"},
		"Envs":[{"Key":"APP_ENV","Value":"prod"}],
		"Ks3MountConfig":{"Ks3Enable":true,"Ks3MountPoints":[{"BucketName":"bucket-a","RemotePath":"/datasets","LocalMountPath":"/mnt/ks3","ReadOnly":true}]}
	}`)
	if err := json.Unmarshal(raw, &sandbox); err != nil {
		t.Fatal(err)
	}
	if sandbox.Identifier() != "ins-1" || sandbox.TemplateIdentifier() != "tpl-1" {
		t.Fatalf("unexpected sandbox identifiers: %#v", sandbox)
	}
	if sandbox.URLs == nil || sandbox.URLs.Code != "https://code.example.com" || sandbox.URLs.TerminalURL != "https://terminal.example.com" {
		t.Fatalf("urls decode failed: %#v", sandbox.URLs)
	}
	if sandbox.SdnsURLs["app"] != "https://sdns.example.com" {
		t.Fatalf("sdnsUrls decode failed: %#v", sandbox.SdnsURLs)
	}
	if sandbox.CustomConfiguration == nil || sandbox.CustomConfiguration.ImageURL == "" || sandbox.CustomConfiguration.Port != 8000 {
		t.Fatalf("customConfiguration decode failed: %#v", sandbox.CustomConfiguration)
	}
	if len(sandbox.Envs) != 1 || len(sandbox.KS3MountConfig.Points()) != 1 {
		t.Fatalf("runtime detail decode failed: %#v", sandbox)
	}
}

func jsonFieldEquals(raw []byte, key, expected string) bool {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	return obj[key] == expected
}

func jsonNumberEquals(raw []byte, key string, expected float64) bool {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	return obj[key] == expected
}

func jsonArrayFieldExists(raw []byte, key string) bool {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	_, ok := obj[key].([]any)
	return ok
}
