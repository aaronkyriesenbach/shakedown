import { useAdminUsers, useUpdateUserRole } from '@/api/admin';
import { useMe } from '@/api/auth';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { UserCheck, UserX, Loader2 } from 'lucide-react';

export function UserManagement() {
  const { data: users, isLoading } = useAdminUsers();
  const { mutate: updateRole, isPending } = useUpdateUserRole();
  const { data: currentUser } = useMe();

  if (isLoading) {
    return (
      <div className="space-y-4">
        {[1, 2, 3].map((i) => (
          <div key={i} className="flex items-center justify-between p-4 border rounded-lg animate-pulse">
            <div className="flex items-center gap-4">
              <div className="h-10 w-10 bg-muted rounded-full" />
              <div className="space-y-2">
                <div className="h-4 w-32 bg-muted rounded" />
                <div className="h-3 w-48 bg-muted rounded" />
              </div>
            </div>
            <div className="h-9 w-28 bg-muted rounded" />
          </div>
        ))}
      </div>
    );
  }

  if (!users || users.length === 0) {
    return (
      <div className="text-center p-8 border rounded-lg bg-muted/20 text-muted-foreground">
        No users found.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {users.map((user) => {
        const isSelf = currentUser?.id === user.id;
        const isAdmin = user.role === 'admin';

        return (
          <div key={user.id} className="flex items-center justify-between p-4 border rounded-lg transition-colors hover:bg-muted/20">
            <div className="flex items-center gap-4">
              <Avatar>
                <AvatarImage src={user.avatar_url} alt={user.display_name} />
                <AvatarFallback>{user.display_name.slice(0, 2).toUpperCase()}</AvatarFallback>
              </Avatar>
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-medium">{user.display_name}</span>
                  {isAdmin ? (
                    <Badge variant="default" className="bg-indigo-500 hover:bg-indigo-600">Admin</Badge>
                  ) : (
                    <Badge variant="secondary">User</Badge>
                  )}
                  {isSelf && <Badge variant="outline" className="text-muted-foreground">You</Badge>}
                </div>
                <p className="text-sm text-muted-foreground">{user.email}</p>
              </div>
            </div>

            <Button
              variant={isAdmin ? "outline" : "secondary"}
              size="sm"
              disabled={isSelf || isPending}
              onClick={() => updateRole({ userId: user.id, role: isAdmin ? 'user' : 'admin' })}
            >
              {isPending ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : isAdmin ? (
                <UserX className="mr-2 h-4 w-4" />
              ) : (
                <UserCheck className="mr-2 h-4 w-4" />
              )}
              {isAdmin ? "Revoke Admin" : "Make Admin"}
            </Button>
          </div>
        );
      })}
    </div>
  );
}
