package cli

import (
	"testing"

	"github.com/shhac/agent-vercel/internal/credential"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
)

const kcPlaceholder = "__KEYCHAIN__"

func credsWith(defaultAuth string, auths ...credential.Auth) *credential.Credentials {
	return &credential.Credentials{DefaultAuth: defaultAuth, Auths: auths}
}

func TestResolveTokenPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		flagAuth  string
		envToken  string
		creds     *credential.Credentials
		wantToken string
		wantLabel string // expected returned *Auth label ("" = nil auth)
	}{
		{
			name:      "flag --auth wins over env and default",
			flagAuth:  "work",
			envToken:  "env-tok",
			creds:     credsWith("personal", credential.Auth{Label: "work", Secret: "work-tok"}, credential.Auth{Label: "personal", Secret: "personal-tok"}),
			wantToken: "work-tok",
			wantLabel: "work",
		},
		{
			name:      "env wins over stored default when no --auth",
			envToken:  "env-tok",
			creds:     credsWith("personal", credential.Auth{Label: "personal", Secret: "personal-tok"}),
			wantToken: "env-tok",
			wantLabel: "", // env token has no backing credential
		},
		{
			name:      "stored default used when no --auth and no env",
			envToken:  "",
			creds:     credsWith("personal", credential.Auth{Label: "personal", Secret: "personal-tok"}),
			wantToken: "personal-tok",
			wantLabel: "personal",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VERCEL_TOKEN", tc.envToken)
			tok, auth, err := resolveToken(&GlobalFlags{Auth: tc.flagAuth}, tc.creds)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tok != tc.wantToken {
				t.Fatalf("token = %q; want %q", tok, tc.wantToken)
			}
			switch {
			case tc.wantLabel == "" && auth != nil:
				t.Fatalf("expected nil auth, got %q", auth.Label)
			case tc.wantLabel != "" && (auth == nil || auth.Label != tc.wantLabel):
				t.Fatalf("auth = %v; want label %q", auth, tc.wantLabel)
			}
		})
	}
}

func TestResolveTokenErrors(t *testing.T) {
	tests := []struct {
		name        string
		flagAuth    string
		envToken    string
		creds       *credential.Credentials
		wantFixable agenterrors.FixableBy
	}{
		{
			name:        "unknown --auth label is agent error",
			flagAuth:    "ghost",
			creds:       credsWith("personal", credential.Auth{Label: "personal", Secret: "t"}),
			wantFixable: agenterrors.FixableByAgent,
		},
		{
			name:        "--auth with placeholder secret is human error",
			flagAuth:    "work",
			creds:       credsWith("work", credential.Auth{Label: "work", Secret: kcPlaceholder}),
			wantFixable: agenterrors.FixableByHuman,
		},
		{
			name:        "default with placeholder secret is human error",
			creds:       credsWith("work", credential.Auth{Label: "work", Secret: kcPlaceholder}),
			wantFixable: agenterrors.FixableByHuman,
		},
		{
			name:        "no credential configured is human error",
			creds:       credsWith(""),
			wantFixable: agenterrors.FixableByHuman,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VERCEL_TOKEN", tc.envToken)
			_, _, err := resolveToken(&GlobalFlags{Auth: tc.flagAuth}, tc.creds)
			if err == nil {
				t.Fatal("expected error")
			}
			var aerr *agenterrors.APIError
			if !agenterrors.As(err, &aerr) || aerr.FixableBy != tc.wantFixable {
				t.Fatalf("err = %v; want fixable_by %q", err, tc.wantFixable)
			}
		})
	}
}
