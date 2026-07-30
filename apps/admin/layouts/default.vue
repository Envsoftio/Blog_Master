<template>
  <div class="app-root">
    <AppShell v-if="usesAdminShell">
      <slot />
    </AppShell>
    <main v-else class="auth-layout">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const authRoutes = new Set(['/', '/forgot-password', '/reset-password'])
const usesAdminShell = computed(() => !authRoutes.has(route.path) && !route.path.startsWith('/invitations/'))
</script>
