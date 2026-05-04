import router from './router'
import { getToken } from './utils/storage'
import { hasPermission, loadCurrentUser, authState } from './auth'

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
    if (!hasPermission(to.meta?.permission)) {
      return '/dashboard'
    }
    return true
  } catch (error) {
    return '/login'
  }
})
