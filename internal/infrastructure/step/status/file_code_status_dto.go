package status

import (
	"time"
)

type FileCodeStatusDTO struct {
	Fingerprint string
	Date        time.Time
}

func ToFileCodeStatusDTO(fingerprint string, date time.Time) FileCodeStatusDTO {
	return FileCodeStatusDTO{
		Fingerprint: fingerprint,
		Date:        date,
	}
}

func FromFileCodeStatusDTO(dto FileCodeStatusDTO) (string, time.Time) {
	return dto.Fingerprint, dto.Date
}
