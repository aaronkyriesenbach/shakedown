import { Shield, AlertTriangle } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { AdminDump } from '@/components/admin/AdminDump';
import { UserManagement } from '@/components/admin/UserManagement';

export default function AdminPage() {
  return (
    <div className="container max-w-4xl py-8 space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <div className="flex items-center gap-3 border-b pb-6">
        <div className="rounded-xl bg-indigo-500/10 p-3 text-indigo-500">
          <Shield className="h-8 w-8" />
        </div>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Admin Panel</h1>
          <p className="text-muted-foreground">System management and data export.</p>
        </div>
      </div>

      <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 flex items-center gap-4 text-destructive">
        <AlertTriangle className="h-5 w-5 shrink-0" />
        <p className="text-sm font-medium">
          Warning: Admin actions affect all users and system state. Proceed with care.
        </p>
      </div>

      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Data Export</CardTitle>
            <CardDescription>
              Download a complete archive of the system state.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <AdminDump />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>User Management</CardTitle>
            <CardDescription>
              Manage user roles and permissions across the platform.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <UserManagement />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
