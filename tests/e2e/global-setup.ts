import * as fs from 'fs'
import * as path from 'path'
import * as os from 'os'
import { spawn } from 'child_process'
import { E2E_PORT, E2E_JWT_SECRET, E2E_ADMIN_USER, E2E_ADMIN_PASS } from './playwright.config'

const DATA_DIR = path.join(os.tmpdir(), `ufshare-e2e-${Date.now()}`)
const BINARY = path.resolve(__dirname, '../../ufshare')
export const STATE_FILE = path.join(os.tmpdir(), 'ufshare-e2e-state.json')

async function waitReady(url: string, timeout = 15000) {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(url)
      if (resp.ok) return
    } catch {}
    await new Promise(r => setTimeout(r, 200))
  }
  throw new Error(`Server did not start within ${timeout}ms (${url})`)
}

export default async function globalSetup() {
  if (!fs.existsSync(BINARY)) {
    throw new Error(
      `ufshare binary not found at ${BINARY}.\n` +
      `Run first: go build -o ufshare ./cmd/ufshare`
    )
  }

  fs.mkdirSync(DATA_DIR, { recursive: true })

  const server = spawn(BINARY, [
    '--address', `127.0.0.1:${E2E_PORT}`,
    '--data', DATA_DIR,
  ], {
    env: { ...process.env, JWT_SECRET: E2E_JWT_SECRET },
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: false,
  })

  server.stderr?.on('data', (d: Buffer) => process.stderr.write(d))

  await waitReady(`http://127.0.0.1:${E2E_PORT}/npm/-/ping`)

  // 登录拿 token
  const loginResp = await fetch(`http://127.0.0.1:${E2E_PORT}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: E2E_ADMIN_USER, password: E2E_ADMIN_PASS }),
  })
  const loginBody: any = await loginResp.json()
  const token: string = loginBody?.data?.token ?? ''

  // 预热：通过代理缓存 lodash 和 axios（供 Web UI 展示）
  const base = `http://127.0.0.1:${E2E_PORT}`
  for (const pkg of ['lodash', 'axios']) {
    try {
      const r = await fetch(`${base}/npm/${pkg}`)
      if (!r.ok) console.warn(`[setup] seeding ${pkg} returned ${r.status}`)
      else await r.text()
      console.log(`[setup] seeded ${pkg}`)
    } catch (e) {
      console.warn(`[setup] failed to seed ${pkg}:`, e)
    }
  }

  fs.writeFileSync(STATE_FILE, JSON.stringify({
    pid: server.pid,
    dataDir: DATA_DIR,
    token,
  }))
}
