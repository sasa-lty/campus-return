import fs from 'node:fs';
export function adminPassword() {
  if (process.env.ADMIN_PASSWORD) return process.env.ADMIN_PASSWORD;
  const file = process.env.CAMPUS_CREDENTIALS_FILE;
  if (file) {
    const match = fs.readFileSync(file, 'utf8').match(/^export ADMIN_PASSWORD=(.+)$/m);
    if (match) return match[1].trim();
  }
  throw new Error('请设置 ADMIN_PASSWORD 或 CAMPUS_CREDENTIALS_FILE，勿将凭据写入测试源码。');
}
export function managerAccount() {
  const username = process.env.MANAGER_USERNAME, password = process.env.MANAGER_PASSWORD;
  if (!username || !password) throw new Error('此专项测试需设置 MANAGER_USERNAME 和 MANAGER_PASSWORD。');
  return { username, password };
}
