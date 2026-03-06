import { useState, useEffect, useCallback } from 'react'
import {
  Box, Chip, Collapse, IconButton, Paper, Table, TableBody,
  TableCell, TableContainer, TableHead, TableRow, Tooltip,
  Typography, CircularProgress, Alert, Stack,
} from '@mui/material'
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown'
import KeyboardArrowRightIcon from '@mui/icons-material/KeyboardArrowRight'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CloudIcon from '@mui/icons-material/Cloud'
import StorageIcon from '@mui/icons-material/Storage'
import type { NpmPackage, NpmVersion } from '../types'
import { npmApi } from '../api'

// ── 版本子表 ──────────────────────────────────────────────────────────────────

function VersionRows({ name }: { name: string }) {
  const [versions, setVersions] = useState<NpmVersion[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    npmApi.listVersions(name)
      .then(res => setVersions(res.data.data ?? []))
      .catch(() => setError('Failed to load versions'))
      .finally(() => setLoading(false))
  }, [name])

  const formatSize = (b: number) => {
    if (!b) return '-'
    if (b < 1024) return `${b} B`
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
    return `${(b / 1024 / 1024).toFixed(1)} MB`
  }

  if (loading) return (
    <TableRow><TableCell colSpan={6}><CircularProgress size={20} sx={{ m: 1 }} /></TableCell></TableRow>
  )
  if (error) return (
    <TableRow><TableCell colSpan={6}><Alert severity="error" sx={{ py: 0 }}>{error}</Alert></TableCell></TableRow>
  )

  return (
    <>
      {versions.map(v => (
        <TableRow key={v.version} sx={{ bgcolor: 'action.hover' }}>
          <TableCell sx={{ pl: 6 }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography variant="body2" fontFamily="monospace">{v.version}</Typography>
            </Stack>
          </TableCell>
          <TableCell>
            <Chip
              size="small"
              icon={v.cached ? <StorageIcon /> : <CloudIcon />}
              label={v.cached ? 'Cached' : 'Proxy only'}
              color={v.cached ? 'success' : 'default'}
              variant="outlined"
            />
          </TableCell>
          <TableCell>{formatSize(v.size)}</TableCell>
          <TableCell>
            {v.uploader
              ? <Chip label={v.uploader} size="small" color="primary" variant="outlined" />
              : <Typography variant="body2" color="text.secondary">upstream</Typography>}
          </TableCell>
          <TableCell>
            <Tooltip title={v.shasum}>
              <Typography variant="body2" fontFamily="monospace" fontSize={11}>
                {v.shasum ? v.shasum.slice(0, 12) + '…' : '-'}
              </Typography>
            </Tooltip>
          </TableCell>
          <TableCell>
            <Typography variant="body2" color="text.secondary">{v.created_at}</Typography>
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}

// ── 包行 ──────────────────────────────────────────────────────────────────────

function PackageRow({ pkg, registryURL }: { pkg: NpmPackage; registryURL: string }) {
  const [open, setOpen] = useState(false)

  const installCmd = `npm install ${pkg.name} --registry ${registryURL}`

  const copy = (text: string) => navigator.clipboard.writeText(text)

  const latest = pkg.dist_tags?.latest ?? ''

  return (
    <>
      <TableRow hover sx={{ cursor: 'pointer' }} onClick={() => setOpen(o => !o)}>
        <TableCell width={32} padding="none" sx={{ pl: 1 }}>
          <IconButton size="small">
            {open ? <KeyboardArrowDownIcon fontSize="small" /> : <KeyboardArrowRightIcon fontSize="small" />}
          </IconButton>
        </TableCell>

        <TableCell>
          <Typography fontWeight="medium" fontFamily="monospace">{pkg.name}</Typography>
          {pkg.description && (
            <Typography variant="caption" color="text.secondary">{pkg.description}</Typography>
          )}
        </TableCell>

        <TableCell>
          {latest && <Chip label={`latest: ${latest}`} size="small" color="primary" />}
          {Object.entries(pkg.dist_tags ?? {})
            .filter(([k]) => k !== 'latest')
            .map(([k, v]) => (
              <Chip key={k} label={`${k}: ${v}`} size="small" sx={{ ml: 0.5 }} />
            ))}
        </TableCell>

        <TableCell align="center">
          <Chip label={`${pkg.version_count} versions`} size="small" variant="outlined" />
        </TableCell>

        <TableCell align="center">
          <Chip
            label={`${pkg.cached_count} cached`}
            size="small"
            color={pkg.cached_count > 0 ? 'success' : 'default'}
            variant="outlined"
          />
        </TableCell>

        <TableCell onClick={e => e.stopPropagation()}>
          <Tooltip title={installCmd}>
            <Box
              sx={{
                display: 'flex', alignItems: 'center', gap: 0.5,
                bgcolor: 'action.selected', borderRadius: 1, px: 1, py: 0.5,
                fontFamily: 'monospace', fontSize: 12, cursor: 'text',
                maxWidth: 320, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}
            >
              <Typography variant="body2" fontFamily="monospace" noWrap sx={{ flexGrow: 1, fontSize: 11 }}>
                {installCmd}
              </Typography>
              <IconButton size="small" onClick={() => copy(installCmd)} sx={{ p: 0.25 }}>
                <ContentCopyIcon sx={{ fontSize: 14 }} />
              </IconButton>
            </Box>
          </Tooltip>
        </TableCell>
      </TableRow>

      <TableRow>
        <TableCell colSpan={7} padding="none">
          <Collapse in={open} unmountOnExit>
            <Table size="small">
              <TableHead>
                <TableRow sx={{ bgcolor: 'background.default' }}>
                  <TableCell sx={{ pl: 6 }}>Version</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Size</TableCell>
                  <TableCell>Publisher</TableCell>
                  <TableCell>SHA1</TableCell>
                  <TableCell>Date</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {open && <VersionRows name={pkg.name} />}
              </TableBody>
            </Table>
          </Collapse>
        </TableCell>
      </TableRow>
    </>
  )
}

// ── 主页面 ────────────────────────────────────────────────────────────────────

export default function NpmPage() {
  const [packages, setPackages] = useState<NpmPackage[]>([])
  const [loading, setLoading] = useState(false)

  // 从当前页面地址推断 registry URL
  const registryURL = `${window.location.origin}/npm`

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await npmApi.listPackages()
      setPackages(res.data.data ?? [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Box display="flex" alignItems="center" gap={1}>
          <Typography fontWeight="medium">npm Registry</Typography>
          <Chip label={`${packages.length} packages`} size="small" />
        </Box>
        <Tooltip title="npm 配置地址">
          <Box
            sx={{
              bgcolor: 'action.selected', borderRadius: 1, px: 1.5, py: 0.5,
              fontFamily: 'monospace', fontSize: 12, display: 'flex', alignItems: 'center', gap: 1,
            }}
          >
            <Typography variant="body2" fontFamily="monospace" fontSize={12}>
              {registryURL}
            </Typography>
            <IconButton size="small" sx={{ p: 0.25 }}
              onClick={() => navigator.clipboard.writeText(`npm set registry ${registryURL}`)}>
              <ContentCopyIcon sx={{ fontSize: 14 }} />
            </IconButton>
          </Box>
        </Tooltip>
      </Box>

      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell width={32} />
              <TableCell>Package</TableCell>
              <TableCell>Tags</TableCell>
              <TableCell align="center">Versions</TableCell>
              <TableCell align="center">Cached</TableCell>
              <TableCell>Install</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading
              ? <TableRow><TableCell colSpan={6} align="center"><CircularProgress size={24} sx={{ m: 2 }} /></TableCell></TableRow>
              : packages.length === 0
                ? <TableRow><TableCell colSpan={6} align="center">
                    <Typography color="text.secondary" sx={{ py: 3 }}>
                      No packages yet. Use <code>npm publish --registry {registryURL}</code> to publish, or install any package to proxy &amp; cache it.
                    </Typography>
                  </TableCell></TableRow>
                : packages.map(pkg => (
                    <PackageRow key={pkg.name} pkg={pkg} registryURL={registryURL} />
                  ))
            }
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}
