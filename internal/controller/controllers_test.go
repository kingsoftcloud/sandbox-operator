package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sandboxv1 "sandbox-operator/api/v1alpha1"
	"sandbox-operator/internal/annotations"
	"sandbox-operator/internal/credentials"
	"sandbox-operator/internal/openapi"
)

func TestSandboxClaimConsumesSandboxIDsAndDoesNotRecreateExpiredChild(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	claim := &sandboxv1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "full-claim",
			Namespace: "sandbox-demo",
			Finalizers: []string{
				ClaimFinalizer,
			},
			Annotations: map[string]string{
				annotations.TemplateID: "template-1",
				annotations.SandboxIDs: annotations.EncodeStringSlice([]string{"sandbox-1"}),
			},
		},
		Spec: sandboxv1.SandboxClaimSpec{
			Replicas:    1,
			TemplateRef: sandboxv1.TemplateReference{Name: "full-template"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(claim).
		WithStatusSubresource(&sandboxv1.SandboxClaim{}).
		Build()
	reconciler := &SandboxClaimReconciler{Client: c, Scheme: scheme}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name}}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	var updatedClaim sandboxv1.SandboxClaim
	if err := c.Get(ctx, request.NamespacedName, &updatedClaim); err != nil {
		t.Fatalf("get claim after first reconcile: %v", err)
	}
	if annotations.Get(updatedClaim.Annotations, annotations.SandboxIDs) != "" {
		t.Fatalf("sandbox-ids annotation should be consumed, got %q", updatedClaim.Annotations[annotations.SandboxIDs])
	}

	childKey := types.NamespacedName{Namespace: claim.Namespace, Name: "full-claim-0"}
	var child sandboxv1.Sandbox
	if err := c.Get(ctx, childKey, &child); err != nil {
		t.Fatalf("expected child sandbox to be created: %v", err)
	}
	if got := annotations.Get(child.Annotations, annotations.SandboxID); got != "sandbox-1" {
		t.Fatalf("unexpected child sandbox-id annotation: %q", got)
	}

	updatedClaim.Status.Desired = 1
	updatedClaim.Status.Created = 1
	updatedClaim.Status.Phase = sandboxv1.PhaseSuccessful
	if err := c.Status().Update(ctx, &updatedClaim); err != nil {
		t.Fatalf("seed materialized claim status: %v", err)
	}
	if err := c.Delete(ctx, &child); err != nil {
		t.Fatalf("delete child sandbox: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}
	if err := c.Get(ctx, childKey, &child); !apierrors.IsNotFound(err) {
		t.Fatalf("expired child sandbox should not be recreated, get err=%v", err)
	}

	if err := c.Get(ctx, request.NamespacedName, &updatedClaim); err != nil {
		t.Fatalf("get claim after second reconcile: %v", err)
	}
	if updatedClaim.Status.Phase != sandboxv1.PhaseSuccessful {
		t.Fatalf("terminal claim phase should remain unchanged, got %q", updatedClaim.Status.Phase)
	}
}

func TestTerminalSandboxClaimDoesNotMaterializeOrMutate(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	claim := &sandboxv1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "terminal-claim",
			Namespace: "sandbox-demo",
			Finalizers: []string{
				ClaimFinalizer,
			},
			Annotations: map[string]string{
				annotations.TemplateID: "template-1",
				annotations.SandboxIDs: annotations.EncodeStringSlice([]string{"sandbox-1"}),
			},
		},
		Spec: sandboxv1.SandboxClaimSpec{
			Replicas:    1,
			TemplateRef: sandboxv1.TemplateReference{Name: "full-template"},
		},
		Status: sandboxv1.SandboxClaimStatus{
			Phase:   sandboxv1.PhaseSuccessful,
			Desired: 1,
			Created: 1,
			Ready:   1,
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(claim).
		WithStatusSubresource(&sandboxv1.SandboxClaim{}).
		Build()
	reconciler := &SandboxClaimReconciler{Client: c, Scheme: scheme}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name}}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("reconcile terminal claim failed: %v", err)
	}

	var child sandboxv1.Sandbox
	childKey := types.NamespacedName{Namespace: claim.Namespace, Name: "terminal-claim-0"}
	if err := c.Get(ctx, childKey, &child); !apierrors.IsNotFound(err) {
		t.Fatalf("terminal claim should not materialize child, get err=%v", err)
	}

	var updatedClaim sandboxv1.SandboxClaim
	if err := c.Get(ctx, request.NamespacedName, &updatedClaim); err != nil {
		t.Fatalf("get terminal claim after reconcile: %v", err)
	}
	if annotations.Get(updatedClaim.Annotations, annotations.SandboxIDs) == "" {
		t.Fatalf("terminal claim should not be mutated")
	}
	if updatedClaim.Status.Phase != sandboxv1.PhaseSuccessful || updatedClaim.Status.Ready != 1 {
		t.Fatalf("terminal claim status should remain unchanged: %#v", updatedClaim.Status)
	}
}

func TestDeletingSandboxClaimDoesNotDeleteClaimedSandboxes(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	now := metav1.Now()
	claim := &sandboxv1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "one-shot",
			Namespace:         "sandbox-demo",
			Finalizers:        []string{ClaimFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: sandboxv1.SandboxClaimSpec{
			Replicas:    1,
			TemplateRef: sandboxv1.TemplateReference{Name: "full-template"},
		},
	}
	child := &sandboxv1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "one-shot-0", Namespace: "sandbox-demo"},
		Spec: sandboxv1.SandboxSpec{
			ClaimRef: &sandboxv1.ClaimReference{Name: "one-shot"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(claim, child).
		WithStatusSubresource(&sandboxv1.SandboxClaim{}, &sandboxv1.Sandbox{}).
		Build()
	reconciler := &SandboxClaimReconciler{Client: c, Scheme: scheme}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name}}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("reconcile deleting claim failed: %v", err)
	}

	var gotChild sandboxv1.Sandbox
	if err := c.Get(ctx, types.NamespacedName{Namespace: child.Namespace, Name: child.Name}, &gotChild); err != nil {
		t.Fatalf("deleting claim should not delete claimed sandbox: %v", err)
	}
	var gotClaim sandboxv1.SandboxClaim
	if err := c.Get(ctx, request.NamespacedName, &gotClaim); !apierrors.IsNotFound(err) {
		t.Fatalf("claim should be removed after finalizer cleanup, get err=%v", err)
	}
}

func TestDeletingSandboxSubmitsOpenAPIDeleteOnlyOnce(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	now := metav1.Now()
	sandbox := &sandboxv1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "delete-once",
			Namespace:         "sandbox-demo",
			DeletionTimestamp: &now,
			Finalizers:        []string{SandboxFinalizer},
			Annotations: map[string]string{
				annotations.SandboxID: "sandbox-1",
			},
		},
	}
	credential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentials.DefaultOpenAPISecretName,
			Namespace: sandbox.Namespace,
		},
		Data: map[string][]byte{
			credentials.KeyAccessKeyID:     []byte("ak"),
			credentials.KeySecretAccessKey: []byte("sk"),
			credentials.KeyRegion:          []byte("cn-beijing-6"),
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sandbox, credential).
		WithStatusSubresource(&sandboxv1.Sandbox{}).
		Build()
	api := &countingOpenAPI{}
	reconciler := &SandboxReconciler{
		Client:      c,
		Credentials: credentials.NewManager(c, credentials.DefaultOpenAPISecretName),
		OpenAPI:     api,
	}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: sandbox.Namespace, Name: sandbox.Name}}

	result, err := reconciler.Reconcile(ctx, request)
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if !result.Requeue || api.deleteSandboxCalls != 1 {
		t.Fatalf("first reconcile should submit exactly one delete and requeue, result=%#v calls=%d", result, api.deleteSandboxCalls)
	}

	var marked sandboxv1.Sandbox
	if err := c.Get(ctx, request.NamespacedName, &marked); err != nil {
		t.Fatalf("get marked sandbox: %v", err)
	}
	if annotations.Get(marked.Annotations, annotations.DeleteRequested) != "true" {
		t.Fatalf("successful delete must be persisted before finalizer removal: %#v", marked.Annotations)
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if api.deleteSandboxCalls != 1 {
		t.Fatalf("second reconcile must not submit another delete, calls=%d", api.deleteSandboxCalls)
	}
}

type countingOpenAPI struct {
	deleteSandboxCalls int
}

func (c *countingOpenAPI) CreateTemplate(context.Context, openapi.Credential, openapi.CreateTemplateRequest) (*openapi.CreateTemplateResponse, error) {
	return nil, nil
}

func (c *countingOpenAPI) UpdateTemplate(context.Context, openapi.Credential, openapi.UpdateTemplateRequest) error {
	return nil
}

func (c *countingOpenAPI) DeleteTemplate(context.Context, openapi.Credential, string) error {
	return nil
}

func (c *countingOpenAPI) GetTemplate(context.Context, openapi.Credential, string) (*openapi.Template, error) {
	return nil, nil
}

func (c *countingOpenAPI) ListTemplates(context.Context, openapi.Credential, openapi.ListTemplatesRequest) (*openapi.TemplateList, error) {
	return nil, nil
}

func (c *countingOpenAPI) StartSandbox(context.Context, openapi.Credential, openapi.StartSandboxRequest) (*openapi.StartSandboxResponse, error) {
	return nil, nil
}

func (c *countingOpenAPI) UpdateSandbox(context.Context, openapi.Credential, openapi.UpdateSandboxRequest) error {
	return nil
}

func (c *countingOpenAPI) DeleteSandbox(_ context.Context, _ openapi.Credential, instanceIDs []string) error {
	if len(instanceIDs) != 1 || instanceIDs[0] != "sandbox-1" {
		return apierrors.NewBadRequest("unexpected sandbox ids")
	}
	c.deleteSandboxCalls++
	return nil
}

func (c *countingOpenAPI) GetSandbox(context.Context, openapi.Credential, string) (*openapi.Sandbox, error) {
	return nil, nil
}

func (c *countingOpenAPI) ListSandboxes(context.Context, openapi.Credential, openapi.ListSandboxesRequest) (*openapi.SandboxList, error) {
	return nil, nil
}
