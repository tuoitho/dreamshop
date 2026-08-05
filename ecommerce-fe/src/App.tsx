import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { RequireAuth } from "@/routes/require-auth"
import LoginPage from "@/pages/login-page"
import RegisterPage from "@/pages/register-page"
import DashboardPage from "@/pages/dashboard-page"

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route
          path="/dashboard"
          element={
            <RequireAuth>
              <DashboardPage />
            </RequireAuth>
          }
        />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
