import { chromium, expect } from '../frontend/node_modules/@playwright/test/index.mjs';
import fs from 'node:fs';
const base=process.env.BASE_URL||'http://127.0.0.1:8088';
const browser=await chromium.launch({channel:'msedge',headless:true});
fs.mkdirSync('artifacts/carousel',{recursive:true});
const errors=[],checks=[];
async function open(options={}){
 const p=await browser.newPage(options);p.on('pageerror',e=>errors.push(e.message));await p.goto(base+'/#/');
 await expect(p.locator('.campus-hero-image.is-active')).toBeVisible();
 await expect.poll(()=>p.locator('.campus-hero-image.is-active').evaluate(e=>e.complete&&e.naturalWidth>0)).toBeTruthy();return p;
}
async function noOverflow(p){if(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1))throw Error('Horizontal overflow');}
try{
 const before=await (await fetch(base+'/api/v1/stats')).json();
 const p=await open({viewport:{width:1440,height:1000},deviceScaleFactor:2,colorScheme:'dark'});
 await expect(p.locator('body')).toHaveCSS('background-color','rgb(248, 246, 241)');
 await expect.poll(()=>p.locator('.campus-hero-image.is-active').evaluate(e=>e.currentSrc)).toContain('3840');
 for(let index=0;index<6;index++){
  await p.locator('.hero-carousel-dots button').nth(index).click();
  await expect(p.locator('.hero-carousel-dots button').nth(index)).toHaveAttribute('aria-current','true');
  await expect(p.locator('.campus-hero-image.is-active')).toHaveCSS('opacity','1');await noOverflow(p);
  await p.screenshot({path:`artifacts/carousel/desktop-${index+1}.png`,scale:'css'});
 }
 checks.push('六张用户照片逐一切换，高分屏选择3840px资源，无空白及脚本错误');
 await p.locator('.brand').focus();await p.mouse.move(0,0);
 await expect(p.locator('.hero-carousel-dots button').nth(0)).toHaveAttribute('aria-current','true',{timeout:10000});
 await p.getByRole('button',{name:'暂停轮播',exact:true}).click();
 await p.locator('.brand').focus();await p.mouse.move(0,0);
 await p.waitForTimeout(7000);await expect(p.locator('.hero-carousel-dots button').nth(0)).toHaveAttribute('aria-current','true');
 checks.push('自动循环及手动暂停生效');
 await p.goto(base+'/#/login');await expect(p.locator('body')).toHaveCSS('background-color','rgb(248, 246, 241)');
 await p.getByLabel('用户名',{exact:true}).fill('campus_user');await p.getByLabel('密码',{exact:true}).fill('sixsix');
 await p.screenshot({path:'artifacts/carousel/system-dark-stays-light.png',scale:'css'});await p.close();checks.push('系统设为深色时首页与登录页仍为浅色');
 const mobile=await open({viewport:{width:390,height:844},deviceScaleFactor:2,isMobile:true,hasTouch:true});
 await mobile.getByRole('button',{name:'下一张校园照片',exact:true}).tap();
 await expect(mobile.locator('.hero-carousel-dots button').nth(1)).toHaveAttribute('aria-current','true');
 await expect(mobile.locator('.campus-hero-image.is-active')).toHaveCSS('opacity','1');await noOverflow(mobile);
 await mobile.screenshot({path:'artifacts/carousel/mobile.png',scale:'css'});await mobile.close();checks.push('手机高分屏、手动切换与无横向溢出');
 const still=await open({reducedMotion:'reduce'});
 await expect(still.getByRole('button',{name:'暂停轮播',exact:true})).toHaveCount(0);
 await still.getByRole('button',{name:'下一张校园照片',exact:true}).click();
 await expect(still.locator('.hero-carousel-dots button').nth(1)).toHaveAttribute('aria-current','true');
 await still.evaluate(()=>scrollTo(0,350));await expect(still.locator('.campus-hero-image.is-active')).toHaveCSS('transform','none');await still.close();checks.push('减少动态效果时不自动轮播，仍可手动选图');
 const after=await (await fetch(base+'/api/v1/stats')).json();if(JSON.stringify(before)!==JSON.stringify(after))throw Error('Business data changed');
 if(errors.length)throw Error(errors.join('\n'));
 fs.writeFileSync('artifacts/carousel/check.json',JSON.stringify({ok:true,checks,businessWrites:0},null,2));console.log(JSON.stringify({ok:true,checks}));
}finally{await browser.close();}
