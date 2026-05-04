<template>
  <span class="brand-logo" aria-hidden="true">
    <svg class="brand-logo__svg" viewBox="0 0 48 48" fill="none" focusable="false">
      <rect x="3" y="3" width="42" height="42" rx="14" fill="currentColor" />
      <path
        d="M13 16h9c5.5 0 10 4.5 10 10v2"
        stroke="var(--app-secondary-color)"
        stroke-width="4"
        stroke-linecap="round"
      />
      <path d="M15 30.5 21 36l13-16" stroke="#fff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
      <circle cx="13" cy="16" r="4" fill="#fff" />
      <circle cx="32" cy="16" r="4" fill="var(--app-accent-color)" />
      <circle cx="32" cy="28" r="3" fill="#fff" fill-opacity="0.9" />
    </svg>
  </span>
</template>

<script>
export default {
  name: 'BrandLogo'
}
</script>

<style scoped>
.brand-logo {
  --brand-logo-size: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--brand-logo-size);
  height: var(--brand-logo-size);
  color: var(--app-primary-color);
  flex: 0 0 auto;
}

.brand-logo__svg {
  width: 100%;
  height: 100%;
  display: block;
  filter: drop-shadow(0 8px 18px rgba(15, 23, 42, 0.18));
}
</style>
