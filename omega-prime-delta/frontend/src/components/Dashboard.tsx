import { LiveMarketTicker } from './LiveMarketTicker';
import { InternationalClocks } from './InternationalClocks';
import { InteractiveChart } from './InteractiveChart';
import { OrderBook } from './OrderBook';
import { OrderEntryPanel } from './OrderEntryPanel';
import { PortfolioPanel } from './PortfolioPanel';
import { AIAgentHub } from './AIAgentHub';
import { LLMChatAssistant } from './LLMChatAssistant';
import { AnomalyMonitor } from './AnomalyMonitor';
import { DeFiDashboard } from './DeFiDashboard';
import { PaymentPortal } from './PaymentPortal';

export function Dashboard() {
  return (
    <main style={{ padding: 16, display: 'grid', gap: 12 }}>
      <h1>ΩMEGA Prime Δ Dashboard</h1>
      <LiveMarketTicker />
      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 12 }}>
        <InteractiveChart />
        <InternationalClocks />
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
        <OrderBook />
        <OrderEntryPanel />
        <PortfolioPanel />
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <AIAgentHub />
        <LLMChatAssistant />
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
        <AnomalyMonitor />
        <DeFiDashboard />
        <PaymentPortal />
      </div>
    </main>
  );
}
