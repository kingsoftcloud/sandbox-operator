package openapi

import (
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

func TestResponseErrorParsesTopLevelCode(t *testing.T) {
	code, message := responseError(baseResponse{}, []byte(`{"Code":"TemplateNotFound","Message":"模板不存在"}`))
	if code != "TemplateNotFound" || message != "模板不存在" {
		t.Fatalf("unexpected parsed error: code=%q message=%q", code, message)
	}
}
