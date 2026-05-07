import { useMemo } from 'react';
import { PieChart, Pie, Cell, Tooltip, Legend } from 'recharts';
import { useWebSocket } from '../../hooks/useWebSocket';

const demoAllocations = [
  { strategy: 'MAMBA', amount: 35 },
  { strategy: 'NEXUS', amount: 25 },
  { strategy: 'ORBIT', amount: 20 },
  { strategy: 'SURGE', amount: 20 },
];

export function HedgeFundPanel() {
  const { sendMessage } = useWebSocket('/ws/hedgefund');
  const allocations = useMemo(() => demoAllocations, []);

  const requestRebalance = () => sendMessage(JSON.stringify({ type: 'rebalance_request' }));

  return (
    <div className="bg-gray-900 p-4 rounded border border-gray-800">
      <h3 className="text-cyan-400 font-mono">🏦 Hedge Fund Dashboard</h3>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <h4 className="text-gray-400">Capital Allocation</h4>
          <PieChart width={240} height={220}>
            <Pie data={allocations} dataKey="amount" nameKey="strategy" outerRadius={80}>
              {allocations.map((_, i) => (
                <Cell key={i} fill={`hsl(${i * 45}, 70%, 50%)`} />
              ))}
            </Pie>
            <Tooltip />
            <Legend />
          </PieChart>
        </div>
        <div>
          <h4 className="text-gray-400">Strategy Scores</h4>
          <button onClick={requestRebalance} className="bg-blue-600 px-3 py-1 rounded text-white">
            Request Rebalance
          </button>
        </div>
      </div>
    </div>
  );
}
