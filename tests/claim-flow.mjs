import { adminPassword as readAdminPassword } from './credentials.mjs';
import { chromium, expect, request } from '../frontend/node_modules/@playwright/test/index.mjs';
import fs from 'node:fs';
import crypto from 'node:crypto';
const base = process.env.BASE_URL || 'http://127.0.0.1:8088';
const adminPassword = readAdminPassword();
const http = await request.newContext();
async function call(path, token = '', method = 'GET', data) {
  const response = await http.fetch(base + '/api/v1' + path, { method, data, headers: { ...(token ? { Authorization: 'Bearer ' + token } : {}), 'Idempotency-Key': crypto.randomUUID() } });
  if (!response.ok()) throw Error(method + ' ' + path + ': HTTP ' + response.status());
  return response.json();
}
async function wait(operation, token) {
  for (let i = 0; i < 100; i++) {
    const op = await call('/operations/' + operation.id, token);
    if (op.status === 'FAILED') throw Error('test operation failed');
    if (op.status === 'SUCCEEDED') return op;
    await new Promise(resolve => setTimeout(resolve, 300));
  }
  throw Error('test operation timed out');
}
const suffix = Date.now().toString(36);
async function account(prefix) {
  const username = prefix + suffix;
  const user = await call('/auth/register', '', 'POST', { username, password: 'Test12', displayName: '认领交互验收' });
  const login = await call('/auth/login', '', 'POST', { username, password: 'Test12' });
  return { username, id: user.user.id, token: login.token };
}
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const errors = [];
async function loginPage(username, password) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  page.on('pageerror', e => errors.push(e.message));
  await page.goto(base + '/#/login');
  await page.getByLabel('用户名', { exact: true }).fill(username);
  await page.getByLabel('密码', { exact: true }).fill(password);
  await page.locator('form').getByRole('button', { name: '登录', exact: true }).click();
  await page.waitForURL(url => !url.hash.includes('/login'));
  return page;
}
try {
  const health = await call('/health');
  if (health.mode !== 'hyperledger-fabric' || !health.ok) throw Error('real Fabric required');
  const finder = await account('finderui'), claimant = await account('claimui'), other = await account('otherui'), manager = await account('managerui');
  const admin = (await call('/auth/login', '', 'POST', { username: 'admin', password: adminPassword })).token;
  const name = '认领交互验收物品 ' + suffix;
  const created = await wait(await call('/items', finder.token, 'POST', { content: { name, category: '生活用品', location: '虚拟验收区域', foundAt: new Date(Date.now() - 60000).toISOString(), description: '虚拟记录，用于认领交互与权限验收。', question: '验收代号是什么？' }, answer: 'Sample-Only' }), finder.token);
  const id = created.itemId;
  const page = await loginPage(claimant.username, 'Test12');
  await page.goto(base + '/#/items/' + id);
  await page.getByPlaceholder('请输入认领证明').fill('Sample-Only');
  await page.getByRole('button', { name: '提交申请', exact: true }).click();
  await page.waitForURL(url => url.hash.includes('/me?tab=claims'));
  await expect(page.getByRole('heading', { name: '认领申请已提交', exact: true })).toBeVisible();
  const card = page.locator('article').filter({ has: page.getByRole('heading', { name, exact: true }) });
  await expect(card).toContainText('待审核');
  await card.getByRole('link', { name, exact: true }).click();
  await expect(page.getByRole('heading', { name: '我的认领申请', exact: true })).toBeVisible();
  await expect(page.getByPlaceholder('请输入认领证明')).toHaveCount(0);
  await expect(page.getByRole('button', { name: '撤回认领', exact: true })).toBeVisible();
  const claimedState = await call('/items/' + id);
  const initialNotices = await call('/notifications', claimant.token);
  const claimNotice = initialNotices.items.find(n => n.itemId === id);
  if (claimNotice?.title !== '认领申请已提交，等待管理员审核') throw Error('incorrect claimant notice');
  const otherPage = await loginPage(other.username, 'Test12');
  await otherPage.goto(base + '/#/items/' + id);
  await expect(otherPage.getByPlaceholder('请输入认领证明')).toBeVisible();
  await wait(await call('/items/' + id + '/claims', other.token, 'POST', { evidence: 'Another-Sample', expectedVersion: claimedState.version }), other.token);
  const after = await call('/notifications', claimant.token);
  if (after.total !== initialNotices.total) throw Error('unrelated applicant notification leaked');
  const finderNotices = await call('/notifications', finder.token);
  if (!finderNotices.items.some(n => n.itemId === id && n.title === '收到新的认领申请')) throw Error('finder did not receive application');
  await page.goto(base + '/#/me?tab=notices');
  await expect(page.getByRole('heading', { name: /认领申请已提交，等待管理员审核/ })).toBeVisible();
  let releaseRead;
  const readGate = new Promise(resolve => { releaseRead = resolve; });
  await page.route('**/api/v1/notifications*', async route => { await readGate; await route.continue(); });
  await page.getByRole('button', { name: '我的认领', exact: true }).click();
  await page.getByRole('button', { name: '消息', exact: true }).click();
  await page.getByRole('button', { name: '账户设置', exact: true }).click();
  await page.getByLabel('显示名称', { exact: true }).fill('未保存的资料草稿');
  await expect(page.getByRole('button', { name: '更新资料', exact: true })).toBeEnabled();
  releaseRead();
  await page.unrouteAll({ behavior: 'wait' });
  await expect(page.getByLabel('显示名称', { exact: true })).toHaveValue('未保存的资料草稿');
  await otherPage.goto(base + '/#/admin');
  await expect(otherPage.getByText('需要管理员账号。', { exact: true })).toBeVisible();
  const forbidden = await http.get(base + '/api/v1/admin/stats', { headers: { Authorization: 'Bearer ' + other.token } });
  if (forbidden.status() !== 403) throw Error('ordinary user reached admin API');
  await call('/admin/users/' + manager.id, admin, 'PATCH', { role: 'ADMIN', reason: '虚拟验收授权业务管理员' });
  const managerPage = await loginPage(manager.username, 'Test12');
  await expect(managerPage.getByRole('heading', { name: '实际业务统计' })).toBeVisible();
  await expect(managerPage.getByRole('button', { name: '用户与权限', exact: true })).not.toBeVisible();
  const superPage = await loginPage('admin', adminPassword);
  await expect(superPage.getByRole('button', { name: '用户与权限', exact: true })).toBeVisible();
  if (errors.length) throw Error(errors.join('\n'));
  const mine = await call('/me/operations', claimant.token);
  const claimOp = mine.items.find(op => op.itemId === id && op.command.action === 'CLAIM');
  if (!/^[0-9a-f]{64}$/.test(claimOp?.txId || '')) throw Error('missing real claim transaction');
  const result = { ok: true, mode: health.mode, base, itemId: id, txId: claimOp.txId, checks: ['认领成功跳转与回执', '本人待审核不再显示填写框', '其他用户仍可并行申请', '申请人与登记人通知区分', '其他申请人不收到新申请通知', '读取消息不禁用资料按钮', '普通用户页面及接口拒绝', '业务管理员与超管菜单权限'] };
  fs.writeFileSync('artifacts/claim-flow-check.json', JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result));
  for (const action of ['CLOSE', 'HIDE']) {
    const current = await call('/items/' + id, admin);
    await wait(await call('/admin/items/' + id + '/actions', admin, 'POST', { action, note: '虚拟验收结束，归档测试记录', expectedVersion: current.version }), admin);
  }
  await call('/admin/users/' + manager.id, admin, 'PATCH', { role: 'USER', status: 'DISABLED', reason: '虚拟验收结束，撤销临时测试管理权限' });
} finally { await browser.close(); await http.dispose(); }
