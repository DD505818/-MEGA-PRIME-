export function ComplianceView() {
  const reports = [
    { name: 'MiFID II Daily Report',   date: '2026-04-12', status: 'Ready' },
    { name: 'SEC Form ADV',            date: '2026-04-01', status: 'Ready' },
    { name: 'Risk Capital Statement',  date: '2026-04-12', status: 'Ready' },
    { name: 'Trade Execution Report',  date: '2026-04-12', status: 'Generating' },
  ]
  return (
    <div className="p-3 grid grid-cols-2 gap-3">
      <div className="card">
        <p className="kicker mb-3">Regulatory Reports</p>
        <div className="space-y-2">
          {reports.map((r) => (
            <div key={r.name} className="flex items-center justify-between border border-border/50 rounded-lg p-2">
              <div>
                <p className="text-sm font-semibold text-text">{r.name}</p>
                <p className="text-xs text-muted">{r.date}</p>
              </div>
              <button className={`text-xs px-3 py-1 rounded border transition-all ${r.status === 'Ready'
                ? 'border-green/40 text-green hover:bg-green/10'
                : 'border-gold/40 text-gold cursor-wait'}`}>
                {r.status === 'Ready' ? 'Download' : r.status}
              </button>
            </div>
          ))}
        </div>
      </div>
      <div className="card">
        <p className="kicker mb-3">System Health Certificate</p>
        <div className="flex flex-col items-center justify-center h-40 gap-3 text-center">
          <div className="w-16 h-16 rounded-full border-4 border-green/60 bg-green/10 flex items-center justify-center">
            <span className="text-green text-2xl">✓</span>
          </div>
          <p className="text-sm text-text font-semibold">All systems operational</p>
          <p className="text-xs text-muted">Last audit: 2026-04-12 06:00 UTC</p>
          <p className="text-xs text-muted">Risk controls verified: PASS</p>
        </div>
      </div>
    </div>
  )
}
