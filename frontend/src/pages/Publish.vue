<script setup lang="ts">
import { useRouter } from "vue-router";
import ContentForm from "../components/ContentForm.vue";
import { safe, mutate, session } from "../api";
const router = useRouter();
async function submit(q: any) {
  await safe(async () => {
    const v = await mutate("/items", q);
    if (v.status === "SUCCEEDED") router.push("/items/" + v.itemId);
  });
}
</script>
<template>
  <section class="narrow">
    <h1>登记拾物</h1>
    <p>请如实填写拾到的信息，并保留可核验的物品细节。</p>
    <div v-if="session.user?.role === 'USER'" class="panel">
      <ContentForm @submit="submit" />
    </div>
    <p v-else>请使用普通用户账号登录后登记。</p>
  </section>
</template>
