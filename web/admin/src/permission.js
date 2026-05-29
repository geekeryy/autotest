import router from './router'
import { getToken } from './utils/storage'
import { loadCurrentUser, authState, routePermissionAllowed } from './auth'

const oauthCallbackPaths = ['/login/oauth/callback', '/login/github/callback']
const pendingAllowedPaths = ['/pending-approval', ...oauthCallbackPaths]
const passwordChangeAllowedPaths = ['/change-password', ...oauthCallbackPaths]

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

  if (user?.mustChangePassword) {
    if (passwordChangeAllowedPaths.includes(to.path)) {
      return true
    }
    return '/change-password'
  }

  if (user && !user.mustChangePassword && to.path === '/change-password') {
    return '/'
  }

  if (user?.active && to.path === '/pending-approval') {
    return '/'
  }

  if (!routePermissionAllowed(to.meta)) {
    return '/dashboard'
  }
  return true
})
