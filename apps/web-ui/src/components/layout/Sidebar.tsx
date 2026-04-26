import Link from 'next/link';

const routes = ['/', '/markets', '/portfolio', '/agents', '/studio', '/rl', '/execution', '/risk', '/reports', '/settings'];

export function Sidebar() {
  return (
    <aside className="hidden w-56 border-r border-white/10 p-4 md:block">
      <p className="mb-4 text-xs uppercase text-white/60">ΩMEGA PRIME Δ</p>
      <nav className="space-y-2 text-sm">
        {routes.map((route) => (
          <Link key={route} href={route} className="block rounded px-2 py-1 hover:bg-white/10">
            {route === '/' ? 'dashboard' : route.slice(1)}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
