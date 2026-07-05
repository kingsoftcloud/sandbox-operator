package controller

import "testing"

func TestExternalResourceNamePrefersValidKubernetesName(t *testing.T) {
	if got := externalResourceName("custom-app", "efa5fb6f-d260-4045-a71e-11bb0c8c8619"); got != "custom-app" {
		t.Fatalf("externalResourceName valid name = %q", got)
	}
}

func TestExternalResourceNameFallsBackToIDForInvalidName(t *testing.T) {
	id := "efa5fb6f-d260-4045-a71e-11bb0c8c8619"
	for _, name := range []string{"Custom-App", "", "bad_name"} {
		if got := externalResourceName(name, id); got != id {
			t.Fatalf("externalResourceName(%q) = %q, want %q", name, got, id)
		}
	}
}

func TestExternalTemplateResourceNameUsesTemplateName(t *testing.T) {
	id := "efa5fb6f-d260-4045-a71e-11bb0c8c8619"
	if got := externalTemplateResourceName("custom-app", id); got != "custom-app" {
		t.Fatalf("externalTemplateResourceName valid name = %q", got)
	}
	if got := externalTemplateResourceName("Custom_App", id); got != "custom-app" {
		t.Fatalf("externalTemplateResourceName should sanitize template name, got %q", got)
	}
	if got := externalTemplateResourceName("", id); got != id {
		t.Fatalf("externalTemplateResourceName empty name = %q, want %q", got, id)
	}
}
