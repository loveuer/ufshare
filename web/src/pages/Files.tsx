import { useState, useEffect, useCallback } from 'react'
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogTitle, IconButton, LinearProgress, Tooltip, Typography,
} from '@mui/material'
import { DataGrid, type GridColDef } from '@mui/x-data-grid'
import UploadIcon from '@mui/icons-material/Upload'
import DeleteIcon from '@mui/icons-material/Delete'
import DownloadIcon from '@mui/icons-material/Download'
import FolderIcon from '@mui/icons-material/Folder'
import type { FileEntry } from '../types'

export default function FilesPage() {
  const [files, setFiles] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [deleteTarget, setDeleteTarget] = useState<FileEntry | null>(null)

  const loadFiles = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetch('/file-store')
      const json = await res.json()
      setFiles(json.data ?? [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadFiles() }, [loadFiles])

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    e.target.value = ''
    setUploading(true)
    setUploadProgress(0)

    const xhr = new XMLHttpRequest()
    xhr.upload.onprogress = (ev) => {
      if (ev.total) setUploadProgress(Math.round((ev.loaded * 100) / ev.total))
    }
    xhr.onload = () => {
      setUploading(false)
      if (xhr.status >= 200 && xhr.status < 300) {
        loadFiles()
      } else {
        alert(`Upload failed: ${xhr.status} ${xhr.statusText}`)
      }
    }
    xhr.onerror = () => { setUploading(false); alert('Upload failed: network error') }

    const token = localStorage.getItem('token')
    xhr.open('PUT', `/file-store/${encodeURIComponent(file.name)}`)
    xhr.setRequestHeader('Content-Type', 'application/octet-stream')
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    xhr.send(file)
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    const token = localStorage.getItem('token')
    await fetch(`/file-store/${deleteTarget.path}`, {
      method: 'DELETE',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    setDeleteTarget(null)
    loadFiles()
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
            <IconButton size="small" onClick={() => window.open(`/file-store/${row.path}`, '_blank')}>
              <DownloadIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title="Delete">
            <IconButton size="small" color="error" onClick={() => setDeleteTarget(row)}>
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
      ),
    },
  ]

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Box display="flex" alignItems="center" gap={1}>
          <FolderIcon color="action" />
          <Typography fontWeight="medium">File Store</Typography>
          <Chip label={`${files.length} files`} size="small" />
        </Box>
        <Button
          variant="contained"
          startIcon={<UploadIcon />}
          component="label"
          disabled={uploading}
        >
          Upload
          <input type="file" hidden onChange={handleUpload} />
        </Button>
      </Box>

      {uploading && <LinearProgress variant="determinate" value={uploadProgress} sx={{ mb: 1 }} />}

      <DataGrid
        rows={files}
        columns={columns}
        loading={loading}
        autoHeight
        disableRowSelectionOnClick
      />

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
