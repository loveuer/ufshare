import { type ReactNode, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  AppBar, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle,
  Drawer, List, ListItemButton, ListItemIcon, ListItemText,
  TextField, Toolbar, Typography, Menu, MenuItem,
  Avatar, Divider,
} from '@mui/material'
import PeopleIcon from '@mui/icons-material/People'
import FolderIcon from '@mui/icons-material/Folder'
import LogoutIcon from '@mui/icons-material/Logout'
import LockIcon from '@mui/icons-material/Lock'
import ViewModuleIcon from '@mui/icons-material/ViewModule'
import StorageIcon from '@mui/icons-material/Storage'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import SettingsIcon from '@mui/icons-material/Settings'
import AccountCircleIcon from '@mui/icons-material/AccountCircle'
import MavenIcon from '@mui/icons-material/AccountTree'
import { useAuth } from '../store/auth'
import { authApi } from '../api'

const DRAWER_WIDTH = 200

const navItems = [
  { label: 'File Store', path: '/files', icon: <FolderIcon /> },
  { label: 'npm', path: '/npm', icon: <ViewModuleIcon /> },
  { label: 'Go Modules', path: '/go', icon: <StorageIcon /> },
  { label: 'Docker', path: '/docker', icon: <CloudDownloadIcon /> },
  { label: 'Maven', path: '/maven', icon: <MavenIcon /> },
  { label: 'Users', path: '/users', icon: <PeopleIcon /> },
  { label: 'Settings', path: '/settings', icon: <SettingsIcon /> },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  const [pwdOpen, setPwdOpen] = useState(false)
  const [pwdData, setPwdData] = useState({ old: '', new_: '', confirm: '' })
  const [pwdError, setPwdError] = useState('')
  const [pwdSuccess, setPwdSuccess] = useState(false)

  // User menu state
  const [userMenuAnchor, setUserMenuAnchor] = useState<null | HTMLElement>(null)
  const userMenuOpen = Boolean(userMenuAnchor)

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const handleChangePwd = async () => {
    setPwdError('')
    if (!pwdData.old || !pwdData.new_) { setPwdError('All fields are required'); return }
    if (pwdData.new_.length < 6) { setPwdError('New password must be at least 6 characters'); return }
    if (pwdData.new_ !== pwdData.confirm) { setPwdError('Passwords do not match'); return }
    try {
      await authApi.changePassword(pwdData.old, pwdData.new_)
      setPwdSuccess(true)
      setTimeout(() => { setPwdOpen(false); setPwdSuccess(false); setPwdData({ old: '', new_: '', confirm: '' }) }, 1500)
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message
      setPwdError(msg || 'Failed to change password')
    }
  }

  const closePwdDialog = () => {
    setPwdOpen(false)
    setPwdError('')
    setPwdSuccess(false)
    setPwdData({ old: '', new_: '', confirm: '' })
  }

  const handleUserMenuClick = (event: React.MouseEvent<HTMLElement>) => {
    setUserMenuAnchor(event.currentTarget)
  }

  const handleUserMenuClose = () => {
    setUserMenuAnchor(null)
  }

  const handleOpenChangePassword = () => {
    handleUserMenuClose()
    setPwdOpen(true)
  }

  const handleLogoutClick = () => {
    handleUserMenuClose()
    handleLogout()
  }

  return (
    <Box display="flex">
      <AppBar position="fixed" sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
        <Toolbar>
          <Typography variant="h6" fontWeight="bold" sx={{ flexGrow: 1 }}>
            UFShare
          </Typography>

          {/* User menu button */}
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              cursor: 'pointer',
              borderRadius: 1,
              px: 1,
              py: 0.5,
              '&:hover': { bgcolor: 'rgba(255,255,255,0.1)' },
            }}
            onClick={handleUserMenuClick}
          >
            <Avatar sx={{ width: 32, height: 32, mr: 1, bgcolor: 'primary.light' }}>
              <AccountCircleIcon />
            </Avatar>
            <Typography variant="body2" sx={{ mr: 0.5 }}>
              {user?.username}
            </Typography>
          </Box>

          {/* User dropdown menu */}
          <Menu
            anchorEl={userMenuAnchor}
            open={userMenuOpen}
            onClose={handleUserMenuClose}
            onClick={handleUserMenuClose}
            PaperProps={{
              sx: { minWidth: 180 },
            }}
            transformOrigin={{ horizontal: 'right', vertical: 'top' }}
            anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
          >
            <Box sx={{ px: 2, py: 1 }}>
              <Typography variant="subtitle2" noWrap>
                {user?.username}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {user?.is_admin ? 'Administrator' : 'User'}
              </Typography>
            </Box>
            <Divider />
            <MenuItem onClick={handleOpenChangePassword}>
              <LockIcon fontSize="small" sx={{ mr: 1 }} />
              Change Password
            </MenuItem>
            <MenuItem onClick={handleLogoutClick}>
              <LogoutIcon fontSize="small" sx={{ mr: 1 }} />
              Logout
            </MenuItem>
          </Menu>
        </Toolbar>
      </AppBar>

      <Drawer
        variant="permanent"
        sx={{
          width: DRAWER_WIDTH,
          '& .MuiDrawer-paper': { width: DRAWER_WIDTH, boxSizing: 'border-box' },
        }}
      >
        <Toolbar />
        <List dense>
          {navItems.map((item) => (
            <ListItemButton
              key={item.path}
              component={Link}
              to={item.path}
              selected={location.pathname.startsWith(item.path)}
            >
              <ListItemIcon sx={{ minWidth: 36 }}>{item.icon}</ListItemIcon>
              <ListItemText primary={item.label} />
            </ListItemButton>
          ))}
        </List>
      </Drawer>

      <Box component="main" sx={{ flexGrow: 1, p: 3, ml: `${DRAWER_WIDTH}px` }}>
        <Toolbar />
        {children}
      </Box>

      {/* 修改密码对话框（自助） */}
      <Dialog open={pwdOpen} onClose={closePwdDialog} maxWidth="xs" fullWidth>
        <DialogTitle>Change Password</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            {pwdError && <Typography color="error" variant="body2">{pwdError}</Typography>}
            {pwdSuccess && <Typography color="success.main" variant="body2">Password changed successfully!</Typography>}
            <TextField
              label="Current Password *"
              type="password"
              value={pwdData.old}
              onChange={(e) => setPwdData({ ...pwdData, old: e.target.value })}
              autoFocus
            />
            <TextField
              label="New Password *"
              type="password"
              value={pwdData.new_}
              onChange={(e) => setPwdData({ ...pwdData, new_: e.target.value })}
            />
            <TextField
              label="Confirm New Password *"
              type="password"
              value={pwdData.confirm}
              onChange={(e) => setPwdData({ ...pwdData, confirm: e.target.value })}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={closePwdDialog}>Cancel</Button>
          <Button variant="contained" onClick={handleChangePwd} disabled={pwdSuccess}>Change</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
