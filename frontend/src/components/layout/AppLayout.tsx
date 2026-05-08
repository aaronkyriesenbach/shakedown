import { Outlet } from 'react-router-dom';
import { Music } from 'lucide-react';
import { Header } from './Header';
import { NavLinks } from './NavLinks';
import { MobileNav } from './MobileNav';

export function AppLayout() {
  return (
    <div className="flex min-h-screen w-full bg-background text-foreground">
      {/* Desktop Sidebar */}
      <aside className="hidden w-[240px] flex-col border-r bg-card/50 sm:flex">
        <div className="flex h-16 items-center gap-2 border-b px-6">
          <Music className="h-6 w-6" />
          <span className="text-lg font-bold tracking-tight">Shakedown</span>
        </div>
        <div className="flex-1 overflow-auto">
          <NavLinks />
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex flex-1 flex-col overflow-hidden pb-16 sm:pb-0">
        <Header />
        <main className="flex-1 overflow-auto p-4 md:p-6 lg:p-8">
          <Outlet />
        </main>
      </div>

      <MobileNav />
    </div>
  );
}
