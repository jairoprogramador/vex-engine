package status

import (
	"time"
)

type FileTimeStatusDTO struct {
	At   time.Time
	Date time.Time
}

func ToFileTimeStatusDTO(at, date time.Time) FileTimeStatusDTO {
	return FileTimeStatusDTO{
		At:   at,
		Date: date,
	}
}

func FromFileTimeStatusDTO(dto FileTimeStatusDTO) (time.Time, time.Time) {
	return dto.At, dto.Date
}
