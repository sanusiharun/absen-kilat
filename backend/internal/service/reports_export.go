package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// MaxExportDays membatasi rentang ekspor supaya satu permintaan tidak menarik
// dataset raksasa dari VPS kecil.
const MaxExportDays = 92

// ExportRow adalah satu baris CSV laporan kehadiran (kontrak di docs).
type ExportRow struct {
	Date           string
	NIP            string
	Name           string
	Email          string
	Type           string
	Status         string
	ApprovalStatus string
	Source         string
	Device         string
	IPAddress      string
	DistanceM      string
}

// AttendanceCSVHeader adalah header tetap laporan kehadiran.
var AttendanceCSVHeader = []string{
	"tanggal", "nip", "nama", "email", "tipe", "status",
	"approval_status", "sumber", "perangkat", "ip", "jarak_meter",
}

// ParseExportRange memvalidasi parameter rentang ekspor. Semua penolakan memakai
// kode 400 (validation) sehingga klien tahu masalahnya tanpa menebak.
func ParseExportRange(fromRaw, toRaw string, loc *time.Location) (time.Time, time.Time, error) {
	if fromRaw == "" || toRaw == "" {
		return time.Time{}, time.Time{}, pkgerrors.Validation("range", "from dan to wajib diisi (YYYY-MM-DD)")
	}
	from, err := time.ParseInLocation("2006-01-02", fromRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, pkgerrors.Validation("from", "format harus YYYY-MM-DD")
	}
	to, err := time.ParseInLocation("2006-01-02", toRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, pkgerrors.Validation("to", "format harus YYYY-MM-DD")
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, pkgerrors.Validation("range", "to tidak boleh sebelum from")
	}
	days := int(to.Sub(from).Hours()/24) + 1
	if days > MaxExportDays {
		return time.Time{}, time.Time{}, pkgerrors.Validation("range",
			fmt.Sprintf("rentang maksimum %d hari, diminta %d", MaxExportDays, days))
	}
	return from, to, nil
}

// ExportAttendance mengambil baris kehadiran pada rentang tanggal (inklusif),
// memakai zona waktu pemanggil agar batas hari konsisten dengan /admin/stats.
func (s *AdminService) ExportAttendance(ctx context.Context, loc *time.Location, fromRaw, toRaw string) ([]ExportRow, error) {
	from, to, err := ParseExportRange(fromRaw, toRaw, loc)
	if err != nil {
		return nil, err
	}
	start := from.UTC()
	end := to.AddDate(0, 0, 1).UTC()

	rows, _, err := s.attendances.ListAdmin(ctx, repo.AdminAttendanceFilter{
		From:  start,
		To:    end,
		Limit: 100000,
	})
	if err != nil {
		return nil, err
	}

	out := make([]ExportRow, 0, len(rows))
	for _, r := range rows {
		dist := ""
		if r.DistanceM.Valid {
			dist = strconv.FormatInt(r.DistanceM.Int64, 10)
		}
		out = append(out, ExportRow{
			Date:           exportDate(r.OccurredAt, loc),
			NIP:            "", // NIP belum diekspos repo baris admin; lihat docs/EXPORT-SCHEMA.md
			Name:           r.UserName,
			Email:          r.UserEmail,
			Type:           r.Type,
			Status:         r.Status,
			ApprovalStatus: r.ApprovalStatus,
			Source:         r.Source,
			Device:         r.Device,
			IPAddress:      r.IPAddress,
			DistanceM:      dist,
		})
	}
	return out, nil
}

// WriteAttendanceCSV menulis CSV memakai encoding/csv sehingga escaping koma,
// tanda kutip, dan newline ditangani standar.
func WriteAttendanceCSV(w io.Writer, rows []ExportRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(AttendanceCSVHeader); err != nil {
		return fmt.Errorf("reports.WriteAttendanceCSV: header: %w", err)
	}
	for _, r := range rows {
		if err := cw.Write([]string{
			r.Date, r.NIP, r.Name, r.Email, r.Type, r.Status,
			r.ApprovalStatus, r.Source, r.Device, r.IPAddress, r.DistanceM,
		}); err != nil {
			return fmt.Errorf("reports.WriteAttendanceCSV: row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// exportDate mengubah timestamp RFC3339 dari repo menjadi tanggal lokal pemanggil.
// Nilai yang tidak terbaca dikembalikan apa adanya supaya tidak ada data hilang diam-diam.
func exportDate(occurred string, loc *time.Location) string {
	ts, err := time.Parse(time.RFC3339, occurred)
	if err != nil {
		return occurred
	}
	return ts.In(loc).Format("2006-01-02")
}
