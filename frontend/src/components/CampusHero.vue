<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from "vue";
import { session } from "../api";
import gate from "../assets/campus/gate-1920.webp";
import gateLarge from "../assets/campus/gate-3840.webp";
import lake from "../assets/campus/lake-panorama-1920.webp";
import lakeLarge from "../assets/campus/lake-panorama-3840.webp";
import square from "../assets/campus/sunset-square-1920.webp";
import squareLarge from "../assets/campus/sunset-square-3840.webp";
import sunset from "../assets/campus/lake-sunset-1920.webp";
import sunsetLarge from "../assets/campus/lake-sunset-3840.webp";
import lotus from "../assets/campus/lotus-lake-1920.webp";
import lotusLarge from "../assets/campus/lotus-lake-3840.webp";
import trails from "../assets/campus/crescent-trails-1920.webp";
import trailsLarge from "../assets/campus/crescent-trails-3840.webp";
const slides = [
  { src: gate, large: gateLarge, alt: "晚霞下的紫金港校门", position: "55% 58%" },
  { src: lake, large: lakeLarge, alt: "湖畔建筑与落日倒影", position: "59% 54%" },
  { src: square, large: squareLarge, alt: "夕阳下的校园广场", position: "53% 59%" },
  { src: sunset, large: sunsetLarge, alt: "晚霞与湖畔月牙楼", position: "55% 56%" },
  { src: lotus, large: lotusLarge, alt: "荷叶映衬的校园湖景", position: "60% 56%" },
  { src: trails, large: trailsLarge, alt: "暮色中的月牙楼与车轨", position: "58% 58%" },
];
const hero = ref<HTMLElement>();
const active = ref(0), paused = ref(false), hovering = ref(false), focused = ref(false);
const prepared = ref(slides.map((_, i) => i === 0));
const loading = ref(false), loadError = ref(false);
const reduced = ref(false);
let frame = 0;
const motion = matchMedia("(prefers-reduced-motion: reduce)");
let timer: ReturnType<typeof setTimeout> | undefined;
let cleanup: ReturnType<typeof setTimeout> | undefined;
let observer: IntersectionObserver | undefined;
let visible = true, alive = true;
const loaded = new Map<number, Promise<void>>();
function srcset(i: number) { return `${slides[i].src} 1920w, ${slides[i].large} 3840w`; }
function preload(index: number) {
  if (!loaded.has(index)) {
    const promise = new Promise<void>((resolve, reject) => {
      const image = new Image();
      image.sizes = "108vw";
      image.srcset = srcset(index);
      image.src = slides[index].src;
      image.decode().then(() => resolve(), reject);
    }).catch((error) => { loaded.delete(index); throw error; });
    loaded.set(index, promise);
  }
  return loaded.get(index)!;
}
function schedule() {
  clearTimeout(timer);
  if (!alive || paused.value || hovering.value || focused.value || motion.matches || document.hidden || !visible) return;
  timer = setTimeout(() => { void go(active.value + 1); }, 6500);
}
function firstLoaded() {
  if (!alive) return;
  void preload((active.value + 1) % slides.length).catch(() => {});
  schedule();
}
async function go(index: number) {
  if (!alive || loading.value) return;
  const target = (index + slides.length) % slides.length;
  if (target === active.value) return;
  clearTimeout(timer);
  clearTimeout(cleanup);
  loading.value = true;
  loadError.value = false;
  try {
    // 新照片解码后才切换，避免网络慢时出现空白；离屏照片及时卸载。
    await preload(target);
    if (!alive) return;
    prepared.value[target] = true;
    await nextTick();
    if (!alive) return;
    hero.value?.getBoundingClientRect();
    active.value = target;
    cleanup = setTimeout(() => { prepared.value = slides.map((_, i) => i === active.value); }, 1300);
    void preload((target + 1) % slides.length).catch(() => {});
  } catch { if (alive) loadError.value = true; }
  finally { if (alive) { loading.value = false; schedule(); } }
}
function togglePause() { paused.value = !paused.value; schedule(); }
function focusOut(event: FocusEvent) {
  if (!hero.value?.contains(event.relatedTarget as Node | null)) { focused.value = false; schedule(); }
}
function update() {
  frame = 0;
  const el = hero.value;
  if (!el) return;
  const progress = motion.matches ? 0 : Math.max(0, Math.min(1, -el.getBoundingClientRect().top / el.offsetHeight));
  el.style.setProperty("--journey", String(progress));
}
function scroll() { if (!frame) frame = requestAnimationFrame(update); }
function preference() { reduced.value = motion.matches; scroll(); schedule(); }
onMounted(() => {
  reduced.value = motion.matches;
  update();
  window.addEventListener("scroll", scroll, { passive: true });
  motion.addEventListener("change", preference);
  document.addEventListener("visibilitychange", schedule);
  observer = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; schedule(); });
  if (hero.value) observer.observe(hero.value);
  schedule();
});
onUnmounted(() => {
  alive = false; clearTimeout(timer); clearTimeout(cleanup); cancelAnimationFrame(frame); observer?.disconnect();
  window.removeEventListener("scroll", scroll); motion.removeEventListener("change", preference); document.removeEventListener("visibilitychange", schedule);
});
</script>
<template>
  <section ref="hero" class="campus-hero" aria-labelledby="home-heading" aria-roledescription="轮播图"
    @mouseenter="hovering = true; schedule()" @mouseleave="hovering = false; schedule()"
    @focusin="focused = true; schedule()" @focusout="focusOut">
    <template v-for="(slide, index) in slides" :key="slide.src">
      <img v-if="prepared[index]" class="campus-hero-image" :class="{ 'is-active': active === index }"
        :src="slide.src" :srcset="srcset(index)" sizes="108vw" :alt="slide.alt" :aria-hidden="active !== index"
        :style="{ objectPosition: slide.position }" :fetchpriority="index === 0 ? 'high' : 'auto'"
        @load="index === active && firstLoaded()" />
    </template>
    <div class="campus-hero-shade" aria-hidden="true"></div>
    <div class="campus-hero-copy">
      <p class="hero-kicker">校园互助 · 失物招领</p>
      <h1 id="home-heading">让遗失的日常，<br />重新回到身边。</h1>
      <p class="hero-lead">在校园里发现、认领与交还，<br class="mobile-break" />每一份善意都有回应。</p>
      <div class="hero-actions">
        <RouterLink :to="{ path: '/', hash: '#hall' }" class="btn btn-primary">浏览招领大厅 <span aria-hidden="true">↓</span></RouterLink>
        <RouterLink :to="session.user ? '/publish' : '/login'" class="btn hero-secondary">登记拾物</RouterLink>
      </div>
    </div>
    <div class="hero-carousel-controls" aria-label="校园照片切换">
      <button type="button" aria-label="上一张校园照片" :disabled="loading" @click="go(active - 1)">‹</button>
      <div class="hero-carousel-dots">
        <button v-for="(slide, index) in slides" :key="slide.src" type="button" :aria-label="`第${index + 1}张：${slide.alt}`"
          :aria-current="active === index ? 'true' : undefined" :disabled="loading" @click="go(index)"><span /></button>
      </div>
      <button type="button" aria-label="下一张校园照片" :disabled="loading" @click="go(active + 1)">›</button>
      <span class="hero-slide-count" aria-hidden="true">{{ String(active + 1).padStart(2, '0') }} / 06</span>
      <button v-if="!reduced" type="button" :aria-label="paused ? '播放轮播' : '暂停轮播'" :aria-pressed="paused" @click="togglePause">{{ paused ? '▶' : 'Ⅱ' }}</button>
    </div>
    <p v-if="loadError" class="hero-carousel-error" role="status">照片暂未加载，请再次切换。</p>
    <RouterLink :to="{ path: '/', hash: '#hall' }" class="hero-scroll" aria-label="下滑进入招领大厅"><span>发现身边的拾物</span><span aria-hidden="true">↓</span></RouterLink>
  </section>
</template>
