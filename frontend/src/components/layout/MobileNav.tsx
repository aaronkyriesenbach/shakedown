import { Link, useLocation } from 'react-router-dom';
import { Home, Upload, Shield } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAuth } from '@/hooks/useAuth';

export function MobileNav() {
  const location = useLocation();
  const { user } = useAuth();
  
  const navItems = [
    { label: 'Library', icon: Home, path: '/' },
    { label: 'Upload', icon: Upload, path: '/upload' },
  ];
  
  if (user?.role === 'admin') {
    navItems.push({ label: 'Admin', icon: Shield, path: '/admin' });
  }

  return (
    <nav className="flex sm:hidden fixed bottom-0 left-0 right-0 z-50 bg-background border-t pb-safe">
      <div className="flex w-full justify-around items-center h-16 px-2">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = location.pathname === item.path || (item.path !== '/' && location.pathname.startsWith(item.path));
          
          return (
            <Link
              key={item.path}
              to={item.path}
              className={cn(
                "flex flex-col items-center justify-center w-full h-full gap-1 transition-colors text-muted-foreground hover:text-foreground",
                isActive && "text-violet-500 hover:text-violet-600 dark:text-violet-400 dark:hover:text-violet-300"
              )}
            >
              <Icon className="w-5 h-5" />
              <span className="text-[10px] font-medium">{item.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
