package genagent

import (
	"context"
	"encoding/json"
	"fmt"

	"autotest/internal/aitools"
	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/scenariogen"

	"github.com/google/uuid"
)

// ScenarioRunner executes scenarios on a real environment.
type ScenarioRunner interface {
	RunScenario(ctx context.Context, scenarioID uuid.UUID, input runner.RunScenarioInput) (*runner.RunScenarioOutput, error)
}

type scenarioRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*scenario.Scenario, error)
	ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error)
}

// CoverageGenerator builds depgraph-driven scenarios.
type CoverageGenerator interface {
	GenerateCoverage(ctx context.Context, projectID, serviceID uuid.UUID, opts *scenariogen.CoverageOptions) (*scenariogen.CoverageResult, error)
}

// Agent orchestrates generate → execute → classify → repair loops.
type Agent struct {
	Jobs         JobRepository
	Generator    CoverageGenerator
	Runner       ScenarioRunner
	Scenarios    ScenarioRepairService
	ScenarioRepo scenarioRepository
	Repairer     *Repairer
}

// RunAsync starts a job in the background and returns immediately.
func (a *Agent) RunAsync(ctx context.Context, cfg RunConfig) (*Job, error) {
	if err := AssertRunAllowed(cfg); err != nil {
		return nil, err
	}
	if a.Jobs == nil || a.Generator == nil || a.Runner == nil {
		return nil, errDepsMissing
	}
	job, err := a.Jobs.Create(ctx, cfg)
	if err != nil {
		return nil, err
	}
	go func() {
		bg := context.WithoutCancel(ctx)
		if sink := aitools.GenJobProgressSinkFrom(ctx); sink != nil {
			bg = aitools.WithGenJobProgress(bg, sink)
		}
		_ = a.run(bg, job.ID, cfg)
	}()
	return job, nil
}

// Run executes synchronously (primarily for tests).
func (a *Agent) Run(ctx context.Context, cfg RunConfig) (*JobResult, error) {
	job, err := a.Jobs.Create(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := a.run(ctx, job.ID, cfg); err != nil {
		return nil, err
	}
	updated, err := a.Jobs.Get(ctx, cfg.ProjectID, job.ID)
	if err != nil {
		return nil, err
	}
	var result JobResult
	if len(updated.Result) > 0 {
		_ = json.Unmarshal(updated.Result, &result)
	}
	return &result, nil
}

func (a *Agent) run(ctx context.Context, jobID uuid.UUID, cfg RunConfig) error {
	maxRounds := cfg.normalizedMaxRounds()
	_ = a.Jobs.Update(ctx, jobID, JobRunning, 0, nil)
	emitProgress(ctx, Progress{
		JobID: jobID.String(), Status: string(JobRunning), Round: 0, MaxRounds: maxRounds,
		Phase: "generate", Message: "正在生成覆盖场景…",
	})

	coverage, err := a.Generator.GenerateCoverage(ctx, cfg.ProjectID, cfg.ServiceID, &scenariogen.CoverageOptions{
		LoginCredentials: cfg.LoginCredentials,
	})
	if err != nil {
		_ = a.Jobs.Update(ctx, jobID, JobFailed, 0, mustJSON(map[string]string{"error": err.Error()}))
		emitProgress(ctx, Progress{JobID: jobID.String(), Status: string(JobFailed), Phase: "error", Message: err.Error()})
		return err
	}

	jobResult := JobResult{Scenarios: len(coverage.Scenarios), CoverageResult: mustJSON(coverage)}
	repairHistory := map[uuid.UUID][]RepairAttempt{}

	for round := 1; round <= maxRounds; round++ {
		emitProgress(ctx, Progress{
			JobID: jobID.String(), Status: string(JobRunning), Round: round, MaxRounds: maxRounds,
			Phase: "execute", Message: fmt.Sprintf("第 %d 轮：真实环境执行中…", round),
		})

		roundResult := RoundResult{Round: round, ScenariosTotal: len(coverage.Scenarios)}
		scenarioRuns := a.executeScenarios(ctx, cfg, coverage.Scenarios, round)
		roundResult.ScenarioRuns = scenarioRuns

		for _, ref := range scenarioRuns {
			if ref.Status == "passed" {
				roundResult.Passed++
			} else {
				roundResult.Failed++
			}
		}
		if roundResult.ScenariosTotal > 0 {
			jobResult.FinalPassRate = float64(roundResult.Passed) / float64(roundResult.ScenariosTotal)
		}

		emitProgress(ctx, Progress{
			JobID: jobID.String(), Status: string(JobRunning), Round: round, MaxRounds: maxRounds,
			Phase: "execute", PassRate: jobResult.FinalPassRate,
			Message: fmt.Sprintf("通过率 %.0f%%（%d/%d）", jobResult.FinalPassRate*100, roundResult.Passed, roundResult.ScenariosTotal),
		})

		jobResult.Rounds = append(jobResult.Rounds, roundResult)
		_ = a.Jobs.Update(ctx, jobID, JobRunning, round, mustJSON(jobResult))

		if roundResult.Failed == 0 {
			jobResult.FinalPassRate = 1
			_ = a.Jobs.Update(ctx, jobID, JobCompleted, round, mustJSON(jobResult))
			emitProgress(ctx, Progress{
				JobID: jobID.String(), Status: string(JobCompleted), Round: round, MaxRounds: maxRounds,
				Phase: "done", PassRate: 1, Message: "全部场景通过",
			})
			return nil
		}

		if !cfg.AutoRepair || round >= maxRounds {
			status := JobCompleted
			msg := "已达最大轮次"
			if !cfg.AutoRepair {
				status = JobStopped
				msg = "未开启自动修复，停止于首轮执行"
			}
			roundResult.StoppedReason = msg
			jobResult.Rounds[len(jobResult.Rounds)-1] = roundResult
			_ = a.Jobs.Update(ctx, jobID, status, round, mustJSON(jobResult))
			emitProgress(ctx, Progress{
				JobID: jobID.String(), Status: string(status), Round: round, MaxRounds: maxRounds,
				Phase: "done", PassRate: jobResult.FinalPassRate, Message: msg,
			})
			return nil
		}

		emitProgress(ctx, Progress{
			JobID: jobID.String(), Status: string(JobRunning), Round: round, MaxRounds: maxRounds,
			Phase: "repair", Message: "分析失败并自动修复…",
		})

		repairs, realDefects, stopped := a.repairFailures(ctx, cfg, scenarioRuns, &roundResult, repairHistory, round)
		roundResult.Repairs = repairs
		jobResult.RealDefects += realDefects
		jobResult.Rounds[len(jobResult.Rounds)-1] = roundResult
		_ = a.Jobs.Update(ctx, jobID, JobRunning, round, mustJSON(jobResult))

		emitProgress(ctx, Progress{
			JobID: jobID.String(), Status: string(JobRunning), Round: round, MaxRounds: maxRounds,
			Phase: "repair", PassRate: jobResult.FinalPassRate, Repairs: repairs,
			Message: fmt.Sprintf("完成 %d 项修复", len(repairs)),
		})

		if stopped {
			_ = a.Jobs.Update(ctx, jobID, JobStopped, round, mustJSON(jobResult))
			emitProgress(ctx, Progress{
				JobID: jobID.String(), Status: string(JobStopped), Round: round, MaxRounds: maxRounds,
				Phase: "done", PassRate: jobResult.FinalPassRate, Message: roundResult.StoppedReason, Repairs: repairs,
			})
			return nil
		}
	}
	return nil
}

func (a *Agent) executeScenarios(ctx context.Context, cfg RunConfig, scenarios []scenariogen.CreatedScenario, round int) []ScenarioRunRef {
	out := make([]ScenarioRunRef, len(scenarios))
	// Serial execution: scenarios may have interdependencies (e.g. scenario A
	// creates data that scenario B needs). Parallel execution causes race
	// conditions on shared state. Serial execution ensures deterministic,
	// reproducible results.
	for i, sc := range scenarios {
		ref := ScenarioRunRef{ScenarioID: sc.ScenarioID, ScenarioName: sc.Name}
		runOut, runErr := a.Runner.RunScenario(ctx, sc.ScenarioID, runner.RunScenarioInput{
			EnvironmentID: cfg.EnvironmentID,
			Name:          fmt.Sprintf("gen-verify %s r%d", sc.Name, round),
		})
		if runErr != nil {
			ref.Status = "error"
			ref.Failures = []string{runErr.Error()}
		} else {
			ref.RunID = runOut.Run.ID
			ok, fails := summarizeRun(runOut)
			if ok {
				ref.Status = "passed"
			} else {
				ref.Status = "failed"
				ref.Failures = fails
			}
			ref.Output = runOut
		}
		out[i] = ref
	}
	return out
}

func (a *Agent) repairFailures(ctx context.Context, cfg RunConfig, runs []ScenarioRunRef, roundResult *RoundResult, repairHistory map[uuid.UUID][]RepairAttempt, round int) (repairs []RepairSummary, realDefects int, stopped bool) {
	if a.Repairer == nil || a.Scenarios == nil {
		return nil, 0, false
	}
	for _, ref := range runs {
		if ref.Status == "passed" || ref.Output == nil {
			continue
		}
		steps, err := a.Scenarios.ListSteps(ctx, ref.ScenarioID)
		if err != nil {
			continue
		}
		stepByID := map[uuid.UUID]scenario.Step{}
		for _, st := range steps {
			stepByID[st.ID] = st
		}
		for _, sr := range ref.Output.StepResults {
			if sr.Result == nil || sr.Result.Status == report.ResultPassed {
				continue
			}
			step := sr.Step
			if step.ID == uuid.Nil {
				if st, ok := stepByID[sr.Step.ID]; ok {
					step = st
				}
			}
			evidence := buildFailureEvidence(ref.ScenarioName, step, sr, ref.Output, repairHistory[step.ID])
			class, _ := a.Repairer.AI.ClassifyFailure(ctx, cfg.ProjectID, evidence)
			repair, err := applyRepair(ctx, a.Repairer, cfg, cfg.ProjectID, ref.ScenarioID, &step, sr, class)
			if err != nil || repair == nil {
				continue
			}
			repairs = append(repairs, *repair)
			// Record repair attempt for cross-round memory.
			success := repair.Action != "skip" && repair.Action != "noop"
			repairHistory[step.ID] = append(repairHistory[step.ID], RepairAttempt{
				Round:    round,
				Category: repair.Category,
				Action:   repair.Action,
				Detail:   repair.Detail,
				Success:  success,
			})
			if repair.Category == "real_defect" {
				realDefects++
				if cfg.StopOnRealDefect {
					roundResult.StoppedReason = "检测到真实缺陷，已停止"
					return repairs, realDefects, true
				}
			}
		}
	}
	return repairs, realDefects, false
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
