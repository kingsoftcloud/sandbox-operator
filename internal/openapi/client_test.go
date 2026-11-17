package openapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestIsNotFoundRecognizesOpenAPICode(t *testing.T) {
	err := &APIError{
		Action:     "GetSandboxTemplate",
		StatusCode: http.StatusBadRequest,
		Code:       "TemplateNotFound",
		Message:    "模板不存在",
	}
	if !IsNotFound(err) {
		t.Fatal("TemplateNotFound should be treated as not found")
	}
	if IsNotFound(errors.New("plain error")) {
		t.Fatal("plain error should not be treated as not found")
	}
}

func TestIsNotFoundRecognizesGatewayInstanceVariants(t *testing.T) {
	tests := []APIError{
		{Action: "GetSandboxInstance", StatusCode: http.StatusBadRequest, Code: "SandboxInstanceNotFound"},
		{Action: "GetSandboxInstance", StatusCode: http.StatusBadRequest, Code: "Instance.NotFound"},
		{Action: "GetSandboxInstance", StatusCode: http.StatusBadRequest, Message: "实例不存在"},
	}
	for _, tt := range tests {
		if !IsNotFound(&tt) {
			t.Fatalf("%+v should be treated as not found", tt)
		}
	}
}

func TestSandboxServiceErrorNeedsListConfirmation(t *testing.T) {
	err := &APIError{Action: "GetSandboxInstance", StatusCode: 400, Code: "SandboxServiceError"}
	if IsNotFound(err) || !IsSandboxServiceError(err) {
		t.Fatal("masked service error must not be treated as not found without list confirmation")
	}
	if IsSandboxServiceError(&APIError{Action: "GetSandboxTemplate", Code: "SandboxServiceError"}) {
		t.Fatal("template errors must not use the sandbox compatibility path")
	}
}

func TestSandboxPhaseTransitionTimeDecodesFromDetailAndList(t *testing.T) {
	var detail Sandbox
	if err := json.Unmarshal([]byte(`{"InstanceId":"id-1","PhaseTransitionTime":"2026-11-17T02:00:00Z"}`), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.PhaseTransitionTime != "2026-11-17T02:00:00Z" {
		t.Fatalf("detail phase transition time: %q", detail.PhaseTransitionTime)
	}
	var list SandboxList
	if err := json.Unmarshal([]byte(`{"InstanceSet":[{"InstanceId":"id-1","PhaseTransitionTime":"2026-11-17T02:00:00Z"}]}`), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].PhaseTransitionTime != detail.PhaseTransitionTime {
		t.Fatalf("list phase transition time: %#v", list.Items)
	}
}

func TestResponseErrorParsesTopLevelCode(t *testing.T) {
	code, message := responseError(baseResponse{}, []byte(`{"Code":"TemplateNotFound","Message":"模板不存在"}`))
	if code != "TemplateNotFound" || message != "模板不存在" {
		t.Fatalf("unexpected parsed error: code=%q message=%q", code, message)
	}
}
