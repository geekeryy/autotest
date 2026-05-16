<template>
  <el-config-provider :locale="locale">
    <router-view />
  </el-config-provider>
</template>

<script>
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'

export default {
  name: 'App',
  data() {
    return {
      locale: zhCn,
    }
  },
}
</script>
