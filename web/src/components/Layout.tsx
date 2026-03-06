import { type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  AppBar, Box, Drawer, List, ListItemButton, ListItemIcon, ListItemText,
  Toolbar, Typography, IconButton, Tooltip,
} from '@mui/material'
import PeopleIcon from '@mui/icons-material/People'
import ExtensionIcon from '@mui/icons-material/Extension'
import FolderIcon from '@mui/icons-material/Folder'
import LogoutIcon from '@mui/icons-material/Logout'
import { useAuth } from '../store/auth'

const DRAWER_WIDTH = 200

const navItems = [
  { label: 'Files', path: '/files', icon: <FolderIcon /> },
  { label: 'Users', path: '/users', icon: <PeopleIcon /> },
  { label: 'Modules', path: '/modules', icon: <ExtensionIcon /> },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <Box display="flex">
      <AppBar position="fixed" sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
        <Toolbar>
          <Typography variant="h6" fontWeight="bold" sx={{ flexGrow: 1 }}>
            UFShare
          </Typography>
          <Typography variant="body2" sx={{ mr: 2, opacity: 0.8 }}>
            {user?.username}
          </Typography>
          <Tooltip title="Logout">
            <IconButton color="inherit" onClick={handleLogout}>
              <LogoutIcon />
            </IconButton>
          </Tooltip>
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
    </Box>
  )
}
