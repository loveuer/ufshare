import { useState, useEffect } from 'react'
import {
  Box, Chip, Dialog, DialogTitle, DialogContent, DialogActions,
  Button, FormControlLabel, Switch, IconButton, Tooltip, Typography,
} from '@mui/material'
import { DataGrid, type GridColDef } from '@mui/x-data-grid'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import EditIcon from '@mui/icons-material/Edit'
import type { Permission, Module, User } from '../types'
import { permissionApi, moduleApi } from '../api'

interface Props {
  user: User
  onClose: () => void
}

export default function PermissionDialog({ user, onClose }: Props) {
  const [perms, setPerms] = useState<Permission[]>([])
  const [modules, setModules] = useState<Module[]>([])
  const [grantOpen, setGrantOpen] = useState(false)
  const [editPerm, setEditPerm] = useState<Permission | null>(null)
  const [selectedModule, setSelectedModule] = useState<number>(0)
  const [canRead, setCanRead] = useState(true)
  const [canWrite, setCanWrite] = useState(false)

  const load = async () => {
    const [permRes, modRes] = await Promise.all([
      permissionApi.getUserPermissions(user.id),
      moduleApi.list(),
    ])
    setPerms(permRes.data.data)
    setModules(modRes.data.data)
  }

  useEffect(() => { load() }, [])

  const openGrant = (perm?: Permission) => {
    if (perm) {
      setEditPerm(perm)
      setSelectedModule(perm.module_id)
      setCanRead(perm.can_read)
      setCanWrite(perm.can_write)
    } else {
      setEditPerm(null)
      setSelectedModule(modules[0]?.id ?? 0)
      setCanRead(true)
      setCanWrite(false)
    }
    setGrantOpen(true)
  }

  const handleGrant = async () => {
    await permissionApi.grant(user.id, selectedModule, canRead, canWrite)
    setGrantOpen(false)
    load()
  }

  const handleRevoke = async (moduleId: number) => {
    if (!confirm('Revoke this permission?')) return
    await permissionApi.revoke(user.id, moduleId)
    load()
  }

  const columns: GridColDef[] = [
    { field: 'module_id', headerName: 'Module ID', width: 90 },
    {
      field: 'module', headerName: 'Module', flex: 1,
      valueGetter: (_val, row) => row.module?.name ?? row.module_id,
    },
    {
      field: 'can_read', headerName: 'Read', width: 90,
      renderCell: ({ value }) => <Chip label={value ? 'Yes' : 'No'} color={value ? 'success' : 'default'} size="small" />,
    },
    {
      field: 'can_write', headerName: 'Write', width: 90,
      renderCell: ({ value }) => <Chip label={value ? 'Yes' : 'No'} color={value ? 'warning' : 'default'} size="small" />,
    },
    {
      field: 'actions', headerName: 'Actions', width: 100, sortable: false,
      renderCell: ({ row }) => (
        <Box>
          <Tooltip title="Edit">
            <IconButton size="small" onClick={() => openGrant(row)}><EditIcon fontSize="small" /></IconButton>
          </Tooltip>
          <Tooltip title="Revoke">
            <IconButton size="small" color="error" onClick={() => handleRevoke(row.module_id)}><DeleteIcon fontSize="small" /></IconButton>
          </Tooltip>
        </Box>
      ),
    },
  ]

  const grantedModuleIds = perms.map((p) => p.module_id)
  const availableModules = editPerm
    ? modules
    : modules.filter((m) => !grantedModuleIds.includes(m.id))

  return (
    <>
      <Dialog open onClose={onClose} maxWidth="md" fullWidth>
        <DialogTitle>
          <Box display="flex" justifyContent="space-between" alignItems="center">
            <span>Permissions: {user.username}</span>
            <Button size="small" variant="outlined" startIcon={<AddIcon />} onClick={() => openGrant()}>
              Grant
            </Button>
          </Box>
        </DialogTitle>
        <DialogContent>
          <DataGrid
            rows={perms}
            columns={columns}
            autoHeight
            disableRowSelectionOnClick
            sx={{ mt: 1 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={grantOpen} onClose={() => setGrantOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>{editPerm ? 'Edit Permission' : 'Grant Permission'}</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            <Box>
              <Typography variant="body2" color="text.secondary" mb={0.5}>Module</Typography>
              <select
                value={selectedModule}
                onChange={(e) => setSelectedModule(Number(e.target.value))}
                style={{ width: '100%', padding: '8px', borderRadius: '4px', border: '1px solid #ccc' }}
                disabled={!!editPerm}
              >
                {availableModules.map((m) => (
                  <option key={m.id} value={m.id}>{m.name} ({m.type})</option>
                ))}
              </select>
            </Box>
            <FormControlLabel
              control={<Switch checked={canRead} onChange={(e) => setCanRead(e.target.checked)} />}
              label="Can Read"
            />
            <FormControlLabel
              control={<Switch checked={canWrite} onChange={(e) => setCanWrite(e.target.checked)} />}
              label="Can Write"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setGrantOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleGrant}>Save</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
