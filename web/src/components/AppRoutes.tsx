import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from '../store/auth'
import Layout from '../components/Layout'
import LoginPage from '../pages/Login'
import UsersPage from '../pages/Users'
import ModulesPage from '../pages/Modules'

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
                <Route index element={<Navigate to="/users" replace />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="modules" element={<ModulesPage />} />
              </Routes>
            </Layout>
          </RequireAuth>
        }
      />
    </Routes>
  )
}
