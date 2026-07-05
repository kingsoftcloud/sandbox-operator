package controller

import (
	"context"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sandboxv1 "sandbox-operator/api/v1alpha1"
	"sandbox-operator/internal/annotations"
	"sandbox-operator/internal/credentials"
	"sandbox-operator/internal/mapper"
	"sandbox-operator/internal/openapi"
	statusutil "sandbox-operator/internal/status"
)

type Poller struct {
	Client                  client.Client
	Credentials             *credentials.Manager
	OpenAPI                 openapi.Interface
	Interval                time.Duration
	PageSize                int
	MaxConcurrentNamespaces int
	AdoptExternal           bool
	SyncNamespaces          []string
}

func (p *Poller) Run(ctx context.Context) error {
	if p.Interval <= 0 {
		p.Interval = 30 * time.Second
	}
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		if err := p.SyncAll(ctx); err != nil {
			log.FromContext(ctx).Error(err, "sandbox poller sync failed")
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Poller) Start(ctx context.Context) error {
	return p.Run(ctx)
}

func (p *Poller) SyncAll(ctx context.Context) error {
	namespaces := p.SyncNamespaces
	if len(namespaces) == 0 {
		var list corev1.NamespaceList
		if err := p.Client.List(ctx, &list); err != nil {
			return err
		}
		namespaces = make([]string, 0, len(list.Items))
		for _, ns := range list.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}
	limit := p.MaxConcurrentNamespaces
	if limit <= 0 {
		limit = 5
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, namespace := range namespaces {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if err := p.SyncNamespace(ctx, namespace); err != nil {
				log.FromContext(ctx).Error(err, "sync namespace failed", "namespace", namespace)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func (p *Poller) SyncNamespace(ctx context.Context, namespace string) error {
	logger := log.FromContext(ctx).WithValues("namespace", namespace)
	cred, err := p.Credentials.GetOpenAPI(ctx, namespace, nil)
	if err != nil {
		if credentials.IsOpenAPICredentialNotFound(err) {
			logger.V(1).Info("skip namespace without openapi credential")
			return nil
		}
		logger.Error(err, "openapi credential unavailable")
		return err
	}
	logger.V(1).Info("sync namespace with openapi credential", "secret", cred.SecretName)
	openapiCred := mapper.OpenAPICredential(cred)
	var firstErr error
	if err := p.syncTemplates(ctx, namespace, openapiCred); err != nil {
		firstErr = err
	}
	if err := p.syncSandboxes(ctx, namespace, openapiCred); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (p *Poller) syncTemplates(ctx context.Context, namespace string, cred openapi.Credential) error {
	logger := log.FromContext(ctx).WithValues("namespace", namespace)
	var local sandboxv1.SandboxTemplateList
	if err := p.Client.List(ctx, &local, client.InNamespace(namespace)); err != nil {
		return err
	}
	knownIDs := map[string]bool{}
	knownNames := map[string]bool{}

	for i := range local.Items {
		obj := &local.Items[i]
		knownNames[obj.Name] = true
		statusBefore := cloneForCompare(obj.Status)
		templateID := annotations.Get(obj.Annotations, annotations.TemplateID)
		if templateID == "" {
			continue
		}
		knownIDs[templateID] = true
		remote, err := p.OpenAPI.GetTemplate(ctx, cred, templateID)
		if err != nil {
			if openapi.IsNotFound(err) {
				if err := p.handleMissingTemplate(ctx, obj); err != nil {
					return err
				}
				continue
			}
			return err
		}
		specBefore := cloneForCompare(obj.Spec)
		mapper.ApplyTemplateSpecFromOpenAPI(obj, *remote)
		if hasChanged(specBefore, obj.Spec) {
			if err := p.Client.Update(ctx, obj); err != nil {
				return ignoreConflict(err)
			}
		}
		mapper.ApplyTemplateStatusFromOpenAPI(obj, *remote)
		statusutil.SetCondition(&obj.Status.Conditions, sandboxv1.ConditionSynced, "True", "OpenAPISynced", "Template has been synced from Sandbox OpenAPI.", obj.Generation)
		if hasChanged(statusBefore, obj.Status) {
			if err := p.Client.Status().Update(ctx, obj); err != nil {
				return ignoreConflict(err)
			}
		}
	}
	if p.AdoptExternal {
		remotes, err := p.listAllTemplates(ctx, cred)
		if err != nil {
			return err
		}
		logger.V(1).Info("listed sandbox templates from openapi", "remoteCount", len(remotes), "localCount", len(local.Items))
		adopted := 0
		for _, remote := range remotes {
			templateID := remote.Identifier()
			if templateID == "" {
				logger.Info("skip openapi template without id", "templateName", remote.TemplateName, "status", remote.Status)
				continue
			}
			if knownIDs[templateID] {
				continue
			}
			detail, err := p.OpenAPI.GetTemplate(ctx, cred, templateID)
			if err != nil {
				if openapi.IsNotFound(err) {
					continue
				}
				return err
			}
			remote = *detail
			name := uniqueName(externalResourceName(remote.TemplateName, templateID), knownNames)
			obj := &sandboxv1.SandboxTemplate{
				TypeMeta: metav1.TypeMeta{APIVersion: sandboxv1.GroupVersion.String(), Kind: "SandboxTemplate"},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
					Labels: map[string]string{
						"sandbox.kce.ksyun.com/adopted": "true",
					},
					Annotations: map[string]string{
						annotations.TemplateID: templateID,
					},
				},
			}
			mapper.ApplyTemplateSpecFromOpenAPI(obj, remote)
			if err := p.Client.Create(ctx, obj); err != nil {
				return err
			}
			mapper.ApplyTemplateStatusFromOpenAPI(obj, remote)
			statusutil.SetCondition(&obj.Status.Conditions, sandboxv1.ConditionSynced, "True", "OpenAPIAdopted", "Template has been adopted from Sandbox OpenAPI.", obj.Generation)
			if err := p.Client.Status().Update(ctx, obj); err != nil {
				return ignoreConflict(err)
			}
			knownIDs[templateID] = true
			knownNames[name] = true
			adopted++
			logger.Info("adopted sandbox template from openapi", "name", name, "templateID", templateID)
		}
		if adopted > 0 {
			logger.Info("adopted sandbox templates from openapi", "count", adopted)
		}
	}
	return nil
}

func (p *Poller) handleMissingTemplate(ctx context.Context, obj *sandboxv1.SandboxTemplate) error {
	return client.IgnoreNotFound(p.Client.Delete(ctx, obj))
}

func (p *Poller) handleMissingSandbox(ctx context.Context, obj *sandboxv1.Sandbox) error {
	return client.IgnoreNotFound(p.Client.Delete(ctx, obj))
}

func (p *Poller) syncSandboxes(ctx context.Context, namespace string, cred openapi.Credential) error {
	logger := log.FromContext(ctx).WithValues("namespace", namespace)
	var local sandboxv1.SandboxList
	if err := p.Client.List(ctx, &local, client.InNamespace(namespace)); err != nil {
		return err
	}
	knownIDs := map[string]bool{}
	knownNames := map[string]bool{}

	for i := range local.Items {
		obj := &local.Items[i]
		knownNames[obj.Name] = true
		statusBefore := cloneForCompare(obj.Status)
		sandboxID := annotations.Get(obj.Annotations, annotations.SandboxID)
		if sandboxID == "" {
			continue
		}
		knownIDs[sandboxID] = true
		remote, err := p.OpenAPI.GetSandbox(ctx, cred, sandboxID)
		if err != nil {
			if openapi.IsNotFound(err) {
				if err := p.handleMissingSandbox(ctx, obj); err != nil {
					return err
				}
				continue
			}
			return err
		}
		specBefore := cloneForCompare(obj.Spec)
		mapper.ApplySandboxSpecFromOpenAPI(obj, *remote)
		if hasChanged(specBefore, obj.Spec) {
			if err := p.Client.Update(ctx, obj); err != nil {
				return ignoreConflict(err)
			}
		}
		mapper.ApplySandboxStatusFromOpenAPI(obj, *remote)
		statusutil.SetCondition(&obj.Status.Conditions, sandboxv1.ConditionSynced, "True", "OpenAPISynced", "Sandbox has been synced from Sandbox OpenAPI.", obj.Generation)
		if hasChanged(statusBefore, obj.Status) {
			if err := p.Client.Status().Update(ctx, obj); err != nil {
				return ignoreConflict(err)
			}
		}
	}
	if p.AdoptExternal {
		remotes, err := p.listAllSandboxes(ctx, cred)
		if err != nil {
			return err
		}
		logger.V(1).Info("listed sandboxes from openapi", "remoteCount", len(remotes), "localCount", len(local.Items))
		adopted := 0
		for _, remote := range remotes {
			sandboxID := remote.Identifier()
			if sandboxID == "" || knownIDs[sandboxID] {
				continue
			}
			name := uniqueName(externalResourceName(remote.Name(), sandboxID), knownNames)
			obj := &sandboxv1.Sandbox{
				TypeMeta: metav1.TypeMeta{APIVersion: sandboxv1.GroupVersion.String(), Kind: "Sandbox"},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
					Labels: map[string]string{
						"sandbox.kce.ksyun.com/adopted": "true",
					},
					Annotations: map[string]string{
						annotations.SandboxID:  sandboxID,
						annotations.TemplateID: remote.TemplateIdentifier(),
					},
				},
				Spec: sandboxv1.SandboxSpec{
					Name:           name,
					TemplateRef:    sandboxv1.TemplateReference{ID: remote.TemplateIdentifier()},
					TimeoutSeconds: remote.Timeout,
				},
			}
			mapper.ApplySandboxSpecFromOpenAPI(obj, remote)
			if err := p.Client.Create(ctx, obj); err != nil {
				return err
			}
			mapper.ApplySandboxStatusFromOpenAPI(obj, remote)
			statusutil.SetCondition(&obj.Status.Conditions, sandboxv1.ConditionSynced, "True", "OpenAPIAdopted", "Sandbox has been adopted from Sandbox OpenAPI.", obj.Generation)
			if err := p.Client.Status().Update(ctx, obj); err != nil {
				return ignoreConflict(err)
			}
			knownIDs[sandboxID] = true
			knownNames[name] = true
			adopted++
			logger.Info("adopted sandbox from openapi", "name", name, "sandboxID", sandboxID)
		}
		if adopted > 0 {
			logger.Info("adopted sandboxes from openapi", "count", adopted)
		}
	}
	return nil
}

func (p *Poller) listAllTemplates(ctx context.Context, cred openapi.Credential) ([]openapi.Template, error) {
	pageSize := p.pageSize()
	page := 1
	var out []openapi.Template
	for {
		list, err := p.OpenAPI.ListTemplates(ctx, cred, openapi.ListTemplatesRequest{PageNum: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		out = append(out, list.Items...)
		if len(list.Items) < pageSize || (list.Total > 0 && len(out) >= list.Total) {
			return out, nil
		}
		page++
	}
}

func (p *Poller) listAllSandboxes(ctx context.Context, cred openapi.Credential) ([]openapi.Sandbox, error) {
	var out []openapi.Sandbox
	seen := map[string]bool{}
	for _, state := range sandboxListStates() {
		items, err := p.listSandboxesByState(ctx, cred, state)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			id := item.Identifier()
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, item)
		}
	}
	return out, nil
}

func (p *Poller) listSandboxesByState(ctx context.Context, cred openapi.Credential, state string) ([]openapi.Sandbox, error) {
	pageSize := p.pageSize()
	page := 1
	var out []openapi.Sandbox
	for {
		list, err := p.OpenAPI.ListSandboxes(ctx, cred, openapi.ListSandboxesRequest{State: state, PageNum: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		out = append(out, list.Items...)
		if len(list.Items) < pageSize || (list.Total > 0 && len(out) >= list.Total) {
			return out, nil
		}
		page++
	}
}

func sandboxListStates() []string {
	return []string{"", "STARTING", "RUNNING", "KILLING", "FAILED", "UNHEALTHY", "PAUSED", "RESUMING"}
}

func (p *Poller) pageSize() int {
	if p.PageSize <= 0 {
		return 100
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

func sanitizeName(value, fallback string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		out = fallback
	}
	if len(out) > 253 {
		out = out[:253]
	}
	return out
}

func uniqueName(base string, used map[string]bool) string {
	if !used[base] {
		return base
	}
	for i := 1; ; i++ {
		suffix := "-" + shortID(time.Now().Format("150405.000000000")) + "-" + string(rune('a'+(i%26)))
		candidate := base
		if len(candidate)+len(suffix) > 253 {
			candidate = candidate[:253-len(suffix)]
		}
		candidate += suffix
		if !used[candidate] {
			return candidate
		}
	}
}

func externalResourceName(preferredName, id string) string {
	if isValidKubernetesName(preferredName) {
		return preferredName
	}
	return sanitizeName(id, "unknown")
}

func isValidKubernetesName(name string) bool {
	if name == "" {
		return false
	}
	return len(validation.IsDNS1123Subdomain(name)) == 0
}

func shortID(id string) string {
	id = sanitizeName(id, "unknown")
	if len(id) <= 12 {
		return id
	}
	return id[len(id)-12:]
}
