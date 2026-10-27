<script setup lang="ts">
import { ref, onMounted } from "vue";
import QRCode from "qrcode";
const qr = ref("");
const address = location.origin;
const local = /^(localhost|127\.0\.0\.1|\[::1\])$/.test(location.hostname);
onMounted(async () => {
  if (!local) qr.value = await QRCode.toDataURL(address, { width: 320, margin: 2 });
});
</script>
<template>
  <section class="narrow panel share-page">
    <h1>手机访问</h1>
    <p v-if="!local">用手机相机扫描二维码，即可打开平台。</p>
    <p v-if="local" class="warning">
      当前为本机地址，仅能在这台电脑访问。手机访问需要可共享的平台地址。
    </p>
    <img v-if="!local" :src="qr" alt="平台入口二维码" />
    <p class="break">{{ address }}</p>

    <a v-if="!local" :href="qr" download="校园失物招领平台-体验二维码.png">下载二维码</a>
  </section>
</template>
