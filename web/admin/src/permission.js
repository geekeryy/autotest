import router from './router'
import { getToken } from './utils/storage'
import { hasPermission, loadCurrentUser, authState } from './auth'

const pendingAllowedPaths = ['/pending-approval', '/login/github/callback']

router.beforeEach(async (to) => {
  document.title = `${to.meta?.title || '管理后台'} - Autotest`

  if (to.meta?.public) {
    return true
  }

  if (!getToken()) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  try {
    if (!authState.loaded) {
      await loadCurrentUser()
    }
  } catch {
    return '/login'
  }

  const user = authState.user
  if (user && !user.active) {
    if (pendingAllowedPaths.includes(to.path)) {
      return true
    }
    return '/pending-approval'
  }

  if (user?.active && to.path === '/pending-approval') {
    return '/dashboard'
  }

  if (!hasPermission(to.meta?.permission)) {
    return '/dashboard'
  }
  return true
})
