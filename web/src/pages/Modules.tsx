import { useState, useEffect } from 'react'
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  FormControlLabel, IconButton, MenuItem, Switch, TextField, Tooltip, Typography,
} from '@mui/material'
import { DataGrid, type GridColDef } from '@mui/x-data-grid'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import type { Module } from '../types'
import { moduleApi } from '../api'

const MODULE_TYPES = ['file', 'npm', 'go', 'pypi', 'maven']

const typeColor: Record<string, 'default' | 'primary' | 'secondary' | 'success' | 'warning'> = {
  file: 'default',
  npm: 'warning',
  go: 'primary',
  pypi: 'success',
  maven: 'secondary',
}

export default function ModulesPage() {
  const [modules, setModules] = useState<Module[]>([])
  const [loading, setLoading] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editModule, setEditModule] = useState<Module | null>(null)
  const [form, setForm] = useState({ name: '', type: 'file', description: '', public_read: false, public_write: false })

  const load = async () => {
    setLoading(true)
    try {
      const res = await moduleApi.list()
      setModules(res.data.data)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const openCreate = () => {
    setEditModule(null)
    setForm({ name: '', type: 'file', description: '', public_read: false, public_write: false })
    setDialogOpen(true)
  }

  const openEdit = (m: Module) => {
    setEditModule(m)
    setForm({ name: m.name, type: m.type, description: m.description, public_read: m.public_read, public_write: m.public_write })
    setDialogOpen(true)
  }

  const handleSave = async () => {
    if (editModule) {
      await moduleApi.update(editModule.id, { description: form.description, public_read: form.public_read, public_write: form.public_write })
    } else {
      await moduleApi.create(form)
    }
    setDialogOpen(false)
    load()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this module?')) return
    await moduleApi.delete(id)
    load()
  }

  const columns: GridColDef[] = [
    { field: 'id', headerName: 'ID', width: 70 },
    { field: 'name', headerName: 'Name', flex: 1 },
    {
      field: 'type', headerName: 'Type', width: 100,
      renderCell: ({ value }) => (
        <Chip label={value} color={typeColor[value] ?? 'default'} size="small" />
      ),
    },
    { field: 'description', headerName: 'Description', flex: 1.5 },
    {
      field: 'public_read', headerName: 'Public Read', width: 110,
      renderCell: ({ value }) => <Chip label={value ? 'Yes' : 'No'} color={value ? 'success' : 'default'} size="small" />,
    },
    {
      field: 'public_write', headerName: 'Public Write', width: 110,
      renderCell: ({ value }) => <Chip label={value ? 'Yes' : 'No'} color={value ? 'warning' : 'default'} size="small" />,
    },
    {
      field: 'actions', headerName: 'Actions', width: 100, sortable: false,
      renderCell: ({ row }) => (
        <Box>
          <Tooltip title="Edit">
            <IconButton size="small" onClick={() => openEdit(row)}><EditIcon fontSize="small" /></IconButton>
          </Tooltip>
          <Tooltip title="Delete">
            <IconButton size="small" color="error" onClick={() => handleDelete(row.id)}><DeleteIcon fontSize="small" /></IconButton>
          </Tooltip>
        </Box>
      ),
    },
  ]

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="h6">Modules</Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>New Module</Button>
      </Box>
      <DataGrid
        rows={modules}
        columns={columns}
        loading={loading}
        autoHeight
        disableRowSelectionOnClick
      />

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>{editModule ? 'Edit Module' : 'New Module'}</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            <TextField
              label="Name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              disabled={!!editModule}
              required
            />
            <TextField
              select
              label="Type"
              value={form.type}
              onChange={(e) => setForm({ ...form, type: e.target.value })}
              disabled={!!editModule}
            >
              {MODULE_TYPES.map((t) => <MenuItem key={t} value={t}>{t}</MenuItem>)}
            </TextField>
            <TextField
              label="Description"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
            <FormControlLabel
              control={<Switch checked={form.public_read} onChange={(e) => setForm({ ...form, public_read: e.target.checked })} />}
              label="Public Read"
            />
            <FormControlLabel
              control={<Switch checked={form.public_write} onChange={(e) => setForm({ ...form, public_write: e.target.checked })} />}
              label="Public Write"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleSave}>Save</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
