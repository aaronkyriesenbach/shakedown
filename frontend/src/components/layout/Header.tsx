import { Moon, Sun, Monitor, UserIcon, LogOut } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTheme } from '@/hooks/useTheme';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { useAuth } from '@/hooks/useAuth';
import { useLogout } from '@/api/auth';
import { Link } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export function Header() {
  const { theme, setTheme } = useTheme();
  const { user, isLoading, isAuthenticated } = useAuth();
  const logoutMutation = useLogout();

  const cycleTheme = () => {
    if (theme === 'system') setTheme('light');
    else if (theme === 'light') setTheme('dark');
    else setTheme('system');
  };

  const getInitials = (name: string) => {
    return name
      .split(' ')
      .map((n) => n[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  return (
    <header className="sticky top-0 z-50 flex h-16 items-center gap-4 border-b bg-background px-4 md:px-6">
      <div className="flex sm:hidden items-center gap-2">
        <img src="/icon.png" alt="Shakedown" className="h-6 w-6 rounded" />
        <span className="text-lg font-bold">Shakedown</span>
      </div>
      <div className="hidden sm:flex flex-1" />
      <div className="flex flex-1 sm:flex-none justify-end items-center gap-4">
        <Button
          variant="ghost"
          size="icon"
          onClick={cycleTheme}
          className="rounded-full"
        >
          {theme === 'system' ? (
            <Monitor className="h-5 w-5" />
          ) : theme === 'dark' ? (
            <Moon className="h-5 w-5" />
          ) : (
            <Sun className="h-5 w-5" />
          )}
          <span className="sr-only">Toggle theme</span>
        </Button>
        {isLoading ? (
          <Avatar className="h-8 w-8">
            <AvatarFallback className="animate-pulse" />
          </Avatar>
        ) : isAuthenticated && user ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Avatar className="h-8 w-8 cursor-pointer hover:opacity-80 transition-opacity">
                {user.avatar_url && <AvatarImage src={user.avatar_url} alt={user.display_name} />}
                <AvatarFallback>
                  {user.display_name ? getInitials(user.display_name) : <UserIcon className="h-4 w-4" />}
                </AvatarFallback>
              </Avatar>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <div className="flex items-center justify-start gap-2 p-2">
                <div className="flex flex-col space-y-1 leading-none">
                  <p className="font-medium">{user.display_name}</p>
                  <p className="w-[200px] truncate text-sm text-muted-foreground">
                    {user.email}
                  </p>
                </div>
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-red-600 focus:text-red-600 cursor-pointer"
                onClick={() => logoutMutation.mutate()}
              >
                <LogOut className="mr-2 h-4 w-4" />
                <span>Log out</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : (
          <Button asChild variant="default" size="sm">
            <Link to="/login">Login</Link>
          </Button>
        )}
      </div>
    </header>
  );
}
