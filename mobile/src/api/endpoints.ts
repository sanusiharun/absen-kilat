// Auth + profile + attendance API functions.
import { api } from './client';

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  is_email_verified: boolean;
  notification_enabled: boolean;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: string;
}

export interface Attendance {
  id: string;
  type: 'check_in' | 'check_out';
  status: 'on_time' | 'late';
  photo_url: string;
  latitude?: number;
  longitude?: number;
  address?: string;
  device?: string;
  ip_address?: string;
  distance_m?: number;
  notes?: string;
  occurred_at: string;
  // FASE B: status persetujuan + sumber absen + lokasi terpilih.
  approval_status?: 'pending' | 'approved' | 'rejected';
  source?: string;
  location_id?: string;
  location_name?: string;
}

// Lokasi absen (GET /locations): hanya yang active, type ∈ office|client|field.
export interface AbsenLocation {
  id: string;
  name: string;
  type: 'office' | 'client' | 'field';
}

// Pengaturan publik (GET /settings/public).
export interface PublicSettings {
  work_start: string;
  work_end: string;
  absen_mode: 'both' | 'checkin_only';
  photo_policy: string;
}

export interface TodayState {
  checked_in: boolean;
  checked_out: boolean;
  records: Attendance[];
}

export interface Notification {
  id: string;
  title: string;
  body: string;
  is_read: boolean;
  created_at: string;
}

export const authApi = {
  register: (email: string, password: string, name: string) =>
    api.post<User>('/auth/register', { email, password, name }),
  login: (email: string, password: string) =>
    api.post<TokenPair>('/auth/login', { email, password }, { auth: false }),
  refresh: (refresh_token: string) =>
    api.post<TokenPair>('/auth/refresh', { refresh_token }, { auth: false }),
  forgotPassword: (email: string) =>
    api.post<{ sent: boolean }>('/auth/forgot-password', { email }, { auth: false }),
  resetPassword: (token: string, new_password: string) =>
    api.post<{ reset: boolean }>('/auth/reset-password', { token, new_password }, { auth: false }),
  verifyEmail: (token: string) =>
    api.post<{ verified: boolean }>('/auth/verify-email', { token }, { auth: false }),
};

export const profileApi = {
  me: () => api.get<User>('/me'),
  update: (data: { name?: string; notification_enabled?: boolean }) =>
    api.patch<User>('/me', data),
  uploadAvatar: (formData: FormData) => api.upload<User>('/me/avatar', formData),
  changePassword: (current_password: string, new_password: string) =>
    api.post<{ changed: boolean }>('/me/password', { current_password, new_password }),
};

export const attendanceApi = {
  checkIn: (formData: FormData) => api.upload<Attendance>('/attendance', formData),
  checkOut: (formData: FormData) => api.upload<Attendance>('/attendance', formData),
  submit: (formData: FormData) => api.upload<Attendance>('/attendance', formData),
  today: () => api.get<TodayState>('/attendance/today'),
  list: (date: string) => api.get<{ records: Attendance[]; date: string }>(`/attendance?date=${date}`),
  detail: (id: string) => api.get<Attendance>(`/attendance/${id}`),
};

export const geocodeApi = {
  reverse: (lat: number, lng: number) =>
    api.get<{ address: string; short_address: string }>(`/geocode?lat=${lat}&lng=${lng}`),
};

// FASE B: daftar lokasi absen + pengaturan publik.
export const locationApi = {
  list: () => api.get<{ locations: AbsenLocation[] }>('/locations'),
};

export const settingsApi = {
  public: () => api.get<PublicSettings>('/settings/public'),
};

export const notificationApi = {
  list: () => api.get<{ notifications: Notification[] }>('/notifications'),
  unreadCount: () => api.get<{ unread: number }>('/notifications/unread-count'),
  markRead: (id: string) => api.post<{ ok: boolean }>(`/notifications/${id}/read`),
  markAllRead: () => api.post<{ ok: boolean }>('/notifications/read-all'),
};

// Modul Cuti (leave management).
export interface LeaveType {
  code: string;
  name: string;
  default_days: number;
  max_extra_days: number;
}

export interface Leave {
  id: string;
  user_id: string;
  leave_type: string;
  start_date: string;
  end_date: string;
  total_days: number;
  reason?: string;
  status: 'pending' | 'approved' | 'rejected';
  approved_by?: string;
  approved_at?: string;
  rejection_reason?: string;
  attachment_url?: string;
  created_at: string;
}

export const leaveApi = {
  types: () => api.get<{ leave_types: LeaveType[] }>('/leave/types'),
  list: () => api.get<{ leaves: Leave[] }>('/leave'),
  apply: (body: {
    leave_type: string;
    start_date: string;
    end_date: string;
    reason?: string;
    extra_days?: number;
  }) => api.post<{ leave: Leave }>('/leave', body),
  cancel: (id: string) => api.post<{ leave: Leave }>(`/leave/${id}/cancel`),
};
