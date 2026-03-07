import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from '../store/auth'
import Layout from '../components/Layout'
import LoginPage from '../pages/Login'
import UsersPage from '../pages/Users'
import FilesPage from '../pages/Files'
import NpmPage from '../pages/Npm'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { token } = useAuth()
  return token ? <>{children}</> : <Navigate to="/login" replace />
}

export default function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/*"
        element={
          <RequireAuth>
            <Layout>
              <Routes>
                <Route index element={<Navigate to="/files" replace />} />
                <Route path="files" element={<FilesPage />} />
                <Route path="npm" element={<NpmPage />} />
                <Route path="users" element={<UsersPage />} />
              </Routes>
            </Layout>
          </RequireAuth>
        }
      />
    </Routes>
  )
}
