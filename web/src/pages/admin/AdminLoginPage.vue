<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TitleLogo from '../../components/TitleLogo.vue'
import McButton from '../../components/McButton.vue'
import McTextField from '../../components/McTextField.vue'
import { adminLogin } from '../../stores/admin'
import { errorMessage } from '../../api'
import { usePageTitle } from '../../lib/title'

usePageTitle(() => 'Admin login')
const route = useRoute()
const router = useRouter()
const name = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  if (!name.value || !password.value) {
    error.value = 'Enter your admin name and password'
    return
  }
  busy.value = true
  error.value = ''
  try {
    await adminLogin(name.value, password.value)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/admin') ? route.query.next : '/admin'
    router.replace(next)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login">
    <TitleLogo compact />
    <form class="form" @submit.prevent="submit">
      <h1 class="shadow heading">Admin login</h1>
      <label for="name" class="shadow">Name</label>
      <McTextField id="name" v-model="name" autocomplete="username" autocapitalize="off" spellcheck="false" />
      <label for="pw" class="shadow">Password</label>
      <McTextField id="pw" v-model="password" type="password" autocomplete="current-password" :invalid="!!error" />
      <p v-if="error" class="error shadow" role="alert">{{ error }}</p>
      <McButton type="submit" :disabled="busy">{{ busy ? 'Logging in...' : 'Log in' }}</McButton>
      <McButton to="/">Back to shop</McButton>
    </form>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: calc(var(--u) * 12);
  padding: 16px;
}

.form {
  width: min(100%, calc(var(--u) * 200));
  display: flex;
  flex-direction: column;
  gap: calc(var(--u) * 3);
}

.heading {
  margin: 0 0 calc(var(--u) * 4);
  font-size: var(--font-size);
  font-weight: normal;
  text-align: center;
}

label {
  color: var(--gray);
}

.error {
  margin: 0;
  color: var(--red);
}
</style>
