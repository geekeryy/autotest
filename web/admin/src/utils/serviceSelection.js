const SERVICE_ENV_SERVICE_KEY = 'autotest.serviceEnv.serviceId.'

export function readStoredServiceId(projectId) {
  if (!projectId) return ''
  try {
    return localStorage.getItem(SERVICE_ENV_SERVICE_KEY + projectId) || ''
  } catch {
    return ''
  }
}

export function persistServiceId(projectId, serviceId) {
  if (!projectId) return
  try {
    if (serviceId) {
      localStorage.setItem(SERVICE_ENV_SERVICE_KEY + projectId, serviceId)
    } else {
      localStorage.removeItem(SERVICE_ENV_SERVICE_KEY + projectId)
    }
  } catch {
    // ignore storage failures
  }
}
