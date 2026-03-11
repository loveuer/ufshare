import { useEffect, useState } from 'react'
import {
  Box, Card, CardContent, Typography, Alert,
  CircularProgress, Paper, Grid, IconButton, Tooltip, Table,
  TableBody, TableCell, TableContainer, TableHead, TableRow,
  TablePagination, Collapse, Dialog, DialogActions, DialogContent,
  DialogContentText, DialogTitle, Button,
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import DeleteIcon from '@mui/icons-material/Delete'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import SearchIcon from '@mui/icons-material/Search'
import TextField from '@mui/material/TextField'
import { ociApi } from '../api'
import type { OciRepository, OciTagInfo, OciCacheStats } from '../types'

export default function DockerPage() {
  const [stats, setStats] = useState<OciCacheStats | null>(null)
  const [repos, setRepos] = useState<OciRepository[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(20)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)
  const [expandedRepo, setExpandedRepo] = useState<string | null>(null)
  const [repoTags, setRepoTags] = useState<OciTagInfo[]>([])
  const [tagsLoading, setTagsLoading] = useState(false)
  const [cleanDialogOpen, setCleanDialogOpen] = useState(false)
  const [cleaning, setCleaning] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<number | null>(null)

  const loadStats = async () => {
    try {
      const res = await ociApi.getStats()
      setStats(res.data.data)
    } catch {
      // ignore
    }
  }

  const loadRepos = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await ociApi.listRepos(page + 1, rowsPerPage, search)
      setRepos(res.data.data || [])
      setTotal((res.data as unknown as { total: number }).total || 0)
    } catch {
      setError('Failed to load repositories')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStats()
    loadRepos()
  }, [])

  useEffect(() => {
    loadRepos()
  }, [page, rowsPerPage])

  const handleSearch = () => {
    setPage(0)
    loadRepos()
  }

  const handleExpandRepo = async (name: string) => {
    if (expandedRepo === name) {
      setExpandedRepo(null)
      return
    }
    setExpandedRepo(name)
    setTagsLoading(true)
    try {
      const res = await ociApi.listTags(name)
      setRepoTags(res.data.data || [])
    } catch {
      setRepoTags([])
    } finally {
      setTagsLoading(false)
    }
  }

  const handleDeleteRepo = async (id: number) => {
    try {
      await ociApi.deleteRepo(id)
      setDeleteDialogOpen(null)
      loadRepos()
      loadStats()
    } catch {
      setError('Failed to delete repository')
    }
  }

  const handleCleanCache = async () => {
    setCleaning(true)
    try {
      await ociApi.cleanCache()
      await loadStats()
      await loadRepos()
      setCleanDialogOpen(false)
    } catch {
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

  const getRegistryUrl = () => {
    return window.location.host
  }

  const copyText = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Box>
      <Typography variant="h5" fontWeight="bold" mb={2}>
        Docker Registry
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Grid container spacing={3}>
        {/* Registry URL */}
        <Grid size={{ xs: 12 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>Registry URL</Typography>
              <Paper
                variant="outlined"
                sx={{ p: 2, display: 'flex', alignItems: 'center', justifyContent: 'space-between', bgcolor: 'grey.50' }}
              >
                <Typography variant="body1" fontFamily="monospace" sx={{ wordBreak: 'break-all' }}>
                  {getRegistryUrl()}
                </Typography>
                <Tooltip title={copied ? 'Copied!' : 'Copy'}>
                  <IconButton onClick={() => copyText(getRegistryUrl())} color={copied ? 'success' : 'default'}>
                    <ContentCopyIcon />
                  </IconButton>
                </Tooltip>
              </Paper>
              <Typography variant="body2" color="text.secondary" mt={1}>
                Configure Docker daemon to use this registry as a mirror
              </Typography>
              <Paper
                variant="outlined"
                sx={{ p: 1.5, mt: 1, bgcolor: 'grey.900', color: 'grey.100', fontFamily: 'monospace', fontSize: '0.875rem', whiteSpace: 'pre' }}
              >
{`# /etc/docker/daemon.json
{
  "registry-mirrors": ["http://${getRegistryUrl()}"],
  "insecure-registries": ["${getRegistryUrl()}"]
}`}
              </Paper>
            </CardContent>
          </Card>
        </Grid>

        {/* Cache Stats */}
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
                <Typography variant="h6">Cache Statistics</Typography>
                <Box>
                  <Tooltip title="Refresh">
                    <IconButton onClick={() => { loadStats(); loadRepos() }} size="small">
                      <RefreshIcon />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title="Clean Cache">
                    <IconButton
                      onClick={() => setCleanDialogOpen(true)}
                      size="small"
                      color="error"
                      disabled={!stats || stats.repo_count === 0}
                    >
                      <DeleteIcon />
                    </IconButton>
                  </Tooltip>
                </Box>
              </Box>

              {stats ? (
                <Grid container spacing={2}>
                  <Grid size={{ xs: 6 }}>
                    <Typography variant="body2" color="text.secondary">Repositories</Typography>
                    <Typography variant="h6">{stats.repo_count}</Typography>
                  </Grid>
                  <Grid size={{ xs: 6 }}>
                    <Typography variant="body2" color="text.secondary">Tags</Typography>
                    <Typography variant="h6">{stats.tag_count}</Typography>
                  </Grid>
                  <Grid size={{ xs: 6 }}>
                    <Typography variant="body2" color="text.secondary">Cached Blobs</Typography>
                    <Typography variant="h6">{stats.blob_count}</Typography>
                  </Grid>
                  <Grid size={{ xs: 6 }}>
                    <Typography variant="body2" color="text.secondary">Total Size</Typography>
                    <Typography variant="h6">{formatBytes(stats.size_bytes)}</Typography>
                  </Grid>
                </Grid>
              ) : (
                <Typography color="text.secondary">Loading...</Typography>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Upstream Info */}
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>Upstream</Typography>
              {stats ? (
                <Box>
                  <Typography variant="body2" color="text.secondary">
                    Registry
                  </Typography>
                  <Typography variant="body2" fontFamily="monospace" gutterBottom>
                    {stats.upstream || 'https://registry-1.docker.io'}
                  </Typography>
                </Box>
              ) : (
                <Typography color="text.secondary">Loading...</Typography>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Repository List */}
        <Grid size={{ xs: 12 }}>
          <Card>
            <CardContent>
              <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
                <Typography variant="h6">Repositories</Typography>
                <Box display="flex" gap={1} alignItems="center">
                  <TextField
                    size="small"
                    placeholder="Search..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                    sx={{ width: 200 }}
                  />
                  <IconButton onClick={handleSearch} size="small">
                    <SearchIcon />
                  </IconButton>
                </Box>
              </Box>

              {loading ? (
                <Box display="flex" justifyContent="center" py={4}>
                  <CircularProgress />
                </Box>
              ) : repos.length === 0 ? (
                <Typography color="text.secondary" textAlign="center" py={4}>
                  No repositories found. Images will appear here after the first docker pull through this proxy.
                </Typography>
              ) : (
                <>
                  <TableContainer>
                    <Table size="small">
                      <TableHead>
                        <TableRow>
                          <TableCell width={40} />
                          <TableCell>Repository</TableCell>
                          <TableCell align="right">Tags</TableCell>
                          <TableCell align="right">Cached Blobs</TableCell>
                          <TableCell align="right">Size</TableCell>
                          <TableCell align="right">Actions</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {repos.map((repo) => (
                          <>
                            <TableRow
                              key={repo.id}
                              hover
                              sx={{ cursor: 'pointer' }}
                              onClick={() => handleExpandRepo(repo.name)}
                            >
                              <TableCell>
                                {expandedRepo === repo.name ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
                              </TableCell>
                              <TableCell>
                                <Typography variant="body2" fontFamily="monospace" fontWeight="bold">
                                  {repo.name}
                                </Typography>
                                <Typography variant="caption" color="text.secondary">
                                  {repo.upstream}
                                </Typography>
                              </TableCell>
                              <TableCell align="right">{repo.tag_count}</TableCell>
                              <TableCell align="right">{repo.cached_blob_count}</TableCell>
                              <TableCell align="right">{formatBytes(repo.total_size)}</TableCell>
                              <TableCell align="right">
                                <Tooltip title="Delete">
                                  <IconButton
                                    size="small"
                                    color="error"
                                    onClick={(e) => { e.stopPropagation(); setDeleteDialogOpen(repo.id) }}
                                  >
                                    <DeleteIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                              </TableCell>
                            </TableRow>
                            <TableRow key={`${repo.id}-tags`}>
                              <TableCell colSpan={6} sx={{ py: 0, border: expandedRepo === repo.name ? undefined : 'none' }}>
                                <Collapse in={expandedRepo === repo.name} timeout="auto" unmountOnExit>
                                  <Box sx={{ py: 2, pl: 4 }}>
                                    {tagsLoading ? (
                                      <CircularProgress size={20} />
                                    ) : repoTags.length === 0 ? (
                                      <Typography variant="body2" color="text.secondary">No tags</Typography>
                                    ) : (
                                      <Table size="small">
                                        <TableHead>
                                          <TableRow>
                                            <TableCell>Tag</TableCell>
                                            <TableCell>Digest</TableCell>
                                            <TableCell>Media Type</TableCell>
                                            <TableCell align="right">Size</TableCell>
                                            <TableCell>Created</TableCell>
                                          </TableRow>
                                        </TableHead>
                                        <TableBody>
                                          {repoTags.map((tag) => (
                                            <TableRow key={tag.tag}>
                                              <TableCell>
                                                <Typography variant="body2" fontFamily="monospace">{tag.tag}</Typography>
                                              </TableCell>
                                              <TableCell>
                                                <Tooltip title={tag.manifest_digest}>
                                                  <Typography variant="body2" fontFamily="monospace" sx={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                                                    {tag.manifest_digest.substring(0, 19)}...
                                                  </Typography>
                                                </Tooltip>
                                              </TableCell>
                                              <TableCell>
                                                <Typography variant="caption">
                                                  {tag.media_type.replace('application/vnd.docker.distribution.', '').replace('application/vnd.oci.image.', 'oci.')}
                                                </Typography>
                                              </TableCell>
                                              <TableCell align="right">{formatBytes(tag.size)}</TableCell>
                                              <TableCell>
                                                <Typography variant="caption">
                                                  {new Date(tag.created_at).toLocaleDateString()}
                                                </Typography>
                                              </TableCell>
                                            </TableRow>
                                          ))}
                                        </TableBody>
                                      </Table>
                                    )}

                                    <Box mt={1}>
                                      <Typography variant="caption" color="text.secondary">
                                        Pull: <code style={{ background: '#f5f5f5', padding: '2px 4px', borderRadius: 4 }}>
                                          docker pull {getRegistryUrl()}/{repo.name}:{'<tag>'}
                                        </code>
                                      </Typography>
                                    </Box>
                                  </Box>
                                </Collapse>
                              </TableCell>
                            </TableRow>
                          </>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                  <TablePagination
                    component="div"
                    count={total}
                    page={page}
                    onPageChange={(_, p) => setPage(p)}
                    rowsPerPage={rowsPerPage}
                    onRowsPerPageChange={(e) => { setRowsPerPage(parseInt(e.target.value, 10)); setPage(0) }}
                    rowsPerPageOptions={[10, 20, 50]}
                  />
                </>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Clean Cache Dialog */}
      <Dialog open={cleanDialogOpen} onClose={() => setCleanDialogOpen(false)}>
        <DialogTitle>Clean Docker Cache</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to clean all cached Docker images? This will remove all repositories, tags, manifests, and blobs. This action cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCleanDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleCleanCache} color="error" disabled={cleaning}>
            {cleaning ? <CircularProgress size={20} /> : 'Clean All'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete Repo Dialog */}
      <Dialog open={deleteDialogOpen !== null} onClose={() => setDeleteDialogOpen(null)}>
        <DialogTitle>Delete Repository</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete this repository and all its cached data?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialogOpen(null)}>Cancel</Button>
          <Button onClick={() => deleteDialogOpen && handleDeleteRepo(deleteDialogOpen)} color="error">
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
