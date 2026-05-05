package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

// scenarioScriptPreamble provides Postman-style pm.* and console for scenario
// script steps (no HTTP response — execution is sandboxed in goja).
const scenarioScriptPreamble = `
var __pmTests = [];

var AssertionError = (function() {
  function AssertionError(msg) {
    this.message = msg || 'assertion failed';
    this.name = 'AssertionError';
  }
  AssertionError.prototype = Object.create(Error.prototype);
  AssertionError.prototype.constructor = AssertionError;
  AssertionError.prototype.toString = function() { return this.name + ': ' + this.message; };
  return AssertionError;
}());

var pm = {
  variables: {
    get: function(key) {
      var v = __varGet(String(key));
      return v !== undefined && v !== null ? String(v) : '';
    },
    set: function(key, value) {
      __varSet(String(key), String(value));
    }
  },
  environment: {
    get: function(key) {
      return pm.variables.get(key);
    }
  },
  test: function(name, fn) {
    try {
      fn();
      __pmTests.push({ name: name, passed: true, message: '' });
    } catch (e) {
      __pmTests.push({ name: name, passed: false, message: (e && e.message) ? e.message : String(e) });
    }
  }
};

function assert(cond, msg) {
  if (!cond) throw new AssertionError(msg || 'assertion failed');
}

var console = {
  log: function() {
    __logLine(Array.prototype.slice.call(arguments).map(function(a) {
      return typeof a === 'object' ? JSON.stringify(a) : String(a);
    }).join(' '));
  },
  error: function() {
    __logErr(Array.prototype.slice.call(arguments).map(function(a) { return String(a); }).join(' '));
  },
  warn: function() {
    __logErr('[warn] ' + Array.prototype.slice.call(arguments).map(function(a) { return String(a); }).join(' '));
  }
};
`

type jsTestResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// runScenarioJavaScript executes user scenario script with pm.variables / pm.test / console.
// vars is mutated in place: pm.variables.set updates this map.
// ctx should enforce deadlines (e.g. via context.WithTimeout); cancellation interrupts goja.
func runScenarioJavaScript(ctx context.Context, userScript string, vars map[string]string) (stdout, stderr string, exitCode int, err error) {
	if vars == nil {
		vars = map[string]string{}
	}
	rt := goja.New()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			rt.Interrupt(ctx.Err())
		case <-done:
		}
	}()

	var stdoutLines, stderrLines []string
	rt.Set("__varGet", func(key string) string {
		return vars[key]
	})
	rt.Set("__varSet", func(key, val string) {
		vars[key] = val
	})
	rt.Set("__logLine", func(line string) {
		stdoutLines = append(stdoutLines, line)
	})
	rt.Set("__logErr", func(line string) {
		stderrLines = append(stderrLines, line)
	})

	full := scenarioScriptPreamble + "\n" + userScript
	_, runErr := rt.RunString(full)

	stdout = truncateScriptCapture(strings.Join(stdoutLines, "\n"))
	stderr = truncateScriptCapture(strings.Join(stderrLines, "\n"))

	var pmTests []jsTestResult
	if tv := rt.GlobalObject().Get("__pmTests"); tv != nil && !goja.IsUndefined(tv) && !goja.IsNull(tv) {
		if exported := tv.Export(); exported != nil {
			if b, jerr := json.Marshal(exported); jerr == nil {
				_ = json.Unmarshal(b, &pmTests)
			}
		}
	}

	exitCode = 0
	if len(pmTests) > 0 {
		for _, t := range pmTests {
			if !t.Passed {
				exitCode = 1
				name := t.Name
				if name == "" {
					name = "test"
				}
				err = fmt.Errorf("%s: %s", name, t.Message)
				break
			}
		}
	}
	if err == nil && runErr != nil {
		err = runErr
		exitCode = 1
	}

	return stdout, stderr, exitCode, err
}

func truncateScriptCapture(s string) string {
	if len(s) <= maxScriptOutputBytes {
		return s
	}
	return s[:maxScriptOutputBytes]
}
