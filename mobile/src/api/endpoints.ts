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
  occurred_at: string;
}

export interface TodayState {
  checked_in: boolean;
  checked_out: boolean;
  records: Attendance[];
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
  today: () => api.get<TodayState>('/attendance/today'),
  list: (date: string) => api.get<{ records: Attendance[]; date: string }>(`/attendance?date=${date}`),
  detail: (id: string) => api.get<Attendance>(`/attendance/${id}`),
};
