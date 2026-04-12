import { BrowserRouter, Routes, Route, Navigate, NavLink, useLocation } from 'react-router-dom'
import { useWebSocket }    from './hooks/useWebSocket'
import { useAppStore }     from './store/useAppStore'
import { TopBar }          from './components/layout/TopBar'
import { StatusBar }       from './components/layout/StatusBar'
import { Onboarding }      from './pages/Onboarding'
import { MarketView }      from './pages/MarketView'
import { ExecutionView }   from './pages/ExecutionView'
import { PortfolioView }   from './pages/PortfolioView'
import { AnalyticsView }   from './pages/AnalyticsView'
import { AgentsView }      from './pages/AgentsView'
import { DeFiView }        from './pages/DeFiView'
import { ComplianceView }  from './pages/ComplianceView'
import { SettingsView }    from './pages/SettingsView'
import clsx from 'clsx'

const TABS = [
  { path: '/market',     label: 'Market' },
  { path: '/execution',  label: 'Execution' },
  { path: '/portfolio',  label: 'Portfolio' },
  { path: '/analytics',  label: 'Analytics' },
  { path: '/agents',     label: 'Agents' },
  { path: '/defi',       label: 'DeFi' },
  { path: '/compliance', label: 'Compliance' },
  { path: '/settings',   label: 'Settings' },
]

function TabBar() {
  return (
    <nav className="flex border-b border-border bg-panel/60 px-2">
      {TABS.map((t) => (
        <NavLink key={t.path} to={t.path}
          className={({ isActive }) => clsx('tab', isActive && 'tab-active')}>
          {t.label}
        </NavLink>
      ))}
    </nav>
  )
}

function DashboardLayout() {
  // Initialize WebSocket connection
  useWebSocket()

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <TopBar />
      <TabBar />
      <main className="flex-1 overflow-y-auto">
        <Routes>
          <Route path="/"           element={<Navigate to="/market" replace />} />
          <Route path="/market"     element={<MarketView />} />
          <Route path="/execution"  element={<ExecutionView />} />
          <Route path="/portfolio"  element={<PortfolioView />} />
          <Route path="/analytics"  element={<AnalyticsView />} />
          <Route path="/agents"     element={<AgentsView />} />
          <Route path="/defi"       element={<DeFiView />} />
          <Route path="/compliance" element={<ComplianceView />} />
          <Route path="/settings"   element={<SettingsView />} />
        </Routes>
      </main>
      <StatusBar />
    </div>
  )
}

export default function App() {
  const onboardingComplete = useAppStore((s) => s.onboardingComplete)

  return (
    <BrowserRouter>
      {onboardingComplete ? <DashboardLayout /> : <Onboarding />}
    </BrowserRouter>
  )
}
