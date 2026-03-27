import { useEffect, useRef } from 'react';
import { AreaSeries, createChart, type IChartApi } from 'lightweight-charts';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { pnlSeries } from '../data/mockData';
import { Panel } from './Panel';

const candles = [
  { time: '2026-03-21', value: 67800 },
  { time: '2026-03-22', value: 68100 },
  { time: '2026-03-23', value: 67650 },
  { time: '2026-03-24', value: 68420 },
  { time: '2026-03-25', value: 68910 },
  { time: '2026-03-26', value: 68520 },
  { time: '2026-03-27', value: 68420 },
];


export function InteractiveChart() {
  const tvRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!tvRef.current) return;

    const chart: IChartApi = createChart(tvRef.current, {
      layout: { background: { color: 'transparent' }, textColor: '#d9f4ff' },
      grid: { vertLines: { color: '#1e3659' }, horzLines: { color: '#1e3659' } },
      width: tvRef.current.clientWidth,
      height: 280,
      rightPriceScale: { borderColor: '#4077b2' },
      timeScale: { borderColor: '#4077b2' },
    });

    const area = chart.addSeries(AreaSeries, {
      lineColor: '#4de8ff',
      topColor: 'rgba(77,232,255,0.35)',
      bottomColor: 'rgba(77,232,255,0.04)',
    });
    area.setData(candles);

    const observer = new ResizeObserver(() => {
      if (tvRef.current) chart.applyOptions({ width: tvRef.current.clientWidth });
    });
    observer.observe(tvRef.current);

    return () => {
      observer.disconnect();
      chart.remove();
    };
  }, []);

  return (
    <Panel title="Interactive Chart" subtitle="TradingView Lightweight + Recharts P&L">
      <div ref={tvRef} className="chart-tv" />
      <div className="chart-pnl">
        <ResponsiveContainer width="100%" height={220}>
          <AreaChart data={pnlSeries}>
            <defs>
              <linearGradient id="pnlGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#84f1ff" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#84f1ff" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#23466f" />
            <XAxis dataKey="time" stroke="#d9f4ff" />
            <YAxis stroke="#d9f4ff" />
            <Tooltip />
            <Area type="monotone" dataKey="pnl" stroke="#84f1ff" fillOpacity={1} fill="url(#pnlGradient)" />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </Panel>
  );
}
