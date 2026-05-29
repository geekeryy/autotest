// Command ai-eval runs the AI assistant tool-routing Golden Set evaluation
// (internal/aiprovider/eval) and prints a human-readable report. It exits
// non-zero when any case regresses, so it can gate CI via `make ai-eval`.
//
// It does NOT require a real LLM or database: each case carries a mocked
// planner output and the harness exercises the deterministic Router rules.
package main

import (
	"flag"
	"fmt"
	"os"

	"autotest/internal/aiprovider/eval"
)

func main() {
	dir := flag.String("dir", "testdata/ai_assistant_eval", "Golden Set directory (*.yaml)")
	flag.Parse()

	cases, err := eval.LoadCases(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load cases: %v\n", err)
		os.Exit(2)
	}
	if len(cases) == 0 {
		fmt.Fprintf(os.Stderr, "no cases found under %s\n", *dir)
		os.Exit(2)
	}

	cat := eval.DefaultCatalog()
	rep := eval.Evaluate(cases, cat)

	fmt.Printf("AI 工具路由 Golden Set 评测（%d 条用例，catalog=%d 工具）\n", len(cases), cat.Len())
	fmt.Println("------------------------------------------------------------")
	for _, r := range rep.Results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-26s recall@hop1=%.0f%% (%d/%d) offered=%d\n",
			status, r.Name, r.RecallRate()*100, r.RecallHit, r.RecallTotal, len(r.FirstHopTools))
		if len(r.MissingTools) > 0 {
			fmt.Printf("        缺失期望工具: %v\n", r.MissingTools)
		}
		if len(r.ForbidHits) > 0 {
			fmt.Printf("        误纳禁用工具: %v\n", r.ForbidHits)
		}
		if len(r.MissingDomain) > 0 {
			fmt.Printf("        未覆盖域: %v\n", r.MissingDomain)
		}
	}
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("平均 Tool Recall@Hop1=%.1f%%  Mis-route Rate=%.1f%%\n",
		rep.MeanRecall()*100, rep.MisrouteRate()*100)

	if !rep.Passed() {
		fmt.Fprintln(os.Stderr, "Golden Set 评测存在失败用例")
		os.Exit(1)
	}
	fmt.Println("Golden Set 全部通过")
}
