import './App.css';
import { LiveMarketTicker } from './components/LiveMarketTicker';
import { InternationalClocks } from './components/InternationalClocks';
import { InteractiveChart } from './components/InteractiveChart';
import { DragAndDropLayout } from './components/DragAndDropLayout';
import { OrderBook } from './components/OrderBook';
import { OrderEntryPanel } from './components/OrderEntryPanel';
import { PortfolioPanel } from './components/PortfolioPanel';
import { AIAgentHub } from './components/AIAgentHub';
import { LLMChatAssistant } from './components/LLMChatAssistant';
import { AnomalyMonitor } from './components/AnomalyMonitor';
import { DeFiDashboard } from './components/DeFiDashboard';
import { PaymentPortal } from './components/PaymentPortal';

function App() {
  return (
    <main className="app-shell">
      <header className="hero">
        <h1>OMEGA PRIME // Unified Trading Command Center</h1>
        <p>Production-grade multi-asset monitoring, execution and AI operations cockpit.</p>
      </header>

      <LiveMarketTicker />

      <div className="layout-two-col">
        <InteractiveChart />
        <InternationalClocks />
      </div>

      <div className="layout-three-col">
        <OrderBook />
        <OrderEntryPanel />
        <PortfolioPanel />
      </div>

      <div className="layout-two-col">
        <AIAgentHub />
        <LLMChatAssistant />
      </div>

      <div className="layout-three-col">
        <AnomalyMonitor />
        <DeFiDashboard />
        <PaymentPortal />
      </div>

      <DragAndDropLayout />
    </main>
  );
}

export default App;
