import * as fs from 'fs'
import { STATE_FILE } from './global-setup'

export default async function globalTeardown() {
  try {
    const state = JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8'))

    // 停止服务进程
    if (state.pid) {
      try { process.kill(state.pid, 'SIGTERM') } catch {}
    }

    // 清理测试数据目录
    if (state.dataDir && fs.existsSync(state.dataDir)) {
      fs.rmSync(state.dataDir, { recursive: true, force: true })
      console.log(`[teardown] removed data dir: ${state.dataDir}`)
    }

    fs.unlinkSync(STATE_FILE)
  } catch (e) {
    console.warn('[teardown] warning:', e)
  }
}
