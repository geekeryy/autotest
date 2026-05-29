package genagent

import "errors"

var (
	errAutorunDisabled     = errors.New("genagent: 场景生成真实环境验证未在平台配置中开启，禁止在真实环境执行场景")
	errEnvironmentRequired = errors.New("genagent: environmentId 必填")
	errServiceRequired     = errors.New("genagent: serviceId 必填")
	errDepsMissing         = errors.New("genagent: 依赖未配置")
)
