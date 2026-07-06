package config

import "testing"

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		wantErr bool
	}{
		{name: "default template", tmpl: "{year}/{date}/{title}.{ext}"},
		{name: "all tokens", tmpl: "{year}/{month}/{date}/{id}-{title}.{ext}"},
		{name: "no tokens", tmpl: "static/path.bin"},
		{name: "empty", tmpl: ""},
		{name: "unknown token", tmpl: "{bogus}/{title}.{ext}", wantErr: true},
		{name: "unknown token mixed with valid", tmpl: "{year}/{bogus}", wantErr: true},
		{name: "unterminated token", tmpl: "{year}/{date", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplate(tt.tmpl)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateTemplate(%q): expected error, got nil", tt.tmpl)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateTemplate(%q): unexpected error: %v", tt.tmpl, err)
			}
		})
	}
}
