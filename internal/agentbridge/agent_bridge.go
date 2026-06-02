package agentbridge

import (
	"time"

	"autotest/internal/agent"
	"autotest/internal/aiprovider"
	"autotest/internal/aitools"

	"github.com/google/uuid"
)

// BuildAgentTasks converts a PlannerOutput with SubTasks into a slice of
// SubAgentTask that can be dispatched to the Orchestrator. Returns nil if
// the plan has no sub-tasks (single-agent mode).
func BuildAgentTasks(plan aiprovider.PlannerOutput, catalog *aitools.Catalog) []agent.SubAgentTask {
	if len(plan.SubTasks) == 0 {
		return nil
	}

	tasks := make([]agent.SubAgentTask, 0, len(plan.SubTasks))
	for _, st := range plan.SubTasks {
		// Resolve tool names from domains.
		toolNames := resolveToolNames(st.Domains, catalog)

		timeout := time.Duration(st.Timeout) * time.Second
		if timeout == 0 {
			timeout = 120 * time.Second
		}

		task := agent.SubAgentTask{
			ID:          uuid.New(),
			Name:        st.Name,
			Description: st.Description,
			Tools:       toolNames,
			MaxHops:     10,
			Timeout:     timeout,
			Context: map[string]any{
				"domains":   st.Domains,
				"dependsOn": st.DependsOn,
			},
		}
		tasks = append(tasks, task)
	}
	return tasks
}

// resolveToolNames maps domain names to concrete tool names using the catalog.
func resolveToolNames(domains []string, catalog *aitools.Catalog) []string {
	if catalog == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	for _, d := range domains {
		for _, n := range catalog.ByDomain(d) {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				names = append(names, n)
			}
		}
	}
	return names
}

// TopoSort orders sub-tasks by dependency. Returns the ordered tasks and an
// error if a cycle is detected or duplicate names exist. Tasks with no
// dependencies come first.
func TopoSort(tasks []aiprovider.SubTaskPlan) ([]aiprovider.SubTaskPlan, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	// Build adjacency and in-degree, detecting duplicate names.
	nameIdx := map[string]int{}
	for i, t := range tasks {
		if _, dup := nameIdx[t.Name]; dup {
			return nil, &AgentBridgeError{Code: "DUPLICATE_NAME", Message: "duplicate sub-task name: " + t.Name}
		}
		nameIdx[t.Name] = i
	}
	inDegree := make([]int, len(tasks))
	dependents := map[int][]int{} // from -> []to
	for i, t := range tasks {
		for _, dep := range t.DependsOn {
			j, ok := nameIdx[dep]
			if !ok {
				continue // skip unknown deps
			}
			dependents[j] = append(dependents[j], i)
			inDegree[i]++
		}
	}

	// BFS topological sort.
	queue := []int{}
	for i, d := range inDegree {
		if d == 0 {
			queue = append(queue, i)
		}
	}

	ordered := make([]aiprovider.SubTaskPlan, 0, len(tasks))
	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		ordered = append(ordered, tasks[idx])
		for _, dep := range dependents[idx] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(ordered) != len(tasks) {
		return nil, &AgentBridgeError{Code: "CYCLE_DETECTED", Message: "sub-task dependency cycle detected"}
	}
	return ordered, nil
}

// AgentBridgeError represents an error in agent bridge operations.
type AgentBridgeError struct {
	Code    string
	Message string
}

func (e *AgentBridgeError) Error() string {
	return e.Message
}
