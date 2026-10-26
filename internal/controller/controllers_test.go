package controller

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestDeletingSandboxWaitsForDetailEndpointToDisappear(t *testing.T) {
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
			Name:              "delete-with-lag",
			Namespace:         "sandbox-demo",
			DeletionTimestamp: &now,
			Finalizers:        []string{SandboxFinalizer},
			Annotations: map[string]string{
				annotations.SandboxID: "sandbox-1",
			},
		},
	}
	credential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: credentials.DefaultOpenAPISecretName, Namespace: sandbox.Namespace},
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
	api := &eventuallyDeletedSandboxOpenAPI{detailVisible: true}
	reconciler := &SandboxReconciler{
		Client:      c,
		Credentials: credentials.NewManager(c, credentials.DefaultOpenAPISecretName),
		OpenAPI:     api,
	}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: sandbox.Namespace, Name: sandbox.Name}}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("submit delete: %v", err)
	}
	result, err := reconciler.Reconcile(ctx, request)
	if err != nil {
		t.Fatalf("wait for delayed detail endpoint: %v", err)
	}
	if result.RequeueAfter != FastRequeue || api.deleteSandboxCalls != 1 {
		t.Fatalf("must wait without another delete, result=%#v calls=%d", result, api.deleteSandboxCalls)
	}

	api.detailVisible = false
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("complete deletion after detail disappears: %v", err)
	}
	var deleted sandboxv1.Sandbox
	if err := c.Get(ctx, request.NamespacedName, &deleted); !apierrors.IsNotFound(err) {
		t.Fatalf("sandbox should be removed after detail disappears, get err=%v", err)
	}
	if api.deleteSandboxCalls != 1 {
		t.Fatalf("detail lag must not repeat external delete, calls=%d", api.deleteSandboxCalls)
	}
}

func TestDeletingSandboxRetriesStaleConfirmedDelete(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	deletedAt := metav1.NewTime(time.Now().Add(-2 * SandboxDeleteRetryInterval))
	sandbox := &sandboxv1.Sandbox{ObjectMeta: metav1.ObjectMeta{
		Name: "stale-delete", Namespace: "sandbox-demo", DeletionTimestamp: &deletedAt,
		Finalizers: []string{SandboxFinalizer}, Annotations: map[string]string{
			annotations.SandboxID: "sandbox-1", annotations.DeleteRequested: "true",
		},
	}}
	credential := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: credentials.DefaultOpenAPISecretName, Namespace: sandbox.Namespace},
		Data: map[string][]byte{
			credentials.KeyAccessKeyID: []byte("ak"), credentials.KeySecretAccessKey: []byte("sk"), credentials.KeyRegion: []byte("cn-beijing-6"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sandbox, credential).Build()
	api := &eventuallyDeletedSandboxOpenAPI{detailVisible: true}
	r := &SandboxReconciler{Client: c, Credentials: credentials.NewManager(c, credentials.DefaultOpenAPISecretName), OpenAPI: api}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: sandbox.Namespace, Name: sandbox.Name}}
	if _, err := r.Reconcile(ctx, request); err != nil || api.deleteSandboxCalls != 1 {
		t.Fatalf("stale deletion must be retried once: err=%v calls=%d", err, api.deleteSandboxCalls)
	}
	if _, err := r.Reconcile(ctx, request); err != nil || api.deleteSandboxCalls != 1 {
		t.Fatalf("deletion retry must be rate limited: err=%v calls=%d", err, api.deleteSandboxCalls)
	}
	api.detailVisible = false
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatalf("finalize after remote disappears: %v", err)
	}
	var deleted sandboxv1.Sandbox
	if err := c.Get(ctx, request.NamespacedName, &deleted); !apierrors.IsNotFound(err) {
		t.Fatalf("sandbox should be finalized, get err=%v", err)
	}
}

func TestMissingSandboxDoesNotSubmitAnotherDelete(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sandbox := &sandboxv1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "missing-sandbox",
			Namespace:  "sandbox-demo",
			Finalizers: []string{SandboxFinalizer},
			Annotations: map[string]string{
				annotations.SandboxID: "sandbox-1",
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sandbox).Build()
	api := &countingOpenAPI{}
	reconciler := &SandboxReconciler{Client: c, OpenAPI: api}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: sandbox.Namespace, Name: sandbox.Name}}

	if err := reconciler.handleMissingSandbox(ctx, sandbox); err != nil {
		t.Fatalf("handle missing sandbox: %v", err)
	}
	var terminating sandboxv1.Sandbox
	if err := c.Get(ctx, request.NamespacedName, &terminating); err != nil {
		t.Fatalf("get terminating sandbox: %v", err)
	}
	if annotations.Get(terminating.Annotations, annotations.DeleteRequested) != "true" {
		t.Fatalf("missing sandbox must skip remote deletion: %#v", terminating.Annotations)
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("reconcile terminating sandbox: %v", err)
	}
	if api.deleteSandboxCalls != 0 {
		t.Fatalf("a sandbox already absent from OpenAPI must not be deleted again, calls=%d", api.deleteSandboxCalls)
	}
}

func TestPollerDoesNotAdoptStaleSandboxListItem(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	api := &staleSandboxOpenAPI{}
	poller := &Poller{Client: c, OpenAPI: api, AdoptExternal: true}
	if err := poller.syncSandboxes(ctx, "sandbox-demo", openapi.Credential{}); err != nil {
		t.Fatalf("sync stale sandbox list: %v", err)
	}
	if api.getSandboxCalls != 1 {
		t.Fatalf("stale sandbox list item must be verified once, calls=%d", api.getSandboxCalls)
	}
	var sandboxes sandboxv1.SandboxList
	if err := c.List(ctx, &sandboxes); err != nil {
		t.Fatal(err)
	}
	if len(sandboxes.Items) != 0 {
		t.Fatalf("stale list item must not be adopted: %#v", sandboxes.Items)
	}
}

func TestPollerContinuesAdoptionAfterSandboxDetailFailure(t *testing.T) {
	for _, oldStillListed := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "still-listed"}[oldStillListed], func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			if err := sandboxv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			old := &sandboxv1.Sandbox{ObjectMeta: metav1.ObjectMeta{
				Name: "old", Namespace: "sandbox-demo", Finalizers: []string{SandboxFinalizer},
				Annotations: map[string]string{annotations.SandboxID: "old-id"},
			}}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(old).WithStatusSubresource(&sandboxv1.Sandbox{}).Build()
			api := &sandboxDetailFailureOpenAPI{oldStillListed: oldStillListed}
			p := &Poller{Client: c, OpenAPI: api, AdoptExternal: true}
			err := p.syncSandboxes(ctx, old.Namespace, openapi.Credential{})
			if oldStillListed && err == nil {
				t.Fatal("unresolved detail failure must be reported")
			}
			if !oldStillListed && err != nil {
				t.Fatalf("confirmed missing sandbox: %v", err)
			}
			var adopted sandboxv1.Sandbox
			if err := c.Get(ctx, types.NamespacedName{Namespace: old.Namespace, Name: "new"}, &adopted); err != nil {
				t.Fatalf("new sandbox must be adopted: %v", err)
			}
			var existing sandboxv1.Sandbox
			if err := c.Get(ctx, types.NamespacedName{Namespace: old.Namespace, Name: old.Name}, &existing); err != nil {
				t.Fatal(err)
			}
			if got := annotations.Get(existing.Annotations, annotations.DeleteRequested) == "true"; got != !oldStillListed {
				t.Fatalf("unexpected delete-requested=%v", got)
			}
		})
	}
}

func TestSandboxDeleteBarrierConfirmsMaskedNotFoundWithList(t *testing.T) {
	ctx := context.Background()
	r := &SandboxReconciler{OpenAPI: &sandboxDetailFailureOpenAPI{}}
	obj := &sandboxv1.Sandbox{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{annotations.SandboxID: "old-id"},
	}}
	// A credential manager is required for the real deletion barrier; exercise
	// the list cross-check through a fake secret in the same namespace.
	obj.Namespace = "sandbox-demo"
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: credentials.DefaultOpenAPISecretName, Namespace: obj.Namespace}, Data: map[string][]byte{
		credentials.KeyAccessKeyID: []byte("ak"), credentials.KeySecretAccessKey: []byte("sk"), credentials.KeyRegion: []byte("cn-beijing-6"),
	}}
	r.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	r.Credentials = credentials.NewManager(r.Client, credentials.DefaultOpenAPISecretName)
	deleted, err := r.sandboxDeletedFromOpenAPI(ctx, obj)
	if err != nil || !deleted {
		t.Fatalf("missing sandbox should clear deletion barrier: deleted=%v err=%v", deleted, err)
	}
	r.OpenAPI = &sandboxDetailFailureOpenAPI{oldStillListed: true}
	deleted, err = r.sandboxDeletedFromOpenAPI(ctx, obj)
	if err == nil || deleted {
		t.Fatalf("listed sandbox must keep deletion barrier: deleted=%v err=%v", deleted, err)
	}
}

func TestPollerKeepsSandboxWhenListFails(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := sandboxv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	old := &sandboxv1.Sandbox{ObjectMeta: metav1.ObjectMeta{
		Name: "old", Namespace: "sandbox-demo", Finalizers: []string{SandboxFinalizer},
		Annotations: map[string]string{annotations.SandboxID: "old-id"},
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(old).WithStatusSubresource(&sandboxv1.Sandbox{}).Build()
	p := &Poller{Client: c, OpenAPI: &sandboxDetailFailureOpenAPI{listErr: errors.New("list unavailable")}, AdoptExternal: true}
	if err := p.syncSandboxes(ctx, old.Namespace, openapi.Credential{}); err == nil {
		t.Fatal("list failure must be reported")
	}
	var current sandboxv1.Sandbox
	if err := c.Get(ctx, types.NamespacedName{Namespace: old.Namespace, Name: old.Name}, &current); err != nil {
		t.Fatal(err)
	}
	if annotations.Get(current.Annotations, annotations.DeleteRequested) != "" {
		t.Fatal("list failure must not cause sandbox deletion")
	}
}

type sandboxDetailFailureOpenAPI struct {
	countingOpenAPI
	oldStillListed bool
	listErr        error
}

func (c *sandboxDetailFailureOpenAPI) GetSandbox(_ context.Context, _ openapi.Credential, id string) (*openapi.Sandbox, error) {
	if id == "old-id" {
		return nil, &openapi.APIError{Action: "GetSandboxInstance", StatusCode: 400, Code: "SandboxServiceError"}
	}
	return &openapi.Sandbox{InstanceID: id, SandboxName: "new", TemplateID: "template-1", Status: "RUNNING"}, nil
}

func (c *sandboxDetailFailureOpenAPI) ListSandboxes(_ context.Context, _ openapi.Credential, req openapi.ListSandboxesRequest) (*openapi.SandboxList, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	if req.State != "" {
		return &openapi.SandboxList{}, nil
	}
	items := []openapi.Sandbox{{InstanceID: "new-id", SandboxName: "new", TemplateID: "template-1", Status: "RUNNING"}}
	if c.oldStillListed {
		items = append(items, openapi.Sandbox{InstanceID: "old-id"})
	}
	return &openapi.SandboxList{Items: items, Total: len(items)}, nil
}

type countingOpenAPI struct {
	deleteSandboxCalls int
}

type staleSandboxOpenAPI struct {
	countingOpenAPI
	getSandboxCalls int
}

type eventuallyDeletedSandboxOpenAPI struct {
	countingOpenAPI
	detailVisible bool
}

func (c *eventuallyDeletedSandboxOpenAPI) GetSandbox(context.Context, openapi.Credential, string) (*openapi.Sandbox, error) {
	if c.detailVisible {
		return &openapi.Sandbox{InstanceID: "sandbox-1"}, nil
	}
	return nil, &openapi.APIError{StatusCode: 400, Code: "SandboxNotFound", Message: "sandbox deleted"}
}

func (c *staleSandboxOpenAPI) GetSandbox(context.Context, openapi.Credential, string) (*openapi.Sandbox, error) {
	c.getSandboxCalls++
	return nil, &openapi.APIError{StatusCode: 400, Code: "SandboxNotFound", Message: "sandbox deleted"}
}

func (c *staleSandboxOpenAPI) ListSandboxes(context.Context, openapi.Credential, openapi.ListSandboxesRequest) (*openapi.SandboxList, error) {
	return &openapi.SandboxList{
		Items: []openapi.Sandbox{{InstanceID: "stale-1", SandboxName: "stale-sandbox"}},
		Total: 1,
	}, nil
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

func (c *countingOpenAPI) CreateSandboxImageCommit(context.Context, openapi.Credential, openapi.CreateSandboxImageCommitRequest) (*openapi.CreateSandboxImageCommitResponse, error) {
	return nil, nil
}

func (c *countingOpenAPI) GetSandboxImageCommit(context.Context, openapi.Credential, string, string) (*openapi.SandboxImageCommit, error) {
	return nil, nil
}
