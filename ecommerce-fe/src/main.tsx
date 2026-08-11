import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { AuthProvider } from '@/context/auth-provider'
import { CartProvider } from '@/context/cart-provider'
import { Toaster } from 'sonner'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AuthProvider>
      <CartProvider>
        <App />
        <Toaster richColors position="top-right" />
      </CartProvider>
    </AuthProvider>
  </StrictMode>,
)
