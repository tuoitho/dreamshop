import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { AuthProvider } from '@/context/auth-provider'
import { CartProvider } from '@/context/cart-provider'
import { FavoritesProvider } from '@/context/favorites-provider'
import { Toaster } from 'sonner'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AuthProvider>
      <FavoritesProvider>
        <CartProvider>
          <App />
          <Toaster richColors position="top-center" />
        </CartProvider>
      </FavoritesProvider>
    </AuthProvider>
  </StrictMode>,
)
