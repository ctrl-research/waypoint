import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import './index.css'
import { Shell } from './Shell'
import { HomePage } from './pages/Home'
import { LoginPage } from './pages/Login'
import { TripDetailPage } from './pages/TripDetail'
import { PublicTripPage } from './pages/PublicTrip'
import { StatsPage } from './pages/Stats'

const rootRoute = createRootRoute({ component: Shell })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
})

const tripRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/trips/$tripId',
  component: TripDetailPage,
})

const shareRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/share/$token',
  component: PublicTripPage,
})

const statsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/stats',
  component: StatsPage,
})

const router = createRouter({ routeTree: rootRoute.addChildren([indexRoute, loginRoute, tripRoute, shareRoute, statsRoute]) })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
