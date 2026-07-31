<template>
  <section class="auth-page">
    <div class="auth-page__top">
      <NuxtLink class="auth-brand" to="/">
        <span><PenLine :size="18" /></span>
        <strong>Editorial</strong>
      </NuxtLink>
      <div class="theme-switcher" role="group" aria-label="Color theme">
        <button
          v-for="option in themeOptions"
          :key="option.value"
          class="theme-switcher__option"
          :class="{ 'is-active': colorMode.preference === option.value }"
          type="button"
          :title="`${option.label} theme`"
          :aria-label="`${option.label} theme`"
          @click="colorMode.preference = option.value"
        >
          <component :is="option.icon" :size="15" />
        </button>
      </div>
    </div>
    <main class="auth-page__main">
      <div class="auth-card surface">
        <div class="auth-card__heading">
          <span>{{ eyebrow }}</span>
          <h1>{{ title }}</h1>
          <p>{{ description }}</p>
        </div>
        <slot />
      </div>
    </main>
  </section>
</template>

<script setup lang="ts">
import { Laptop, Moon, PenLine, Sun } from 'lucide-vue-next'

defineProps<{ eyebrow: string, title: string, description: string }>()
const colorMode = useColorMode()
const themeOptions = [
  { value: 'system', label: 'System', icon: Laptop },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon }
]
</script>

<style scoped>
.auth-page { min-height: 100vh; background: var(--bg); color: var(--text); }
.auth-page__top { position: fixed; z-index: 5; inset: 0 0 auto; display: flex; height: 68px; align-items: center; justify-content: space-between; padding: 0 28px; }
.auth-brand { display: inline-flex; align-items: center; gap: 9px; color: var(--text); text-decoration: none; }
.auth-brand > span { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 7px; background: var(--primary); color: white; }
.auth-brand strong { font-size: 13px; }
.auth-page__main { display: grid; min-height: 100vh; place-items: center; padding: 90px 18px 32px; }
.auth-card { width: 100%; max-width: 430px; padding: 29px; box-shadow: var(--shadow-md); }
.auth-card__heading { margin-bottom: 24px; }
.auth-card__heading > span { color: var(--primary); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.auth-card__heading h1 { margin: 5px 0 0; font-size: 22px; }
.auth-card__heading p { margin: 6px 0 0; color: var(--text-soft); font-size: 12px; }
@media (max-width: 520px) { .auth-page__top { padding-inline: 15px; } .auth-card { padding: 23px 18px; } }
</style>
