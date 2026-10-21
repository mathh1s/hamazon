<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TitleLogo from '../components/TitleLogo.vue'
import McButton from '../components/McButton.vue'
import McTextField from '../components/McTextField.vue'
import { unlock } from '../stores/session'
import { errorMessage } from '../api'
import { usePageTitle } from '../lib/title'

usePageTitle(() => 'Enter password')
const route = useRoute()
const router = useRouter()
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  if (!password.value) {
    error.value = 'Enter the password your server admin gave you'
    return
  }
  busy.value = true
  error.value = ''
  try {
    await unlock(password.value)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/'
    router.replace(next)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="gate">
    <TitleLogo />
    <form class="form" @submit.prevent="submit">
      <label for="password" class="shadow">Server password</label>
      <McTextField
        id="password"
        v-model="password"
        type="password"
        autocomplete="current-password"
        :invalid="!!error"
        :aria-describedby="error ? 'password-error' : 'password-help'"
        autofocus
      />
      <p v-if="error" id="password-error" class="error shadow" role="alert">{{ error }}</p>
      <p v-else id="password-help" class="help">Hamazon is only open to players of our server.</p>
      <McButton type="submit" :disabled="busy">{{ busy ? 'Checking...' : 'Open the shop' }}</McButton>
    </form>
  </div>
</template>

<style scoped>
.gate {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: calc(var(--u) * 16);
  padding: 16px;
}

.form {
  width: min(100%, calc(var(--u) * 200));
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 4);
}

.form .mc-button {
  margin-top: calc(var(--u) * 4);
}

label {
  color: var(--gray);
}

.help,
.error {
  margin: 0;
}

.help {
  color: var(--gray);
}

.error {
  color: var(--red);
}
</style>
