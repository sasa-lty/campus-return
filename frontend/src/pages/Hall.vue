<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from "vue";
import { api, safe } from "../api";
import ItemCard from "../components/ItemCard.vue";
import CampusHero from "../components/CampusHero.vue";
const filters = reactive({
  q: "",
  category: "",
  location: "",
  status: "",
  from: "",
  to: "",
  sort: "newest",
});
const data = ref<any>({ items: [], total: 0 });
const stats = ref<any>(null);
const page = ref(1);
const loading = ref(false);
const loaded = ref(false);
function browseHall() {
  document.getElementById("hall")?.scrollIntoView({
    behavior: matchMedia("(prefers-reduced-motion: reduce)").matches
      ? "auto"
      : "smooth",
  });
}
async function load() {
  if (loading.value) return;
  loading.value = true;
  try {
    await safe(
      async () => {
        const p = new URLSearchParams({
          ...filters,
          page: String(page.value),
          pageSize: "12",
        });
        [data.value, stats.value] = await Promise.all([
          api("/items?" + p),
          api("/stats"),
        ]);
        loaded.value = true;
      },
      { busy: false },
    );
  } finally {
    loading.value = false;
  }
}
function search() {
  page.value = 1;
  load();
}
let timer: any;
function refresh() {
  if (!document.hidden) load();
}
onMounted(() => {
  load();
  timer = setInterval(refresh, 15000);
  window.addEventListener("focus", refresh);
});
onUnmounted(() => {
  clearInterval(timer);
  window.removeEventListener("focus", refresh);
});
</script>
<template>
  <CampusHero />
  <div class="hall-content">
  <div class="metrics" v-if="stats">
    <span
      >公开物品 <b>{{ stats.counts.total }}</b></span
    ><span
      >等待认领 <b>{{ stats.counts.FOUND }}</b></span
    ><span
      >已归还 <b>{{ stats.counts.RETURNED }}</b></span
    >
  </div>
  <section id="hall" class="hall-section" aria-labelledby="hall-heading">
    <div class="section-head">
      <div>
        <p class="eyebrow">浏览与认领</p>
        <h2 id="hall-heading">招领大厅</h2>
      </div>
      <span class="hint" aria-live="polite">{{
        loading ? "正在更新…" : data.total + " 条记录"
      }}</span>
    </div>
    <form class="filters" @submit.prevent="search">
      <input
        v-model="filters.q"
        placeholder="搜索名称或描述"
        aria-label="搜索"
      /><input
        v-model="filters.location"
        placeholder="地点"
        aria-label="地点"
      /><select aria-label="分类" v-model="filters.category">
        <option value="">全部分类</option>
        <option>生活用品</option>
        <option>数码设备</option>
        <option>证件卡片</option>
        <option>书籍文具</option>
        <option>衣物配饰</option>
        <option>其他</option></select
      ><select v-model="filters.status">
        <option value="">全部状态</option>
        <option value="FOUND">待认领</option>
        <option value="CLAIM_PENDING">审核中</option>
        <option value="CLAIMED">已确权</option>
        <option value="RETURNED">已归还</option></select
      ><input type="date" v-model="filters.from" aria-label="起始日期" /><input
        type="date"
        v-model="filters.to"
        aria-label="结束日期"
      /><select v-model="filters.sort" aria-label="排序">
        <option value="newest">最近更新</option>
        <option value="oldest">较早更新</option>
        <option value="name">按名称排序</option></select
      ><button :disabled="loading">查找</button>
    </form>
    <div class="item-grid" :aria-busy="loading">
      <ItemCard v-for="item in data.items" :key="item.id" :item="item" />
    </div>
    <div v-if="loaded && !data.total" class="panel empty">
      <h3>暂时没有符合条件的物品</h3>
      <p>试试减少筛选条件，或登记你拾到的物品。</p>
    </div>
    <p v-if="!loaded && loading" class="empty" role="status">
      正在读取招领记录…
    </p>
    <div class="pagination">
      <button
        :disabled="page === 1"
        @click="
          page--;
          load();
        "
      >
        上一页</button
      ><span>{{ page }}</span
      ><button
        :disabled="page * 12 >= data.total"
        @click="
          page++;
          load();
        "
      >
        下一页
      </button>
    </div>
  </section>
  </div>
</template>
