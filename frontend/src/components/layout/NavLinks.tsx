import { NavLink } from 'react-router-dom';
import { Music, Upload, Shield } from 'lucide-react';
import { cn } from '@/lib/utils';

export function NavLinks({ onClick }: { onClick?: () => void }) {
  const navItems = [
    { to: '/', icon: Music, label: 'Library', end: true },
    { to: '/upload', icon: Upload, label: 'Upload' },
    { to: '/admin', icon: Shield, label: 'Admin' },
  ];

  return (
    <nav className="grid gap-1 px-2 py-4">
      {navItems.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          onClick={onClick}
          className={({ isActive }) =>
            cn(
              'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
              isActive
                ? 'bg-secondary text-secondary-foreground'
                : 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'
            )
          }
        >
          <item.icon className="h-4 w-4" />
          {item.label}
        </NavLink>
      ))}
    </nav>
  );
}
