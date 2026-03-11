import { useEffect, useState } from 'react'
import {
  Box, Card, CardContent, Typography, Alert,
  CircularProgress, Paper, Grid, IconButton, Tooltip,
  Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle,
  Button,
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import DeleteIcon from '@mui/icons-material/Delete'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import { goApi } from '../api'
import type { GoCacheStats } from '../types'

export default function GoPage() {
  const [stats, setStats] = useState<GoCacheStats | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [cleanDialogOpen, setCleanDialogOpen] = useState(false)
  const [cleaning, setCleaning] = useState(false)
  const [copied, setCopied] = useState(false)

  const loadStats = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await goApi.getStats()
      setStats(res.data.data)
    } catch (err: unknown) {
      setError('Failed to load cache stats')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStats()
  }, [])

  const handleCleanCache = async () => {
    setCleaning(true)
    try {
      await goApi.cleanCache()
      await loadStats()
      setCleanDialogOpen(false)
    } catch (err: unknown) {
      setError('Failed to clean cache')
    } finally {
      setCleaning(false)
    }
  }

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const getProxyUrl = () => {
    const baseUrl = window.location.origin
    return `${baseUrl}/go`
  }

  const copyProxyUrl = () => {
    navigator.clipboard.writeText(getProxyUrl())
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Box>
      <Typography variant="h5" fontWeight="bold" mb={2}>
        Go Module Proxy
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Grid container spacing={3}>
        {/* 代理地址卡片 */}
        <Grid size={{ xs: 12 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Proxy URL
              </Typography>
              <Paper
                variant="outlined"
                sx={{
                  p: 2,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  bgcolor: 'grey.50',
                }}
              >
                <Typography
                  variant="body1"
                  fontFamily="monospace"
                  sx={{ wordBreak: 'break-all' }}
                >
                  {getProxyUrl()}
                </Typography>
                <Tooltip title={copied ? 'Copied!' : 'Copy URL'}>
                  <IconButton onClick={copyProxyUrl} color={copied ? 'success' : 'default'}>
                    <ContentCopyIcon />
                  </IconButton>
                </Tooltip>
              </Paper>
              <Typography variant="body2" color="text.secondary" mt={1}>
                Set this URL as your{' '}
                <code style={{ background: '#f5f5f5', padding: '2px 4px', borderRadius: 4 }}>
                  GOPROXY
                </code>{' '}
                environment variable
              </Typography>
              <Paper
                variant="outlined"
                sx={{
                  p: 1.5,
                  mt: 1,
                  bgcolor: 'grey.900',
                  color: 'grey.100',
                  fontFamily: 'monospace',
                  fontSize: '0.875rem',
                }}
              >
                export GOPROXY={getProxyUrl()}
              </Paper>
            </CardContent>
          </Card>
        </Grid>

        {/* 缓存统计 */}
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
                <Typography variant="h6">Cache Statistics</Typography>
                <Box>
                  <Tooltip title="Refresh">
                    <IconButton onClick={loadStats} disabled={loading} size="small">
                      <RefreshIcon />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title="Clean Cache">
                    <IconButton
                      onClick={() => setCleanDialogOpen(true)}
                      disabled={loading || !stats || stats.file_count === 0}
                      size="small"
                      color="error"
                    >
                      <DeleteIcon />
                    </IconButton>
                  </Tooltip>
                </Box>
              </Box>

              {loading ? (
                <Box display="flex" justifyContent="center" py={4}>
                  <CircularProgress />
                </Box>
              ) : stats ? (
                <Grid container spacing={2}>
                  <Grid size={{ xs: 6 }}>
                    <Typography variant="body2" color="text.secondary">
                      Cache Size
                    </Typography>
                    <Typography variant="h6">{formatBytes(stats.size_bytes)}</Typography>
                  </Grid>
                  <Grid size={{ xs: 6 }}>
                    <Typography variant="body2" color="text.secondary">
                      Files Cached
                    </Typography>
                    <Typography variant="h6">{stats.file_count}</Typography>
                  </Grid>
                  <Grid size={{ xs: 12 }}>
                    <Typography variant="body2" color="text.secondary">
                      Cache Directory
                    </Typography>
                    <Typography variant="body2" fontFamily="monospace">
                      {stats.cache_dir}
                    </Typography>
                  </Grid>
                  <Grid size={{ xs: 12 }}>
                    <Typography variant="body2" color="text.secondary">
                      Upstream
                    </Typography>
                    <Typography variant="body2" fontFamily="monospace">
                      {stats.upstream || 'https://goproxy.cn,direct'}
                    </Typography>
                  </Grid>
                  {stats.goprivate && (
                    <Grid size={{ xs: 12 }}>
                      <Typography variant="body2" color="text.secondary">
                        GOPRIVATE
                      </Typography>
                      <Typography variant="body2" fontFamily="monospace">
                        {stats.goprivate}
                      </Typography>
                    </Grid>
                  )}
                </Grid>
              ) : (
                <Typography color="text.secondary">No cache data available</Typography>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* 使用说明 */}
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Usage Guide
              </Typography>

              <Box display="flex" flexDirection="column" gap={2}>
                <Box>
                  <Typography variant="subtitle2" gutterBottom>
                    1. Configure Go to use this proxy
                  </Typography>
                  <Paper
                    variant="outlined"
                    sx={{
                      p: 1.5,
                      bgcolor: 'grey.900',
                      color: 'grey.100',
                      fontFamily: 'monospace',
                      fontSize: '0.875rem',
                    }}
                  >
                    go env -w GOPROXY={getProxyUrl()}
                  </Paper>
                </Box>

                <Box>
                  <Typography variant="subtitle2" gutterBottom>
                    2. Or set environment variable temporarily
                  </Typography>
                  <Paper
                    variant="outlined"
                    sx={{
                      p: 1.5,
                      bgcolor: 'grey.900',
                      color: 'grey.100',
                      fontFamily: 'monospace',
                      fontSize: '0.875rem',
                    }}
                  >
                    export GOPROXY={getProxyUrl()}
                  </Paper>
                </Box>

                <Box>
                  <Typography variant="subtitle2" gutterBottom>
                    3. For private modules, also set GOPRIVATE
                  </Typography>
                  <Paper
                    variant="outlined"
                    sx={{
                      p: 1.5,
                      bgcolor: 'grey.900',
                      color: 'grey.100',
                      fontFamily: 'monospace',
                      fontSize: '0.875rem',
                    }}
                  >
                    go env -w GOPRIVATE=github.com/mycompany/*
                  </Paper>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* 清理缓存确认对话框 */}
      <Dialog open={cleanDialogOpen} onClose={() => setCleanDialogOpen(false)}>
        <DialogTitle>Clean Cache</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to clean all cached Go modules? This action cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCleanDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleCleanCache}
            color="error"
            disabled={cleaning}
          >
            {cleaning ? <CircularProgress size={20} /> : 'Clean'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
