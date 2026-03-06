import { useState, useEffect, useCallback } from 'react'
import {
  Box, Breadcrumbs, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogTitle, IconButton, LinearProgress, MenuItem, Select,
  Tooltip, Typography,
} from '@mui/material'
import { DataGrid, type GridColDef } from '@mui/x-data-grid'
import UploadIcon from '@mui/icons-material/Upload'
import DeleteIcon from '@mui/icons-material/Delete'
import DownloadIcon from '@mui/icons-material/Download'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import type { FileEntry, Module } from '../types'
import { moduleApi } from '../api'
import http from '../api/http'

export default function FilesPage() {
  const [modules, setModules] = useState<Module[]>([])
  const [selectedModule, setSelectedModule] = useState<Module | null>(null)
  const [files, setFiles] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [deleteTarget, setDeleteTarget] = useState<FileEntry | null>(null)

  useEffect(() => {
    moduleApi.list().then((res) => {
      const fileModules = res.data.data.filter((m) => m.type === 'file')
      setModules(fileModules)
      if (fileModules.length > 0) setSelectedModule(fileModules[0])
    })
  }, [])

  const loadFiles = useCallback(async () => {
    if (!selectedModule) return
    setLoading(true)
    try {
      const res = await http.get(`/api/v1/files/${selectedModule.name}`)
      setFiles(res.data.data ?? [])
    } finally {
      setLoading(false)
    }
  }, [selectedModule])

  useEffect(() => { loadFiles() }, [loadFiles])

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file || !selectedModule) return
    e.target.value = ''
    setUploading(true)
    setUploadProgress(0)
    try {
      await http.put(`/api/v1/files/${selectedModule.name}/${file.name}`, file, {
        headers: { 'Content-Type': 'application/octet-stream' },
        onUploadProgress: (ev) => {
          if (ev.total) setUploadProgress(Math.round((ev.loaded * 100) / ev.total))
        },
      })
      loadFiles()
    } finally {
      setUploading(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget || !selectedModule) return
    await http.delete(`/api/v1/files/${selectedModule.name}/${deleteTarget.path}`)
    setDeleteTarget(null)
    loadFiles()
  }

  const handleDownload = (entry: FileEntry) => {
    window.open(`/api/v1/files/${selectedModule?.name}/${entry.path}`, '_blank')
  }

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
    return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
  }

  const columns: GridColDef[] = [
    { field: 'path', headerName: 'Path', flex: 2 },
    {
      field: 'size', headerName: 'Size', width: 110,
      renderCell: ({ value }) => formatSize(value),
    },
    { field: 'mime_type', headerName: 'Type', width: 200 },
    { field: 'uploader', headerName: 'Uploader', width: 120 },
    {
      field: 'sha256', headerName: 'SHA256', width: 130,
      renderCell: ({ value }) => (
        <Tooltip title={value}>
          <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{value?.slice(0, 12)}…</span>
        </Tooltip>
      ),
    },
    {
      field: 'actions', headerName: '', width: 90, sortable: false,
      renderCell: ({ row }) => (
        <Box>
          <Tooltip title="Download">
            <IconButton size="small" onClick={() => handleDownload(row)}><DownloadIcon fontSize="small" /></IconButton>
          </Tooltip>
          <Tooltip title="Delete">
            <IconButton size="small" color="error" onClick={() => setDeleteTarget(row)}><DeleteIcon fontSize="small" /></IconButton>
          </Tooltip>
        </Box>
      ),
    },
  ]

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Breadcrumbs separator={<NavigateNextIcon fontSize="small" />}>
          <Typography color="text.primary" fontWeight="medium">Files</Typography>
          {selectedModule && <Chip label={selectedModule.name} size="small" color="default" />}
        </Breadcrumbs>

        <Box display="flex" gap={1} alignItems="center">
          {modules.length > 0 && (
            <Select
              size="small"
              value={selectedModule?.name ?? ''}
              onChange={(e) => setSelectedModule(modules.find((m) => m.name === e.target.value) ?? null)}
              sx={{ minWidth: 160 }}
            >
              {modules.map((m) => (
                <MenuItem key={m.id} value={m.name}>{m.name}</MenuItem>
              ))}
            </Select>
          )}
          <Button
            variant="contained"
            startIcon={<UploadIcon />}
            component="label"
            disabled={!selectedModule || uploading}
          >
            Upload
            <input type="file" hidden onChange={handleUpload} />
          </Button>
        </Box>
      </Box>

      {uploading && <LinearProgress variant="determinate" value={uploadProgress} sx={{ mb: 1 }} />}

      {modules.length === 0 ? (
        <Typography color="text.secondary">
          No file modules found. Create a module of type <code>file</code> first.
        </Typography>
      ) : (
        <DataGrid
          rows={files}
          columns={columns}
          loading={loading}
          autoHeight
          disableRowSelectionOnClick
        />
      )}

      <Dialog open={!!deleteTarget} onClose={() => setDeleteTarget(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Delete File</DialogTitle>
        <DialogContent>
          <Typography>
            Delete <strong>{deleteTarget?.path}</strong>? This cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)}>Cancel</Button>
          <Button variant="contained" color="error" onClick={handleDelete}>Delete</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
