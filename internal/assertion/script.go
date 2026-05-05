package assertion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// ScriptAssertion runs a JavaScript test script and collects the results of all
// pm.test() calls. It implements MultiAssertion so each named test becomes an
// independent Result entry in the assertion log.
//
// The script runs in a sandboxed goja (ECMAScript 5.1+ / ES2015+) runtime with
// a Postman-compatible pm API and a global assert() helper.
//
// JSON spec:
//
//	{
//	  "type": "script",
//	  "script": "pm.test('status ok', () => pm.response.to.have.status(200));"
//	}
//
// Available globals inside the script:
//
//	pm.response.status          – HTTP status code (number)
//	pm.response.responseTime    – response duration in ms (number)
//	pm.response.headers         – flat object { "content-type": "..." }
//	pm.response.text()          – raw body string
//	pm.response.json()          – parsed JSON body (throws if not JSON)
//	pm.response.to.have.status(n)
//	pm.response.to.have.header(name[, value])
//	pm.response.to.have.jsonBody()
//	pm.test(name, fn)           – register a named test (fn throws on failure)
//	pm.expect(val)              – Chai-style assertion chain
//	pm.variables.get(key)       – read variable from current run context
//	pm.variables.set(key, val)  – write to run context (currently session-only)
//	assert(cond[, msg])         – throw AssertionError if condition is false
//	response                    – alias for pm.response
//	console.log / .error / .warn
type ScriptAssertion struct {
	Script string
}

func (a ScriptAssertion) Type() string { return "script" }

// Assert satisfies the Assertion interface by delegating to AssertMulti and
// collapsing multiple results into one (used when only one Result slot is
// available, e.g. legacy callers). Prefer AssertMulti for richer output.
func (a ScriptAssertion) Assert(ctx context.Context, input Input) Result {
	results := a.AssertMulti(ctx, input)
	if len(results) == 0 {
		return Result{Type: a.Type(), Passed: true}
	}
	// If all pass, return the first; otherwise return the first failure.
	for _, r := range results {
		if !r.Passed {
			return r
		}
	}
	return results[0]
}

// AssertMulti implements MultiAssertion: each pm.test() call becomes one Result.
func (a ScriptAssertion) AssertMulti(ctx context.Context, input Input) []Result {
	rt := goja.New()

	// Cancel the runtime when context expires or a 10-second hard timeout fires.
	done := make(chan struct{})
	defer close(done)
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	go func() {
		select {
		case <-timeoutCtx.Done():
			rt.Interrupt(timeoutCtx.Err())
		case <-done:
		}
	}()

	// ── inject host-side globals ─────────────────────────────────────────────
	rt.Set("__responseStatus", input.StatusCode)
	rt.Set("__responseBody", string(input.Body))
	rt.Set("__durationMs", input.DurationMillis)

	// Flatten headers to lowercase keys.
	headers := make(map[string]string, len(input.Headers))
	for k, vals := range input.Headers {
		if len(vals) > 0 {
			headers[strings.ToLower(k)] = vals[0]
		}
	}
	rt.Set("__responseHeaders", headers)

	// Copy variables so script can read them.
	vars := make(map[string]string, len(input.Variables))
	for k, v := range input.Variables {
		vars[k] = v
	}
	rt.Set("__variables", vars)

	// console.log sink (captured but not surfaced in results to keep output clean).
	rt.Set("__log", func(msg string) {}) // no-op sink; replace with real logger if needed

	// ── run preamble + user script ──────────────────────────────────────────
	fullScript := jsPreamble + "\n" + a.Script
	_, runErr := rt.RunString(fullScript)

	// ── collect pm.test results ─────────────────────────────────────────────
	type jsTestResult struct {
		Name    string `json:"name"`
		Passed  bool   `json:"passed"`
		Message string `json:"message"`
	}
	var pmTests []jsTestResult

	if testsVal := rt.GlobalObject().Get("__pmTests"); testsVal != nil &&
		!goja.IsUndefined(testsVal) && !goja.IsNull(testsVal) {
		if exported := testsVal.Export(); exported != nil {
			if b, err := json.Marshal(exported); err == nil {
				_ = json.Unmarshal(b, &pmTests)
			}
		}
	}

	if len(pmTests) == 0 {
		// No pm.test calls: script-level pass/fail.
		if runErr != nil {
			return []Result{{Type: "script", Passed: false, Message: runErr.Error()}}
		}
		return []Result{{Type: "script", Passed: true}}
	}

	results := make([]Result, 0, len(pmTests)+1)
	for i, t := range pmTests {
		name := t.Name
		if name == "" {
			name = fmt.Sprintf("test %d", i+1)
		}
		results = append(results, Result{
			Type:    "script",
			Name:    name,
			Passed:  t.Passed,
			Message: t.Message,
		})
	}

	// A top-level script error (outside pm.test) is appended as a separate result.
	if runErr != nil {
		results = append(results, Result{
			Type:    "script",
			Name:    "script error",
			Passed:  false,
			Message: runErr.Error(),
		})
	}
	return results
}

// jsPreamble is prepended to every user script. It provides:
//   - AssertionError class
//   - Expect chain (Chai-style pm.expect)
//   - pm object (response, test, expect, variables)
//   - assert() global
//   - response alias
//   - console shim
const jsPreamble = `
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

var Expect = (function() {
  function Expect(val, neg) {
    this._val = val;
    this._neg = !!neg;
  }
  var p = Expect.prototype;

  p._assert = function(cond, msg, negMsg) {
    if (this._neg ? cond : !cond) {
      throw new AssertionError(this._neg ? (negMsg || 'not: ' + msg) : msg);
    }
    return this;
  };

  // Chainable pass-through properties
  ['to','be','been','is','that','which','and','has','have','with','at','of','same'].forEach(function(k) {
    Object.defineProperty(p, k, { get: function() { return this; }, configurable: true });
  });

  Object.defineProperty(p, 'not', {
    get: function() { return new Expect(this._val, !this._neg); },
    configurable: true
  });

  // Immediate-evaluation getters
  Object.defineProperty(p, 'ok', {
    get: function() {
      return this._assert(!!this._val,
        'expected truthy, got ' + JSON.stringify(this._val),
        'expected falsy, got ' + JSON.stringify(this._val));
    }, configurable: true
  });
  Object.defineProperty(p, 'null', {
    get: function() {
      return this._assert(this._val === null,
        'expected null, got ' + JSON.stringify(this._val), 'expected not null');
    }, configurable: true
  });
  Object.defineProperty(p, 'undefined', {
    get: function() {
      return this._assert(this._val === undefined,
        'expected undefined, got ' + JSON.stringify(this._val), 'expected not undefined');
    }, configurable: true
  });
  Object.defineProperty(p, 'empty', {
    get: function() {
      var v = this._val;
      var cond = v == null || v === '' ||
        (Array.isArray(v) && v.length === 0) ||
        (typeof v === 'object' && v !== null && !Array.isArray(v) && Object.keys(v).length === 0);
      return this._assert(cond, 'expected empty, got ' + JSON.stringify(v), 'expected not empty');
    }, configurable: true
  });
  Object.defineProperty(p, 'true', {
    get: function() {
      return this._assert(this._val === true,
        'expected true, got ' + JSON.stringify(this._val), 'expected not true');
    }, configurable: true
  });
  Object.defineProperty(p, 'false', {
    get: function() {
      return this._assert(this._val === false,
        'expected false, got ' + JSON.stringify(this._val), 'expected not false');
    }, configurable: true
  });

  p.equal = function(exp) {
    return this._assert(this._val === exp,
      'expected ' + JSON.stringify(this._val) + ' to equal ' + JSON.stringify(exp),
      'expected ' + JSON.stringify(this._val) + ' not to equal ' + JSON.stringify(exp));
  };
  p.eql = function(exp) {
    return this._assert(JSON.stringify(this._val) === JSON.stringify(exp),
      'expected deep equal ' + JSON.stringify(exp),
      'expected not deep equal ' + JSON.stringify(exp));
  };
  p.include = function(val) {
    var v = this._val;
    var cond = typeof v === 'string' ? v.indexOf(String(val)) >= 0 :
               Array.isArray(v) ? v.indexOf(val) >= 0 : false;
    return this._assert(cond,
      JSON.stringify(v) + ' should include ' + JSON.stringify(val),
      JSON.stringify(v) + ' should not include ' + JSON.stringify(val));
  };
  p.match = function(re) {
    var s = String(this._val);
    var cond = (re instanceof RegExp) ? re.test(s) : new RegExp(String(re)).test(s);
    return this._assert(cond,
      JSON.stringify(s) + ' should match ' + re,
      JSON.stringify(s) + ' should not match ' + re);
  };
  p.a = function(type) {
    var v = this._val;
    var cond = type === 'array' ? Array.isArray(v) :
               type === 'null'  ? v === null :
               typeof v === type;
    return this._assert(cond,
      'expected ' + JSON.stringify(v) + ' to be a ' + type,
      'expected ' + JSON.stringify(v) + ' not to be a ' + type);
  };
  p.above    = function(n) { return this._assert(this._val > n,  'expected ' + this._val + ' > ' + n,  'expected ' + this._val + ' <= ' + n); };
  p.below    = function(n) { return this._assert(this._val < n,  'expected ' + this._val + ' < ' + n,  'expected ' + this._val + ' >= ' + n); };
  p.least    = function(n) { return this._assert(this._val >= n, 'expected ' + this._val + ' >= ' + n, 'expected ' + this._val + ' < ' + n); };
  p.most     = function(n) { return this._assert(this._val <= n, 'expected ' + this._val + ' <= ' + n, 'expected ' + this._val + ' > ' + n); };
  p.within   = function(lo, hi) { return this._assert(this._val >= lo && this._val <= hi, 'expected ' + this._val + ' in [' + lo + ',' + hi + ']', 'expected ' + this._val + ' not in [' + lo + ',' + hi + ']'); };
  p.lengthOf = function(n) {
    var len = (this._val != null && this._val.length != null) ? this._val.length : 0;
    return this._assert(len === n, 'expected length ' + n + ', got ' + len, 'expected length not ' + n);
  };
  p.property = function(key, val) {
    var obj = this._val;
    var has = obj != null && Object.prototype.hasOwnProperty.call(Object(obj), key);
    if (arguments.length < 2) {
      return this._assert(has, 'expected to have property "' + key + '"', 'expected not to have property "' + key + '"');
    }
    return this._assert(has && obj[key] === val,
      'expected property "' + key + '" to equal ' + JSON.stringify(val) + ', got ' + JSON.stringify(obj && obj[key]),
      'expected property "' + key + '" not to equal ' + JSON.stringify(val));
  };
  p.oneOf = function(list) {
    return this._assert(list.indexOf(this._val) >= 0,
      'expected ' + JSON.stringify(this._val) + ' to be one of ' + JSON.stringify(list),
      'expected ' + JSON.stringify(this._val) + ' not to be one of ' + JSON.stringify(list));
  };

  return Expect;
}());

// ── pm object ─────────────────────────────────────────────────────────────
var pm = (function() {
  var _status  = __responseStatus;
  var _body    = __responseBody;
  var _headers = __responseHeaders;
  var _ms      = __durationMs;

  var _response = {
    status:       _status,
    responseTime: _ms,
    headers:      _headers,
    text: function() { return _body; },
    json: function() {
      try { return JSON.parse(_body); }
      catch(e) { throw new AssertionError('response body is not valid JSON: ' + e.message); }
    },
    to: {
      have: {
        status: function(expected) {
          if (_status !== expected) {
            throw new AssertionError('expected status ' + expected + ', got ' + _status);
          }
        },
        header: function(name, value) {
          var v = _headers[name.toLowerCase()];
          if (v == null) throw new AssertionError('expected header "' + name + '" to exist');
          if (arguments.length > 1 && v !== value) {
            throw new AssertionError('expected header "' + name + '" to be "' + value + '", got "' + v + '"');
          }
        },
        jsonBody: function() {
          try { JSON.parse(_body); }
          catch(e) { throw new AssertionError('response body is not valid JSON'); }
        }
      }
    }
  };

  return {
    response: _response,

    test: function(name, fn) {
      try {
        fn();
        __pmTests.push({ name: name, passed: true, message: '' });
      } catch(e) {
        __pmTests.push({ name: name, passed: false, message: e.message || String(e) });
      }
    },

    expect: function(val) { return new Expect(val); },

    variables: {
      get: function(key) {
        var v = __variables[key];
        return v !== undefined ? String(v) : '';
      },
      set: function(key, value) {
        __variables[key] = String(value);
      }
    },

    // Postman also exposes pm.environment as an alias for variables in simple environments.
    environment: {
      get: function(key) {
        var v = __variables[key];
        return v !== undefined ? String(v) : '';
      }
    }
  };
}());

// Global helpers
function assert(cond, msg) {
  if (!cond) throw new AssertionError(msg || 'assertion failed');
}

var response = pm.response;

var console = {
  log:   function() { __log(Array.prototype.slice.call(arguments).map(function(a){ return typeof a==='object'?JSON.stringify(a):String(a); }).join(' ')); },
  error: function() { __log('[error] '+Array.prototype.slice.call(arguments).map(function(a){ return String(a); }).join(' ')); },
  warn:  function() { __log('[warn] '+Array.prototype.slice.call(arguments).map(function(a){ return String(a); }).join(' ')); }
};
`
