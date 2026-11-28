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

// exportPageSize adalah ukuran satu halaman query. Nilainya sama dengan batas
// keras ListAdmin (repo meng-clamp limit ke 500), jadi paginasi berjalan penuh
// tanpa ada baris yang dipotong diam-diam.
const exportPageSize = 500

// MaxExportRows adalah plafon keamanan ekspor. Di atas plafon ini permintaan
// DITOLAK dengan error eksplisit (bukan CSV parsial): sebelumnya ekspor meminta
// 100000 baris, repo meng-clamp ke 500, dan klien menerima 200 + CSV terpotong
// tanpa tanda apa pun. Variabel (bukan konstanta) supaya tes bisa menurunkannya.
var MaxExportRows = 100000

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
//
// Seluruh baris pada rentang diambil lewat paginasi (offset bertahap) sehingga
// tidak ada pemotongan diam-diam pada 500 baris. Bila jumlah baris melebihi
// MaxExportRows, permintaan ditolak dengan error validasi yang menyebut jumlah
// baris sebenarnya — klien tidak pernah menerima CSV parsial tanpa penjelasan.
func (s *AdminService) ExportAttendance(ctx context.Context, loc *time.Location, fromRaw, toRaw string) ([]ExportRow, error) {
	from, to, err := ParseExportRange(fromRaw, toRaw, loc)
	if err != nil {
		return nil, err
	}
	start := from.UTC()
	end := to.AddDate(0, 0, 1).UTC()

	var out []ExportRow
	offset := 0
	for {
		rows, total, err := s.attendances.ListAdmin(ctx, repo.AdminAttendanceFilter{
			From:   start,
			To:     end,
			Limit:  exportPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		if total > MaxExportRows {
			return nil, pkgerrors.Validation("range", fmt.Sprintf(
				"rentang memuat %d baris (plafon %d); persempit rentang tanggal atau filter laporan",
				total, MaxExportRows))
		}
		for _, r := range rows {
			out = append(out, newExportRow(r, loc))
		}
		offset += len(rows)
		if len(rows) == 0 || len(rows) < exportPageSize || offset >= total {
			break
		}
	}
	return out, nil
}

// newExportRow memetakan satu baris admin ke baris CSV laporan.
func newExportRow(r repo.AdminAttendanceRow, loc *time.Location) ExportRow {
	dist := ""
	if r.DistanceM.Valid {
		dist = strconv.FormatInt(r.DistanceM.Int64, 10)
	}
	return ExportRow{
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
	}
}

// WriteAttendanceCSV menulis CSV memakai encoding/csv sehingga escaping koma,
// tanda kutip, dan newline ditangani standar.
//
// Setiap sel dilewatkan sanitizeCSVCell lebih dulu: nilai yang dimulai dengan
// karakter pemicu formula spreadsheet di-escape supaya isian pengguna (nama,
// perangkat, alamat IP) tidak bisa menjalankan formula di Excel/Sheets saat
// laporan dibuka admin.
func WriteAttendanceCSV(w io.Writer, rows []ExportRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(AttendanceCSVHeader); err != nil {
		return fmt.Errorf("reports.WriteAttendanceCSV: header: %w", err)
	}
	for _, r := range rows {
		if err := cw.Write([]string{
			sanitizeCSVCell(r.Date), sanitizeCSVCell(r.NIP), sanitizeCSVCell(r.Name),
			sanitizeCSVCell(r.Email), sanitizeCSVCell(r.Type), sanitizeCSVCell(r.Status),
			sanitizeCSVCell(r.ApprovalStatus), sanitizeCSVCell(r.Source),
			sanitizeCSVCell(r.Device), sanitizeCSVCell(r.IPAddress), sanitizeCSVCell(r.DistanceM),
		}); err != nil {
			return fmt.Errorf("reports.WriteAttendanceCSV: row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// sanitizeCSVCell menetralkan CSV formula injection (OWASP): sel yang dimulai
// dengan = + - @ TAB CR dianggap formula oleh Excel/Sheets/LibreOffice, sehingga
// diberi awalan kutip tunggal agar tetap tampil sebagai teks.
func sanitizeCSVCell(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '	', '\r', '\v', '\f':
		return "'" + v
	}
	return v
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
