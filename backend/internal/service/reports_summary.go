package service

import (
	"context"
	"sort"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// SummaryDay adalah agregat satu hari untuk laporan ringkas.
type SummaryDay struct {
	Date    string `json:"date"`
	Hadir   int    `json:"hadir"`
	Telat   int    `json:"telat"`
	Pulang  int    `json:"pulang"`
	Lainnya int    `json:"lainnya"`
}

// ReportSummary adalah ringkasan kehadiran pada satu rentang tanggal.
type ReportSummary struct {
	From   string       `json:"from"`
	To     string       `json:"to"`
	Totals SummaryDay   `json:"totals"`
	ByDay  []SummaryDay `json:"by_day"`
}

// SummaryReports menghitung ringkasan kehadiran per hari pada rentang tanggal,
// memakai zona waktu pemanggil agar batas hari konsisten dengan endpoint lain.
// Angka berasal langsung dari baris absensi (bukan dummy); lembur belum diikutkan
// karena modul lembur punya sumber data sendiri.
func (s *AdminService) SummaryReports(ctx context.Context, loc *time.Location, fromRaw, toRaw string) (*ReportSummary, error) {
	rows, err := s.ExportAttendance(ctx, loc, fromRaw, toRaw)
	if err != nil {
		return nil, err
	}

	perDay := map[string]*SummaryDay{}
	for _, r := range rows {
		d, ok := perDay[r.Date]
		if !ok {
			d = &SummaryDay{Date: r.Date}
			perDay[r.Date] = d
		}
		switch {
		case r.Type == string(domain.AttendanceCheckOut):
			d.Pulang++
		case r.Status == string(domain.StatusLate):
			d.Telat++
		case r.Status == string(domain.StatusOnTime):
			d.Hadir++
		default:
			d.Lainnya++
		}
	}

	out := &ReportSummary{From: fromRaw, To: toRaw, ByDay: make([]SummaryDay, 0, len(perDay))}
	for _, d := range perDay {
		out.ByDay = append(out.ByDay, *d)
		out.Totals.Hadir += d.Hadir
		out.Totals.Telat += d.Telat
		out.Totals.Pulang += d.Pulang
		out.Totals.Lainnya += d.Lainnya
	}
	sort.Slice(out.ByDay, func(i, j int) bool { return out.ByDay[i].Date < out.ByDay[j].Date })
	return out, nil
}
