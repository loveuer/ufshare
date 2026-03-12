import { useState, useEffect } from 'react'
import {
  Box, Button, CircularProgress, TextField, Typography, Alert,
  Divider, FormControlLabel, Switch, Tab, Tabs, Paper,
} from '@mui/material'
import StorageIcon from '@mui/icons-material/Storage'
import FolderIcon from '@mui/icons-material/Folder'
import TerminalIcon from '@mui/icons-material/Terminal'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import AccountTreeIcon from '@mui/icons-material/AccountTree'
import { settingApi } from '../api'

interface FieldConfig {
  key: string
  label: string
  hint: string
  placeholder?: string
  /** 若指定，此字段始终显示，不受 enabled 控制 */
  alwaysShow?: boolean
}

interface ModuleConfig {
  icon: React.ReactNode
  title: string
  /** 标识该模块专用端口是否启用的 setting key */
  enabledKey: string
  enabledLabel: string
  fields: FieldConfig[]
}

const MODULES: ModuleConfig[] = [
  {
    icon: <FolderIcon fontSize="small" />,
    title: 'File Store',
    enabledKey: 'file.enabled',
    enabledLabel: 'Enable dedicated port',
    fields: [
      {
        key: 'file.addr',
        label: 'Dedicated Listen Address',
        hint: 'Clients access files directly without the /file-store/ prefix',
        placeholder: '0.0.0.0:8001',
      },
    ],
  },
  {
    icon: <StorageIcon fontSize="small" />,
    title: 'npm',
    enabledKey: 'npm.enabled',
    enabledLabel: 'Enable dedicated port',
    fields: [
      {
        key: 'npm.upstream',
        label: 'Upstream Registry',
        hint: 'Proxy target for packages not cached locally',
        placeholder: 'https://registry.npmmirror.com',
        alwaysShow: true,
      },
      {
        key: 'npm.addr',
        label: 'Dedicated Listen Address',
        hint: 'npm clients point here directly: npm set registry http://host:4873',
        placeholder: '0.0.0.0:4873',
      },
    ],
  },
  {
    icon: <TerminalIcon fontSize="small" />,
    title: 'Go Modules',
    enabledKey: 'go.enabled',
    enabledLabel: 'Enable dedicated port',
    fields: [
      {
        key: 'go.upstream',
        label: 'Upstream Proxy',
        hint: 'Go proxy upstream servers, comma separated',
        placeholder: 'https://goproxy.cn,direct',
        alwaysShow: true,
      },
      {
        key: 'go.private',
        label: 'GOPRIVATE',
        hint: 'Modules that should not use the proxy (e.g., github.com/mycompany/*)',
        placeholder: 'github.com/mycompany/*',
        alwaysShow: true,
      },
      {
        key: 'go.addr',
        label: 'Dedicated Listen Address',
        hint: 'Go clients point here directly: export GOPROXY=http://host:8081',
        placeholder: '0.0.0.0:8081',
      },
    ],
  },
  {
    icon: <CloudDownloadIcon fontSize="small" />,
    title: 'Docker',
    enabledKey: 'oci.enabled',
    enabledLabel: 'Enable dedicated port',
    fields: [
      {
        key: 'oci.upstream',
        label: 'Upstream Registry',
        hint: 'Docker registry upstream for proxy/cache',
        placeholder: 'https://registry-1.docker.io',
        alwaysShow: true,
      },
      {
        key: 'oci.http_proxy',
        label: 'HTTP Proxy',
        hint: 'HTTP proxy for upstream connections',
        placeholder: 'http://proxy:8080',
        alwaysShow: true,
      },
      {
        key: 'oci.https_proxy',
        label: 'HTTPS Proxy',
        hint: 'HTTPS proxy for upstream connections',
        placeholder: 'http://proxy:8080',
        alwaysShow: true,
      },
      {
        key: 'oci.addr',
        label: 'Dedicated Listen Address',
        hint: 'Docker clients point here directly',
        placeholder: '0.0.0.0:5000',
      },
    ],
  },
  {
    icon: <AccountTreeIcon fontSize="small" />,
    title: 'Maven',
    enabledKey: 'maven.enabled',
    enabledLabel: 'Enable dedicated port',
    fields: [
      {
        key: 'maven.upstream',
        label: 'Upstream Repository',
        hint: 'Maven repository upstream for proxy/cache',
        placeholder: 'https://repo.maven.apache.org/maven2',
        alwaysShow: true,
      },
      {
        key: 'maven.addr',
        label: 'Dedicated Listen Address',
        hint: 'Maven clients point here directly',
        placeholder: '0.0.0.0:8082',
      },
    ],
  },
]

// 每个模块独立管理保存状态
interface ModuleStatus {
  saving: boolean
  success: boolean
  error: string
}

function defaultStatus(): ModuleStatus {
  return { saving: false, success: false, error: '' }
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [loading, setLoading]   = useState(false)
  const [tab, setTab]           = useState(0)
  const [status, setStatus]     = useState<ModuleStatus[]>(MODULES.map(defaultStatus))

  const load = async () => {
    setLoading(true)
    try {
      const res = await settingApi.getAll()
      setSettings(res.data.data)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const set = (key: string, value: string) =>
    setSettings((prev) => ({ ...prev, [key]: value }))

  const patchStatus = (idx: number, patch: Partial<ModuleStatus>) =>
    setStatus((prev) => prev.map((s, i) => i === idx ? { ...s, ...patch } : s))

  const handleSave = async (idx: number) => {
    const mod = MODULES[idx]
    // 只保存该模块相关的 keys
    const keys = [mod.enabledKey, ...mod.fields.map((f) => f.key)]
    const payload = Object.fromEntries(keys.map((k) => [k, settings[k] ?? '']))

    patchStatus(idx, { saving: true, success: false, error: '' })
    try {
      await settingApi.update(payload)
      patchStatus(idx, { saving: false, success: true })
      setTimeout(() => patchStatus(idx, { success: false }), 3000)
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message
      patchStatus(idx, { saving: false, error: msg || 'Failed to save settings' })
    }
  }

  if (loading) return <Box display="flex" justifyContent="center" mt={4}><CircularProgress /></Box>

  return (
    <Box>
      <Typography variant="h6" fontWeight="medium" mb={3}>Settings</Typography>

      <Paper variant="outlined" sx={{ display: 'flex', minHeight: 320 }}>
        {/* 左侧 Tab 列表 */}
        <Tabs
          orientation="vertical"
          value={tab}
          onChange={(_, v) => setTab(v)}
          sx={{
            borderRight: 1,
            borderColor: 'divider',
            minWidth: 140,
            pt: 1,
          }}
        >
          {MODULES.map((mod, idx) => (
            <Tab
              key={idx}
              icon={mod.icon as React.ReactElement}
              iconPosition="start"
              label={mod.title}
              sx={{ justifyContent: 'flex-start', minHeight: 48, px: 2, gap: 1 }}
            />
          ))}
        </Tabs>

        {/* 右侧内容区 */}
        {MODULES.map((mod, idx) => {
          if (tab !== idx) return null
          const enabled = settings[mod.enabledKey] === 'true'
          const st = status[idx]

          return (
            <Box key={idx} flex={1} p={3} display="flex" flexDirection="column" gap={2.5}>
              {/* 模块标题 */}
              <Box>
                <Typography variant="subtitle1" fontWeight="bold">
                  {mod.title} Module
                </Typography>
                <Divider sx={{ mt: 1 }} />
              </Box>

              {/* 状态提示 */}
              {st.success && (
                <Alert severity="success" sx={{ py: 0.5 }}>Saved successfully.</Alert>
              )}
              {st.error && (
                <Alert severity="error" sx={{ py: 0.5 }}>{st.error}</Alert>
              )}

              {/* Always-visible fields */}
              {mod.fields.filter((f) => f.alwaysShow).map((f) => (
                <TextField
                  key={f.key}
                  label={f.label}
                  helperText={f.hint}
                  placeholder={f.placeholder}
                  value={settings[f.key] ?? ''}
                  onChange={(e) => set(f.key, e.target.value)}
                  size="small"
                  fullWidth
                />
              ))}

              {/* Enable toggle */}
              <FormControlLabel
                control={
                  <Switch
                    checked={enabled}
                    onChange={(e) => set(mod.enabledKey, e.target.checked ? 'true' : 'false')}
                    size="small"
                  />
                }
                label={<Typography variant="body2">{mod.enabledLabel}</Typography>}
              />

              {/* Conditional fields (only when enabled) */}
              {enabled && mod.fields.filter((f) => !f.alwaysShow).map((f) => (
                <TextField
                  key={f.key}
                  label={f.label}
                  helperText={f.hint}
                  placeholder={f.placeholder}
                  value={settings[f.key] ?? ''}
                  onChange={(e) => set(f.key, e.target.value)}
                  size="small"
                  fullWidth
                />
              ))}

              <Box mt="auto" pt={1}>
                <Button
                  variant="contained"
                  size="small"
                  onClick={() => handleSave(idx)}
                  disabled={st.saving}
                >
                  {st.saving ? 'Saving…' : 'Save'}
                </Button>
              </Box>
            </Box>
          )
        })}
      </Paper>
    </Box>
  )
}
