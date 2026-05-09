package mockdata

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestExpandReturnsInputWhenNoTokens(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"plain text",
		"{{varName}}",
		"{{sql.userSeed.id}}",
		"{{$steps[1].body.token}}",
		// Has $mock literally but not the full token shape.
		"$mock.uuid",
	}
	for _, in := range cases {
		if got := Expand(in); got != in {
			t.Fatalf("Expand(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestExpandKnownHelpersReplaceToken(t *testing.T) {
	t.Parallel()

	got := Expand("user-{{$mock.uuid}}/req")
	if !strings.HasPrefix(got, "user-") || !strings.HasSuffix(got, "/req") {
		t.Fatalf("expected literal prefix/suffix preserved, got %q", got)
	}
	id := strings.TrimPrefix(strings.TrimSuffix(got, "/req"), "user-")
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("expected valid uuid in expansion, got %q (%v)", id, err)
	}
}

func TestExpandUnknownHelperLeavesTokenIntact(t *testing.T) {
	t.Parallel()

	in := "id={{$mock.unknownHelper}};x={{$mock.uuid}}"
	got := Expand(in)
	if !strings.Contains(got, "{{$mock.unknownHelper}}") {
		t.Fatalf("expected unknown token to be preserved, got %q", got)
	}
	// The known token must still be expanded.
	if strings.Contains(got, "{{$mock.uuid}}") {
		t.Fatalf("expected uuid token to be replaced, got %q", got)
	}
}

func TestExpandIndependentValuesPerOccurrence(t *testing.T) {
	t.Parallel()

	// Two uuid tokens in one string should each receive a fresh value.
	got := Expand("{{$mock.uuid}}|{{$mock.uuid}}")
	parts := strings.Split(got, "|")
	if len(parts) != 2 {
		t.Fatalf("expected two segments, got %q", got)
	}
	if parts[0] == parts[1] {
		t.Fatalf("expected two independent uuids, got duplicates: %q", got)
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		t.Fatalf("first segment is not a uuid: %q (%v)", parts[0], err)
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		t.Fatalf("second segment is not a uuid: %q (%v)", parts[1], err)
	}
}

func TestExpandIntHelperHonoursRange(t *testing.T) {
	t.Parallel()

	for i := 0; i < 50; i++ {
		got := Expand("{{$mock.int(10,12)}}")
		n, err := strconv.Atoi(got)
		if err != nil {
			t.Fatalf("expected integer output, got %q: %v", got, err)
		}
		if n < 10 || n > 12 {
			t.Fatalf("expected value within [10,12], got %d", n)
		}
	}
}

func TestExpandIntHelperHandlesReversedAndDefaults(t *testing.T) {
	t.Parallel()

	for i := 0; i < 50; i++ {
		got := Expand("{{$mock.int(12,10)}}")
		n, err := strconv.Atoi(got)
		if err != nil {
			t.Fatalf("expected integer output, got %q", got)
		}
		if n < 10 || n > 12 {
			t.Fatalf("expected value within swapped range, got %d", n)
		}
	}

	got := Expand("{{$mock.int}}")
	if _, err := strconv.Atoi(got); err != nil {
		t.Fatalf("expected default int helper to return integer, got %q", got)
	}
}

func TestExpandFloatHelperRespectsPrecision(t *testing.T) {
	t.Parallel()

	got := Expand("{{$mock.float(1,2,3)}}")
	if !regexp.MustCompile(`^\d+\.\d{3}$`).MatchString(got) {
		t.Fatalf("expected three decimal places, got %q", got)
	}
	value, err := strconv.ParseFloat(got, 64)
	if err != nil {
		t.Fatalf("parse float: %v", err)
	}
	if value < 1 || value > 2 {
		t.Fatalf("expected value within [1,2], got %g", value)
	}
}

func TestExpandTimestampUnits(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	secs, err := strconv.ParseInt(Expand("{{$mock.timestamp}}"), 10, 64)
	if err != nil {
		t.Fatalf("seconds parse: %v", err)
	}
	if abs(secs-now.Unix()) > 5 {
		t.Fatalf("seconds drifted too far: %d vs %d", secs, now.Unix())
	}

	millis, err := strconv.ParseInt(Expand("{{$mock.timestamp(ms)}}"), 10, 64)
	if err != nil {
		t.Fatalf("millis parse: %v", err)
	}
	if millis < now.UnixMilli()-5000 || millis > now.UnixMilli()+5000 {
		t.Fatalf("millis drifted too far: %d", millis)
	}

	nanos, err := strconv.ParseInt(Expand("{{$mock.timestamp(ns)}}"), 10, 64)
	if err != nil {
		t.Fatalf("nanos parse: %v", err)
	}
	if nanos < now.UnixNano()-int64(5*time.Second) || nanos > now.UnixNano()+int64(5*time.Second) {
		t.Fatalf("nanos drifted too far: %d", nanos)
	}
}

func TestExpandNowAcceptsCustomLayout(t *testing.T) {
	t.Parallel()

	got := Expand("{{$mock.now('2006-01-02')}}")
	if _, err := time.Parse("2006-01-02", got); err != nil {
		t.Fatalf("expected output to match layout, got %q: %v", got, err)
	}
}

func TestExpandPickHelperSelectsFromArgs(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{"alpha": true, "beta": true, "gamma": true}
	for i := 0; i < 30; i++ {
		got := Expand("{{$mock.pick(alpha,beta,gamma)}}")
		if !allowed[got] {
			t.Fatalf("pick produced unexpected value %q", got)
		}
	}

	// Quoted values keep commas/whitespace.
	got := Expand("{{$mock.pick('hello, world')}}")
	if got != "hello, world" {
		t.Fatalf("expected quoted argument preserved, got %q", got)
	}
}

func TestExpandStringHelperLength(t *testing.T) {
	t.Parallel()

	got := Expand("{{$mock.string(12)}}")
	if len([]rune(got)) != 12 {
		t.Fatalf("expected length 12, got %d (%q)", len([]rune(got)), got)
	}
	for _, r := range got {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			t.Fatalf("expected alphabetic characters, got %q", got)
		}
	}
}

func TestExpandIPAndEmailHelpers(t *testing.T) {
	t.Parallel()

	if ip := Expand("{{$mock.ipv4}}"); net.ParseIP(ip) == nil || strings.Contains(ip, ":") {
		t.Fatalf("expected IPv4, got %q", ip)
	}
	if ip := Expand("{{$mock.ipv6}}"); net.ParseIP(ip) == nil || !strings.Contains(ip, ":") {
		t.Fatalf("expected IPv6, got %q", ip)
	}
	if email := Expand("{{$mock.email}}"); !strings.Contains(email, "@") {
		t.Fatalf("expected email-like value, got %q", email)
	}
}

func TestParseArgsHandlesQuotesAndCommas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{"  1 ,  2 ", []string{"1", "2"}},
		{"'hello, world',5", []string{"hello, world", "5"}},
		{`"a, b","c"`, []string{"a, b", "c"}},
		{"alpha,,beta", []string{"alpha", "", "beta"}},
		{"trailing,", []string{"trailing"}},
	}

	for _, tc := range tests {
		got := ParseArgsForTest(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("ParseArgs(%q) length = %d, want %d (%#v)", tc.in, len(got), len(tc.want), got)
		}
		for i, v := range got {
			if v != tc.want[i] {
				t.Fatalf("ParseArgs(%q)[%d] = %q, want %q", tc.in, i, v, tc.want[i])
			}
		}
	}
}

func TestEvalIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	if _, ok := Eval("UUID", nil); !ok {
		t.Fatalf("expected UUID (uppercase) to resolve")
	}
	if _, ok := Eval(" Email ", nil); !ok {
		t.Fatalf("expected ' Email ' to resolve after trimming")
	}
	if _, ok := Eval("not_a_helper", nil); ok {
		t.Fatalf("expected unknown helper to return false")
	}
}

func TestListHelpersIsNonEmptyAndUnique(t *testing.T) {
	t.Parallel()

	helpers := ListHelpers()
	if len(helpers) < 10 {
		t.Fatalf("expected helper catalogue to be substantial, got %d", len(helpers))
	}
	seen := map[string]bool{}
	for _, h := range helpers {
		key := strings.ToLower(h.Name)
		if seen[key] {
			t.Fatalf("helper %q listed twice", h.Name)
		}
		seen[key] = true
		if h.Example == "" || h.Description == "" {
			t.Fatalf("helper %q missing example or description", h.Name)
		}
	}
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
