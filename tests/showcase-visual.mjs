import { chromium, expect } from '../frontend/node_modules/@playwright/test/index.mjs';
import fs from 'node:fs';
const base=process.env.BASE_URL||'http://127.0.0.1:8088';
const browser=await chromium.launch({channel:'msedge',headless:true});
fs.mkdirSync('artifacts/showcase',{recursive:true});
const errors=[],checks=[];
try {
 for(const [name,width,height] of [['desktop',1440,1000],['mobile',390,844]]){
  const p=await browser.newPage({viewport:{width,height}});
  p.on('pageerror',e=>errors.push(e.message));
  await p.goto(base+'/#/');
  await expect(p.locator('.campus-hero-image.is-active')).toBeVisible();
  await expect.poll(()=>p.locator('.campus-hero-image.is-active').evaluate(e=>e.naturalWidth)).toBeGreaterThan(0);
  await expect(p.locator('body')).not.toContainText('账本服务已接入');
  if(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1))throw Error(name+' horizontal overflow');
  await p.screenshot({path:`artifacts/showcase/${name}-home.png`});
  await p.locator('.campus-hero .btn-primary').click();
  await expect.poll(()=>p.locator('#hall').evaluate(e=>Math.round(e.getBoundingClientRect().top))).toBeLessThan(220);
  await expect.poll(()=>p.locator('.campus-hero').evaluate(e=>Number(e.style.getPropertyValue('--journey')))).toBeGreaterThan(0);
  await p.screenshot({path:`artifacts/showcase/${name}-hall.png`});
  await p.goto(base+'/#/login');
  for(const mode of ['注册','忘记密码','登录'])await p.locator('.auth .tabs').getByRole('button',{name:mode,exact:true}).click();
  await expect(p.getByText('管理员如何登录与开通？')).toHaveCount(0);
  await p.getByLabel('用户名',{exact:true}).fill('campus_user');
  await p.getByLabel('密码',{exact:true}).fill('sixsix');
  await p.screenshot({path:`artifacts/showcase/${name}-login.png`});
  await p.goto(base+'/#/share');await expect(p.getByRole('heading',{name:'手机访问',exact:true})).toBeVisible();
  await expect(p.getByAltText('平台入口二维码')).toHaveCount(0);
  checks.push(name+' 首页/本地图片/大厅直达/滚动/登录页签/手机入口/无横向溢出');
  await p.close();
 }
 const p=await browser.newPage({reducedMotion:'reduce'});await p.goto(base+'/#/');await p.evaluate(()=>scrollTo(0,400));
 await expect.poll(()=>p.locator('.campus-hero-image.is-active').evaluate(e=>getComputedStyle(e).transform)).toBe('none');checks.push('减少动态效果采用静态画面');await p.close();
 const dark=await browser.newPage({colorScheme:'dark',viewport:{width:1440,height:1000}});await dark.goto(base+'/#/login');await expect(dark.getByLabel('用户名',{exact:true})).toBeVisible();
 await dark.getByLabel('用户名',{exact:true}).fill('campus_user');await dark.getByLabel('密码',{exact:true}).fill('sixsix');
 const colors=await dark.locator('.auth label').first().evaluate(e=>({fg:getComputedStyle(e).color,bg:getComputedStyle(e.closest('.auth')).backgroundColor}));
 await expect(dark.locator('body')).toHaveCSS('background-color','rgb(248, 246, 241)');
 await dark.screenshot({path:'artifacts/showcase/dark-login.png'});await dark.close();checks.push('系统深色偏好下仍保持浅色，表单已填充字段清晰');
 if(errors.length)throw Error(errors.join('\n'));
 fs.writeFileSync('artifacts/showcase/visual-check.json',JSON.stringify({ok:true,checks},null,2));console.log(JSON.stringify({ok:true,checks}));
} finally { await browser.close(); }
