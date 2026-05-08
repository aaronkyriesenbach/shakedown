import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import LibraryPage from '@/pages/LibraryPage';
import UploadPage from '@/pages/UploadPage';
import AdminPage from '@/pages/AdminPage';
import RecordingPage from '@/pages/RecordingPage';
import SharePage from '@/pages/SharePage';
import LoginPage from '@/pages/LoginPage';
import { AuthGuard } from '@/components/auth/AuthGuard';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<AppLayout />}>
          <Route index element={<AuthGuard><LibraryPage /></AuthGuard>} />
          <Route path="upload" element={<AuthGuard><UploadPage /></AuthGuard>} />
          <Route path="admin" element={<AuthGuard requireAdmin><AdminPage /></AuthGuard>} />
          <Route path="recordings/:id" element={<AuthGuard><RecordingPage /></AuthGuard>} />
        </Route>
        <Route path="/s/:token" element={<SharePage />} />
        <Route path="/login" element={<LoginPage />} />
      </Routes>
    </BrowserRouter>
  );
}
