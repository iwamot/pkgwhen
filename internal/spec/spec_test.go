package spec

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		arg     string
		want    Spec
		wantErr string
	}{
		{"pypi:openai-agents", Spec{"pypi", "openai-agents", ""}, ""},
		{"pypi:openai-agents@0.21.0", Spec{"pypi", "openai-agents", "0.21.0"}, ""},
		{"npm:fastq@1.20.3", Spec{"npm", "fastq", "1.20.3"}, ""},
		{"npm:@types/node", Spec{"npm", "@types/node", ""}, ""},
		{"npm:@types/node@22.0.0", Spec{"npm", "@types/node", "22.0.0"}, ""},
		{"github-releases:jdx/aube", Spec{"github-releases", "jdx/aube", ""}, ""},
		{"github-releases:jdx/aube@v2.2.12", Spec{"github-releases", "jdx/aube", "v2.2.12"}, ""},
		{"openai-agents", Spec{}, "want REGISTRY:NAME"},
		{":openai-agents", Spec{}, "want REGISTRY:NAME"},
		{"cargo:serde", Spec{}, `unknown registry "cargo"`},
		{"pypi:", Spec{}, "no package name"},
		{"pypi:openai-agents@", Spec{}, "nothing after @"},
		{"github-releases:aube", Spec{}, "wants OWNER/REPO"},
		{"github-releases:/aube", Spec{}, "wants OWNER/REPO"},
		{"github-releases:jdx/", Spec{}, "wants OWNER/REPO"},
		{"github-releases:jdx/aube/extra", Spec{}, "wants OWNER/REPO"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			got, err := Parse(tt.arg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse err = %v", err)
			}
			if got != tt.want {
				t.Errorf("Parse = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	if got := (Spec{"npm", "@types/node", ""}).String(); got != "npm:@types/node" {
		t.Errorf("String = %q", got)
	}
	if got := (Spec{"npm", "@types/node", "22.0.0"}).String(); got != "npm:@types/node@22.0.0" {
		t.Errorf("String = %q", got)
	}
}
