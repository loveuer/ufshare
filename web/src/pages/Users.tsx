import { useState, useEffect } from 'react'
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  FormControlLabel, IconButton, Switch, TextField, Tooltip, Typography,
} from '@mui/material'
import { DataGrid, type GridColDef } from '@mui/x-data-grid'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import SecurityIcon from '@mui/icons-material/Security'
import type { User } from '../types'
import { userApi } from '../api'
import PermissionDialog from '../components/PermissionDialog'

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [loading, setLoading] = useState(false)
  const [editUser, setEditUser] = useState<User | null>(null)
  const [permUser, setPermUser] = useState<User | null>(null)
  const [editData, setEditData] = useState({ email: '', is_admin: false, status: 1 })

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
      field: 'actions', headerName: 'Actions', width: 130, sortable: false,
      renderCell: ({ row }) => (
        <Box>
          <Tooltip title="Permissions">
            <IconButton size="small" onClick={() => setPermUser(row)}><SecurityIcon fontSize="small" /></IconButton>
          </Tooltip>
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
      <Typography variant="h6" mb={2}>Users</Typography>
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

      {/* 权限对话框 */}
      {permUser && (
        <PermissionDialog user={permUser} onClose={() => setPermUser(null)} />
      )}
    </Box>
  )
}
