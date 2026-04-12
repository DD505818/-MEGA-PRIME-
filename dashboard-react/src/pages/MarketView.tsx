import { Watchlist }        from '../components/widgets/Watchlist'
import { InteractiveChart } from '../components/widgets/InteractiveChart'
import { OrderEntry }        from '../components/widgets/OrderEntry'
import { PortfolioPanel }   from '../components/widgets/PortfolioPanel'
import { RiskPanel }         from '../components/widgets/RiskPanel'
import { LLMChat }           from '../components/widgets/LLMChat'

export function MarketView() {
  return (
    <div className="grid grid-cols-12 gap-2 p-2 h-full">
      {/* Left sidebar */}
      <div className="col-span-2 flex flex-col gap-2 min-h-0">
        <div className="flex-1 min-h-0"><Watchlist /></div>
      </div>

      {/* Center */}
      <div className="col-span-7 flex flex-col gap-2">
        <InteractiveChart />
        <PortfolioPanel />
      </div>

      {/* Right sidebar */}
      <div className="col-span-3 flex flex-col gap-2">
        <OrderEntry />
        <RiskPanel />
        <div className="flex-1 min-h-0"><LLMChat /></div>
      </div>
    </div>
  )
}
