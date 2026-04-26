import { EquityChart } from '@/components/charts/EquityChart';
import { PortfolioSummary } from '@/components/portfolio/PortfolioSummary';
import { MarketTable } from '@/components/markets/MarketTable';
import { RiskPanel } from '@/components/risk/RiskPanel';
import { ExecutionBlotter } from '@/components/execution/ExecutionBlotter';

export function DashboardGrid() {
  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
      <PortfolioSummary />
      <RiskPanel />
      <EquityChart />
      <div className="lg:col-span-2">
        <MarketTable />
      </div>
      <ExecutionBlotter />
    </div>
  );
}
