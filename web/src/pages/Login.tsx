import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Box, Card, CardContent, TextField, Button, Typography,
  Alert, CircularProgress, Avatar, InputAdornment,
  IconButton, Divider,
} from '@mui/material'
import { styled } from '@mui/material/styles'
import { useAuth } from '../store/auth'
import { authApi } from '../api'
import StorageIcon from '@mui/icons-material/Storage'
import VisibilityIcon from '@mui/icons-material/Visibility'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOff'
import AccountCircleIcon from '@mui/icons-material/AccountCircle'
import LockIcon from '@mui/icons-material/Lock'

const GradientBackground = styled(Box)({
  minHeight: '100vh',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: 'linear-gradient(135deg, #11998e 0%, #0d7a6e 100%)',
  position: 'relative',
  overflow: 'hidden',
  '&::before': {
    content: '""',
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'radial-gradient(circle at 20% 80%, rgba(17, 153, 142, 0.3) 0%, transparent 50%), radial-gradient(circle at 80% 20%, rgba(255, 255, 255, 0.1) 0%, transparent 50%)',
  },
})

const FloatingShape = styled(Box)({
  position: 'absolute',
  borderRadius: '50%',
  background: 'rgba(255, 255, 255, 0.05)',
  animation: 'float 20s infinite ease-in-out',
  '@keyframes float': {
    '0%, 100%': { transform: 'translateY(0) rotate(0deg)' },
    '50%': { transform: 'translateY(-20px) rotate(180deg)' },
  },
})

const StyledCard = styled(Card)({
  width: 420,
  maxWidth: '90vw',
  borderRadius: 16,
  boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
  backdropFilter: 'blur(10px)',
  background: 'rgba(255, 255, 255, 0.95)',
  position: 'relative',
  zIndex: 1,
})

const LogoAvatar = styled(Avatar)({
  width: 72,
  height: 72,
  background: 'linear-gradient(135deg, #11998e 0%, #0d7a6e 100%)',
  boxShadow: '0 10px 20px rgba(17, 153, 142, 0.3)',
  marginBottom: 16,
})

export default function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await authApi.login(username, password)
      login(res.data.data.token, res.data.data.user)
      navigate('/')
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })
        ?.response?.data?.message
      setError(msg || 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  const toggleShowPassword = () => {
    setShowPassword(!showPassword)
  }

  return (
    <GradientBackground>
      {/* 装饰性浮动形状 */}
      <FloatingShape sx={{ width: 300, height: 300, top: '10%', left: '10%', animationDelay: '0s' }} />
      <FloatingShape sx={{ width: 200, height: 200, top: '60%', right: '15%', animationDelay: '5s' }} />
      <FloatingShape sx={{ width: 150, height: 150, bottom: '20%', left: '20%', animationDelay: '10s' }} />

      <StyledCard>
        <CardContent sx={{ p: 4, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          {/* Logo */}
          <LogoAvatar>
            <StorageIcon sx={{ fontSize: 40 }} />
          </LogoAvatar>

          {/* 标题 */}
          <Typography variant="h4" fontWeight="bold" mb={0.5} sx={{
            background: 'linear-gradient(135deg, #11998e 0%, #0d7a6e 100%)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
          }}>
            UFShare
          </Typography>
          <Typography variant="body1" color="text.secondary" mb={3} textAlign="center">
            Artifact Repository Manager
          </Typography>

          {/* 错误提示 */}
          {error && (
            <Alert severity="error" sx={{ mb: 2, width: '100%', borderRadius: 2 }}>
              {error}
            </Alert>
          )}

          {/* 登录表单 */}
          <Box component="form" onSubmit={handleSubmit} display="flex" flexDirection="column" gap={2.5} width="100%">
            <TextField
              label="Username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
              fullWidth
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <AccountCircleIcon color="action" />
                  </InputAdornment>
                ),
              }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  borderRadius: 2,
                },
              }}
            />
            <TextField
              label="Password"
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              fullWidth
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <LockIcon color="action" />
                  </InputAdornment>
                ),
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton onClick={toggleShowPassword} edge="end" size="small">
                      {showPassword ? <VisibilityOffIcon /> : <VisibilityIcon />}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  borderRadius: 2,
                },
              }}
            />

            <Button
              type="submit"
              variant="contained"
              size="large"
              disabled={loading}
              fullWidth
              sx={{
                mt: 1,
                py: 1.5,
                borderRadius: 2,
                background: 'linear-gradient(135deg, #11998e 0%, #0d7a6e 100%)',
                boxShadow: '0 4px 15px rgba(17, 153, 142, 0.4)',
                '&:hover': {
                  background: 'linear-gradient(135deg, #0d7a6e 0%, #11998e 100%)',
                  boxShadow: '0 6px 20px rgba(17, 153, 142, 0.5)',
                },
              }}
            >
              {loading ? <CircularProgress size={24} color="inherit" /> : 'Sign In'}
            </Button>
          </Box>

          <Divider sx={{ width: '100%', my: 3 }} />

          {/* 底部信息 */}
          <Typography variant="caption" color="text.secondary" textAlign="center">
            Default credentials: admin / admin123
          </Typography>
        </CardContent>
      </StyledCard>

      {/* 版本信息 */}
      <Box
        sx={{
          position: 'absolute',
          bottom: 16,
          right: 16,
          color: 'rgba(255, 255, 255, 0.6)',
        }}
      >
        <Typography variant="caption">v2.4.2</Typography>
      </Box>
    </GradientBackground>
  )
}
