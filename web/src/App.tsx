import { BrowserRouter } from 'react-router-dom'
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material'
import { AuthProvider } from './store/auth'
import AppRoutes from './components/AppRoutes'

// 蓝绿色主题配色
const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#11998e',
      light: '#38ef7d',
      dark: '#0d7a6e',
      contrastText: '#fff',
    },
    secondary: {
      main: '#38ef7d',
      light: '#6aff9e',
      dark: '#2bc46a',
    },
    background: {
      default: '#f5f7fa',
      paper: '#ffffff',
    },
  },
  components: {
    MuiAppBar: {
      styleOverrides: {
        root: {
          background: 'linear-gradient(135deg, #11998e 0%, #0d7a6e 100%)',
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        contained: {
          background: 'linear-gradient(135deg, #11998e 0%, #38ef7d 100%)',
          '&:hover': {
            background: 'linear-gradient(135deg, #0d7a6e 0%, #2bc46a 100%)',
          },
        },
      },
    },
  },
})

export default function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <AuthProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </AuthProvider>
    </ThemeProvider>
  )
}
