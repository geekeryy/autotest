package runner

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"autotest/internal/scenario"
	"autotest/internal/templating"
)

// stepExtractRule copies one field from the current step output into scenario variables.
type stepExtractRule struct {
	Name string `json:"name"`
	From string `json:"from"`
}

var stepVarNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type stepRefIndex struct {
	outputs map[int]any
	byName  map[string]int
	bySlug  map[string]int
}

func newStepRefIndex(outputs map[int]any, steps []scenario.Step) *stepRefIndex {
	idx := &stepRefIndex{
		outputs: outputs,
		byName:  map[string]int{},
		bySlug:  map[string]int{},
	}
	for _, step := range steps {
		name := strings.TrimSpace(step.Name)
		if name == "" || step.StepSeq <= 0 {
			continue
		}
		if _, exists := idx.byName[name]; !exists {
			idx.byName[name] = step.StepSeq
		}
		slug := slugStepName(name)
		if slug != "" {
			if _, exists := idx.bySlug[slug]; !exists {
				idx.bySlug[slug] = step.StepSeq
			}
		}
	}
	return idx
}

func slugStepName(name string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func (idx *stepRefIndex) resolve(tok templating.StepToken) (string, bool) {
	seq := tok.Seq
	if seq <= 0 {
		if tok.Name != "" {
			if s, ok := idx.byName[tok.Name]; ok {
				seq = s
			} else if s, ok := idx.bySlug[tok.Name]; ok {
				seq = s
			}
		}
	}
	if seq <= 0 {
		return "", false
	}
	return resolveStepRef(seq, tok.Path, idx.outputs)
}

func parseStepExtracts(raw json.RawMessage) []stepExtractRule {
	if len(raw) == 0 {
		return nil
	}
	var cfg struct {
		Extracts []stepExtractRule `json:"extracts"`
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return nil
	}
	return cfg.Extracts
}

func applyStepExtracts(config json.RawMessage, output map[string]any, vars map[string]string) {
	if output == nil || vars == nil {
		return
	}
	for _, ex := range parseStepExtracts(config) {
		name := strings.TrimSpace(ex.Name)
		from := strings.TrimSpace(ex.From)
		if name == "" || from == "" || !stepVarNameRE.MatchString(name) {
			continue
		}
		if v, ok := resolveFromStepOutput(output, from); ok {
			vars[name] = v
		}
	}
}

func resolveFromStepOutput(output map[string]any, path string) (string, bool) {
	path = normalizeStepRefPath(path)
	val, err := traverseOutputPath(output, path)
	if err != nil {
		if alt := tokenPathFallback(path); alt != "" {
			val, err = traverseOutputPath(output, alt)
		}
	}
	if err != nil {
		return "", false
	}
	return stepOutputStr(val), true
}
