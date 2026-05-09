package testdata

import (
	"strings"
	"testing"
)

func TestParseInlineReferencesParsesAllShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  InlineReference
	}{
		{
			input: "{{$ds.users.email}}",
			want: InlineReference{
				Expression: "$ds.users.email",
				TableKey:   "users",
				Column:     "email",
			},
		},
		{
			input: "{{$ds.users[role=admin].email}}",
			want: InlineReference{
				Expression:   "$ds.users[role=admin].email",
				TableKey:     "users",
				FilterColumn: "role",
				FilterValue:  "admin",
				Column:       "email",
			},
		},
		{
			input: `{{$ds.users[role="ad min"].email}}`,
			want: InlineReference{
				Expression:   `$ds.users[role="ad min"].email`,
				TableKey:     "users",
				FilterColumn: "role",
				FilterValue:  "ad min",
				Column:       "email",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			refs, err := ParseInlineReferences(tc.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.input, err)
			}
			if len(refs) != 1 {
				t.Fatalf("expected one reference, got %d (%v)", len(refs), refs)
			}
			if refs[0] != tc.want {
				t.Fatalf("expected reference %#v, got %#v", tc.want, refs[0])
			}
		})
	}
}

func TestParseInlineReferencesScansMixedInput(t *testing.T) {
	t.Parallel()

	refs, err := ParseInlineReferences("hello {{$ds.users.email}} {{$mock.uuid}} {{$ds.users[role=admin].email}}")
	if err != nil {
		t.Fatalf("parse mixed: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected two ds refs, got %d (%v)", len(refs), refs)
	}
}

func TestParseInlineReferencesReportsInvalidShape(t *testing.T) {
	t.Parallel()

	cases := []string{
		"{{$ds.users}}",
		"{{$ds.}}",
		"{{$ds.users[].email}}",
		"{{$ds.users[role].email}}",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			refs, err := ParseInlineReferences(in)
			if err == nil {
				t.Fatalf("expected error for %q, got refs=%v", in, refs)
			}
			if !strings.Contains(err.Error(), "invalid test data inline reference") {
				t.Fatalf("expected descriptive error, got %v", err)
			}
		})
	}
}

func TestDedupeReferencesPreservesOrderAndDropsDuplicates(t *testing.T) {
	t.Parallel()

	deduped := DedupeReferences([]InlineReference{
		{Expression: "$ds.users.email"},
		{Expression: ""},
		{Expression: "$ds.users.id"},
		{Expression: "$ds.users.email"},
	})
	if len(deduped) != 2 {
		t.Fatalf("expected two unique references, got %d (%v)", len(deduped), deduped)
	}
	if deduped[0].Expression != "$ds.users.email" || deduped[1].Expression != "$ds.users.id" {
		t.Fatalf("expected order preserved, got %#v", deduped)
	}
}

func TestHasInlineTokenIsCheap(t *testing.T) {
	t.Parallel()

	if !HasInlineToken("prefix {{$ds.foo.bar}}") {
		t.Fatalf("expected token to be detected")
	}
	if HasInlineToken("plain text without ds tokens") {
		t.Fatalf("expected no detection for plain text")
	}
	if HasInlineToken("{{$mock.uuid}}") {
		t.Fatalf("expected no detection for mock token")
	}
}
