package status

import (
	"time"
)

type FileVarsStatusDTO struct {
	Fingerprint string
	Date        time.Time
}

func ToFileVarsStatusDTO(fingerprint string, date time.Time) FileVarsStatusDTO {
	return FileVarsStatusDTO{
		Fingerprint: fingerprint,
		Date:        date,
	}
}

func FromFileVarsStatusDTO(dto FileVarsStatusDTO) (string, time.Time) {
	return dto.Fingerprint, dto.Date
}
