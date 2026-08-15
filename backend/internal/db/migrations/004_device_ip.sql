-- 004_device_ip.sql — capture perangkat & IP saat absen.
-- Device: label perangkat klien (mis. "Pixel 8", "iPhone"). Ip_address: IP klien.

ALTER TABLE attendances ADD COLUMN device     TEXT NOT NULL DEFAULT '';
ALTER TABLE attendances ADD COLUMN ip_address TEXT NOT NULL DEFAULT '';
