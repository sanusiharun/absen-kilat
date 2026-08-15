-- 005_geofence.sql — jarak dari titik kantor saat absen (anti-spoofing geofencing).
-- distance_m: NULL jika lokasi tidak dikirim / geofencing nonaktif.

ALTER TABLE attendances ADD COLUMN distance_m INTEGER; -- meter dari titik kantor
