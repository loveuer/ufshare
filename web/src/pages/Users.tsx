import { useState, useEffect } from 'react'
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  FormControlLabel, IconButton, Switch, TextField, Tooltip, Typography,
} from '@mui/material'
import { DataGrid, type GridColDef } from '@mui/x-data-grid'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import type { User } from '../types'
import { userApi } from '../api'

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [loading, setLoading] = useState(false)

  // 编辑
  const [editUser, setEditUser] = useState<User | null>(null)
  const [editData, setEditData] = useState({ email: '', is_admin: false, status: 1 })

  // 新建
  const [createOpen, setCreateOpen] = useState(false)
  const [createData, setCreateData] = useState({ username: '', password: '', email: '', is_admin: false })
  const [createError, setCreateError] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const res = await userApi.list(page + 1, 20)
      setUsers(res.data.data.items)
      setTotal(res.data.data.total)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [page])

  const handleEdit = (user: User) => {
    setEditUser(user)
    setEditData({ email: user.email, is_admin: user.is_admin, status: user.status })
  }

  const handleSave = async () => {
    if (!editUser) return
    await userApi.update(editUser.id, editData)
    setEditUser(null)
    load()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this user?')) return
    await userApi.delete(id)
    load()
  }

  const handleCreate = async () => {
    setCreateError('')
    if (!createData.username || !createData.password) {
      setCreateError('Username and password are required')
      return
    }
    try {
      await userApi.create(createData)
      setCreateOpen(false)
      setCreateData({ username: '', password: '', email: '', is_admin: false })
      load()
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message
      setCreateError(msg || 'Failed to create user')
    }
  }

  const columns: GridColDef[] = [
    { field: 'id', headerName: 'ID', width: 70 },
    { field: 'username', headerName: 'Username', flex: 1 },
    { field: 'email', headerName: 'Email', flex: 1 },
    {
      field: 'is_admin', headerName: 'Role', width: 100,
      renderCell: ({ value }) => (
        <Chip label={value ? 'Admin' : 'User'} color={value ? 'primary' : 'default'} size="small" />
      ),
    },
    {
      field: 'status', headerName: 'Status', width: 100,
      renderCell: ({ value }) => (
        <Chip label={value === 1 ? 'Active' : 'Disabled'} color={value === 1 ? 'success' : 'error'} size="small" />
      ),
    },
    {
      field: 'actions', headerName: 'Actions', width: 100, sortable: false,
      renderCell: ({ row }) => (
        <Box>
          <Tooltip title="Edit">
            <IconButton size="small" onClick={() => handleEdit(row)}><EditIcon fontSize="small" /></IconButton>
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
        <Typography variant="h6">Users</Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateOpen(true)}>
          New User
        </Button>
      </Box>
      <DataGrid
        rows={users}
        columns={columns}
        loading={loading}
        rowCount={total}
        pageSizeOptions={[20]}
        paginationModel={{ page, pageSize: 20 }}
        paginationMode="server"
        onPaginationModelChange={(m) => setPage(m.page)}
        autoHeight
        disableRowSelectionOnClick
      />

      {/* 新建对话框 */}
      <Dialog open={createOpen} onClose={() => { setCreateOpen(false); setCreateError('') }} maxWidth="xs" fullWidth>
        <DialogTitle>New User</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            {createError && (
              <Typography color="error" variant="body2">{createError}</Typography>
            )}
            <TextField
              label="Username *"
              value={createData.username}
              onChange={(e) => setCreateData({ ...createData, username: e.target.value })}
            />
            <TextField
              label="Password *"
              type="password"
              value={createData.password}
              onChange={(e) => setCreateData({ ...createData, password: e.target.value })}
            />
            <TextField
              label="Email"
              value={createData.email}
              onChange={(e) => setCreateData({ ...createData, email: e.target.value })}
            />
            <FormControlLabel
              control={<Switch checked={createData.is_admin} onChange={(e) => setCreateData({ ...createData, is_admin: e.target.checked })} />}
              label="Admin"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setCreateOpen(false); setCreateError('') }}>Cancel</Button>
          <Button variant="contained" onClick={handleCreate}>Create</Button>
        </DialogActions>
      </Dialog>

      {/* 编辑对话框 */}
      <Dialog open={!!editUser} onClose={() => setEditUser(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Edit User: {editUser?.username}</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            <TextField
              label="Email"
              value={editData.email}
              onChange={(e) => setEditData({ ...editData, email: e.target.value })}
            />
            <FormControlLabel
              control={<Switch checked={editData.is_admin} onChange={(e) => setEditData({ ...editData, is_admin: e.target.checked })} />}
              label="Admin"
            />
            <FormControlLabel
              control={<Switch checked={editData.status === 1} onChange={(e) => setEditData({ ...editData, status: e.target.checked ? 1 : 0 })} />}
              label="Active"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditUser(null)}>Cancel</Button>
          <Button variant="contained" onClick={handleSave}>Save</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

