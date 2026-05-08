package status

import (
	"time"
)

type FileInstStatusDTO struct {
	Fingerprint string
	Date        time.Time
}

func ToFileInstStatusDTO(fingerprint string, date time.Time) FileInstStatusDTO {
	return FileInstStatusDTO{
		Fingerprint: fingerprint,
		Date:        date,
	}
}

func FromFileInstStatusDTO(dto FileInstStatusDTO) (string, time.Time) {
	return dto.Fingerprint, dto.Date
}
